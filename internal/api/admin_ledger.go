package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/httpx"
)

type adminLedgerRow struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"userId"`
	Name         string    `json:"name"`
	Delta        int64     `json:"delta"`
	BalanceAfter int64     `json:"balanceAfter"`
	Reason       string    `json:"reason"`
	Memo         *string   `json:"memo"`
	Operator     *string   `json:"operator"`
	CreatedAt    time.Time `json:"createdAt"`
}

func (s *Server) handleAdminLedger(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	pg := httpx.ParsePage(r, 50, 100)
	b := &argBuilder{}
	where := "1=1"
	if raw := httpx.QueryString(r, "user", ""); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return httpx.BadRequest("用户编号无效")
		}
		where += " AND l.user_id = " + b.add(id)
	}
	if q := httpx.QueryString(r, "q", ""); q != "" {
		where += " AND u.name ILIKE " + b.add(likeContains(q)) + ` ESCAPE '\'`
	}
	if reason := httpx.QueryString(r, "reason", ""); reason != "" {
		where += " AND l.reason = " + b.add(reason)
	}
	from := ` FROM coin_ledger l JOIN users u ON u.id = l.user_id LEFT JOIN users o ON o.id = l.operator_id WHERE ` + where

	var total int
	var in, out int64
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*), COALESCE(SUM(l.delta) FILTER (WHERE l.delta > 0), 0), COALESCE(SUM(l.delta) FILTER (WHERE l.delta < 0), 0)`+from,
		b.all()...).Scan(&total, &in, &out); err != nil {
		return err
	}
	limit, offset := b.add(pg.Size), b.add(pg.Offset)
	rows, err := s.db.Query(ctx, `
		SELECT l.id, l.user_id, u.name, l.delta, l.balance_after, l.reason, l.memo, o.name, l.created_at`+from+`
		ORDER BY l.created_at DESC, l.id DESC
		LIMIT `+limit+` OFFSET `+offset, b.all()...)
	if err != nil {
		return err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (adminLedgerRow, error) {
		var x adminLedgerRow
		err := row.Scan(&x.ID, &x.UserID, &x.Name, &x.Delta, &x.BalanceAfter, &x.Reason, &x.Memo, &x.Operator, &x.CreatedAt)
		return x, err
	})
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"items": items, "meta": httpx.Meta(pg, total), "income": in, "expense": out})
	return nil
}

type grantAllRequest struct {
	Delta int64  `json:"delta"`
	Memo  string `json:"memo"`
}

func (s *Server) handleAdminGrantAll(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	op := auth.MustUser(ctx)
	var req grantAllRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.Delta <= 0 {
		return httpx.BadRequest("发放数量须为正数")
	}
	var n int64
	err := s.db.InSerializableTx(ctx, func(tx pgx.Tx) error {
		return tx.QueryRow(ctx, `
			WITH paid AS (
				UPDATE users SET coins = coins + $1
				WHERE banned_at IS NULL
				RETURNING id, coins
			), logged AS (
				INSERT INTO coin_ledger (user_id, delta, balance_after, reason, ref_type, memo, operator_id)
				SELECT id, $1, coins, 'admin_grant', 'admin', $2, $3 FROM paid
				RETURNING 1
			)
			SELECT COUNT(*) FROM logged`, req.Delta, nullIfEmpty(strings.TrimSpace(req.Memo)), op.ID).Scan(&n)
	})
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"users": n})
	return nil
}

type taskCompletion struct {
	UserID      int64      `json:"userId"`
	Name        string     `json:"name"`
	Progress    int        `json:"progress"`
	Target      int        `json:"target"`
	CompletedAt *time.Time `json:"completedAt"`
	ClaimedAt   *time.Time `json:"claimedAt"`
}

type taskPeriod struct {
	Period    string `json:"period"`
	Completed int    `json:"completed"`
	Claimed   int    `json:"claimed"`
}

func (s *Server) handleAdminTaskCompletions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	code := urlParam(r, "code")
	var kind string
	if err := s.db.QueryRow(ctx, `SELECT kind FROM task_defs WHERE code = $1`, code).Scan(&kind); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("任务不存在")
		}
		return err
	}

	rows, err := s.db.Query(ctx, `
		SELECT period, COUNT(*) FILTER (WHERE completed_at IS NOT NULL), COUNT(*) FILTER (WHERE claimed_at IS NOT NULL)
		FROM task_progress WHERE task_code = $1
		GROUP BY period ORDER BY period DESC LIMIT 30`, code)
	if err != nil {
		return err
	}
	periods, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (taskPeriod, error) {
		var p taskPeriod
		err := row.Scan(&p.Period, &p.Completed, &p.Claimed)
		return p, err
	})
	if err != nil {
		return err
	}

	period := httpx.QueryString(r, "period", s.scanner.Period(time.Now(), kind))
	pg := httpx.ParsePage(r, 50, 100)
	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM task_progress WHERE task_code = $1 AND period = $2 AND completed_at IS NOT NULL`,
		code, period).Scan(&total); err != nil {
		return err
	}
	rows, err = s.db.Query(ctx, `
		SELECT p.user_id, u.name, p.progress, p.target, p.completed_at, p.claimed_at
		FROM task_progress p JOIN users u ON u.id = p.user_id
		WHERE p.task_code = $1 AND p.period = $2 AND p.completed_at IS NOT NULL
		ORDER BY p.completed_at, p.user_id
		LIMIT $3 OFFSET $4`, code, period, pg.Size, pg.Offset)
	if err != nil {
		return err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (taskCompletion, error) {
		var c taskCompletion
		err := row.Scan(&c.UserID, &c.Name, &c.Progress, &c.Target, &c.CompletedAt, &c.ClaimedAt)
		return c, err
	})
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"period": period, "periods": periods, "items": items, "meta": httpx.Meta(pg, total)})
	return nil
}

type stockHolder struct {
	UserID      int64   `json:"userId"`
	Name        string  `json:"name"`
	Qty         int64   `json:"qty"`
	LockedQty   int64   `json:"lockedQty"`
	AvgCost     float64 `json:"avgCost"`
	Margin      int64   `json:"margin"`
	RealizedPnl int64   `json:"realizedPnl"`
	Value       int64   `json:"value"`
}

func (s *Server) handleAdminStockHolders(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	code := urlParam(r, "code")
	pg := httpx.ParsePage(r, 50, 100)
	var total int
	var price float64
	if err := s.db.QueryRow(ctx, `
		SELECT (SELECT COUNT(*) FROM stock_positions WHERE symbol = $1 AND (qty <> 0 OR locked_qty <> 0)),
		       COALESCE((SELECT p_ref FROM stock_maker WHERE symbol = $1), 0)`, code).Scan(&total, &price); err != nil {
		return err
	}
	rows, err := s.db.Query(ctx, `
		SELECT p.user_id, u.name, p.qty, p.locked_qty, p.avg_cost, p.margin, p.realized_pnl,
		       ROUND(p.qty * $2::numeric)::bigint
		FROM stock_positions p JOIN users u ON u.id = p.user_id
		WHERE p.symbol = $1 AND (p.qty <> 0 OR p.locked_qty <> 0)
		ORDER BY abs(p.qty) DESC, p.user_id
		LIMIT $3 OFFSET $4`, code, price, pg.Size, pg.Offset)
	if err != nil {
		return err
	}
	items, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (stockHolder, error) {
		var h stockHolder
		err := row.Scan(&h.UserID, &h.Name, &h.Qty, &h.LockedQty, &h.AvgCost, &h.Margin, &h.RealizedPnl, &h.Value)
		return h, err
	})
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"items": items, "meta": httpx.Meta(pg, total)})
	return nil
}
