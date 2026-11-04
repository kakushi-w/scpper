package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env      string
	HTTPAddr string

	DatabaseURL string

	PwikitDatabaseURL string
	PwikitSite        string
	PwikitSecretFile  string
	PwikitSecret      string
	PwikitSyncEvery   time.Duration
	PwikitURL         string
	PwikitMount       string

	CromGraphQLURL     string
	CromQPS            float64
	CromRecentPages    int
	CromRecentInterval time.Duration
	CromHistoryFrom    time.Time

	StockTickInterval  time.Duration
	TaskScanInterval   time.Duration
	ActiveTaskInterval time.Duration

	BootstrapAdmin string
}

func Load() (*Config, error) {
	loadDotEnv()

	c := &Config{
		Env:      env("APP_ENV", "development"),
		HTTPAddr: env("HTTP_ADDR", "127.0.0.1:8100"),

		DatabaseURL: env("DATABASE_URL", "postgres://scpper:scpper@localhost:5432/scpper?sslmode=disable"),

		PwikitDatabaseURL: env("PWIKIT_DATABASE_URL", ""),
		PwikitSite:        env("PWIKIT_SITE", ""),
		PwikitSecretFile:  env("PWIKIT_SECRET_FILE", ""),
		PwikitSyncEvery:   envDuration("PWIKIT_SYNC_INTERVAL", 5*time.Minute),
		PwikitURL:         strings.TrimRight(env("PWIKIT_URL", ""), "/"),
		PwikitMount:       env("PWIKIT_MOUNT", "/scpper/"),

		CromGraphQLURL:     env("CROM_GRAPHQL_URL", "https://apiv1.crom.avn.sh/graphql"),
		CromQPS:            envFloat("CROM_QPS", 1),
		CromRecentPages:    envInt("CROM_RECENT_PAGES", 600),
		CromRecentInterval: envDuration("CROM_RECENT_INTERVAL", time.Hour),

		StockTickInterval:  envDuration("STOCK_TICK_INTERVAL", time.Hour),
		TaskScanInterval:   envDuration("TASK_SCAN_INTERVAL", 1*time.Hour),
		ActiveTaskInterval: envDuration("ACTIVE_TASK_INTERVAL", 5*time.Minute),

		BootstrapAdmin: env("BOOTSTRAP_ADMIN", ""),
	}

	if v := strings.TrimSpace(os.Getenv("CROM_HISTORY_FROM")); v != "" {
		t, err := time.ParseInLocation("2006-01-02", v, time.FixedZone("CST", 8*3600))
		if err != nil {
			return nil, fmt.Errorf("CROM_HISTORY_FROM %q is not a YYYY-MM-DD date", v)
		}
		c.CromHistoryFrom = t
	}

	switch {
	case c.PwikitDatabaseURL == "":
		return nil, fmt.Errorf("PWIKIT_DATABASE_URL is not set")
	case c.PwikitSite == "":
		return nil, fmt.Errorf("PWIKIT_SITE is not set")
	case c.PwikitSecretFile == "":
		return nil, fmt.Errorf("PWIKIT_SECRET_FILE is not set")
	}
	raw, err := os.ReadFile(c.PwikitSecretFile)
	if err != nil {
		return nil, fmt.Errorf("read PWIKIT_SECRET_FILE: %w", err)
	}
	if c.PwikitSecret = strings.TrimSpace(string(raw)); c.PwikitSecret == "" {
		return nil, fmt.Errorf("%s is empty", c.PwikitSecretFile)
	}
	return c, nil
}

func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 5; i++ {
		path := filepath.Join(dir, ".env")
		if f, err := os.Open(path); err == nil {
			applyEnvFile(f)
			f.Close()
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func applyEnvFile(f *os.File) {
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		if len(val) >= 2 && (val[0] == '"' || val[0] == '\'') && val[len(val)-1] == val[0] {
			val = val[1 : len(val)-1]
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

func envFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return f
		}
	}
	return def
}

func envDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(strings.TrimSpace(v)); err == nil {
			return d
		}
	}
	return def
}
