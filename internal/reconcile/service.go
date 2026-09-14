package reconcile

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/domain"
	"github.com/woodleighschool/metabasis/internal/intent"
	"github.com/woodleighschool/metabasis/internal/metrics"
	"github.com/woodleighschool/metabasis/internal/planner"
	"github.com/woodleighschool/metabasis/internal/store"
)

// Directory is the consumer-owned boundary for Entra identity and membership operations.
type Directory interface {
	Resolve(context.Context, string, map[string][]string) (domain.User, error)
	AddGroupMember(context.Context, string, string) error
	RemoveGroupMember(context.Context, string, string) error
}

// Result describes one subject reconciliation attempt.
type Result struct {
	Subject       string        `json:"subject"`
	Plan          *planner.Plan `json:"plan,omitempty"`
	Error         string        `json:"error,omitempty"`
	AddedGroups   []string      `json:"added_groups"`
	RemovedGroups []string      `json:"removed_groups"`
}

// Service derives and applies explicit group membership assertions for subjects.
type Service struct {
	logger    *slog.Logger
	config    *config.Config
	store     *store.Store
	directory Directory
	metrics   *metrics.Recorder
	now       func() time.Time
}

// New creates a reconciliation service from validated configuration and concrete state.
func New(cfg *config.Config, intentStore *store.Store, directory Directory, recorder *metrics.Recorder, logger *slog.Logger) (*Service, error) {
	if cfg == nil || intentStore == nil || directory == nil {
		return nil, fmt.Errorf("config, store, and directory are required")
	}
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}
	return &Service{logger: logger, config: cfg, store: intentStore, directory: directory, metrics: recorder, now: time.Now}, nil
}

// ReconcileAll reconciles every subject with an accepted intent.
func (s *Service) ReconcileAll(ctx context.Context) ([]Result, error) {
	done := s.stage(ctx, "Loading accepted subjects")
	subjects, err := s.store.ListSubjects(ctx)
	done(err)
	if err != nil {
		return nil, err
	}
	return s.reconcileSubjects(ctx, subjects)
}

// ReconcileDue reconciles subjects whose transition or retry is due.
func (s *Service) ReconcileDue(ctx context.Context) ([]Result, error) {
	done := s.stage(ctx, "Loading due subjects")
	subjects, err := s.store.ListSubjectsDue(ctx, s.now().UTC())
	done(err)
	if err != nil {
		return nil, err
	}
	return s.reconcileSubjects(ctx, subjects)
}

// ReconcileSubject derives and applies the current membership assertions.
func (s *Service) ReconcileSubject(ctx context.Context, subject string) (result Result, err error) {
	result.Subject = subject
	result.AddedGroups, result.RemovedGroups = []string{}, []string{}
	defer func() {
		if err != nil {
			result.Error = err.Error()
		}
		s.logger.InfoContext(ctx, "Subject reconciled", "subject", subject, "subject_result", true, "error", err,
			"changes", len(result.AddedGroups)+len(result.RemovedGroups))
	}()
	observedAt := time.Now()
	defer func() { s.metrics.RecordReconciliation(err, time.Since(observedAt)) }()
	done := s.stage(ctx, "Loading subject state", "subject", subject)
	defer func() { done(err) }()
	subjectSession, err := s.store.LockSubject(ctx, subject)
	if err != nil {
		return result, err
	}
	defer func() { err = errors.Join(err, subjectSession.Close()) }()
	started := s.now().UTC()
	state, err := subjectSession.GetState(ctx)
	if err != nil {
		return result, err
	}
	nextTransition := state.NextTransitionAt
	intents, err := subjectSession.ListIntents(ctx)
	if err != nil {
		return result, s.recordFailure(ctx, subjectSession, state, started, nextTransition, err)
	}
	nextTransition = nextTransitionAt(intents, started)
	done(nil)
	done = s.stage(ctx, "Resolving identity", "subject", subject)
	user, err := s.directory.Resolve(ctx, subject, s.config.Identity.Groups)
	if err != nil {
		return result, s.recordFailure(ctx, subjectSession, state, started, nextTransition, err)
	}
	done(nil)
	done = s.stage(ctx, "Planning memberships", "subject", subject)
	plan, err := planner.Build(s.config, user, intents, started)
	if err != nil {
		return result, s.recordFailure(ctx, subjectSession, state, started, nextTransition, err)
	}
	result.Plan = &plan
	total := len(plan.AddGroups) + len(plan.RemoveGroups)
	done(nil)
	done = s.stage(ctx, "Applying memberships", "subject", subject, "total", total, "unit", "changes")
	for _, alias := range result.Plan.AddGroups {
		if err := s.directory.AddGroupMember(ctx, s.config.Identity.Groups[alias][0], user.ID); err != nil {
			return result, s.recordFailure(ctx, subjectSession, state, started, nextTransition, fmt.Errorf("add group %q: %w", alias, err))
		}
		result.AddedGroups = append(result.AddedGroups, alias)
		completed := len(result.AddedGroups) + len(result.RemovedGroups)
		s.logger.InfoContext(ctx, "Applying memberships", "subject", subject, "progress", true, "current", completed, "total", total, "unit", "changes", "progress_final", completed == total)
	}
	for _, alias := range result.Plan.RemoveGroups {
		if err := s.directory.RemoveGroupMember(ctx, s.config.Identity.Groups[alias][0], user.ID); err != nil {
			return result, s.recordFailure(ctx, subjectSession, state, started, nextTransition, fmt.Errorf("remove group %q: %w", alias, err))
		}
		result.RemovedGroups = append(result.RemovedGroups, alias)
		completed := len(result.AddedGroups) + len(result.RemovedGroups)
		s.logger.InfoContext(ctx, "Applying memberships", "subject", subject, "progress", true, "current", completed, "total", total, "unit", "changes", "progress_final", completed == total)
	}
	done(nil)
	done = s.stage(ctx, "Saving reconciliation state", "subject", subject)
	if err := subjectSession.RecordSuccess(ctx, started, result.Plan.NextTransition); err != nil {
		return result, err
	}
	return result, nil
}

// PlanEvent overlays one canonical event on persisted state without writing either system.
func (s *Service) PlanEvent(ctx context.Context, event intent.Intent) (result planner.Plan, runErr error) {
	if err := event.Validate(); err != nil {
		return planner.Plan{}, err
	}
	done := s.stage(ctx, "Loading accepted intents")
	defer func() { done(runErr) }()
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
	done(nil)
	done = s.stage(ctx, "Resolving identity")
	user, err := s.directory.Resolve(ctx, event.Subject, s.config.Identity.Groups)
	if err != nil {
		return planner.Plan{}, err
	}
	done(nil)
	done = s.stage(ctx, "Planning memberships")
	return planner.Build(s.config, user, intents, s.now().UTC())
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
	subjectSession *store.SubjectSession,
	state store.State,
	attemptedAt time.Time,
	nextTransition *time.Time,
	cause error,
) error {
	if ctx.Err() != nil {
		return cause
	}
	nextRetry := attemptedAt.Add(retryDelay(
		s.config.Reconcile.RetryInitial.Duration,
		s.config.Reconcile.RetryMax.Duration,
		state.RetryCount,
	))
	if err := subjectSession.RecordFailure(ctx, attemptedAt, cause.Error(), nextTransition, nextRetry); err != nil {
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

func (s *Service) stage(ctx context.Context, message string, attrs ...any) func(error) {
	started := time.Now()
	s.logger.InfoContext(ctx, message, append([]any{"stage", true}, attrs...)...)
	var once sync.Once
	return func(err error) {
		once.Do(func() {
			result := append([]any{"stage_result", true, "elapsed", time.Since(started).Round(time.Millisecond)}, attrs...)
			if err != nil {
				result = append(result, "error", err)
			}
			s.logger.InfoContext(ctx, message, result...)
		})
	}
}
