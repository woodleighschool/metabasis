package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/intent"
)

const migrationLockID int64 = 557901720260820001

//go:embed migrations/*.sql
var migrations embed.FS

// State is the persisted operational outcome for one subject.
type State struct {
	Subject          string     `json:"subject"`
	LastAttemptAt    *time.Time `json:"last_attempt_at,omitempty"`
	LastSuccessAt    *time.Time `json:"last_success_at,omitempty"`
	LastError        string     `json:"last_error,omitempty"`
	NextTransitionAt *time.Time `json:"next_transition_at,omitempty"`
	NextRetryAt      *time.Time `json:"next_retry_at,omitempty"`
	RetryCount       int        `json:"retry_count"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

// Store is the concrete PostgreSQL intent and reconciliation store.
type Store struct {
	pool *pgxpool.Pool
}

// Open connects to PostgreSQL and optionally applies embedded migrations.
func Open(ctx context.Context, database config.Database, migrate bool) (*Store, error) {
	poolConfig, err := pgxpool.ParseConfig(database.URL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	poolConfig.MinConns = database.MinConnections
	poolConfig.MaxConns = database.MaxConnections
	poolConfig.MaxConnLifetime = database.MaxConnLifetime.Duration
	poolConfig.MaxConnIdleTime = database.MaxConnIdleTime.Duration
	poolConfig.HealthCheckPeriod = database.HealthCheckPeriod.Duration

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	store := &Store{pool: pool}
	if migrate {
		if err := store.Migrate(ctx); err != nil {
			pool.Close()
			return nil, err
		}
	}
	return store, nil
}

// Close closes the connection pool.
func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

// Ping verifies database connectivity.
func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

// Migrate applies pending embedded migrations under a PostgreSQL advisory lock.
func (s *Store) Migrate(ctx context.Context) error {
	migrationFiles, err := fs.Sub(migrations, "migrations")
	if err != nil {
		return fmt.Errorf("open embedded migrations: %w", err)
	}
	locker, err := lock.NewPostgresSessionLocker(lock.WithLockID(migrationLockID))
	if err != nil {
		return fmt.Errorf("create migration lock: %w", err)
	}
	sqlDB := stdlib.OpenDBFromPool(s.pool)
	defer func() { _ = sqlDB.Close() }()
	provider, err := goose.NewProvider(
		goose.DialectPostgres,
		sqlDB,
		migrationFiles,
		goose.WithLogger(goose.NopLogger()),
		goose.WithSessionLocker(locker),
	)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

// UpsertIntent durably replaces an intent and marks every affected subject due for reconciliation.
func (s *Store) UpsertIntent(ctx context.Context, accepted intent.Intent, now time.Time) error {
	transaction, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin intent upsert: %w", err)
	}
	defer func() { _ = transaction.Rollback(context.WithoutCancel(ctx)) }()
	if _, err := transaction.Exec(
		ctx,
		"SELECT pg_advisory_xact_lock(hashtext($1), hashtext($2))",
		accepted.Source,
		accepted.ID,
	); err != nil {
		return fmt.Errorf("lock intent identity: %w", err)
	}

	var previousSubject string
	err = transaction.QueryRow(
		ctx,
		"SELECT subject FROM intents WHERE source = $1 AND id = $2 FOR UPDATE",
		accepted.Source, accepted.ID,
	).Scan(&previousSubject)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("load previous intent: %w", err)
	}
	_, err = transaction.Exec(ctx, `
INSERT INTO intents (source, id, subject, starts_at, ends_at, cancelled, created_at, updated_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
ON CONFLICT (source, id) DO UPDATE SET
    subject = EXCLUDED.subject,
    starts_at = EXCLUDED.starts_at,
    ends_at = EXCLUDED.ends_at,
    cancelled = EXCLUDED.cancelled,
    updated_at = EXCLUDED.updated_at`,
		accepted.Source, accepted.ID, accepted.Subject, accepted.StartsAt, accepted.EndsAt, accepted.Cancelled, now,
	)
	if err != nil {
		return fmt.Errorf("upsert intent: %w", err)
	}
	for _, subject := range uniqueSubjects(previousSubject, accepted.Subject) {
		if _, err := transaction.Exec(ctx, `
INSERT INTO reconciliation_state (subject, next_retry_at, updated_at)
VALUES ($1, $2, $2)
ON CONFLICT (subject) DO UPDATE SET next_retry_at = EXCLUDED.next_retry_at, updated_at = EXCLUDED.updated_at`,
			subject, now,
		); err != nil {
			return fmt.Errorf("mark subject %q due: %w", subject, err)
		}
	}
	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit intent upsert: %w", err)
	}
	return nil
}

// ListIntents returns all accepted intents for a subject, including ended and cancelled intents.
func (s *Store) ListIntents(ctx context.Context, subject string) ([]intent.Intent, error) {
	rows, err := s.pool.Query(ctx, `
SELECT source, id, subject, starts_at, ends_at, cancelled, created_at, updated_at
FROM intents WHERE subject = $1 ORDER BY source, id`, subject)
	if err != nil {
		return nil, fmt.Errorf("list subject intents: %w", err)
	}
	return scanIntents(rows)
}

// ListAllIntents returns every accepted intent in stable order.
func (s *Store) ListAllIntents(ctx context.Context) ([]intent.Intent, error) {
	rows, err := s.pool.Query(ctx, `
SELECT source, id, subject, starts_at, ends_at, cancelled, created_at, updated_at
FROM intents ORDER BY source, id`)
	if err != nil {
		return nil, fmt.Errorf("list intents: %w", err)
	}
	return scanIntents(rows)
}

// GetIntent returns one accepted intent by its source-owned identity.
func (s *Store) GetIntent(ctx context.Context, source, id string) (intent.Intent, error) {
	var accepted intent.Intent
	err := s.pool.QueryRow(ctx, `
SELECT source, id, subject, starts_at, ends_at, cancelled, created_at, updated_at
FROM intents WHERE source = $1 AND id = $2`, source, id).Scan(
		&accepted.Source, &accepted.ID, &accepted.Subject, &accepted.StartsAt, &accepted.EndsAt,
		&accepted.Cancelled, &accepted.CreatedAt, &accepted.UpdatedAt,
	)
	if err != nil {
		return intent.Intent{}, fmt.Errorf("get intent: %w", err)
	}
	return accepted, nil
}

// ListSubjects returns every subject that has an accepted intent.
func (s *Store) ListSubjects(ctx context.Context) ([]string, error) {
	return s.listSubjectQuery(ctx, `
SELECT subject FROM intents
UNION
SELECT subject FROM reconciliation_state
WHERE next_transition_at IS NOT NULL OR next_retry_at IS NOT NULL
ORDER BY subject`)
}

// ListSubjectsDue returns subjects whose state is new, transitioning, or retryable at now.
func (s *Store) ListSubjectsDue(ctx context.Context, now time.Time) ([]string, error) {
	return s.listSubjectQuery(ctx, `
SELECT subject
FROM (
    SELECT i.subject
    FROM intents i
    LEFT JOIN reconciliation_state r ON r.subject = i.subject
    WHERE r.subject IS NULL
    UNION
    SELECT subject
    FROM reconciliation_state
    WHERE next_transition_at <= $1 OR next_retry_at <= $1
) AS due
ORDER BY subject`, now)
}

func (s *Store) listSubjectQuery(ctx context.Context, query string, args ...any) ([]string, error) {
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list subjects: %w", err)
	}
	defer rows.Close()
	var subjects []string
	for rows.Next() {
		var subject string
		if err := rows.Scan(&subject); err != nil {
			return nil, fmt.Errorf("scan subject: %w", err)
		}
		subjects = append(subjects, subject)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read subjects: %w", err)
	}
	return subjects, nil
}

// GetState returns the operational state for a subject. A missing row yields zero state.
func (s *Store) GetState(ctx context.Context, subject string) (State, error) {
	state := State{Subject: subject}
	err := s.pool.QueryRow(ctx, `
SELECT last_attempt_at, last_success_at, last_error, next_transition_at, next_retry_at, retry_count, updated_at
FROM reconciliation_state WHERE subject = $1`, subject).Scan(
		&state.LastAttemptAt, &state.LastSuccessAt, &state.LastError, &state.NextTransitionAt,
		&state.NextRetryAt, &state.RetryCount, &state.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return state, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("get reconciliation state: %w", err)
	}
	return state, nil
}

// RecordSuccess persists a complete successful reconciliation outcome.
func (s *Store) RecordSuccess(ctx context.Context, subject string, attemptedAt time.Time, nextTransition *time.Time) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO reconciliation_state (
    subject, last_attempt_at, last_success_at, last_error, next_transition_at, next_retry_at, retry_count, updated_at
) VALUES ($1, $2, $2, '', $3, NULL, 0, $2)
ON CONFLICT (subject) DO UPDATE SET
    last_attempt_at = EXCLUDED.last_attempt_at,
    last_success_at = EXCLUDED.last_success_at,
    last_error = '',
    next_transition_at = EXCLUDED.next_transition_at,
    next_retry_at = NULL,
    retry_count = 0,
    updated_at = EXCLUDED.updated_at`, subject, attemptedAt, nextTransition)
	if err != nil {
		return fmt.Errorf("record reconciliation success: %w", err)
	}
	return nil
}

// RecordFailure persists an error, the derived transition, and the next retry.
func (s *Store) RecordFailure(
	ctx context.Context,
	subject string,
	attemptedAt time.Time,
	message string,
	nextTransition *time.Time,
	nextRetry time.Time,
) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO reconciliation_state (
    subject, last_attempt_at, last_error, next_transition_at, next_retry_at, retry_count, updated_at
) VALUES ($1, $2, $3, $4, $5, 1, $2)
ON CONFLICT (subject) DO UPDATE SET
    last_attempt_at = EXCLUDED.last_attempt_at,
    last_error = EXCLUDED.last_error,
    next_transition_at = EXCLUDED.next_transition_at,
    next_retry_at = EXCLUDED.next_retry_at,
    retry_count = reconciliation_state.retry_count + 1,
    updated_at = EXCLUDED.updated_at`, subject, attemptedAt, message, nextTransition, nextRetry)
	if err != nil {
		return fmt.Errorf("record reconciliation failure: %w", err)
	}
	return nil
}

// NextWake returns the earliest persisted transition or retry after now.
func (s *Store) NextWake(ctx context.Context, now time.Time) (*time.Time, error) {
	var next *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT MIN(wake_at)
FROM (
    SELECT next_transition_at AS wake_at FROM reconciliation_state WHERE next_transition_at > $1
    UNION ALL
    SELECT next_retry_at AS wake_at FROM reconciliation_state WHERE next_retry_at > $1
) AS wakes`, now).Scan(&next)
	if err != nil {
		return nil, fmt.Errorf("get next wake: %w", err)
	}
	return next, nil
}

func scanIntents(rows pgx.Rows) ([]intent.Intent, error) {
	defer rows.Close()
	var intents []intent.Intent
	for rows.Next() {
		var accepted intent.Intent
		if err := rows.Scan(
			&accepted.Source, &accepted.ID, &accepted.Subject, &accepted.StartsAt, &accepted.EndsAt,
			&accepted.Cancelled, &accepted.CreatedAt, &accepted.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan intent: %w", err)
		}
		intents = append(intents, accepted)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read intents: %w", err)
	}
	return intents, nil
}

func uniqueSubjects(values ...string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
