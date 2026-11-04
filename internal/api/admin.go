package api

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/httpx"
)

func (s *Server) handleAdminOverview(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	var ov struct {
		Users       int   `json:"users"`
		DailyActive int   `json:"dailyActive"`
		TotalCoins  int64 `json:"totalCoins"`
		Pages       int   `json:"pages"`
		Tips        int64 `json:"tips"`
	}
	err := s.db.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users WHERE banned_at IS NULL),
			(SELECT COUNT(*) FROM users WHERE banned_at IS NULL
			   AND last_active_at >= date_trunc('day', now() AT TIME ZONE INTERVAL '+08:00') AT TIME ZONE INTERVAL '+08:00'),
			(SELECT COALESCE(SUM(coins), 0) FROM users),
			(SELECT COUNT(*) FROM articles WHERE status <> 'deleted'),
			(SELECT COALESCE(SUM(amount), 0) FROM tips)`,
	).Scan(&ov.Users, &ov.DailyActive, &ov.TotalCoins, &ov.Pages, &ov.Tips)
	if err != nil {
		return err
	}
	httpx.OK(w, ov)
	return nil
}

func (s *Server) handleAdminUsers(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	pg := httpx.ParsePage(r, 30, 100)
	b := &argBuilder{}
	where := "1=1"
	if q := httpx.QueryString(r, "q", ""); q != "" {
		where += " AND name ILIKE " + b.add(likeContains(q)) + ` ESCAPE '\'`
	}
	var total int
	if err := s.db.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE `+where, b.all()...).Scan(&total); err != nil {
		return err
	}
	limit, offset := b.add(pg.Size), b.add(pg.Offset)
	rows, err := s.db.Query(ctx, `
		SELECT id, pwikit_id, name, role, coins, created_at, last_active_at, banned_at
		FROM users WHERE `+where+`
		ORDER BY created_at DESC
		LIMIT `+limit+` OFFSET `+offset, b.all()...)
	if err != nil {
		return err
	}
	defer rows.Close()

	type adminUser struct {
		ID           int64      `json:"id"`
		PwikitID     int64      `json:"pwikitId"`
		Name         string     `json:"name"`
		Role         string     `json:"role"`
		Coins        int64      `json:"coins"`
		CreatedAt    time.Time  `json:"createdAt"`
		LastActiveAt *time.Time `json:"lastActiveAt"`
		BannedAt     *time.Time `json:"bannedAt"`
	}
	items := []adminUser{}
	for rows.Next() {
		var u adminUser
		if err := rows.Scan(&u.ID, &u.PwikitID, &u.Name, &u.Role, &u.Coins, &u.CreatedAt, &u.LastActiveAt, &u.BannedAt); err != nil {
			return err
		}
		items = append(items, u)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"items": items, "meta": httpx.Meta(pg, total)})
	return nil
}

type grantCoinsRequest struct {
	Delta  int64  `json:"delta"`
	Clear  bool   `json:"clear"`
	Reason string `json:"reason"`
	Memo   string `json:"memo"`
}

func (s *Server) handleAdminGrantCoins(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	op := auth.MustUser(ctx)
	targetID, err := urlParamInt64(r, "id")
	if err != nil {
		return err
	}
	var req grantCoinsRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if !req.Clear && req.Delta == 0 {
		return httpx.BadRequest("变动金额不可为 0")
	}
	switch req.Reason {
	case "recharge", "admin_grant", "correction":
	default:
		req.Reason = "admin_grant"
	}
	if req.Clear {
		req.Reason = "admin_clear"
	}

	var balance int64
	err = s.db.InSerializableTx(ctx, func(tx pgx.Tx) error {
		delta := req.Delta
		if req.Clear {
			var now int64
			if err := tx.QueryRow(ctx, `SELECT coins FROM users WHERE id = $1 FOR UPDATE`, targetID).Scan(&now); err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					return httpx.NotFound("用户不存在")
				}
				return err
			}
			delta = -now
		}
		bal, err := applyCoinDelta(ctx, tx, targetID, delta, req.Reason, "admin", &op.ID, nullIfEmpty(req.Memo), &op.ID)
		if errors.Is(err, ErrInsufficientCoins) {
			return httpx.BadRequest("该用户余额不足，无法扣减")
		}
		balance = bal
		return err
	})
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"balance": balance})
	return nil
}

type setRoleRequest struct {
	Role   string `json:"role"`
	Banned bool   `json:"banned"`
}

func (s *Server) handleAdminSetRole(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	op := auth.MustUser(ctx)
	targetID, err := urlParamInt64(r, "id")
	if err != nil {
		return err
	}
	if targetID == op.ID {
		return httpx.BadRequest("无法修改自己的权限")
	}
	var req setRoleRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.Role != "admin" && req.Role != "user" {
		return httpx.BadRequest("身份须为 admin 或 user")
	}
	if _, err := s.db.Exec(ctx, `
		UPDATE users SET role = $2,
		       banned_at = CASE WHEN $3 THEN COALESCE(banned_at, now()) ELSE NULL END
		WHERE id = $1`, targetID, req.Role, req.Banned); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"ok": true})
	return nil
}

type adminTask struct {
	Code        string  `json:"code"`
	Kind        string  `json:"kind"`
	Metric      string  `json:"metric"`
	Tag         string  `json:"tag"`
	Category    string  `json:"category"`
	Score       *int    `json:"score"`
	Title       string  `json:"title"`
	Description *string `json:"description"`
	Reward      int64   `json:"reward"`
	Target      int     `json:"target"`
	Sort        int     `json:"sort"`
	Enabled     bool    `json:"enabled"`
}

func (s *Server) handleAdminTasks(w http.ResponseWriter, r *http.Request) error {
	rows, err := s.db.Query(r.Context(), `
		SELECT code, kind, metric, tag, category, score, title, description, reward, target, sort, enabled
		FROM task_defs ORDER BY kind, sort, code`)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []adminTask{}
	for rows.Next() {
		var t adminTask
		if err := rows.Scan(&t.Code, &t.Kind, &t.Metric, &t.Tag, &t.Category, &t.Score, &t.Title, &t.Description, &t.Reward, &t.Target, &t.Sort, &t.Enabled); err != nil {
			return err
		}
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"items": items})
	return nil
}

var taskMetrics = map[string]bool{
	"vote": true, "vote_up": true, "vote_down": true, "post": true, "comment": true, "thread_post": true,
	"edit": true, "page": true, "page_score": true, "comments_received": true, "score_received": true,
	"tip_given": true, "tip_received": true, "gacha": true, "trade": true,
}

func (t *adminTask) check() error {
	t.Title = strings.TrimSpace(t.Title)
	switch {
	case t.Title == "":
		return httpx.BadRequest("任务标题不可为空")
	case t.Kind != "daily" && t.Kind != "weekly" && t.Kind != "monthly":
		return httpx.BadRequest("周期须为每日、每周或每月")
	case !taskMetrics[t.Metric]:
		return httpx.BadRequest("统计项无效")
	case t.Reward < 0:
		return httpx.BadRequest("奖励不可为负数")
	case t.Target < 1:
		return httpx.BadRequest("目标数量至少为 1")
	case t.Metric == "page_score" && t.Score == nil:
		return httpx.BadRequest("请设定作品需达到的评分")
	}
	t.Tag = strings.TrimSpace(t.Tag)
	t.Category = strings.TrimSpace(t.Category)
	return nil
}

func (s *Server) handleAdminSaveTask(w http.ResponseWriter, r *http.Request) error {
	var t adminTask
	if err := httpx.DecodeJSON(r, &t); err != nil {
		return err
	}
	if code := urlParam(r, "code"); code != "" {
		t.Code = code
	}
	t.Code = strings.TrimSpace(t.Code)
	if t.Code == "" {
		return httpx.BadRequest("任务代码不可为空")
	}
	if err := t.check(); err != nil {
		return err
	}
	_, err := s.db.Exec(r.Context(), `
		INSERT INTO task_defs (code, kind, metric, tag, category, score, title, description, reward, target, sort, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (code) DO UPDATE SET
			kind = EXCLUDED.kind, metric = EXCLUDED.metric, tag = EXCLUDED.tag,
			category = EXCLUDED.category, score = EXCLUDED.score, title = EXCLUDED.title,
			description = EXCLUDED.description, reward = EXCLUDED.reward, target = EXCLUDED.target,
			sort = EXCLUDED.sort, enabled = EXCLUDED.enabled`,
		t.Code, t.Kind, t.Metric, t.Tag, t.Category, t.Score, t.Title, t.Description, t.Reward, t.Target, t.Sort, t.Enabled)
	if err != nil {
		return err
	}
	httpx.OK(w, t)
	return nil
}
