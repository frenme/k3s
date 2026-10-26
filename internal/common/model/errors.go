package model

import (
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var ErrAppConfigNotFound = httpx.Error{Status: http.StatusNotFound, Code: "app_config_not_found", Message: "AppConfig not found"}
