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

	activedirectory "github.com/woodleighschool/adoverseas/internal/activeDirectory"
	"github.com/woodleighschool/adoverseas/internal/auth"
	"github.com/woodleighschool/adoverseas/internal/config"
	"github.com/woodleighschool/adoverseas/internal/graph"
	httpapi "github.com/woodleighschool/adoverseas/internal/http"
	"github.com/woodleighschool/adoverseas/internal/schedules"
	"github.com/woodleighschool/adoverseas/internal/store"
)

var (
	buildVersion = "dev"
	gitCommit    = "unknown"
	buildDate    = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
		os.Exit(1)
	}
	defer db.Close()

	if err := db.Migrate(ctx); err != nil {
		logger.Error("run migrations", "err", err)
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
		os.Exit(1)
	}

	sessions, err := auth.NewSessionManager(
		cfg.SessionCookieName,
		cfg.SessionSecret,
		strings.HasPrefix(cfg.SiteBaseURL, "https"),
	)
	if err != nil {
		logger.Error("session manager", "err", err)
		os.Exit(1)
	}

	scheduler := schedules.NewScheduler(logger)
	graphClient, err := graph.NewClient(ctx, cfg.GraphTenantID, cfg.GraphClientID, cfg.GraphClientSecret)
	if err != nil {
		logger.Warn("graph client", "err", err)
	}
	if graphClient != nil && graphClient.Enabled() {
		if err := scheduler.Add("@every 5m", "entra-users", schedules.NewUserJob(db, graphClient, logger, cfg)); err != nil {
			logger.Warn("schedule users", "err", err)
		}
	}

	adClient, err := activedirectory.NewClient(cfg)
	if err != nil {
		logger.Error("ad client", "err", err)
		os.Exit(1)
	}

	if err := scheduler.Add("@every 10m", "task-checker", schedules.NewTaskJob(db, adClient, cfg, logger)); err != nil {
		logger.Error("task checker", "err", err)
	}

	scheduler.Start()
	defer scheduler.Stop()

	deps := httpapi.Deps{
		Store:        db,
		Logger:       logger,
		Sessions:     sessions,
		OIDCProvider: oidcProvider,
		BuildInfo:    buildInfo,
	}
	router := httpapi.NewRouter(cfg, deps)

	server, errCh := startServer(cfg, logger, router, cfg.ListenAddr)

	select {
	case <-ctx.Done():
		logger.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			logger.Error("server error", "err", err)
			os.Exit(1)
		}
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	<-errCh

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown", "err", err)
	}
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

func startServer(cfg config.Config, logger *slog.Logger, router http.Handler, listenAddress string) (*http.Server, <-chan error) {
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
