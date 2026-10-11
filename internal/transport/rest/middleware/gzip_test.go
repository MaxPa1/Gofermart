package middleware

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func gzipBytes(t *testing.T, data string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	_, err := zw.Write([]byte(data))
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestDecompress(t *testing.T) {
	const payload = `{"login":"gopher","password":"secret"}`

	tests := []struct {
		name            string
		body            []byte
		contentEncoding string
		wantStatus      int
		wantBody        string
	}{
		{name: "gzip body", body: gzipBytes(t, payload), contentEncoding: "gzip", wantStatus: http.StatusOK, wantBody: payload},
		{name: "gzip in other case", body: gzipBytes(t, payload), contentEncoding: "GZIP", wantStatus: http.StatusOK, wantBody: payload},
		{name: "plain body", body: []byte(payload), wantStatus: http.StatusOK, wantBody: payload},
		{name: "broken gzip", body: []byte("definitely not gzip"), contentEncoding: "gzip", wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotBody string
			var gotEncoding string
			var reached bool
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				reached = true
				gotEncoding = r.Header.Get("Content-Encoding")
				b, err := io.ReadAll(r.Body)
				require.NoError(t, err)
				gotBody = string(b)
			})

			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(tt.body))
			if tt.contentEncoding != "" {
				req.Header.Set("Content-Encoding", tt.contentEncoding)
			}
			rec := httptest.NewRecorder()
			Decompress(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			if tt.wantStatus != http.StatusOK {
				assert.False(t, reached, "handler must not be called for a broken body")
				return
			}
			assert.Equal(t, tt.wantBody, gotBody)
			assert.Empty(t, gotEncoding, "handler must see an uncompressed request")
		})
	}
}

func TestDecompress_LimitAppliesToDecompressedSize(t *testing.T) {
	const limit = 1024
	big := strings.Repeat("a", 100*limit)
	compressed := gzipBytes(t, big)
	require.Less(t, len(compressed), limit)

	var readErr error
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, readErr = io.ReadAll(http.MaxBytesReader(w, r.Body, limit))
	})

	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(compressed))
	req.Header.Set("Content-Encoding", "gzip")
	Decompress(next).ServeHTTP(httptest.NewRecorder(), req)

	var maxBytesErr *http.MaxBytesError
	assert.ErrorAs(t, readErr, &maxBytesErr)
}
