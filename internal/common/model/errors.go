package model

import (
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var ErrSettingNotFound = httpx.Error{Status: http.StatusNotFound, Code: "setting_not_found", Message: "Setting not found"}
