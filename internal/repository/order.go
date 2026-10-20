package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaxPa1/Gofermart/internal/model"
)

type OrderRepository struct {
	pool *pgxpool.Pool
}

func NewOrderRepository(pool *pgxpool.Pool) *OrderRepository {
	return &OrderRepository{pool: pool}
}

func (r *OrderRepository) Create(ctx context.Context, userID int64, number string) (ownerID int64, created bool, err error) {
	const insertQuery = `
		INSERT INTO orders (number, user_id)
		VALUES ($1, $2)
		ON CONFLICT (number) DO NOTHING`

	tag, err := r.pool.Exec(ctx, insertQuery, number, userID)
	if err != nil {
		return 0, false, fmt.Errorf("insert order: %w", err)
	}
	if tag.RowsAffected() == 1 {
		return userID, true, nil
	}

	const ownerQuery = `
		SELECT user_id
		FROM orders
		WHERE number = $1`

	if err := r.pool.QueryRow(ctx, ownerQuery, number).Scan(&ownerID); err != nil {
		return 0, false, fmt.Errorf("select order owner: %w", err)
	}
	return ownerID, false, nil
}

func (r *OrderRepository) ListByUser(ctx context.Context, userID int64) ([]model.Order, error) {
	const query = `
		SELECT number, status, (accrual * 100)::bigint, uploaded_at
		FROM orders
		WHERE user_id = $1
		ORDER BY uploaded_at DESC, number DESC`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("select orders: %w", err)
	}

	orders, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Order, error) {
		var (
			o       model.Order
			status  string
			accrual int64
		)
		if err := row.Scan(&o.Number, &status, &accrual, &o.UploadedAt); err != nil {
			return model.Order{}, err
		}
		o.UserID = userID
		o.Status = model.OrderStatus(status)
		o.Accrual = model.Amount(accrual)
		return o, nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan orders: %w", err)
	}
	return orders, nil
}
