package model

import "errors"

var (
	ErrLoginTaken = errors.New("login already taken")

	ErrInvalidCredentials = errors.New("invalid login or password")

	ErrUserNotFound = errors.New("user not found")

	ErrInvalidInput = errors.New("invalid input")
)
