package api

import (
	"context"
	"math"

	"github.com/jackc/pgx/v5"
)

func tradeFee(gross int64, feePct float64) int64 {
	if gross <= 0 {
		return 0
	}
	fee := int64(math.Round(float64(gross) * feePct / 100))
	if fee < 1 {
		fee = 1
	}
	return fee
}

func makerTakeOther(ctx context.Context, tx pgx.Tx, symbol, userSide string, qty, gross int64) error {
	if userSide == "buy" {
		_, err := tx.Exec(ctx, `
			UPDATE stock_maker
			SET inventory = inventory - $1, cash = cash + $2, updated_at = now()
			WHERE symbol = $3`, qty, gross, symbol)
		return err
	}
	_, err := tx.Exec(ctx, `
		UPDATE stock_maker
		SET inventory = inventory + $1, cash = cash - $2, updated_at = now()
		WHERE symbol = $3`, qty, gross, symbol)
	return err
}

func recordFill(
	ctx context.Context, tx pgx.Tx, symbol string, userID int64, side string,
	qty int64, price float64, gross, fee int64,
) error {
	other := "sell"
	if side == "sell" {
		other = "buy"
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO stock_trades (symbol, user_id, side, qty, price, gross, fee, taker_side)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$3)`,
		symbol, userID, side, qty, price, gross, fee); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `
		INSERT INTO stock_trades (symbol, user_id, side, qty, price, gross, fee, taker_side)
		VALUES ($1,NULL,$2,$3,$4,$5,0,$6)`,
		symbol, other, qty, price, gross, side)
	return err
}

func settleUserLeg(
	ctx context.Context, tx pgx.Tx, symbol string, userID, dQty int64, price float64,
	fee int64, leverage, feePct float64, orderID int64,
) error {
	marginTaken, payout, err := applyFill(ctx, tx, symbol, userID, dQty, price, fee, leverage, feePct)
	if err != nil {
		return err
	}
	net := payout - marginTaken
	switch {
	case net > 0:
		_, err := applyCoinDelta(ctx, tx, userID, net, "stock_settle", "stock", nil, nil, nil)
		return err
	case net < 0:
		need := -net
		take := int64(0)
		if orderID > 0 {
			var hold int64
			if err := tx.QueryRow(ctx,
				`SELECT hold_coins FROM stock_orders WHERE id = $1 FOR UPDATE`,
				orderID).Scan(&hold); err != nil {
				return err
			}
			take = need
			if take > hold {
				take = hold
			}
			if take > 0 {
				if _, err := tx.Exec(ctx,
					`UPDATE stock_orders SET hold_coins = hold_coins - $2 WHERE id = $1`,
					orderID, take); err != nil {
					return err
				}
			}
		}
		if rest := need - take; rest > 0 {
			var ref *int64
			if orderID > 0 {
				ref = &orderID
			}
			if _, err := applyCoinDelta(ctx, tx, userID, -rest,
				"stock_margin", "stock_order", ref, nil, nil); err != nil {
				return err
			}
		}
	}
	return nil
}

func abs64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
