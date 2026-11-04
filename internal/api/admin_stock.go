package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"math"
	"net/http"
	"strings"
	"time"

	"github.com/scpwikicloud/scpper/internal/httpx"
	"github.com/scpwikicloud/scpper/internal/stockcfg"
)

type adminStockState struct {
	Symbol  string      `json:"symbol"`
	Name    string      `json:"name"`
	Source  string      `json:"source"`
	Params  stockParams `json:"params"`
	Overlay stockParams `json:"overlay"`

	Symbols []adminSymbolRow `json:"symbols"`

	Book struct {
		Inventory      int64   `json:"inventory"`
		Cash           int64   `json:"cash"`
		Reserve        int64   `json:"reserve"`
		InventoryValue int64   `json:"inventoryValue"`
		NetWorth       int64   `json:"netWorth"`
		PRef           float64 `json:"pRef"`
		PFair          float64 `json:"pFair"`
	} `json:"book"`

	Players struct {
		Holders     int   `json:"holders"`
		SharesHeld  int64 `json:"sharesHeld"`
		RealizedPnl int64 `json:"realizedPnl"`
		FloatingPnl int64 `json:"floatingPnl"`
		FeesPaid    int64 `json:"feesPaid"`
		NetTransfer int64 `json:"netTransfer"`
		TurnoverDay int64 `json:"turnoverDay"`
		OpenOrders  int   `json:"openOrders"`
	} `json:"players"`

	Shares struct {
		Positions int64 `json:"positions"`
		Maker     int64 `json:"maker"`
		Bots      int64 `json:"bots"`
		Total     int64 `json:"total"`
		Float     int64 `json:"float"`
		Drift     int64 `json:"drift"`
	} `json:"shares"`

	Top []adminStockPlayer `json:"top"`

	Auctions []auctionRow `json:"auctions"`
}

type adminSymbolRow struct {
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	ShortName string     `json:"shortName"`
	Source    string     `json:"source"`
	SiteURL   string     `json:"siteUrl,omitempty"`
	Enabled   bool       `json:"enabled"`
	ListedAt  *time.Time `json:"listedAt,omitempty"`
	Price     float64    `json:"price"`
	Fair      float64    `json:"fair"`
	ScaleBase int64      `json:"scaleBase"`

	MirrorPages   int        `json:"mirrorPages"`
	LastPageAt    *time.Time `json:"lastPageAt,omitempty"`
	LastVoteAt    *time.Time `json:"lastVoteAt,omitempty"`
	SweptAt       *time.Time `json:"sweptAt,omitempty"`
	UpstreamStale bool       `json:"upstreamStale"`
}

type adminStockPlayer struct {
	UserID      int64   `json:"userId"`
	Name        string  `json:"name"`
	Qty         int64   `json:"qty"`
	AvgCost     float64 `json:"avgCost"`
	RealizedPnl int64   `json:"realizedPnl"`
	FloatingPnl int64   `json:"floatingPnl"`
	TotalPnl    int64   `json:"totalPnl"`
}

func (s *Server) adminMarket(r *http.Request) (market, error) {
	code := strings.ToUpper(strings.TrimSpace(httpx.QueryString(r, "symbol", stockcfg.Default)))
	m, err := stockcfg.Load(r.Context(), s.db, code)
	if err != nil {
		return m, httpx.NotFound("代码为 %s 的指数不存在", code)
	}
	return m, nil
}

func (s *Server) adminSymbolRows(ctx context.Context) ([]adminSymbolRow, error) {
	rows, err := s.db.Query(ctx, `
		SELECT y.code, y.name, y.short_name, y.source, COALESCE(y.site_url, ''),
		       y.enabled, y.listed_at,
		       COALESCE(k.p_ref, 0), COALESCE(k.p_fair, 0),
		       COALESCE((y.params->>'scaleBase')::bigint, 0),
		       COALESCE(c.page_count, 0), c.last_page_at, c.last_vote_at, c.swept_at
		FROM stock_symbols y
		LEFT JOIN stock_maker k ON k.symbol   = y.code
		LEFT JOIN crom_sites  c ON c.site_url = y.site_url
		ORDER BY y.sort, y.code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []adminSymbolRow{}
	for rows.Next() {
		var v adminSymbolRow
		if err := rows.Scan(&v.Code, &v.Name, &v.ShortName, &v.Source, &v.SiteURL,
			&v.Enabled, &v.ListedAt, &v.Price, &v.Fair, &v.ScaleBase,
			&v.MirrorPages, &v.LastPageAt, &v.LastVoteAt, &v.SweptAt); err != nil {
			return nil, err
		}
		v.Price, v.Fair = round2(v.Price), round2(v.Fair)

		if v.Source == "crom" {
			v.UpstreamStale = v.LastVoteAt == nil ||
				time.Since(*v.LastVoteAt) > 5*24*time.Hour
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Server) handleAdminStock(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	m, err := s.adminMarket(r)
	if err != nil {
		return err
	}

	st := adminStockState{
		Symbol:  m.Code,
		Name:    m.Name,
		Source:  m.Source,
		Params:  s.stockParams(ctx),
		Overlay: m.Params,
	}

	if st.Symbols, err = s.adminSymbolRows(ctx); err != nil {
		return err
	}

	if err := s.db.QueryRow(ctx,
		`SELECT inventory, cash, reserve, p_ref, p_fair FROM stock_maker WHERE symbol = $1`, m.Code,
	).Scan(&st.Book.Inventory, &st.Book.Cash, &st.Book.Reserve,
		&st.Book.PRef, &st.Book.PFair); err != nil {
		if !m.Enabled {
			httpx.OK(w, st)
			return nil
		}
		return err
	}

	st.Book.InventoryValue = int64(math.Round(float64(st.Book.Inventory) * st.Book.PRef))
	st.Book.NetWorth = st.Book.Cash + st.Book.InventoryValue + st.Book.Reserve

	price := st.Book.PRef
	if err := s.db.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE qty > 0),
			COALESCE(SUM(qty), 0),
			COALESCE(SUM(realized_pnl), 0),
			COALESCE(SUM(ROUND(qty * ($1::numeric - avg_cost))), 0)
		FROM stock_positions WHERE symbol = $2`, price, m.Code,
	).Scan(&st.Players.Holders, &st.Players.SharesHeld,
		&st.Players.RealizedPnl, &st.Players.FloatingPnl); err != nil {
		return err
	}

	if err := s.db.QueryRow(ctx, `
		SELECT
			COALESCE(SUM(fee) FILTER (WHERE user_id IS NOT NULL), 0),
			COALESCE(SUM(qty) FILTER (
				WHERE at >= date_trunc('day', now()) AND side = taker_side), 0),
			(SELECT COUNT(*) FROM stock_orders
			  WHERE symbol = $1 AND status = 'open' AND user_id IS NOT NULL)
		FROM stock_trades WHERE symbol = $1`, m.Code,
	).Scan(&st.Players.FeesPaid, &st.Players.TurnoverDay, &st.Players.OpenOrders); err != nil {
		return err
	}
	st.Players.NetTransfer = st.Players.RealizedPnl + st.Players.FloatingPnl

	if err := s.db.QueryRow(ctx, `
		SELECT
			COALESCE((SELECT SUM(qty)       FROM stock_positions WHERE symbol = $1), 0),
			COALESCE((SELECT inventory      FROM stock_maker     WHERE symbol = $1), 0),
			COALESCE((SELECT SUM(inventory) FROM stock_bots      WHERE symbol = $1), 0)`,
		m.Code,
	).Scan(&st.Shares.Positions, &st.Shares.Maker, &st.Shares.Bots); err != nil {
		return err
	}
	st.Shares.Total = st.Shares.Positions + st.Shares.Maker + st.Shares.Bots
	st.Shares.Float = m.Params.FloatShares
	st.Shares.Drift = st.Shares.Total - st.Shares.Float

	rows, err := s.db.Query(ctx, `
		SELECT p.user_id, u.name, p.qty, p.avg_cost, p.realized_pnl,
		       ROUND(p.qty * ($1::numeric - p.avg_cost))::bigint AS floating
		FROM stock_positions p
		JOIN users u ON u.id = p.user_id
		WHERE p.symbol = $2
		ORDER BY p.realized_pnl + ROUND(p.qty * ($1::numeric - p.avg_cost)) DESC
		LIMIT 20`, price, m.Code)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var p adminStockPlayer
		if err := rows.Scan(&p.UserID, &p.Name, &p.Qty, &p.AvgCost,
			&p.RealizedPnl, &p.FloatingPnl); err != nil {
			return err
		}
		p.TotalPnl = p.RealizedPnl + p.FloatingPnl
		st.Top = append(st.Top, p)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if st.Top == nil {
		st.Top = []adminStockPlayer{}
	}

	auctionRows, err := s.db.Query(ctx,
		`SELECT `+auctionCols+` FROM stock_auctions WHERE symbol = $1 ORDER BY at DESC LIMIT 30`, m.Code)
	if err != nil {
		return err
	}
	defer auctionRows.Close()
	st.Auctions = []auctionRow{}
	for auctionRows.Next() {
		a, err := scanAuction(auctionRows)
		if err != nil {
			return err
		}
		st.Auctions = append(st.Auctions, a)
	}
	if err := auctionRows.Err(); err != nil {
		return err
	}
	auctionRows.Close()
	if err := fillEventTitles(ctx, s.db, st.Auctions); err != nil {
		return err
	}

	httpx.OK(w, st)
	return nil
}

func (s *Server) handleAdminStockParams(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()

	cur := s.stockParams(ctx)
	if err := httpx.DecodeJSON(r, &cur); err != nil {
		return err
	}
	if err := validateStockParams(cur); err != nil {
		return err
	}

	if err := s.putSyncJSON(ctx, "stock.params", cur); err != nil {
		return err
	}
	httpx.OK(w, cur)
	return nil
}

func validateStockParams(cur stockParams) error {
	switch {
	case cur.FloatShares <= 0:
		return httpx.BadRequest("总股本须大于 0")
	case cur.FeePct < 0 || cur.FeePct > 10:
		return httpx.BadRequestMsg("手续费须在 0% 到 10% 之间")
	case cur.DailyBandPct <= 0 || cur.DailyBandPct > 50:
		return httpx.BadRequestMsg("单场涨跌幅须在 0% 到 50% 之间")
	case cur.MaxPositionPct <= 0 || cur.MaxPositionPct > 100:
		return httpx.BadRequestMsg("持仓上限须在 0% 到 100% 之间")
	case cur.MinPrice <= 0:
		return httpx.BadRequest("最低价须大于 0")
	case cur.BasePrice <= 0 || cur.BasePrice > 10000:
		return httpx.BadRequest("基准价须在 0 到 10000 之间")
	case cur.ScaleWeight < 0 || cur.ScaleWeight > 1:
		return httpx.BadRequest("规模权重须在 0 到 1 之间")
	case cur.ScaleBase < 0:
		return httpx.BadRequest("规模基准不可为负数")
	case cur.SigmaDaily < 0 || cur.SigmaDaily > 0.5:
		return httpx.BadRequestMsg("日波动率须在 0 到 0.5 之间")
	case cur.HalfLifeDays <= 0 || cur.HalfLifeDays > 365:
		return httpx.BadRequest("回归半衰期须在 0 到 365 天之间")
	case cur.ImpactPct < 0 || cur.ImpactPct > 100:
		return httpx.BadRequestMsg("满仓推动幅度须在 0% 到 100% 之间")
	case cur.EventPct < 0 || cur.EventPct > 20:
		return httpx.BadRequestMsg("站点动态幅度须在 0% 到 20% 之间")
	case cur.EventCapPct < cur.EventPct || cur.EventCapPct > 50:
		return httpx.BadRequestMsg("站点动态上限须不小于站点动态幅度，且不超过 50%")
	case len(cur.Slots()) == 0 || len(cur.Slots()) != len(cur.AuctionTimes):
		return httpx.BadRequestMsg("竞价时间须为 HH:MM 格式，且至少设置一场")
	}
	return nil
}

type symbolParamsReq struct {
	BasePrice   *float64 `json:"basePrice"`
	ScaleBase   *int64   `json:"scaleBase"`
	FloatShares *int64   `json:"floatShares"`
	SigmaDaily  *float64 `json:"sigmaDaily"`
}

func (s *Server) handleAdminStockSymbolParams(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	code := strings.ToUpper(strings.TrimSpace(urlParam(r, "code")))

	m, err := stockcfg.Load(ctx, s.db, code)
	if err != nil {
		return httpx.NotFound("代码为 %s 的指数不存在", code)
	}

	var req symbolParamsReq
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}

	if m.Enabled {
		if req.ScaleBase != nil && *req.ScaleBase != m.Params.ScaleBase {
			return httpx.BadRequest("%s 已上市，规模基准不可更改", code)
		}
		if req.FloatShares != nil && *req.FloatShares != m.Params.FloatShares {
			return httpx.BadRequest("%s 已上市，总股本不可更改", code)
		}
	}

	next := m.Params
	over := map[string]any{}
	if req.BasePrice != nil {
		next.BasePrice = *req.BasePrice
		over["basePrice"] = *req.BasePrice
	}
	if req.ScaleBase != nil {
		next.ScaleBase = *req.ScaleBase
		over["scaleBase"] = *req.ScaleBase
	}
	if req.FloatShares != nil {
		next.FloatShares = *req.FloatShares
		over["floatShares"] = *req.FloatShares
	}
	if req.SigmaDaily != nil {
		next.SigmaDaily = *req.SigmaDaily
		over["sigmaDaily"] = *req.SigmaDaily
	}
	if len(over) == 0 {
		httpx.OK(w, m)
		return nil
	}
	if err := validateStockParams(next); err != nil {
		return err
	}

	raw, err := json.Marshal(over)
	if err != nil {
		return err
	}
	if _, err := s.db.Exec(ctx,
		`UPDATE stock_symbols SET params = params || $2::jsonb WHERE code = $1`,
		code, raw); err != nil {
		return err
	}

	out, err := stockcfg.Load(ctx, s.db, code)
	if err != nil {
		return err
	}
	httpx.OK(w, out)
	return nil
}

func (s *Server) handleAdminStockList(w http.ResponseWriter, r *http.Request) error {
	code := strings.ToUpper(strings.TrimSpace(urlParam(r, "code")))
	if err := s.queueSync(r.Context(), "stock-list", code, ""); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"queued": "stock-list", "symbol": code})
	return nil
}

func (s *Server) handleAdminStockDelist(w http.ResponseWriter, r *http.Request) error {
	code := strings.ToUpper(strings.TrimSpace(urlParam(r, "code")))
	if code == stockcfg.Default {
		return httpx.BadRequest("云国指数不可停牌")
	}
	if err := s.queueSync(r.Context(), "stock-delist", code, ""); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"queued": "stock-delist", "symbol": code})
	return nil
}

func (s *Server) handleAdminCromBackfill(w http.ResponseWriter, r *http.Request) error {
	site := strings.TrimSpace(httpx.QueryString(r, "site", ""))
	if site == "" {
		return httpx.BadRequest("请指定分站地址")
	}
	if err := s.queueSync(r.Context(), "crom-backfill", "", site); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"queued": "crom-backfill", "site": site})
	return nil
}

func (s *Server) handleAdminCromFull(w http.ResponseWriter, r *http.Request) error {
	site := strings.TrimSpace(httpx.QueryString(r, "site", ""))
	if site == "" {
		return httpx.BadRequest("请指定分站地址")
	}
	if err := s.queueSync(r.Context(), "crom-full", "", site); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"queued": "crom-full", "site": site})
	return nil
}

func (s *Server) handleAdminStockHistory(w http.ResponseWriter, r *http.Request) error {
	code := strings.ToUpper(strings.TrimSpace(urlParam(r, "code")))
	if err := s.queueSync(r.Context(), "stock-history", code, ""); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"queued": "stock-history", "symbol": code})
	return nil
}

func (s *Server) handleAdminStockJobs(w http.ResponseWriter, _ *http.Request) error {
	httpx.OK(w, map[string]any{"items": s.engine.Jobs()})
	return nil
}

func (s *Server) queueSync(_ context.Context, kind, symbol, site string) error {
	target := symbol
	if site != "" {
		target = site
	}
	if s.engine.JobRunning(kind, target) {
		return httpx.Conflict("该任务正在进行中")
	}
	go func() {
		ctx := context.Background()
		var err error
		switch kind {
		case "stock-list":
			err = s.engine.ListSymbol(ctx, symbol, 0, time.Now())
		case "stock-delist":
			err = s.engine.DelistSymbol(ctx, symbol)
		case "crom-backfill":
			_, err = s.engine.BackfillCrom(ctx, site)
		case "crom-full":
			_, err = s.engine.SyncCromFull(ctx, site)
		case "stock-history":
			err = s.engine.RedrawHistory(ctx, symbol)
		}
		if err != nil {
			slog.Error("admin job", "kind", kind, "symbol", symbol, "site", site, "err", err)
		} else {
			slog.Info("admin job done", "kind", kind, "symbol", symbol, "site", site)
		}
	}()
	return nil
}

func (s *Server) putSyncJSON(ctx context.Context, key string, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(ctx, `
		INSERT INTO sync_state (key, value, updated_at)
		VALUES ($1, $2::jsonb, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`,
		key, raw)
	return err
}
