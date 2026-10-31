package httpx

import (
	"context"

	"github.com/danielgtaylor/huma/v2"
)

const (
	UserIDHeader    = "X-User-ID"
	UserRolesHeader = "X-User-Roles"
)

type userKey struct{}

func identify(ctx huma.Context, next func(huma.Context)) {
	if id := ctx.Header(UserIDHeader); id != "" {
		ctx = huma.WithValue(ctx, userKey{}, id)
	}
	next(ctx)
}

func CurrentUser(ctx context.Context) (string, error) {
	id, ok := ctx.Value(userKey{}).(string)
	if !ok {
		return "", ErrUnauthorized
	}
	return id, nil
}
