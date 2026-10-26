package httpx

import (
	"context"
	"net/http"
	"slices"

	"github.com/danielgtaylor/huma/v2"
)

type Access string

const (
	AccessPublic Access = "public"
	AccessUser   Access = "user"
	AccessAdmin  Access = "admin"

	AccessExtension = "x-access"
)

func Register[I, O any](api huma.API, op huma.Operation, access Access, handler func(context.Context, *I) (*O, error)) {
	extensions := make(map[string]any, len(op.Extensions)+1)
	for k, v := range op.Extensions {
		extensions[k] = v
	}
	extensions[AccessExtension] = string(access)
	op.Extensions = extensions
	switch access {
	case AccessUser:
		op.Security = []map[string][]string{{"bearer": {}}}
		op.Errors = withStatus(op.Errors, http.StatusUnauthorized)
	case AccessAdmin:
		op.Security = []map[string][]string{{"basic": {}}}
		op.Errors = withStatus(op.Errors, http.StatusUnauthorized)
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
		"basic":  {Type: "http", Scheme: "basic"},
	}
}
