package gateway

import (
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var (
	errUnknownRoute        = httpx.Error{Status: http.StatusNotFound, Code: "unknown_route", Message: "Unknown route"}
	errUpstreamUnavailable = httpx.Error{Status: http.StatusBadGateway, Code: "upstream_unavailable", Message: "Service is temporarily unavailable"}
)
