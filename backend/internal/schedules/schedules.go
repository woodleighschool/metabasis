package schedules

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/woodleighschool/adoverseas/internal/config"
	"github.com/woodleighschool/adoverseas/internal/graph"
	"github.com/woodleighschool/adoverseas/internal/store"
	"github.com/woodleighschool/adoverseas/internal/store/sqlc"
)

func NewTaskJob(store *store.Store, graphClient *graph.Client, cfg config.Config, logger *slog.Logger) Job {
	return func(ctx context.Context) error {
		if store == nil {
			return fmt.Errorf("db missing")
		}
		currentUrgentTasks, err := store.ListUrgentSchedules(ctx)
		if err != nil {
			logger.Error("unable to retrieve urgent tasks", "err", err)
		}
		if len(currentUrgentTasks) == 0 {
			logger.Debug("No urgent tasks to action")
		}
		for _, task := range currentUrgentTasks {
			user, err := store.GetUser(ctx, task.Userid)
			if err != nil {
				logger.Error("unable to get user from urgent task", "err", err)
			}
			if err := enableMFA(ctx, user, graphClient); err != nil {
				logger.Error("unable to enable MFA for user", "user", user.Upn, "err", err)
			}
			if err := store.DeleteUrgentSchedule(ctx, task.ID); err != nil {
				logger.Error("unable to delete urgent job from store", "task", task.ID, "err", err)
			}
			if err == nil {
				logger.Debug("Successfully completed urgent task", "task", task.ID, "user", user.Upn)
			}
		}

		currentTasks, err := store.ListScheduleSummaries(ctx)
		if err != nil {
			return fmt.Errorf("list tasks: %w", err)
		}
		if len(currentTasks) == 0 {
			logger.Debug("No tasks to action")
			return nil
		}
		currentTime := time.Now()
		for _, task := range currentTasks {
			if task.Overseas && task.ReturningDate.Time.Before(currentTime) {
				user, err := store.GetUser(ctx, task.Userid)
				if err != nil {
					logger.Error("unable to find user from task", "err", err, "task", task.ID.String())
					return fmt.Errorf("%w", err)
				}
				if err := userReturning(ctx, user, graphClient); err != nil {
					logger.Error("unable to execute returning user", "err", err, "task", task.ID.String())
					return fmt.Errorf("%w", err)
				}
				if err := store.DeleteSchedule(ctx, task.ID); err != nil {
					logger.Error("failed to remove leftover task", "err", err, "task", task.ID.String())
					return fmt.Errorf("%w", err)
				}
				logger.Debug("successfully completed returning task", "task", task.ID.String(), "user", user.Upn)
				continue
			} else if !task.Overseas && task.LeavingDate.Time.Before(currentTime) {
				user, err := store.GetUser(ctx, task.Userid)
				if err != nil {
					logger.Error("unable to find user from task", "err", err, "task", task.ID.String())
					return fmt.Errorf("%w", err)
				}
				if err := userLeaving(ctx, user, graphClient); err != nil {
					logger.Error("unable to execute leaving user", "err", err, "task", task.ID.String())
					return fmt.Errorf("%w", err)
				}
				if err := store.FlipSchedule(ctx, task.ID); err != nil {
					logger.Error("failed to update overseas flag on task", "err", err, "task", task.ID.String())
					return fmt.Errorf("%w", err)
				}
				logger.Debug("successfully completed leaving task", "task", task.ID.String(), "user", user.Upn)
				continue
			}
		}
		logger.Debug("All tasks actioned")
		return nil
	}
}

func userLeaving(ctx context.Context, user sqlc.User, graphClient *graph.Client) error {
	userID, err := graphClient.FetchUserObjectID(ctx, user.Upn)
	if err != nil {
		return fmt.Errorf("fetch user object id: %w", err)
	}
	for _, group := range graphClient.GroupConfig.AwayGroups {
		err := graphClient.AddGroupMember(ctx, group, userID)
		if err != nil {
			return fmt.Errorf("unable to add user to group: %w", err)
		}
	}

	if graphClient.GroupConfig.ForceMFAGroup != "" && !user.Staff.Bool {
		err := graphClient.AddGroupMember(ctx, graphClient.GroupConfig.ForceMFAGroup, userID)
		if err != nil {
			return fmt.Errorf("unable to add user to mfa group: %w", err)
		}
	}

	for _, group := range graphClient.GroupConfig.HomeGroups {
		err := graphClient.RemoveGroupMember(ctx, group, userID)
		if err != nil {
			return fmt.Errorf("unable to remove user from group: %w", err)
		}
	}
	return nil
}

func userReturning(ctx context.Context, user sqlc.User, graphClient *graph.Client) error {
	userID, err := graphClient.FetchUserObjectID(ctx, user.Upn)
	if err != nil {
		return fmt.Errorf("fetch user object id: %w", err)
	}
	for _, group := range graphClient.GroupConfig.HomeGroups {
		err := graphClient.AddGroupMember(ctx, group, userID)
		if err != nil {
			return fmt.Errorf("unable to add user to group: %w", err)
		}
	}

	for _, group := range graphClient.GroupConfig.AwayGroups {
		err := graphClient.RemoveGroupMember(ctx, group, userID)
		if err != nil {
			return fmt.Errorf("unable to remove user from group: %w", err)
		}
	}

	if graphClient.GroupConfig.ForceMFAGroup != "" && !user.Staff.Bool {
		err := graphClient.RemoveGroupMember(ctx, graphClient.GroupConfig.ForceMFAGroup, userID)
		if err != nil {
			return fmt.Errorf("unable to remove user from force mfa group: %w", err)
		}
		err = graphClient.RemoveGroupMember(ctx, graphClient.GroupConfig.EnableMFAGroup, userID)
		if err != nil {
			return fmt.Errorf("unable to remove user from enable mfa group: %w", err)
		}
	}
	return nil
}

func enableMFA(ctx context.Context, user sqlc.User, graphClient *graph.Client) error {
	userID, err := graphClient.FetchUserObjectID(ctx, user.Upn)
	if err != nil {
		return fmt.Errorf("fetch user object id: %w", err)
	}
	err = graphClient.AddGroupMember(ctx, graphClient.GroupConfig.EnableMFAGroup, userID)
	if err != nil {
		return fmt.Errorf("unable to add user to enable mfa group: %w", err)
	}
	return nil
}
