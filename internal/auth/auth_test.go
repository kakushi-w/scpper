package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func signed(secret string, id identity) string {
	raw, _ := json.Marshal(id)
	body := base64.RawURLEncoding.EncodeToString(raw)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return body + "." + hex.EncodeToString(mac.Sum(nil))
}

func TestVerify(t *testing.T) {
	s := &Service{secret: []byte("k")}
	fresh := identity{ID: 7, Username: "alice", Expires: time.Now().Add(time.Minute).Unix()}
	cases := map[string]struct {
		header string
		want   bool
	}{
		"good":     {signed("k", fresh), true},
		"wrongkey": {signed("other", fresh), false},
		"stale":    {signed("k", identity{ID: 7, Username: "alice", Expires: time.Now().Add(-time.Second).Unix()}), false},
		"noid":     {signed("k", identity{Username: "alice", Expires: fresh.Expires}), false},
		"garbage":  {"x.y", false},
		"empty":    {"", false},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
			if c.header != "" {
				r.Header.Set(UserHeader, c.header)
			}
			if got := s.verify(r) != nil; got != c.want {
				t.Errorf("verify(%s) = %v, want %v", name, got, c.want)
			}
		})
	}
}

func TestVerifySuperuser(t *testing.T) {
	s := &Service{secret: []byte("k")}
	exp := time.Now().Add(time.Minute).Unix()
	for _, want := range []bool{true, false} {
		r := httptest.NewRequest(http.MethodGet, "/api/me", nil)
		r.Header.Set(UserHeader, signed("k", identity{ID: 1, Username: "root", Superuser: want, Expires: exp}))
		id := s.verify(r)
		if id == nil {
			t.Fatalf("verify(superuser=%v) = nil, want an identity", want)
		}
		if id.Superuser != want {
			t.Errorf("verify().Superuser = %v, want %v", id.Superuser, want)
		}
	}
}

func TestSameOrigin(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	cases := map[string]struct {
		method, origin, fetchSite, host string
		want                            int
	}{
		"get":         {http.MethodGet, "https://evil.example", "cross-site", "wiki.example", http.StatusNoContent},
		"same":        {http.MethodPost, "https://wiki.example", "same-origin", "wiki.example", http.StatusNoContent},
		"noheaders":   {http.MethodPost, "", "", "wiki.example", http.StatusNoContent},
		"crossorigin": {http.MethodPost, "https://evil.example", "", "wiki.example", http.StatusForbidden},
		"crossfetch":  {http.MethodPost, "", "cross-site", "wiki.example", http.StatusForbidden},
		"samesitesub": {http.MethodPost, "https://sub.wiki.example", "same-site", "wiki.example", http.StatusForbidden},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			r := httptest.NewRequest(c.method, "/api/tips", nil)
			r.Header.Set("X-Forwarded-Host", c.host)
			if c.origin != "" {
				r.Header.Set("Origin", c.origin)
			}
			if c.fetchSite != "" {
				r.Header.Set("Sec-Fetch-Site", c.fetchSite)
			}
			w := httptest.NewRecorder()
			SameOrigin(ok).ServeHTTP(w, r)
			if w.Code != c.want {
				t.Errorf("SameOrigin(%s) = %d, want %d", name, w.Code, c.want)
			}
		})
	}
}
