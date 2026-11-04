package sync

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/scpwikicloud/scpper/internal/stockcfg"
)

const cromStaleAfterDays = 5

type StockParams = stockcfg.Params

type Symbol = stockcfg.Symbol

func (e *Engine) Symbol(ctx context.Context, code string) (Symbol, error) {
	return stockcfg.Load(ctx, e.db, code)
}

func (e *Engine) ListedSymbols(ctx context.Context) ([]Symbol, error) {
	return stockcfg.LoadAll(ctx, e.db, true)
}

func (e *Engine) StockFairPriceAt(ctx context.Context, sym Symbol, at time.Time) (float64, error) {
	if sym.IsCrom() {
		return e.cromFairPriceAt(ctx, sym, at)
	}
	return e.localFairPriceAt(ctx, sym, at)
}

func (e *Engine) StockFairPrice(ctx context.Context, sym Symbol) (float64, error) {
	return e.StockFairPriceAt(ctx, sym, time.Now())
}

func (e *Engine) localFairPriceAt(ctx context.Context, sym Symbol, at time.Time) (float64, error) {
	var a float64
	err := e.db.QueryRow(ctx, `
		WITH s AS (
			SELECT (SELECT MIN(voted_at) FROM real_vote_events) AS vs
		), k AS (
			SELECT vs, vs IS NOT NULL AND vs <= $1::timestamptz - interval '7 days' AS known,
			       GREATEST(EXTRACT(EPOCH FROM ($1::timestamptz - GREATEST(vs, $1::timestamptz - interval '90 days'))) / 86400, 7) AS days
			FROM s
		), w AS (
			SELECT
			  (SELECT COUNT(*) FROM articles
			    WHERE status <> 'deleted'
			      AND created_at BETWEEN $1::timestamptz - interval '7 days' AND $1::timestamptz)::float     AS pages7,
			  (SELECT COALESCE(SUM(new_vote - old_vote), 0) FROM real_vote_events
			    WHERE voted_at BETWEEN $1::timestamptz - interval '7 days' AND $1::timestamptz)::float       AS score7,
			  (SELECT COUNT(DISTINCT uid) FROM real_vote_events
			    WHERE voted_at BETWEEN $1::timestamptz - interval '7 days' AND $1::timestamptz AND uid <> '')::float AS voters7,
			  (SELECT COUNT(*) FROM revisions
			    WHERE changed_at BETWEEN $1::timestamptz - interval '7 days' AND $1::timestamptz)::float     AS edits7,
			  (SELECT COUNT(*) FROM forum_posts
			    WHERE created_at BETWEEN $1::timestamptz - interval '7 days' AND $1::timestamptz)::float     AS posts7,
			  (SELECT COUNT(DISTINCT author_uid) FROM forum_posts
			    WHERE created_at BETWEEN $1::timestamptz - interval '7 days' AND $1::timestamptz AND author_uid <> '')::float AS posters7,

			  (SELECT COUNT(*) FROM articles
			    WHERE status <> 'deleted'
			      AND created_at BETWEEN $1::timestamptz - interval '90 days' AND $1::timestamptz)::float / 12.857 AS pages_avg,
			  GREATEST((SELECT COALESCE(ABS(SUM(new_vote - old_vote)), 0) FROM real_vote_events
			    WHERE voted_at BETWEEN $1::timestamptz - interval '90 days' AND $1::timestamptz)::float
			    / ((SELECT days FROM k) / 7), 1)                                                            AS score_avg,
			  GREATEST((SELECT COUNT(DISTINCT uid) FROM real_vote_events
			    WHERE voted_at BETWEEN $1::timestamptz - interval '90 days' AND $1::timestamptz AND uid <> '')::float
			    / (6 * (SELECT days FROM k) / 90), 1)                                                       AS voters_avg,
			  GREATEST((SELECT COUNT(*) FROM revisions
			    WHERE changed_at BETWEEN $1::timestamptz - interval '90 days' AND $1::timestamptz)::float / 12.857, 1) AS edits_avg,
			  GREATEST((SELECT COUNT(*) FROM forum_posts
			    WHERE created_at BETWEEN $1::timestamptz - interval '90 days' AND $1::timestamptz)::float / 12.857, 1) AS posts_avg,
			  GREATEST((SELECT COUNT(DISTINCT author_uid) FROM forum_posts
			    WHERE created_at BETWEEN $1::timestamptz - interval '90 days' AND $1::timestamptz AND author_uid <> '')::float / 6, 1) AS posters_avg
		)
		SELECT
		  CASE WHEN (SELECT known FROM k) THEN
		         0.30 * LEAST((pages7 / GREATEST(pages_avg, 1)), 5)
		       + 0.20 * LEAST((ABS(score7) / score_avg), 5)
		       + 0.20 * LEAST((voters7 / voters_avg), 5)
		       + 0.10 * LEAST((edits7 / edits_avg), 5)
		       + 0.10 * LEAST((posts7 / posts_avg), 5)
		       + 0.10 * LEAST((posters7 / posters_avg), 5)
		       ELSE
		         0.50 * LEAST((pages7 / GREATEST(pages_avg, 1)), 5)
		       + (1.0 / 6) * LEAST((edits7 / edits_avg), 5)
		       + (1.0 / 6) * LEAST((posts7 / posts_avg), 5)
		       + (1.0 / 6) * LEAST((posters7 / posters_avg), 5)
		  END
		FROM w`, at).Scan(&a)
	if err != nil {
		return 0, err
	}
	pages, err := e.localPageCount(ctx, at)
	if err != nil {
		return 0, err
	}
	return assemble(sym.Params, a, pages), nil
}

func (e *Engine) FirstPage(ctx context.Context) (time.Time, bool, error) {
	var at *time.Time
	err := e.db.QueryRow(ctx, `SELECT MIN(created_at) FROM articles WHERE status <> 'deleted'`).Scan(&at)
	if err != nil || at == nil {
		return time.Time{}, false, err
	}
	return *at, true, nil
}

func (e *Engine) localPageCount(ctx context.Context, at time.Time) (int64, error) {
	var n int64
	err := e.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM articles
		WHERE status <> 'deleted' AND created_at <= $1::timestamptz`, at).Scan(&n)
	return n, err
}

func (e *Engine) cromFairPriceAt(ctx context.Context, sym Symbol, at time.Time) (float64, error) {
	var a float64
	err := e.db.QueryRow(ctx, `
		WITH d AS (
			SELECT ($1::timestamptz AT TIME ZONE 'UTC')::date AS d0
		), w AS (
			SELECT
			  (SELECT COUNT(*) FROM crom_content_pages
			    WHERE site_url = $2
			      AND created_at BETWEEN $1::timestamptz - interval '7 days' AND $1::timestamptz)::float  AS pages7,
			  (SELECT COALESCE(SUM(p.rating_delta), 0) FROM crom_day_page p
			     JOIN crom_content_pages c ON c.url = p.url
			    WHERE p.site_url = $2 AND p.day BETWEEN (SELECT d0 FROM d) - 6 AND (SELECT d0 FROM d))::float AS score7,
			  (SELECT COUNT(DISTINCT uid) FROM crom_day_voter
			    WHERE site_url = $2 AND day BETWEEN (SELECT d0 FROM d) - 6 AND (SELECT d0 FROM d))::float  AS voters7,
			  (SELECT COALESCE(SUM(p.edits), 0) FROM crom_day_page p
			     JOIN crom_content_pages c ON c.url = p.url
			    WHERE p.site_url = $2 AND p.day BETWEEN (SELECT d0 FROM d) - 6 AND (SELECT d0 FROM d))::float AS edits7,

			  (SELECT COUNT(*) FROM crom_content_pages
			    WHERE site_url = $2
			      AND created_at BETWEEN $1::timestamptz - interval '90 days' AND $1::timestamptz)::float / 12.857 AS pages_avg,
			  GREATEST((SELECT COALESCE(ABS(SUM(p.rating_delta)), 0) FROM crom_day_page p
			     JOIN crom_content_pages c ON c.url = p.url
			    WHERE p.site_url = $2 AND p.day BETWEEN (SELECT d0 FROM d) - 89 AND (SELECT d0 FROM d))::float / 12.857, 1) AS score_avg,
			  GREATEST((SELECT COUNT(DISTINCT uid) FROM crom_day_voter
			    WHERE site_url = $2 AND day BETWEEN (SELECT d0 FROM d) - 89 AND (SELECT d0 FROM d))::float / 6, 1)          AS voters_avg,
			  EXISTS (SELECT 1 FROM crom_day_voter
			    WHERE site_url = $2 AND day BETWEEN (SELECT d0 FROM d) - 89 AND (SELECT d0 FROM d))                      AS voted,
			  GREATEST((SELECT COALESCE(SUM(p.edits), 0) FROM crom_day_page p
			     JOIN crom_content_pages c ON c.url = p.url
			    WHERE p.site_url = $2 AND p.day BETWEEN (SELECT d0 FROM d) - 89 AND (SELECT d0 FROM d))::float / 12.857, 1) AS edits_avg
		)
		SELECT CASE WHEN voted THEN
		         0.35 * LEAST((pages7  / GREATEST(pages_avg, 1)), 5)
		       + 0.25 * LEAST((ABS(score7) / score_avg), 5)
		       + 0.25 * LEAST((voters7 / voters_avg), 5)
		       + 0.15 * LEAST((edits7  / edits_avg), 5)
		       ELSE
		         0.70 * LEAST((pages7  / GREATEST(pages_avg, 1)), 5)
		       + 0.30 * LEAST((edits7  / edits_avg), 5)
		       END
		FROM w`, at, sym.SiteURL).Scan(&a)
	if err != nil {
		return 0, err
	}

	var pages int64
	if err := e.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM crom_content_pages
		WHERE site_url = $1 AND created_at <= $2::timestamptz`, sym.SiteURL, at).Scan(&pages); err != nil {
		return 0, err
	}
	return assemble(sym.Params, a, pages), nil
}

func assemble(p StockParams, a float64, pages int64) float64 {
	if a <= 0 || math.IsNaN(a) || math.IsInf(a, 0) {
		a = 1
	}
	if p.ScaleWeight > 0 && p.ScaleBase > 0 {
		g := float64(pages) / float64(p.ScaleBase)
		a = (1-p.ScaleWeight)*a + p.ScaleWeight*g
	}
	return p.BasePrice * math.Pow(a, 0.7)
}

func (e *Engine) CromFresh(ctx context.Context, siteURL string) (bool, time.Time, error) {
	var last *time.Time
	if err := e.db.QueryRow(ctx,
		`SELECT last_vote_at FROM crom_sites WHERE site_url = $1`, siteURL).Scan(&last); err != nil {
		return false, time.Time{}, err
	}
	if last == nil {
		return false, time.Time{}, nil
	}
	return time.Since(*last) < cromStaleAfterDays*24*time.Hour, *last, nil
}

func (e *Engine) StockTick(ctx context.Context) error {
	syms, err := e.ListedSymbols(ctx)
	if err != nil {
		return err
	}
	var firstErr error
	for _, sym := range syms {
		if err := e.StockTickSymbol(ctx, sym); err != nil {
			slog.Warn("stock tick", "symbol", sym.Code, "err", err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

func (e *Engine) StockTickSymbol(ctx context.Context, sym Symbol) error {
	if sym.IsCrom() {
		fresh, last, err := e.CromFresh(ctx, sym.SiteURL)
		if err != nil {
			return err
		}
		if !fresh {
			slog.Warn("crom data is stale, fair price paused",
				"symbol", sym.Code, "site", sym.SiteURL, "last_vote", last.Format("2006-01-02"))
			return nil
		}
	}

	fair, err := e.StockFairPrice(ctx, sym)
	if err != nil {
		return err
	}
	if fair < sym.Params.MinPrice {
		fair = sym.Params.MinPrice
	}
	if _, err := e.db.Exec(ctx,
		`UPDATE stock_maker SET p_fair = $2, updated_at = now() WHERE symbol = $1`,
		sym.Code, fair); err != nil {
		return err
	}
	slog.Info("fair price", "symbol", sym.Code, "fair", math.Round(fair*100)/100)
	return nil
}
