package admin

import (
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/woodleighschool/adoverseas/internal/http/utils"
	"github.com/woodleighschool/adoverseas/internal/store/sqlc"
)

type userResponse struct {
	ID          uuid.UUID `json:"id"`
	UPN         string    `json:"upn"`
	DisplayName string    `json:"displayName"`
	Staff       bool      `json:"staff"`
}

func (h Handler) userRoutes(r chi.Router) {
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getSchedule)
	})
}

func (h Handler) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r, "id")
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	ctx := r.Context()
	user, err := h.Store.GetUser(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondError(w, http.StatusNotFound, "user not found")
			return
		}
		h.Logger.Error("get user", "err", err)
		utils.RespondError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	userResponse := mapUser(user)
	utils.RespondJSON(w, http.StatusOK, userResponse)
}

func mapUser(u sqlc.User) userResponse {
	return userResponse{
		ID:          u.ID,
		UPN:         u.Upn,
		DisplayName: u.DisplayName,
		Staff:       u.Staff.Bool,
	}
}
