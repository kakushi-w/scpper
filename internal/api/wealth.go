package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/httpx"
)

type wealthRow struct {
	Rank      int    `json:"rank"`
	PwikitID  int64  `json:"pwikitId"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatarUrl"`
	Coins     int64  `json:"coins"`
}

func (s *Server) handleRankWealth(w http.ResponseWriter, r *http.Request) error {
	ctx := r.Context()
	pg := httpx.ParsePage(r, 50, 250)
	var total int
	if err := s.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM users WHERE banned_at IS NULL`).Scan(&total); err != nil {
		return err
	}
	rows, err := s.db.Query(ctx, `
		SELECT row_number() OVER (ORDER BY coins DESC, lower(name), pwikit_id)::int, pwikit_id, name,
		       coalesce(avatar_url, ''), coins
		FROM users
		WHERE banned_at IS NULL
		ORDER BY coins DESC, lower(name), pwikit_id
		LIMIT $1 OFFSET $2`, pg.Size, pg.Offset)
	if err != nil {
		return err
	}
	defer rows.Close()
	items := []wealthRow{}
	for rows.Next() {
		var row wealthRow
		if err := rows.Scan(&row.Rank, &row.PwikitID, &row.Name, &row.AvatarURL, &row.Coins); err != nil {
			return err
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"items": items, "meta": httpx.Meta(pg, total)})
	return nil
}

type standing struct {
	PwikitID int64  `json:"pwikitId"`
	Name     string `json:"name"`
	Coins    int64  `json:"coins"`
	Rank     *int   `json:"rank"`
	Ranked   int    `json:"ranked"`
}

func (s *Server) standingOf(r *http.Request, pwikitID int64) (*standing, error) {
	var st standing
	err := s.db.QueryRow(r.Context(), `
		SELECT u.pwikit_id, u.name, u.coins,
		       CASE WHEN u.banned_at IS NULL THEN
		            (SELECT COUNT(*) + 1 FROM users o WHERE o.banned_at IS NULL
		              AND (o.coins > u.coins OR (o.coins = u.coins
		                   AND (lower(o.name), o.pwikit_id) < (lower(u.name), u.pwikit_id))))::int END,
		       (SELECT COUNT(*) FROM users WHERE banned_at IS NULL)::int
		FROM users u WHERE u.pwikit_id = $1`, pwikitID,
	).Scan(&st.PwikitID, &st.Name, &st.Coins, &st.Rank, &st.Ranked)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	return &st, err
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) error {
	u := auth.FromContext(r.Context())
	if u == nil {
		httpx.OK(w, map[string]any{"user": nil})
		return nil
	}
	st, err := s.standingOf(r, u.PwikitID)
	if err != nil {
		return err
	}
	httpx.OK(w, map[string]any{"user": u, "standing": st})
	return nil
}

func (s *Server) handleStanding(w http.ResponseWriter, r *http.Request) error {
	id, err := strconv.ParseInt(strings.TrimSpace(urlParam(r, "pwikitId")), 10, 64)
	if err != nil || id <= 0 {
		return httpx.BadRequest("用户编号无效")
	}
	st, err := s.anyStanding(r, id)
	if err != nil {
		return err
	}
	httpx.OK(w, st)
	return nil
}

// Readers who never opened this site have no row yet. They hold nothing, so
// they tie with everyone else at zero.
func (s *Server) anyStanding(r *http.Request, id int64) (*standing, error) {
	st, err := s.standingOf(r, id)
	if err != nil || st != nil {
		return st, err
	}
	st = &standing{PwikitID: id}
	var rank int
	if err := s.db.QueryRow(r.Context(), `
		SELECT (SELECT COUNT(*) + 1 FROM users WHERE banned_at IS NULL AND coins > 0)::int,
		       (SELECT COUNT(*) FROM users WHERE banned_at IS NULL)::int`,
	).Scan(&rank, &st.Ranked); err != nil {
		return nil, err
	}
	st.Rank = &rank
	return st, nil
}
