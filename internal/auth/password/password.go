package password

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const MaxLength = 72

var (
	ErrTooLong = errors.New("password exceeds 72 bytes")

	ErrInvalidCost = errors.New("bcrypt cost out of range")
)

type Hasher struct {
	cost int
}

func NewHasher(cost int) (*Hasher, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("%w: %d (allowed %d..%d)", ErrInvalidCost, cost, bcrypt.MinCost, bcrypt.MaxCost)
	}
	return &Hasher{cost: cost}, nil
}

func (h *Hasher) Hash(password string) (string, error) {
	if len(password) > MaxLength {
		return "", ErrTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", fmt.Errorf("generate bcrypt hash: %w", err)
	}
	return string(hash), nil
}

func (h *Hasher) Compare(hash, password string) (bool, error) {
	if len(password) > MaxLength {
		return false, nil
	}
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("compare bcrypt hash: %w", err)
	}
	return true, nil
}
