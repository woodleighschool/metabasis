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
		for _, u := range users {
			if u.UPN == "" {
				continue
			}
			userID, hasObjectID := parseDirectoryUserID(u.ObjectID)
			if shouldSkipUser(u) {
				if err := deleteDirectoryUser(ctx, store, userID, hasObjectID, u.UPN); err != nil {
					logger.Error("delete user", "upn", u.UPN, "err", err)
				}
				continue
			}

			staff := slices.Contains(cfg.StaffDepartment, u.Department)

			if !hasObjectID {
				existing, err := store.GetUserByUPN(ctx, u.UPN)
				if err != nil && !errors.Is(err, pgx.ErrNoRows) {
					logger.Error("lookup user by UPN", "upn", u.UPN, "err", err)
					continue
				}
				if err == nil {
					userID = existing.ID
				} else {
					userID = uuid.New()
				}
			}
			if _, err := store.UpsertUser(ctx, sqlc.UpsertUserParams{
				ID:          userID,
				Upn:         u.UPN,
				DisplayName: u.DisplayName,
				Staff:       pgtype.Bool{Bool: staff, Valid: true},
			}); err != nil {
				logger.Error("upsert user", "upn", u.UPN, "err", err)
			}

			if u.Photo != nil {
				if _, err := store.UpsertUserAsset(ctx, sqlc.UpsertUserAssetParams{
					Userid:      userID,
					ContentType: "image/jpeg",
					Data:        u.Photo,
				}); err != nil {
					logger.Error("upsert user asset", "upn", u.UPN, "err", err)
				}
			}
		}
		return nil
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
