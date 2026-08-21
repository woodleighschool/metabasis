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

// State is the aggregate temporal state of a subject's accepted intents.
type State string

const (
	StatePending State = "pending"
	StateActive  State = "active"
	StateSettled State = "settled"
)

// Plan contains the explicit membership assertions for one subject.
type Plan struct {
	Subject        string        `json:"subject"`
	User           domain.User   `json:"user"`
	Rule           string        `json:"rule,omitempty"`
	State          State         `json:"state,omitempty"`
	Intents        []IntentPhase `json:"intents"`
	PresentGroups  []string      `json:"present_groups"`
	AbsentGroups   []string      `json:"absent_groups"`
	CurrentGroups  []string      `json:"current_groups"`
	AddGroups      []string      `json:"add_groups"`
	RemoveGroups   []string      `json:"remove_groups"`
	NextTransition *time.Time    `json:"next_transition,omitempty"`
}

// Build derives membership assertions from all known intents for a subject.
func Build(cfg *config.Config, user domain.User, intents []intent.Intent, now time.Time) (Plan, error) {
	if cfg == nil {
		return Plan{}, fmt.Errorf("config is required")
	}
	plan := Plan{
		User:          user,
		Intents:       make([]IntentPhase, 0, len(intents)),
		CurrentGroups: uniqueSorted(user.Groups),
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

	hasPending := false
	hasActive := false
	for _, accepted := range intents {
		phase := accepted.PhaseAt(now)
		plan.Intents = append(plan.Intents, IntentPhase{Source: accepted.Source, ID: accepted.ID, Phase: phase})
		if transition := accepted.NextTransitionAt(now); transition != nil &&
			(plan.NextTransition == nil || transition.Before(*plan.NextTransition)) {
			value := *transition
			plan.NextTransition = &value
		}
		switch phase {
		case intent.PhasePending:
			hasPending = true
		case intent.PhaseActive:
			hasActive = true
		case intent.PhaseEnded, intent.PhaseCancelled:
		}
	}

	if len(intents) != 0 {
		plan.State = StateSettled
	}
	if hasPending {
		plan.State = StatePending
	}
	if hasActive {
		plan.State = StateActive
	}
	if ruleIndex >= 0 && plan.State != "" {
		var assertions config.GroupAssertions
		switch plan.State {
		case StatePending:
			assertions = cfg.Rules[ruleIndex].States.Pending
		case StateActive:
			assertions = cfg.Rules[ruleIndex].States.Active
		case StateSettled:
			assertions = cfg.Rules[ruleIndex].States.Settled
		}
		plan.PresentGroups = uniqueSorted(assertions.Present)
		plan.AbsentGroups = uniqueSorted(assertions.Absent)
		plan.AddGroups = difference(plan.PresentGroups, plan.CurrentGroups)
		plan.RemoveGroups = intersection(plan.AbsentGroups, plan.CurrentGroups)
	}
	sort.Slice(plan.Intents, func(i, j int) bool {
		if plan.Intents[i].Source != plan.Intents[j].Source {
			return plan.Intents[i].Source < plan.Intents[j].Source
		}
		return plan.Intents[i].ID < plan.Intents[j].ID
	})
	return plan, nil
}

func intersection(left, right []string) []string {
	result := make([]string, 0, len(left))
	for _, value := range left {
		if slices.Contains(right, value) {
			result = append(result, value)
		}
	}
	return result
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
