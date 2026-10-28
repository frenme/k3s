package model

import (
	"net/http"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var (
	ErrUnknownRoute     = httpx.Error{Status: http.StatusNotFound, Code: "unknown_route", Message: "Unknown route"}
	ErrForbidden        = httpx.Error{Status: http.StatusForbidden, Code: "forbidden", Message: "Not enough permissions"}
	ErrAuthUnavailable  = httpx.Error{Status: http.StatusServiceUnavailable, Code: "auth_unavailable", Message: "Authorization is temporarily unavailable"}
	ErrPayloadTooLarge  = httpx.Error{Status: http.StatusRequestEntityTooLarge, Code: "payload_too_large", Message: "Request body is too large"}
	ErrUpstreamTimeout  = httpx.Error{Status: http.StatusGatewayTimeout, Code: "upstream_timeout", Message: "Service did not answer in time"}
	ErrUpstreamOpen     = httpx.Error{Status: http.StatusServiceUnavailable, Code: "upstream_circuit_open", Message: "Service is temporarily unavailable"}
	ErrUpstreamOverload = httpx.Error{Status: http.StatusServiceUnavailable, Code: "upstream_overloaded", Message: "Service is overloaded"}
)
