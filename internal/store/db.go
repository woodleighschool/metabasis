package store

import (
	"context"
	"embed"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/woodleighschool/adoverseas/internal/store/sqlc"
)

//go:embed migrate/*.sql
var migrations embed.FS

type Options struct {
	URL             string
	MaxConnections  int32
	MinConnections  int32
	MaxConnLifetime time.Duration
}

type Store struct {
	pool    *pgxpool.Pool
	queries *sqlc.Queries
}

func Open(ctx context.Context, opts Options) (*Store, error) {
	cfg, err := pgxpool.ParseConfig(opts.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	if opts.MaxConnections > 0 {
		cfg.MaxConns = opts.MaxConnections
	}
	if opts.MinConnections > 0 {
		cfg.MinConns = opts.MinConnections
	}
	if opts.MaxConnLifetime > 0 {
		cfg.MaxConnLifetime = opts.MaxConnLifetime
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	return &Store{pool: pool, queries: sqlc.New(pool)}, nil
}

func (s *Store) Close() {
	if s == nil || s.pool == nil {
		return
	}
	s.pool.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrations.ReadDir("migrate")
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		stmt, err := migrations.ReadFile("migrate/" + entry.Name())
		if err != nil {
			return fmt.Errorf("read migration %s: %w", entry.Name(), err)
		}
		if _, err := s.pool.Exec(ctx, string(stmt)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
	}
	return nil
}
