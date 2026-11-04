package api

import (
	"errors"
	"math"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/httpx"
)

type placeReq struct {
	Side     string  `json:"side"`
	Qty      int64   `json:"qty"`
	Leverage float64 `json:"leverage"`
}

func (s *Server) handleStockPlace(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.MustUser(ctx)

	var req placeReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.Side != "buy" && req.Side != "sell" {
		return httpx.BadRequest("买卖方向无效")
	}
	if req.Qty <= 0 || req.Qty > 100000 {
		return httpx.BadRequest("数量须在 1 到 100000 股之间")
	}

	m, err := s.market(r)
	if err != nil {
		return err
	}
	p := m.Params
	if req.Leverage <= 0 {
		req.Leverage = 1
	}
	if req.Leverage > p.MaxLeverage {
		return httpx.BadRequest("杠杆不可超过 %.0f 倍", p.MaxLeverage)
	}

	var (
		orderID int64
		hold    int64
		balance int64
	)
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		var pRef float64
		if err := tx.QueryRow(ctx,
			`SELECT p_ref FROM stock_maker WHERE symbol = $1 FOR SHARE`, m.Code).Scan(&pRef); err != nil {
			return err
		}

		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM stock_orders
			               WHERE user_id = $1 AND symbol = $2 AND status = 'open')`,
			u.ID, m.Code).Scan(&exists); err != nil {
			return err
		}
		if exists {
			return httpx.BadRequest("本场已有挂单，请撤单后重新下单")
		}

		pos, err := loadPosition(ctx, tx, u.ID, m.Code)
		if err != nil {
			return err
		}
		dir := int64(1)
		if req.Side == "sell" {
			dir = -1
		}
		closing := int64(0)
		if pos.Qty != 0 && sign64(pos.Qty) != int(dir) {
			closing = minAbs(req.Qty, pos.Qty)
		}
		opening := req.Qty - closing

		if opening > 0 && dir < 0 && !p.AllowShort {
			return httpx.BadRequest("当前不允许做空")
		}
		if opening > 0 {
			limit := maxPosition(p)
			if after := abs64(pos.Qty + dir*req.Qty); after > limit {
				return httpx.BadRequest("超出单个账号的持仓上限 %d 股", limit)
			}
		}

		if opening > 0 {
			capPx := auctionCap(p, pRef)
			hold = int64(math.Ceil(float64(opening) * capPx * (1 + p.FeePct/100) / req.Leverage))
			if _, err := applyCoinDelta(ctx, tx, u.ID, -hold,
				"stock_hold", "stock_order", nil, nil, nil); err != nil {
				return err
			}
		}

		if err := tx.QueryRow(ctx, `
			INSERT INTO stock_orders (symbol, user_id, side, kind, qty, hold_coins, leverage)
			VALUES ($1,$2,$3,'auction',$4,$5,$6) RETURNING id`,
			m.Code, u.ID, req.Side, req.Qty, hold, req.Leverage).Scan(&orderID); err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return httpx.BadRequest("本场已有挂单，请撤单后重新下单")
			}
			return err
		}
		return tx.QueryRow(ctx, `SELECT coins FROM users WHERE id = $1`, u.ID).Scan(&balance)
	})
	if err != nil {
		if errors.Is(err, ErrInsufficientCoins) {
			return httpx.Err(http.StatusPaymentRequired, "insufficient_coins", "云币不足")
		}
		return err
	}

	httpx.OK(w, map[string]any{
		"symbol":        m.Code,
		"orderId":       orderID,
		"hold":          hold,
		"balance":       balance,
		"nextAuctionAt": p.NextSlot(time.Now()),
	})
	return nil
}

func (s *Server) handleStockCancel(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.MustUser(ctx)
	id, err := urlParamInt64(r, "id")
	if err != nil {
		return err
	}

	var balance int64
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		var symbol string
		err := tx.QueryRow(ctx, `
			SELECT symbol FROM stock_orders
			WHERE id = $1 AND user_id = $2 AND status = 'open'`, id, u.ID).Scan(&symbol)
		if errors.Is(err, pgx.ErrNoRows) {
			return httpx.NotFound("该订单不存在或已成交")
		}
		if err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`SELECT 1 FROM stock_maker WHERE symbol = $1 FOR SHARE`, symbol); err != nil {
			return err
		}
		var status string
		if err := tx.QueryRow(ctx,
			`SELECT status FROM stock_orders WHERE id = $1 FOR UPDATE`, id).Scan(&status); err != nil {
			return err
		}
		if status != "open" {
			return httpx.NotFound("该订单不存在或已成交")
		}
		if err := cancelOrderRefund(ctx, tx, id, u.ID); err != nil {
			return err
		}
		return tx.QueryRow(ctx, `SELECT coins FROM users WHERE id = $1`, u.ID).Scan(&balance)
	})
	if err != nil {
		return err
	}

	httpx.OK(w, map[string]any{"ok": true, "balance": balance})
	return nil
}
