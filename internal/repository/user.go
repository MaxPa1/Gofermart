package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MaxPa1/Gofermart/internal/model"
)

const usersLoginKey = "users_login_key"

// UserRepository хранит учётные записи пользователей.
type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// Create сохраняет пользователя с логином login и хэшем пароля passwordHash и возвращает его с
// присвоенными ID и CreatedAt. Возвращает model.ErrLoginTaken, если логин уже занят.
func (r *UserRepository) Create(ctx context.Context, login, passwordHash string) (model.User, error) {
	const query = `
		INSERT INTO users (login, password_hash)
		VALUES ($1, $2)
		RETURNING id, created_at`

	user := model.User{Login: login, PasswordHash: passwordHash}
	err := r.pool.QueryRow(ctx, query, login, passwordHash).Scan(&user.ID, &user.CreatedAt)
	if err != nil {
		if isUniqueViolation(err, usersLoginKey) {
			return model.User{}, model.ErrLoginTaken
		}
		return model.User{}, fmt.Errorf("insert user: %w", err)
	}
	return user, nil
}

// GetByLogin возвращает пользователя с логином login или model.ErrUserNotFound, если такого нет.
func (r *UserRepository) GetByLogin(ctx context.Context, login string) (model.User, error) {
	const query = `
		SELECT id, login, password_hash, created_at
		FROM users
		WHERE login = $1`

	var user model.User
	err := r.pool.QueryRow(ctx, query, login).Scan(&user.ID, &user.Login, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return model.User{}, model.ErrUserNotFound
		}
		return model.User{}, fmt.Errorf("select user by login: %w", err)
	}
	return user, nil
}
