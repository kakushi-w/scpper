package api

import (
	"context"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/stockcfg"
)

type position struct {
	Qty       int64
	AvgCost   float64
	Margin    int64
	LockedQty int64
}

func loadPosition(ctx context.Context, tx pgx.Tx, userID int64, symbol string) (position, error) {
	var p position
	err := tx.QueryRow(ctx, `
		SELECT qty, avg_cost, margin, locked_qty FROM stock_positions
		WHERE user_id = $1 AND symbol = $2 FOR UPDATE`,
		userID, symbol).Scan(&p.Qty, &p.AvgCost, &p.Margin, &p.LockedQty)
	if err == pgx.ErrNoRows {
		return position{}, nil
	}
	return p, err
}

func (p position) notional(price float64) int64 {
	return int64(math.Round(math.Abs(float64(p.Qty)) * price))
}

func (p position) unrealized(price float64) int64 {
	return int64(math.Round(float64(p.Qty) * (price - p.AvgCost)))
}

func (p position) equity(price float64) int64 {
	return p.Margin + p.unrealized(price)
}

func (p position) marginRatio(price float64) float64 {
	n := p.notional(price)
	if n <= 0 {
		return math.Inf(1)
	}
	return float64(p.equity(price)) / float64(n)
}

func (p position) liqPrice(maintPct float64) float64 {
	if p.Qty == 0 {
		return 0
	}
	m := maintPct / 100
	q := float64(p.Qty)
	if p.Qty > 0 {
		den := q * (m - 1)
		if den == 0 {
			return 0
		}
		v := (float64(p.Margin) - q*p.AvgCost) / den
		if v < 0 {
			return 0
		}
		return v
	}
	aq := -q
	return (float64(p.Margin) + aq*p.AvgCost) / (aq * (1 + m))
}

func applyFill(
	ctx context.Context, tx pgx.Tx, symbol string, userID int64,
	dQty int64, price float64, fee int64, leverage, feePct float64,
) (marginTaken, payout int64, err error) {
	cur, err := loadPosition(ctx, tx, userID, symbol)
	if err != nil {
		return 0, 0, err
	}
	if leverage < 1 {
		leverage = 1
	}

	qty, avg, margin := cur.Qty, cur.AvgCost, cur.Margin
	realizedDelta := int64(0)

	closing := int64(0)
	if qty != 0 && sign64(dQty) != sign64(qty) {
		closing = minAbs(dQty, qty)
	}

	if closing > 0 {
		absQty := abs64(qty)
		pnl := int64(math.Round(float64(closing) * (price - avg) * float64(sign64(qty))))
		released := int64(math.Round(float64(margin) * float64(closing) / float64(absQty)))
		if released > margin {
			released = margin
		}
		margin -= released
		payout += released + pnl
		realizedDelta += pnl
		qty += int64(sign64(dQty)) * closing
		if qty == 0 {
			payout += margin
			margin = 0
			avg = 0
		}
	}

	if opening := abs64(dQty) - closing; opening > 0 {
		absQty := abs64(qty)
		newAbs := absQty + opening
		avg = (avg*float64(absQty) + float64(opening)*price) / float64(newAbs)
		qty += int64(sign64(dQty)) * opening
		marginTaken = int64(math.Ceil(float64(opening) * price * (1 + feePct/100) / leverage))
		margin += marginTaken
	}

	switch {
	case payout >= fee:
		payout -= fee
	case margin >= fee-payout:
		margin -= fee - payout
		payout = 0
	default:
		margin = 0
		payout = 0
	}
	realizedDelta -= fee

	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_positions (user_id, symbol, qty, avg_cost, margin, realized_pnl, updated_at)
		VALUES ($1,$6,$2,$3,$4,$5,now())
		ON CONFLICT (user_id, symbol) DO UPDATE
		SET qty = $2, avg_cost = $3, margin = $4,
		    realized_pnl = stock_positions.realized_pnl + $5, updated_at = now()`,
		userID, qty, avg, margin, realizedDelta, symbol); err != nil {
		return 0, 0, err
	}
	return marginTaken, payout, nil
}

func sign64(v int64) int {
	switch {
	case v > 0:
		return 1
	case v < 0:
		return -1
	}
	return 0
}

func minAbs(a, b int64) int64 {
	x, y := abs64(a), abs64(b)
	if x < y {
		return x
	}
	return y
}

const liqLockID = 0x5C99_0002

func (s *Server) MarginLoop(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = time.Minute
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.marginTick(ctx)
		}
	}
}

func (s *Server) marginTick(ctx context.Context) {
	var got bool
	if err := s.db.QueryRow(ctx,
		`SELECT pg_try_advisory_lock($1)`, int64(liqLockID)).Scan(&got); err != nil || !got {
		return
	}
	defer func() { _, _ = s.db.Exec(ctx, `SELECT pg_advisory_unlock($1)`, int64(liqLockID)) }()

	markets, err := stockcfg.LoadAll(ctx, s.db, true)
	if err != nil {
		slog.Warn("read symbols", "err", err)
		return
	}

	if err := s.accrueBorrowFees(ctx, markets); err != nil {
		slog.Warn("charge borrow fee", "err", err)
	}
	for _, m := range markets {
		if err := s.liquidateUnderMargin(ctx, m); err != nil {
			slog.Warn("liquidation check", "symbol", m.Code, "err", err)
		}
	}
}

func (s *Server) accrueBorrowFees(ctx context.Context, markets []market) error {
	rateBySymbol := map[string]float64{}
	for _, m := range markets {
		if m.Params.BorrowFeeDailyPct > 0 {
			rateBySymbol[m.Code] = m.Params.BorrowFeeDailyPct
		}
	}
	if len(rateBySymbol) == 0 {
		return nil
	}

	return s.db.InTx(ctx, func(tx pgx.Tx) error {
		var last time.Time
		_ = tx.QueryRow(ctx,
			`SELECT value->>'at' FROM sync_state WHERE key = 'stock.borrowfee'`).Scan(&last)
		now := time.Now()
		days := now.Sub(last).Hours() / 24
		if last.IsZero() || days <= 0 || days > 1 {
			days = 0
		}

		if days > 0 {
			if _, err := tx.Exec(ctx,
				`SELECT 1 FROM stock_maker ORDER BY symbol FOR UPDATE`); err != nil {
				return err
			}
			rows, err := tx.Query(ctx, `
				SELECT user_id, symbol, qty, avg_cost, margin FROM stock_positions
				WHERE qty <> 0 AND margin > 0 FOR UPDATE`)
			if err != nil {
				return err
			}
			type row struct {
				id     int64
				symbol string
				qty    int64
				avg    float64
				margin int64
			}
			var list []row
			for rows.Next() {
				var r row
				if err := rows.Scan(&r.id, &r.symbol, &r.qty, &r.avg, &r.margin); err != nil {
					rows.Close()
					return err
				}
				list = append(list, r)
			}
			rows.Close()
			if err := rows.Err(); err != nil {
				return err
			}

			totals := map[string]int64{}
			for _, r := range list {
				daily, ok := rateBySymbol[r.symbol]
				if !ok {
					continue
				}
				rate := daily / 100 * days
				cost := math.Abs(float64(r.qty)) * r.avg
				base := cost
				if r.qty > 0 {
					base = cost - float64(r.margin)
				}
				if base <= 0 {
					continue
				}
				fee := int64(math.Round(base * rate))
				if fee <= 0 {
					continue
				}
				if fee > r.margin {
					fee = r.margin
				}
				if _, err := tx.Exec(ctx, `
					UPDATE stock_positions SET margin = margin - $3, updated_at = now()
					WHERE user_id = $1 AND symbol = $2`, r.id, r.symbol, fee); err != nil {
					return err
				}
				totals[r.symbol] += fee
			}
			for symbol, total := range totals {
				if total <= 0 {
					continue
				}
				if _, err := tx.Exec(ctx,
					`UPDATE stock_maker SET reserve = reserve + $1 WHERE symbol = $2`,
					total, symbol); err != nil {
					return err
				}
			}
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO sync_state (key, value, updated_at)
			VALUES ('stock.borrowfee', jsonb_build_object('at', now()), now())
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`)
		return err
	})
}

func (s *Server) liquidateUnderMargin(ctx context.Context, m market) error {
	var price float64
	if err := s.db.QueryRow(ctx,
		`SELECT p_ref FROM stock_maker WHERE symbol = $1`, m.Code).Scan(&price); err != nil {
		return err
	}
	maint := m.Params.MaintenanceMarginPct / 100

	rows, err := s.db.Query(ctx, `
		SELECT user_id, qty, avg_cost, margin FROM stock_positions
		WHERE symbol = $1 AND qty <> 0`, m.Code)
	if err != nil {
		return err
	}
	type cand struct {
		id  int64
		pos position
	}
	var list []cand
	for rows.Next() {
		var c cand
		if err := rows.Scan(&c.id, &c.pos.Qty, &c.pos.AvgCost, &c.pos.Margin); err != nil {
			rows.Close()
			return err
		}
		if c.pos.marginRatio(price) < maint {
			list = append(list, c)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, c := range list {
		if err := s.liquidate(ctx, m, c.id); err != nil {
			slog.Warn("liquidate", "symbol", m.Code, "userID", c.id, "err", err)
		}
	}
	return nil
}

func (s *Server) liquidate(ctx context.Context, m market, userID int64) error {
	return s.db.InTx(ctx, func(tx pgx.Tx) error {
		var price float64
		if err := tx.QueryRow(ctx,
			`SELECT p_ref FROM stock_maker WHERE symbol = $1 FOR UPDATE`, m.Code).Scan(&price); err != nil {
			return err
		}
		pos, err := loadPosition(ctx, tx, userID, m.Code)
		if err != nil || pos.Qty == 0 {
			return err
		}
		if pos.marginRatio(price) >= m.Params.MaintenanceMarginPct/100 {
			return nil
		}

		if err := cancelAllOrders(ctx, tx, userID, m.Code); err != nil {
			return err
		}

		beforeMargin, beforeEquity := pos.Margin, pos.equity(price)
		var balBefore int64
		if err := tx.QueryRow(ctx,
			`SELECT coins FROM users WHERE id = $1`, userID).Scan(&balBefore); err != nil {
			return err
		}

		side := "sell"
		if pos.Qty < 0 {
			side = "buy"
		}
		qty := abs64(pos.Qty)
		gross := int64(math.Round(float64(qty) * price))
		fee := tradeFee(gross, m.Params.FeePct)

		if err := settleUserLeg(ctx, tx, m.Code, userID, -pos.Qty, price, fee, 1, m.Params.FeePct, 0); err != nil {
			return err
		}
		if err := makerTakeOther(ctx, tx, m.Code, side, qty, gross); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE stock_maker SET reserve = reserve + $1 WHERE symbol = $2`, fee, m.Code); err != nil {
			return err
		}
		if err := recordFill(ctx, tx, m.Code, userID, side, qty, price, gross, fee); err != nil {
			return err
		}

		var balAfter int64
		if err := tx.QueryRow(ctx,
			`SELECT coins FROM users WHERE id = $1`, userID).Scan(&balAfter); err != nil {
			return err
		}
		payout := balAfter - balBefore
		if payout < 0 {
			payout = 0
		}
		shortfall := int64(0)
		if beforeEquity < 0 {
			shortfall = -beforeEquity
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_liquidations (symbol, user_id, qty, price, margin, equity, payout, shortfall)
			VALUES ($8,$1,$2,$3,$4,$5,$6,$7)`,
			userID, pos.Qty, price, beforeMargin, beforeEquity, payout, shortfall,
			m.Code); err != nil {
			return err
		}

		return nil
	})
}

func cancelAllOrders(ctx context.Context, tx pgx.Tx, userID int64, symbol string) error {
	rows, err := tx.Query(ctx, `
		SELECT id FROM stock_orders
		WHERE user_id = $1 AND symbol = $2 AND status = 'open'`, userID, symbol)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	for _, id := range ids {
		if err := cancelOrderRefund(ctx, tx, id, userID); err != nil {
			return err
		}
	}
	return nil
}
