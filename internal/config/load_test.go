package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

const validConfig = `version: 1
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
managed_groups:
  overseas_access: site-overseas
`)
	rules := writeConfig(t, "rules.yaml", `rules:
  - name: everyone
    when: user.present
    phases:
      active:
        groups: [overseas_access]
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
	if got, want := cfg.ManagedGroups["overseas_access"], "site-overseas"; got != want {
		t.Errorf("overseas_access = %q, want %q", got, want)
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

func TestLoadRejectsUnknownManagedGroupAndInvalidCEL(t *testing.T) {
	t.Parallel()
	unknownGroup := writeConfig(t, "unknown-group.yaml", strings.Replace(
		validConfig,
		"groups: [mfa_registration, overseas_access]",
		"groups: [mfa_registration, unowned]",
		1,
	))
	_, err := load([]string{unknownGroup}, func(string) (string, bool) { return "secret", true })
	if err == nil || !strings.Contains(err.Error(), `unknown managed group "unowned"`) {
		t.Fatalf("unknown managed group error = %v", err)
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

func writeConfig(t *testing.T, name, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
