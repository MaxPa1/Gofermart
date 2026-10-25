package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// Decompress распаковывает тело запросов с заголовком Content-Encoding: gzip
func Decompress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.EqualFold(strings.TrimSpace(r.Header.Get("Content-Encoding")), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			http.Error(w, "invalid gzip body", http.StatusBadRequest)
			return
		}
		defer gz.Close()

		r.Body = gz
		r.Header.Del("Content-Encoding")
		r.Header.Del("Content-Length")
		r.ContentLength = -1

		next.ServeHTTP(w, r)
	})
}

var gzipWriters = sync.Pool{
	New: func() any { return gzip.NewWriter(io.Discard) },
}

// Compress сжимает тело ответа gzip, если клиент указал gzip в заголовке Accept-Encoding
func Compress(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if r.Method == http.MethodHead || !acceptsGzip(r.Header.Values("Accept-Encoding")) {
			next.ServeHTTP(w, r)
			return
		}

		gw := &gzipResponseWriter{ResponseWriter: w}
		defer gw.close()
		next.ServeHTTP(gw, r)
	})
}

type gzipResponseWriter struct {
	http.ResponseWriter
	status  int
	started bool
	gz      *gzip.Writer
}

func (w *gzipResponseWriter) WriteHeader(code int) {
	if code < http.StatusOK {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	if w.status == 0 {
		w.status = code
	}
}

func (w *gzipResponseWriter) Write(p []byte) (int, error) {
	if !w.started {
		if len(p) == 0 {
			return 0, nil
		}
		w.start(p)
	}
	if w.gz != nil {
		return w.gz.Write(p)
	}
	return w.ResponseWriter.Write(p)
}

func (w *gzipResponseWriter) start(p []byte) {
	w.started = true
	if w.status == 0 {
		w.status = http.StatusOK
	}

	h := w.Header()
	if h.Get("Content-Type") == "" {
		h.Set("Content-Type", http.DetectContentType(p))
	}
	if w.status != http.StatusNoContent && w.status != http.StatusNotModified &&
		h.Get("Content-Encoding") == "" && compressible(h.Get("Content-Type")) {
		h.Set("Content-Encoding", "gzip")
		h.Del("Content-Length")
		w.gz = gzipWriters.Get().(*gzip.Writer)
		w.gz.Reset(w.ResponseWriter)
	}
	w.ResponseWriter.WriteHeader(w.status)
}

func (w *gzipResponseWriter) close() {
	if !w.started {
		if w.status != 0 {
			w.ResponseWriter.WriteHeader(w.status)
		}
		return
	}
	if w.gz == nil {
		return
	}

	_ = w.gz.Close()
	w.gz.Reset(io.Discard)
	gzipWriters.Put(w.gz)
	w.gz = nil
}

func acceptsGzip(values []string) bool {
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			coding, params, _ := strings.Cut(part, ";")
			coding = strings.TrimSpace(coding)
			if strings.EqualFold(coding, "gzip") || strings.EqualFold(coding, "x-gzip") {
				return !refused(params)
			}
		}
	}
	return false
}

func refused(params string) bool {
	for param := range strings.SplitSeq(params, ";") {
		key, value, ok := strings.Cut(strings.TrimSpace(param), "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "q") {
			q, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			return err == nil && q <= 0
		}
	}
	return false
}

func compressible(contentType string) bool {
	mediaType, _, _ := strings.Cut(contentType, ";")
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	return strings.HasPrefix(mediaType, "text/") || mediaType == "application/json"
}
