package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"go.uber.org/zap"

	"github.com/MaxPa1/Gofermart/internal/model"
	"github.com/MaxPa1/Gofermart/internal/transport/rest/middleware"
)

//go:generate go tool mockgen -source=auth.go -destination=mocks_test.go -package=handler -write_package_comment=false

const maxCredentialsBodySize = 4 << 10 // 4 КБ

type AuthService interface {
	Register(ctx context.Context, login, password string) (string, error)
	Login(ctx context.Context, login, password string) (string, error)
}

type credentialsRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type AuthHandler struct {
	auth         AuthService
	logger       *zap.Logger
	secureCookie bool
}

func NewAuthHandler(auth AuthService, logger *zap.Logger, secureCookie bool) *AuthHandler {
	return &AuthHandler{auth: auth, logger: logger, secureCookie: secureCookie}
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeCredentials(w, r)
	if !ok {
		return
	}

	token, err := h.auth.Register(r.Context(), req.Login, req.Password)
	switch {
	case errors.Is(err, model.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, model.ErrLoginTaken):
		writeError(w, http.StatusConflict, "")
		return
	case err != nil:
		writeInternalError(w, r, h.logger, "register", err)
		return
	}

	h.writeToken(w, token)
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	req, ok := h.decodeCredentials(w, r)
	if !ok {
		return
	}

	token, err := h.auth.Login(r.Context(), req.Login, req.Password)
	switch {
	case errors.Is(err, model.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
		return
	case errors.Is(err, model.ErrInvalidCredentials):
		writeError(w, http.StatusUnauthorized, "")
		return
	case err != nil:
		writeInternalError(w, r, h.logger, "login", err)
		return
	}

	h.writeToken(w, token)
}

func (h *AuthHandler) decodeCredentials(w http.ResponseWriter, r *http.Request) (credentialsRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCredentialsBodySize)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req credentialsRequest
	if err := dec.Decode(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "")
			return credentialsRequest{}, false
		}
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return credentialsRequest{}, false
	}

	return req, true
}

func (h *AuthHandler) writeToken(w http.ResponseWriter, token string) {
	w.Header().Set("Cache-Control", "no-store")

	http.SetCookie(w, &http.Cookie{
		Name:     middleware.AuthCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.secureCookie,
		SameSite: http.SameSiteStrictMode,
	})

	w.WriteHeader(http.StatusOK)
}
