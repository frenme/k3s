package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/users/user"
)

type Handler struct {
	svc *user.Service
}

func NewHandler(svc *user.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(public, internal huma.API) {
	httpx.Register(public, huma.Operation{
		OperationID: "list-users",
		Method:      http.MethodGet,
		Path:        "/v1/users",
		Summary:     "List users",
		Tags:        []string{"Users"},
	}, httpx.AccessAdmin, h.list)
	httpx.Register(public, huma.Operation{
		OperationID:   "create-user",
		Method:        http.MethodPost,
		Path:          "/v1/users",
		Summary:       "Create a user",
		Tags:          []string{"Users"},
		DefaultStatus: http.StatusCreated,
		Errors:        []int{http.StatusForbidden, http.StatusConflict, http.StatusBadGateway},
	}, httpx.AccessPublic, h.create)
	httpx.Register(public, huma.Operation{
		OperationID: "get-user",
		Method:      http.MethodGet,
		Path:        "/v1/users/{id}",
		Summary:     "Get your own user, any other id answers 404",
		Tags:        []string{"Users"},
		Errors:      []int{http.StatusNotFound},
	}, httpx.AccessUser, h.getSelf)
	httpx.RegisterInternal(internal, huma.Operation{
		OperationID: "internal-get-user",
		Method:      http.MethodGet,
		Path:        "/internal/v1/users/{id}",
		Summary:     "Get a user for other services",
		Tags:        []string{"Internal"},
		Errors:      []int{http.StatusNotFound},
	}, h.get)
}

func (h *Handler) list(ctx context.Context, in *ListUsersRequest) (*ListUsersResponse, error) {
	page, err := h.svc.List(ctx, in.Params)
	if err != nil {
		return nil, err
	}
	return &ListUsersResponse{Body: pagination.Map(page, toUser)}, nil
}

func (h *Handler) getSelf(ctx context.Context, in *GetUserRequest) (*UserResponse, error) {
	viewer, err := httpx.CurrentUser(ctx)
	if err != nil {
		return nil, err
	}
	u, err := h.svc.GetSelf(ctx, viewer, in.ID)
	if err != nil {
		return nil, err
	}
	return &UserResponse{Body: toUser(u)}, nil
}

func (h *Handler) get(ctx context.Context, in *GetUserRequest) (*UserResponse, error) {
	u, err := h.svc.Get(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return &UserResponse{Body: toUser(u)}, nil
}

func (h *Handler) create(ctx context.Context, in *CreateUserRequest) (*UserResponse, error) {
	u, err := h.svc.Create(ctx, in.Body.Email, in.Body.Name)
	if err != nil {
		return nil, err
	}
	return &UserResponse{Body: toUser(u)}, nil
}
