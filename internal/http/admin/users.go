package admin

import (
	"bytes"
	"errors"
	"net/http"
	"time"

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
	r.Get("/", h.getUsers)
	r.Route("/{id}", func(r chi.Router) {
		r.Get("/", h.getUser)
		r.Get("/photo", h.getUserAsset)
	})
}

func (h Handler) getUser(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r)
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
		h.Logger.ErrorContext(r.Context(), "get user", "err", err)
		utils.RespondError(w, http.StatusInternalServerError, "failed to get user")
		return
	}
	userResponse := mapUser(user)
	utils.RespondJSON(w, http.StatusOK, userResponse)
}

func (h Handler) getUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.Store.GetUsers(r.Context())
	if err != nil {
		h.Logger.ErrorContext(r.Context(), "get users", "err", err)
		utils.RespondError(w, http.StatusInternalServerError, "failed to get users")
		return
	}
	resp := make([]userResponse, 0, len(users))
	for _, u := range users {
		resp = append(resp, mapUser(u))
	}
	utils.RespondJSON(w, http.StatusOK, resp)
}

func mapUser(u sqlc.User) userResponse {
	return userResponse{
		ID:          u.ID,
		UPN:         u.Upn,
		DisplayName: u.DisplayName,
		Staff:       u.Staff.Bool,
	}
}

func (h Handler) getUserAsset(w http.ResponseWriter, r *http.Request) {
	id, err := parseUUIDParam(r)
	if err != nil {
		utils.RespondError(w, http.StatusBadRequest, "invalid user id")
		return
	}
	ctx := r.Context()
	asset, err := h.Store.GetUserAsset(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			utils.RespondError(w, http.StatusNotFound, "user asset not found")
			return
		}
		h.Logger.ErrorContext(r.Context(), "get user asset", "err", err)
		utils.RespondError(w, http.StatusInternalServerError, "failed to get user asset")
		return
	}

	modTime := time.Now()
	if asset.UpdatedAt.Valid {
		modTime = asset.UpdatedAt.Time
	}

	w.Header().Set("Content-Type", asset.ContentType)
	w.Header().Set("Cache-Control", "public, max-age=300, stale-while-revalidate=300")
	http.ServeContent(w, r, id.String(), modTime, bytes.NewReader(asset.Data))
}
