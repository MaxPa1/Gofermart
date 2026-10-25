package model

import "time"

// User — зарегистрированный пользователь.
type User struct {
	// ID — идентификатор пользователя
	ID int64
	// Login — уникальный логин.
	Login string
	// PasswordHash — bcrypt-хэш пароля
	PasswordHash string
	// CreatedAt — время регистрации.
	CreatedAt time.Time
}
