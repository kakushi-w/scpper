package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/scpwikicloud/scpper/internal/httpx"
)

const badgeShape = `<circle cx="200" cy="210" r="167"/>` +
	`<polygon points="145,18 255,18 273,90 127,90"/>` +
	`<polygon points="145,18 255,18 273,90 127,90" transform="rotate(120 200 210)"/>` +
	`<polygon points="145,18 255,18 273,90 127,90" transform="rotate(240 200 210)"/>`

const badgeFont = `'Helvetica Neue',Arial,'PingFang SC','Microsoft YaHei',sans-serif`

func (s *Server) handleBadge(w http.ResponseWriter, r *http.Request) error {
	raw, ok := strings.CutSuffix(urlParam(r, "file"), ".svg")
	id, err := strconv.ParseInt(raw, 10, 64)
	if !ok || err != nil || id <= 0 {
		return httpx.NotFound("该徽章不存在")
	}
	st, err := s.anyStanding(r, id)
	if err != nil {
		return err
	}
	rank := "#-"
	if st.Rank != nil {
		rank = "#" + strconv.Itoa(*st.Rank)
	}
	w.Header().Set("Content-Type", "image/svg+xml")
	w.Header().Set("Cache-Control", "public, max-age=300")
	_, err = w.Write([]byte(badgeSVG(rank, strconv.FormatInt(st.Coins, 10))))
	return err
}

func badgeSVG(rank, coins string) string {
	return fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="-14 4 428 390" role="img" aria-label="云币财富榜 %[1]s，持有 %[2]s 云币">`+
		`<g fill="#17345f" stroke="#17345f" stroke-width="26" stroke-linejoin="round">%[3]s</g>`+
		`<g fill="#f3f8ff">%[3]s</g>`+
		`<g fill="#17345f" text-anchor="middle" font-family="%[4]s">`+
		`<text x="200" y="186" font-size="%[5]d" font-weight="800">%[1]s</text>`+
		`<text x="200" y="268" font-size="%[6]d" font-weight="700" fill="#2f6fd0">%[2]s</text>`+
		`<text x="200" y="340" font-size="56" font-weight="700" letter-spacing="8">云币</text>`+
		`</g></svg>`, rank, coins, badgeShape, badgeFont, fit(rank, 108, 260), fit(coins, 76, 270))
}

func fit(text string, size, width int) int {
	if n := len(text); n > 0 && float64(size)*0.66*float64(n) > float64(width) {
		return int(float64(width) / (0.66 * float64(n)))
	}
	return size
}
