package sync

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/crom"
)

const cromRetentionDays = 500

type CromSite struct {
	SiteURL     string
	Display     string
	SweepCursor string
}

type CromStats struct {
	Site        string
	Swept       int
	Dirty       int
	Detailed    int
	Gone        int
	DayRows     int
	VoterRows   int
	Duration    time.Duration
	SweepDone   bool
	BudgetLeft  int
	LastVoteDay time.Time
}

func (e *Engine) cromSites(ctx context.Context, only string) ([]CromSite, error) {
	q := `SELECT site_url, display, COALESCE(sweep_cursor, '') FROM crom_sites`
	var args []any
	if only != "" {
		q += ` WHERE site_url = $1`
		args = append(args, only)
	} else {
		q += ` WHERE site_url IN (SELECT site_url FROM stock_symbols WHERE source = 'crom' AND enabled)`
	}
	q += ` ORDER BY site_url`

	rows, err := e.db.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CromSite
	for rows.Next() {
		var s CromSite
		if err := rows.Scan(&s.SiteURL, &s.Display, &s.SweepCursor); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

func (e *Engine) SyncCromRecent(ctx context.Context, only string, pages int) ([]CromStats, error) {
	if pages <= 0 {
		pages = 600
	}
	out, err := e.syncCrom(ctx, only, crom.ByNewest, pages)
	if err != nil {
		return out, err
	}
	if _, err := e.FillCromTitles(ctx, only, 0); err != nil {
		slog.Warn("fill crom titles", "err", err)
	}
	return out, nil
}

func (e *Engine) SyncCromFull(ctx context.Context, site string) ([]CromStats, error) {
	return e.syncCrom(ctx, site, crom.ByURL, 0)
}

func (e *Engine) syncCrom(ctx context.Context, only string, order crom.SweepOrder, limit int) ([]CromStats, error) {
	if e.crom == nil {
		return nil, fmt.Errorf("crom client is not set up")
	}
	sites, err := e.cromSites(ctx, only)
	if err != nil {
		return nil, err
	}
	if len(sites) == 0 {
		return nil, nil
	}

	var out []CromStats
	for _, s := range sites {
		st, err := e.syncCromSite(ctx, s, order, limit)
		if err != nil {
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			slog.Warn("crom sync", "site", s.SiteURL, "err", err)
			continue
		}
		out = append(out, st)
		slog.Info("crom synced",
			"site", st.Site, "swept", st.Swept, "dirty", st.Dirty, "detailed", st.Detailed,
			"gone", st.Gone, "took", st.Duration.Round(time.Second), "quota_left", st.BudgetLeft)
	}
	return out, nil
}

func (e *Engine) syncCromSite(ctx context.Context, site CromSite, order crom.SweepOrder, limit int) (CromStats, error) {
	start := time.Now()
	st := CromStats{Site: site.SiteURL}

	cursor := ""
	if order == crom.ByURL {
		cursor = site.SweepCursor
	}
	for {
		res, err := e.crom.Sweep(ctx, site.SiteURL, cursor, order, crom.SweepPageSize)
		if err != nil {
			return st, err
		}
		if len(res.Pages) > 0 {
			if err := e.upsertCromPages(ctx, site.SiteURL, res.Pages); err != nil {
				return st, err
			}
			st.Swept += len(res.Pages)
		}
		cursor = res.NextCursor

		if order == crom.ByURL {
			next := cursor
			if !res.HasNext {
				next = ""
			}
			if _, err := e.db.Exec(ctx, `
				UPDATE crom_sites SET sweep_cursor = NULLIF($2, ''), swept_at = now(), updated_at = now()
				WHERE site_url = $1`, site.SiteURL, next); err != nil {
				return st, err
			}
		}

		if !res.HasNext {
			st.SweepDone = true
			break
		}
		if limit > 0 && st.Swept >= limit {
			break
		}
	}

	if err := e.detailDirtyPages(ctx, site.SiteURL, &st); err != nil {
		return st, err
	}
	if err := e.refreshCromSite(ctx, site.SiteURL); err != nil {
		return st, err
	}

	_ = e.db.QueryRow(ctx,
		`SELECT last_vote_at FROM crom_sites WHERE site_url = $1`, site.SiteURL).Scan(&st.LastVoteDay)
	st.BudgetLeft, _, _, _ = e.crom.Budget()
	st.Duration = time.Since(start)
	return st, nil
}

func (e *Engine) upsertCromPages(ctx context.Context, siteURL string, pages []crom.Page) error {
	n := len(pages)
	urls := make([]string, n)
	cats := make([]string, n)
	created := make([]*time.Time, n)
	revs := make([]int32, n)
	votes := make([]int32, n)
	ratings := make([]int32, n)
	comments := make([]int32, n)
	private := make([]bool, n)
	fps := make([]string, n)

	for i, p := range pages {
		urls[i] = p.URL
		cats[i] = p.Category
		if !p.CreatedAt.IsZero() {
			t := p.CreatedAt
			created[i] = &t
		}
		revs[i] = int32(p.RevCount)
		votes[i] = int32(p.VoteCount)
		ratings[i] = int32(p.Rating)
		comments[i] = int32(p.Comments)
		private[i] = p.IsPrivate
		fps[i] = p.Fingerprint()
	}

	_, err := e.db.Exec(ctx, `
		INSERT INTO crom_pages (url, site_url, category, created_at, rev_count, vote_count,
		                        rating, comments, is_private, fp, seen_at)
		SELECT u, $1, c, ca, r, v, ra, co, pr, f, now()
		FROM unnest($2::text[], $3::text[], $4::timestamptz[], $5::int[], $6::int[],
		            $7::int[], $8::int[], $9::boolean[], $10::text[])
		     AS t(u, c, ca, r, v, ra, co, pr, f)
		ON CONFLICT (url) DO UPDATE SET
			site_url   = EXCLUDED.site_url,
			category   = EXCLUDED.category,
			created_at = EXCLUDED.created_at,
			rev_count  = EXCLUDED.rev_count,
			vote_count = EXCLUDED.vote_count,
			rating     = EXCLUDED.rating,
			comments   = EXCLUDED.comments,
			is_private = EXCLUDED.is_private,
			fp         = EXCLUDED.fp,
			seen_at    = now()`,
		siteURL, urls, cats, created, revs, votes, ratings, comments, private, fps)
	return err
}

func (e *Engine) detailDirtyPages(ctx context.Context, siteURL string, st *CromStats) error {
	for {
		rows, err := e.db.Query(ctx, `
			SELECT url FROM crom_pages
			WHERE site_url = $1 AND (detail_fp IS NULL OR detail_fp <> fp)
			ORDER BY created_at DESC NULLS LAST
			LIMIT $2`, siteURL, crom.DetailBatchSize)
		if err != nil {
			return err
		}
		var urls []string
		for rows.Next() {
			var u string
			if err := rows.Scan(&u); err != nil {
				rows.Close()
				return err
			}
			urls = append(urls, u)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		if len(urls) == 0 {
			return nil
		}
		st.Dirty += len(urls)

		details, err := e.crom.Details(ctx, urls)
		if err != nil {
			return err
		}
		if err := e.applyCromDetails(ctx, siteURL, details, st); err != nil {
			return err
		}
	}
}

func (e *Engine) applyCromDetails(ctx context.Context, siteURL string, details []crom.Detail, st *CromStats) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -cromRetentionDays).Truncate(24 * time.Hour)

	var (
		gone []string

		pages []crom.Page

		dpURL   []string
		dpDay   []time.Time
		dpEdit  []int32
		dpDelta []int32

		vDay []time.Time
		vUID []int64
	)
	seenVoter := map[[2]int64]bool{}

	for _, d := range details {
		if !d.Found {
			gone = append(gone, d.URL)
			continue
		}
		pages = append(pages, d.Page)

		type agg struct{ edits, delta int32 }
		byDay := map[time.Time]*agg{}
		at := func(day time.Time) *agg {
			a := byDay[day]
			if a == nil {
				a = &agg{}
				byDay[day] = a
			}
			return a
		}

		for _, r := range d.Revisions {
			day := r.At.UTC().Truncate(24 * time.Hour)
			if day.Before(cutoff) {
				continue
			}
			at(day).edits++
		}

		votes := append([]crom.Vote(nil), d.Votes...)
		sort.SliceStable(votes, func(i, j int) bool { return votes[i].Day.Before(votes[j].Day) })
		prev := map[int64]int{}
		for _, v := range votes {
			delta := v.Direction - prev[v.UID]
			prev[v.UID] = v.Direction
			if v.Day.Before(cutoff) {
				continue
			}
			at(v.Day).delta += int32(delta)

			key := [2]int64{v.Day.Unix(), v.UID}
			if !seenVoter[key] {
				seenVoter[key] = true
				vDay = append(vDay, v.Day)
				vUID = append(vUID, v.UID)
			}
		}

		for day, a := range byDay {
			dpURL = append(dpURL, d.URL)
			dpDay = append(dpDay, day)
			dpEdit = append(dpEdit, a.edits)
			dpDelta = append(dpDelta, a.delta)
		}
	}

	st.Detailed += len(pages)
	st.Gone += len(gone)
	st.DayRows += len(dpURL)
	st.VoterRows += len(vDay)

	touched := make([]string, 0, len(details))
	for _, d := range details {
		if d.Found {
			touched = append(touched, d.URL)
		}
	}

	return e.db.InTx(ctx, func(tx pgx.Tx) error {

		if len(gone) > 0 {
			if _, err := tx.Exec(ctx, `DELETE FROM crom_pages WHERE url = ANY($1)`, gone); err != nil {
				return err
			}
		}
		if len(pages) == 0 {
			return nil
		}

		n := len(pages)
		urls := make([]string, n)
		cats := make([]string, n)
		created := make([]*time.Time, n)
		revs := make([]int32, n)
		vcnt := make([]int32, n)
		ratings := make([]int32, n)
		comments := make([]int32, n)
		private := make([]bool, n)
		fps := make([]string, n)
		for i, p := range pages {
			urls[i] = p.URL
			cats[i] = p.Category
			if !p.CreatedAt.IsZero() {
				t := p.CreatedAt
				created[i] = &t
			}
			revs[i] = int32(p.RevCount)
			vcnt[i] = int32(p.VoteCount)
			ratings[i] = int32(p.Rating)
			comments[i] = int32(p.Comments)
			private[i] = p.IsPrivate
			fps[i] = p.Fingerprint()
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO crom_pages (url, site_url, category, created_at, rev_count, vote_count,
			                        rating, comments, is_private, fp, detail_fp, detail_at, seen_at)
			SELECT u, $1, c, ca, r, v, ra, co, pr, f, f, now(), now()
			FROM unnest($2::text[], $3::text[], $4::timestamptz[], $5::int[], $6::int[],
			            $7::int[], $8::int[], $9::boolean[], $10::text[])
			     AS t(u, c, ca, r, v, ra, co, pr, f)
			ON CONFLICT (url) DO UPDATE SET
				site_url   = EXCLUDED.site_url,
				category   = EXCLUDED.category,
				created_at = EXCLUDED.created_at,
				rev_count  = EXCLUDED.rev_count,
				vote_count = EXCLUDED.vote_count,
				rating     = EXCLUDED.rating,
				comments   = EXCLUDED.comments,
				is_private = EXCLUDED.is_private,
				fp         = EXCLUDED.fp,
				detail_fp  = EXCLUDED.fp,
				detail_at  = now(),
				seen_at    = now()`,
			siteURL, urls, cats, created, revs, vcnt, ratings, comments, private, fps); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx,
			`DELETE FROM crom_day_page WHERE url = ANY($1)`, touched); err != nil {
			return err
		}
		if len(dpURL) > 0 {
			if _, err := tx.Exec(ctx, `
				INSERT INTO crom_day_page (url, site_url, day, edits, rating_delta)
				SELECT u, $1, d::date, e, r
				FROM unnest($2::text[], $3::timestamptz[], $4::int[], $5::int[]) AS t(u, d, e, r)`,
				siteURL, dpURL, dpDay, dpEdit, dpDelta); err != nil {
				return err
			}
		}
		if len(vDay) > 0 {

			if _, err := tx.Exec(ctx, `
				INSERT INTO crom_day_voter (site_url, day, uid)
				SELECT $1, d::date, u
				FROM unnest($2::timestamptz[], $3::bigint[]) AS t(d, u)
				ON CONFLICT DO NOTHING`,
				siteURL, vDay, vUID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (e *Engine) refreshCromSite(ctx context.Context, siteURL string) error {
	_, err := e.db.Exec(ctx, `
		UPDATE crom_sites s SET
			page_count   = (SELECT COUNT(*) FROM crom_content_pages WHERE site_url = s.site_url),
			last_page_at = (SELECT MAX(created_at) FROM crom_content_pages WHERE site_url = s.site_url),
			last_vote_at = (SELECT MAX(day)::timestamptz FROM crom_day_voter WHERE site_url = s.site_url),
			last_rev_at  = (SELECT MAX(day)::timestamptz FROM crom_day_page
			                 WHERE site_url = s.site_url AND edits > 0),
			detailed_at  = now(),
			updated_at   = now()
		WHERE site_url = $1`, siteURL)
	return err
}

func (e *Engine) PruneCrom(ctx context.Context) error {
	cutoff := time.Now().UTC().AddDate(0, 0, -cromRetentionDays)
	tag, err := e.db.Exec(ctx, `DELETE FROM crom_day_page WHERE day < $1::date`, cutoff)
	if err != nil {
		return err
	}
	tag2, err := e.db.Exec(ctx, `DELETE FROM crom_day_voter WHERE day < $1::date`, cutoff)
	if err != nil {
		return err
	}
	if n := tag.RowsAffected() + tag2.RowsAffected(); n > 0 {
		slog.Info("crom pruned", "rows", n, "keep_days", cromRetentionDays)
	}
	return nil
}

func (e *Engine) BackfillCrom(ctx context.Context, siteURL string) (CromStats, error) {
	if e.crom == nil {
		return CromStats{}, fmt.Errorf("crom client is not set up")
	}
	start := time.Now()
	st := CromStats{Site: siteURL}

	var cursor string
	if err := e.db.QueryRow(ctx,
		`SELECT COALESCE(sweep_cursor, '') FROM crom_sites WHERE site_url = $1`,
		siteURL).Scan(&cursor); err != nil {
		return st, fmt.Errorf("site %s is not in crom_sites: %w", siteURL, err)
	}

	for {
		details, next, hasNext, err := e.crom.SweepHeavy(ctx, siteURL, cursor, crom.SweepPageSize)
		if err != nil {
			return st, err
		}
		if len(details) > 0 {
			if err := e.applyCromDetails(ctx, siteURL, details, &st); err != nil {
				return st, err
			}
			st.Swept += len(details)
		}
		cursor = next

		save := cursor
		if !hasNext {
			save = ""
		}
		if _, err := e.db.Exec(ctx, `
			UPDATE crom_sites SET sweep_cursor = NULLIF($2, ''), swept_at = now(), updated_at = now()
			WHERE site_url = $1`, siteURL, save); err != nil {
			return st, err
		}

		if st.Swept%2000 == 0 && st.Swept > 0 {
			left, _, _, _ := e.crom.Budget()
			slog.Info("crom backfill", "site", siteURL, "swept", st.Swept, "quota_left", left)
		}
		if !hasNext {
			st.SweepDone = true
			break
		}
	}

	if err := e.refreshCromSite(ctx, siteURL); err != nil {
		return st, err
	}
	st.BudgetLeft, _, _, _ = e.crom.Budget()
	st.Duration = time.Since(start)
	slog.Info("crom backfill done",
		"site", siteURL, "pages", st.Swept, "day_rows", st.DayRows,
		"took", st.Duration.Round(time.Second), "quota_left", st.BudgetLeft)
	return st, nil
}
