// Package auth trusts the reader pwikit signs into each forwarded request.
// scpper keeps no passwords or sessions of its own.
package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/scpwikicloud/scpper/internal/db"
	"github.com/scpwikicloud/scpper/internal/httpx"
)

const UserHeader = "X-Pwikit-User"

type ctxKey struct{}

type User struct {
	ID        int64     `json:"id"`
	PwikitID  int64     `json:"pwikitId"`
	Name      string    `json:"name"`
	AvatarURL *string   `json:"avatarUrl"`
	Role      string    `json:"role"`
	Coins     int64     `json:"coins"`
	CreatedAt time.Time `json:"createdAt"`
}

func (u *User) IsAdmin() bool { return u != nil && u.Role == "admin" }

type Service struct {
	db        *db.DB
	secret    []byte
	bootstrap string
}

func NewService(database *db.DB, secret, bootstrapAdmin string) *Service {
	return &Service{db: database, secret: []byte(secret), bootstrap: strings.TrimSpace(bootstrapAdmin)}
}

type identity struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	Name      string `json:"name"`
	Superuser bool   `json:"superuser"`
	Expires   int64  `json:"exp"`
}

func (s *Service) verify(r *http.Request) *identity {
	body, sig, ok := strings.Cut(r.Header.Get(UserHeader), ".")
	if !ok || body == "" {
		return nil
	}
	mac := hmac.New(sha256.New, s.secret)
	mac.Write([]byte(body))
	want := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil
	}
	var id identity
	if json.Unmarshal(raw, &id) != nil || id.ID <= 0 || time.Now().Unix() > id.Expires {
		return nil
	}
	return &id
}

func (s *Service) upsert(ctx context.Context, id *identity) (*User, error) {
	role := "user"
	if id.Superuser || (s.bootstrap != "" && strings.EqualFold(s.bootstrap, id.Username)) {
		role = "admin"
	}
	var u User
	var banned *time.Time
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (pwikit_id, name, role, last_active_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (pwikit_id) DO UPDATE SET
			name = EXCLUDED.name,
			role = CASE WHEN EXCLUDED.role = 'admin' THEN 'admin' ELSE users.role END,
			last_active_at = CASE WHEN users.last_active_at IS NULL OR users.last_active_at < now() - interval '5 minutes'
			                      THEN now() ELSE users.last_active_at END
		RETURNING id, pwikit_id, name, avatar_url, role, coins, created_at, banned_at`,
		id.ID, id.Username, role,
	).Scan(&u.ID, &u.PwikitID, &u.Name, &u.AvatarURL, &u.Role, &u.Coins, &u.CreatedAt, &banned)
	if err != nil {
		return nil, err
	}
	if banned != nil {
		return nil, httpx.Forbidden("账号已被封禁")
	}
	return &u, nil
}

func (s *Service) Optional(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := s.verify(r); id != nil {
			u, err := s.upsert(r.Context(), id)
			if err != nil {
				httpx.WriteError(w, r, err)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), ctxKey{}, u))
		}
		next.ServeHTTP(w, r)
	})
}

func SameOrigin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}
		if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" {
			httpx.WriteError(w, r, httpx.Forbidden("请求来源无效"))
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" {
			host := r.Header.Get("X-Forwarded-Host")
			if host == "" {
				host = r.Host
			}
			parsed, err := url.Parse(origin)
			if err != nil || !strings.EqualFold(parsed.Host, host) {
				httpx.WriteError(w, r, httpx.Forbidden("请求来源无效"))
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Service) Required(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if FromContext(r.Context()) == nil {
			httpx.WriteError(w, r, httpx.Unauthorized("请先登录"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := FromContext(r.Context())
		if u == nil {
			httpx.WriteError(w, r, httpx.Unauthorized("请先登录"))
			return
		}
		if !u.IsAdmin() {
			httpx.WriteError(w, r, httpx.Forbidden("需要管理员权限"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func FromContext(ctx context.Context) *User {
	u, _ := ctx.Value(ctxKey{}).(*User)
	return u
}

func MustUser(ctx context.Context) *User {
	u := FromContext(ctx)
	if u == nil {
		panic("MustUser called without a reader")
	}
	return u
}
