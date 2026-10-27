package v1

import (
	"context"
	"log/slog"
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
	log   *slog.Logger
}

func NewHandler(users users, log *slog.Logger) *Handler {
	return &Handler{users: users, log: log}
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
	after, err := pagination.Decode(in.Cursor)
	if err != nil {
		return nil, httpx.Fail(ctx, h.log, err)
	}
	found, err := h.users.List(ctx, in.Limit, after)
	if err != nil {
		return nil, httpx.Fail(ctx, h.log, err)
	}
	page := pagination.NewPage(found, in.Limit, func(u storage.User) pagination.Cursor {
		return pagination.Cursor{CreatedAt: u.CreatedAt, ID: u.ID}
	})
	out := &ListUsersResponse{}
	out.Body.Items = make([]User, 0, len(page.Items))
	for _, u := range page.Items {
		out.Body.Items = append(out.Body.Items, toUser(u))
	}
	out.Body.NextCursor = page.Next
	return out, nil
}
