package reconcile

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/graph"
	"github.com/woodleighschool/metabasis/internal/intent"
	"github.com/woodleighschool/metabasis/internal/planner"
	"github.com/woodleighschool/metabasis/internal/store"
)

// Directory is the consumer-owned boundary for Entra identity and membership operations.
type Directory interface {
	Resolve(context.Context, string, map[string][]string, map[string]string) (graph.Snapshot, error)
	AddGroupMember(context.Context, string, string) error
	RemoveGroupMember(context.Context, string, string) error
}

// Result describes one subject reconciliation attempt.
type Result struct {
	Subject string       `json:"subject"`
	Plan    planner.Plan `json:"plan"`
	Error   string       `json:"error,omitempty"`
}

// Service derives and applies complete managed-group state for subjects.
type Service struct {
	config    *config.Config
	store     *store.Store
	directory Directory
	now       func() time.Time
}

// New creates a reconciliation service from validated configuration and concrete state.
func New(cfg *config.Config, intentStore *store.Store, directory Directory) (*Service, error) {
	if cfg == nil || intentStore == nil || directory == nil {
		return nil, fmt.Errorf("config, store, and directory are required")
	}
	return &Service{config: cfg, store: intentStore, directory: directory, now: time.Now}, nil
}

// ReconcileAll reconciles every subject with an accepted intent.
func (s *Service) ReconcileAll(ctx context.Context) ([]Result, error) {
	subjects, err := s.store.ListSubjects(ctx)
	if err != nil {
		return nil, err
	}
	return s.reconcileSubjects(ctx, subjects)
}

// ReconcileDue reconciles subjects whose transition or retry is due.
func (s *Service) ReconcileDue(ctx context.Context) ([]Result, error) {
	subjects, err := s.store.ListSubjectsDue(ctx, s.now().UTC())
	if err != nil {
		return nil, err
	}
	return s.reconcileSubjects(ctx, subjects)
}

// ReconcileSubject derives current desired state and applies only its managed-group diff.
func (s *Service) ReconcileSubject(ctx context.Context, subject string) (result Result, err error) {
	result.Subject = subject
	started := s.now().UTC()
	state, err := s.store.GetState(ctx, subject)
	if err != nil {
		return result, err
	}
	nextTransition := state.NextTransitionAt
	intents, err := s.store.ListIntents(ctx, subject)
	if err != nil {
		return result, s.recordFailure(ctx, &result, state, started, nextTransition, err)
	}
	nextTransition = nextTransitionAt(intents, started)
	snapshot, err := s.directory.Resolve(ctx, subject, s.config.Identity.Groups, s.config.ManagedGroups)
	if err != nil {
		return result, s.recordFailure(ctx, &result, state, started, nextTransition, err)
	}
	result.Plan, err = planner.Build(s.config, snapshot.User, intents, snapshot.ManagedGroups, started)
	if err != nil {
		return result, s.recordFailure(ctx, &result, state, started, nextTransition, err)
	}
	for _, alias := range result.Plan.AddGroups {
		if err := s.directory.AddGroupMember(ctx, s.config.ManagedGroups[alias], snapshot.User.ID); err != nil {
			return result, s.recordFailure(ctx, &result, state, started, nextTransition, fmt.Errorf("add managed group %q: %w", alias, err))
		}
	}
	for _, alias := range result.Plan.RemoveGroups {
		if err := s.directory.RemoveGroupMember(ctx, s.config.ManagedGroups[alias], snapshot.User.ID); err != nil {
			return result, s.recordFailure(ctx, &result, state, started, nextTransition, fmt.Errorf("remove managed group %q: %w", alias, err))
		}
	}
	if err := s.store.RecordSuccess(ctx, subject, started, result.Plan.NextTransition); err != nil {
		return result, err
	}
	return result, nil
}

// PlanSubject derives a read-only plan without changing PostgreSQL or Entra.
func (s *Service) PlanSubject(ctx context.Context, subject string) (planner.Plan, error) {
	intents, err := s.store.ListIntents(ctx, subject)
	if err != nil {
		return planner.Plan{}, err
	}
	snapshot, err := s.directory.Resolve(ctx, subject, s.config.Identity.Groups, s.config.ManagedGroups)
	if err != nil {
		return planner.Plan{}, err
	}
	return planner.Build(s.config, snapshot.User, intents, snapshot.ManagedGroups, s.now().UTC())
}

// PlanEvent overlays one canonical event on persisted state without writing either system.
func (s *Service) PlanEvent(ctx context.Context, event intent.Intent) (planner.Plan, error) {
	if err := event.Validate(); err != nil {
		return planner.Plan{}, err
	}
	intents, err := s.store.ListIntents(ctx, event.Subject)
	if err != nil {
		return planner.Plan{}, err
	}
	replaced := false
	for index := range intents {
		if intents[index].Source == event.Source && intents[index].ID == event.ID {
			intents[index] = event
			replaced = true
			break
		}
	}
	if !replaced {
		intents = append(intents, event)
	}
	snapshot, err := s.directory.Resolve(ctx, event.Subject, s.config.Identity.Groups, s.config.ManagedGroups)
	if err != nil {
		return planner.Plan{}, err
	}
	return planner.Build(s.config, snapshot.User, intents, snapshot.ManagedGroups, s.now().UTC())
}

// NextWake returns the next persisted phase boundary or retry time.
func (s *Service) NextWake(ctx context.Context) (*time.Time, error) {
	return s.store.NextWake(ctx, s.now().UTC())
}

func (s *Service) reconcileSubjects(ctx context.Context, subjects []string) ([]Result, error) {
	results := make([]Result, 0, len(subjects))
	var reconciliationErrors []error
	for _, subject := range subjects {
		if err := ctx.Err(); err != nil {
			return results, errors.Join(append(reconciliationErrors, err)...)
		}
		result, err := s.ReconcileSubject(ctx, subject)
		results = append(results, result)
		if err != nil {
			reconciliationErrors = append(reconciliationErrors, fmt.Errorf("reconcile %s: %w", subject, err))
		}
	}
	return results, errors.Join(reconciliationErrors...)
}

func (s *Service) recordFailure(
	ctx context.Context,
	result *Result,
	state store.State,
	attemptedAt time.Time,
	nextTransition *time.Time,
	cause error,
) error {
	if ctx.Err() != nil {
		return cause
	}
	result.Error = cause.Error()
	nextRetry := attemptedAt.Add(retryDelay(
		s.config.Reconcile.RetryInitial.Duration,
		s.config.Reconcile.RetryMax.Duration,
		state.RetryCount,
	))
	if err := s.store.RecordFailure(ctx, result.Subject, attemptedAt, cause.Error(), nextTransition, nextRetry); err != nil {
		return errors.Join(cause, err)
	}
	return cause
}

func nextTransitionAt(intents []intent.Intent, now time.Time) *time.Time {
	var next *time.Time
	for _, accepted := range intents {
		transition := accepted.NextTransitionAt(now)
		if transition != nil && (next == nil || transition.Before(*next)) {
			value := *transition
			next = &value
		}
	}
	return next
}

func retryDelay(initial, maximum time.Duration, previousFailures int) time.Duration {
	delay := initial
	for range previousFailures {
		if delay >= maximum || delay > maximum/2 {
			return maximum
		}
		delay *= 2
	}
	return min(delay, maximum)
}
