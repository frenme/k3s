package model

import (
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var (
	ErrOrderNotFound = httpx.Error{Status: http.StatusNotFound, Code: "order_not_found", Message: "Order not found"}
	ErrUnknownUser   = httpx.Error{Status: http.StatusBadRequest, Code: "unknown_user", Message: "User does not exist"}
)
