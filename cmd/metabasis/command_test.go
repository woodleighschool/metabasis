package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateWritesOneJSONReport(t *testing.T) {
	path := writeCommandConfig(t, t.TempDir(), "config.yaml", commandConfig)
	command, output := newRootCommand()
	var report, diagnostics bytes.Buffer
	command.SetOut(&report)
	command.SetErr(&diagnostics)
	command.SetArgs([]string{"validate", "--config", path, "--json"})
	executed, err := command.ExecuteC()
	output.finish(executed, err)
	if err != nil {
		t.Fatal(err)
	}
	var result struct {
		Valid bool `json:"valid"`
	}
	if err := json.Unmarshal(report.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if !result.Valid || diagnostics.Len() != 0 {
		t.Fatalf("result = %#v, diagnostics = %q", result, diagnostics.String())
	}
}

func TestValidateDefaultsToConfigInCurrentDirectory(t *testing.T) {
	directory := t.TempDir()
	writeCommandConfig(t, directory, "config.yaml", commandConfig)
	t.Chdir(directory)
	command, _ := newRootCommand()
	command.SetArgs([]string{"validate"})
	var output bytes.Buffer
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := output.String(), "✓ Configuration is valid.\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestValidateAcceptsOrderedConfigurationFiles(t *testing.T) {
	directory := t.TempDir()
	basePath := writeCommandConfig(t, directory, "base.yaml", commandConfig)
	overlayPath := writeCommandConfig(t, directory, "overlay.yaml", `reconcile:
  poll_interval: 2m
`)
	command, _ := newRootCommand()
	command.SetArgs([]string{"validate", "--config", basePath, "--config", overlayPath})
	var output bytes.Buffer
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := output.String(), "✓ Configuration is valid.\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestApplyRequiresExactlyOneScope(t *testing.T) {
	t.Parallel()
	tests := [][]string{
		{"apply"},
		{"apply", "--subject", "user@example.com", "--all"},
	}
	for _, args := range tests {
		command, _ := newRootCommand()
		command.SetArgs(args)
		err := command.Execute()
		if err == nil || !strings.Contains(err.Error(), "set exactly one of --subject or --all") {
			t.Errorf("Execute(%v) error = %v", args, err)
		}
	}
}

func writeCommandConfig(t *testing.T, directory, name, contents string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

const commandConfig = `version: 2
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
    students: [student-group]
    overseas_access: [overseas-access]
rules:
  - name: students
    when: '"students" in user.groups'
    states:
      active:
        present: [overseas_access]
`
