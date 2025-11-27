package admin

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/woodleighschool/adoverseas/internal/http/utils"
	"github.com/woodleighschool/adoverseas/internal/store/sqlc"
)

type scheduleBody struct {
	UPN           string `json:"upn"`
	LeavingDate   string `json:"leaving_date"`
	ReturningDate string `json:"returning_date"`
	LastUpdatedBy string `json:"last_updated_by"`
}

type updateScheduleRequest struct {
	UPN           string  `json:"upn"`
	LeavingDate   *string `json:"leaving_date,omitempty"`
	ReturningDate *string `json:"returning_date,omitempty"`
	LastUpdatedBy string  `json:"last_updated_by,omitempty"`
}

type scheduleSummaryResponse struct {
	ID            uuid.UUID `json:"id"`
	User          uuid.UUID `json:"user"`
	DisplayName   string    `json:"display_name"`
	UPN           string    `json:"upn"`
	LeavingDate   time.Time `json:"leaving_date"`
	ReturningDate time.Time `json:"returning_date"`
	Overseas      bool      `json:"overseas"`
	LastUpdatedBy string    `json:"last_updated_by"`
	LastUpdated   time.Time `json:"last_updated"`
}

type scheduleResponse struct {
	ID            uuid.UUID `json:"id"`
	User          uuid.UUID `json:"user"`
	LeavingDate   time.Time `json:"leaving_date"`
	ReturningDate time.Time `json:"returning_date"`
	Overseas      bool      `json:"overseas"`
	LastUpdatedBy string    `json:"last_updated_by"`
	LastUpdated   time.Time `json:"last_updated"`
}

func (h Handler) schedulesRoutes(r chi.Router) {
	r.Get("/", h.listSchedules)
	r.Post("/", h.insertSchedules)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getSchedule)
		r.Patch("/", h.updateSchedule)
		r.Delete("/", h.deleteSchedule)
	})
}

func (h Handler) listSchedules(w http.ResponseWriter, r *http.Request) {
	tasks, err := h.Store.ListScheduleSummaries(r.Context())
	if err != nil {
		h.Logger.Error("list tasks", "err", err)
		utils.RespondError(w, http.StatusInternalServerError, "failed to list tasks")
		return
	}
	resp := make([]scheduleSummaryResponse, 0, len(tasks))
	for _, t := range tasks {
		resp = append(resp, mapScheduleSummary(t))
	}
	utils.RespondJSON(w, http.StatusOK, resp)
}

func (h Handler) insertSchedules(w http.ResponseWriter, r *http.Request) {
	var body scheduleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	user, err := h.Store.GetUserByUPN(r.Context(), body.UPN)
	if err != nil {
		h.Logger.Error("retrieve user from api body", "err", err)
		utils.RespondError(w, http.StatusBadRequest, "failed to retrieve user from body")
		return
	}
	leavingDate, err := time.Parse(time.RFC3339Nano, body.LeavingDate)
	if err != nil {
		h.Logger.Error("parsing leaving date", "err", err)
		utils.RespondError(w, http.StatusBadRequest, "unable to parse leaving date")
		return
	}
	returningDate, err := time.Parse(time.RFC3339Nano, body.ReturningDate)
	if err != nil {
		h.Logger.Error("parsing returning date", "err", err)
		utils.RespondError(w, http.StatusBadRequest, "unable to parse returning date")
		return
	}
	_, err = h.Store.InsertSchedule(r.Context(), sqlc.InsertScheduleParams{
		Userid:        user.ID,
		LeavingDate:   pgtype.Timestamptz{Time: leavingDate, Valid: true},
		ReturningDate: pgtype.Timestamptz{Time: returningDate, Valid: true},
		LastChangedBy: body.LastUpdatedBy,
	})
	utils.RespondJSON(w, http.StatusAccepted, map[string]any{
		"message": "successfully added schedule",
	})
}

func (h Handler) getSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}
	ctx := r.Context()
	schedule, err := h.Store.GetSchedule(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondError(w, http.StatusNotFound, "schedule not found")
			return
		}
		h.Logger.Error("get schedule", "err", err)
		utils.RespondError(w, http.StatusInternalServerError, "failed to get schedule")
		return
	}
	scheduleResponse := mapSchedule(schedule)
	utils.RespondJSON(w, http.StatusOK, scheduleResponse)
}

func (h Handler) updateSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}
	var body updateScheduleRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	current, err := h.Store.GetSchedule(r.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondError(w, http.StatusNotFound, "schedule not found")
			return
		}
		h.Logger.Error("get schedule", "err", err)
		utils.RespondError(w, http.StatusInternalServerError, "failed to load schedule")
		return
	}
	payload := sqlc.UpdateScheduleParams{
		ID:            current.ID,
		LeavingDate:   current.LeavingDate,
		ReturningDate: current.ReturningDate,
		Overseas:      current.Overseas,
		LastChangedBy: current.LastChangedBy,
	}
	if body.LeavingDate != nil {
		leavingDate, err := time.Parse(time.RFC3339Nano, *body.LeavingDate)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, "unable to parse leaving_date")
			return
		}
		if leavingDate.Before(time.Now()) {
			payload.Overseas = true
		}
		payload.LeavingDate = pgtype.Timestamptz{Time: leavingDate, Valid: true}
	}
	if body.ReturningDate != nil {
		returningDate, err := time.Parse(time.RFC3339Nano, *body.ReturningDate)
		if err != nil {
			utils.RespondError(w, http.StatusBadRequest, "unable to parse returning_date")
			return
		}
		payload.ReturningDate = pgtype.Timestamptz{Time: returningDate, Valid: true}
	}
	payload.LastChangedBy = body.LastUpdatedBy
	if err := h.Store.UpdateSchedule(r.Context(), payload); err != nil {
		h.Logger.Error("update schedule", "err", err)
		utils.RespondError(w, http.StatusInternalServerError, "failed to update schedule")
		return
	}
	h.Logger.Info("successfully updated schedule", "user", body.UPN, "leaving_date", payload.LeavingDate, "returning_date", payload.ReturningDate)
	utils.RespondJSON(w, http.StatusOK, map[string]any{
		"message": "successfully updated schedule",
	})
}

func (h Handler) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}
	if err := h.Store.DeleteSchedule(r.Context(), id); err != nil {
		h.Logger.Error("delete schedule", "err", err)
		utils.RespondError(w, http.StatusInternalServerError, "failed to delete schedule")
	}
	w.WriteHeader(http.StatusNoContent)
}

func mapScheduleSummary(s sqlc.ListScheduleSummariesRow) scheduleSummaryResponse {
	var leavingDate, returningDate, lastChanged time.Time
	if s.LeavingDate.Valid {
		leavingDate = s.LeavingDate.Time
	}
	if s.ReturningDate.Valid {
		returningDate = s.ReturningDate.Time
	}
	if s.LastChanged.Valid {
		lastChanged = s.LastChanged.Time
	}
	return scheduleSummaryResponse{
		ID:            s.ID,
		User:          s.Userid,
		DisplayName:   s.DisplayName,
		UPN:           s.Upn,
		LeavingDate:   leavingDate,
		ReturningDate: returningDate,
		Overseas:      s.Overseas,
		LastUpdatedBy: s.LastChangedBy,
		LastUpdated:   lastChanged,
	}
}

func mapSchedule(s sqlc.Schedule) scheduleResponse {
	var leavingDate, returningDate, lastChanged time.Time
	if s.LeavingDate.Valid {
		leavingDate = s.LeavingDate.Time
	}
	if s.ReturningDate.Valid {
		returningDate = s.ReturningDate.Time
	}
	if s.LastChanged.Valid {
		lastChanged = s.LastChanged.Time
	}
	return scheduleResponse{
		ID:            s.ID,
		User:          s.Userid,
		LeavingDate:   leavingDate,
		ReturningDate: returningDate,
		Overseas:      s.Overseas,
		LastUpdatedBy: s.LastChangedBy,
		LastUpdated:   lastChanged,
	}
}
