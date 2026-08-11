package schedules

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/woodleighschool/adoverseas/internal/graph"
	"github.com/woodleighschool/adoverseas/internal/store"
	"github.com/woodleighschool/adoverseas/internal/store/sqlc"
)

func NewTaskJob(store *store.Store, graphClient *graph.Client, logger *slog.Logger) Job {
	return func(ctx context.Context) error {
		if store == nil {
			return fmt.Errorf("db missing")
		}
		processUrgentTasks(ctx, store, graphClient, logger)

		currentTasks, err := store.ListScheduleSummaries(ctx)
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
		}
		if len(currentTasks) == 0 {
			logger.DebugContext(ctx, "No tasks to action")
			return nil
		}
		currentTime := time.Now()
		for _, task := range currentTasks {
			if err := processScheduledTask(ctx, store, graphClient, logger, task, currentTime); err != nil {
				return err
			}
		}
		logger.DebugContext(ctx, "All tasks actioned")
		return nil
	}
}

func processUrgentTasks(ctx context.Context, store *store.Store, graphClient *graph.Client, logger *slog.Logger) {
	tasks, err := store.ListUrgentSchedules(ctx)
	if err != nil {
		logger.ErrorContext(ctx, "unable to retrieve urgent tasks", "err", err)
	}
	if len(tasks) == 0 {
		logger.DebugContext(ctx, "No urgent tasks to action")
	}
	for _, task := range tasks {
		user, err := store.GetUser(ctx, task.Userid)
		if err != nil {
			logger.ErrorContext(ctx, "unable to get user from urgent task", "err", err)
		}
		if err := enableMFA(ctx, user, graphClient); err != nil {
			logger.ErrorContext(ctx, "unable to enable MFA for user", "user", user.Upn, "err", err)
		}
		if err := store.DeleteUrgentSchedule(ctx, task.ID); err != nil {
			logger.ErrorContext(ctx, "unable to delete urgent job from store", "task", task.ID, "err", err)
		}
		if err == nil {
			logger.InfoContext(ctx, "Successfully completed urgent task", "task", task.ID, "user", user.Upn)
		}
	}
}

func processScheduledTask(
	ctx context.Context,
	store *store.Store,
	graphClient *graph.Client,
	logger *slog.Logger,
	task sqlc.ListScheduleSummariesRow,
	currentTime time.Time,
) error {
	if task.Overseas && task.ReturningDate.Time.Before(currentTime) {
		return processReturningTask(ctx, store, graphClient, logger, task)
	}
	if !task.Overseas && task.LeavingDate.Time.Before(currentTime) {
		return processLeavingTask(ctx, store, graphClient, logger, task)
	}
	return nil
}

func processReturningTask( //nolint:dupl // Returning and leaving tasks keep their distinct graph and store operations explicit.
	ctx context.Context,
	store *store.Store,
	graphClient *graph.Client,
	logger *slog.Logger,
	task sqlc.ListScheduleSummariesRow,
) error {
	user, err := store.GetUser(ctx, task.Userid)
	if err != nil {
		logger.ErrorContext(ctx, "unable to find user from task", "err", err, "task", task.ID.String())
		return fmt.Errorf("%w", err)
	}
	if err := userReturning(ctx, user, graphClient); err != nil {
		logger.ErrorContext(ctx, "unable to execute returning user", "err", err, "task", task.ID.String())
		return fmt.Errorf("%w", err)
	}
	if err := store.DeleteSchedule(ctx, task.ID); err != nil {
		logger.ErrorContext(ctx, "failed to remove leftover task", "err", err, "task", task.ID.String())
		return fmt.Errorf("%w", err)
	}
	logger.InfoContext(ctx, "successfully completed returning task", "task", task.ID.String(), "user", user.Upn)
	return nil
}

func processLeavingTask( //nolint:dupl // Returning and leaving tasks keep their distinct graph and store operations explicit.
	ctx context.Context,
	store *store.Store,
	graphClient *graph.Client,
	logger *slog.Logger,
	task sqlc.ListScheduleSummariesRow,
) error {
	user, err := store.GetUser(ctx, task.Userid)
	if err != nil {
		logger.ErrorContext(ctx, "unable to find user from task", "err", err, "task", task.ID.String())
		return fmt.Errorf("%w", err)
	}
	if err := userLeaving(ctx, user, graphClient); err != nil {
		logger.ErrorContext(ctx, "unable to execute leaving user", "err", err, "task", task.ID.String())
		return fmt.Errorf("%w", err)
	}
	if err := store.FlipSchedule(ctx, task.ID); err != nil {
		logger.ErrorContext(ctx, "failed to update overseas flag on task", "err", err, "task", task.ID.String())
		return fmt.Errorf("%w", err)
	}
	logger.InfoContext(ctx, "successfully completed leaving task", "task", task.ID.String(), "user", user.Upn)
	return nil
}

func userLeaving(ctx context.Context, user sqlc.User, graphClient *graph.Client) error {
	graphUser, err := graphClient.FetchUser(ctx, user.Upn)
	if err != nil {
		return fmt.Errorf("fetch user object id: %w", err)
	}
	for _, group := range graphClient.GroupConfig.AwayGroups {
		err := graphClient.AddGroupMember(ctx, group, graphUser)
		if err != nil {
			return fmt.Errorf("unable to add user to group: %w", err)
		}
	}

	if graphClient.GroupConfig.ForceMFAGroup != "" && !user.Staff.Bool {
		err := graphClient.AddGroupMember(ctx, graphClient.GroupConfig.ForceMFAGroup, graphUser)
		if err != nil {
			return fmt.Errorf("unable to add user to mfa group: %w", err)
		}
	}

	for _, group := range graphClient.GroupConfig.HomeGroups {
		err := graphClient.RemoveGroupMember(ctx, group, graphUser)
		if err != nil {
			return fmt.Errorf("unable to remove user from group: %w", err)
		}
	}
	return nil
}

func userReturning(ctx context.Context, user sqlc.User, graphClient *graph.Client) error {
	graphUser, err := graphClient.FetchUser(ctx, user.Upn)
	if err != nil {
		return fmt.Errorf("fetch user object id: %w", err)
	}
	for _, group := range graphClient.GroupConfig.HomeGroups {
		err := graphClient.AddGroupMember(ctx, group, graphUser)
		if err != nil {
			return fmt.Errorf("unable to add user to group: %w", err)
		}
	}

	for _, group := range graphClient.GroupConfig.AwayGroups {
		err := graphClient.RemoveGroupMember(ctx, group, graphUser)
		if err != nil {
			return fmt.Errorf("unable to remove user from group: %w", err)
		}
	}

	if graphClient.GroupConfig.ForceMFAGroup != "" && !user.Staff.Bool {
		err := graphClient.RemoveGroupMember(ctx, graphClient.GroupConfig.ForceMFAGroup, graphUser)
		if err != nil {
			return fmt.Errorf("unable to remove user from force mfa group: %w", err)
		}
		err = graphClient.RemoveGroupMember(ctx, graphClient.GroupConfig.EnableMFAGroup, graphUser)
		if err != nil {
			return fmt.Errorf("unable to remove user from enable mfa group: %w", err)
		}
	}
	return nil
}

func enableMFA(ctx context.Context, user sqlc.User, graphClient *graph.Client) error {
	graphUser, err := graphClient.FetchUser(ctx, user.Upn)
	if err != nil {
		return fmt.Errorf("fetch user object id: %w", err)
	}
	err = graphClient.AddGroupMember(ctx, graphClient.GroupConfig.EnableMFAGroup, graphUser)
	if err != nil {
		return fmt.Errorf("unable to add user to enable mfa group: %w", err)
	}
	return nil
}
