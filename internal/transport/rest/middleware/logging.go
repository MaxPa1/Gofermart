package middleware

import (
	"errors"
	"net/http"
	"runtime/debug"
	"time"

	"go.uber.org/zap"
)

// Logger возвращает middleware, которое пишет в logger одну запись уровня Info на каждый запрос:
// request ID, метод, путь, код и размер ответа, длительность обработки.
func Logger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			rec := &responseRecorder{ResponseWriter: w}

			next.ServeHTTP(rec, r)

			status := rec.status
			if status == 0 {
				status = http.StatusOK
			}

			logger.Info("http request",
				zap.String("request_id", RequestIDFromContext(r.Context())),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", status),
				zap.Int("size", rec.size),
				zap.Duration("duration", time.Since(start)),
			)
		})
	}
}

type responseRecorder struct {
	http.ResponseWriter
	status int
	size   int
}

func (r *responseRecorder) WriteHeader(code int) {
	if r.status == 0 && code >= http.StatusOK {
		r.status = code
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *responseRecorder) Write(p []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(p)
	r.size += n
	return n, err
}

func (r *responseRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}

// Recoverer возвращает middleware, которое перехватывает панику в обработчике, пишет её вместе со
// стеком вызовов в logger на уровне Error и отвечает 500 Internal Server Error.
func Recoverer(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}

				if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(rec)
				}

				logger.Error("panic recovered",
					zap.String("request_id", RequestIDFromContext(r.Context())),
					zap.String("method", r.Method),
					zap.String("path", r.URL.Path),
					zap.Any("panic", rec),
					zap.ByteString("stack", debug.Stack()),
				)
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			}()

			next.ServeHTTP(w, r)
		})
	}
}
