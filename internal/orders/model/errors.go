package model

import (
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var ErrNotFound = httpx.Error{Status: http.StatusNotFound, Code: "order_not_found", Message: "Order not found"}
