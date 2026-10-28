package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/admin/storage"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/pagination"
)

type users interface {
	List(ctx context.Context, limit int, after *pagination.Cursor) ([]storage.User, error)
}

type Handler struct {
	users users
}

func NewHandler(users users) *Handler {
	return &Handler{users: users}
}

func (h *Handler) Register(public huma.API) {
	httpx.Register(public, huma.Operation{
		OperationID: "admin-list-users",
		Method:      http.MethodGet,
		Path:        "/v1/admin/users",
		Summary:     "List users from the users schema",
		Tags:        []string{"Admin"},
	}, httpx.AccessAdmin, h.list)
}

func (h *Handler) list(ctx context.Context, in *ListRequest) (*ListUsersResponse, error) {
	page, err := pagination.Fetch(ctx, in.Params, h.users.List, func(u storage.User) pagination.Cursor {
		return pagination.Cursor{CreatedAt: u.CreatedAt, ID: u.ID}
	})
	if err != nil {
		return nil, err
	}
	return &ListUsersResponse{Body: pagination.Map(page, toUser)}, nil
}
