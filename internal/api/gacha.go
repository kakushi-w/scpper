package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/httpx"
)

const (
	rarityUR  = "ur"
	raritySSR = "ssr"
	raritySR  = "sr"
	rarityR   = "r"
	rarityN   = "n"
)

var rarityOrder = []string{rarityUR, raritySSR, raritySR, rarityR, rarityN}

func rarityRank(r string) int {
	for i, v := range rarityOrder {
		if v == r {
			return i
		}
	}
	return len(rarityOrder)
}

type gachaParams struct {
	CostSingle   int64          `json:"costSingle"`
	CostTen      int64          `json:"costTen"`
	Rates        map[string]int `json:"rates"`
	TenGuarantee string         `json:"tenGuarantee"`
	PityCount    int            `json:"pityCount"`
	TradeFeePct  int            `json:"tradeFeePct"`
}

func defaultGachaParams() gachaParams {
	return gachaParams{
		CostSingle:   10,
		CostTen:      90,
		Rates:        map[string]int{rarityUR: 150, raritySSR: 400, raritySR: 1200, rarityR: 3000, rarityN: 5250},
		TenGuarantee: raritySR,
		PityCount:    60,
		TradeFeePct:  5,
	}
}

func (s *Server) gachaParams(ctx context.Context) gachaParams {
	p := defaultGachaParams()
	var raw []byte
	if err := s.db.QueryRow(ctx,
		`SELECT value FROM sync_state WHERE key = 'gacha.params'`).Scan(&raw); err == nil {
		_ = json.Unmarshal(raw, &p)
	}
	if p.CostSingle <= 0 || p.CostTen <= 0 || len(p.Rates) == 0 {
		return defaultGachaParams()
	}
	return p
}

func randInt(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("invalid random range %d", n)
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		return 0, err
	}
	return int(v.Int64()), nil
}

func rollRarity(p gachaParams, minRarity string) (string, error) {
	limit := rarityRank(minRarity)
	total := 0
	for _, r := range rarityOrder {
		if rarityRank(r) <= limit {
			total += p.Rates[r]
		}
	}
	if total <= 0 {
		return raritySSR, nil
	}
	roll, err := randInt(total)
	if err != nil {
		return "", err
	}
	acc := 0
	for _, r := range rarityOrder {
		if rarityRank(r) > limit {
			continue
		}
		acc += p.Rates[r]
		if roll < acc {
			return r, nil
		}
	}
	return rarityN, nil
}

type pulledCard struct {
	ArticleID int64    `json:"articleId"`
	Wiki      string   `json:"wiki"`
	Page      string   `json:"page"`
	PageID    *int64   `json:"pageId"`
	Title     string   `json:"title"`
	Author    string   `json:"author"`
	Rating    float64  `json:"rating"`
	Tags      []string `json:"tags"`
	Rarity    string   `json:"rarity"`
	ByPity    bool     `json:"byPity"`
	IsNew     bool     `json:"isNew"`
}

func (s *Server) handleGachaPull(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.FromContext(ctx)
	if u == nil {
		return httpx.Unauthorized("请先登录")
	}

	var body struct {
		Batch string `json:"batch"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		return err
	}

	p := s.gachaParams(ctx)
	count, cost := 1, p.CostSingle
	if body.Batch == "ten" {
		count, cost = 10, p.CostTen
	} else {
		body.Batch = "single"
	}

	var (
		results []pulledCard
		balance int64
	)
	err := s.db.InTx(ctx, func(tx pgx.Tx) error {
		var err error
		balance, err = applyCoinDelta(ctx, tx, u.ID, -cost,
			"gacha_pull", "gacha", nil, nil, nil)
		if err != nil {
			return err
		}

		var sinceSSR int
		err = tx.QueryRow(ctx, `
			INSERT INTO gacha_state (user_id) VALUES ($1)
			ON CONFLICT (user_id) DO UPDATE SET user_id = EXCLUDED.user_id
			RETURNING since_ssr`, u.ID).Scan(&sinceSSR)
		if err != nil {
			return err
		}

		bestRank := len(rarityOrder)
		for i := 0; i < count; i++ {
			minRarity := rarityN
			byPity := false

			if p.PityCount > 0 && sinceSSR+1 >= p.PityCount {
				minRarity, byPity = raritySSR, true
			}
			if count == 10 && i == count-1 && rarityRank(p.TenGuarantee) < bestRank {
				if rarityRank(p.TenGuarantee) < rarityRank(minRarity) {
					minRarity, byPity = p.TenGuarantee, true
				}
			}

			rarity, err := rollRarity(p, minRarity)
			if err != nil {
				return err
			}

			card, err := drawCard(ctx, tx, rarity)
			if err != nil {
				return err
			}
			card.ByPity = byPity

			if rarityRank(rarity) < bestRank {
				bestRank = rarityRank(rarity)
			}
			if rarityRank(rarity) <= rarityRank(raritySSR) {
				sinceSSR = 0
			} else {
				sinceSSR++
			}

			isNew, err := grantCard(ctx, tx, u.ID, card.ArticleID)
			if err != nil {
				return err
			}
			card.IsNew = isNew

			if _, err := tx.Exec(ctx, `
				INSERT INTO gacha_pulls (user_id, article_id, rarity, by_pity, batch)
				VALUES ($1,$2,$3,$4,$5)`,
				u.ID, card.ArticleID, rarity, byPity, body.Batch); err != nil {
				return err
			}
			results = append(results, *card)
		}

		_, err = tx.Exec(ctx, `
			UPDATE gacha_state
			SET total_pulls = total_pulls + $2, since_ssr = $3, updated_at = now()
			WHERE user_id = $1`, u.ID, count, sinceSSR)
		return err
	})
	if err != nil {
		if errors.Is(err, ErrInsufficientCoins) {
			return httpx.Err(http.StatusPaymentRequired, "insufficient_coins",
				"云币不足，本次需要 %d 云币", cost)
		}
		return err
	}

	httpx.OK(w, map[string]any{
		"cards":   results,
		"cost":    cost,
		"balance": balance,
	})
	return nil
}

func drawCard(ctx context.Context, tx pgx.Tx, rarity string) (*pulledCard, error) {
	var n int
	if err := tx.QueryRow(ctx,
		`SELECT COUNT(*) FROM gacha_pool WHERE rarity = $1`, rarity).Scan(&n); err != nil {
		return nil, err
	}
	if n == 0 {
		if rarity == rarityN {
			return nil, fmt.Errorf("card pool is empty")
		}
		return drawCard(ctx, tx, rarityN)
	}

	off, err := randInt(n)
	if err != nil {
		return nil, err
	}

	var c pulledCard
	var author *string
	err = tx.QueryRow(ctx, `
		SELECT article_id, wiki, page, page_id, title, author, rating, tags, rarity
		FROM gacha_pool WHERE rarity = $1
		ORDER BY article_id
		LIMIT 1 OFFSET $2`, rarity, off,
	).Scan(&c.ArticleID, &c.Wiki, &c.Page, &c.PageID, &c.Title, &author,
		&c.Rating, &c.Tags, &c.Rarity)
	if err != nil {
		return nil, err
	}
	if author != nil {
		c.Author = *author
	}
	c.Tags = nonNil(c.Tags)
	return &c, nil
}

func grantCard(ctx context.Context, tx pgx.Tx, userID, articleID int64) (bool, error) {
	var qty int
	err := tx.QueryRow(ctx, `
		INSERT INTO gacha_cards (user_id, article_id, qty) VALUES ($1,$2,1)
		ON CONFLICT (user_id, article_id) DO UPDATE SET qty = gacha_cards.qty + 1
		RETURNING qty`, userID, articleID).Scan(&qty)
	return qty == 1, err
}

func (s *Server) handleGachaInfo(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	p := s.gachaParams(ctx)

	rows, err := s.db.Query(ctx,
		`SELECT rarity, COUNT(*)::int FROM gacha_pool GROUP BY rarity`)
	if err != nil {
		return err
	}
	defer rows.Close()
	pool := map[string]int{}
	for rows.Next() {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return err
		}
		pool[k] = n
	}
	if err := rows.Err(); err != nil {
		return err
	}

	tiers := make([]map[string]any, 0, len(rarityOrder))
	for _, k := range rarityOrder {
		tiers = append(tiers, map[string]any{
			"rarity": k,
			"rate":   float64(p.Rates[k]) / 100,
			"pool":   pool[k],
		})
	}

	out := map[string]any{
		"costSingle":   p.CostSingle,
		"costTen":      p.CostTen,
		"pityCount":    p.PityCount,
		"tenGuarantee": p.TenGuarantee,
		"tradeFeePct":  p.TradeFeePct,
		"tiers":        tiers,
	}

	if u := auth.FromContext(ctx); u != nil {
		var totalPulls int64
		var sinceSSR int
		_ = s.db.QueryRow(ctx,
			`SELECT total_pulls, since_ssr FROM gacha_state WHERE user_id = $1`,
			u.ID).Scan(&totalPulls, &sinceSSR)
		var owned, distinct int
		_ = s.db.QueryRow(ctx, `
			SELECT COALESCE(SUM(qty),0)::int, COUNT(*)::int
			FROM gacha_cards WHERE user_id = $1 AND qty > 0`, u.ID).Scan(&owned, &distinct)
		out["me"] = map[string]any{
			"totalPulls":   totalPulls,
			"sinceSSR":     sinceSSR,
			"pityIn":       max(0, p.PityCount-sinceSSR),
			"ownedTotal":   owned,
			"ownedKinds":   distinct,
			"poolKinds":    pool[rarityUR] + pool[raritySSR] + pool[raritySR] + pool[rarityR] + pool[rarityN],
			"balanceCoins": u.Coins,
		}
	}

	httpx.OK(w, out)
	return nil
}

func (s *Server) handleGachaCollection(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.FromContext(ctx)
	if u == nil {
		return httpx.Unauthorized("请先登录")
	}
	pg := httpx.ParsePage(r, 60, 200)
	rarity := httpx.QueryString(r, "rarity", "")

	b := &argBuilder{}
	where := "c.user_id = " + b.add(u.ID) + " AND c.qty > 0"
	if rarityRank(rarity) < len(rarityOrder) {
		where += " AND p.rarity = " + b.add(rarity)
	}

	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM gacha_cards c JOIN gacha_pool p ON p.article_id = c.article_id
		 WHERE `+where, b.all()...).Scan(&total); err != nil {
		return err
	}

	limitPh := b.add(pg.Size)
	offsetPh := b.add(pg.Offset)
	rows, err := s.db.Query(ctx, `
		SELECT p.article_id, p.wiki, p.page, p.page_id, p.title, p.author, p.rating,
		       p.tags, p.rarity, c.qty, c.first_at
		FROM gacha_cards c
		JOIN gacha_pool p ON p.article_id = c.article_id
		WHERE `+where+`
		ORDER BY CASE p.rarity
		           WHEN 'ur' THEN 0 WHEN 'ssr' THEN 1 WHEN 'sr' THEN 2
		           WHEN 'r' THEN 3 ELSE 4 END,
		         abs(p.rating) DESC, p.article_id
		LIMIT `+limitPh+` OFFSET `+offsetPh, b.all()...)
	if err != nil {
		return err
	}
	defer rows.Close()

	type ownedCard struct {
		pulledCard
		Qty     int       `json:"qty"`
		FirstAt time.Time `json:"firstAt"`
	}
	items := []ownedCard{}
	for rows.Next() {
		var (
			it     ownedCard
			author *string
		)
		if err := rows.Scan(&it.ArticleID, &it.Wiki, &it.Page, &it.PageID, &it.Title,
			&author, &it.Rating, &it.Tags, &it.Rarity, &it.Qty, &it.FirstAt); err != nil {
			return err
		}
		if author != nil {
			it.Author = *author
		}
		it.Tags = nonNil(it.Tags)
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	httpx.OK(w, map[string]any{"items": items, "meta": httpx.Meta(pg, total)})
	return nil
}

func (s *Server) handleGachaHistory(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.FromContext(ctx)
	if u == nil {
		return httpx.Unauthorized("请先登录")
	}
	pg := httpx.ParsePage(r, 50, 200)

	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM gacha_pulls WHERE user_id = $1`, u.ID).Scan(&total); err != nil {
		return err
	}

	rows, err := s.db.Query(ctx, `
		SELECT g.rarity, g.by_pity, g.batch, g.created_at,
		       a.wiki, a.page, a.page_id, a.title, COALESCE(a.author,''), a.rating
		FROM gacha_pulls g
		JOIN articles a ON a.id = g.article_id
		WHERE g.user_id = $1
		ORDER BY g.id DESC
		LIMIT $2 OFFSET $3`, u.ID, pg.Size, pg.Offset)
	if err != nil {
		return err
	}
	defer rows.Close()

	type histRow struct {
		Rarity    string    `json:"rarity"`
		ByPity    bool      `json:"byPity"`
		Batch     string    `json:"batch"`
		CreatedAt time.Time `json:"createdAt"`
		Wiki      string    `json:"wiki"`
		Page      string    `json:"page"`
		PageID    *int64    `json:"pageId"`
		Title     string    `json:"title"`
		Author    string    `json:"author"`
		Rating    float64   `json:"rating"`
	}
	items := []histRow{}
	for rows.Next() {
		var it histRow
		if err := rows.Scan(&it.Rarity, &it.ByPity, &it.Batch, &it.CreatedAt,
			&it.Wiki, &it.Page, &it.PageID, &it.Title, &it.Author, &it.Rating); err != nil {
			return err
		}
		items = append(items, it)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	httpx.OK(w, map[string]any{"items": items, "meta": httpx.Meta(pg, total)})
	return nil
}
