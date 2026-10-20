package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	testToken  = "header.payload.signature"
	testUserID = int64(42)
)

func protectedHandler(gotUserID *int64, reached *bool) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*reached = true
		*gotUserID, _ = UserIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
}

func TestAuth(t *testing.T) {
	tests := []struct {
		name       string
		cookie     *http.Cookie
		setup      func(p *MockTokenParser)
		wantStatus int
		wantUserID int64
	}{
		{
			name:   "valid token",
			cookie: &http.Cookie{Name: AuthCookieName, Value: testToken},
			setup: func(p *MockTokenParser) {
				p.EXPECT().Parse(testToken).Return(testUserID, nil)
			},
			wantStatus: http.StatusOK,
			wantUserID: testUserID,
		},
		{
			name:   "invalid token",
			cookie: &http.Cookie{Name: AuthCookieName, Value: testToken},
			setup: func(p *MockTokenParser) {
				p.EXPECT().Parse(testToken).Return(int64(0), errors.New("invalid token"))
			},
			wantStatus: http.StatusUnauthorized,
		},
		{name: "no cookie", wantStatus: http.StatusUnauthorized},
		{name: "empty cookie", cookie: &http.Cookie{Name: AuthCookieName, Value: ""}, wantStatus: http.StatusUnauthorized},
		{name: "other cookie", cookie: &http.Cookie{Name: "session", Value: testToken}, wantStatus: http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewMockTokenParser(gomock.NewController(t))
			if tt.setup != nil {
				tt.setup(parser)
			}

			var gotUserID int64
			var reached bool
			h := Auth(parser)(protectedHandler(&gotUserID, &reached))

			req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
			if tt.cookie != nil {
				req.AddCookie(tt.cookie)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			assert.Equal(t, tt.wantStatus, rec.Code)
			assert.Equal(t, tt.wantStatus == http.StatusOK, reached, "handler must be reached only with a valid token")
			assert.Equal(t, tt.wantUserID, gotUserID)
		})
	}
}

func TestAuth_BearerHeaderIsIgnored(t *testing.T) {
	parser := NewMockTokenParser(gomock.NewController(t))
	var gotUserID int64
	var reached bool
	h := Auth(parser)(protectedHandler(&gotUserID, &reached))

	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	req.Header.Set("Authorization", "Bearer "+testToken)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.False(t, reached)
}

func TestUserIDFromContext(t *testing.T) {
	userID, ok := UserIDFromContext(WithUserID(context.Background(), testUserID))
	require.True(t, ok)
	assert.Equal(t, testUserID, userID)

	userID, ok = UserIDFromContext(context.Background())
	assert.False(t, ok)
	assert.Zero(t, userID)
}
