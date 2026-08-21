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

func TestBuildUsesFirstMatchingRuleAndActiveStateForOverlappingIntents(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	cfg := testConfig(t)
	user := domain.User{Present: true, Groups: []string{"home_access", "staff", "students"}}
	intents := []intent.Intent{
		{Source: "freshservice", ID: "active", Subject: "student@example.com", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)},
		{Source: "freshservice", ID: "pending", Subject: "student@example.com", StartsAt: now.Add(30 * time.Minute), EndsAt: now.Add(2 * time.Hour)},
	}

	plan, err := Build(cfg, user, intents, now)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got, want := plan.Rule, "students"; got != want {
		t.Errorf("Rule = %q, want %q", got, want)
	}
	if got, want := plan.State, StateActive; got != want {
		t.Errorf("State = %q, want %q", got, want)
	}
	if got, want := plan.PresentGroups, []string{"mfa_registration", "overseas_access", "overseas_mfa"}; !slices.Equal(got, want) {
		t.Errorf("PresentGroups = %v, want %v", got, want)
	}
	if got, want := plan.AbsentGroups, []string{"home_access"}; !slices.Equal(got, want) {
		t.Errorf("AbsentGroups = %v, want %v", got, want)
	}
	if got, want := plan.AddGroups, []string{"mfa_registration", "overseas_access", "overseas_mfa"}; !slices.Equal(got, want) {
		t.Errorf("AddGroups = %v, want %v", got, want)
	}
	if got, want := plan.RemoveGroups, []string{"home_access"}; !slices.Equal(got, want) {
		t.Errorf("RemoveGroups = %v, want %v", got, want)
	}
	if plan.NextTransition == nil || !plan.NextTransition.Equal(now.Add(30*time.Minute)) {
		t.Errorf("NextTransition = %v, want %v", plan.NextTransition, now.Add(30*time.Minute))
	}
}

func TestBuildSettledAssertionsPreserveUnmentionedStaffGroup(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	cfg := testConfig(t)
	intents := []intent.Intent{
		{Source: "freshservice", ID: "ended", Subject: "staff@example.com", StartsAt: now.Add(-2 * time.Hour), EndsAt: now.Add(-time.Hour)},
		{Source: "freshservice", ID: "cancelled", Subject: "staff@example.com", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour), Cancelled: true},
	}

	plan, err := Build(cfg, domain.User{
		Present: true,
		Groups:  []string{"staff", "overseas_access", "overseas_mfa"},
	}, intents, now)
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if got, want := plan.State, StateSettled; got != want {
		t.Errorf("State = %q, want %q", got, want)
	}
	if got, want := plan.AddGroups, []string{"home_access"}; !slices.Equal(got, want) {
		t.Errorf("AddGroups = %v, want %v", got, want)
	}
	if got, want := plan.RemoveGroups, []string{"overseas_access"}; !slices.Equal(got, want) {
		t.Errorf("RemoveGroups = %v, want %v", got, want)
	}
}

func TestBuildWithNoIntentsMakesNoAssertions(t *testing.T) {
	t.Parallel()
	cfg := testConfig(t)
	plan, err := Build(cfg, domain.User{
		Present: true,
		Groups:  []string{"staff", "overseas_access", "overseas_mfa"},
	}, nil, time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("Build() error = %v", err)
	}
	if plan.State != "" || len(plan.PresentGroups) != 0 || len(plan.AbsentGroups) != 0 ||
		len(plan.AddGroups) != 0 || len(plan.RemoveGroups) != 0 {
		t.Fatalf("plan with no intents contains assertions: %+v", plan)
	}
}

func TestBuildAppliesTravelMembershipContract(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 9, 15, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		userGroups []string
		intent     intent.Intent
		wantState  State
		wantAdd    []string
		wantRemove []string
	}{
		{
			name:       "student pending",
			userGroups: []string{"home_access", "students"},
			intent:     intent.Intent{StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour)},
			wantState:  StatePending,
			wantAdd:    []string{"mfa_registration"},
		},
		{
			name:       "student active",
			userGroups: []string{"home_access", "mfa_registration", "students"},
			intent:     intent.Intent{StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)},
			wantState:  StateActive,
			wantAdd:    []string{"overseas_access", "overseas_mfa"},
			wantRemove: []string{"home_access"},
		},
		{
			name:       "student settled",
			userGroups: []string{"mfa_registration", "overseas_access", "overseas_mfa", "students"},
			intent:     intent.Intent{StartsAt: now.Add(-2 * time.Hour), EndsAt: now.Add(-time.Hour)},
			wantState:  StateSettled,
			wantAdd:    []string{"home_access"},
			wantRemove: []string{"mfa_registration", "overseas_access", "overseas_mfa"},
		},
		{
			name:       "staff pending preserves force MFA",
			userGroups: []string{"home_access", "overseas_mfa", "staff"},
			intent:     intent.Intent{StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour)},
			wantState:  StatePending,
		},
		{
			name:       "staff active",
			userGroups: []string{"home_access", "overseas_mfa", "staff"},
			intent:     intent.Intent{StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)},
			wantState:  StateActive,
			wantAdd:    []string{"overseas_access"},
			wantRemove: []string{"home_access"},
		},
		{
			name:       "staff settled preserves force MFA",
			userGroups: []string{"overseas_access", "overseas_mfa", "staff"},
			intent:     intent.Intent{StartsAt: now.Add(-2 * time.Hour), EndsAt: now.Add(-time.Hour)},
			wantState:  StateSettled,
			wantAdd:    []string{"home_access"},
			wantRemove: []string{"overseas_access"},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			accepted := test.intent
			accepted.Source = "freshservice"
			accepted.ID = "SR-1"
			accepted.Subject = "user@example.com"
			plan, err := Build(
				testConfig(t),
				domain.User{Present: true, Groups: test.userGroups},
				[]intent.Intent{accepted},
				now,
			)
			if err != nil {
				t.Fatalf("Build() error = %v", err)
			}
			if plan.State != test.wantState {
				t.Errorf("State = %q, want %q", plan.State, test.wantState)
			}
			if !slices.Equal(plan.AddGroups, test.wantAdd) {
				t.Errorf("AddGroups = %v, want %v", plan.AddGroups, test.wantAdd)
			}
			if !slices.Equal(plan.RemoveGroups, test.wantRemove) {
				t.Errorf("RemoveGroups = %v, want %v", plan.RemoveGroups, test.wantRemove)
			}
		})
	}
}

func testConfig(t *testing.T) *config.Config {
	t.Helper()
	directory := t.TempDir()
	path := directory + "/config.yaml"
	contents := `version: 2
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
    home_access: [home-access]
    mfa_registration: [mfa-registration]
    overseas_access: [overseas-access]
    overseas_mfa: [overseas-mfa]
rules:
  - name: students
    when: '"students" in user.groups'
    states:
      pending:
        present: [home_access, mfa_registration]
        absent: [overseas_access, overseas_mfa]
      active:
        present: [mfa_registration, overseas_access, overseas_mfa]
        absent: [home_access]
      settled:
        present: [home_access]
        absent: [mfa_registration, overseas_access, overseas_mfa]
  - name: staff
    when: '"staff" in user.groups'
    states:
      pending:
        present: [home_access]
        absent: [overseas_access]
      active:
        present: [overseas_access, overseas_mfa]
        absent: [home_access]
      settled:
        present: [home_access]
        absent: [overseas_access]
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
