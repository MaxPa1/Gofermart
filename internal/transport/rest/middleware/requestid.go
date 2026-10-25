package middleware

import (
	"context"
	"crypto/rand"
	"net/http"
)

// RequestIDHeader — заголовок, в котором клиент может передать идентификатор запроса.
const RequestIDHeader = "X-Request-Id"

const maxRequestIDLen = 64

type requestIDKey struct{}

// RequestID сохраняет в контексте запроса его идентификатор, который затем читает RequestIDFromContext.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(RequestIDHeader)
		if !validRequestID(id) {
			id = rand.Text()
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey{}, id)))
	})
}

// RequestIDFromContext возвращает идентификатор запроса, сохранённый RequestID, или пустую строку
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

func validRequestID(id string) bool {
	if id == "" || len(id) > maxRequestIDLen {
		return false
	}
	for i := 0; i < len(id); i++ {
		if id[i] <= ' ' || id[i] > '~' {
			return false
		}
	}
	return true
}
