package auth

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/go-chi/chi/v5"

	"github.com/woodleighschool/adoverseas/internal/auth"
	"github.com/woodleighschool/adoverseas/internal/config"
	"github.com/woodleighschool/adoverseas/internal/store"
)

const (
	stateCookieTTL  = 10 * time.Minute
	defaultRedirect = "/"
)

type Handler struct {
	provider             *auth.OIDCProvider
	sessions             *auth.SessionManager
	store                *store.Store
	logger               *slog.Logger
	siteURL              string
	stateCookie          string
	secureCookie         bool
	initialAdminPassword string
}

type oidcState struct {
	State    string `json:"state"`
	Nonce    string `json:"nonce"`
	Redirect string `json:"redirect"`
}

type activeUser struct {
	DisplayName string `json:"display_name"`
	Photo       []byte `json:"photo"`
}

func RegisterRoutes(r chi.Router, cfg config.Config, provider *auth.OIDCProvider, sessions *auth.SessionManager, logger *slog.Logger) {
	if sessions == nil {
		if logger != nil {
			logger.Warn("auth routes disabled: missing session manager")
		}
		return
	}
	h := &Handler{
		provider:             provider,
		sessions:             sessions,
		logger:               logger,
		siteURL:              cfg.SiteBaseURL,
		stateCookie:          cfg.SessionCookieName + "_oidc_state",
		secureCookie:         strings.HasPrefix(cfg.SiteBaseURL, "https"),
		initialAdminPassword: cfg.InitialAdminPassword,
	}

	r.Route("/login", func(r chi.Router) {
		r.Get("/", h.login)
		r.Post("/", h.login)
	})

	if provider != nil {
		r.Get("/callback", h.callback)
	}

	r.Get("/me", h.me)
	r.Get("/providers", h.providers)
	r.Post("/logout", h.logout)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	method := r.URL.Query().Get("method")

	if r.Method == "POST" || method == "local" {
		h.localLogin(w, r)
		return
	}

	if h.provider == nil {
		http.Error(w, "OAuth login not configured", http.StatusNotFound)
		return
	}

	state, err := randomString(32)
	if err != nil {
		http.Error(w, "failed to generate state", http.StatusInternalServerError)
		return
	}
	nonce, err := randomString(32)
	if err != nil {
		http.Error(w, "failed to generate nonce", http.StatusInternalServerError)
		return
	}
	redirect := r.URL.Query().Get("redirect")
	if redirect == "" || !strings.HasPrefix(redirect, h.siteURL) {
		redirect = strings.TrimRight(h.siteURL, "/") + defaultRedirect
	}
	if err := h.setStateCookie(w, oidcState{State: state, Nonce: nonce, Redirect: redirect}); err != nil {
		http.Error(w, "failed to persist state", http.StatusInternalServerError)
		return
	}
	authURL := h.provider.AuthCodeURL(state, oidc.Nonce(nonce))
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (h *Handler) callback(w http.ResponseWriter, r *http.Request) {
	stored, err := h.readStateCookie(r)
	if err != nil {
		http.Error(w, "missing login context", http.StatusBadRequest)
		return
	}
	h.clearStateCookie(w)
	if r.URL.Query().Get("state") != stored.State {
		http.Error(w, "state mismatch", http.StatusBadRequest)
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "authorization code missing", http.StatusBadRequest)
		return
	}
	ctx := r.Context()
	token, err := h.provider.Exchange(ctx, code)
	if err != nil {
		h.logger.Error("oidc exchange failed", "err", err)
		http.Error(w, "oidc exchange failed", http.StatusBadRequest)
		return
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		http.Error(w, "id token missing", http.StatusBadRequest)
		return
	}
	idToken, err := h.provider.VerifyIDToken(ctx, rawIDToken)
	if err != nil {
		h.logger.Error("verify id token", "err", err)
		http.Error(w, "invalid id token", http.StatusBadRequest)
		return
	}
	if idToken.Nonce != stored.Nonce {
		http.Error(w, "nonce mismatch", http.StatusBadRequest)
		return
	}
	claims := map[string]any{}
	if err := idToken.Claims(&claims); err != nil {
		h.logger.Error("decode claims", "err", err)
		http.Error(w, "invalid claims", http.StatusBadRequest)
		return
	}
	session := auth.Session{
		Subject: idToken.Subject,
		Claims:  sanitiseClaims(claims),
	}
	if err := h.sessions.Issue(w, session); err != nil {
		h.logger.Error("issue session", "err", err)
		http.Error(w, "failed to issue session", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, stored.Redirect, http.StatusFound)
}

func (h *Handler) logout(w http.ResponseWriter, _ *http.Request) {
	h.sessions.Clear(w)
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	session, err := h.sessions.Read(r)
	if err != nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	displayName := session.Subject
	if name, ok := session.Claims["name"].(string); ok && name != "" {
		displayName = name
	}

	userID := "none"
	if id, ok := session.Claims["oid"].(string); ok && id != "" {
		userID = id
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"display_name": displayName, "user_id": userID}); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) providers(w http.ResponseWriter, r *http.Request) {
	providers := map[string]bool{
		"oauth": h.provider != nil,
		"local": h.initialAdminPassword != "",
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(providers); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) localLogin(w http.ResponseWriter, r *http.Request) {
	if h.initialAdminPassword == "" {
		http.Error(w, "Local login disabled", http.StatusNotFound)
		return
	}

	var loginReq struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&loginReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if loginReq.Username != "admin" || loginReq.Password != h.initialAdminPassword {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	session := auth.Session{
		Subject: "local:admin",
		Claims: map[string]any{
			"name": "Administrator",
		},
	}

	if err := h.sessions.Issue(w, session); err != nil {
		h.logger.Error("issue session for local admin", "err", err)
		http.Error(w, "failed to create session", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Conten-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]string{"display_name": "Master Claus"}); err != nil {
		h.logger.Error("encode local login response", "err", err)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (h *Handler) setStateCookie(w http.ResponseWriter, st oidcState) error {
	payload, err := json.Marshal(st)
	if err != nil {
		return err
	}
	value := base64.RawURLEncoding.EncodeToString(payload)
	http.SetCookie(w, &http.Cookie{
		Name:     h.stateCookie,
		Value:    value,
		Path:     "/api/auth",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(stateCookieTTL.Seconds()),
	})
	return nil
}

func (h *Handler) readStateCookie(r *http.Request) (oidcState, error) {
	cookie, err := r.Cookie(h.stateCookie)
	if err != nil {
		return oidcState{}, err
	}
	data, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return oidcState{}, err
	}
	var st oidcState
	if err := json.Unmarshal(data, &st); err != nil {
		return oidcState{}, err
	}
	if st.State == "" || st.Nonce == "" {
		return oidcState{}, errors.New("missing state payload")
	}
	return st, nil
}

func (h *Handler) clearStateCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     h.stateCookie,
		Value:    "",
		Path:     "/api/auth",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteLaxMode,
	})
}

func sanitiseClaims(claims map[string]any) map[string]any {
	out := make(map[string]any)
	for k, v := range claims {
		switch k {
		case "email", "name", "preferred_username", "upn", "oid", "tid":
			out[k] = v
		}
	}
	return out
}

func randomString(length int) (string, error) {
	buf := make([]byte, length)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}
