package stockcfg

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

type Querier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Params struct {
	FloatShares          int64   `json:"floatShares"`
	BasePrice            float64 `json:"basePrice"`
	ScaleWeight          float64 `json:"scaleWeight"`
	ScaleBase            int64   `json:"scaleBase"`
	MaxLeverage          float64 `json:"maxLeverage"`
	MaintenanceMarginPct float64 `json:"maintenanceMarginPct"`
	BorrowFeeDailyPct    float64 `json:"borrowFeeDailyPct"`
	AllowShort           bool    `json:"allowShort"`
	SigmaDaily           float64 `json:"sigmaDaily"`
	HalfLifeDays         float64 `json:"halfLifeDays"`
	FeePct               float64 `json:"feePct"`
	DailyBandPct         float64 `json:"dailyBandPct"`
	MaxPositionPct       float64 `json:"maxPositionPct"`
	MinPrice             float64 `json:"minPrice"`

	AuctionTimes []string `json:"auctionTimes"`
	ImpactPct    float64  `json:"impactPct"`
	EventPct     float64  `json:"eventPct"`
	EventCapPct  float64  `json:"eventCapPct"`
}

func Defaults() Params {
	return Params{
		FloatShares: 10000, BasePrice: 20, ScaleWeight: 0.3,
		SigmaDaily: 0.020, HalfLifeDays: 3, FeePct: 0.5,
		DailyBandPct: 10, MaxPositionPct: 10, MinPrice: 0.1,
		MaxLeverage: 3, MaintenanceMarginPct: 25, BorrowFeeDailyPct: 0.02, AllowShort: true,
		AuctionTimes: []string{"10:00", "15:00", "21:00"},
		ImpactPct:    15, EventPct: 1.5, EventCapPct: 6,
	}
}

func (p *Params) Clamp() {
	if p.FloatShares <= 0 {
		p.FloatShares = 10000
	}
	if p.BasePrice <= 0 {
		p.BasePrice = 20
	}
	if p.ScaleWeight < 0 || p.ScaleWeight > 1 {
		p.ScaleWeight = 0
	}
	if p.MaxLeverage < 1 {
		p.MaxLeverage = 1
	}
	if p.MaintenanceMarginPct <= 0 || p.MaintenanceMarginPct >= 100 {
		p.MaintenanceMarginPct = 25
	}
	if p.HalfLifeDays <= 0 {
		p.HalfLifeDays = 3
	}
	if p.MinPrice <= 0 {
		p.MinPrice = 0.1
	}
	if len(p.AuctionTimes) == 0 {
		p.AuctionTimes = Defaults().AuctionTimes
	}
	if p.ImpactPct < 0 {
		p.ImpactPct = 0
	}
	if p.EventPct < 0 {
		p.EventPct = 0
	}
	if p.EventCapPct < p.EventPct {
		p.EventCapPct = p.EventPct
	}
}

func (p Params) Slots() []int {
	out := make([]int, 0, len(p.AuctionTimes))
	for _, t := range p.AuctionTimes {
		tm, err := time.Parse("15:04", t)
		if err != nil {
			continue
		}
		out = append(out, tm.Hour()*60+tm.Minute())
	}
	sort.Ints(out)
	return out
}

var beijing = time.FixedZone("CST", 8*3600)

func (p Params) LastSlot(now time.Time) time.Time {
	slots := p.Slots()
	b := now.In(beijing)
	day := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, beijing)
	for d := 0; d < 2; d++ {
		base := day.AddDate(0, 0, -d)
		for i := len(slots) - 1; i >= 0; i-- {
			t := base.Add(time.Duration(slots[i]) * time.Minute)
			if !t.After(now) {
				return t
			}
		}
	}
	return time.Time{}
}

func (p Params) NextSlot(now time.Time) time.Time {
	slots := p.Slots()
	b := now.In(beijing)
	day := time.Date(b.Year(), b.Month(), b.Day(), 0, 0, 0, 0, beijing)
	for d := 0; d < 2; d++ {
		base := day.AddDate(0, 0, d)
		for _, m := range slots {
			t := base.Add(time.Duration(m) * time.Minute)
			if t.After(now) {
				return t
			}
		}
	}
	return time.Time{}
}

type Symbol struct {
	Code      string     `json:"code"`
	Name      string     `json:"name"`
	ShortName string     `json:"shortName"`
	Source    string     `json:"source"`
	SiteURL   string     `json:"siteUrl,omitempty"`
	Enabled   bool       `json:"enabled"`
	ListedAt  *time.Time `json:"listedAt,omitempty"`
	Sort      int        `json:"sort"`
	Params    Params     `json:"params"`
}

func (s Symbol) IsCrom() bool { return s.Source == "crom" }

const Default = "CLOUD"

const symbolCols = `code, name, short_name, source, COALESCE(site_url, ''), enabled, listed_at, sort, params`

func globalParams(ctx context.Context, q Querier) Params {
	p := Defaults()
	var raw []byte
	if err := q.QueryRow(ctx,
		`SELECT value FROM sync_state WHERE key = 'stock.params'`).Scan(&raw); err == nil {
		_ = json.Unmarshal(raw, &p)
	}
	return p
}

func GlobalParams(ctx context.Context, q Querier) Params {
	p := globalParams(ctx, q)
	p.Clamp()
	return p
}

func scanSymbol(row pgx.Row, base Params) (Symbol, error) {
	var (
		s   Symbol
		raw []byte
	)
	if err := row.Scan(&s.Code, &s.Name, &s.ShortName, &s.Source, &s.SiteURL,
		&s.Enabled, &s.ListedAt, &s.Sort, &raw); err != nil {
		return Symbol{}, err
	}

	s.Params = base
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &s.Params)
	}
	s.Params.Clamp()
	return s, nil
}

func Load(ctx context.Context, q Querier, code string) (Symbol, error) {
	if code == "" {
		code = Default
	}
	base := globalParams(ctx, q)
	return scanSymbol(q.QueryRow(ctx,
		`SELECT `+symbolCols+` FROM stock_symbols WHERE code = $1`, code), base)
}

func LoadAll(ctx context.Context, q Querier, onlyEnabled bool) ([]Symbol, error) {
	base := globalParams(ctx, q)
	sql := `SELECT ` + symbolCols + ` FROM stock_symbols`
	if onlyEnabled {
		sql += ` WHERE enabled`
	}
	sql += ` ORDER BY sort, code`

	rows, err := q.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Symbol
	for rows.Next() {
		s, err := scanSymbol(rows, base)
		if err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
