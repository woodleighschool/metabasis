package httpapi

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/woodleighschool/metabasis/internal/config"
	"github.com/woodleighschool/metabasis/internal/intent"
)

func TestWebhookAuthenticationValidationAndDurableAcceptance(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Webhooks: map[string]config.Webhook{
		"freshservice": {Path: "/webhooks/freshservice", BearerToken: "secret"},
	}}
	var accepted []intent.Intent
	accept := func(_ context.Context, value intent.Intent, _ time.Time) error {
		accepted = append(accepted, value)
		return nil
	}
	wake := make(chan struct{}, 1)
	handler := newHandler(cfg, accept, func(context.Context) error { return nil }, wake, slog.New(slog.DiscardHandler))
	body := `{"id":"SR-1","subject":"student@example.com","starts_at":"2026-09-12T08:00:00+10:00","ends_at":"2026-09-27T18:00:00+10:00","cancelled":false}`

	unauthorized := httptest.NewRequest(http.MethodPost, "/webhooks/freshservice", bytes.NewBufferString(body))
	unauthorizedResponse := httptest.NewRecorder()
	handler.ServeHTTP(unauthorizedResponse, unauthorized)
	if got := unauthorizedResponse.Code; got != http.StatusUnauthorized {
		t.Fatalf("unauthorized status = %d, want %d", got, http.StatusUnauthorized)
	}

	invalid := httptest.NewRequest(http.MethodPost, "/webhooks/freshservice", bytes.NewBufferString(`{"id":"SR-1"}`))
	invalid.Header.Set("Authorization", "Bearer secret")
	invalidResponse := httptest.NewRecorder()
	handler.ServeHTTP(invalidResponse, invalid)
	if got := invalidResponse.Code; got != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, want %d", got, http.StatusBadRequest)
	}

	valid := httptest.NewRequest(http.MethodPost, "/webhooks/freshservice", bytes.NewBufferString(body))
	valid.Header.Set("Authorization", "Bearer secret")
	validResponse := httptest.NewRecorder()
	handler.ServeHTTP(validResponse, valid)
	if got := validResponse.Code; got != http.StatusAccepted {
		t.Fatalf("valid status = %d, want %d: %s", got, http.StatusAccepted, validResponse.Body.String())
	}
	if len(accepted) != 1 || accepted[0].Source != "freshservice" || accepted[0].ID != "SR-1" {
		t.Fatalf("accepted = %+v", accepted)
	}
	select {
	case <-wake:
	default:
		t.Fatal("accepted webhook did not wake reconciler")
	}
}

func TestWebhookPersistenceFailureIsNotAcknowledgedOrWoken(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Webhooks: map[string]config.Webhook{
		"freshservice": {Path: "/webhooks/freshservice", BearerToken: "secret"},
	}}
	wake := make(chan struct{}, 1)
	handler := newHandler(
		cfg,
		func(context.Context, intent.Intent, time.Time) error { return errors.New("database unavailable") },
		func(context.Context) error { return nil },
		wake,
		slog.New(slog.DiscardHandler),
	)
	body := `{"id":"SR-1","subject":"student@example.com","starts_at":"2026-09-12T08:00:00Z","ends_at":"2026-09-12T09:00:00Z"}`
	request := httptest.NewRequest(http.MethodPost, "/webhooks/freshservice", bytes.NewBufferString(body))
	request.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if got := response.Code; got != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", got, http.StatusInternalServerError)
	}
	select {
	case <-wake:
		t.Fatal("failed persistence woke reconciler")
	default:
	}
}

func TestWebhookRejectsUnknownFieldsAndInvalidWindow(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{Webhooks: map[string]config.Webhook{
		"freshservice": {Path: "/webhooks/freshservice", BearerToken: "secret"},
	}}
	handler := newHandler(
		cfg,
		func(context.Context, intent.Intent, time.Time) error { return nil },
		func(context.Context) error { return nil },
		make(chan struct{}, 1),
		slog.New(slog.DiscardHandler),
	)
	tests := []string{
		`{"id":"SR-1","subject":"student@example.com","starts_at":"2026-09-12T08:00:00Z","ends_at":"2026-09-12T07:00:00Z"}`,
		`{"id":"SR-1","subject":"student@example.com","starts_at":"2026-09-12T08:00:00Z","ends_at":"2026-09-12T09:00:00Z","native_payload":true}`,
	}
	for _, body := range tests {
		request := httptest.NewRequest(http.MethodPost, "/webhooks/freshservice", bytes.NewBufferString(body))
		request.Header.Set("Authorization", "Bearer secret")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if got := response.Code; got != http.StatusBadRequest {
			t.Errorf("status = %d, want %d for %s", got, http.StatusBadRequest, body)
		}
	}
}
