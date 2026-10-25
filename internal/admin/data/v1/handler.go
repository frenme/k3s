package v1

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/admin/data"
	"github.com/egor/k3s-base/internal/lib/httpx"
)

type Handler struct {
	svc *data.Service
	log *slog.Logger
}

func NewHandler(svc *data.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Register(public huma.API) {
	huma.Register(public, huma.Operation{
		OperationID: "admin-list-users",
		Method:      http.MethodGet,
		Path:        "/v1/admin/users",
		Summary:     "List users from the users schema",
		Tags:        []string{"Admin"},
	}, h.users)
	huma.Register(public, huma.Operation{
		OperationID: "admin-list-orders",
		Method:      http.MethodGet,
		Path:        "/v1/admin/orders",
		Summary:     "List orders from the orders schema",
		Tags:        []string{"Admin"},
	}, h.orders)
}

func (h *Handler) users(ctx context.Context, in *ListRequest) (*ListUsersResponse, error) {
	users, err := h.svc.Users(ctx, in.Limit)
	if err != nil {
		return nil, httpx.Fail(ctx, h.log, err)
	}
	out := &ListUsersResponse{}
	out.Body.Items = make([]User, 0, len(users))
	for _, u := range users {
		out.Body.Items = append(out.Body.Items, toUser(u))
	}
	return out, nil
}

func (h *Handler) orders(ctx context.Context, in *ListRequest) (*ListOrdersResponse, error) {
	orders, err := h.svc.Orders(ctx, in.Limit)
	if err != nil {
		return nil, httpx.Fail(ctx, h.log, err)
	}
	out := &ListOrdersResponse{}
	out.Body.Items = make([]Order, 0, len(orders))
	for _, o := range orders {
		out.Body.Items = append(out.Body.Items, toOrder(o))
	}
	return out, nil
}
