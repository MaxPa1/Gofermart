package model

import "time"

// OrderStatus — статус обработки заказа в системе лояльности. Жизненный цикл: OrderStatusNew →
// OrderStatusProcessing → OrderStatusInvalid или OrderStatusProcessed.
type OrderStatus string

const (
	// OrderStatusNew — заказ загружен, но система расчёта начислений ещё не начала его обработку.
	OrderStatusNew OrderStatus = "NEW"
	// OrderStatusProcessing — начисление по заказу рассчитывается.
	OrderStatusProcessing OrderStatus = "PROCESSING"
	// OrderStatusInvalid — система расчёта отказала в начислении. Статус окончательный.
	OrderStatusInvalid OrderStatus = "INVALID"
	// OrderStatusProcessed — расчёт завершён, начисление зачислено на баланс. Статус окончательный.
	OrderStatusProcessed OrderStatus = "PROCESSED"
)

// Order — заказ, загруженный пользователем для начисления баллов.
type Order struct {
	// Number — номер заказа: строка цифр.
	Number string
	// UserID — идентификатор пользователя, загрузившего заказ.
	UserID int64
	// Status — текущий статус обработки.
	Status OrderStatus
	// Accrual — начисленные баллы
	Accrual Amount
	// UploadedAt — время загрузки заказа.
	UploadedAt time.Time
}
