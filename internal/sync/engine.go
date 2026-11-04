// Package sync keeps the mirror of the pwikit site and the Crom index, and
// moves the stock market along from them.
package sync

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/scpwikicloud/scpper/internal/config"
	"github.com/scpwikicloud/scpper/internal/crom"
	"github.com/scpwikicloud/scpper/internal/db"
)

type Engine struct {
	cfg  *config.Config
	db   *db.DB
	crom *crom.Client
}

func NewEngine(cfg *config.Config, database *db.DB) *Engine {
	return &Engine{
		cfg:  cfg,
		db:   database,
		crom: crom.New(cfg.CromGraphQLURL, cfg.CromQPS, 2),
	}
}

func (e *Engine) getSyncTime(ctx context.Context, key string) (time.Time, bool) {
	var raw []byte
	err := e.db.QueryRow(ctx, `SELECT value FROM sync_state WHERE key = $1`, key).Scan(&raw)
	if err != nil {
		return time.Time{}, false
	}
	var v struct {
		At time.Time `json:"at"`
	}
	if json.Unmarshal(raw, &v) != nil || v.At.IsZero() {
		return time.Time{}, false
	}
	return v.At, true
}

func (e *Engine) setSyncTime(ctx context.Context, key string, t time.Time) {
	payload, _ := json.Marshal(map[string]any{"at": t.UTC()})
	_, err := e.db.Exec(ctx, `
		INSERT INTO sync_state (key, value, updated_at) VALUES ($1, $2, now())
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, key, payload)
	if err != nil {
		slog.Warn("write sync cursor", "key", key, "err", err)
	}
}
