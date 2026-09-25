package main

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"

	"github.com/woodleighschool/metabasis/internal/intent"
	"github.com/woodleighschool/metabasis/internal/planner"
	"github.com/woodleighschool/metabasis/internal/reconcile"
	"github.com/woodleighschool/metabasis/internal/store"
)

type planReport struct {
	Plan  *planner.Plan `json:"plan,omitempty"`
	Error string        `json:"error,omitempty"`
}

type applyTotals struct {
	Subjects  int `json:"subjects"`
	Applied   int `json:"applied"`
	Unchanged int `json:"unchanged"`
	Failed    int `json:"failed"`
	Added     int `json:"added"`
	Removed   int `json:"removed"`
}

type applyReport struct {
	Subjects []reconcile.Result `json:"subjects"`
	Totals   applyTotals        `json:"totals"`
	Error    string             `json:"error,omitempty"`
}

func writePlan(writer io.Writer, jsonOutput bool, plan planner.Plan, planErr error) error {
	report := planReport{Plan: &plan}
	if planErr != nil {
		report.Plan = nil
		report.Error = planErr.Error()
	}
	if jsonOutput {
		return writeJSON(writer, report)
	}
	style := newTextStyle(writer)
	var text strings.Builder
	if planErr != nil {
		fmt.Fprintf(&text, "%s %s\n", style.paint("Plan failed:", color.FgHiRed), reportText(planErr.Error()))
	} else {
		fmt.Fprintf(&text, "%s %s\n", style.paint("Plan:", color.Bold), style.paint(reportText(plan.Subject), color.Bold))
		writePlanContext(&text, plan)
		if len(plan.Intents) > 0 {
			fmt.Fprintln(&text, "  Intents:")
			for _, accepted := range plan.Intents {
				fmt.Fprintf(&text, "    %s/%s: %s\n", reportText(accepted.Source), reportText(accepted.ID), accepted.Phase)
			}
		}
		if len(plan.PresentGroups)+len(plan.AbsentGroups) > 0 {
			fmt.Fprintln(&text, "  Membership assertions:")
			for _, group := range plan.PresentGroups {
				state := style.paint("present (satisfied)", color.Faint)
				if !slices.Contains(plan.CurrentGroups, group) {
					state = style.paint("absent -> present (add)", color.FgHiGreen)
				}
				fmt.Fprintf(&text, "    %s: %s\n", reportText(group), state)
			}
			for _, group := range plan.AbsentGroups {
				state := style.paint("absent (satisfied)", color.Faint)
				if slices.Contains(plan.CurrentGroups, group) {
					state = style.paint("present -> absent (remove)", color.FgHiRed)
				}
				fmt.Fprintf(&text, "    %s: %s\n", reportText(group), state)
			}
		} else {
			fmt.Fprintf(&text, "  %s\n", noChangeReason(plan))
		}
		var preserved []string
		for _, group := range plan.CurrentGroups {
			if !slices.Contains(plan.PresentGroups, group) && !slices.Contains(plan.AbsentGroups, group) {
				preserved = append(preserved, group)
			}
		}
		if len(preserved) > 0 {
			fmt.Fprintf(&text, "  Other configured memberships preserved: %s\n", reportText(strings.Join(preserved, ", ")))
		}
		fmt.Fprintf(&text, "  Next intent boundary: %s\n", reportTime(plan.NextTransition))
		fmt.Fprintf(&text, "\n%s add %d, remove %d. Event not saved.\n", style.paint("Proposed:", color.Bold), len(plan.AddGroups), len(plan.RemoveGroups))
	}
	_, err := io.WriteString(writer, text.String())
	return err
}

func writePlanContext(text *strings.Builder, plan planner.Plan) {
	if plan.User.UserPrincipalName != "" && plan.User.UserPrincipalName != plan.Subject {
		fmt.Fprintf(text, "  Resolved: %s\n", reportText(plan.User.UserPrincipalName))
	}
	rule := plan.Rule
	if rule == "" {
		rule = "none (no matching rule)"
	}
	state := string(plan.State)
	if state == "" {
		state = "none (no accepted intents)"
	}
	fmt.Fprintf(text, "  Rule: %s\n  State: %s\n", reportText(rule), reportText(state))
}

func noChangeReason(plan planner.Plan) string {
	switch {
	case len(plan.Intents) == 0:
		return "No accepted intents; no membership assertions."
	case plan.Rule == "":
		return "No matching rule; no membership assertions."
	case len(plan.PresentGroups)+len(plan.AbsentGroups) == 0:
		return "No membership assertions for this state."
	default:
		return "All membership assertions satisfied."
	}
}

func writeIntents(writer io.Writer, jsonOutput bool, intents []intent.Intent, now time.Time) error {
	if jsonOutput {
		type listedIntent struct {
			intent.Intent
			Phase intent.Phase `json:"phase"`
		}
		rows := make([]listedIntent, 0, len(intents))
		for _, accepted := range intents {
			rows = append(rows, listedIntent{Intent: accepted, Phase: accepted.PhaseAt(now)})
		}
		return writeJSON(writer, map[string]any{"intents": rows})
	}
	if len(intents) == 0 {
		_, err := fmt.Fprintln(writer, "No accepted intents.")
		return err
	}
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "SOURCE\tID\tSUBJECT\tPHASE\tSTARTS\tENDS"); err != nil {
		return err
	}
	for _, accepted := range intents {
		if _, err := fmt.Fprintf(table, "%s\t%s\t%s\t%s\t%s\t%s\n",
			reportText(accepted.Source), reportText(accepted.ID), reportText(accepted.Subject), accepted.PhaseAt(now),
			accepted.StartsAt.Format(time.RFC3339), accepted.EndsAt.Format(time.RFC3339)); err != nil {
			return err
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(writer, "\n%s %d\n", newTextStyle(writer).paint("Intents:", color.Bold), len(intents))
	return err
}

// writeApplyReport ends an apply: the JSON document, or the totals below the
// subjects that streamed as they finished.
func writeApplyReport(writer io.Writer, jsonOutput, includeUnchanged bool, results []reconcile.Result, runErr error) error {
	report := applyReport{Subjects: []reconcile.Result{}}
	for _, result := range results {
		report.Totals.Subjects++
		report.Totals.Added += len(result.AddedGroups)
		report.Totals.Removed += len(result.RemovedGroups)
		switch subjectStatus(result) {
		case "failed":
			report.Totals.Failed++
		case "applied":
			report.Totals.Applied++
		default:
			report.Totals.Unchanged++
			if !jsonOutput && !includeUnchanged {
				continue
			}
		}
		report.Subjects = append(report.Subjects, result)
	}
	if runErr != nil {
		report.Error = runErr.Error()
	}
	if jsonOutput {
		return writeJSON(writer, report)
	}
	style := newTextStyle(writer)
	var text strings.Builder
	if runErr != nil && report.Totals.Failed == 0 {
		fmt.Fprintf(&text, "%s %s\n\n", style.paint("Apply failed:", color.FgHiRed), reportText(runErr.Error()))
	}
	fmt.Fprintf(&text, "%s %d total, %d applied, %d unchanged, %s; %d shown\n",
		style.paint("Subjects:", color.Bold), report.Totals.Subjects, report.Totals.Applied, report.Totals.Unchanged, failedCount(style, report.Totals.Failed), len(report.Subjects))
	fmt.Fprintf(&text, "%s %d added, %d removed\n", style.paint("Completed:", color.Bold), report.Totals.Added, report.Totals.Removed)
	_, err := io.WriteString(writer, text.String())
	return err
}

func failedCount(style textStyle, failed int) string {
	text := fmt.Sprintf("%d failed", failed)
	if failed == 0 {
		return text
	}
	return style.paint(text, color.FgHiRed)
}

// subjectStatus names a subject's outcome.
func subjectStatus(result reconcile.Result) string {
	switch {
	case result.Error != "":
		return "failed"
	case len(result.AddedGroups)+len(result.RemovedGroups) > 0:
		return "applied"
	}
	return "unchanged"
}

// subjectHeading names a subject and its outcome.
func subjectHeading(style textStyle, result reconcile.Result) string {
	status := subjectStatus(result)
	attribute := map[string]color.Attribute{"failed": color.FgHiRed, "applied": color.FgHiGreen, "unchanged": color.Faint}[status]
	return fmt.Sprintf("%s %s (%s)\n", style.paint("Subject:", color.Bold), style.paint(reportText(result.Subject), color.Bold), style.paint(status, attribute))
}

// renderSubject renders one subject's report block.
func renderSubject(style textStyle, result reconcile.Result) string {
	var text strings.Builder
	text.WriteString(subjectHeading(style, result))
	if result.Plan != nil {
		writePlanContext(&text, *result.Plan)
	}
	for _, group := range result.AddedGroups {
		fmt.Fprintf(&text, "  %s %s\n", style.paint("Added:", color.FgHiGreen), reportText(group))
	}
	for _, group := range result.RemovedGroups {
		fmt.Fprintf(&text, "  %s %s\n", style.paint("Removed:", color.FgHiRed), reportText(group))
	}
	if result.FailedOperation != nil {
		fmt.Fprintf(&text, "  %s %s %s\n", style.paint("Failed:", color.FgHiRed), reportText(result.FailedOperation.Action), reportText(result.FailedOperation.Group))
	}
	if result.Plan != nil {
		writeOutstanding(&text, style, "add", result.Plan.AddGroups, result.AddedGroups, result.FailedOperation)
		writeOutstanding(&text, style, "remove", result.Plan.RemoveGroups, result.RemovedGroups, result.FailedOperation)
		if subjectStatus(result) == "unchanged" {
			fmt.Fprintf(&text, "  %s\n", noChangeReason(*result.Plan))
		}
		fmt.Fprintf(&text, "  Next intent boundary: %s\n", reportTime(result.Plan.NextTransition))
	}
	if result.Error != "" {
		fmt.Fprintf(&text, "  %s %s\n", style.paint("Error:", color.FgHiRed), reportText(result.Error))
	}
	text.WriteByte('\n')
	return text.String()
}

func writeOutstanding(text *strings.Builder, style textStyle, action string, planned, completed []string, failed *reconcile.MembershipOperation) {
	for _, group := range planned {
		if slices.Contains(completed, group) || failed != nil && failed.Action == action && failed.Group == group {
			continue
		}
		fmt.Fprintf(text, "  %s %s %s\n", style.paint("Not attempted:", color.FgHiYellow), action, reportText(group))
	}
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeIntent(writer io.Writer, jsonOutput bool, accepted intent.Intent, state store.State, now time.Time) error {
	if jsonOutput {
		return writeJSON(writer, map[string]any{
			"intent": accepted,
			"phase":  accepted.PhaseAt(now),
			"state":  state,
		})
	}
	style := newTextStyle(writer)
	var text strings.Builder
	fmt.Fprintf(&text, "%s %s\n", style.paint("Intent:", color.Bold), style.paint(reportText(accepted.Source)+"/"+reportText(accepted.ID), color.Bold))
	fmt.Fprintf(&text, "  Subject: %s\n  Phase: %s\n", reportText(accepted.Subject), accepted.PhaseAt(now))
	fmt.Fprintf(&text, "  Starts: %s\n  Ends: %s\n", accepted.StartsAt.Format(time.RFC3339), accepted.EndsAt.Format(time.RFC3339))
	fmt.Fprintf(&text, "  Updated: %s\n\nSubject reconciliation:\n", accepted.UpdatedAt.Format(time.RFC3339))
	for _, field := range []struct {
		name  string
		value *time.Time
	}{
		{"Last attempt", state.LastAttemptAt}, {"Last success", state.LastSuccessAt},
		{"Next intent boundary", state.NextTransitionAt}, {"Next retry", state.NextRetryAt},
	} {
		fmt.Fprintf(&text, "  %s: %s\n", field.name, reportTime(field.value))
	}
	fmt.Fprintf(&text, "  Retry count: %d\n", state.RetryCount)
	if state.LastError != "" {
		fmt.Fprintf(&text, "  %s %s\n", style.paint("Last error:", color.FgHiRed), reportText(state.LastError))
	}
	_, err := io.WriteString(writer, text.String())
	return err
}

func reportTime(value *time.Time) string {
	if value == nil {
		return "none"
	}
	return value.Format(time.RFC3339)
}

// reportText escapes characters that could change the report layout or
// control the terminal, and keeps printable text as it is.
func reportText(value string) string {
	var text strings.Builder
	for _, r := range value {
		if strconv.IsPrint(r) {
			text.WriteRune(r)
		} else {
			quoted := strconv.QuoteRune(r)
			text.WriteString(quoted[1 : len(quoted)-1])
		}
	}
	return text.String()
}
