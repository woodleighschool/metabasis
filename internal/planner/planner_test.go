package planner

import (
	"os"
	"slices"
	"testing"
	"time"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/domain"
	"github.com/woodleighschool/metabasis/internal/intent"
)

func TestBuildUsesFirstMatchingRuleAndUnionsOverlappingIntents(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	cfg := testConfig(t)
	user := domain.User{Present: true, Groups: []string{"staff", "students"}}
	intents := []intent.Intent{
		{Source: "freshservice", ID: "active", Subject: "student@example.com", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)},
		{Source: "freshservice", ID: "pending", Subject: "student@example.com", StartsAt: now.Add(30 * time.Minute), EndsAt: now.Add(2 * time.Hour)},
	}

	plan, err := Build(cfg, user, intents, []string{"unowned-not-supplied", "overseas_access"}, now)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got, want := plan.Rule, "students"; got != want {
		t.Errorf("Rule = %q, want %q", got, want)
	}
	if got, want := plan.DesiredGroups, []string{"mfa_registration", "overseas_access", "overseas_mfa"}; !slices.Equal(got, want) {
		t.Errorf("DesiredGroups = %v, want %v", got, want)
	}
	if got, want := plan.AddGroups, []string{"mfa_registration", "overseas_mfa"}; !slices.Equal(got, want) {
		t.Errorf("AddGroups = %v, want %v", got, want)
	}
	if len(plan.RemoveGroups) != 0 {
		t.Errorf("RemoveGroups = %v, want none because unowned groups are outside the plan", plan.RemoveGroups)
	}
	if plan.NextTransition == nil || !plan.NextTransition.Equal(now.Add(30*time.Minute)) {
		t.Errorf("NextTransition = %v, want %v", plan.NextTransition, now.Add(30*time.Minute))
	}
}

func TestBuildEndedAndCancelledIntentsRemoveOnlySuppliedManagedGroups(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	cfg := testConfig(t)
	intents := []intent.Intent{
		{Source: "freshservice", ID: "ended", Subject: "student@example.com", StartsAt: now.Add(-2 * time.Hour), EndsAt: now.Add(-time.Hour)},
		{Source: "freshservice", ID: "cancelled", Subject: "student@example.com", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour), Cancelled: true},
	}

	plan, err := Build(cfg, domain.User{Present: true, Groups: []string{"students"}}, intents, []string{"overseas_access"}, now)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if len(plan.DesiredGroups) != 0 {
		t.Errorf("DesiredGroups = %v, want none", plan.DesiredGroups)
	}
	if got, want := plan.RemoveGroups, []string{"overseas_access"}; !slices.Equal(got, want) {
		t.Errorf("RemoveGroups = %v, want %v", got, want)
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	directory := t.TempDir()
	path := directory + "/config.yaml"
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
  overseas_mfa: overseas-mfa
rules:
  - name: students
    when: '"students" in user.groups'
    phases:
      pending:
        groups: [mfa_registration]
      active:
        groups: [mfa_registration, overseas_access, overseas_mfa]
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
