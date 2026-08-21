package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const validConfig = `version: 2
connections:
  microsoft:
    type: microsoft_graph
    tenant_id: tenant
    client_id: client
    client_secret: ${CLIENT_SECRET}
database:
  url: postgres://localhost/metabasis
webhooks:
  freshservice:
    path: /webhooks/freshservice
    bearer_token: token
identity:
  connection: microsoft
  groups:
    students: [student-group]
    staff: [staff-group]
    home_access: [home-access]
    mfa_registration: [mfa-registration]
    overseas_access: [overseas-access]
rules:
  - name: students
    when: '"students" in user.groups'
    states:
      pending:
        present: [home_access, mfa_registration]
        absent: [overseas_access]
      active:
        present: [mfa_registration, overseas_access]
        absent: [home_access]
      settled:
        present: [home_access]
        absent: [mfa_registration, overseas_access]
  - name: staff
    when: '"staff" in user.groups'
    states:
      active:
        present: [overseas_access]
        absent: [home_access]
`

func TestLoadStrictConfigurationAndDefaults(t *testing.T) {
	t.Parallel()
	path := writeConfig(t, "config.yaml", validConfig)
	cfg, err := load([]string{path}, func(name string) (string, bool) {
		if name == "CLIENT_SECRET" {
			return "secret:with#yaml", true
		}
		return "", false
	})
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if got, want := cfg.Connections["microsoft"].ClientSecret, "secret:with#yaml"; got != want {
		t.Errorf("ClientSecret = %q, want %q", got, want)
	}
	if got, want := cfg.Listen, ":8080"; got != want {
		t.Errorf("Listen = %q, want %q", got, want)
	}
	if got, want := cfg.MetricsListen, ":8081"; got != want {
		t.Errorf("MetricsListen = %q, want %q", got, want)
	}
	if got, want := cfg.Reconcile.PollInterval.Duration, time.Minute; got != want {
		t.Errorf("PollInterval = %v, want %v", got, want)
	}
	if got, want := len(cfg.Programs), 2; got != want {
		t.Errorf("compiled programs = %d, want %d", got, want)
	}
}

func TestLoadAppliesOrderedOverlays(t *testing.T) {
	t.Parallel()
	base := writeConfig(t, "base.yaml", validConfig)
	groups := writeConfig(t, "groups.yaml", `identity:
  groups:
    students: [site-students]
    overseas_access: [site-overseas]
`)
	rules := writeConfig(t, "rules.yaml", `rules:
  - name: everyone
    when: user.present
    states:
      active:
        present: [overseas_access]
`)
	cfg, err := load([]string{base, groups, rules}, func(name string) (string, bool) {
		return "secret", name == "CLIENT_SECRET"
	})
	if err != nil {
		t.Fatalf("load() error = %v", err)
	}
	if got, want := cfg.Identity.Groups["students"], []string{"site-students"}; !slices.Equal(got, want) {
		t.Errorf("students = %v, want %v", got, want)
	}
	if got, want := cfg.Identity.Groups["staff"], []string{"staff-group"}; !slices.Equal(got, want) {
		t.Errorf("staff = %v, want %v", got, want)
	}
	if got, want := cfg.Identity.Groups["overseas_access"], []string{"site-overseas"}; !slices.Equal(got, want) {
		t.Errorf("overseas_access = %v, want %v", got, want)
	}
	if got, want := len(cfg.Rules), 1; got != want || cfg.Rules[0].Name != "everyone" {
		t.Errorf("rules = %+v, want replacement everyone rule", cfg.Rules)
	}
}

func TestLoadRejectsUnknownFieldsAndPartialEnvironmentPlaceholders(t *testing.T) {
	t.Parallel()
	unknown := writeConfig(t, "unknown.yaml", validConfig+"obsolete_web_ui: true\n")
	_, err := load([]string{unknown}, func(string) (string, bool) { return "secret", true })
	if err == nil || !strings.Contains(err.Error(), "field obsolete_web_ui not found") {
		t.Fatalf("unknown field error = %v", err)
	}

	partial := writeConfig(t, "partial.yaml", strings.Replace(validConfig, "${CLIENT_SECRET}", "prefix-${CLIENT_SECRET}", 1))
	_, err = load([]string{partial}, func(string) (string, bool) { return "secret", true })
	if err == nil || !strings.Contains(err.Error(), "must occupy an entire YAML scalar") {
		t.Fatalf("partial placeholder error = %v", err)
	}
}

func TestLoadRejectsUnknownGroupAliasAndInvalidCEL(t *testing.T) {
	t.Parallel()
	unknownGroup := writeConfig(t, "unknown-group.yaml", strings.Replace(
		validConfig,
		"present: [mfa_registration, overseas_access]",
		"present: [mfa_registration, unknown]",
		1,
	))
	_, err := load([]string{unknownGroup}, func(string) (string, bool) { return "secret", true })
	if err == nil || !strings.Contains(err.Error(), `unknown identity group alias "unknown"`) {
		t.Fatalf("unknown group alias error = %v", err)
	}

	invalidCEL := writeConfig(t, "invalid-cel.yaml", strings.Replace(
		validConfig,
		`'"students" in user.groups'`,
		`user.missing_field`,
		1,
	))
	_, err = load([]string{invalidCEL}, func(string) (string, bool) { return "secret", true })
	if err == nil || !strings.Contains(err.Error(), "compile CEL expression") {
		t.Fatalf("invalid CEL error = %v", err)
	}
}

func TestLoadRejectsAmbiguousAndConflictingGroupAssertions(t *testing.T) {
	t.Parallel()
	ambiguous := writeConfig(t, "ambiguous.yaml", strings.Replace(
		validConfig,
		"overseas_access: [overseas-access]",
		"overseas_access: [overseas-access, second-overseas-access]",
		1,
	))
	_, err := load([]string{ambiguous}, func(string) (string, bool) { return "secret", true })
	if err == nil || !strings.Contains(err.Error(), `identity group alias "overseas_access" must resolve to exactly one group ID`) {
		t.Fatalf("ambiguous group assertion error = %v", err)
	}

	conflicting := writeConfig(t, "conflicting.yaml", strings.Replace(
		validConfig,
		"absent: [home_access]",
		"absent: [home_access, overseas_access]",
		1,
	))
	_, err = load([]string{conflicting}, func(string) (string, bool) { return "secret", true })
	if err == nil || !strings.Contains(err.Error(), `both requires and forbids identity group alias "overseas_access"`) {
		t.Fatalf("conflicting group assertion error = %v", err)
	}
}

func writeConfig(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
