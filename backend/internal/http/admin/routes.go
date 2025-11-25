package admin

import (
	"log/slog"

	"github.com/go-chi/chi/v5"

	"github.com/woodleighschool/adoverseas/internal/config"
	"github.com/woodleighschool/adoverseas/internal/store"
)

type Handler struct {
	Store  *store.Store
	Logger *slog.Logger
	Config config.Config
}

func RegisterRoutes(r chi.Router, cfg config.Config, store *store.Store, logger *slog.Logger) {
	h := Handler{Store: store, Logger: logger, Config: cfg}
	r.Route("/schedules", h.schedulesRoutes)
	r.Route("/users", h.userRoutes)
}
