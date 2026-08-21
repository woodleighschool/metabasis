package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/intent"
	"github.com/woodleighschool/metabasis/internal/metrics"
	"github.com/woodleighschool/metabasis/internal/store"
)

const maximumWebhookBody = 64 << 10

type acceptFunc func(context.Context, intent.Intent, time.Time) error

// NewHandler builds the operational HTTP surface for configured webhook sources and probes.
func NewHandler(cfg *config.Config, intentStore *store.Store, wake chan<- struct{}, logger *slog.Logger, recorder *metrics.Recorder) http.Handler {
	return newHandler(cfg, intentStore.UpsertIntent, intentStore.Ping, wake, logger, recorder)
}

func newHandler(
	cfg *config.Config,
	accept acceptFunc,
	ready func(context.Context) error,
	wake chan<- struct{},
	logger *slog.Logger,
	recorder *metrics.Recorder,
) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(response http.ResponseWriter, _ *http.Request) {
		response.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("GET /readyz", func(response http.ResponseWriter, request *http.Request) {
		ctx, cancel := context.WithTimeout(request.Context(), 2*time.Second)
		defer cancel()
		if err := ready(ctx); err != nil {
			writeError(response, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		response.WriteHeader(http.StatusNoContent)
	})
	for source, webhook := range cfg.Webhooks {
		mux.HandleFunc("POST "+webhook.Path, func(response http.ResponseWriter, request *http.Request) {
			handleWebhook(response, request, source, webhook.BearerToken, accept, wake, logger, recorder)
		})
	}
	return mux
}

func handleWebhook(
	response http.ResponseWriter,
	request *http.Request,
	source string,
	bearerToken string,
	accept acceptFunc,
	wake chan<- struct{},
	logger *slog.Logger,
	recorder *metrics.Recorder,
) {
	if !validBearerToken(request.Header.Get("Authorization"), bearerToken) {
		recorder.RecordWebhook(source, metrics.WebhookUnauthorized)
		response.Header().Set("WWW-Authenticate", "Bearer")
		writeError(response, http.StatusUnauthorized, "invalid authentication")
		return
	}
	request.Body = http.MaxBytesReader(response, request.Body, maximumWebhookBody)
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	var accepted intent.Intent
	if err := decoder.Decode(&accepted); err != nil {
		recorder.RecordWebhook(source, metrics.WebhookInvalid)
		writeError(response, http.StatusBadRequest, "invalid JSON body")
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		recorder.RecordWebhook(source, metrics.WebhookInvalid)
		writeError(response, http.StatusBadRequest, "body must contain one JSON object")
		return
	}
	accepted.Source = source
	if err := accepted.Validate(); err != nil {
		recorder.RecordWebhook(source, metrics.WebhookInvalid)
		writeError(response, http.StatusBadRequest, err.Error())
		return
	}
	if err := accept(request.Context(), accepted, time.Now().UTC()); err != nil {
		recorder.RecordWebhook(source, metrics.WebhookError)
		logger.ErrorContext(request.Context(), "persist webhook intent", "source", source, "error", err)
		writeError(response, http.StatusInternalServerError, "failed to persist intent")
		return
	}
	recorder.RecordWebhook(source, metrics.WebhookAccepted)
	select {
	case wake <- struct{}{}:
	default:
	}
	writeJSON(response, http.StatusAccepted, map[string]string{
		"status": "accepted",
		"source": source,
		"id":     accepted.ID,
	})
}

func validBearerToken(header, expected string) bool {
	scheme, token, found := strings.Cut(header, " ")
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

func writeError(response http.ResponseWriter, status int, message string) {
	writeJSON(response, status, map[string]string{"error": message})
}

func writeJSON(response http.ResponseWriter, status int, body any) {
	response.Header().Set("Content-Type", "application/json")
	response.WriteHeader(status)
	_ = json.NewEncoder(response).Encode(body)
}
