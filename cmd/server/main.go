package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/woodleighschool/adoverseas/internal/auth"
	"github.com/woodleighschool/adoverseas/internal/config"
	"github.com/woodleighschool/adoverseas/internal/graph"
	httpapi "github.com/woodleighschool/adoverseas/internal/http"
	"github.com/woodleighschool/adoverseas/internal/schedules"
	"github.com/woodleighschool/adoverseas/internal/store"
	webdist "github.com/woodleighschool/adoverseas/web"
)

var (
	buildVersion = "dev"
	gitCommit    = "unknown"
	buildDate    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := newLogger(cfg.LogLevel)
	buildInfo := httpapi.BuildInfo{
		Version:   buildVersion,
		GitCommit: gitCommit,
		BuildDate: buildDate,
	}
	logger.Info("starting adoverseas",
		"version", buildInfo.Version,
		"commit", buildInfo.GitCommit,
		"build_date", buildInfo.BuildDate,
	)

	db, err := store.Open(ctx, store.Options{
		URL:             cfg.DatabaseURL(),
		MaxConnections:  cfg.MaxConnections,
		MinConnections:  cfg.MinConnections,
		MaxConnLifetime: cfg.MaxConnLifetime,
	})
	if err != nil {
		logger.Error("connect db", "err", err)
		stop()
		os.Exit(1)
	}

	if err := db.Migrate(ctx); err != nil {
		logger.Error("run migrations", "err", err)
		db.Close()
		stop()
		os.Exit(1)
	}

	oidcProvider, err := auth.NewOIDCProvider(
		ctx,
		cfg.AdminIssuer,
		cfg.AdminClientID,
		cfg.AdminClientSecret,
		cfg.SiteBaseURL,
	)
	if err != nil {
		logger.Error("oidc provider", "err", err)
		db.Close()
		stop()
		os.Exit(1)
	}

	sessions, err := auth.NewSessionManager(
		cfg.SessionCookieName,
		cfg.SessionSecret,
		strings.HasPrefix(cfg.SiteBaseURL, "https"),
	)
	if err != nil {
		logger.Error("session manager", "err", err)
		db.Close()
		stop()
		os.Exit(1)
	}

	scheduler := startScheduler(ctx, cfg, db, logger)

	deps := httpapi.Deps{
		Store:        db,
		Logger:       logger,
		Sessions:     sessions,
		OIDCProvider: oidcProvider,
		BuildInfo:    buildInfo,
	}
	router := httpapi.NewRouter(cfg, deps, webdist.DistDirFS)

	server, errCh := startServer(cfg, logger, router)

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", "err", err)
			scheduler.Stop()
			db.Close()
			stop()
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	<-errCh

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown", "err", err)
	}
	scheduler.Stop()
	db.Close()
	stop()
}

func startScheduler(ctx context.Context, cfg config.Config, db *store.Store, logger *slog.Logger) *schedules.Scheduler {
	scheduler := schedules.NewScheduler(logger)
	graphClient, err := graph.NewClient(
		cfg.GraphTenantID,
		cfg.GraphClientID,
		cfg.GraphClientSecret,
		cfg.AwayGroups,
		cfg.HomeGroups,
		cfg.EnableMFAGroup,
		cfg.ForceMFAGroup,
	)
	if err != nil {
		logger.WarnContext(ctx, "graph client", "err", err)
	}
	if graphClient != nil && graphClient.Enabled() {
		if err := scheduler.Add(
			"@every 5m",
			"entra-users",
			schedules.NewUserJob(db, graphClient, logger, cfg),
		); err != nil {
			logger.WarnContext(ctx, "schedule users", "err", err)
		}
	}

	if err := scheduler.Add(
		"@every 1m",
		"task-checker",
		schedules.NewTaskJob(db, graphClient, logger),
	); err != nil {
		logger.ErrorContext(ctx, "task checker", "err", err)
	}

	scheduler.Start()
	return scheduler
}

func newLogger(level string) *slog.Logger {
	lvl := new(slog.LevelVar)
	switch strings.ToLower(level) {
	case "debug":
		lvl.Set(slog.LevelDebug)
	case "warn":
		lvl.Set(slog.LevelWarn)
	case "error":
		lvl.Set(slog.LevelError)
	default:
		lvl.Set(slog.LevelInfo)
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: lvl})
	return slog.New(handler)
}

func startServer(cfg config.Config, logger *slog.Logger, router http.Handler) (*http.Server, <-chan error) {
	server := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		logger.Info("listening", "server", "adoverseas", "addr", cfg.ListenAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
			return
		}
		errCh <- nil
	}()

	return server, errCh
}
