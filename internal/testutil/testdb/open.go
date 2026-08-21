//go:build postgres

package testdb

import (
	"os"
	"testing"
	"time"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/store"
)

const testDatabaseURL = "METABASIS_TEST_DATABASE_URL"

// Open returns an isolated migrated test store.
func Open(t testing.TB) *store.Store {
	return OpenWithMaxConnections(t, 10)
}

// OpenWithMaxConnections returns an isolated store with a bounded connection pool.
func OpenWithMaxConnections(t testing.TB, maximum int32) *store.Store {
	t.Helper()
	baseURL := os.Getenv(testDatabaseURL)
	if baseURL == "" {
		t.Fatalf("%s is required for PostgreSQL tests", testDatabaseURL)
	}
	databaseURL := Create(t, baseURL)
	intentStore, err := store.Open(t.Context(), config.Database{
		URL:               databaseURL,
		MaxConnections:    maximum,
		MaxConnLifetime:   config.Duration{Duration: 30 * time.Minute},
		MaxConnIdleTime:   config.Duration{Duration: 5 * time.Minute},
		HealthCheckPeriod: config.Duration{Duration: time.Minute},
	}, true)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(intentStore.Close)
	return intentStore
}
