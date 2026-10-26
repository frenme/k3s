package v1

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/idempotency"
	"github.com/egor/k3s-base/internal/orders/checkout"
	"github.com/egor/k3s-base/internal/orders/model"
)

type Handler struct {
	svc *checkout.Service
	log *slog.Logger
}

func NewHandler(svc *checkout.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
}

func (h *Handler) Register(public huma.API) {
	httpx.Register(public, huma.Operation{
		OperationID: "list-orders",
		Method:      http.MethodGet,
		Path:        "/v1/orders",
		Summary:     "List orders",
		Tags:        []string{"Orders"},
	}, httpx.AccessUser, h.list)
	httpx.Register(public, huma.Operation{
		OperationID:   "create-order",
		Method:        http.MethodPost,
		Path:          "/v1/orders",
		Summary:       "Create an order, replays return the first order with 200",
		Tags:          []string{"Orders"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusBadGateway},
	}, httpx.AccessUser, h.create)
	create := public.OpenAPI().Paths["/v1/orders"].Post
	create.Responses["200"] = &huma.Response{
		Description: "Replay of an earlier order with the same Idempotency-Key",
		Content:     create.Responses["201"].Content,
	}
	httpx.Register(public, huma.Operation{
		OperationID: "get-order",
		Method:      http.MethodGet,
		Path:        "/v1/orders/{id}",
		Summary:     "Get an order",
		Tags:        []string{"Orders"},
		Errors:      []int{http.StatusNotFound},
	}, httpx.AccessUser, h.get)
}

func (h *Handler) list(ctx context.Context, in *ListOrdersRequest) (*ListOrdersResponse, error) {
	page, err := h.svc.List(ctx, in.UserID, in.Limit, in.Cursor)
	if err != nil {
		return nil, h.fail(ctx, err)
	}
	out := &ListOrdersResponse{}
	out.Body.Items = make([]Order, 0, len(page.Items))
	for _, o := range page.Items {
		out.Body.Items = append(out.Body.Items, toOrder(o))
	}
	out.Body.NextCursor = page.Next
	return out, nil
}

func (h *Handler) get(ctx context.Context, in *GetOrderRequest) (*OrderResponse, error) {
	o, err := h.svc.Get(ctx, in.ID)
	if err != nil {
		return nil, h.fail(ctx, err)
	}
	return &OrderResponse{Status: http.StatusOK, Body: toOrder(o)}, nil
}

func (h *Handler) create(ctx context.Context, in *CreateOrderRequest) (*OrderResponse, error) {
	idem, err := idempotency.New(in.Body.UserID, in.IdempotencyKey, in.Body)
	if err != nil {
		return nil, h.fail(ctx, err)
	}
	o, created, err := h.svc.Create(ctx, model.Order{
		UserID:      in.Body.UserID,
		Item:        in.Body.Item,
		AmountCents: in.Body.AmountCents,
	}, idem)
	if err != nil {
		return nil, h.fail(ctx, err)
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	return &OrderResponse{Status: status, Body: toOrder(o)}, nil
}

func (h *Handler) fail(ctx context.Context, err error) error {
	return httpx.Fail(ctx, h.log, err)
}
