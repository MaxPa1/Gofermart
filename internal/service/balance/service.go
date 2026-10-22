// Package balance реализует бизнес-логику баланса баллов: получение текущего баланса, списание баллов
// в счёт оплаты заказа и историю списаний.
package balance

import (
	"context"
	"fmt"

	"github.com/MaxPa1/Gofermart/internal/model"
	"github.com/MaxPa1/Gofermart/pkg/luhn"
)

//go:generate go tool mockgen -source=service.go -destination=mocks_test.go -package=balance -write_package_comment=false

// Repository — хранилище балансов и списаний, которое использует Service.
type Repository interface {
	// Get возвращает баланс пользователя; пользователь без начислений получает нулевой баланс.
	Get(ctx context.Context, userID int64) (model.Balance, error)
	// Withdraw атомарно списывает sum с баланса и записывает списание; возвращает
	// model.ErrInsufficientFunds, если баллов недостаточно.
	Withdraw(ctx context.Context, userID int64, orderNumber string, sum model.Amount) error
	// ListWithdrawals возвращает списания пользователя от новых к старым.
	ListWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error)
}

// Service управляет балансом баллов пользователей.
type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// Balance возвращает текущий баланс пользователя userID и сумму его списаний за всё время.
func (s *Service) Balance(ctx context.Context, userID int64) (model.Balance, error) {
	b, err := s.repo.Get(ctx, userID)
	if err != nil {
		return model.Balance{}, fmt.Errorf("get balance: %w", err)
	}
	return b, nil
}

// Withdraw списывает sum баллов с баланса пользователя userID в счёт оплаты заказа orderNumber.
func (s *Service) Withdraw(ctx context.Context, userID int64, orderNumber string, sum model.Amount) error {
	if !luhn.Valid(orderNumber) {
		return model.ErrInvalidOrderNumber
	}
	if sum <= 0 {
		return fmt.Errorf("%w: sum must be positive", model.ErrInvalidInput)
	}

	if err := s.repo.Withdraw(ctx, userID, orderNumber, sum); err != nil {
		return fmt.Errorf("withdraw: %w", err)
	}
	return nil
}

// Withdrawals возвращает историю списаний пользователя userID от новых к старым
func (s *Service) Withdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	withdrawals, err := s.repo.ListWithdrawals(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}
	return withdrawals, nil
}
