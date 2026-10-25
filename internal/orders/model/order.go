package model

import (
	"errors"
	"time"
)

var (
	ErrNotFound    = errors.New("order not found")
	ErrUnknownUser = errors.New("user does not exist")
)

type ValidationError string

func (e ValidationError) Error() string { return string(e) }

type Order struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id" db:"user_id"`
	Item        string    `json:"item"`
	AmountCents int64     `json:"amount_cents" db:"amount_cents"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}
