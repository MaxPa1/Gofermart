package token

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const MinSecretLength = 32

const issuer = "gophermart"

var (
	ErrInvalidToken = errors.New("invalid token")

	ErrShortSecret = errors.New("jwt secret is too short")

	ErrInvalidTTL = errors.New("token ttl must be positive")

	ErrInvalidUserID = errors.New("user id must be positive")
)

type Manager struct {
	secret []byte
	ttl    time.Duration
	parser *jwt.Parser
}

func NewManager(secret []byte, ttl time.Duration) (*Manager, error) {
	if len(secret) < MinSecretLength {
		return nil, fmt.Errorf("%w: got %d bytes, need at least %d", ErrShortSecret, len(secret), MinSecretLength)
	}
	if ttl <= 0 {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTTL, ttl)
	}

	secretCopy := make([]byte, len(secret))
	copy(secretCopy, secret)

	parser := jwt.NewParser(
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithIssuedAt(),
		jwt.WithIssuer(issuer),
	)

	return &Manager{
		secret: secretCopy,
		ttl:    ttl,
		parser: parser,
	}, nil
}

func (m *Manager) Generate(userID int64) (string, error) {
	if userID <= 0 {
		return "", fmt.Errorf("%w: %d", ErrInvalidUserID, userID)
	}

	now := time.Now()

	claims := jwt.RegisteredClaims{
		Subject:   strconv.FormatInt(userID, 10),
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
	}

	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}

func (m *Manager) Parse(token string) (int64, error) {
	var claims jwt.RegisteredClaims

	_, err := m.parser.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) {
		return m.secret, nil
	})
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: parse subject: %w", ErrInvalidToken, err)
	}
	if userID <= 0 {
		return 0, fmt.Errorf("%w: non-positive subject %d", ErrInvalidToken, userID)
	}

	return userID, nil
}
