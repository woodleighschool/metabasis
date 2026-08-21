package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateDefaultsToConfigInCurrentDirectory(t *testing.T) {
	directory := t.TempDir()
	writeCommandConfig(t, directory, "config.yaml", commandConfig)
	t.Chdir(directory)
	command := newRootCommand()
	command.SetArgs([]string{"validate"})
	var output bytes.Buffer
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := output.String(), "configuration valid\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestValidateAcceptsOrderedConfigurationFiles(t *testing.T) {
	directory := t.TempDir()
	basePath := writeCommandConfig(t, directory, "base.yaml", commandConfig)
	overlayPath := writeCommandConfig(t, directory, "overlay.yaml", `reconcile:
  poll_interval: 2m
`)
	command := newRootCommand()
	command.SetArgs([]string{"validate", "--config", basePath, "--config", overlayPath})
	var output bytes.Buffer
	command.SetOut(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if got, want := output.String(), "configuration valid\n"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestReconcileRequiresExactlyOneScope(t *testing.T) {
	t.Parallel()
	tests := [][]string{
		{"reconcile"},
		{"reconcile", "--subject", "user@example.com", "--all"},
	}
	for _, args := range tests {
		command := newRootCommand()
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
