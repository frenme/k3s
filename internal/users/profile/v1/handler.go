package v1

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/users/profile"
)

type Handler struct {
	svc *profile.Service
	log *slog.Logger
}

func NewHandler(svc *profile.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log}
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
		Summary:     "Get a user",
		Tags:        []string{"Users"},
		Errors:      []int{http.StatusNotFound},
	}, httpx.AccessUser, h.get)
	huma.Register(internal, huma.Operation{
		OperationID: "internal-get-user",
		Method:      http.MethodGet,
		Path:        "/internal/v1/users/{id}",
		Summary:     "Get a user for other services",
		Tags:        []string{"Internal"},
		Errors:      []int{http.StatusNotFound},
	}, h.get)
}

func (h *Handler) list(ctx context.Context, in *ListUsersRequest) (*ListUsersResponse, error) {
	page, err := h.svc.List(ctx, in.Limit, in.Cursor)
	if err != nil {
		return nil, h.fail(ctx, err)
	}
	out := &ListUsersResponse{}
	out.Body.Items = make([]User, 0, len(page.Items))
	for _, u := range page.Items {
		out.Body.Items = append(out.Body.Items, toUser(u))
	}
	out.Body.NextCursor = page.Next
	return out, nil
}

func (h *Handler) get(ctx context.Context, in *GetUserRequest) (*UserResponse, error) {
	u, err := h.svc.Get(ctx, in.ID)
	if err != nil {
		return nil, h.fail(ctx, err)
	}
	return &UserResponse{Body: toUser(u)}, nil
}

func (h *Handler) create(ctx context.Context, in *CreateUserRequest) (*UserResponse, error) {
	u, err := h.svc.Create(ctx, in.Body.Email, in.Body.Name)
	if err != nil {
		return nil, h.fail(ctx, err)
	}
	return &UserResponse{Body: toUser(u)}, nil
}

func (h *Handler) fail(ctx context.Context, err error) error {
	return httpx.Fail(ctx, h.log, err)
}
