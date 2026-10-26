package model

import (
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var ErrConfigNotFound = httpx.Error{Status: http.StatusNotFound, Code: "config_not_found", Message: "Config not found"}
