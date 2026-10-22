// Package repository реализует хранилище сервиса на PostgreSQL (драйвер pgx)
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/MaxPa1/Gofermart/migrations"
)

// PoolConfig — параметры пула соединений.
// подключения.
type PoolConfig struct {
	// MaxConns — максимальное число соединений в пуле.
	MaxConns int32
	// MinConns — число соединений, которое пул держит открытыми без нагрузки.
	MinConns int32
	// MaxConnLifetime — максимальный срок жизни соединения.
	MaxConnLifetime time.Duration
	// MaxConnIdleTime — время простоя, после которого соединение закрывается.
	MaxConnIdleTime time.Duration
}

// NewPool создаёт пул соединений с PostgreSQL по строке подключения dsn и параметрам cfg и проверяет
// доступность базы запросом Ping.
func NewPool(ctx context.Context, dsn string, cfg PoolConfig) (*pgxpool.Pool, error) {
	poolCfg, err := newPoolConfig(dsn, cfg)
	if err != nil {
		return nil, err
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}

func newPoolConfig(dsn string, cfg PoolConfig) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, errors.New("parse database URI: invalid format")
	}

	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.MaxConnLifetimeJitter = cfg.MaxConnLifetime / 10

	return poolCfg, nil
}

// Migrate применяет к базе все ещё не применённые миграции из migrations.FS. Использует goose
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	db := stdlib.OpenDBFromPool(pool)
	defer db.Close()

	provider, err := goose.NewProvider(goose.DialectPostgres, db, migrations.FS)
	if err != nil {
		return fmt.Errorf("create migration provider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("apply migrations: %w", err)
	}
	return nil
}

func isUniqueViolation(err error, constraint string) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) &&
		pgErr.Code == pgerrcode.UniqueViolation &&
		pgErr.ConstraintName == constraint
}
