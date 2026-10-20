package orders

import (
	"context"
	"fmt"

	"github.com/MaxPa1/Gofermart/internal/model"
	"github.com/MaxPa1/Gofermart/pkg/luhn"
)

//go:generate go tool mockgen -source=service.go -destination=mocks_test.go -package=orders -write_package_comment=false

type OrderRepository interface {
	Create(ctx context.Context, userID int64, number string) (ownerID int64, created bool, err error)
	ListByUser(ctx context.Context, userID int64) ([]model.Order, error)
}

type Service struct {
	orders OrderRepository
}

func NewService(orders OrderRepository) *Service {
	return &Service{orders: orders}
}

func (s *Service) Upload(ctx context.Context, userID int64, number string) error {
	if number == "" {
		return fmt.Errorf("%w: order number is empty", model.ErrInvalidInput)
	}
	if !luhn.Valid(number) {
		return model.ErrInvalidOrderNumber
	}

	ownerID, created, err := s.orders.Create(ctx, userID, number)
	if err != nil {
		return fmt.Errorf("upload order: %w", err)
	}

	switch {
	case created:
		return nil
	case ownerID == userID:
		return model.ErrOrderAlreadyUploaded
	default:
		return model.ErrOrderOwnedByAnother
	}
}

func (s *Service) List(ctx context.Context, userID int64) ([]model.Order, error) {
	orders, err := s.orders.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	return orders, nil
}
