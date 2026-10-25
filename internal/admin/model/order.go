package model

import "time"

type Order struct {
	ID          string
	UserID      string
	Item        string
	AmountCents int64
	CreatedAt   time.Time
}
