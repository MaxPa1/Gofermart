package model

import "time"

// Balance — баланс баллов пользователя.
type Balance struct {
	// Current — сумма баллов, доступная для списания.
	Current Amount
	// Withdrawn — сумма баллов, списанных за всё время.
	Withdrawn Amount
}

// Withdrawal — списание баллов пользователя в счёт оплаты заказа.
type Withdrawal struct {
	// UserID — идентификатор пользователя, списавшего баллы.
	UserID int64
	// OrderNumber — номер заказа, в счёт оплаты которого списаны баллы.
	OrderNumber string
	// Sum — списанная сумма; всегда положительная.
	Sum Amount
	// ProcessedAt — время списания.
	ProcessedAt time.Time
}
