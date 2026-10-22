// Package orders реализует бизнес-логику заказов: загрузку номера заказа для начисления баллов с
// проверкой алгоритмом Луна и получение списка заказов пользователя.
package orders

import (
	"context"
	"fmt"

	"github.com/MaxPa1/Gofermart/internal/model"
	"github.com/MaxPa1/Gofermart/pkg/luhn"
)

//go:generate go tool mockgen -source=service.go -destination=mocks_test.go -package=orders -write_package_comment=false

// OrderRepository — хранилище заказов, которое использует Service.
type OrderRepository interface {
	// Create сохраняет заказ, если его ещё нет, и возвращает его владельца; created сообщает, был ли заказ
	// создан этим вызовом.
	Create(ctx context.Context, userID int64, number string) (ownerID int64, created bool, err error)
	// ListByUser возвращает заказы пользователя от новых к старым.
	ListByUser(ctx context.Context, userID int64) ([]model.Order, error)
}

// Service управляет заказами пользователей.
// использования.
type Service struct {
	orders OrderRepository
}

func NewService(orders OrderRepository) *Service {
	return &Service{orders: orders}
}

// Upload загружает заказ number пользователя userID для расчёта начисления; новый заказ получает
// статус NEW.
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

// List возвращает заказы пользователя userID от новых к старым
func (s *Service) List(ctx context.Context, userID int64) ([]model.Order, error) {
	orders, err := s.orders.ListByUser(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("list orders: %w", err)
	}
	return orders, nil
}
