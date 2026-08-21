//go:build postgres

package reconcile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/domain"
	"github.com/woodleighschool/metabasis/internal/graph"
	"github.com/woodleighschool/metabasis/internal/intent"
	"github.com/woodleighschool/metabasis/internal/testutil/testdb"
)

type fakeDirectory struct {
	snapshot graph.Snapshot
	err      error
	added    []string
	removed  []string
}

func (d *fakeDirectory) Resolve(context.Context, string, map[string][]string, map[string]string) (graph.Snapshot, error) {
	return d.snapshot, d.err
}

func (d *fakeDirectory) AddGroupMember(_ context.Context, groupID, _ string) error {
	d.added = append(d.added, groupID)
	return d.err
}

func (d *fakeDirectory) RemoveGroupMember(_ context.Context, groupID, _ string) error {
	d.removed = append(d.removed, groupID)
	return d.err
}

func TestGraphFailurePersistsRetryAndRestartRecoversMissedTransition(t *testing.T) {
	t.Parallel()
	intentStore := testdb.Open(t)
	cfg := loadTestConfig(t)
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	accepted := intent.Intent{
		Source: "freshservice", ID: "SR-1", Subject: "student@example.com",
		StartsAt: now.Add(time.Minute), EndsAt: now.Add(time.Hour),
	}
	if err := intentStore.UpsertIntent(t.Context(), accepted, now); err != nil {
		t.Fatalf("UpsertIntent() error = %v", err)
	}
	failingDirectory := &fakeDirectory{
		snapshot: graph.Snapshot{User: domain.User{Present: true, ID: "user-id", Groups: []string{"students"}}},
		err:      errors.New("Graph unavailable"),
	}
	service, err := New(cfg, intentStore, failingDirectory, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	service.now = func() time.Time { return now }
	result, err := service.ReconcileSubject(t.Context(), accepted.Subject)
	if err == nil || result.Error == "" {
		t.Fatalf("ReconcileSubject() result = %+v, error = %v", result, err)
	}
	state, err := intentStore.GetState(t.Context(), accepted.Subject)
	if err != nil {
		t.Fatalf("GetState() error = %v", err)
	}
	if state.RetryCount != 1 || state.NextRetryAt == nil || !state.NextRetryAt.Equal(now.Add(30*time.Second)) {
		t.Fatalf("retry state = %+v", state)
	}
	if state.NextTransitionAt == nil || !state.NextTransitionAt.Equal(accepted.StartsAt) {
		t.Fatalf("next transition after failure = %v, want %v", state.NextTransitionAt, accepted.StartsAt)
	}

	recoveredDirectory := &fakeDirectory{
		snapshot: graph.Snapshot{User: domain.User{Present: true, ID: "user-id", Groups: []string{"students"}}},
	}
	restarted, err := New(cfg, intentStore, recoveredDirectory, nil)
	if err != nil {
		t.Fatalf("restart New() error = %v", err)
	}
	restarted.now = func() time.Time { return now.Add(2 * time.Minute) }
	results, err := restarted.ReconcileAll(t.Context())
	if err != nil {
		t.Fatalf("restart ReconcileAll() error = %v", err)
	}
	if len(results) != 1 || results[0].Plan.Intents[0].Phase != intent.PhaseActive {
		t.Fatalf("restart results = %+v", results)
	}
	if got, want := recoveredDirectory.added, []string{"mfa-registration", "overseas-access"}; !slices.Equal(got, want) {
		t.Errorf("added groups = %v, want %v", got, want)
	}
	state, err = intentStore.GetState(t.Context(), accepted.Subject)
	if err != nil {
		t.Fatalf("GetState() after recovery error = %v", err)
	}
	if state.RetryCount != 0 || state.LastError != "" || state.NextRetryAt != nil {
		t.Fatalf("recovered state = %+v", state)
	}
}

func TestReconciliationIsIdempotentWhenManagedMembershipMatches(t *testing.T) {
	t.Parallel()
	intentStore := testdb.Open(t)
	cfg := loadTestConfig(t)
	now := time.Date(2026, 9, 1, 0, 30, 0, 0, time.UTC)
	accepted := intent.Intent{
		Source: "freshservice", ID: "SR-1", Subject: "staff@example.com",
		StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour),
	}
	if err := intentStore.UpsertIntent(t.Context(), accepted, now); err != nil {
		t.Fatalf("UpsertIntent() error = %v", err)
	}
	directory := &fakeDirectory{snapshot: graph.Snapshot{
		User:          domain.User{Present: true, ID: "user-id", Groups: []string{"staff"}},
		ManagedGroups: []string{"overseas_access"},
	}}
	service, err := New(cfg, intentStore, directory, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	service.now = func() time.Time { return now }
	if _, err := service.ReconcileSubject(t.Context(), accepted.Subject); err != nil {
		t.Fatalf("ReconcileSubject() error = %v", err)
	}
	if len(directory.added) != 0 || len(directory.removed) != 0 {
		t.Fatalf("Graph mutations: added=%v removed=%v", directory.added, directory.removed)
	}
}

func TestReconciliationDoesNotRemoveAbsentManagedMembership(t *testing.T) {
	t.Parallel()
	intentStore := testdb.Open(t)
	cfg := loadTestConfig(t)
	now := time.Date(2026, 9, 1, 2, 0, 0, 0, time.UTC)
	accepted := intent.Intent{
		Source: "freshservice", ID: "SR-1", Subject: "staff@example.com",
		StartsAt: now.Add(-2 * time.Hour), EndsAt: now.Add(-time.Hour),
	}
	if err := intentStore.UpsertIntent(t.Context(), accepted, now); err != nil {
		t.Fatalf("UpsertIntent() error = %v", err)
	}
	directory := &fakeDirectory{snapshot: graph.Snapshot{
		User: domain.User{Present: true, ID: "user-id", Groups: []string{"staff"}},
	}}
	service, err := New(cfg, intentStore, directory, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	service.now = func() time.Time { return now }
	if _, err := service.ReconcileSubject(t.Context(), accepted.Subject); err != nil {
		t.Fatalf("ReconcileSubject() error = %v", err)
	}
	if len(directory.added) != 0 || len(directory.removed) != 0 {
		t.Fatalf("Graph mutations: added=%v removed=%v", directory.added, directory.removed)
	}
}

func TestReconciliationUsesLockedConnectionWithSingleConnectionPool(t *testing.T) {
	t.Parallel()
	intentStore := testdb.OpenWithMaxConnections(t, 1)
	cfg := loadTestConfig(t)
	now := time.Date(2026, 9, 1, 0, 30, 0, 0, time.UTC)
	accepted := intent.Intent{
		Source: "freshservice", ID: "SR-1", Subject: "staff@example.com",
		StartsAt: now.Add(-time.Minute), EndsAt: now.Add(time.Hour),
	}
	if err := intentStore.UpsertIntent(t.Context(), accepted, now); err != nil {
		t.Fatalf("UpsertIntent() error = %v", err)
	}
	directory := &fakeDirectory{snapshot: graph.Snapshot{
		User: domain.User{Present: true, ID: "user-id", Groups: []string{"staff"}},
	}}
	service, err := New(cfg, intentStore, directory, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	service.now = func() time.Time { return now }
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()
	if _, err := service.ReconcileSubject(ctx, accepted.Subject); err != nil {
		t.Fatalf("ReconcileSubject() error = %v", err)
	}
}

func TestPlanEventDoesNotModifyPersistedIntentOrGraph(t *testing.T) {
	t.Parallel()
	intentStore := testdb.Open(t)
	cfg := loadTestConfig(t)
	now := time.Date(2026, 9, 1, 0, 30, 0, 0, time.UTC)
	persisted := intent.Intent{
		Source: "freshservice", ID: "SR-1", Subject: "student@example.com",
		StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour),
	}
	if err := intentStore.UpsertIntent(t.Context(), persisted, now); err != nil {
		t.Fatalf("UpsertIntent() error = %v", err)
	}
	directory := &fakeDirectory{snapshot: graph.Snapshot{
		User:          domain.User{Present: true, ID: "user-id", Groups: []string{"students"}},
		ManagedGroups: []string{"overseas_access"},
	}}
	service, err := New(cfg, intentStore, directory, nil)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	service.now = func() time.Time { return now }
	hypothetical := persisted
	hypothetical.Cancelled = true
	plan, err := service.PlanEvent(t.Context(), hypothetical)
	if err != nil {
		t.Fatalf("PlanEvent() error = %v", err)
	}
	if got := plan.Intents[0].Phase; got != intent.PhaseCancelled {
		t.Errorf("planned phase = %q, want %q", got, intent.PhaseCancelled)
	}
	stored, err := intentStore.GetIntent(t.Context(), persisted.Source, persisted.ID)
	if err != nil {
		t.Fatalf("GetIntent() error = %v", err)
	}
	if stored.Cancelled || !stored.StartsAt.Equal(persisted.StartsAt) {
		t.Errorf("stored intent changed during plan: %+v", stored)
	}
	if len(directory.added) != 0 || len(directory.removed) != 0 {
		t.Fatalf("plan mutated Graph: added=%v removed=%v", directory.added, directory.removed)
	}
}

func loadTestConfig(t *testing.T) *config.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	contents := `version: 1
connections:
  microsoft:
    type: microsoft_graph
    tenant_id: tenant
    client_id: client
    client_secret: secret
database:
  url: postgres://localhost/metabasis
webhooks:
  freshservice:
    path: /webhooks/freshservice
    bearer_token: token
identity:
  connection: microsoft
  groups:
    students: [students]
    staff: [staff]
managed_groups:
  mfa_registration: mfa-registration
  overseas_access: overseas-access
rules:
  - name: students
    when: '"students" in user.groups'
    phases:
      pending:
        groups: [mfa_registration]
      active:
        groups: [mfa_registration, overseas_access]
  - name: staff
    when: '"staff" in user.groups'
    phases:
      active:
        groups: [overseas_access]
`
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	return cfg
}
