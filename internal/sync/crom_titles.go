package sync

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

const cromTitleWindow = 30 * 24 * time.Hour

func (e *Engine) FillCromTitles(ctx context.Context, only string, limit int) (int, error) {
	if e.crom == nil {
		return 0, fmt.Errorf("crom client is not set up")
	}
	if limit <= 0 {
		limit = 400
	}
	rows, err := e.db.Query(ctx, `
		SELECT url FROM crom_pages
		WHERE created_at > now() - $1::interval
		  AND ($2 = '' OR site_url = $2)
		  AND (title_at IS NULL
		       OR (COALESCE(alt_title, '') = '' AND title_at < now() - interval '12 hours'))
		ORDER BY title_at NULLS FIRST, created_at DESC
		LIMIT $3`, cromTitleWindow.String(), only, limit)
	if err != nil {
		return 0, err
	}
	var urls []string
	for rows.Next() {
		var u string
		if err := rows.Scan(&u); err != nil {
			rows.Close()
			return 0, err
		}
		urls = append(urls, u)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	done := 0
	for len(urls) > 0 {
		batch := urls
		if len(batch) > 40 {
			batch = batch[:40]
		}
		urls = urls[len(batch):]

		titles, err := e.crom.Titles(ctx, batch)
		if err != nil {
			return done, err
		}
		for _, u := range batch {
			t := titles[u]
			if _, err := e.db.Exec(ctx, `
				UPDATE crom_pages
				SET title     = COALESCE(NULLIF($2, ''), title),
				    alt_title = COALESCE(NULLIF($3, ''), alt_title),
				    title_at  = now()
				WHERE url = $1`, u, t.Title, t.Alt); err != nil {
				return done, err
			}
			done++
		}
	}
	if done > 0 {
		slog.Info("crom titles filled", "pages", done)
	}
	return done, nil
}
