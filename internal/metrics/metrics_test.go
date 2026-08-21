package metrics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/woodleighschool/metabasis/internal/store"
)

type fakeStateReader struct {
	snapshot store.MetricsSnapshot
	err      error
}

func (r fakeStateReader) CollectMetrics(context.Context, time.Time) (store.MetricsSnapshot, error) {
	return r.snapshot, r.err
}

func TestStateCollectorEmitsEveryPhaseIncludingZero(t *testing.T) {
	t.Parallel()
	collector := newStateCollector(fakeStateReader{snapshot: store.MetricsSnapshot{
		ActiveIntentCount:  2,
		FailedSubjectCount: 1,
		DueSubjectCount:    3,
	}}, slog.New(slog.DiscardHandler))
	want := `
# HELP metabasis_intents Current persisted intents by temporal phase.
# TYPE metabasis_intents gauge
metabasis_intents{phase="active"} 2
metabasis_intents{phase="cancelled"} 0
metabasis_intents{phase="ended"} 0
metabasis_intents{phase="pending"} 0
# HELP metabasis_reconciliation_due_subjects Subjects currently due for reconciliation.
# TYPE metabasis_reconciliation_due_subjects gauge
metabasis_reconciliation_due_subjects 3
# HELP metabasis_reconciliation_failed_subjects Subjects whose most recent reconciliation failed.
# TYPE metabasis_reconciliation_failed_subjects gauge
metabasis_reconciliation_failed_subjects 1
# HELP metabasis_state_collection_success Whether the latest scrape collected persisted Metabasis state.
# TYPE metabasis_state_collection_success gauge
metabasis_state_collection_success 1
`
	if err := testutil.CollectAndCompare(
		collector,
		strings.NewReader(want),
		"metabasis_intents",
		"metabasis_reconciliation_due_subjects",
		"metabasis_reconciliation_failed_subjects",
		"metabasis_state_collection_success",
	); err != nil {
		t.Fatal(err)
	}
}

func TestStateCollectorKeepsScrapeHealthyWhenDatabaseCollectionFails(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	collector := newStateCollector(
		fakeStateReader{err: errors.New("database connection refused")},
		slog.New(slog.NewTextHandler(&logs, nil)),
	)
	want := `
# HELP metabasis_state_collection_success Whether the latest scrape collected persisted Metabasis state.
# TYPE metabasis_state_collection_success gauge
metabasis_state_collection_success 0
`
	if err := testutil.CollectAndCompare(
		collector,
		strings.NewReader(want),
		"metabasis_intents",
		"metabasis_reconciliation_due_subjects",
		"metabasis_reconciliation_failed_subjects",
		"metabasis_state_collection_success",
	); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(logs.String(), "database connection refused") {
		t.Fatalf("log = %q, want database error", logs.String())
	}
}

func TestRecorderUsesBoundedEventResults(t *testing.T) {
	t.Parallel()
	recorder := New(BuildInfo{Version: "3.1.0", Revision: "abc123"}, slog.New(slog.DiscardHandler))
	recorder.RecordWebhook("freshservice", WebhookAccepted)
	recorder.RecordWebhook("freshservice", WebhookInvalid)
	recorder.RecordReconciliation(nil, 250*time.Millisecond)
	recorder.RecordReconciliation(errors.New("Graph unavailable"), time.Second)

	if got := testutil.ToFloat64(recorder.webhookRequests.WithLabelValues("freshservice", "accepted")); got != 1 {
		t.Errorf("accepted webhooks = %v, want 1", got)
	}
	if got := testutil.ToFloat64(recorder.webhookRequests.WithLabelValues("freshservice", "invalid")); got != 1 {
		t.Errorf("invalid webhooks = %v, want 1", got)
	}
	if got := testutil.ToFloat64(recorder.reconciliations.WithLabelValues("success")); got != 1 {
		t.Errorf("successful reconciliations = %v, want 1", got)
	}
	if got := testutil.ToFloat64(recorder.reconciliations.WithLabelValues("error")); got != 1 {
		t.Errorf("failed reconciliations = %v, want 1", got)
	}
	buildInfo := fmt.Sprintf(`
# HELP metabasis_build_info Build information for the running Metabasis process.
# TYPE metabasis_build_info gauge
metabasis_build_info{goversion=%q,revision="abc123",version="3.1.0"} 1
`, runtime.Version())
	if err := testutil.GatherAndCompare(recorder.registry, strings.NewReader(buildInfo), "metabasis_build_info"); err != nil {
		t.Fatal(err)
	}
}
