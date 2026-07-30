package schedule

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/woodleighschool/adoverseas/internal/config"
	"github.com/woodleighschool/adoverseas/internal/http/utils"
	"github.com/woodleighschool/adoverseas/internal/store"
	"github.com/woodleighschool/adoverseas/internal/store/sqlc"
)

type scheduleBody struct {
	Email         string `json:"email"`
	LeavingDate   string `json:"leaving_date"`
	ReturningDate string `json:"returning_date"`
	UpdatedBy     string `json:"updatedBy"`
}

type Handler struct {
	Store  *store.Store
	Logger *slog.Logger
	Config config.Config
	TZ     *time.Location
}

func RegisterRoutes(r chi.Router, cfg config.Config, store *store.Store, logger *slog.Logger) {
	h := Handler{Store: store, Logger: logger, Config: cfg}
	tz, err := time.LoadLocation(cfg.TimeLocation)
	if err != nil {
		logger.Warn("Unable to determine timezone from TIME_LOCATION", "err", err)
		h.TZ = time.UTC
	} else {
		h.TZ = tz
	}

	r.Route("/", func(r chi.Router) {
		r.Post("/", h.insertSchedule)
	})
}

func (h Handler) insertSchedule(w http.ResponseWriter, r *http.Request) {
	var body scheduleBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid body")
		return
	}
	user, err := h.Store.GetUserByUPN(r.Context(), body.Email)
	if err != nil {
		h.Logger.ErrorContext(r.Context(), "retrieve user from api body", "err", err)
		utils.RespondError(w, http.StatusBadRequest, "failed to retrieve user from body")
		return
	}
	leavingDate, err := time.ParseInLocation(time.RFC3339Nano, body.LeavingDate, h.TZ)
	if err != nil {
		h.Logger.ErrorContext(r.Context(), "parsing leaving date", "err", err)
		utils.RespondError(w, http.StatusBadRequest, "unable to parse leaving date")
		return
	}
	returningDate, err := time.ParseInLocation(time.RFC3339Nano, body.ReturningDate, h.TZ)
	if err != nil {
		h.Logger.ErrorContext(r.Context(), "parsing returning date", "err", err)
		utils.RespondError(w, http.StatusBadRequest, "unable to parse returning date")
		return
	}
	if !user.Staff.Bool {
		_, err := h.Store.InsertUrgentSchedule(r.Context(), user.ID)
		if err != nil {
			h.Logger.ErrorContext(r.Context(), "failed to insert urgent schedule", "err", err)
		}
	}
	if _, err := h.Store.InsertSchedule(r.Context(), sqlc.InsertScheduleParams{
		Userid:        user.ID,
		LeavingDate:   pgtype.Timestamptz{Time: leavingDate, Valid: true},
		ReturningDate: pgtype.Timestamptz{Time: returningDate, Valid: true},
	}); err != nil {
		h.Logger.ErrorContext(r.Context(), "insert schedule", "err", err)
		utils.RespondError(w, http.StatusInternalServerError, "failed to insert schedule")
		return
	}
	h.Logger.InfoContext(r.Context(), "new schedule added", "user", user.Upn, "leaving_date", leavingDate, "returning_date", returningDate)
	utils.RespondJSON(w, http.StatusAccepted, map[string]any{
		"status":         "success",
		"user":           user.Upn,
		"leaving_date":   leavingDate,
		"returning_date": returningDate,
	})
}
