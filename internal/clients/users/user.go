package users

import "errors"

var ErrNotFound = errors.New("user not found")

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}
