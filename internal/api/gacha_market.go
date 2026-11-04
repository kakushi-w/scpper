package api

import (
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/httpx"
)

const listingsMax = 20

func (s *Server) handleGachaListings(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	pg := httpx.ParsePage(r, 40, 100)
	rarity := httpx.QueryString(r, "rarity", "")
	q := httpx.QueryString(r, "q", "")
	sort := httpx.QueryString(r, "sort", "new")

	b := &argBuilder{}
	where := "l.status = 'open'"
	if rarityRank(rarity) < len(rarityOrder) {
		where += " AND p.rarity = " + b.add(rarity)
	}
	if q != "" {
		ph := b.add("%" + q + "%")
		where += " AND (p.title ILIKE " + ph + " OR p.page ILIKE " + ph + " OR p.author ILIKE " + ph + ")"
	}

	order := "l.id DESC"
	switch sort {
	case "price":
		order = "l.price ASC, l.id DESC"
	case "rarity":
		order = `CASE p.rarity WHEN 'ur' THEN 0 WHEN 'ssr' THEN 1 WHEN 'sr' THEN 2
		                       WHEN 'r' THEN 3 ELSE 4 END, l.price ASC`
	}

	var total int
	if err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM gacha_listings l
		JOIN gacha_pool p ON p.article_id = l.article_id
		WHERE `+where, b.all()...).Scan(&total); err != nil {
		return err
	}

	limitPh := b.add(pg.Size)
	offsetPh := b.add(pg.Offset)
	rows, err := s.db.Query(ctx, `
		SELECT l.id, l.price, l.created_at, u.name,
		       p.article_id, p.wiki, p.page, p.page_id, p.title, p.author, p.rating,
		       p.tags, p.rarity
		FROM gacha_listings l
		JOIN gacha_pool p ON p.article_id = l.article_id
		JOIN users u      ON u.id = l.seller_id
		WHERE `+where+`
		ORDER BY `+order+`
		LIMIT `+limitPh+` OFFSET `+offsetPh, b.all()...)
	if err != nil {
		return err
	}
	defer rows.Close()

	type listing struct {
		ID        int64     `json:"id"`
		Price     int64     `json:"price"`
		CreatedAt time.Time `json:"createdAt"`
		Seller    string    `json:"seller"`
		pulledCard
	}
	items := []listing{}
	for rows.Next() {
		var (
			it     listing
			author *string
		)
		if err := rows.Scan(&it.ID, &it.Price, &it.CreatedAt, &it.Seller,
			&it.ArticleID, &it.Wiki, &it.Page, &it.PageID, &it.Title, &author,
			&it.Rating, &it.Tags, &it.Rarity); err != nil {
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

	httpx.OK(w, map[string]any{
		"items":       items,
		"meta":        httpx.Meta(pg, total),
		"tradeFeePct": s.gachaParams(ctx).TradeFeePct,
	})
	return nil
}

func (s *Server) handleGachaList(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.FromContext(ctx)
	if u == nil {
		return httpx.Unauthorized("请先登录")
	}

	var body struct {
		ArticleID int64 `json:"articleId"`
		Price     int64 `json:"price"`
	}
	if err := httpx.DecodeJSON(r, &body); err != nil {
		return err
	}
	if body.ArticleID <= 0 {
		return httpx.BadRequest("请选择要上架的卡")
	}
	if body.Price <= 0 || body.Price > 1_000_000 {
		return httpx.BadRequest("价格须在 1 到 1000000 云币之间")
	}

	var listingID int64
	err := s.db.InTx(ctx, func(tx pgx.Tx) error {
		var open int
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*) FROM gacha_listings WHERE seller_id = $1 AND status = 'open'`,
			u.ID).Scan(&open); err != nil {
			return err
		}
		if open >= listingsMax {
			return httpx.BadRequest("在售数量已达上限，请先撤下部分卡牌")
		}

		ct, err := tx.Exec(ctx, `
			UPDATE gacha_cards SET qty = qty - 1
			WHERE user_id = $1 AND article_id = $2 AND qty >= 1`, u.ID, body.ArticleID)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 0 {
			return httpx.BadRequest("您没有可上架的该卡牌")
		}

		return tx.QueryRow(ctx, `
			INSERT INTO gacha_listings (seller_id, article_id, price)
			VALUES ($1,$2,$3) RETURNING id`,
			u.ID, body.ArticleID, body.Price).Scan(&listingID)
	})
	if err != nil {
		return err
	}

	httpx.OK(w, map[string]any{"id": listingID, "ok": true})
	return nil
}

func (s *Server) handleGachaUnlist(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.FromContext(ctx)
	if u == nil {
		return httpx.Unauthorized("请先登录")
	}
	id, err := urlParamInt64(r, "id")
	if err != nil {
		return httpx.BadRequest("挂单编号无效")
	}

	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		var articleID int64
		err := tx.QueryRow(ctx, `
			UPDATE gacha_listings SET status = 'cancelled', closed_at = now()
			WHERE id = $1 AND seller_id = $2 AND status = 'open'
			RETURNING article_id`, id, u.ID).Scan(&articleID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.NotFound("该挂单不存在或已结束")
			}
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO gacha_cards (user_id, article_id, qty) VALUES ($1,$2,1)
			ON CONFLICT (user_id, article_id) DO UPDATE SET qty = gacha_cards.qty + 1`,
			u.ID, articleID)
		return err
	})
	if err != nil {
		return err
	}

	httpx.OK(w, map[string]any{"ok": true})
	return nil
}

func (s *Server) handleGachaBuy(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.FromContext(ctx)
	if u == nil {
		return httpx.Unauthorized("请先登录")
	}
	id, err := urlParamInt64(r, "id")
	if err != nil {
		return httpx.BadRequest("挂单编号无效")
	}

	feePct := int64(s.gachaParams(ctx).TradeFeePct)

	var (
		balance   int64
		price     int64
		articleID int64
	)
	err = s.db.InTx(ctx, func(tx pgx.Tx) error {
		var sellerID int64
		err := tx.QueryRow(ctx, `
			SELECT seller_id, article_id, price FROM gacha_listings
			WHERE id = $1 AND status = 'open' FOR UPDATE`, id,
		).Scan(&sellerID, &articleID, &price)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return httpx.NotFound("该挂单已售出或已撤下")
			}
			return err
		}
		if sellerID == u.ID {
			return httpx.BadRequest("无法购买自己上架的卡牌")
		}

		balance, err = applyCoinDelta(ctx, tx, u.ID, -price,
			"gacha_buy", "gacha_listing", &id, nil, nil)
		if err != nil {
			return err
		}

		fee := price * feePct / 100
		if _, err := applyCoinDelta(ctx, tx, sellerID, price-fee,
			"gacha_sell", "gacha_listing", &id, nil, nil); err != nil {
			return err
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO gacha_cards (user_id, article_id, qty) VALUES ($1,$2,1)
			ON CONFLICT (user_id, article_id) DO UPDATE SET qty = gacha_cards.qty + 1`,
			u.ID, articleID); err != nil {
			return err
		}

		_, err = tx.Exec(ctx, `
			UPDATE gacha_listings SET status = 'sold', buyer_id = $2, closed_at = now()
			WHERE id = $1`, id, u.ID)
		return err
	})
	if err != nil {
		if errors.Is(err, ErrInsufficientCoins) {
			return httpx.Err(http.StatusPaymentRequired, "insufficient_coins", "云币不足")
		}
		return err
	}

	httpx.OK(w, map[string]any{"ok": true, "price": price, "balance": balance})
	return nil
}

func (s *Server) handleGachaMyListings(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.FromContext(ctx)
	if u == nil {
		return httpx.Unauthorized("请先登录")
	}

	rows, err := s.db.Query(ctx, `
		SELECT l.id, l.price, l.status, l.created_at, l.closed_at,
		       p.article_id, p.wiki, p.page, p.page_id, p.title, p.author, p.rating,
		       p.tags, p.rarity
		FROM gacha_listings l
		JOIN gacha_pool p ON p.article_id = l.article_id
		WHERE l.seller_id = $1
		ORDER BY (l.status = 'open') DESC, l.id DESC
		LIMIT 100`, u.ID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type myListing struct {
		ID        int64      `json:"id"`
		Price     int64      `json:"price"`
		Status    string     `json:"status"`
		CreatedAt time.Time  `json:"createdAt"`
		ClosedAt  *time.Time `json:"closedAt"`
		pulledCard
	}
	items := []myListing{}
	for rows.Next() {
		var (
			it     myListing
			author *string
		)
		if err := rows.Scan(&it.ID, &it.Price, &it.Status, &it.CreatedAt, &it.ClosedAt,
			&it.ArticleID, &it.Wiki, &it.Page, &it.PageID, &it.Title, &author,
			&it.Rating, &it.Tags, &it.Rarity); err != nil {
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

	httpx.OK(w, map[string]any{"items": items})
	return nil
}
