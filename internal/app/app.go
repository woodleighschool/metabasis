package app

import (
	"context"
	"fmt"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/graph"
	"github.com/woodleighschool/metabasis/internal/metrics"
	"github.com/woodleighschool/metabasis/internal/reconcile"
	"github.com/woodleighschool/metabasis/internal/store"
)

// App owns the concrete PostgreSQL, Graph, reconciliation, and HTTP components.
type App struct {
	Store      *store.Store
	Reconciler *reconcile.Service
}

// Build creates application components and optionally applies database migrations.
func Build(ctx context.Context, cfg *config.Config, migrate bool, recorder *metrics.Recorder) (*App, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	intentStore, err := store.Open(ctx, cfg.Database, migrate)
	if err != nil {
		return nil, err
	}
	recorder.RegisterState(intentStore)
	connection := cfg.Connections[cfg.Identity.Connection]
	directory, err := graph.NewClient(connection)
	if err != nil {
		intentStore.Close()
		return nil, err
	}
	reconciler, err := reconcile.New(cfg, intentStore, directory, recorder)
	if err != nil {
		intentStore.Close()
		return nil, err
	}
	return &App{Store: intentStore, Reconciler: reconciler}, nil
}

// Close releases the PostgreSQL connection pool.
func (a *App) Close() {
	if a != nil && a.Store != nil {
		a.Store.Close()
	}
}
