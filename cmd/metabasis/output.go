package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/woodleighschool/metabasis/internal/intent"
	"github.com/woodleighschool/metabasis/internal/planner"
	"github.com/woodleighschool/metabasis/internal/reconcile"
	"github.com/woodleighschool/metabasis/internal/store"
)

type planReport struct {
	Plan  *planner.Plan `json:"plan,omitempty"`
	Error string        `json:"error,omitempty"`
}

type applyReport struct {
	Subjects []reconcile.Result `json:"subjects,omitzero"`
	Error    string             `json:"error,omitempty"`
}

func writePlan(writer io.Writer, output string, plan planner.Plan, planErr error) error {
	report := planReport{Plan: &plan}
	if planErr != nil {
		report.Plan = nil
		report.Error = planErr.Error()
	}
	if output == "json" {
		return writeJSON(writer, report)
	}
	if planErr != nil {
		_, err := fmt.Fprintln(writer, "No membership plan available.")
		return err
	}
	phases := make([]string, 0, len(plan.Intents))
	for _, accepted := range plan.Intents {
		phases = append(phases, accepted.Source+"/"+accepted.ID+":"+string(accepted.Phase))
	}
	next := ""
	if plan.NextTransition != nil {
		next = plan.NextTransition.Format(time.RFC3339)
	}
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "SUBJECT\tRESOLVED\tRULE\tSTATE\tPHASES\tPRESENT\tABSENT\tCURRENT\tADD\tREMOVE\tNEXT"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(
		table,
		"%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		plan.Subject,
		plan.User.UserPrincipalName,
		plan.Rule,
		plan.State,
		strings.Join(phases, ","),
		strings.Join(plan.PresentGroups, ","),
		strings.Join(plan.AbsentGroups, ","),
		strings.Join(plan.CurrentGroups, ","),
		strings.Join(plan.AddGroups, ","),
		strings.Join(plan.RemoveGroups, ","),
		next,
	); err != nil {
		return err
	}
	return table.Flush()
}

func writeIntents(writer io.Writer, intents []intent.Intent, now time.Time) error {
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "SOURCE\tID\tSUBJECT\tPHASE\tSTARTS\tENDS\tUPDATED"); err != nil {
		return err
	}
	for _, accepted := range intents {
		if _, err := fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			accepted.Source,
			accepted.ID,
			accepted.Subject,
			accepted.PhaseAt(now),
			accepted.StartsAt.Format(time.RFC3339),
			accepted.EndsAt.Format(time.RFC3339),
			accepted.UpdatedAt.Format(time.RFC3339),
		); err != nil {
			return err
		}
	}
	return table.Flush()
}

func writeApplyReport(writer io.Writer, output string, results []reconcile.Result, runErr error) error {
	report := applyReport{Subjects: results}
	if runErr != nil {
		report.Error = runErr.Error()
	}
	if output == "json" {
		return writeJSON(writer, report)
	}
	if results == nil && runErr != nil {
		_, err := fmt.Fprintln(writer, "No subject results available.")
		return err
	}
	applied, unchanged, failed := 0, 0, 0
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "SUBJECT\tRESULT\tRULE\tADDED\tREMOVED\tERROR"); err != nil {
		return err
	}
	for _, result := range results {
		rule := ""
		if result.Plan != nil {
			rule = result.Plan.Rule
		}
		status := "unchanged"
		switch {
		case result.Error != "":
			status = "failed"
			failed++
		case len(result.AddedGroups) != 0 || len(result.RemovedGroups) != 0:
			status = "applied"
			applied++
		default:
			unchanged++
		}
		if _, err := fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%s\t%s\n",
			result.Subject,
			status,
			rule,
			strings.Join(result.AddedGroups, ","),
			strings.Join(result.RemovedGroups, ","),
			result.Error,
		); err != nil {
			return err
		}
	}
	if err := table.Flush(); err != nil {
		return err
	}
	_, err := fmt.Fprintf(writer, "\nSubjects: %d total, %d applied, %d unchanged, %d failed\n", len(results), applied, unchanged, failed)
	return err
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func writeIntent(writer io.Writer, accepted intent.Intent, state store.State, now time.Time) error {
	if err := writeIntents(writer, []intent.Intent{accepted}, now); err != nil {
		return err
	}
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	for _, field := range []struct {
		name  string
		value *time.Time
	}{
		{"Last attempt", state.LastAttemptAt}, {"Last success", state.LastSuccessAt},
		{"Next transition", state.NextTransitionAt}, {"Next retry", state.NextRetryAt},
	} {
		value := "—"
		if field.value != nil {
			value = field.value.Format(time.RFC3339)
		}
		if _, err := fmt.Fprintf(table, "%s:\t%s\n", field.name, value); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(table, "Retry count:\t%d\nLast error:\t%s\n", state.RetryCount, state.LastError); err != nil {
		return err
	}
	return table.Flush()
}
