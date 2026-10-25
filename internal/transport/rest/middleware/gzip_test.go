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

func TestCompress(t *testing.T) {
	const body = `{"message":"hello, gopher"}`

	tests := []struct {
		name           string
		method         string
		acceptEncoding string
		contentType    string
		status         int
		body           string
		wantGzip       bool
	}{
		{name: "json", method: http.MethodGet, acceptEncoding: "gzip", contentType: "application/json", status: http.StatusOK, body: body, wantGzip: true},
		{name: "plain text error", method: http.MethodPost, acceptEncoding: "gzip, deflate, br", contentType: "text/plain; charset=utf-8", status: http.StatusBadRequest, body: "bad request", wantGzip: true},
		{name: "sniffed content type", method: http.MethodGet, acceptEncoding: "gzip", status: http.StatusOK, body: "plain words", wantGzip: true},
		{name: "gzip with quality", method: http.MethodGet, acceptEncoding: "br;q=1.0, gzip;q=0.5", contentType: "application/json", status: http.StatusOK, body: body, wantGzip: true},
		{name: "legacy x-gzip", method: http.MethodGet, acceptEncoding: "x-gzip", contentType: "application/json", status: http.StatusOK, body: body, wantGzip: true},
		{name: "gzip forbidden by negative q", method: http.MethodGet, acceptEncoding: "gzip;q=-1", contentType: "application/json", status: http.StatusOK, body: body},
		{name: "no accept encoding", method: http.MethodGet, contentType: "application/json", status: http.StatusOK, body: body},
		{name: "gzip forbidden by q=0", method: http.MethodGet, acceptEncoding: "gzip;q=0", contentType: "application/json", status: http.StatusOK, body: body},
		{name: "other encoding only", method: http.MethodGet, acceptEncoding: "br", contentType: "application/json", status: http.StatusOK, body: body},
		{name: "binary content", method: http.MethodGet, acceptEncoding: "gzip", contentType: "image/png", status: http.StatusOK, body: "png bytes"},
		{name: "no content", method: http.MethodGet, acceptEncoding: "gzip", status: http.StatusNoContent},
		{name: "head", method: http.MethodHead, acceptEncoding: "gzip", contentType: "application/json", status: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.contentType != "" {
					w.Header().Set("Content-Type", tt.contentType)
				}
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			})

			req := httptest.NewRequest(tt.method, "/", nil)
			if tt.acceptEncoding != "" {
				req.Header.Set("Accept-Encoding", tt.acceptEncoding)
			}
			rec := httptest.NewRecorder()
			Compress(next).ServeHTTP(rec, req)

			assert.Equal(t, tt.status, rec.Code)
			assert.Equal(t, "Accept-Encoding", rec.Header().Get("Vary"))

			got := rec.Body.String()
			if tt.wantGzip {
				require.Equal(t, "gzip", rec.Header().Get("Content-Encoding"))
				assert.NotEqual(t, "application/x-gzip", rec.Header().Get("Content-Type"), "content type must describe the uncompressed body")
				zr, err := gzip.NewReader(rec.Body)
				require.NoError(t, err)
				b, err := io.ReadAll(zr)
				require.NoError(t, err)
				got = string(b)
			} else {
				assert.Empty(t, rec.Header().Get("Content-Encoding"))
			}
			assert.Equal(t, tt.body, got)
		})
	}
}

func TestCompress_StatusWithoutBody(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	})

	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	Compress(next).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusAccepted, rec.Code)
	assert.Empty(t, rec.Header().Get("Content-Encoding"))
	assert.Empty(t, rec.Body.Bytes())
}
