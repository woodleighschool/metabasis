package main

import (
	"bytes"
	"io"
	"log/slog"
	"strings"
	"testing"

	"time"

	"github.com/woodleighschool/metabasis/internal/reconcile"
)

func TestSubjectStagesStayUntilTheSubjectFinishes(t *testing.T) {
	var report, logs bytes.Buffer
	output := &commandOutput{out: io.Discard, interactive: true}
	logger := slog.New(&activityHandler{Handler: slog.NewJSONHandler(&logs, &slog.HandlerOptions{Level: &output.threshold}), output: output}).With("subject", "student@example.invalid")
	logger.Info("Resolving identity", "stage", true)
	logger.Info("Resolving identity", "stage_result", true, "detail", "student@example.invalid")
	logger.Info("Applying memberships", "stage", true, "total", 2, "unit", "changes")
	logger.Info("Applying memberships", "progress", true, "current", 1, "total", 2, "unit", "changes")
	live := strings.Split(output.progress.view(time.Now().Add(time.Second), 100, 20), "\n")
	if len(live) != 2 || live[0] != "student@example.invalid" || strings.Contains(live[1], "Resolving identity") ||
		!strings.Contains(live[1], "Applying memberships  "+strings.Repeat("━", 10)+strings.Repeat("─", 10)+"  1 / 2 changes  (1s)") {
		t.Fatalf("subject tree: %q", live)
	}
	if err := output.subjectDone(&report, reconcile.Result{Subject: "student@example.invalid", AddedGroups: []string{"allow_overseas"}}, false); err != nil {
		t.Fatal(err)
	}
	if len(output.progress.groups) != 0 || report.String() != "Subject: student@example.invalid (applied)\n  Added: allow_overseas\n\n" || logs.Len() != 0 {
		t.Fatalf("finished subject: live=%v report=%q logs=%q", output.progress.groups, report.String(), logs.String())
	}
}

func TestFinishedSubjectsLeaveAnOutcomeLineInATerminal(t *testing.T) {
	for _, interactive := range []bool{true, false} {
		var report bytes.Buffer
		output := &commandOutput{out: io.Discard, interactive: interactive}
		if err := output.subjectDone(reportWriter{Writer: &report, output: output}, reconcile.Result{Subject: "unchanged@example.invalid"}, false); err != nil {
			t.Fatal(err)
		}
		want := ""
		if report.String() != want || output.interactive != interactive {
			t.Fatalf("interactive=%v: report %q, still interactive %v", interactive, report.String(), output.interactive)
		}
	}
}

func TestOverlappingActivitiesFinishIndependently(t *testing.T) {
	p := newTerminalProgress(io.Discard)
	p.update(activity{scope: "example", label: "First operation", stage: true})
	p.update(activity{scope: "example", label: "Second operation", stage: true})
	p.update(activity{scope: "example", label: "First operation", status: true})
	got := p.view(time.Now().Add(time.Second), 100, 28)
	if !strings.Contains(got, "Second operation") || strings.Contains(got, "First operation") {
		t.Fatal(got)
	}
	p.update(activity{scope: "example", label: "Second operation", status: true})
	if got := p.view(time.Now().Add(time.Second), 100, 28); got != "" {
		t.Fatal(got)
	}
}

func TestPlainProgressIsSparseAndEndsWithWork(t *testing.T) {
	var out bytes.Buffer
	p := newTerminalProgress(&out)
	p.plain = true
	p.update(activity{label: "Reading inventory", stage: true})
	now := time.Now()
	p.draw(now)
	if out.Len() != 0 {
		t.Fatal("fast work printed")
	}
	p.draw(now.Add(3 * time.Second))
	first := out.String()
	if !strings.Contains(first, "Reading inventory") || strings.Contains(first, "\x1b") {
		t.Fatalf("milestone: %q", first)
	}
	p.draw(now.Add(4 * time.Second))
	if out.String() != first {
		t.Fatal("duplicate milestone")
	}
	p.update(activity{label: "Reading inventory", status: true})
	p.draw(now.Add(time.Minute))
	p.stop()
	if out.String() != first {
		t.Fatal("completed work printed")
	}
}
