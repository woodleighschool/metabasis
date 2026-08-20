package reconcile

import (
	"context"
	"log/slog"
	"time"
)

// RunLoop reconciles startup state, then wakes for accepted intents, transitions, retries, and polling.
func RunLoop(ctx context.Context, service *Service, wake <-chan struct{}, pollInterval time.Duration, logger *slog.Logger) {
	runCycle(ctx, service.ReconcileAll, logger)
	for ctx.Err() == nil {
		delay := pollInterval
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

func runCycle(ctx context.Context, reconcile func(context.Context) ([]Result, error), logger *slog.Logger) {
	started := time.Now()
	results, err := reconcile(ctx)
	for _, result := range results {
		attributes := []any{
			"subject", result.Subject,
			"rule", result.Plan.Rule,
			"add_groups", result.Plan.AddGroups,
			"remove_groups", result.Plan.RemoveGroups,
		}
		if result.Error != "" {
			logger.WarnContext(ctx, "subject reconciliation failed", append(attributes, "error", result.Error)...)
		} else {
			logger.InfoContext(ctx, "subject reconciled", attributes...)
		}
	}
	if err != nil {
		logger.ErrorContext(ctx, "reconciliation cycle failed", "subjects", len(results), "duration", time.Since(started), "error", err)
		return
	}
	logger.InfoContext(ctx, "reconciliation cycle complete", "subjects", len(results), "duration", time.Since(started))
}
