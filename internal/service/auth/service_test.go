package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/MaxPa1/Gofermart/internal/model"
)

const (
	testLogin    = "gopher"
	testPassword = "s3cret-Passw0rd"
	testHash     = "$2a$04$hash-of-test-password"
	testDummy    = "$2a$04$hash-of-dummy-password"
	testToken    = "header.payload.signature"
	testUserID   = int64(42)
)

var errInternal = errors.New("boom")

type deps struct {
	users  *MockUserRepository
	hasher *MockPasswordHasher
	tokens *MockTokenIssuer
}

func newTestService(t *testing.T) (*Service, deps) {
	t.Helper()
	ctrl := gomock.NewController(t)
	d := deps{
		users:  NewMockUserRepository(ctrl),
		hasher: NewMockPasswordHasher(ctrl),
		tokens: NewMockTokenIssuer(ctrl),
	}
	d.hasher.EXPECT().Hash(dummyPassword).Return(testDummy, nil)

	svc, err := NewService(d.users, d.hasher, d.tokens)
	require.NoError(t, err)
	return svc, d
}

func TestNewService_DummyHashError(t *testing.T) {
	ctrl := gomock.NewController(t)
	hasher := NewMockPasswordHasher(ctrl)
	hasher.EXPECT().Hash(dummyPassword).Return("", errInternal)

	svc, err := NewService(NewMockUserRepository(ctrl), hasher, NewMockTokenIssuer(ctrl))
	require.ErrorIs(t, err, errInternal)
	assert.Nil(t, svc)
}

var invalidCredentials = []struct {
	name     string
	login    string
	password string
}{
	{name: "empty login", login: "", password: testPassword},
	{name: "too long login", login: strings.Repeat("l", MaxLoginLength+1), password: testPassword},
	{name: "empty password", login: testLogin, password: ""},
	{name: "too long password", login: testLogin, password: strings.Repeat("p", MaxPasswordLength+1)},
	{name: "both empty", login: "", password: ""},
}

func TestService_Register(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(d deps)
		wantToken string
		wantErr   error
	}{
		{
			name: "success",
			setup: func(d deps) {
				d.hasher.EXPECT().Hash(testPassword).Return(testHash, nil)
				d.users.EXPECT().Create(gomock.Any(), testLogin, testHash).
					Return(model.User{ID: testUserID, Login: testLogin, PasswordHash: testHash}, nil)
				d.tokens.EXPECT().Generate(testUserID).Return(testToken, nil)
			},
			wantToken: testToken,
		},
		{
			name: "login taken",
			setup: func(d deps) {
				d.hasher.EXPECT().Hash(testPassword).Return(testHash, nil)
				d.users.EXPECT().Create(gomock.Any(), testLogin, testHash).Return(model.User{}, model.ErrLoginTaken)
			},
			wantErr: model.ErrLoginTaken,
		},
		{
			name: "hash error",
			setup: func(d deps) {
				d.hasher.EXPECT().Hash(testPassword).Return("", errInternal)
			},
			wantErr: errInternal,
		},
		{
			name: "repository error",
			setup: func(d deps) {
				d.hasher.EXPECT().Hash(testPassword).Return(testHash, nil)
				d.users.EXPECT().Create(gomock.Any(), testLogin, testHash).Return(model.User{}, errInternal)
			},
			wantErr: errInternal,
		},
		{
			name: "token error",
			setup: func(d deps) {
				d.hasher.EXPECT().Hash(testPassword).Return(testHash, nil)
				d.users.EXPECT().Create(gomock.Any(), testLogin, testHash).
					Return(model.User{ID: testUserID, Login: testLogin, PasswordHash: testHash}, nil)
				d.tokens.EXPECT().Generate(testUserID).Return("", errInternal)
			},
			wantErr: errInternal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, d := newTestService(t)
			tt.setup(d)

			token, err := svc.Register(context.Background(), testLogin, testPassword)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, token)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

func TestService_Register_InvalidInput(t *testing.T) {
	for _, tt := range invalidCredentials {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService(t)

			token, err := svc.Register(context.Background(), tt.login, tt.password)
			require.ErrorIs(t, err, model.ErrInvalidInput)
			assert.Empty(t, token)
		})
	}
}

func TestService_Register_MaxLengthAccepted(t *testing.T) {
	svc, d := newTestService(t)
	login := strings.Repeat("l", MaxLoginLength)
	password := strings.Repeat("p", MaxPasswordLength)

	d.hasher.EXPECT().Hash(password).Return(testHash, nil)
	d.users.EXPECT().Create(gomock.Any(), login, testHash).Return(model.User{ID: testUserID}, nil)
	d.tokens.EXPECT().Generate(testUserID).Return(testToken, nil)

	token, err := svc.Register(context.Background(), login, password)
	require.NoError(t, err)
	assert.Equal(t, testToken, token)
}

func TestService_Login(t *testing.T) {
	storedUser := model.User{ID: testUserID, Login: testLogin, PasswordHash: testHash}

	tests := []struct {
		name      string
		setup     func(d deps)
		wantToken string
		wantErr   error
	}{
		{
			name: "success",
			setup: func(d deps) {
				d.users.EXPECT().GetByLogin(gomock.Any(), testLogin).Return(storedUser, nil)
				d.hasher.EXPECT().Compare(testHash, testPassword).Return(true, nil)
				d.tokens.EXPECT().Generate(testUserID).Return(testToken, nil)
			},
			wantToken: testToken,
		},
		{
			name: "wrong password",
			setup: func(d deps) {
				d.users.EXPECT().GetByLogin(gomock.Any(), testLogin).Return(storedUser, nil)
				d.hasher.EXPECT().Compare(testHash, testPassword).Return(false, nil)
			},
			wantErr: model.ErrInvalidCredentials,
		},
		{
			name: "unknown login compares with dummy hash",
			setup: func(d deps) {
				d.users.EXPECT().GetByLogin(gomock.Any(), testLogin).Return(model.User{}, model.ErrUserNotFound)
				d.hasher.EXPECT().Compare(testDummy, testPassword).Return(false, nil)
			},
			wantErr: model.ErrInvalidCredentials,
		},
		{
			name: "repository error",
			setup: func(d deps) {
				d.users.EXPECT().GetByLogin(gomock.Any(), testLogin).Return(model.User{}, errInternal)
			},
			wantErr: errInternal,
		},
		{
			name: "compare error",
			setup: func(d deps) {
				d.users.EXPECT().GetByLogin(gomock.Any(), testLogin).Return(storedUser, nil)
				d.hasher.EXPECT().Compare(testHash, testPassword).Return(false, errInternal)
			},
			wantErr: errInternal,
		},
		{
			name: "token error",
			setup: func(d deps) {
				d.users.EXPECT().GetByLogin(gomock.Any(), testLogin).Return(storedUser, nil)
				d.hasher.EXPECT().Compare(testHash, testPassword).Return(true, nil)
				d.tokens.EXPECT().Generate(testUserID).Return("", errInternal)
			},
			wantErr: errInternal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, d := newTestService(t)
			tt.setup(d)

			token, err := svc.Login(context.Background(), testLogin, testPassword)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				assert.Empty(t, token)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantToken, token)
		})
	}
}

func TestService_Login_InvalidInput(t *testing.T) {
	for _, tt := range invalidCredentials {
		t.Run(tt.name, func(t *testing.T) {
			svc, _ := newTestService(t)

			token, err := svc.Login(context.Background(), tt.login, tt.password)
			require.ErrorIs(t, err, model.ErrInvalidInput)
			assert.Empty(t, token)
		})
	}
}

func TestService_Login_InternalErrorIsNotInvalidCredentials(t *testing.T) {
	svc, d := newTestService(t)
	d.users.EXPECT().GetByLogin(gomock.Any(), testLogin).Return(model.User{}, errInternal)

	_, err := svc.Login(context.Background(), testLogin, testPassword)
	require.Error(t, err)
	assert.NotErrorIs(t, err, model.ErrInvalidCredentials)
}
