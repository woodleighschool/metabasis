package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/woodleighschool/metabasis/internal/intent"
	"github.com/woodleighschool/metabasis/internal/planner"
	"github.com/woodleighschool/metabasis/internal/reconcile"
	"github.com/woodleighschool/metabasis/internal/store"
)

func TestApplyReportPreservesPartialOutcomeAndWholeRunTotals(t *testing.T) {
	results := []reconcile.Result{
		{Subject: "unchanged@example.invalid", Plan: &planner.Plan{
			Subject: "unchanged@example.invalid", Rule: "students", State: planner.StateActive,
			Intents:       []planner.IntentPhase{{Source: "freshservice", ID: "SR-1", Phase: intent.PhaseActive}},
			PresentGroups: []string{"allow_overseas"}, CurrentGroups: []string{"allow_overseas", "students"},
		}},
		{
			Subject: "student@example.invalid",
			Plan: &planner.Plan{
				Rule: "students", State: planner.StateActive,
				Intents:       []planner.IntentPhase{{Source: "freshservice", ID: "SR-2", Phase: intent.PhaseActive}},
				PresentGroups: []string{"allow_overseas", "force_mfa"}, AbsentGroups: []string{"block_outside_australia"},
				CurrentGroups: []string{"block_outside_australia", "students"},
				AddGroups:     []string{"allow_overseas", "force_mfa"}, RemoveGroups: []string{"block_outside_australia"},
			},
			AddedGroups:     []string{"allow_overseas"},
			FailedOperation: &reconcile.MembershipOperation{Action: "add", Group: "force_mfa"},
			Error:           "add group force_mfa: membership unavailable",
		},
	}
	for _, includeUnchanged := range []bool{false, true} {
		var output bytes.Buffer
		runErr := errors.New("membership unavailable")
		if err := writeApplyReport(&output, true, includeUnchanged, results, runErr); err != nil {
			t.Fatal(err)
		}
		var report applyReport
		if err := json.Unmarshal(output.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		wantSubjects := 2
		if report.Error == "" || len(report.Subjects) != wantSubjects || report.Totals.Subjects != 2 || report.Totals.Failed != 1 || report.Totals.Unchanged != 1 || report.Totals.Added != 1 {
			t.Fatalf("report = %#v", report)
		}
		failed := report.Subjects[len(report.Subjects)-1]
		if len(failed.Plan.AddGroups) != 2 || len(failed.AddedGroups) != 1 || failed.FailedOperation.Group != "force_mfa" {
			t.Fatalf("partial result = %#v", failed)
		}
		output.Reset()
		stream := &commandOutput{out: io.Discard}
		for _, result := range results {
			if err := stream.subjectDone(&output, result, includeUnchanged); err != nil {
				t.Fatal(err)
			}
		}
		if err := writeApplyReport(&output, false, includeUnchanged, results, runErr); err != nil {
			t.Fatal(err)
		}
		text := output.String()
		for _, want := range []string{
			"Subject: student@example.invalid (failed)", "State: active", "Added: allow_overseas",
			"Failed: add force_mfa", "Not attempted: remove block_outside_australia",
			"2 total, 0 applied, 1 unchanged, 1 failed", "Completed: 1 added, 0 removed",
		} {
			if !strings.Contains(text, want) {
				t.Errorf("missing %q: %s", want, text)
			}
		}
		if strings.Contains(text, "Subject: unchanged@example.invalid") != includeUnchanged || strings.Contains(text, "Added: force_mfa") {
			t.Fatalf("selection or completed writes incorrect: %s", text)
		}
	}
}

func TestPlanReportsSatisfiedAssertionsWithoutWrites(t *testing.T) {
	next := time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)
	plan := planner.Plan{
		Subject: "student@example.invalid", Rule: "students", State: planner.StateActive,
		Intents:       []planner.IntentPhase{{Source: "freshservice", ID: "SR-123", Phase: intent.PhaseActive}},
		PresentGroups: []string{"allow_overseas", "force_mfa"}, AbsentGroups: []string{"block_outside_australia"},
		CurrentGroups: []string{"allow_overseas", "force_mfa", "students"}, NextTransition: &next,
	}
	var output bytes.Buffer
	if err := writePlan(&output, false, plan, nil); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"Rule: students", "State: active", "freshservice/SR-123: active", "allow_overseas: present (satisfied)",
		"force_mfa: present (satisfied)", "block_outside_australia: absent (satisfied)",
		"memberships preserved: students", "Next intent boundary: 2026-09-27T08:00:00Z", "add 0, remove 0. Event not saved.",
	} {
		if !strings.Contains(output.String(), want) {
			t.Errorf("missing %q: %s", want, output.String())
		}
	}
	if strings.Contains(output.String(), "->") {
		t.Fatalf("invented a transition: %s", output.String())
	}
}

func TestPlanReportDoesNotInventAPlanAfterFailure(t *testing.T) {
	var output bytes.Buffer
	if err := writePlan(&output, true, planner.Plan{}, errors.New("directory unavailable")); err != nil {
		t.Fatal(err)
	}
	var report planReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Plan != nil || report.Error != "directory unavailable" {
		t.Fatalf("report = %#v", report)
	}
}

func TestEmptyApplyReportKeepsTotalsAndArray(t *testing.T) {
	var output bytes.Buffer
	if err := writeApplyReport(&output, true, false, []reconcile.Result{{Subject: "unchanged@example.invalid"}}, nil); err != nil {
		t.Fatal(err)
	}
	var report applyReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Subjects == nil || len(report.Subjects) != 1 || report.Totals.Subjects != 1 || report.Totals.Unchanged != 1 {
		t.Fatalf("empty selection = %#v", report)
	}
}

func TestIntentListJSONIncludesCurrentPhases(t *testing.T) {
	now := time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC)
	intents := []intent.Intent{
		{ID: "pending", StartsAt: now.Add(time.Hour), EndsAt: now.Add(2 * time.Hour)},
		{ID: "active", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour)},
	}
	var output bytes.Buffer
	if err := writeIntents(&output, true, intents, now); err != nil {
		t.Fatal(err)
	}
	var report struct {
		Intents []struct {
			ID    string       `json:"id"`
			Phase intent.Phase `json:"phase"`
		} `json:"intents"`
	}
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if len(report.Intents) != 2 || report.Intents[0].Phase != intent.PhasePending || report.Intents[1].Phase != intent.PhaseActive {
		t.Fatalf("intent phases = %#v", report.Intents)
	}
}

func TestHumanReportsEscapeControlsAndKeepPrintableText(t *testing.T) {
	accepted := intent.Intent{Source: "webhook", ID: "request\t2", Subject: "Zoë\x1b[31m\n@example.invalid"}
	plan := planner.Plan{Subject: accepted.Subject, Rule: "équipe", PresentGroups: []string{"MFA\nrequired"}, AddGroups: []string{"MFA\nrequired"}}
	for name, render := range map[string]func(*bytes.Buffer) error{
		"plan": func(out *bytes.Buffer) error { return writePlan(out, false, plan, nil) },
		"apply": func(out *bytes.Buffer) error {
			result := reconcile.Result{Subject: accepted.Subject, Plan: &plan, Error: "failed\x1b[0m"}
			if err := (&commandOutput{out: io.Discard}).subjectDone(out, result, true); err != nil {
				return err
			}
			return writeApplyReport(out, false, true, []reconcile.Result{result}, errors.New("failed"))
		},
		"list": func(out *bytes.Buffer) error { return writeIntents(out, false, []intent.Intent{accepted}, time.Time{}) },
		"show": func(out *bytes.Buffer) error {
			return writeIntent(out, false, accepted, store.State{LastError: "failed\r\n"}, time.Time{})
		},
	} {
		t.Run(name, func(t *testing.T) {
			var output bytes.Buffer
			if err := render(&output); err != nil {
				t.Fatal(err)
			}
			for _, b := range output.Bytes() {
				if b == 0x7f || b < 32 && b != '\n' {
					t.Fatalf("report contains terminal controls: %q", output.String())
				}
			}
			if !strings.Contains(output.String(), "Zoë\\x1b[31m\\n@example.invalid") {
				t.Fatalf("subject was not safely represented: %q", output.String())
			}
		})
	}
}
