package schedules

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/woodleighschool/adoverseas/internal/config"
	"github.com/woodleighschool/adoverseas/internal/graph"
	"github.com/woodleighschool/adoverseas/internal/store"
	"github.com/woodleighschool/adoverseas/internal/store/sqlc"
)

func NewUserJob(store *store.Store, graphClient *graph.Client, logger *slog.Logger, cfg config.Config) Job {
	return func(ctx context.Context) error {
		if graphClient == nil || !graphClient.Enabled() {
			return graph.ErrNotConfigured
		}
		users, err := graphClient.FetchUsers(ctx)
		if err != nil {
			return fmt.Errorf("fetch users: %w", err)
		}
		for _, user := range users {
			syncDirectoryUser(ctx, store, logger, cfg, user)
		}
		return nil
	}
}

func syncDirectoryUser(
	ctx context.Context,
	store *store.Store,
	logger *slog.Logger,
	cfg config.Config,
	user graph.DirectoryUser,
) {
	if user.UPN == "" {
		return
	}
	userID, hasObjectID := parseDirectoryUserID(user.ObjectID)
	if shouldSkipUser(user) {
		if err := deleteDirectoryUser(ctx, store, userID, hasObjectID, user.UPN); err != nil {
			logger.ErrorContext(ctx, "delete user", "upn", user.UPN, "err", err)
		}
		return
	}

	staff := slices.Contains(cfg.StaffDepartment, user.Department)

	if !hasObjectID {
		existing, err := store.GetUserByUPN(ctx, user.UPN)
		if err != nil && !errors.Is(err, pgx.ErrNoRows) {
			logger.ErrorContext(ctx, "lookup user by UPN", "upn", user.UPN, "err", err)
			return
		}
		if err == nil {
			userID = existing.ID
		} else {
			userID = uuid.New()
		}
	}
	if _, err := store.UpsertUser(ctx, sqlc.UpsertUserParams{
		ID:          userID,
		Upn:         user.UPN,
		DisplayName: user.DisplayName,
		Staff:       pgtype.Bool{Bool: staff, Valid: true},
	}); err != nil {
		logger.ErrorContext(ctx, "upsert user", "upn", user.UPN, "err", err)
	}

	if user.Photo != nil {
		if _, err := store.UpsertUserAsset(ctx, sqlc.UpsertUserAssetParams{
			Userid:      userID,
			ContentType: "image/jpeg",
			Data:        user.Photo,
		}); err != nil {
			logger.ErrorContext(ctx, "upsert user asset", "upn", user.UPN, "err", err)
		}
	}
}

func shouldSkipUser(u graph.DirectoryUser) bool {
	if !u.Active {
		return true
	}
	return strings.Contains(strings.ToUpper(u.UPN), "#EXT#")
}

func parseDirectoryUserID(objectID string) (uuid.UUID, bool) {
	if objectID == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(objectID)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

func deleteDirectoryUser(
	ctx context.Context,
	store *store.Store,
	userID uuid.UUID,
	hasObjectID bool,
	upn string,
) error {
	if hasObjectID {
		return store.DeleteUser(ctx, userID)
	}
	if upn == "" {
		return nil
	}
	return store.DeleteUserByUPN(ctx, upn)
}
