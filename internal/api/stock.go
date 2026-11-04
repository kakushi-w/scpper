package api

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/httpx"
	"github.com/scpwikicloud/scpper/internal/stockcfg"
)

type stockParams = stockcfg.Params

type market = stockcfg.Symbol

func (s *Server) stockParams(ctx context.Context) stockParams {
	return stockcfg.GlobalParams(ctx, s.db)
}

func (s *Server) market(r *http.Request) (market, error) {
	code := strings.ToUpper(strings.TrimSpace(httpx.QueryString(r, "symbol", stockcfg.Default)))
	m, err := stockcfg.Load(r.Context(), s.db, code)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, httpx.NotFound("代码为 %s 的指数不存在", code)
	}
	if err != nil {
		return m, err
	}
	if !m.Enabled {
		return m, httpx.NotFound("%s 尚未上市", m.Name)
	}
	return m, nil
}

func (s *Server) handleStockSymbols(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	syms, err := stockcfg.LoadAll(ctx, s.db, true)
	if err != nil {
		return err
	}

	type row struct {
		Code      string     `json:"code"`
		Name      string     `json:"name"`
		ShortName string     `json:"shortName"`
		Source    string     `json:"source"`
		SiteURL   string     `json:"siteUrl,omitempty"`
		ListedAt  *time.Time `json:"listedAt,omitempty"`
		Price     float64    `json:"price"`
		Fair      float64    `json:"fair"`
		DayOpen   float64    `json:"dayOpen"`
		ChangePct float64    `json:"changePct"`
		DayVolume int64      `json:"dayVolume"`
		MyQty     int64      `json:"myQty"`
	}

	var u *auth.User
	if v := auth.FromContext(ctx); v != nil {
		u = v
	}

	items := make([]row, 0, len(syms))
	for _, sym := range syms {
		it := row{
			Code: sym.Code, Name: sym.Name, ShortName: sym.ShortName,
			Source: sym.Source, SiteURL: sym.SiteURL, ListedAt: sym.ListedAt,
		}
		if err := s.db.QueryRow(ctx,
			`SELECT p_ref, p_fair FROM stock_maker WHERE symbol = $1`,
			sym.Code).Scan(&it.Price, &it.Fair); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				continue
			}
			return err
		}
		it.DayOpen = prevClose(ctx, s, sym.Code)
		_ = s.db.QueryRow(ctx, `
			SELECT COALESCE(SUM(qty),0) FROM stock_trades
			WHERE symbol = $1
			  AND at >= date_trunc('day', now() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai'
			  AND side = taker_side`, sym.Code).Scan(&it.DayVolume)
		if it.DayOpen > 0 {
			it.ChangePct = round2((it.Price/it.DayOpen - 1) * 100)
		}
		if u != nil {
			_ = s.db.QueryRow(ctx,
				`SELECT qty FROM stock_positions WHERE user_id = $1 AND symbol = $2`,
				u.ID, sym.Code).Scan(&it.MyQty)
		}
		it.Price, it.Fair, it.DayOpen = round2(it.Price), round2(it.Fair), round2(it.DayOpen)
		items = append(items, it)
	}

	httpx.OK(w, map[string]any{"items": items})
	return nil
}

func prevClose(ctx context.Context, s *Server, symbol string) float64 {
	var v float64
	_ = s.db.QueryRow(ctx, `
		SELECT close FROM stock_ticks
		WHERE symbol = $1
		  AND at < date_trunc('day', now() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai'
		ORDER BY at DESC LIMIT 1`, symbol).Scan(&v)
	return v
}

func (s *Server) handleStockQuote(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	m, err := s.market(r)
	if err != nil {
		return err
	}
	p := m.Params

	var (
		price, pFair float64
		updatedAt    time.Time
	)
	if err := s.db.QueryRow(ctx,
		`SELECT p_ref, p_fair, updated_at FROM stock_maker WHERE symbol = $1`,
		m.Code,
	).Scan(&price, &pFair, &updatedAt); err != nil {
		return err
	}

	base := prevClose(ctx, s, m.Code)
	change := 0.0
	if base > 0 {
		change = (price/base - 1) * 100
	}

	var dayVolume int64
	_ = s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(qty),0) FROM stock_trades
		WHERE symbol = $1 AND side = taker_side
		  AND at >= date_trunc('day', now() AT TIME ZONE 'Asia/Shanghai') AT TIME ZONE 'Asia/Shanghai'`,
		m.Code).Scan(&dayVolume)

	var pendOrders int
	var pendBuy, pendSell int64
	_ = s.db.QueryRow(ctx, `
		SELECT COUNT(*),
		       COALESCE(SUM(qty) FILTER (WHERE side = 'buy'), 0),
		       COALESCE(SUM(qty) FILTER (WHERE side = 'sell'), 0)
		FROM stock_orders
		WHERE symbol = $1 AND status = 'open' AND user_id IS NOT NULL`,
		m.Code).Scan(&pendOrders, &pendBuy, &pendSell)

	now := time.Now()
	out := map[string]any{
		"code":          m.Code,
		"name":          m.Name,
		"shortName":     m.ShortName,
		"source":        m.Source,
		"siteUrl":       m.SiteURL,
		"listedAt":      m.ListedAt,
		"price":         round2(price),
		"fair":          round2(pFair),
		"prevClose":     round2(base),
		"changePct":     round2(change),
		"dayVolume":     dayVolume,
		"floatShares":   p.FloatShares,
		"feePct":        p.FeePct,
		"bandPct":       p.DailyBandPct,
		"maxPosition":   maxPosition(p),
		"impactPct":     p.ImpactPct,
		"updatedAt":     updatedAt,
		"auctionTimes":  p.AuctionTimes,
		"nextAuctionAt": p.NextSlot(now),
		"capPrice":      round2(auctionCap(p, price)),
		"floorPrice":    round2(auctionFloor(p, price)),

		"maxLeverage":    p.MaxLeverage,
		"maintenancePct": p.MaintenanceMarginPct,
		"borrowFeePct":   p.BorrowFeeDailyPct,
		"allowShort":     p.AllowShort,

		"pending": map[string]any{
			"orders":       pendOrders,
			"buyQty":       pendBuy,
			"sellQty":      pendSell,
			"estImpactPct": round2((math.Exp(impactLog(p, pendBuy-pendSell)) - 1) * 100),
		},
	}

	if last, err := scanAuction(s.db.QueryRow(ctx,
		`SELECT `+auctionCols+` FROM stock_auctions WHERE symbol = $1 ORDER BY at DESC LIMIT 1`,
		m.Code)); err == nil {
		list := []auctionRow{last}
		if err := fillEventTitles(ctx, s.db, list); err != nil {
			return err
		}
		out["lastAuction"] = list[0]
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return err
	}

	if u := auth.FromContext(ctx); u != nil {
		var (
			qty      int64
			avgCost  float64
			realized int64
			margin   int64
		)
		_ = s.db.QueryRow(ctx, `
			SELECT qty, avg_cost, realized_pnl, margin
			FROM stock_positions WHERE user_id = $1 AND symbol = $2`,
			u.ID, m.Code).Scan(&qty, &avgCost, &realized, &margin)

		pos := position{Qty: qty, AvgCost: avgCost, Margin: margin}
		me := map[string]any{
			"qty":          qty,
			"avgCost":      round2(avgCost),
			"marketValue":  int64(math.Round(float64(qty) * price)),
			"realizedPnl":  realized,
			"floatingPnl":  pos.unrealized(price),
			"balanceCoins": u.Coins,
			"margin":       margin,
			"equity":       pos.equity(price),
			"notional":     pos.notional(price),
		}
		if qty != 0 {
			me["marginRatio"] = round2(pos.marginRatio(price) * 100)
			me["liqPrice"] = round2(pos.liqPrice(p.MaintenanceMarginPct))
			me["leverage"] = round2(float64(pos.notional(price)) / math.Max(float64(pos.equity(price)), 1))
		}

		var o struct {
			ID        int64     `json:"id"`
			Side      string    `json:"side"`
			Qty       int64     `json:"qty"`
			Leverage  float64   `json:"leverage"`
			Hold      int64     `json:"hold"`
			CreatedAt time.Time `json:"createdAt"`
		}
		if err := s.db.QueryRow(ctx, `
			SELECT id, side, qty, leverage, hold_coins, created_at FROM stock_orders
			WHERE user_id = $1 AND symbol = $2 AND status = 'open'`,
			u.ID, m.Code).Scan(&o.ID, &o.Side, &o.Qty, &o.Leverage, &o.Hold, &o.CreatedAt); err == nil {
			me["pendingOrder"] = o
		}
		out["me"] = me
	}

	httpx.OK(w, out)
	return nil
}

func (s *Server) handleStockCandles(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	m, err := s.market(r)
	if err != nil {
		return err
	}
	span := httpx.QueryString(r, "span", "day")

	var rows pgx.Rows
	if span == "tick" {
		rows, err = s.db.Query(ctx, `
			SELECT at, open, high, low, close, volume
			FROM (SELECT * FROM stock_ticks WHERE symbol = $1 ORDER BY at DESC LIMIT 60) t
			ORDER BY at`, m.Code)
	} else {
		bucket, limit := "day", 120
		switch span {
		case "week":
			bucket, limit = "week", 104
		case "month":
			bucket, limit = "month", 60
		case "year":
			bucket, limit = "year", 20
		}
		rows, err = s.db.Query(ctx, `
			SELECT d, o, h, l, c, v FROM (
				SELECT date_trunc($1, at AT TIME ZONE 'Asia/Shanghai')::date AS d,
				       (array_agg(open  ORDER BY at))[1]                  AS o,
				       MAX(high)                                          AS h,
				       MIN(low)                                           AS l,
				       (array_agg(close ORDER BY at DESC))[1]             AS c,
				       SUM(volume)                                        AS v
				FROM stock_ticks
				WHERE symbol = $3
				GROUP BY 1 ORDER BY 1 DESC LIMIT $2
			) t ORDER BY d`, bucket, limit, m.Code)
	}
	if err != nil {
		return err
	}
	defer rows.Close()

	type candle struct {
		At     any     `json:"at"`
		Open   float64 `json:"open"`
		High   float64 `json:"high"`
		Low    float64 `json:"low"`
		Close  float64 `json:"close"`
		Volume int64   `json:"volume"`
	}
	items := []candle{}
	for rows.Next() {
		var c candle
		if span == "tick" {
			var at time.Time
			if err := rows.Scan(&at, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume); err != nil {
				return err
			}
			c.At = at
		} else {
			var d time.Time
			if err := rows.Scan(&d, &c.Open, &c.High, &c.Low, &c.Close, &c.Volume); err != nil {
				return err
			}
			c.At = d.Format("2006-01-02")
		}
		c.Open, c.High = round2(c.Open), round2(c.High)
		c.Low, c.Close = round2(c.Low), round2(c.Close)
		items = append(items, c)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	httpx.OK(w, map[string]any{"span": span, "items": items, "listedAt": m.ListedAt})
	return nil
}

func (s *Server) handleStockTrades(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.MustUser(ctx)
	pg := httpx.ParsePage(r, 50, 200)

	code := strings.ToUpper(strings.TrimSpace(httpx.QueryString(r, "symbol", "")))

	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM stock_trades
		 WHERE user_id = $1 AND ($2 = '' OR symbol = $2)`, u.ID, code).Scan(&total); err != nil {
		return err
	}

	rows, err := s.db.Query(ctx, `
		SELECT symbol, side, qty, price, gross, fee, at
		FROM stock_trades WHERE user_id = $1 AND ($2 = '' OR symbol = $2)
		ORDER BY id DESC LIMIT $3 OFFSET $4`, u.ID, code, pg.Size, pg.Offset)
	if err != nil {
		return err
	}
	defer rows.Close()

	type tradeRow struct {
		Symbol string    `json:"symbol"`
		Side   string    `json:"side"`
		Qty    int64     `json:"qty"`
		Price  float64   `json:"price"`
		Gross  int64     `json:"gross"`
		Fee    int64     `json:"fee"`
		At     time.Time `json:"at"`
	}
	items := []tradeRow{}
	for rows.Next() {
		var t tradeRow
		if err := rows.Scan(&t.Symbol, &t.Side, &t.Qty, &t.Price, &t.Gross, &t.Fee, &t.At); err != nil {
			return err
		}
		t.Price = round2(t.Price)
		items = append(items, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	httpx.OK(w, map[string]any{"items": items, "meta": httpx.Meta(pg, total)})
	return nil
}

func (s *Server) handleStockPortfolio(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.MustUser(ctx)

	rows, err := s.db.Query(ctx, `
		SELECT p.symbol, y.name, y.short_name, p.qty, p.avg_cost, p.margin,
		       p.locked_qty, p.realized_pnl, k.p_ref, k.p_ref
		FROM stock_positions p
		JOIN stock_symbols y ON y.code = p.symbol
		JOIN stock_maker   k ON k.symbol = p.symbol
		WHERE p.user_id = $1 AND (p.qty <> 0 OR p.realized_pnl <> 0)
		ORDER BY y.sort, y.code`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type holding struct {
		Symbol      string  `json:"symbol"`
		Name        string  `json:"name"`
		ShortName   string  `json:"shortName"`
		Qty         int64   `json:"qty"`
		AvgCost     float64 `json:"avgCost"`
		Price       float64 `json:"price"`
		Margin      int64   `json:"margin"`
		LockedQty   int64   `json:"lockedQty"`
		MarketValue int64   `json:"marketValue"`
		RealizedPnl int64   `json:"realizedPnl"`
		FloatingPnl int64   `json:"floatingPnl"`
		Equity      int64   `json:"equity"`
		MarginRatio float64 `json:"marginRatio"`
		LiqPrice    float64 `json:"liqPrice"`
	}

	p := s.stockParams(ctx)
	items := []holding{}
	var totalRealized, totalFloating, totalMargin int64
	for rows.Next() {
		var (
			h     holding
			pRef  float64
			price float64
		)
		if err := rows.Scan(&h.Symbol, &h.Name, &h.ShortName, &h.Qty, &h.AvgCost,
			&h.Margin, &h.LockedQty, &h.RealizedPnl, &pRef, &price); err != nil {
			return err
		}
		pos := position{Qty: h.Qty, AvgCost: h.AvgCost, Margin: h.Margin, LockedQty: h.LockedQty}
		h.Price = round2(price)
		h.AvgCost = round2(h.AvgCost)
		h.MarketValue = int64(math.Round(float64(h.Qty) * price))
		h.FloatingPnl = pos.unrealized(price)
		h.Equity = pos.equity(price)
		if h.Qty != 0 {
			h.MarginRatio = round2(pos.marginRatio(price) * 100)
			h.LiqPrice = round2(pos.liqPrice(p.MaintenanceMarginPct))
		}
		totalRealized += h.RealizedPnl
		totalFloating += h.FloatingPnl
		totalMargin += h.Margin
		items = append(items, h)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	httpx.OK(w, map[string]any{
		"items": items,
		"total": map[string]any{
			"realizedPnl": totalRealized,
			"floatingPnl": totalFloating,
			"margin":      totalMargin,
			"balance":     u.Coins,
		},
	})
	return nil
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
