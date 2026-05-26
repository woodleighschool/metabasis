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
					return fmt.Errorf("unable to find user from task: %w", err)
				}
				if err := userReturning(ctx, user, graphClient); err != nil {
					return fmt.Errorf("unable to execute returning user: %w", err)
				}
				if err := store.DeleteSchedule(ctx, task.ID); err != nil {
					return fmt.Errorf("failed to remove leftover task: %w", err)
				}
				continue
			} else if !task.Overseas && task.LeavingDate.Time.Before(currentTime) {
				user, err := store.GetUser(ctx, task.Userid)
				if err != nil {
					return fmt.Errorf("unable to find user from task: %w", err)
				}
				if err := userLeaving(ctx, user, graphClient); err != nil {
					return fmt.Errorf("unable to execute leaving user: %w", err)
				}
				if err := store.FlipSchedule(ctx, task.ID); err != nil {
					return fmt.Errorf("failed to update overseas flag on task: %w", err)
				}
				continue
			}
		}
		logger.Debug("All tasks actioned successfully")
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
			return fmt.Errorf("unable to remove user from mfa group: %w", err)
		}
	}
	return nil
}
