package api

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/httpx"
)

const mostTip = 100000

type tipRequest struct {
	PageID int64 `json:"pageId"`
	Amount int64 `json:"amount"`
}

type tipPage struct {
	id          int64
	name        string
	title       string
	authorIDs   []int64
	authorNames []string
}

func (s *Server) tipPage(r *http.Request, pageID int64) (*tipPage, error) {
	var p tipPage
	err := s.db.QueryRow(r.Context(), `
		SELECT id, page, title, author_ids, author_names FROM articles
		WHERE wiki = $1 AND page_id = $2 AND status <> 'deleted'`, s.cfg.PwikitSite, pageID,
	).Scan(&p.id, &p.name, &p.title, &p.authorIDs, &p.authorNames)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, httpx.NotFound("页面不存在或尚未同步")
	}
	return &p, err
}

func (s *Server) handleTip(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	u := auth.MustUser(ctx)
	var req tipRequest
	if err := httpx.DecodeJSON(r, &req); err != nil {
		return err
	}
	if req.Amount < 1 || req.Amount > mostTip {
		return httpx.BadRequest("打赏金额须在 1 到 %d 云币之间", mostTip)
	}
	page, err := s.tipPage(r, req.PageID)
	if err != nil {
		return err
	}
	if len(page.authorIDs) == 0 {
		return httpx.BadRequest("该页面没有作者，无法打赏")
	}
	if slices.Contains(page.authorIDs, u.PwikitID) {
		return httpx.BadRequest("无法打赏自己的页面")
	}

	var balance int64
	shares := map[int64]int64{}
	err = s.db.InSerializableTx(ctx, func(tx pgx.Tx) error {
		recipients := make([]int64, len(page.authorIDs))
		for i, pw := range page.authorIDs {
			if err := tx.QueryRow(ctx, `
				INSERT INTO users (pwikit_id, name) VALUES ($1, $2)
				ON CONFLICT (pwikit_id) DO UPDATE SET name = users.name
				RETURNING id`, pw, page.authorNames[i]).Scan(&recipients[i]); err != nil {
				return err
			}
		}
		var tipID int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO tips (from_user, article_id, amount) VALUES ($1, $2, $3) RETURNING id`,
			u.ID, page.id, req.Amount).Scan(&tipID); err != nil {
			return err
		}
		memo := "打赏：" + page.title
		bal, err := applyCoinDelta(ctx, tx, u.ID, -req.Amount, "tip_out", "tip", &tipID, &memo, nil)
		if errors.Is(err, ErrInsufficientCoins) {
			return httpx.BadRequest("云币不足")
		}
		if err != nil {
			return err
		}
		balance = bal
		share := req.Amount / int64(len(recipients))
		rest := req.Amount - share*int64(len(recipients))
		thanks := "收到打赏：" + page.title + "（来自 " + u.Name + "）"
		for i, id := range recipients {
			amount := share
			if i == 0 {
				amount += rest
			}
			if amount == 0 {
				continue
			}
			shares[page.authorIDs[i]] = amount
			if _, err := applyCoinDelta(ctx, tx, id, amount, "tip_in", "tip", &tipID, &thanks, nil); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for author, amount := range shares {
		s.notify([]int64{author}, "收到打赏",
			fmt.Sprintf("%s 打赏了您的作品《%s》，您获得 %d 云币。", u.Name, page.title, amount), "/"+page.name)
	}
	httpx.OK(w, map[string]any{"balance": balance})
	return nil
}

func (s *Server) handlePageTips(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(strings.TrimSpace(urlParam(r, "pageId")), 10, 64)
	if err != nil {
		return httpx.BadRequest("页面编号无效")
	}
	page, err := s.tipPage(r, id)
	if err != nil {
		return err
	}
	var total, count int64
	if err := s.db.QueryRow(r.Context(),
		`SELECT COALESCE(SUM(amount), 0), COUNT(*) FROM tips WHERE article_id = $1`, page.id,
	).Scan(&total, &count); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"pageId": id, "title": page.title, "authors": page.authorNames, "total": total, "count": count})
	return nil
}
