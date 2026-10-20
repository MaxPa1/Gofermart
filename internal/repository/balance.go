package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaxPa1/Gofermart/internal/model"
)

type BalanceRepository struct {
	pool *pgxpool.Pool
}

func NewBalanceRepository(pool *pgxpool.Pool) *BalanceRepository {
	return &BalanceRepository{pool: pool}
}

func (r *BalanceRepository) Get(ctx context.Context, userID int64) (model.Balance, error) {
	const query = `
		SELECT (current * 100)::bigint, (withdrawn * 100)::bigint
		FROM balances
		WHERE user_id = $1`

	var current, withdrawn int64
	err := r.pool.QueryRow(ctx, query, userID).Scan(&current, &withdrawn)
	if errors.Is(err, pgx.ErrNoRows) {
		return model.Balance{}, nil
	}
	if err != nil {
		return model.Balance{}, fmt.Errorf("select balance: %w", err)
	}
	return model.Balance{Current: model.Amount(current), Withdrawn: model.Amount(withdrawn)}, nil
}

func (r *BalanceRepository) Withdraw(ctx context.Context, userID int64, orderNumber string, sum model.Amount) error {
	const query = `
		WITH debited AS (
			UPDATE balances
			SET current   = current - $3::numeric / 100,
			    withdrawn = withdrawn + $3::numeric / 100
			WHERE user_id = $1 AND current >= $3::numeric / 100
			RETURNING user_id
		)
		INSERT INTO withdrawals (user_id, order_number, sum)
		SELECT user_id, $2, $3::numeric / 100
		FROM debited`

	tag, err := r.pool.Exec(ctx, query, userID, orderNumber, int64(sum))
	if err != nil {
		return fmt.Errorf("withdraw: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return model.ErrInsufficientFunds
	}
	return nil
}

func (r *BalanceRepository) ListWithdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	const query = `
		SELECT order_number, (sum * 100)::bigint, processed_at
		FROM withdrawals
		WHERE user_id = $1
		ORDER BY processed_at DESC, id DESC`

	rows, err := r.pool.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("select withdrawals: %w", err)
	}

	withdrawals, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (model.Withdrawal, error) {
		var (
			w   model.Withdrawal
			sum int64
		)
		if err := row.Scan(&w.OrderNumber, &sum, &w.ProcessedAt); err != nil {
			return model.Withdrawal{}, err
		}
		w.UserID = userID
		w.Sum = model.Amount(sum)
		return w, nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan withdrawals: %w", err)
	}
	return withdrawals, nil
}
