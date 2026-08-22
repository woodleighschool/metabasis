package main

import (
	"context"
	"log/slog"
	"time"

	"github.com/woodleighschool/metabasis/internal/reconcile"
)

type reconciler interface {
	ReconcileAll(context.Context) ([]reconcile.Result, error)
	ReconcileDue(context.Context) ([]reconcile.Result, error)
	NextWake(context.Context) (*time.Time, error)
}

func runLoop(ctx context.Context, interval time.Duration, service reconciler, wake <-chan struct{}, logger *slog.Logger) {
	runCycle(ctx, service.ReconcileAll, logger)
	for ctx.Err() == nil {
		delay := interval
		if next, err := service.NextWake(ctx); err != nil {
			logger.ErrorContext(ctx, "calculate next reconciliation wake", "error", err)
		} else if next != nil {
			until := max(time.Until(*next), 0)
			delay = min(delay, until)
		}
		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return
		case <-wake:
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
		case <-timer.C:
		}
		runCycle(ctx, service.ReconcileDue, logger)
	}
}

func runCycle(
	ctx context.Context,
	reconcileSubjects func(context.Context) ([]reconcile.Result, error),
	logger *slog.Logger,
) {
	started := time.Now()
	results, err := reconcileSubjects(ctx)
	if ctx.Err() != nil {
		return
	}
	for _, result := range results {
		attributes := []any{
			"subject", result.Subject,
			"rule", result.Plan.Rule,
			"add_groups", result.Plan.AddGroups,
			"remove_groups", result.Plan.RemoveGroups,
		}
		switch {
		case result.Error != "":
			logger.WarnContext(ctx, "subject reconciliation failed", append(attributes, "error", result.Error)...)
		case len(result.Plan.AddGroups) != 0 || len(result.Plan.RemoveGroups) != 0:
			logger.InfoContext(ctx, "subject reconciled", attributes...)
		default:
			logger.DebugContext(ctx, "subject reconciled", attributes...)
		}
	}
	if err != nil {
		logger.ErrorContext(ctx, "reconciliation cycle failed", "subjects", len(results), "duration", time.Since(started), "error", err)
		return
	}
	logger.DebugContext(ctx, "reconciliation cycle complete", "subjects", len(results), "duration", time.Since(started))
}
