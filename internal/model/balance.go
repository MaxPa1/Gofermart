package model

import "time"

type Balance struct {
	Current   Amount
	Withdrawn Amount
}

type Withdrawal struct {
	UserID      int64
	OrderNumber string
	Sum         Amount
	ProcessedAt time.Time
}
