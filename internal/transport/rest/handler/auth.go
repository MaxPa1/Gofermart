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

// AuthService — бизнес-логика регистрации и входа, которую использует AuthHandler.
type AuthService interface {
	// Register регистрирует пользователя и возвращает токен аутентификации.
	Register(ctx context.Context, login, password string) (string, error)
	// Login проверяет логин и пароль и возвращает токен аутентификации.
	Login(ctx context.Context, login, password string) (string, error)
}

type credentialsRequest struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

// AuthHandler обрабатывает регистрацию и вход пользователей. При успехе выдаёт токен в cookie
// middleware.AuthCookieName с флагами HttpOnly и SameSite=Strict
type AuthHandler struct {
	auth         AuthService
	logger       *zap.Logger
	secureCookie bool
}

func NewAuthHandler(auth AuthService, logger *zap.Logger, secureCookie bool) *AuthHandler {
	return &AuthHandler{auth: auth, logger: logger, secureCookie: secureCookie}
}

// Register обрабатывает POST /api/user/register с JSON-телом {"login": "…", "password": "…"}. Ответы:
//   - 200 — пользователь зарегистрирован и аутентифицирован, токен выдан в cookie;
//   - 400 — неверный формат запроса;
//   - 409 — логин уже занят;
//   - 413 — тело запроса больше 4 КБ;
//   - 500 — внутренняя ошибка сервера.
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

// Login обрабатывает POST /api/user/login с JSON-телом {"login": "…", "password": "…"}. Ответы:
//   - 200 — пользователь аутентифицирован, токен выдан в cookie;
//   - 400 — неверный формат запроса;
//   - 401 — неверная пара логин/пароль;
//   - 413 — тело запроса больше 4 КБ;
//   - 500 — внутренняя ошибка сервера.
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
