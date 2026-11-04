// Package api answers the stock market, card draws and coin requests.
package api

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/config"
	"github.com/scpwikicloud/scpper/internal/db"
	"github.com/scpwikicloud/scpper/internal/httpx"
	"github.com/scpwikicloud/scpper/internal/sync"
	"github.com/scpwikicloud/scpper/internal/tasks"
)

type Server struct {
	cfg     *config.Config
	db      *db.DB
	auth    *auth.Service
	scanner *tasks.Scanner
	engine  *sync.Engine
	web     fs.FS

	rl *rateLimiter
}

func NewServer(cfg *config.Config, database *db.DB, authSvc *auth.Service, scanner *tasks.Scanner,
	engine *sync.Engine, web fs.FS) *Server {
	return &Server{
		cfg:     cfg,
		db:      database,
		auth:    authSvc,
		scanner: scanner,
		engine:  engine,
		web:     web,
		rl:      newRateLimiter(),
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.OK(w, map[string]any{"ok": true})
	})

	r.Get("/badge/{file}", httpx.Wrap(s.handleBadge))

	r.Route("/api", func(r chi.Router) {
		r.Use(s.auth.Optional)
		r.Use(auth.SameOrigin)

		r.Get("/me", httpx.Wrap(s.handleMe))
		r.Get("/standing/{pwikitId}", httpx.Wrap(s.handleStanding))
		r.Get("/rank/wealth", httpx.Wrap(s.handleRankWealth))
		r.Get("/pages/{pageId}/tips", httpx.Wrap(s.handlePageTips))

		r.Get("/gacha", httpx.Wrap(s.handleGachaInfo))
		r.Get("/gacha/market", httpx.Wrap(s.handleGachaListings))

		r.Get("/stock", httpx.Wrap(s.handleStockQuote))
		r.Get("/stock/symbols", httpx.Wrap(s.handleStockSymbols))
		r.Get("/stock/candles", httpx.Wrap(s.handleStockCandles))
		r.Get("/stock/auctions", httpx.Wrap(s.handleStockAuctions))

		r.Group(func(r chi.Router) {
			r.Use(s.auth.Required)

			r.Get("/me/coins", httpx.Wrap(s.handleCoinLedger))
			r.Get("/me/tasks", httpx.Wrap(s.handleMyTasks))
			r.Post("/me/tasks/{code}/claim", httpx.Wrap(s.handleClaimTask))
			r.Post("/me/tasks/refresh", httpx.Wrap(s.handleRefreshTasks))

			r.Post("/tips", httpx.Wrap(s.handleTip))

			r.Post("/gacha/pull", httpx.Wrap(s.handleGachaPull))
			r.Get("/gacha/collection", httpx.Wrap(s.handleGachaCollection))
			r.Get("/gacha/history", httpx.Wrap(s.handleGachaHistory))
			r.Get("/gacha/my-listings", httpx.Wrap(s.handleGachaMyListings))
			r.Post("/gacha/market", httpx.Wrap(s.handleGachaList))
			r.Post("/gacha/market/{id}/buy", httpx.Wrap(s.handleGachaBuy))
			r.Delete("/gacha/market/{id}", httpx.Wrap(s.handleGachaUnlist))

			r.Post("/stock/orders", httpx.Wrap(s.handleStockPlace))
			r.Delete("/stock/orders/{id}", httpx.Wrap(s.handleStockCancel))
			r.Get("/stock/trades", httpx.Wrap(s.handleStockTrades))
			r.Get("/stock/portfolio", httpx.Wrap(s.handleStockPortfolio))
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(s.auth.Required)
			r.Use(auth.AdminOnly)

			r.Get("/overview", httpx.Wrap(s.handleAdminOverview))
			r.Get("/users", httpx.Wrap(s.handleAdminUsers))
			r.Post("/users/{id}/coins", httpx.Wrap(s.handleAdminGrantCoins))
			r.Post("/users/{id}/role", httpx.Wrap(s.handleAdminSetRole))

			r.Get("/tasks", httpx.Wrap(s.handleAdminTasks))
			r.Post("/tasks", httpx.Wrap(s.handleAdminSaveTask))
			r.Post("/tasks/{code}", httpx.Wrap(s.handleAdminSaveTask))

			r.Get("/gacha", httpx.Wrap(s.handleAdminGacha))
			r.Post("/gacha", httpx.Wrap(s.handleAdminGachaParams))

			r.Get("/stock", httpx.Wrap(s.handleAdminStock))
			r.Post("/stock/params", httpx.Wrap(s.handleAdminStockParams))
			r.Post("/stock/auction", httpx.Wrap(s.handleAdminStockAuction))
			r.Post("/stock/symbols/{code}/params", httpx.Wrap(s.handleAdminStockSymbolParams))
			r.Post("/stock/symbols/{code}/list", httpx.Wrap(s.handleAdminStockList))
			r.Post("/stock/symbols/{code}/delist", httpx.Wrap(s.handleAdminStockDelist))
			r.Post("/stock/crom/backfill", httpx.Wrap(s.handleAdminCromBackfill))
			r.Post("/stock/crom/full", httpx.Wrap(s.handleAdminCromFull))
			r.Get("/stock/jobs", httpx.Wrap(s.handleAdminStockJobs))
			r.Post("/stock/symbols/{code}/history", httpx.Wrap(s.handleAdminStockHistory))
		})
	})

	if s.web != nil {
		r.Handle("/*", s.pages())
	}
	return r
}

func (s *Server) pages() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.Trim(r.URL.Path, "/")
		candidates := []string{path, path + "/index.html"}
		if first, _, ok := strings.Cut(path, "/"); ok {
			candidates = append(candidates, first+"/index.html")
		}
		candidates = append(candidates, "index.html")
		for _, c := range candidates {
			if c == "" {
				continue
			}
			f, err := s.web.Open(c)
			if err != nil {
				continue
			}
			info, err := f.Stat()
			if err != nil || info.IsDir() {
				f.Close()
				continue
			}
			if strings.HasPrefix(c, "_next/") {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			http.ServeContent(w, r, c, info.ModTime(), f.(io.ReadSeeker))
			f.Close()
			return
		}
		http.NotFound(w, r)
	})
}
