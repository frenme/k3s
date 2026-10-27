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

type orders interface {
	List(ctx context.Context, limit int, after *pagination.Cursor) ([]storage.Order, error)
}

type Handler struct {
	orders orders
	log    *slog.Logger
}

func NewHandler(orders orders, log *slog.Logger) *Handler {
	return &Handler{orders: orders, log: log}
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
	after, err := pagination.Decode(in.Cursor)
	if err != nil {
		return nil, httpx.Fail(ctx, h.log, err)
	}
	found, err := h.orders.List(ctx, in.Limit, after)
	if err != nil {
		return nil, httpx.Fail(ctx, h.log, err)
	}
	page := pagination.NewPage(found, in.Limit, func(o storage.Order) pagination.Cursor {
		return pagination.Cursor{CreatedAt: o.CreatedAt, ID: o.ID}
	})
	out := &ListOrdersResponse{}
	out.Body.Items = make([]Order, 0, len(page.Items))
	for _, o := range page.Items {
		out.Body.Items = append(out.Body.Items, toOrder(o))
	}
	out.Body.NextCursor = page.Next
	return out, nil
}
