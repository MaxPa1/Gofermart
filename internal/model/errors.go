package model

import "errors"

var (
	ErrLoginTaken = errors.New("login already taken")

	ErrInvalidCredentials = errors.New("invalid login or password")

	ErrUserNotFound = errors.New("user not found")

	ErrInvalidInput = errors.New("invalid input")

	ErrInvalidOrderNumber = errors.New("invalid order number")

	ErrOrderAlreadyUploaded = errors.New("order already uploaded by this user")

	ErrOrderOwnedByAnother = errors.New("order already uploaded by another user")

	ErrInsufficientFunds = errors.New("insufficient funds")
)
