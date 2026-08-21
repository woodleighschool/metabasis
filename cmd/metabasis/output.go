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
)

func writePlan(writer io.Writer, output string, plan planner.Plan) error {
	if output == "json" {
		return writeJSON(writer, plan)
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

func writeReconcileResults(writer io.Writer, results []reconcile.Result) error {
	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(table, "SUBJECT\tRULE\tADDED\tREMOVED\tERROR"); err != nil {
		return err
	}
	for _, result := range results {
		if _, err := fmt.Fprintf(
			table,
			"%s\t%s\t%s\t%s\t%s\n",
			result.Subject,
			result.Plan.Rule,
			strings.Join(result.Plan.AddGroups, ","),
			strings.Join(result.Plan.RemoveGroups, ","),
			result.Error,
		); err != nil {
			return err
		}
	}
	return table.Flush()
}

func writeJSON(writer io.Writer, value any) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}
