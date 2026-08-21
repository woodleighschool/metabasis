package planner

import (
	"fmt"
	"slices"
	"sort"
	"time"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/domain"
	"github.com/woodleighschool/metabasis/internal/intent"
)

// IntentPhase records how an accepted intent contributes to a plan.
type IntentPhase struct {
	Source string       `json:"source"`
	ID     string       `json:"id"`
	Phase  intent.Phase `json:"phase"`
}

// Plan is the complete desired managed-group state for one subject.
type Plan struct {
	Subject        string        `json:"subject"`
	User           domain.User   `json:"user"`
	Rule           string        `json:"rule,omitempty"`
	Intents        []IntentPhase `json:"intents"`
	DesiredGroups  []string      `json:"desired_groups"`
	CurrentGroups  []string      `json:"current_groups"`
	AddGroups      []string      `json:"add_groups"`
	RemoveGroups   []string      `json:"remove_groups"`
	NextTransition *time.Time    `json:"next_transition,omitempty"`
}

// Build derives desired state from all known intents for a subject.
func Build(cfg *config.Config, user domain.User, intents []intent.Intent, currentGroups []string, now time.Time) (Plan, error) {
	if cfg == nil {
		return Plan{}, fmt.Errorf("config is required")
	}
	plan := Plan{
		User:    user,
		Intents: make([]IntentPhase, 0, len(intents)),
	}
	for _, group := range uniqueSorted(currentGroups) {
		if _, managed := cfg.ManagedGroups[group]; managed {
			plan.CurrentGroups = append(plan.CurrentGroups, group)
		}
	}
	if len(intents) != 0 {
		plan.Subject = intents[0].Subject
	}

	ruleIndex := -1
	for index, program := range cfg.Programs {
		matches, err := program.Eval(user)
		if err != nil {
			return Plan{}, fmt.Errorf("evaluate rule %q: %w", cfg.Rules[index].Name, err)
		}
		if matches {
			ruleIndex = index
			plan.Rule = cfg.Rules[index].Name
			break
		}
	}

	desired := make(map[string]struct{})
	for _, accepted := range intents {
		phase := accepted.PhaseAt(now)
		plan.Intents = append(plan.Intents, IntentPhase{Source: accepted.Source, ID: accepted.ID, Phase: phase})
		if transition := accepted.NextTransitionAt(now); transition != nil &&
			(plan.NextTransition == nil || transition.Before(*plan.NextTransition)) {
			value := *transition
			plan.NextTransition = &value
		}
		if ruleIndex < 0 {
			continue
		}
		var groups []string
		switch phase {
		case intent.PhasePending:
			groups = cfg.Rules[ruleIndex].Phases.Pending.Groups
		case intent.PhaseActive:
			groups = cfg.Rules[ruleIndex].Phases.Active.Groups
		case intent.PhaseEnded, intent.PhaseCancelled:
		}
		for _, group := range groups {
			desired[group] = struct{}{}
		}
	}

	plan.DesiredGroups = sortedSet(desired)
	plan.AddGroups = difference(plan.DesiredGroups, plan.CurrentGroups)
	plan.RemoveGroups = difference(plan.CurrentGroups, plan.DesiredGroups)
	sort.Slice(plan.Intents, func(i, j int) bool {
		if plan.Intents[i].Source != plan.Intents[j].Source {
			return plan.Intents[i].Source < plan.Intents[j].Source
		}
		return plan.Intents[i].ID < plan.Intents[j].ID
	})
	return plan, nil
}

func difference(left, right []string) []string {
	result := make([]string, 0, len(left))
	for _, value := range left {
		if !slices.Contains(right, value) {
			result = append(result, value)
		}
	}
	return result
}

func uniqueSorted(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return slices.Compact(result)
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}
