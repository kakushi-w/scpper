package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/httpx"
	"github.com/scpwikicloud/scpper/internal/model"
)

func (s *Server) handleMyTasks(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.MustUser(ctx)

	now := time.Now()
	day, week, month := s.scanner.Period(now, "daily"), s.scanner.Period(now, "weekly"), s.scanner.Period(now, "monthly")

	rows, err := s.db.Query(ctx, `
		SELECT d.code, d.kind, d.title, d.description, d.reward, d.target,
		       COALESCE(p.progress, 0),
		       CASE d.kind WHEN 'daily' THEN $2 WHEN 'weekly' THEN $3 ELSE $4 END AS period,
		       p.completed_at, p.claimed_at
		FROM task_defs d
		LEFT JOIN task_progress p
		       ON p.task_code = d.code
		      AND p.user_id = $1
		      AND p.period = CASE d.kind WHEN 'daily' THEN $2 WHEN 'weekly' THEN $3 ELSE $4 END
		WHERE d.enabled
		ORDER BY d.sort, d.code`, u.ID, day, week, month)
	if err != nil {
		return err
	}
	defer rows.Close()

	daily := []model.TaskRow{}
	weekly := []model.TaskRow{}
	monthly := []model.TaskRow{}
	var claimable int64
	for rows.Next() {
		var t model.TaskRow
		if err := rows.Scan(&t.Code, &t.Kind, &t.Title, &t.Description, &t.Reward,
			&t.Target, &t.Progress, &t.Period, &t.CompletedAt, &t.ClaimedAt); err != nil {
			return err
		}
		if t.CompletedAt != nil && t.ClaimedAt == nil {
			claimable += t.Reward
		}
		switch t.Kind {
		case "daily":
			daily = append(daily, t)
		case "weekly":
			weekly = append(weekly, t)
		default:
			monthly = append(monthly, t)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	httpx.OK(w, map[string]any{
		"daily":          daily,
		"weekly":         weekly,
		"monthly":        monthly,
		"dailyPeriod":    day,
		"weeklyPeriod":   week,
		"monthlyPeriod":  month,
		"claimableCoins": claimable,
	})
	return nil
}

func (s *Server) handleClaimTask(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.MustUser(ctx)
	code := urlParam(r, "code")
	if code == "" {
		return httpx.BadRequest("缺少任务代码")
	}

	now := time.Now()

	var (
		newBalance int64
		reward     int64
		title      string
	)
	err := s.db.InSerializableTx(ctx, func(tx pgx.Tx) error {
		var kind string
		if err := tx.QueryRow(ctx,
			`SELECT kind, reward, title FROM task_defs WHERE code = $1 AND enabled`, code,
		).Scan(&kind, &reward, &title); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.NotFound("任务不存在")
			}
			return err
		}
		period := s.scanner.Period(now, kind)

		tag, err := tx.Exec(ctx, `
			UPDATE task_progress SET claimed_at = now()
			WHERE user_id = $1 AND task_code = $2 AND period = $3
			  AND completed_at IS NOT NULL AND claimed_at IS NULL`,
			u.ID, code, period)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			var completed, claimed *time.Time
			qerr := tx.QueryRow(ctx,
				`SELECT completed_at, claimed_at FROM task_progress
				 WHERE user_id = $1 AND task_code = $2 AND period = $3`,
				u.ID, code, period).Scan(&completed, &claimed)
			if qerr == nil && claimed != nil {
				return httpx.Conflict("该任务的奖励已领取")
			}
			return httpx.BadRequest("任务尚未完成")
		}

		memo := "任务奖励：" + title
		bal, err := applyCoinDelta(ctx, tx, u.ID, reward, "task", "task", nil, &memo, nil)
		if err != nil {
			return err
		}
		newBalance = bal
		return nil
	})
	if err != nil {
		return err
	}

	httpx.OK(w, map[string]any{
		"claimed": true,
		"reward":  reward,
		"balance": newBalance,
	})
	return nil
}

func (s *Server) handleRefreshTasks(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.MustUser(ctx)

	if ok, wait := s.rl.Allow("taskscan:"+itoa(u.ID), 20*time.Second); !ok {
		return httpx.TooMany("操作过于频繁，请在 %d 秒后重试", int(wait.Seconds())+1)
	}
	if err := s.scanner.ScanUser(ctx, u.ID); err != nil {
		return err
	}
	return s.handleMyTasks(w, r)
}
