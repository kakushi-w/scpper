// Command scpper runs the stock market, card draws and coins beside a pwikit
// site, reached through a pwikit mount.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/scpwikicloud/scpper/internal/api"
	"github.com/scpwikicloud/scpper/internal/auth"
	"github.com/scpwikicloud/scpper/internal/config"
	"github.com/scpwikicloud/scpper/internal/db"
	"github.com/scpwikicloud/scpper/internal/logx"
	"github.com/scpwikicloud/scpper/internal/sync"
	"github.com/scpwikicloud/scpper/internal/tasks"
	"github.com/scpwikicloud/scpper/internal/web"
)

var version = "dev"

func usage() {
	fmt.Fprint(os.Stderr, `Usage: scpper <command>

  serve     run scpper (the default)
  update    replace this binary with the latest release
  version   print the version

Settings come from the environment or a .env file next to where scpper runs.
`)
}

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "update":
		err = update(os.Args[2:])
	case "version":
		fmt.Println(version)
	case "-h", "--help", "help":
		usage()
	default:
		usage()
		err = fmt.Errorf("unknown command %q", cmd)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "scpper:", err)
		os.Exit(1)
	}
}

func serve() error {
	logx.Setup()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	database, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer database.Close()
	if err := database.Migrate(ctx); err != nil {
		return err
	}
	source, err := sync.OpenSource(ctx, cfg.PwikitDatabaseURL, cfg.PwikitSite)
	if err != nil {
		return err
	}
	defer source.Close()

	engine := sync.NewEngine(cfg, database)
	scanner := tasks.NewScanner(database, "Asia/Shanghai")
	authSvc := auth.NewService(database, cfg.PwikitSecret, cfg.BootstrapAdmin)
	server := api.NewServer(cfg, database, authSvc, scanner, engine, web.Pages())

	go background(ctx, cfg, engine, scanner, source, server)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           server.Router(),
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	errs := make(chan error, 1)
	go func() {
		slog.Info("scpper serve", "addr", cfg.HTTPAddr, "version", version, "site", cfg.PwikitSite)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errs <- err
		}
	}()
	select {
	case err := <-errs:
		return err
	case <-ctx.Done():
	}
	shutdown, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return srv.Shutdown(shutdown)
}

func background(ctx context.Context, cfg *config.Config, engine *sync.Engine, scanner *tasks.Scanner, source *sync.Source, server *api.Server) {
	if _, err := engine.SyncPwikit(ctx, source); err != nil {
		slog.Error("first pwikit sync", "err", err)
	} else if err := engine.EnsureLocalHistory(ctx); err != nil {
		slog.Error("draw price history", "err", err)
	}
	go engine.RunPwikit(ctx, source, cfg.PwikitSyncEvery)
	go server.AuctionLoop(ctx, 20*time.Second)
	go server.MarginLoop(ctx, time.Minute)

	every := func(d time.Duration, name string, fn func(context.Context) error) {
		go func() {
			t := time.NewTicker(d)
			defer t.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-t.C:
					if err := fn(ctx); err != nil {
						slog.Error(name, "err", err)
					}
				}
			}
		}()
	}
	every(cfg.StockTickInterval, "stock tick", func(ctx context.Context) error {
		if err := engine.EnsureLocalHistory(ctx); err != nil {
			slog.Error("draw price history", "err", err)
		}
		return engine.StockTick(ctx)
	})
	every(cfg.CromRecentInterval, "crom recent", func(ctx context.Context) error {
		_, err := engine.SyncCromRecent(ctx, "", cfg.CromRecentPages)
		return err
	})
	every(cfg.TaskScanInterval, "scan tasks", func(ctx context.Context) error {
		_, err := scanner.ScanAll(ctx)
		return err
	})
	every(cfg.ActiveTaskInterval, "scan active tasks", func(ctx context.Context) error {
		_, err := scanner.ScanActive(ctx, 2*cfg.ActiveTaskInterval)
		return err
	})
}
