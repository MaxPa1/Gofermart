package rest

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"github.com/MaxPa1/Gofermart/internal/model"
	"github.com/MaxPa1/Gofermart/internal/transport/rest/handler"
	"github.com/MaxPa1/Gofermart/internal/transport/rest/middleware"
)

const testToken = "header.payload.signature"

type fakeAuthService struct{}

func (fakeAuthService) Register(_ context.Context, login, password string) (string, error) {
	if login == "" {
		return "", fmt.Errorf("%w: login is empty", model.ErrInvalidInput)
	}
	return testToken, nil
}

func (fakeAuthService) Login(_ context.Context, login, password string) (string, error) {
	return testToken, nil
}

type fakeTokenParser struct{}

func (fakeTokenParser) Parse(string) (int64, error) { return 1, nil }

func newTestRouter() http.Handler {
	auth := handler.NewAuthHandler(fakeAuthService{}, zap.NewNop(), false)
	return NewRouter(zap.NewNop(), fakeTokenParser{}, auth)
}

func TestRouter_Routes(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       string
		wantStatus int
	}{
		{name: "register", method: http.MethodPost, path: "/api/user/register", body: `{"login":"a","password":"b"}`, wantStatus: http.StatusOK},
		{name: "login", method: http.MethodPost, path: "/api/user/login", body: `{"login":"a","password":"b"}`, wantStatus: http.StatusOK},
		{name: "wrong method", method: http.MethodGet, path: "/api/user/register", wantStatus: http.StatusMethodNotAllowed},
		{name: "unknown path", method: http.MethodPost, path: "/api/user/unknown", wantStatus: http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			newTestRouter().ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantStatus == http.StatusOK {
				cookies := rec.Result().Cookies()
				require.Len(t, cookies, 1)
				assert.Equal(t, middleware.AuthCookieName, cookies[0].Name)
				assert.Equal(t, testToken, cookies[0].Value)
			}
		})
	}
}

func TestRouter_GzipRequest(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte(`{"login":"a","password":"b"}`))
	require.NoError(t, err)
	require.NoError(t, zw.Close())

	req := httptest.NewRequest(http.MethodPost, "/api/user/register", &buf)
	req.Header.Set("Content-Encoding", "gzip")
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRouter_GzipResponse(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(`{"login":"","password":"b"}`))
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	newTestRouter().ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))

	zr, err := gzip.NewReader(rec.Body)
	require.NoError(t, err)
	body, err := io.ReadAll(zr)
	require.NoError(t, err)
	assert.Contains(t, string(body), "login is empty")
}
