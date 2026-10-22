// Package auth реализует бизнес-логику регистрации и входа пользователей: проверку учётных данных,
// хэширование пароля и выпуск токена аутентификации.
package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/MaxPa1/Gofermart/internal/model"
)

//go:generate go tool mockgen -source=service.go -destination=mocks_test.go -package=auth -write_package_comment=false

const (
	// MaxLoginLength — максимальная длина логина в байтах.
	MaxLoginLength = 255

	// MaxPasswordLength — максимальная длина пароля в байтах. Ограничение bcrypt, который не учитывает
	// байты сверх 72.
	MaxPasswordLength = 72
)

const dummyPassword = "gophermart-dummy-password"

// UserRepository — хранилище пользователей, которое использует Service.
type UserRepository interface {
	// Create сохраняет нового пользователя; возвращает model.ErrLoginTaken, если логин занят.
	Create(ctx context.Context, login, passwordHash string) (model.User, error)
	// GetByLogin ищет пользователя по логину; возвращает model.ErrUserNotFound, если его нет.
	GetByLogin(ctx context.Context, login string) (model.User, error)
}

// PasswordHasher хэширует пароли и сверяет их с хэшем.
type PasswordHasher interface {
	// Hash возвращает хэш пароля.
	Hash(password string) (string, error)
	// Compare сообщает, соответствует ли пароль хэшу;
	Compare(hash, password string) (bool, error)
}

// TokenIssuer выпускает токены аутентификации.
type TokenIssuer interface {
	// Generate выпускает токен для пользователя userID.
	Generate(userID int64) (string, error)
}

// Service регистрирует пользователей и выполняет вход.
type Service struct {
	users     UserRepository
	hasher    PasswordHasher
	tokens    TokenIssuer
	dummyHash string
}

// NewService создаёт Service с хранилищем пользователей users, хэшером паролей hasher и выпуском
// токенов tokens.
func NewService(users UserRepository, hasher PasswordHasher, tokens TokenIssuer) (*Service, error) {
	dummyHash, err := hasher.Hash(dummyPassword)
	if err != nil {
		return nil, fmt.Errorf("hash dummy password: %w", err)
	}

	return &Service{
		users:     users,
		hasher:    hasher,
		tokens:    tokens,
		dummyHash: dummyHash,
	}, nil
}

// Register регистрирует пользователя с логином login и паролем password и возвращает токен
// аутентификации.
func (s *Service) Register(ctx context.Context, login, password string) (string, error) {
	if err := validateCredentials(login, password); err != nil {
		return "", err
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}

	user, err := s.users.Create(ctx, login, hash)
	if err != nil {
		return "", fmt.Errorf("create user: %w", err)
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return token, nil
}

// Login проверяет логин и пароль и возвращает токен аутентификации.
func (s *Service) Login(ctx context.Context, login, password string) (string, error) {
	if err := validateCredentials(login, password); err != nil {
		return "", err
	}

	user, err := s.users.GetByLogin(ctx, login)
	if errors.Is(err, model.ErrUserNotFound) {
		_, _ = s.hasher.Compare(s.dummyHash, password)
		return "", model.ErrInvalidCredentials
	}
	if err != nil {
		return "", fmt.Errorf("get user by login: %w", err)
	}

	match, err := s.hasher.Compare(user.PasswordHash, password)
	if err != nil {
		return "", fmt.Errorf("compare password: %w", err)
	}

	if !match {
		return "", model.ErrInvalidCredentials
	}

	token, err := s.tokens.Generate(user.ID)
	if err != nil {
		return "", fmt.Errorf("generate token: %w", err)
	}
	return token, nil
}

func validateCredentials(login, password string) error {
	switch {
	case login == "":
		return fmt.Errorf("%w: login is empty", model.ErrInvalidInput)
	case len(login) > MaxLoginLength:
		return fmt.Errorf("%w: login exceeds %d bytes", model.ErrInvalidInput, MaxLoginLength)
	case password == "":
		return fmt.Errorf("%w: password is empty", model.ErrInvalidInput)
	case len(password) > MaxPasswordLength:
		return fmt.Errorf("%w: password exceeds %d bytes", model.ErrInvalidInput, MaxPasswordLength)
	}
	return nil
}
