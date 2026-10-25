package model

import (
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var (
	ErrNotFound  = httpx.Error{Status: http.StatusNotFound, Code: "user_not_found", Message: "User not found"}
	ErrDuplicate = httpx.Error{Status: http.StatusConflict, Code: "email_taken", Message: "Email is already registered"}
)
