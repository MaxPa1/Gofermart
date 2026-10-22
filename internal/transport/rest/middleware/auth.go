// Package middleware содержит HTTP-middleware сервиса: аутентификацию по cookie с JWT, распаковку
// gzip-запросов, журналирование запросов и перехват паник.
package middleware

import (
	"context"
	"net/http"
)

//go:generate go tool mockgen -source=auth.go -destination=mocks_test.go -package=middleware -write_package_comment=false

// AuthCookieName — имя cookie, в которой клиент передаёт токен аутентификации.
const AuthCookieName = "auth_token"

// TokenParser проверяет токен аутентификации и извлекает из него идентификатор пользователя.
type TokenParser interface {
	// Parse возвращает идентификатор пользователя или ошибку, если токен недействителен.
	Parse(token string) (int64, error)
}

type userIDKey struct{}

// Auth возвращает middleware, пропускающее только аутентифицированные запросы: токен берётся из cookie
// AuthCookieName и проверяется через tokens. При успехе идентификатор пользователя сохраняется в
// контексте запроса, иначе клиент получает 401 Unauthorized
func Auth(tokens TokenParser) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(AuthCookieName)
			if err != nil || cookie.Value == "" {
				unauthorized(w)
				return
			}

			userID, err := tokens.Parse(cookie.Value)
			if err != nil {
				unauthorized(w)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithUserID(r.Context(), userID)))
		})
	}
}

// WithUserID возвращает копию ctx с идентификатором пользователя userID, который затем читает
// UserIDFromContext.
func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

// UserIDFromContext возвращает идентификатор пользователя, сохранённый в ctx через WithUserID
func UserIDFromContext(ctx context.Context) (userID int64, ok bool) {
	userID, ok = ctx.Value(userIDKey{}).(int64)
	return userID, ok
}

func unauthorized(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}
