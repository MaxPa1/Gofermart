package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestID(t *testing.T) {
	tests := []struct {
		name     string
		header   string
		wantSame bool
	}{
		{name: "from header", header: "abc-123", wantSame: true},
		{name: "max length", header: strings.Repeat("a", maxRequestIDLen), wantSame: true},
		{name: "no header"},
		{name: "too long", header: strings.Repeat("a", maxRequestIDLen+1)},
		{name: "with space", header: "abc 123"},
		{name: "with control char", header: "abc\x01"},
		{name: "non-ASCII", header: "запрос"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = RequestIDFromContext(r.Context())
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.header != "" {
				req.Header.Set(RequestIDHeader, tt.header)
			}
			RequestID(next).ServeHTTP(httptest.NewRecorder(), req)

			if tt.wantSame {
				assert.Equal(t, tt.header, got)
				return
			}
			assert.NotEqual(t, tt.header, got)
			assert.True(t, validRequestID(got), "generated id must be valid: %q", got)
		})
	}
}

func TestRequestID_GeneratesUniqueIDs(t *testing.T) {
	ids := make(map[string]struct{})
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ids[RequestIDFromContext(r.Context())] = struct{}{}
	})
	for range 100 {
		RequestID(next).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	}
	assert.Len(t, ids, 100)
}

func TestRequestIDFromContext_Empty(t *testing.T) {
	assert.Empty(t, RequestIDFromContext(t.Context()))
}
