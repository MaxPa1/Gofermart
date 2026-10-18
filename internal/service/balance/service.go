package balance

import (
	"context"
	"fmt"

	"github.com/MaxPa1/Gofermart/internal/model"
	"github.com/MaxPa1/Gofermart/pkg/luhn"
)

//go:generate go tool mockgen -source=service.go -destination=mocks_test.go -package=balance -write_package_comment=false

type Repository interface {
	Get(ctx context.Context, userID int64) (model.Balance, error)
	Withdraw(ctx context.Context, userID int64, orderNumber string, sum model.Amount) error
	ListWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Balance(ctx context.Context, userID int64) (model.Balance, error) {
	b, err := s.repo.Get(ctx, userID)
	if err != nil {
		return model.Balance{}, fmt.Errorf("get balance: %w", err)
	}
	return b, nil
}

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

func (s *Service) Withdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	withdrawals, err := s.repo.ListWithdrawals(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list withdrawals: %w", err)
	}
	return withdrawals, nil
}
