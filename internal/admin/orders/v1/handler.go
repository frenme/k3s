package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/admin/storage"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/pagination"
)

type orders interface {
	List(ctx context.Context, limit int, after *pagination.Cursor) ([]storage.Order, error)
}

type Handler struct {
	orders orders
}

func NewHandler(orders orders) *Handler {
	return &Handler{orders: orders}
}

func (h *Handler) Register(public huma.API) {
	httpx.Register(public, huma.Operation{
		OperationID: "admin-list-orders",
		Method:      http.MethodGet,
		Path:        "/v1/admin/orders",
		Summary:     "List orders from the orders schema",
		Tags:        []string{"Admin"},
	}, httpx.AccessAdmin, h.list)
}

func (h *Handler) list(ctx context.Context, in *ListRequest) (*ListOrdersResponse, error) {
	page, err := pagination.Fetch(ctx, in.Params, h.orders.List, func(o storage.Order) pagination.Cursor {
		return pagination.Cursor{CreatedAt: o.CreatedAt, ID: o.ID}
	})
	if err != nil {
		return nil, err
	}
	return &ListOrdersResponse{Body: pagination.Map(page, toOrder)}, nil
}
