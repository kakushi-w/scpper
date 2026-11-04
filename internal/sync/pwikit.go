package sync

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Source struct {
	pool *pgxpool.Pool
	slug string
}

func OpenSource(ctx context.Context, url, slug string) (*Source, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect to pwikit: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("reach pwikit: %w", err)
	}
	return &Source{pool: pool, slug: slug}, nil
}

func (s *Source) Close() { s.pool.Close() }

func (s *Source) siteID(ctx context.Context) (int64, error) {
	var id int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM web_site WHERE slug = $1`, s.slug).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("pwikit has no site %q", s.slug)
	}
	return id, err
}

const batchSize = 2000

type PwikitReport struct {
	Users, Pages, Deleted, Revisions, Votes, Posts int
}

func (e *Engine) SyncPwikit(ctx context.Context, src *Source) (PwikitReport, error) {
	var rep PwikitReport
	site, err := src.siteID(ctx)
	if err != nil {
		return rep, err
	}
	if rep.Users, err = e.syncUsers(ctx, src); err != nil {
		return rep, fmt.Errorf("users: %w", err)
	}
	if rep.Pages, rep.Deleted, err = e.syncPages(ctx, src, site); err != nil {
		return rep, fmt.Errorf("pages: %w", err)
	}
	ids, err := e.pageIDs(ctx, src.slug)
	if err != nil {
		return rep, err
	}
	if rep.Revisions, err = e.syncRevisions(ctx, src, site, ids); err != nil {
		return rep, fmt.Errorf("revisions: %w", err)
	}
	if rep.Votes, err = e.syncVotes(ctx, src, site, ids); err != nil {
		return rep, fmt.Errorf("votes: %w", err)
	}
	if rep.Posts, err = e.syncPosts(ctx, src, site); err != nil {
		return rep, fmt.Errorf("posts: %w", err)
	}
	return rep, nil
}

// Everyone on the instance has a place on the wealth ranking, so accounts are
// made here rather than on a reader's first visit.
func (e *Engine) syncUsers(ctx context.Context, src *Source) (int, error) {
	rows, err := src.pool.Query(ctx, `
		SELECT id, username::text FROM web_user WHERE type = 'normal'`)
	if err != nil {
		return 0, err
	}
	var ids []int64
	var names []string
	for rows.Next() {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
		names = append(names, name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	tag, err := e.db.Exec(ctx, `
		INSERT INTO users (pwikit_id, name)
		SELECT * FROM unnest($1::bigint[], $2::text[])
		ON CONFLICT (pwikit_id) DO UPDATE SET name = EXCLUDED.name
		WHERE users.name IS DISTINCT FROM EXCLUDED.name`, ids, names)
	if err != nil {
		return 0, err
	}
	return int(tag.RowsAffected()), nil
}

const pagesSQL = `
SELECT a.id,
       CASE WHEN a.category::text = '_default' THEN a.name::text ELSE a.category::text || ':' || a.name::text END,
       a.title, a.created_at, a.updated_at,
       coalesce(v.total, 0), coalesce(v.up, 0), coalesce(v.down, 0), coalesce(v.n, 0),
       coalesce(c.n, 0),
       fa.user_id, fu.username::text,
       coalesce(t.tags, '{}'),
       coalesce(r.n, 0), coalesce(r.editors, 0),
       coalesce(au.ids, '{}'), coalesce(au.names, '{}')
FROM web_article a
LEFT JOIN LATERAL (
	SELECT sum(rate) AS total, count(*) FILTER (WHERE rate > 0) AS up,
	       count(*) FILTER (WHERE rate < 0) AS down, count(*) AS n
	FROM web_vote WHERE article_id = a.id) v ON true
LEFT JOIN LATERAL (
	SELECT count(*) AS n FROM web_forumpost p JOIN web_forumthread th ON th.id = p.thread_id
	WHERE th.article_id = a.id) c ON true
LEFT JOIN LATERAL (
	SELECT user_id FROM web_article_authors WHERE article_id = a.id ORDER BY id LIMIT 1) fa ON true
LEFT JOIN web_user fu ON fu.id = fa.user_id
LEFT JOIN LATERAL (
	SELECT array_agg(aa.user_id ORDER BY aa.id) AS ids, array_agg(u.username::text ORDER BY aa.id) AS names
	FROM web_article_authors aa JOIN web_user u ON u.id = aa.user_id
	WHERE aa.article_id = a.id) au ON true
LEFT JOIN LATERAL (
	SELECT array_agg(CASE WHEN tc.slug = '_default' THEN tg.name ELSE tc.slug || ':' || tg.name END ORDER BY tg.name) AS tags
	FROM web_article_tags at
	JOIN web_tag tg ON tg.id = at.tag_id
	JOIN web_tagscategory tc ON tc.id = tg.category_id
	WHERE at.article_id = a.id) t ON true
LEFT JOIN LATERAL (
	SELECT count(*) AS n, count(DISTINCT user_id) AS editors
	FROM web_articlelogentry WHERE article_id = a.id) r ON true
WHERE a.site_id = $1`

const upsertPageSQL = `
INSERT INTO articles (wiki, page, page_id, title, url, rating, upvotes, downvotes, rating_votes, comments,
                      author, author_id, tags, created_at, lastmod, status, deleted_at,
                      revision_count, editor_count, author_ids, author_names, synced_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, 'active', NULL, $16, $17, $18, $19, now())
ON CONFLICT (wiki, page_id) WHERE page_id IS NOT NULL DO UPDATE SET
	page = EXCLUDED.page, title = EXCLUDED.title, url = EXCLUDED.url,
	rating = EXCLUDED.rating, upvotes = EXCLUDED.upvotes, downvotes = EXCLUDED.downvotes,
	rating_votes = EXCLUDED.rating_votes, comments = EXCLUDED.comments,
	author = EXCLUDED.author, author_id = EXCLUDED.author_id, tags = EXCLUDED.tags,
	created_at = EXCLUDED.created_at, lastmod = EXCLUDED.lastmod,
	status = 'active', deleted_at = NULL,
	revision_count = EXCLUDED.revision_count, editor_count = EXCLUDED.editor_count,
	author_ids = EXCLUDED.author_ids, author_names = EXCLUDED.author_names,
	synced_at = now()`

func (e *Engine) syncPages(ctx context.Context, src *Source, site int64) (int, int, error) {
	rows, err := src.pool.Query(ctx, pagesSQL, site)
	if err != nil {
		return 0, 0, err
	}
	type page struct {
		id                        int64
		name, title               string
		created, updated          time.Time
		rating                    float64
		up, down, votes, comments int
		authorID                  *int64
		author                    *string
		tags                      []string
		revisions, editors        int
		authorIDs                 []int64
		authorNames               []string
	}
	var pages []page
	for rows.Next() {
		var p page
		if err := rows.Scan(&p.id, &p.name, &p.title, &p.created, &p.updated,
			&p.rating, &p.up, &p.down, &p.votes, &p.comments,
			&p.authorID, &p.author, &p.tags, &p.revisions, &p.editors, &p.authorIDs, &p.authorNames); err != nil {
			rows.Close()
			return 0, 0, err
		}
		pages = append(pages, p)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, 0, err
	}

	live := make([]int64, len(pages))
	for i, p := range pages {
		live[i] = p.id
	}
	tag, err := e.db.Exec(ctx, `
		UPDATE articles SET status = 'deleted', deleted_at = now()
		WHERE wiki = $1 AND status <> 'deleted' AND NOT (page_id = ANY($2))`, src.slug, live)
	if err != nil {
		return 0, 0, err
	}

	for start := 0; start < len(pages); start += batchSize {
		end := min(start+batchSize, len(pages))
		batch := &pgx.Batch{}
		for _, p := range pages[start:end] {
			title := p.title
			if title == "" {
				title = p.name
			}
			batch.Queue(upsertPageSQL, src.slug, p.name, p.id, title, "/"+p.name,
				p.rating, p.up, p.down, p.votes, p.comments, p.author, p.authorID, p.tags,
				p.created, p.updated, p.revisions, p.editors, p.authorIDs, p.authorNames)
		}
		if err := e.db.SendBatch(ctx, batch).Close(); err != nil {
			return 0, 0, err
		}
	}
	return len(pages), int(tag.RowsAffected()), nil
}

func (e *Engine) pageIDs(ctx context.Context, slug string) (map[int64]int64, error) {
	rows, err := e.db.Query(ctx, `SELECT page_id, id FROM articles WHERE wiki = $1 AND page_id IS NOT NULL`, slug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64]int64{}
	for rows.Next() {
		var pw, id int64
		if err := rows.Scan(&pw, &id); err != nil {
			return nil, err
		}
		out[pw] = id
	}
	return out, rows.Err()
}

func (e *Engine) cursor(ctx context.Context, key string) int64 {
	var raw []byte
	if err := e.db.QueryRow(ctx, `SELECT value FROM sync_state WHERE key = $1`, key).Scan(&raw); err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(string(raw), 10, 64)
	return n
}

func (e *Engine) setCursor(ctx context.Context, key string, n int64) error {
	_, err := e.db.Exec(ctx, `
		INSERT INTO sync_state (key, value, updated_at) VALUES ($1, to_jsonb($2::bigint), now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, key, n)
	return err
}

func (e *Engine) syncRevisions(ctx context.Context, src *Source, site int64, ids map[int64]int64) (int, error) {
	const key = "pwikit.revisions"
	total := 0
	for {
		after := e.cursor(ctx, key)
		rows, err := src.pool.Query(ctx, `
			SELECT l.id, l.article_id, l.rev_number, l.user_id, u.username::text, l.created_at, l.type, l.comment
			FROM web_articlelogentry l
			JOIN web_article a ON a.id = l.article_id
			LEFT JOIN web_user u ON u.id = l.user_id
			WHERE a.site_id = $1 AND l.id > $2
			ORDER BY l.id LIMIT $3`, site, after, batchSize)
		if err != nil {
			return total, err
		}
		batch := &pgx.Batch{}
		last, n := after, 0
		for rows.Next() {
			var id, article int64
			var rev int
			var user *int64
			var name *string
			var at time.Time
			var kind, comment string
			if err := rows.Scan(&id, &article, &rev, &user, &name, &at, &kind, &comment); err != nil {
				rows.Close()
				return total, err
			}
			last, n = id, n+1
			mirror, ok := ids[article]
			if !ok {
				continue
			}
			batch.Queue(`
				INSERT INTO revisions (article_id, rev_no, rev_id, flags, uid, username, comment, changed_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
				ON CONFLICT (article_id, rev_no) DO UPDATE SET
					rev_id = EXCLUDED.rev_id, flags = EXCLUDED.flags, uid = EXCLUDED.uid,
					username = EXCLUDED.username, comment = EXCLUDED.comment, changed_at = EXCLUDED.changed_at`,
				mirror, rev, id, kind, uidOf(user), name, comment, at)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return total, err
		}
		if n == 0 {
			return total, nil
		}
		if err := e.db.SendBatch(ctx, batch).Close(); err != nil {
			return total, err
		}
		if err := e.setCursor(ctx, key, last); err != nil {
			return total, err
		}
		total += n
	}
}

func (e *Engine) syncVotes(ctx context.Context, src *Source, site int64, ids map[int64]int64) (int, error) {
	const key = "pwikit.votes"
	total := 0
	for {
		after := e.cursor(ctx, key)
		rows, err := src.pool.Query(ctx, `
			SELECT ev.id, ev.article_id, ev.user_id, u.username::text, ev.old_rate, ev.new_rate, ev.at
			FROM pwikit_vote_event ev
			JOIN web_article a ON a.id = ev.article_id
			LEFT JOIN web_user u ON u.id = ev.user_id
			WHERE a.site_id = $1 AND ev.id > $2
			ORDER BY ev.id LIMIT $3`, site, after, batchSize)
		if err != nil {
			return total, err
		}
		batch := &pgx.Batch{}
		last, n := after, 0
		for rows.Next() {
			var id, article, user int64
			var name *string
			var before, now *float64
			var at time.Time
			if err := rows.Scan(&id, &article, &user, &name, &before, &now, &at); err != nil {
				rows.Close()
				return total, err
			}
			last, n = id, n+1
			mirror, ok := ids[article]
			if !ok {
				continue
			}
			kind := "vote"
			if now == nil {
				kind = "cancel"
			}
			batch.Queue(`
				INSERT INTO vote_events (article_id, uid, username, old_vote, new_vote, event_type, voted_at)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (article_id, uid, voted_at, new_vote) DO NOTHING`,
				mirror, strconv.FormatInt(user, 10), name, rounded(before), rounded(now), kind, at)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return total, err
		}
		if n == 0 {
			return total, nil
		}
		if err := e.db.SendBatch(ctx, batch).Close(); err != nil {
			return total, err
		}
		if err := e.setCursor(ctx, key, last); err != nil {
			return total, err
		}
		total += n
	}
}

func (e *Engine) syncPosts(ctx context.Context, src *Source, site int64) (int, error) {
	const key = "pwikit.posts"
	total := 0
	for {
		after := e.cursor(ctx, key)
		rows, err := src.pool.Query(ctx, `
			SELECT p.id, p.thread_id, p.reply_to_id, p.name, p.author_id, p.created_at, t.article_id
			FROM web_forumpost p
			JOIN web_forumthread t ON t.id = p.thread_id
			WHERE t.site_id = $1 AND p.id > $2
			ORDER BY p.id LIMIT $3`, site, after, batchSize)
		if err != nil {
			return total, err
		}
		batch := &pgx.Batch{}
		last, n := after, 0
		for rows.Next() {
			var id, thread int64
			var parent, author, page *int64
			var title string
			var at time.Time
			if err := rows.Scan(&id, &thread, &parent, &title, &author, &at, &page); err != nil {
				rows.Close()
				return total, err
			}
			last, n = id, n+1
			batch.Queue(`
				INSERT INTO forum_posts (id, thread_id, parent_id, title, author_uid, created_at, page_id)
				VALUES ($1, $2, $3, $4, $5, $6, $7)
				ON CONFLICT (id) DO UPDATE SET page_id = EXCLUDED.page_id`,
				id, thread, parent, title, uidOf(author), at, page)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return total, err
		}
		if n == 0 {
			return total, nil
		}
		if err := e.db.SendBatch(ctx, batch).Close(); err != nil {
			return total, err
		}
		if err := e.setCursor(ctx, key, last); err != nil {
			return total, err
		}
		total += n
	}
}

func (e *Engine) RunPwikit(ctx context.Context, src *Source, every time.Duration) {
	for {
		started := time.Now()
		rep, err := e.SyncPwikit(ctx, src)
		if err != nil {
			slog.Error("sync pwikit", "err", err)
		} else {
			slog.Info("sync pwikit", "users", rep.Users, "pages", rep.Pages, "deleted", rep.Deleted, "revisions", rep.Revisions,
				"votes", rep.Votes, "posts", rep.Posts, "took", time.Since(started).Round(time.Millisecond))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

func uidOf(id *int64) string {
	if id == nil {
		return ""
	}
	return strconv.FormatInt(*id, 10)
}

func rounded(v *float64) int {
	if v == nil {
		return 0
	}
	return int(math.Round(*v))
}
