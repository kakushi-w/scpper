package api

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)

		dur := time.Since(start)
		lvl := slog.LevelInfo
		switch {
		case ww.Status() >= 500:
			lvl = slog.LevelError
		case ww.Status() >= 400:
			lvl = slog.LevelWarn
		}
		slog.Log(r.Context(), lvl, "http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.Status(),
			"bytes", ww.BytesWritten(),
			"ms", dur.Milliseconds(),
		)
	})
}

type rateLimiter struct {
	mu   sync.Mutex
	hits map[string]time.Time
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{hits: make(map[string]time.Time)}
	go rl.gc()
	return rl
}

func (rl *rateLimiter) Allow(key string, window time.Duration) (bool, time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	if last, ok := rl.hits[key]; ok {
		if wait := window - now.Sub(last); wait > 0 {
			return false, wait
		}
	}
	rl.hits[key] = now
	return true, 0
}

func (rl *rateLimiter) gc() {
	t := time.NewTicker(10 * time.Minute)
	defer t.Stop()
	for range t.C {
		cutoff := time.Now().Add(-1 * time.Hour)
		rl.mu.Lock()
		for k, v := range rl.hits {
			if v.Before(cutoff) {
				delete(rl.hits, k)
			}
		}
		rl.mu.Unlock()
	}
}
