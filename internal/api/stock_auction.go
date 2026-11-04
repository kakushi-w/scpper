package api

import (
	"context"
	"crypto/rand"
	"errors"
	"log/slog"
	"math"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/httpx"
	"github.com/scpwikicloud/scpper/internal/stockcfg"
)

const auctionLockID = 0x5C99_0003

const cromLag = 30 * time.Minute

type auctionEvent struct {
	Title     string    `json:"title"`
	AltTitle  string    `json:"altTitle,omitempty"`
	URL       string    `json:"url"`
	CreatedAt time.Time `json:"createdAt"`
}

type auctionRow struct {
	At          time.Time      `json:"at"`
	PriceBefore float64        `json:"priceBefore"`
	PriceAfter  float64        `json:"priceAfter"`
	ChangePct   float64        `json:"changePct"`
	Fair        float64        `json:"fair"`
	Orders      int            `json:"orders"`
	BuyQty      int64          `json:"buyQty"`
	SellQty     int64          `json:"sellQty"`
	ImpactPct   float64        `json:"impactPct"`
	DriftPct    float64        `json:"driftPct"`
	ShockPct    float64        `json:"shockPct"`
	EventPct    float64        `json:"eventPct"`
	NewPages    int            `json:"newPages"`
	Expected    float64        `json:"expected"`
	Events      []auctionEvent `json:"events"`
}

const auctionCols = `at, price_before, price_after, fair, orders, buy_qty, sell_qty,
	impact_pct, drift_pct, shock_pct, event_pct, new_pages, expected, events`

func scanAuction(row pgx.Row) (auctionRow, error) {
	var a auctionRow
	err := row.Scan(&a.At, &a.PriceBefore, &a.PriceAfter, &a.Fair, &a.Orders, &a.BuyQty, &a.SellQty,
		&a.ImpactPct, &a.DriftPct, &a.ShockPct, &a.EventPct, &a.NewPages, &a.Expected, &a.Events)
	if err != nil {
		return a, err
	}
	if a.Events == nil {
		a.Events = []auctionEvent{}
	}
	a.ChangePct = round2((a.PriceAfter/a.PriceBefore - 1) * 100)
	a.PriceBefore, a.PriceAfter, a.Fair = round2(a.PriceBefore), round2(a.PriceAfter), round2(a.Fair)
	a.ImpactPct, a.DriftPct = round2(a.ImpactPct), round2(a.DriftPct)
	a.ShockPct, a.EventPct = round2(a.ShockPct), round2(a.EventPct)
	a.Expected = round2(a.Expected)
	return a, nil
}

func maxPosition(p stockParams) int64 {
	return p.FloatShares * int64(p.MaxPositionPct) / 100
}

func impactLog(p stockParams, net int64) float64 {
	mp := float64(maxPosition(p))
	if mp <= 0 {
		return 0
	}
	return p.ImpactPct / 100 * float64(net) / mp
}

func auctionCap(p stockParams, pRef float64) float64 {
	return pRef * (1 + p.DailyBandPct/100)
}

func auctionFloor(p stockParams, pRef float64) float64 {
	return math.Max(p.MinPrice, pRef*(1-p.DailyBandPct/100))
}

func gauss() float64 {
	u := randUnit()
	v := randUnit()
	return math.Sqrt(-2*math.Log(u)) * math.Cos(2*math.Pi*v)
}

func randUnit() float64 {
	const denom = 1 << 53
	n, err := rand.Int(rand.Reader, big.NewInt(denom))
	if err != nil {
		return 0.5
	}
	if f := float64(n.Int64()) / denom; f > 0 {
		return f
	}
	return 1.0 / denom
}

func (s *Server) AuctionLoop(ctx context.Context, every time.Duration) {
	if every <= 0 {
		every = 20 * time.Second
	}
	s.auctionTick(ctx)
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			s.auctionTick(ctx)
		}
	}
}

func (s *Server) auctionTick(ctx context.Context) {
	var got bool
	if err := s.db.QueryRow(ctx,
		`SELECT pg_try_advisory_lock($1)`, int64(auctionLockID)).Scan(&got); err != nil || !got {
		return
	}
	defer func() { _, _ = s.db.Exec(ctx, `SELECT pg_advisory_unlock($1)`, int64(auctionLockID)) }()

	markets, err := stockcfg.LoadAll(ctx, s.db, true)
	if err != nil {
		slog.Warn("read symbols", "err", err)
		return
	}
	now := time.Now()
	for _, m := range markets {
		slot := m.Params.LastSlot(now)
		if slot.IsZero() {
			continue
		}
		if m.ListedAt != nil && m.ListedAt.After(slot) {
			continue
		}
		if _, err := s.RunAuction(ctx, m, slot); err != nil {
			slog.Warn("auction", "symbol", m.Code, "slot", slot, "err", err)
		}
	}
}

func (s *Server) RunAuction(ctx context.Context, m market, at time.Time) (*auctionRow, error) {
	var out *auctionRow
	err := s.db.InTx(ctx, func(tx pgx.Tx) error {
		p := m.Params

		var pRef, pFair float64
		if err := tx.QueryRow(ctx,
			`SELECT p_ref, p_fair FROM stock_maker WHERE symbol = $1 FOR UPDATE`,
			m.Code).Scan(&pRef, &pFair); err != nil {
			return err
		}

		var prevAt *time.Time
		if err := tx.QueryRow(ctx,
			`SELECT MAX(at) FROM stock_auctions WHERE symbol = $1`, m.Code).Scan(&prevAt); err != nil {
			return err
		}
		if prevAt != nil && !at.After(*prevAt) {
			return nil
		}
		since := at.Add(-8 * time.Hour)
		if prevAt != nil {
			since = *prevAt
		}
		days := at.Sub(since).Hours() / 24
		if days > 1 {
			days = 1
		}

		type pending struct {
			id, userID int64
			side       string
			qty        int64
			lev        float64
		}
		rows, err := tx.Query(ctx, `
			SELECT id, user_id, side, qty, leverage FROM stock_orders
			WHERE symbol = $1 AND status = 'open' AND user_id IS NOT NULL
			ORDER BY id FOR UPDATE`, m.Code)
		if err != nil {
			return err
		}
		var orders []pending
		var buyQty, sellQty int64
		for rows.Next() {
			var o pending
			if err := rows.Scan(&o.id, &o.userID, &o.side, &o.qty, &o.lev); err != nil {
				rows.Close()
				return err
			}
			if o.side == "buy" {
				buyQty += o.qty
			} else {
				sellQty += o.qty
			}
			orders = append(orders, o)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}

		impact := impactLog(p, buyQty-sellQty)
		drift := 0.0
		if pFair > 0 && pRef > 0 {
			kappa := math.Ln2 / math.Max(p.HalfLifeDays, 0.1)
			drift = kappa * math.Log(pFair/pRef) * days
		}
		shock := p.SigmaDaily * math.Sqrt(days) * gauss()

		ev, err := auctionEvents(ctx, tx, m, since, at, days)
		if err != nil {
			return err
		}

		next := pRef * math.Exp(impact+drift+shock+math.Log1p(ev.pct/100))
		next = math.Min(next, auctionCap(p, pRef))
		next = math.Max(next, auctionFloor(p, pRef))
		next = math.Round(next*10000) / 10000

		filled := 0
		for _, o := range orders {
			sp, err := tx.Begin(ctx)
			if err != nil {
				return err
			}
			if err := s.fillAuctionOrder(ctx, sp, m, o.id, o.userID, o.side, o.qty, o.lev, next); err != nil {
				_ = sp.Rollback(ctx)
				if !errors.Is(err, ErrInsufficientCoins) {
					return err
				}
				slog.Warn("order cancelled for lack of coins at settlement", "symbol", m.Code, "order", o.id, "user", o.userID)
				if err := cancelOrderRefund(ctx, tx, o.id, o.userID); err != nil {
					return err
				}
				continue
			}
			if err := sp.Commit(ctx); err != nil {
				return err
			}
			filled++
		}

		if _, err := tx.Exec(ctx,
			`UPDATE stock_maker SET p_ref = $2, updated_at = now() WHERE symbol = $1`,
			m.Code, next); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_ticks (symbol, at, open, high, low, close, volume, p_fair, net_qty)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
			ON CONFLICT (symbol, at) DO NOTHING`,
			m.Code, at, pRef, math.Max(pRef, next), math.Min(pRef, next), next,
			buyQty+sellQty, pFair, buyQty-sellQty); err != nil {
			return err
		}

		pct := func(l float64) float64 { return (math.Exp(l) - 1) * 100 }
		row := auctionRow{
			At: at, PriceBefore: pRef, PriceAfter: next, Fair: pFair,
			Orders: filled, BuyQty: buyQty, SellQty: sellQty,
			ImpactPct: pct(impact), DriftPct: pct(drift), ShockPct: pct(shock), EventPct: ev.pct,
			NewPages: ev.n, Expected: ev.expected, Events: ev.list,
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO stock_auctions (symbol, `+auctionCols+`)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
			m.Code, row.At, row.PriceBefore, row.PriceAfter, row.Fair, row.Orders,
			row.BuyQty, row.SellQty, row.ImpactPct, row.DriftPct, row.ShockPct, row.EventPct,
			row.NewPages, row.Expected, row.Events); err != nil {
			return err
		}

		slog.Info("auction",
			"symbol", m.Code, "from", round2(pRef), "to", round2(next),
			"buy", buyQty, "sell", sellQty, "filled", filled,
			"impact_pct", round2(row.ImpactPct), "drift_pct", round2(row.DriftPct),
			"shock_pct", round2(row.ShockPct), "event_pct", round2(row.EventPct), "new_pages", ev.n)
		out = &row
		return nil
	})
	if err != nil || out == nil {
		return out, err
	}

	if err := s.liquidateUnderMargin(ctx, m); err != nil {
		slog.Warn("liquidation check after auction", "symbol", m.Code, "err", err)
	}
	return out, nil
}

func (s *Server) fillAuctionOrder(
	ctx context.Context, tx pgx.Tx, m market,
	orderID, userID int64, side string, qty int64, lev, price float64,
) error {
	p := m.Params
	dQty := qty
	if side == "sell" {
		dQty = -qty
	}
	gross := int64(math.Round(float64(qty) * price))
	fee := tradeFee(gross, p.FeePct)

	if err := settleUserLeg(ctx, tx, m.Code, userID, dQty, price, fee, lev, p.FeePct, orderID); err != nil {
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

	var left int64
	if err := tx.QueryRow(ctx, `
		UPDATE stock_orders SET filled = qty, status = 'filled', closed_at = now(), price = $2
		WHERE id = $1 RETURNING hold_coins`, orderID, price).Scan(&left); err != nil {
		return err
	}
	if left > 0 {
		if _, err := applyCoinDelta(ctx, tx, userID, left,
			"stock_unhold", "stock_order", &orderID, nil, nil); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx,
			`UPDATE stock_orders SET hold_coins = 0 WHERE id = $1`, orderID); err != nil {
			return err
		}
	}

	return nil
}

func cancelOrderRefund(ctx context.Context, tx pgx.Tx, orderID, userID int64) error {
	var hold int64
	err := tx.QueryRow(ctx, `
		UPDATE stock_orders SET status = 'cancelled', closed_at = now()
		WHERE id = $1 AND user_id = $2 AND status = 'open'
		RETURNING hold_coins`, orderID, userID).Scan(&hold)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	if hold <= 0 {
		return nil
	}
	if _, err := applyCoinDelta(ctx, tx, userID, hold,
		"stock_unhold", "stock_order", &orderID, nil, nil); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE stock_orders SET hold_coins = 0 WHERE id = $1`, orderID)
	return err
}

type eventResult struct {
	pct      float64
	n        int
	expected float64
	list     []auctionEvent
}

func auctionEvents(
	ctx context.Context, tx pgx.Tx, m market, since, at time.Time, days float64,
) (eventResult, error) {
	var (
		res  eventResult
		n90  int64
		rows pgx.Rows
		err  error
	)
	if m.IsCrom() {
		since, at = since.Add(-cromLag), at.Add(-cromLag)
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM crom_content_pages
			WHERE site_url = $1
			  AND created_at BETWEEN $2::timestamptz - interval '90 days' AND $2::timestamptz`,
			m.SiteURL, at).Scan(&n90); err != nil {
			return res, err
		}
		rows, err = tx.Query(ctx, `
			SELECT COALESCE(title, ''), url, created_at FROM crom_content_pages
			WHERE site_url = $1 AND created_at > $2 AND created_at <= $3
			ORDER BY created_at DESC`, m.SiteURL, since, at)
	} else {
		if err := tx.QueryRow(ctx, `
			SELECT COUNT(*) FROM articles
			WHERE status <> 'deleted'
			  AND created_at BETWEEN $1::timestamptz - interval '90 days' AND $1::timestamptz`,
			at).Scan(&n90); err != nil {
			return res, err
		}
		rows, err = tx.Query(ctx, `
			SELECT COALESCE(NULLIF(title, ''), page), url, created_at FROM articles
			WHERE status <> 'deleted' AND created_at > $1 AND created_at <= $2
			ORDER BY created_at DESC`, since, at)
	}
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var e auctionEvent
		if err := rows.Scan(&e.Title, &e.URL, &e.CreatedAt); err != nil {
			return res, err
		}
		if e.Title == "" {
			e.Title = e.URL[strings.LastIndex(e.URL, "/")+1:]
		}
		res.n++
		if len(res.list) < 12 {
			res.list = append(res.list, e)
		}
	}
	if err := rows.Err(); err != nil {
		return res, err
	}
	if res.list == nil {
		res.list = []auctionEvent{}
	}

	p := m.Params
	res.expected = float64(n90) / 90 * days
	if p.EventPct > 0 {
		r := (float64(res.n) - res.expected) / math.Max(res.expected, 0.3)
		res.pct = math.Max(-p.EventPct, math.Min(p.EventCapPct, p.EventPct*r))
	}
	return res, nil
}

func fillEventTitles(ctx context.Context, q interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}, list []auctionRow) error {
	var urls []string
	for _, a := range list {
		for _, e := range a.Events {
			urls = append(urls, e.URL)
		}
	}
	if len(urls) == 0 {
		return nil
	}
	rows, err := q.Query(ctx, `
		SELECT url, COALESCE(title, ''), COALESCE(alt_title, '')
		FROM crom_pages WHERE url = ANY($1)`, urls)
	if err != nil {
		return err
	}
	defer rows.Close()
	type tt struct{ title, alt string }
	found := map[string]tt{}
	for rows.Next() {
		var u string
		var t tt
		if err := rows.Scan(&u, &t.title, &t.alt); err != nil {
			return err
		}
		found[u] = t
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range list {
		for j := range list[i].Events {
			e := &list[i].Events[j]
			t, ok := found[e.URL]
			if !ok {
				continue
			}
			if t.title != "" {
				e.Title = t.title
			}
			if t.alt != "" && t.alt != e.Title {
				e.AltTitle = t.alt
			}
		}
	}
	return nil
}

func (s *Server) handleStockAuctions(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	m, err := s.market(r)
	if err != nil {
		return err
	}
	limit := httpx.QueryInt(r, "limit", 20, 1, 100)
	rows, err := s.db.Query(ctx,
		`SELECT `+auctionCols+` FROM stock_auctions WHERE symbol = $1 ORDER BY at DESC LIMIT $2`,
		m.Code, limit)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []auctionRow{}
	for rows.Next() {
		a, err := scanAuction(rows)
		if err != nil {
			return err
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := fillEventTitles(ctx, s.db, items); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"items": items})
	return nil
}

func (s *Server) handleAdminStockAuction(w http.ResponseWriter, r *http.Request) error {
	m, err := s.market(r)
	if err != nil {
		return err
	}
	row, err := s.RunAuction(r.Context(), m, time.Now())
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"symbol": m.Code, "auction": row})
	return nil
}
