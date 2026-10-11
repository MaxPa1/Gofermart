package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func newObservedLogger() (*zap.Logger, *observer.ObservedLogs) {
	core, logs := observer.New(zap.DebugLevel)
	return zap.New(core), logs
}

func TestLogger(t *testing.T) {
	tests := []struct {
		name       string
		handler    http.HandlerFunc
		wantStatus int64
		wantSize   int64
	}{
		{
			name: "explicit status and body",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusAccepted)
				_, _ = w.Write([]byte("hello"))
			},
			wantStatus: http.StatusAccepted,
			wantSize:   5,
		},
		{
			name:       "implicit 200",
			handler:    func(w http.ResponseWriter, r *http.Request) {},
			wantStatus: http.StatusOK,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, logs := newObservedLogger()
			h := chimw.RequestID(Logger(logger)(tt.handler))

			req := httptest.NewRequest(http.MethodPost, "/api/user/login", nil)
			h.ServeHTTP(httptest.NewRecorder(), req)

			entries := logs.All()
			require.Len(t, entries, 1)
			assert.Equal(t, zap.InfoLevel, entries[0].Level)

			fields := entries[0].ContextMap()
			assert.Equal(t, http.MethodPost, fields["method"])
			assert.Equal(t, "/api/user/login", fields["path"])
			assert.Equal(t, tt.wantStatus, fields["status"])
			assert.Equal(t, tt.wantSize, fields["size"])
			assert.NotEmpty(t, fields["request_id"])
			assert.Contains(t, fields, "duration")
		})
	}
}

func TestRecoverer(t *testing.T) {
	logger, logs := newObservedLogger()
	h := Recoverer(logger)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	rec := httptest.NewRecorder()
	require.NotPanics(t, func() {
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	})

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	entries := logs.All()
	require.Len(t, entries, 1)
	assert.Equal(t, zap.ErrorLevel, entries[0].Level)
	fields := entries[0].ContextMap()
	assert.Equal(t, "something went wrong", fields["panic"])
	assert.NotEmpty(t, fields["stack"])
}
