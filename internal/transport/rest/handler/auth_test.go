package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/MaxPa1/Gofermart/internal/model"
	"github.com/MaxPa1/Gofermart/internal/transport/rest/middleware"
)

const (
	testLogin    = "gopher"
	testPassword = "s3cret-Passw0rd"
	testToken    = "header.payload.signature"
	validBody    = `{"login":"gopher","password":"s3cret-Passw0rd"}`
)

var errInternal = errors.New("database is on fire")

func newTestHandler(t *testing.T, secureCookie bool) (*AuthHandler, *MockAuthService, *observer.ObservedLogs) {
	t.Helper()
	svc := NewMockAuthService(gomock.NewController(t))
	core, logs := observer.New(zap.DebugLevel)
	return NewAuthHandler(svc, zap.New(core), secureCookie), svc, logs
}

func serve(handler http.HandlerFunc, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler(rec, req)
	return rec
}

var endpoints = []struct {
	name   string
	path   string
	handle func(h *AuthHandler) http.HandlerFunc
	expect func(svc *MockAuthService) *gomock.Call
}{
	{
		name:   "register",
		path:   "/api/user/register",
		handle: func(h *AuthHandler) http.HandlerFunc { return h.Register },
		expect: func(svc *MockAuthService) *gomock.Call {
			return svc.EXPECT().Register(gomock.Any(), testLogin, testPassword)
		},
	},
	{
		name:   "login",
		path:   "/api/user/login",
		handle: func(h *AuthHandler) http.HandlerFunc { return h.Login },
		expect: func(svc *MockAuthService) *gomock.Call {
			return svc.EXPECT().Login(gomock.Any(), testLogin, testPassword)
		},
	},
}

func TestAuthHandler_Success(t *testing.T) {
	for _, ep := range endpoints {
		t.Run(ep.name, func(t *testing.T) {
			h, svc, _ := newTestHandler(t, true)
			ep.expect(svc).Return(testToken, nil)

			rec := serve(ep.handle(h), ep.path, validBody)

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Empty(t, rec.Header().Get("Authorization"), "token must be sent only in cookie")
			assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
			assert.Empty(t, rec.Body.String())

			cookies := rec.Result().Cookies()
			require.Len(t, cookies, 1)
			c := cookies[0]
			assert.Equal(t, middleware.AuthCookieName, c.Name)
			assert.Equal(t, testToken, c.Value)
			assert.Equal(t, "/", c.Path)
			assert.True(t, c.HttpOnly)
			assert.True(t, c.Secure)
			assert.Equal(t, http.SameSiteStrictMode, c.SameSite)
		})
	}
}

func TestAuthHandler_InsecureCookie(t *testing.T) {
	h, svc, _ := newTestHandler(t, false)
	svc.EXPECT().Login(gomock.Any(), testLogin, testPassword).Return(testToken, nil)

	rec := serve(h.Login, "/api/user/login", validBody)

	require.Equal(t, http.StatusOK, rec.Code)
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	assert.False(t, cookies[0].Secure)
}

func TestAuthHandler_BadRequestBody(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "empty body", body: "", wantStatus: http.StatusBadRequest},
		{name: "invalid JSON", body: `{"login":`, wantStatus: http.StatusBadRequest},
		{name: "not an object", body: `["gopher","pass"]`, wantStatus: http.StatusBadRequest},
		{name: "wrong field type", body: `{"login":123,"password":"p"}`, wantStatus: http.StatusBadRequest},
		{name: "unknown field", body: `{"login":"gopher","passwrd":"p"}`, wantStatus: http.StatusBadRequest},
		{name: "trailing data", body: `{"login":"gopher","password":"p"} garbage`, wantStatus: http.StatusBadRequest},
		{name: "two objects", body: `{"login":"gopher","password":"p"}{"login":"other","password":"p"}`, wantStatus: http.StatusBadRequest},
		{name: "too large", body: `{"login":"` + strings.Repeat("a", maxJSONBodySize) + `"}`, wantStatus: http.StatusRequestEntityTooLarge},
	}
	for _, ep := range endpoints {
		for _, tt := range tests {
			t.Run(ep.name+"/"+tt.name, func(t *testing.T) {
				h, _, _ := newTestHandler(t, false)

				rec := serve(ep.handle(h), ep.path, tt.body)

				assert.Equal(t, tt.wantStatus, rec.Code)
				assert.Empty(t, rec.Result().Cookies())
			})
		}
	}
}

func TestAuthHandler_MissingFieldsArePassedToService(t *testing.T) {
	h, svc, _ := newTestHandler(t, false)
	svc.EXPECT().Register(gomock.Any(), "", "").
		Return("", fmt.Errorf("%w: login is empty", model.ErrInvalidInput))

	rec := serve(h.Register, "/api/user/register", `{}`)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), "login is empty")
}

func TestAuthHandler_Register_ServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid input", err: fmt.Errorf("%w: password is empty", model.ErrInvalidInput), wantStatus: http.StatusBadRequest},
		{name: "login taken", err: fmt.Errorf("create user: %w", model.ErrLoginTaken), wantStatus: http.StatusConflict},
		{name: "internal error", err: errInternal, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, svc, _ := newTestHandler(t, false)
			svc.EXPECT().Register(gomock.Any(), testLogin, testPassword).Return("", tt.err)

			rec := serve(h.Register, "/api/user/register", validBody)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Empty(t, rec.Result().Cookies())
		})
	}
}

func TestAuthHandler_Login_ServiceErrors(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
	}{
		{name: "invalid input", err: fmt.Errorf("%w: login is empty", model.ErrInvalidInput), wantStatus: http.StatusBadRequest},
		{name: "invalid credentials", err: model.ErrInvalidCredentials, wantStatus: http.StatusUnauthorized},
		{name: "internal error", err: errInternal, wantStatus: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, svc, _ := newTestHandler(t, false)
			svc.EXPECT().Login(gomock.Any(), testLogin, testPassword).Return("", tt.err)

			rec := serve(h.Login, "/api/user/login", validBody)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Empty(t, rec.Result().Cookies())
		})
	}
}
