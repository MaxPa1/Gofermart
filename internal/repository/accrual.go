package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaxPa1/Gofermart/internal/model"
)

type AccrualRepository struct {
	pool *pgxpool.Pool
}

func NewAccrualRepository(pool *pgxpool.Pool) *AccrualRepository {
	return &AccrualRepository{pool: pool}
}

func (r *AccrualRepository) TakeOrdersToCheck(ctx context.Context, limit int) ([]model.Order, error) {
	const query = `
		UPDATE orders
		SET checked_at = now()
		WHERE number IN (
			SELECT number
			FROM orders
			WHERE status IN ('NEW', 'PROCESSING')
			ORDER BY checked_at NULLS FIRST, uploaded_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING number, user_id, status, (accrual * 100)::bigint, uploaded_at`

	rows, err := r.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("take orders to check: %w", err)
	}

	orders, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Order, error) {
		var (
			o       model.Order
			status  string
			accrual int64
		)
		if err := row.Scan(&o.Number, &o.UserID, &status, &accrual, &o.UploadedAt); err != nil {
			return model.Order{}, err
		}
		o.Status = model.OrderStatus(status)
		o.Accrual = model.Amount(accrual)
		return o, nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan orders to check: %w", err)
	}
	return orders, nil
}

func (r *AccrualRepository) ApplyAccrual(
	ctx context.Context,
	number string,
	status model.OrderStatus,
	accrual model.Amount,
) (applied bool, err error) {
	const query = `
		WITH updated AS (
			UPDATE orders
			SET status = $2, accrual = $3::numeric / 100
			WHERE number = $1 AND status IN ('NEW', 'PROCESSING')
			RETURNING user_id, accrual
		),
		credited AS (
			INSERT INTO balances (user_id, current)
			SELECT user_id, accrual
			FROM updated
			WHERE accrual > 0
			ON CONFLICT (user_id) DO UPDATE
			SET current = balances.current + EXCLUDED.current
		)
		SELECT count(*) FROM updated`

	var n int
	if err := r.pool.QueryRow(ctx, query, number, string(status), int64(accrual)).Scan(&n); err != nil {
		return false, fmt.Errorf("apply accrual: %w", err)
	}
	return n > 0, nil
}
