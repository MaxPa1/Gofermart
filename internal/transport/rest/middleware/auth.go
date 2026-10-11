package middleware

import (
	"context"
	"net/http"
)

//go:generate go tool mockgen -source=auth.go -destination=mocks_test.go -package=middleware -write_package_comment=false

const AuthCookieName = "auth_token"

type TokenParser interface {
	Parse(token string) (int64, error)
}

type userIDKey struct{}

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

func WithUserID(ctx context.Context, userID int64) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

func UserIDFromContext(ctx context.Context) (userID int64, ok bool) {
	userID, ok = ctx.Value(userIDKey{}).(int64)
	return userID, ok
}

func unauthorized(w http.ResponseWriter) {
	http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
}
