package model

import (
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var (
	ErrUserNotFound       = httpx.Error{Status: http.StatusNotFound, Code: "user_not_found", Message: "User not found"}
	ErrEmailTaken         = httpx.Error{Status: http.StatusConflict, Code: "email_taken", Message: "Email is already registered"}
	ErrRegistrationClosed = httpx.Error{Status: http.StatusForbidden, Code: "registration_closed", Message: "Registration is closed"}
	ErrInvalidRole        = httpx.Error{Status: http.StatusBadRequest, Code: "invalid_role", Message: "Unknown role"}
)
