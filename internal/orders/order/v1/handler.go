package v1

import (
	"context"
	"net/http"
	"time"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/idempotency"
	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/orders/model"
	"github.com/egor/k3s-base/internal/orders/order"
)

type Handler struct {
	svc *order.Service
}

func NewHandler(svc *order.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(public huma.API) {
	httpx.Register(public, huma.Operation{
		OperationID: "list-orders",
		Method:      http.MethodGet,
		Path:        "/v1/orders",
		Summary:     "List your orders",
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
	}, httpx.AccessUser, h.create, httpx.WithTimeout(5*time.Second), httpx.WithMaxBody(64<<10))
	create := public.OpenAPI().Paths["/v1/orders"].Post
	create.Responses["200"] = &huma.Response{
		Description: "Replay of an earlier order with the same Idempotency-Key",
		Content:     create.Responses["201"].Content,
	}
	httpx.Register(public, huma.Operation{
		OperationID: "get-order",
		Method:      http.MethodGet,
		Path:        "/v1/orders/{id}",
		Summary:     "Get your own order, an order of someone else answers 404",
		Tags:        []string{"Orders"},
		Errors:      []int{http.StatusNotFound},
	}, httpx.AccessUser, h.get)
}

func (h *Handler) list(ctx context.Context, in *ListOrdersRequest) (*ListOrdersResponse, error) {
	viewer, err := httpx.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	page, err := h.svc.List(ctx, viewer, in.Params)
	if err != nil {
		return nil, err
	}
	return &ListOrdersResponse{Body: pagination.Map(page, toOrder)}, nil
}

func (h *Handler) get(ctx context.Context, in *GetOrderRequest) (*OrderResponse, error) {
	viewer, err := httpx.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	o, err := h.svc.Get(ctx, viewer, in.ID)
	if err != nil {
		return nil, err
	}
	return &OrderResponse{Status: http.StatusOK, Body: toOrder(o)}, nil
}

func (h *Handler) create(ctx context.Context, in *CreateOrderRequest) (*OrderResponse, error) {
	viewer, err := httpx.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	idem, err := idempotency.New(viewer, in.IdempotencyKey, in.Body)
	if err != nil {
		return nil, err
	}
	o, created, err := h.svc.Create(ctx, model.Order{
		UserID:      viewer,
		Item:        in.Body.Item,
		AmountCents: in.Body.AmountCents,
	}, idem)
	if err != nil {
		return nil, err
	}
	status := http.StatusCreated
	if !created {
		status = http.StatusOK
	}
	return &OrderResponse{Status: status, Body: toOrder(o)}, nil
}
