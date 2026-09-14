package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/woodleighschool/metabasis/internal/planner"
	"github.com/woodleighschool/metabasis/internal/reconcile"
)

func TestApplyReportDistinguishesPlannedAndCompletedChanges(t *testing.T) {
	results := []reconcile.Result{{
		Subject: "student@example.invalid", Plan: &planner.Plan{AddGroups: []string{"first", "second"}},
		AddedGroups: []string{"first"}, Error: "membership unavailable",
	}}
	var output bytes.Buffer
	runErr := errors.New("membership unavailable")
	if err := writeApplyReport(&output, "json", results, runErr); err != nil {
		t.Fatal(err)
	}
	var report applyReport
	if err := json.Unmarshal(output.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Error == "" || len(report.Subjects) != 1 || len(report.Subjects[0].AddedGroups) != 1 || len(report.Subjects[0].Plan.AddGroups) != 2 {
		t.Fatalf("report = %#v", report)
	}
	output.Reset()
	if err := writeApplyReport(&output, "text", results, runErr); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(output.String(), "second") || !strings.Contains(output.String(), "first") || !strings.Contains(output.String(), "1 total, 0 applied, 0 unchanged, 1 failed") {
		t.Fatalf("report = %s", output.String())
	}
}

func TestPlanReportDoesNotInventAPlanAfterFailure(t *testing.T) {
	var output bytes.Buffer
	if err := writePlan(&output, "json", planner.Plan{}, errors.New("directory unavailable")); err != nil {
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
