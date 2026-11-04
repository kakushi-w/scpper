package api

import (
	"context"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/httpx"
	"github.com/scpwikicloud/scpper/internal/model"
)

var ErrInsufficientCoins = errors.New("云币不足")

func applyCoinDelta(
	ctx context.Context, tx pgx.Tx,
	userID int64, delta int64,
	reason, refType string, refID *int64, memo *string, operatorID *int64,
) (int64, error) {
	if delta == 0 {
		var bal int64
		err := tx.QueryRow(ctx, `SELECT coins FROM users WHERE id = $1 FOR UPDATE`, userID).Scan(&bal)
		return bal, err
	}

	var balanceAfter int64
	err := tx.QueryRow(ctx, `
		UPDATE users SET coins = coins + $2
		WHERE id = $1 AND coins + $2 >= 0
		RETURNING coins`, userID, delta).Scan(&balanceAfter)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrInsufficientCoins
		}
		return 0, err
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO coin_ledger (user_id, delta, balance_after, reason, ref_type, ref_id, memo, operator_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)`,
		userID, delta, balanceAfter, reason, nullIfEmpty(refType), refID, memo, operatorID)
	if err != nil {
		return 0, err
	}
	return balanceAfter, nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func strPtr(s string) *string { return &s }

func (s *Server) handleCoinLedger(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.MustUser(ctx)
	pg := httpx.ParsePage(r, 30, 100)

	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM coin_ledger WHERE user_id = $1`, u.ID).Scan(&total); err != nil {
		return err
	}

	rows, err := s.db.Query(ctx, `
		SELECT id, delta, balance_after, reason, memo, created_at
		FROM coin_ledger
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3`, u.ID, pg.Size, pg.Offset)
	if err != nil {
		return err
	}
	defer rows.Close()

	items := []model.CoinLedgerRow{}
	for rows.Next() {
		var row model.CoinLedgerRow
		if err := rows.Scan(&row.ID, &row.Delta, &row.BalanceAfter, &row.Reason,
			&row.Memo, &row.CreatedAt); err != nil {
			return err
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	httpx.OK(w, map[string]any{
		"items":   items,
		"meta":    httpx.Meta(pg, total),
		"balance": u.Coins,
	})
	return nil
}
