// Package password хэширует пароли и сверяет их с хэшем алгоритмом bcrypt.
package password

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// MaxLength — максимальная длина пароля в байтах, которую поддерживает bcrypt.
const MaxLength = 72

var (
	// ErrTooLong возвращается Hasher.Hash, если пароль длиннее MaxLength байт.
	ErrTooLong = errors.New("password exceeds 72 bytes")

	// ErrInvalidCost возвращается NewHasher, если Cost вне диапазона bcrypt.MinCost..bcrypt.MaxCost.
	ErrInvalidCost = errors.New("bcrypt cost out of range")
)

// Hasher хэширует и проверяет пароли алгоритмом bcrypt с фиксированной Сost
type Hasher struct {
	cost int
}

// NewHasher создаёт Hasher со стоимостью cost. Возвращает
// ошибку ErrInvalidCost, если cost вне диапазона bcrypt.MinCost..bcrypt.MaxCost.
func NewHasher(cost int) (*Hasher, error) {
	if cost < bcrypt.MinCost || cost > bcrypt.MaxCost {
		return nil, fmt.Errorf("%w: %d (allowed %d..%d)", ErrInvalidCost, cost, bcrypt.MinCost, bcrypt.MaxCost)
	}
	return &Hasher{cost: cost}, nil
}

// Hash возвращает bcrypt-хэш пароля со случайной солью. Пароль длиннее MaxLength байт отклоняется с
// ErrTooLong.
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

// Compare сообщает, соответствует ли password хэшу hash.
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
