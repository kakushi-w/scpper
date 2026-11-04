package api

import (
	"net/http"

	"github.com/scpwikicloud/scpper/internal/httpx"
)

func (s *Server) handleAdminGacha(w http.ResponseWriter, r *http.Request) error {
	httpx.OK(w, s.gachaParams(r.Context()))
	return nil
}

func (s *Server) handleAdminGachaParams(w http.ResponseWriter, r *http.Request) error {
	var p gachaParams
	if err := httpx.DecodeJSON(r, &p); err != nil {
		return err
	}
	switch {
	case p.CostSingle < 1 || p.CostTen < 1:
		return httpx.BadRequest("单抽和十连抽的价格至少为 1")
	case p.PityCount < 0:
		return httpx.BadRequest("保底抽数不可为负数")
	case p.TradeFeePct < 0 || p.TradeFeePct > 50:
		return httpx.BadRequest("交易手续费须在 0 到 50 之间")
	}
	total := 0
	for _, rarity := range rarityOrder {
		if p.Rates[rarity] < 0 {
			return httpx.BadRequest("概率权重不可为负数")
		}
		total += p.Rates[rarity]
	}
	if total <= 0 {
		return httpx.BadRequest("概率权重之和须大于 0")
	}
	if rarityRank(p.TenGuarantee) >= len(rarityOrder) {
		p.TenGuarantee = raritySR
	}
	if err := s.putSyncJSON(r.Context(), "gacha.params", p); err != nil {
		return err
	}
	httpx.OK(w, s.gachaParams(r.Context()))
	return nil
}
