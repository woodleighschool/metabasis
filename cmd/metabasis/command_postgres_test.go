//go:build postgres

package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/intent"
	"github.com/woodleighschool/metabasis/internal/store"
	"github.com/woodleighschool/metabasis/internal/testutil/testdb"
)

func TestIntentsListAndShowCommands(t *testing.T) {
	t.Parallel()
	databaseURL := testdb.Create(t, testDatabaseURL(t))
	configPath := writeCommandConfig(t, t.TempDir(), "config.yaml", fmt.Sprintf(`version: 1
connections:
  microsoft:
    type: microsoft_graph
    tenant_id: tenant
    client_id: client
    client_secret: secret
database:
  url: %s
webhooks:
  freshservice:
    path: /webhooks/freshservice
    bearer_token: token
identity:
  connection: microsoft
  groups:
    students: [student-group]
managed_groups:
  overseas_access: overseas-access
rules:
  - name: students
    when: '"students" in user.groups'
    phases:
      active:
        groups: [overseas_access]
`, databaseURL))
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("config.Load() error = %v", err)
	}
	intentStore, err := store.Open(t.Context(), cfg.Database, true)
	if err != nil {
		t.Fatalf("store.Open() error = %v", err)
	}
	now := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	if err := intentStore.UpsertIntent(t.Context(), intent.Intent{
		Source: "freshservice", ID: "SR-1", Subject: "student@example.com",
		StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour),
	}, now); err != nil {
		t.Fatalf("UpsertIntent() error = %v", err)
	}
	intentStore.Close()

	listCommand := newRootCommand()
	listCommand.SetArgs([]string{"intents", "list", "--config", configPath})
	var listOutput bytes.Buffer
	listCommand.SetOut(&listOutput)
	if err := listCommand.Execute(); err != nil {
		t.Fatalf("intents list error = %v", err)
	}
	if output := listOutput.String(); !strings.Contains(output, "freshservice") || !strings.Contains(output, "SR-1") {
		t.Errorf("intents list output = %q", output)
	}

	showCommand := newRootCommand()
	showCommand.SetArgs([]string{"intents", "show", "freshservice", "SR-1", "--config", configPath})
	var showOutput bytes.Buffer
	showCommand.SetOut(&showOutput)
	if err := showCommand.Execute(); err != nil {
		t.Fatalf("intents show error = %v", err)
	}
	if output := showOutput.String(); !strings.Contains(output, `"source":"freshservice"`) ||
		!strings.Contains(output, `"subject":"student@example.com"`) {
		t.Errorf("intents show output = %q", output)
	}
}

func testDatabaseURL(t *testing.T) string {
	t.Helper()
	value := os.Getenv("METABASIS_TEST_DATABASE_URL")
	if value == "" {
		t.Fatal("METABASIS_TEST_DATABASE_URL is required for PostgreSQL tests")
	}
	return value
}
