// Package model содержит доменные сущности системы лояльности (пользователь, заказ, баланс, списание),
// денежный тип Amount и доменные sentinel-ошибки.
//
// Модели не зависят от транспорта и хранилища и не содержат JSON- и DB-тегов: форматы API описывают
// DTO транспортного слоя, а маппинг строк базы данных — репозитории.
package model

import "errors"

var (
	// ErrLoginTaken возвращается при регистрации, если логин уже занят.
	ErrLoginTaken = errors.New("login already taken")

	// ErrInvalidCredentials возвращается при входе, если пользователя с таким логином нет или пароль
	// неверен.
	ErrInvalidCredentials = errors.New("invalid login or password")

	// ErrUserNotFound возвращается хранилищем, если пользователь не найден.
	ErrUserNotFound = errors.New("user not found")

	// ErrInvalidInput — базовая ошибка валидации входных данных.
	ErrInvalidInput = errors.New("invalid input")

	// ErrInvalidOrderNumber возвращается, если номер заказа не проходит проверку алгоритмом Луна.
	ErrInvalidOrderNumber = errors.New("invalid order number")

	// ErrOrderAlreadyUploaded возвращается при повторной загрузке заказа тем же пользователем.
	ErrOrderAlreadyUploaded = errors.New("order already uploaded by this user")

	// ErrOrderOwnedByAnother возвращается, если заказ с таким номером уже загружен другим пользователем.
	ErrOrderOwnedByAnother = errors.New("order already uploaded by another user")

	// ErrInsufficientFunds возвращается при списании, если на балансе недостаточно баллов.
	ErrInsufficientFunds = errors.New("insufficient funds")
)
