package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/stockcfg"
)

var listingBotSeed = []struct {
	Name      string
	Inventory int64
	Cash      int64
}{
	{"maker2", 800, 30000},
	{"momentum", 300, 6000},
	{"contrarian", 300, 6000},
	{"random", 300, 6000},
}

var beijing = time.FixedZone("CST", 8*3600)

func beijingMidnight(t time.Time) time.Time {
	b := t.In(beijing)
	return time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, beijing)
}

func (e *Engine) BackfillStock(ctx context.Context, sym Symbol, from, openAt time.Time) (int, error) {
	p := sym.Params

	from = beijingMidnight(from)
	openAt = beijingMidnight(openAt)
	if !from.Before(openAt) {
		return 0, fmt.Errorf("start %s is not before opening day %s",
			from.Format("2006-01-02"), openAt.Format("2006-01-02"))
	}

	var (
		n    int
		prev float64
	)
	for d := from; d.Before(openAt); d = d.AddDate(0, 0, 1) {
		asOf := d.Add(24*time.Hour - time.Second)

		fair, err := e.StockFairPriceAt(ctx, sym, asOf)
		if err != nil {
			return n, err
		}
		if fair < p.MinPrice {
			fair = p.MinPrice
		}
		open := fair
		if prev > 0 {
			open = prev
			if band := p.DailyBandPct / 100; band > 0 {
				fair = math.Max(prev*(1-band), math.Min(prev*(1+band), fair))
				if fair < p.MinPrice {
					fair = p.MinPrice
				}
			}
		}
		high, low := math.Max(open, fair), math.Min(open, fair)

		tag, err := e.db.Exec(ctx, `
			INSERT INTO stock_ticks (symbol, at, open, high, low, close, volume, p_fair, net_qty)
			VALUES ($1,$2,$3,$4,$5,$6,0,$6,0)
			ON CONFLICT (symbol, at) DO NOTHING`,
			sym.Code, asOf.UTC(), round4(open), round4(high), round4(low), round4(fair))
		if err != nil {
			return n, err
		}
		if tag.RowsAffected() > 0 {
			n++
		}
		prev = fair
	}

	openFair, err := e.StockFairPriceAt(ctx, sym, openAt)
	if err != nil {
		return n, err
	}
	if openFair < p.MinPrice {
		openFair = p.MinPrice
	}
	if err := e.db.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`UPDATE stock_symbols SET listed_at = $2 WHERE code = $1`,
			sym.Code, openAt.UTC()); err != nil {
			return err
		}
		_, err := tx.Exec(ctx,
			`UPDATE stock_maker SET p_ref = $2, p_fair = $2, updated_at = now() WHERE symbol = $1`,
			sym.Code, round4(openFair))
		return err
	}); err != nil {
		return n, err
	}

	slog.Info("price history drawn",
		"symbol", sym.Code,
		"from", from.Format("2006-01-02"), "to", openAt.Format("2006-01-02"),
		"days", n, "open", round2(openFair))
	return n, nil
}

func (e *Engine) ListSymbol(ctx context.Context, code string, backfillDays int, openAt time.Time) error {
	sym, err := e.Symbol(ctx, code)
	if err != nil {
		return fmt.Errorf("symbol %s: %w", code, err)
	}
	if sym.Enabled {
		return fmt.Errorf("symbol %s is already listed", code)
	}

	pages, err := e.symbolPageCount(ctx, sym)
	if err != nil {
		return err
	}
	if pages <= 0 {
		return fmt.Errorf("symbol %s has no mirrored pages from %s yet", code, sym.SiteURL)
	}

	if sym.IsCrom() {
		fresh, last, err := e.CromFresh(ctx, sym.SiteURL)
		if err != nil {
			return err
		}
		if !fresh {
			return fmt.Errorf("symbol %s has no crom data since %s",
				code, last.Format("2006-01-02"))
		}
	}

	base := math.Max(2, math.Round(float64(pages)/100)/10)
	over := map[string]any{"scaleBase": pages, "basePrice": base}
	raw, err := json.Marshal(over)
	if err != nil {
		return err
	}
	if _, err := e.db.Exec(ctx,
		`UPDATE stock_symbols SET params = params || $2::jsonb WHERE code = $1`, code, raw); err != nil {
		return err
	}

	if sym, err = e.Symbol(ctx, code); err != nil {
		return err
	}

	if err := e.openBooks(ctx, sym); err != nil {
		return err
	}

	if openAt.IsZero() {
		openAt = time.Now()
	}
	from := openAt.AddDate(0, 0, -backfillDays)
	if backfillDays <= 0 {
		first, ok, err := e.firstPageOf(ctx, sym)
		if err != nil {
			return err
		}
		if ok {
			from = first
		}
		if sym.IsCrom() && !e.cfg.CromHistoryFrom.IsZero() && e.cfg.CromHistoryFrom.After(from) {
			from = e.cfg.CromHistoryFrom
		}
	}
	if _, err := e.BackfillStock(ctx, sym, from, openAt); err != nil {
		return err
	}

	if _, err := e.db.Exec(ctx,
		`UPDATE stock_symbols SET enabled = true WHERE code = $1`, code); err != nil {
		return err
	}

	slog.Info("symbol listed",
		"symbol", sym.Code, "name", sym.Name, "scale_base", pages,
		"open", openAt.Format("2006-01-02"))
	return nil
}

func (e *Engine) firstPageOf(ctx context.Context, sym Symbol) (time.Time, bool, error) {
	if !sym.IsCrom() {
		return e.FirstPage(ctx)
	}
	var first *time.Time
	if err := e.db.QueryRow(ctx,
		`SELECT MIN(created_at) FROM crom_content_pages WHERE site_url = $1`, sym.SiteURL).Scan(&first); err != nil {
		return time.Time{}, false, err
	}
	if first == nil {
		return time.Time{}, false, nil
	}
	return *first, true, nil
}

func (e *Engine) symbolPageCount(ctx context.Context, sym Symbol) (int64, error) {
	if sym.IsCrom() {
		var n int64
		err := e.db.QueryRow(ctx,
			`SELECT COUNT(*) FROM crom_content_pages WHERE site_url = $1`, sym.SiteURL).Scan(&n)
		return n, err
	}
	return e.localPageCount(ctx, time.Now())
}

func (e *Engine) openBooks(ctx context.Context, sym Symbol) error {
	return e.db.InTx(ctx, func(tx pgx.Tx) error {
		var botTotal int64
		for _, b := range listingBotSeed {
			if _, err := tx.Exec(ctx, `
				INSERT INTO stock_bots (symbol, name, inventory, cash)
				VALUES ($1,$2,$3,$4)
				ON CONFLICT (symbol, name) DO NOTHING`,
				sym.Code, b.Name, b.Inventory, b.Cash); err != nil {
				return err
			}
			botTotal += b.Inventory
		}

		if err := tx.QueryRow(ctx,
			`SELECT COALESCE(SUM(inventory), 0) FROM stock_bots WHERE symbol = $1`,
			sym.Code).Scan(&botTotal); err != nil {
			return err
		}

		inv := sym.Params.FloatShares - botTotal
		if inv < 0 {
			return fmt.Errorf("bot inventory %d exceeds float %d", botTotal, sym.Params.FloatShares)
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO stock_maker (symbol, inventory, cash, p_ref, p_fair, reserve)
			VALUES ($1, $2, 0, $3, $3, 0)
			ON CONFLICT (symbol) DO NOTHING`,
			sym.Code, inv, sym.Params.BasePrice)
		return err
	})
}

func (e *Engine) DelistSymbol(ctx context.Context, code string) error {
	if code == stockcfg.Default {
		return fmt.Errorf("CLOUD cannot be delisted")
	}
	return e.db.InTx(ctx, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx,
			`SELECT 1 FROM stock_maker WHERE symbol = $1 FOR UPDATE`, code); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `
			UPDATE stock_orders SET status = 'cancelled', closed_at = now()
			WHERE symbol = $1 AND status = 'open'
			RETURNING id, user_id, hold_coins`, code)
		if err != nil {
			return err
		}
		type refund struct {
			id, user, hold int64
		}
		var list []refund
		for rows.Next() {
			var (
				r    refund
				user *int64
			)
			if err := rows.Scan(&r.id, &user, &r.hold); err != nil {
				rows.Close()
				return err
			}
			if user != nil && r.hold > 0 {
				r.user = *user
				list = append(list, r)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
		for _, r := range list {
			var bal int64
			if err := tx.QueryRow(ctx,
				`UPDATE users SET coins = coins + $2 WHERE id = $1 RETURNING coins`,
				r.user, r.hold).Scan(&bal); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `
				INSERT INTO coin_ledger (user_id, delta, balance_after, reason, ref_type, ref_id)
				VALUES ($1,$2,$3,'stock_unhold','stock_order',$4)`,
				r.user, r.hold, bal, r.id); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx,
				`UPDATE stock_orders SET hold_coins = 0 WHERE id = $1`, r.id); err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `UPDATE stock_symbols SET enabled = false WHERE code = $1`, code)
		return err
	})
}

func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
func round2(v float64) float64 { return math.Round(v*100) / 100 }

func (e *Engine) EnsureLocalHistory(ctx context.Context) error {
	symbols, err := e.ListedSymbols(ctx)
	if err != nil {
		return err
	}
	first, ok, err := e.FirstPage(ctx)
	if err != nil || !ok {
		return err
	}
	for _, sym := range symbols {
		if sym.IsCrom() {
			continue
		}
		var ticks int
		if err := e.db.QueryRow(ctx, `SELECT COUNT(*) FROM stock_ticks WHERE symbol = $1`, sym.Code).Scan(&ticks); err != nil {
			return err
		}
		if ticks > 0 {
			continue
		}
		if _, err := e.BackfillStock(ctx, sym, first, time.Now()); err != nil {
			return err
		}
	}
	return nil
}
