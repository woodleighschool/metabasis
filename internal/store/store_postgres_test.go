//go:build postgres

package store_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/woodleighschool/metabasis/internal/intent"
	"github.com/woodleighschool/metabasis/internal/testutil/testdb"
)

func TestIntentUpsertReplacesDeliveryAndMarksChangedSubjectsDue(t *testing.T) {
	t.Parallel()
	intentStore := testdb.Open(t)
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	first := intent.Intent{
		Source: "freshservice", ID: "SR-1", Subject: "old@example.com",
		StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour),
	}
	if err := intentStore.UpsertIntent(t.Context(), first, now); err != nil {
		t.Fatalf("first UpsertIntent() error = %v", err)
	}
	updated := first
	updated.Subject = "new@example.com"
	updated.StartsAt = now.Add(3 * time.Hour)
	updated.EndsAt = now.Add(4 * time.Hour)
	updated.Cancelled = true
	if err := intentStore.UpsertIntent(t.Context(), updated, now.Add(time.Minute)); err != nil {
		t.Fatalf("updated UpsertIntent() error = %v", err)
	}

	intents, err := intentStore.ListAllIntents(t.Context())
	if err != nil {
		t.Fatalf("ListAllIntents() error = %v", err)
	}
	if len(intents) != 1 {
		t.Fatalf("intents = %+v, want one", intents)
	}
	if got := intents[0]; got.Subject != updated.Subject || !got.StartsAt.Equal(updated.StartsAt) || !got.Cancelled {
		t.Errorf("updated intent = %+v, want %+v", got, updated)
	}
	due, err := intentStore.ListSubjectsDue(t.Context(), now.Add(time.Minute))
	if err != nil {
		t.Fatalf("ListSubjectsDue() error = %v", err)
	}
	if got, want := due, []string{"new@example.com", "old@example.com"}; !slices.Equal(got, want) {
		t.Errorf("due subjects = %v, want %v", got, want)
	}
	oldState, err := intentStore.GetState(t.Context(), "old@example.com")
	if err != nil {
		t.Fatalf("GetState(old) error = %v", err)
	}
	if oldState.NextRetryAt == nil || !oldState.NextRetryAt.Equal(now.Add(time.Minute)) {
		t.Errorf("old subject retry = %v, want update time", oldState.NextRetryAt)
	}
	if err := intentStore.Migrate(t.Context()); err != nil {
		t.Fatalf("second Migrate() error = %v", err)
	}
}

func TestSubjectLockPreventsIntentUpdateFromBeingClearedByStaleReconciliation(t *testing.T) {
	t.Parallel()
	intentStore := testdb.Open(t)
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	accepted := intent.Intent{
		Source: "freshservice", ID: "SR-1", Subject: "student@example.com",
		StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour),
	}
	if err := intentStore.UpsertIntent(t.Context(), accepted, now); err != nil {
		t.Fatalf("initial UpsertIntent() error = %v", err)
	}

	session, err := intentStore.LockSubject(t.Context(), accepted.Subject)
	if err != nil {
		t.Fatalf("LockSubject() error = %v", err)
	}
	if _, err := session.ListIntents(t.Context()); err != nil {
		t.Fatalf("ListIntents() error = %v", err)
	}
	updatedAt := now.Add(time.Minute)
	updated := accepted
	updated.EndsAt = accepted.EndsAt.Add(time.Hour)
	started := make(chan struct{})
	upsertDone := make(chan error, 1)
	go func() {
		close(started)
		upsertDone <- intentStore.UpsertIntent(context.Background(), updated, updatedAt)
	}()
	<-started
	select {
	case err := <-upsertDone:
		t.Fatalf("UpsertIntent() completed while subject lock was held: %v", err)
	case <-time.After(100 * time.Millisecond):
	}

	if err := session.RecordSuccess(t.Context(), now, &accepted.StartsAt); err != nil {
		t.Fatalf("RecordSuccess() error = %v", err)
	}
	if err := session.Close(); err != nil {
		t.Fatalf("SubjectSession.Close() error = %v", err)
	}
	select {
	case err := <-upsertDone:
		if err != nil {
			t.Fatalf("UpsertIntent() after unlock error = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("UpsertIntent() remained blocked after subject unlock")
	}
	state, err := intentStore.GetState(t.Context(), accepted.Subject)
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if state.NextRetryAt == nil || !state.NextRetryAt.Equal(updatedAt) {
		t.Fatalf("next retry = %v, want webhook update time %v", state.NextRetryAt, updatedAt)
	}
}

func TestCollectMetricsReportsIntentPhasesAndReconciliationState(t *testing.T) {
	t.Parallel()
	intentStore := testdb.Open(t)
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	intents := []intent.Intent{
		{Source: "source", ID: "pending", Subject: "pending@example.com", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour)},
		{Source: "source", ID: "active", Subject: "active@example.com", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)},
		{Source: "source", ID: "ended", Subject: "ended@example.com", StartsAt: now.Add(-2 * time.Hour), EndsAt: now.Add(-time.Hour)},
		{Source: "source", ID: "cancelled", Subject: "cancelled@example.com", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), Cancelled: true},
	}
	for _, accepted := range intents {
		if err := intentStore.UpsertIntent(t.Context(), accepted, now.Add(-time.Minute)); err != nil {
			t.Fatalf("UpsertIntent(%s) error = %v", accepted.ID, err)
		}
	}
	if err := intentStore.RecordFailure(t.Context(), "active@example.com", now, "Graph unavailable", nil, now.Add(time.Minute)); err != nil {
		t.Fatalf("RecordFailure() error = %v", err)
	}
	snapshot, err := intentStore.CollectMetrics(t.Context(), now)
	if err != nil {
		t.Fatalf("CollectMetrics() error = %v", err)
	}
	if snapshot.PendingIntentCount != 1 || snapshot.ActiveIntentCount != 1 || snapshot.EndedIntentCount != 1 || snapshot.CancelledIntentCount != 1 {
		t.Errorf("phase counts = %+v", snapshot)
	}
	if snapshot.FailedSubjectCount != 1 || snapshot.DueSubjectCount != 3 {
		t.Errorf("reconciliation counts = %+v, want failed=1 due=3", snapshot)
	}
}

func TestReconciliationStatePersistsRetryAndSuccess(t *testing.T) {
	t.Parallel()
	intentStore := testdb.Open(t)
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	nextRetry := now.Add(30 * time.Second)
	if err := intentStore.RecordFailure(t.Context(), "user@example.com", now, "Graph unavailable", nil, nextRetry); err != nil {
		t.Fatalf("RecordFailure() error = %v", err)
	}
	state, err := intentStore.GetState(t.Context(), "user@example.com")
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if state.RetryCount != 1 || state.LastError != "Graph unavailable" ||
		state.NextRetryAt == nil || !state.NextRetryAt.Equal(nextRetry) {
		t.Fatalf("failure state = %+v", state)
	}
	nextTransition := now.Add(time.Hour)
	if err := intentStore.RecordSuccess(t.Context(), "user@example.com", now.Add(time.Minute), &nextTransition); err != nil {
		t.Fatalf("RecordSuccess() error = %v", err)
	}
	state, err = intentStore.GetState(t.Context(), "user@example.com")
	if err != nil {
		t.Fatalf("GetState() after success error = %v", err)
	}
	if state.RetryCount != 0 || state.LastError != "" || state.NextRetryAt != nil ||
		state.NextTransitionAt == nil || !state.NextTransitionAt.Equal(nextTransition) {
		t.Fatalf("success state = %+v", state)
	}
}
