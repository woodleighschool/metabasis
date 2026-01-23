package httpapi

import (
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/woodleighschool/adoverseas/internal/auth"
	"github.com/woodleighschool/adoverseas/internal/config"
	"github.com/woodleighschool/adoverseas/internal/http/admin"
	authhttp "github.com/woodleighschool/adoverseas/internal/http/auth"
	apischedule "github.com/woodleighschool/adoverseas/internal/http/schedule"
	"github.com/woodleighschool/adoverseas/internal/store"
)

type Deps struct {
	Store        *store.Store
	Logger       *slog.Logger
	Sessions     *auth.SessionManager
	OIDCProvider *auth.OIDCProvider
	BuildInfo    BuildInfo
}

type BuildInfo struct {
	Version   string `json:"version"`
	GitCommit string `json:"gitCommit"`
	BuildDate string `json:"build_date"`
}

func Routes(cfg config.Config, deps Deps) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/api/v1/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":  "ok",
			"version": deps.BuildInfo,
		})
	})

	schedule := chi.NewRouter()
	schedule.Use(ApiAuth(cfg, deps.Logger))
	apischedule.RegisterRoutes(schedule, cfg, deps.Store, deps.Logger)
	r.Mount("/api/schedule", schedule)

	api := chi.NewRouter()
	api.Use(AdminAuth(deps.Sessions, deps.Logger))
	admin.RegisterRoutes(api, cfg, deps.Store, deps.Logger)
	r.Mount("/api/v1", api)

	authRoutes := chi.NewRouter()
	authhttp.RegisterRoutes(authRoutes, cfg, deps.OIDCProvider, deps.Sessions, deps.Logger)
	r.Mount("/api/auth", authRoutes)

	return r
}

func NewRouter(cfg config.Config, deps Deps) http.Handler {
	rootHandler := Routes(cfg, deps)
	if cfg.FrontendDistDir != "" {
		rootHandler = mountStatic(cfg.FrontendDistDir, rootHandler)
	}
	return rootHandler
}

func mountStatic(distDir string, apiHandler http.Handler) http.Handler {
	fileServer := http.FileServer(http.Dir(distDir))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") || strings.HasPrefix(r.URL.Path, "/schedule/") {
			apiHandler.ServeHTTP(w, r)
			return
		}

		path := filepath.Join(distDir, r.URL.Path)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			r.URL.Path = "/"
		}

		fileServer.ServeHTTP(w, r)
	})
}
