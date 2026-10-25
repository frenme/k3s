package model

import (
	"errors"
	"time"
)

var (
	ErrNotFound  = errors.New("user not found")
	ErrDuplicate = errors.New("email already exists")
)

type ValidationError string

func (e ValidationError) Error() string { return string(e) }

type User struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
