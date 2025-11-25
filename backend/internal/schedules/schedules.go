package schedules

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	activedirectory "github.com/woodleighschool/adoverseas/internal/activeDirectory"
	"github.com/woodleighschool/adoverseas/internal/config"
	"github.com/woodleighschool/adoverseas/internal/store"
	"github.com/woodleighschool/adoverseas/internal/store/sqlc"
)

func NewTaskJob(store *store.Store, adClient activedirectory.Client, cfg config.Config, logger *slog.Logger) Job {
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
				if err := UserReturning(user, adClient); err != nil {
					return fmt.Errorf("unable to execute returning user: %w", err)
				}
				if err := store.DeleteSchedule(ctx, task.ID); err != nil {
					return fmt.Errorf("failed to remove leftover task: %w", err)
				}
			} else if !task.Overseas && task.LeavingDate.Time.Before(currentTime) {
				user, err := store.GetUser(ctx, task.Userid)
				if err != nil {
					return fmt.Errorf("unable to find user from task: %w", err)
				}
				if err := UserLeaving(user, adClient); err != nil {
					return fmt.Errorf("unable to execute leaving user: %w", err)
				}
				if err := store.FlipSchedule(ctx, task.ID); err != nil {
					return fmt.Errorf("failed to update overseas flag on task: %w", err)
				}
			}
		}
		logger.Debug("All tasks actioned successfully")
		return nil
	}
}

func UserLeaving(user sqlc.User, adClient activedirectory.Client) error {
	for _, group := range adClient.AwayGroups {
		_, err := adClient.Client.AddGroupMembers(group, user.Upn)
		if err != nil {
			return fmt.Errorf("unable to add user to group: %w", err)
		}
	}

	if adClient.MFAGroup != "" && !user.Staff.Bool {
		_, err := adClient.Client.AddGroupMembers(adClient.MFAGroup, user.Upn)
		if err != nil {
			return fmt.Errorf("unable to add user to mfa group: %w", err)
		}
	}

	for _, group := range adClient.HomeGroups {
		_, err := adClient.Client.DeleteGroupMembers(group, user.Upn)
		if err != nil {
			return fmt.Errorf("unable to remove user from group: %w", err)
		}
	}
	return nil
}

func UserReturning(user sqlc.User, adClient activedirectory.Client) error {
	for _, group := range adClient.HomeGroups {
		_, err := adClient.Client.AddGroupMembers(group, user.Upn)
		if err != nil {
			return fmt.Errorf("unable to add user to group: %w", err)
		}
	}

	for _, group := range adClient.AwayGroups {
		_, err := adClient.Client.DeleteGroupMembers(group, user.Upn)
		if err != nil {
			return fmt.Errorf("unable to remove user from group: %w", err)
		}
	}

	if adClient.MFAGroup != "" && !user.Staff.Bool {
		_, err := adClient.Client.DeleteGroupMembers(adClient.MFAGroup, user.Upn)
		if err != nil {
			return fmt.Errorf("unable to remove user froom mfa group: %w", err)
		}
	}
	return nil
}
