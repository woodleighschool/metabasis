package store

import (
	"context"

	"github.com/google/uuid"

	"github.com/woodleighschool/adoverseas/internal/store/sqlc"
)

func (s *Store) GetUsers(ctx context.Context) ([]sqlc.User, error) {
	return s.queries.GetUsers(ctx)
}

func (s *Store) GetUser(ctx context.Context, id uuid.UUID) (sqlc.User, error) {
	return s.queries.GetUser(ctx, id)
}

func (s *Store) GetUserByUPN(ctx context.Context, upn string) (sqlc.User, error) {
	return s.queries.GetUserByUPN(ctx, upn)
}

func (s *Store) GetUserAsset(ctx context.Context, id uuid.UUID) (sqlc.UserAsset, error) {
	return s.queries.GetUserAsset(ctx, id)
}

func (s *Store) UpsertUserAsset(ctx context.Context, asset sqlc.UpsertUserAssetParams) (sqlc.UserAsset, error) {
	return s.queries.UpsertUserAsset(ctx, asset)
}

func (s *Store) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteUser(ctx, id)
}

func (s *Store) DeleteUserByUPN(ctx context.Context, upn string) error {
	return s.queries.DeleteUserByUPN(ctx, upn)
}

func (s *Store) UpsertUser(ctx context.Context, user sqlc.UpsertUserParams) (sqlc.User, error) {
	return s.queries.UpsertUser(ctx, user)
}

func (s *Store) InsertSchedule(ctx context.Context, schedule sqlc.InsertScheduleParams) (sqlc.Schedule, error) {
	return s.queries.InsertSchedule(ctx, schedule)
}

func (s *Store) UpdateSchedule(ctx context.Context, schedule sqlc.UpdateScheduleParams) error {
	return s.queries.UpdateSchedule(ctx, schedule)
}

func (s *Store) FlipSchedule(ctx context.Context, id uuid.UUID) error {
	return s.queries.FlipSchedule(ctx, id)
}

func (s *Store) ListScheduleSummaries(ctx context.Context) ([]sqlc.ListScheduleSummariesRow, error) {
	return s.queries.ListScheduleSummaries(ctx)
}

func (s *Store) GetSchedule(ctx context.Context, id uuid.UUID) (sqlc.Schedule, error) {
	return s.queries.GetSchdeule(ctx, id)
}

func (s *Store) DeleteSchedule(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteSchedule(ctx, id)
}

func (s *Store) InsertUrgentSchedule(ctx context.Context, userid uuid.UUID) (sqlc.UrgentSchedule, error) {
	return s.queries.InsertUrgentSchedule(ctx, userid)
}

func (s *Store) ListUrgentSchedules(ctx context.Context) ([]sqlc.ListUrgentSchedulesRow, error) {
	return s.queries.ListUrgentSchedules(ctx)
}

func (s *Store) DeleteUrgentSchedule(ctx context.Context, id uuid.UUID) error {
	return s.queries.DeleteUrgentSchedule(ctx, id)
}
