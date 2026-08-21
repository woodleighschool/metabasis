//go:build postgres

package store_test

import (
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
