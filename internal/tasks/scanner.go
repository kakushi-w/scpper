// Package tasks works out how far each member is with the daily, weekly and monthly
// tasks, from what the pwikit mirror holds.
package tasks

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/scpwikicloud/scpper/internal/db"
)

type Scanner struct {
	db  *db.DB
	loc *time.Location
}

func NewScanner(database *db.DB, tz string) *Scanner {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.FixedZone("CST", 8*3600)
	}
	return &Scanner{db: database, loc: loc}
}

func (s *Scanner) Period(now time.Time, kind string) string {
	local := now.In(s.loc)
	switch kind {
	case "monthly":
		return local.Format("2006-01")
	case "weekly":
		year, week := local.ISOWeek()
		return fmt.Sprintf("%d-W%02d", year, week)
	}
	return local.Format("2006-01-02")
}

func (s *Scanner) bounds(now time.Time, kind string) (start, end time.Time) {
	local := now.In(s.loc)
	day := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, s.loc)
	switch kind {
	case "monthly":
		start = time.Date(local.Year(), local.Month(), 1, 0, 0, 0, 0, s.loc)
		return start.UTC(), start.AddDate(0, 1, 0).UTC()
	case "weekly":
		start = day.AddDate(0, 0, -((int(day.Weekday()) + 6) % 7))
		return start.UTC(), start.AddDate(0, 0, 7).UTC()
	}
	return day.UTC(), day.AddDate(0, 0, 1).UTC()
}

type task struct {
	code, kind, metric string
	tag, category      string
	score              *int
	target             int
}

type member struct {
	id       int64
	pwikitID int64
}

func (s *Scanner) ScanAll(ctx context.Context) (int, error) {
	return s.scan(ctx, `SELECT id, pwikit_id FROM users WHERE banned_at IS NULL`)
}

func (s *Scanner) ScanActive(ctx context.Context, within time.Duration) (int, error) {
	return s.scan(ctx, `SELECT id, pwikit_id FROM users
		WHERE banned_at IS NULL AND last_active_at > now() - $1::interval`, within.String())
}

func (s *Scanner) ScanUser(ctx context.Context, userID int64) error {
	_, err := s.scan(ctx, `SELECT id, pwikit_id FROM users WHERE id = $1`, userID)
	return err
}

func (s *Scanner) scan(ctx context.Context, query string, args ...any) (int, error) {
	tasks, err := s.tasks(ctx)
	if err != nil || len(tasks) == 0 {
		return 0, err
	}
	rows, err := s.db.Query(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	var members []member
	for rows.Next() {
		var m member
		if err := rows.Scan(&m.id, &m.pwikitID); err != nil {
			rows.Close()
			return 0, err
		}
		members = append(members, m)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}

	now := time.Now()
	scanned := 0
	for _, m := range members {
		if ctx.Err() != nil {
			return scanned, ctx.Err()
		}
		failed := false
		for _, t := range tasks {
			period := s.Period(now, t.kind)
			start, end := s.bounds(now, t.kind)
			n, err := s.count(ctx, t, m, start, end)
			if err == nil {
				err = s.upsert(ctx, m.id, t.code, period, n, t.target)
			}
			if err != nil {
				slog.Warn("scan task", "user", m.id, "task", t.code, "err", err)
				failed = true
				break
			}
		}
		if !failed {
			scanned++
		}
	}
	return scanned, nil
}

func (s *Scanner) tasks(ctx context.Context) ([]task, error) {
	rows, err := s.db.Query(ctx, `SELECT code, kind, metric, tag, category, score, target FROM task_defs WHERE enabled`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []task
	for rows.Next() {
		var t task
		if err := rows.Scan(&t.code, &t.kind, &t.metric, &t.tag, &t.category, &t.score, &t.target); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *Scanner) count(ctx context.Context, t task, m member, start, end time.Time) (int, error) {
	args := []any{start, end}
	arg := func(v any) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}
	uid := strconv.FormatInt(m.pwikitID, 10)
	tagged := func(alias string) string {
		if t.tag == "" {
			return ""
		}
		return " AND " + alias + ".tags && ARRAY[" + arg(t.tag) + "::text]"
	}
	filed := func(alias string) string {
		switch t.category {
		case "":
			return ""
		case "_default":
			return " AND position(':' in " + alias + ".page) = 0"
		}
		return " AND " + alias + ".page LIKE " + arg(t.category+":%")
	}
	authored := func(alias string) string {
		return alias + ".author_ids @> ARRAY[" + arg(m.pwikitID) + "::bigint] AND " + alias + ".status <> 'deleted'"
	}

	var q string
	switch t.metric {
	case "vote", "vote_up", "vote_down":
		cond := "e.new_vote <> 0"
		if t.metric == "vote_up" {
			cond = "e.new_vote > 0"
		} else if t.metric == "vote_down" {
			cond = "e.new_vote < 0"
		}
		q = `SELECT COUNT(*) FROM real_vote_events e JOIN articles a ON a.id = e.article_id
			WHERE e.uid = ` + arg(uid) + ` AND e.voted_at >= $1 AND e.voted_at < $2 AND ` + cond + tagged("a") + filed("a")
	case "post", "comment", "thread_post":
		where := ""
		if t.metric == "comment" {
			where = " AND p.page_id IS NOT NULL"
		} else if t.metric == "thread_post" {
			where = " AND p.page_id IS NULL"
		}
		q = `SELECT COUNT(*) FROM forum_posts p
			WHERE p.author_uid = ` + arg(uid) + ` AND p.created_at >= $1 AND p.created_at < $2` + where
	case "edit":
		q = `SELECT COUNT(*) FROM revisions r JOIN articles a ON a.id = r.article_id
			WHERE r.uid = ` + arg(uid) + ` AND r.changed_at >= $1 AND r.changed_at < $2` + tagged("a") + filed("a")
	case "page":
		q = `SELECT COUNT(*) FROM articles a WHERE ` + authored("a") +
			` AND a.created_at >= $1 AND a.created_at < $2` + tagged("a") + filed("a")
	case "page_score":
		if t.score == nil {
			return 0, nil
		}
		cmp := " AND a.rating >= " + arg(*t.score)
		if *t.score < 0 {
			cmp = " AND a.rating <= " + arg(*t.score)
		}
		q = `SELECT COUNT(*) FROM articles a WHERE ` + authored("a") +
			` AND a.created_at >= $1 AND a.created_at < $2` + cmp + tagged("a") + filed("a")
	case "comments_received":
		q = `SELECT COUNT(*) FROM forum_posts p JOIN articles a ON a.page_id = p.page_id
			WHERE ` + authored("a") + ` AND p.author_uid <> ` + arg(uid) +
			` AND p.created_at >= $1 AND p.created_at < $2` + tagged("a") + filed("a")
	case "score_received":
		q = `SELECT COALESCE(SUM(e.new_vote - e.old_vote), 0) FROM real_vote_events e JOIN articles a ON a.id = e.article_id
			WHERE ` + authored("a") + ` AND e.voted_at >= $1 AND e.voted_at < $2` + tagged("a") + filed("a")
	case "tip_given":
		q = `SELECT COUNT(*) FROM tips WHERE from_user = ` + arg(m.id) + ` AND created_at >= $1 AND created_at < $2`
	case "tip_received":
		q = `SELECT COALESCE(SUM(delta), 0) FROM coin_ledger
			WHERE user_id = ` + arg(m.id) + ` AND reason = 'tip_in' AND created_at >= $1 AND created_at < $2`
	case "gacha":
		q = `SELECT COUNT(*) FROM gacha_pulls WHERE user_id = ` + arg(m.id) + ` AND created_at >= $1 AND created_at < $2`
	case "trade":
		q = `SELECT COUNT(*) FROM stock_trades WHERE user_id = ` + arg(m.id) + ` AND at >= $1 AND at < $2`
	default:
		return 0, nil
	}
	var n int
	err := s.db.QueryRow(ctx, q, args...).Scan(&n)
	return n, err
}

func (s *Scanner) upsert(ctx context.Context, userID int64, code, period string, progress, target int) error {
	_, err := s.db.Exec(ctx, `
		INSERT INTO task_progress (user_id, task_code, period, progress, target, completed_at, updated_at)
		VALUES ($1, $2, $3, $4::int, $5::int,
		        CASE WHEN $4::int >= $5::int THEN now() ELSE NULL END, now())
		ON CONFLICT (user_id, task_code, period) DO UPDATE SET
			progress     = GREATEST(task_progress.progress, EXCLUDED.progress),
			target       = EXCLUDED.target,
			completed_at = COALESCE(
				task_progress.completed_at,
				CASE WHEN GREATEST(task_progress.progress, EXCLUDED.progress) >= EXCLUDED.target
				     THEN now() ELSE NULL END),
			updated_at   = now()`,
		userID, code, period, progress, target)
	return err
}
