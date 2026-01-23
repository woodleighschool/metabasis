package admin

import (
	"log/slog"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/woodleighschool/adoverseas/internal/config"
	"github.com/woodleighschool/adoverseas/internal/store"
)

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
	r.Route("/schedules", h.schedulesRoutes)
	r.Route("/users", h.userRoutes)
}
