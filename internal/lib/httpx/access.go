package httpx

import (
	"context"
	"net/http"
	"slices"
	"time"

	"github.com/danielgtaylor/huma/v2"
)

type Access string

const (
	AccessPublic Access = "public"
	AccessUser   Access = "user"
	AccessAdmin  Access = "admin"

	AccessExtension   = "x-access"
	TimeoutExtension  = "x-timeout"
	MaxBodyExtension  = "x-max-body"
	CacheTTLExtension = "x-cache-ttl"
)

type RouteOption func(extensions map[string]any)

func WithTimeout(d time.Duration) RouteOption {
	return func(e map[string]any) { e[TimeoutExtension] = d.String() }
}

func WithMaxBody(bytes int64) RouteOption {
	return func(e map[string]any) { e[MaxBodyExtension] = bytes }
}

func WithCache(ttl time.Duration) RouteOption {
	return func(e map[string]any) { e[CacheTTLExtension] = ttl.String() }
}

func Register[I, O any](api huma.API, op huma.Operation, access Access, handler func(context.Context, *I) (*O, error), opts ...RouteOption) {
	extensions := make(map[string]any, len(op.Extensions)+4)
	for k, v := range op.Extensions {
		extensions[k] = v
	}
	extensions[AccessExtension] = string(access)
	for _, opt := range opts {
		opt(extensions)
	}
	op.Extensions = extensions
	switch access {
	case AccessUser:
		op.Security = []map[string][]string{{"bearer": {}}}
		op.Errors = withStatus(op.Errors, http.StatusUnauthorized)
	case AccessAdmin:
		op.Security = []map[string][]string{{"bearer": {}}}
		op.Errors = withStatus(withStatus(op.Errors, http.StatusUnauthorized), http.StatusForbidden)
	}
	huma.Register(api, op, handler)
}

func withStatus(statuses []int, status int) []int {
	if slices.Contains(statuses, status) {
		return statuses
	}
	return append(slices.Clone(statuses), status)
}

func securitySchemes() map[string]*huma.SecurityScheme {
	return map[string]*huma.SecurityScheme{
		"bearer": {Type: "http", Scheme: "bearer", BearerFormat: "JWT"},
	}
}
