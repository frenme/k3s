package v1

import (
	"context"
	"net/http"

	"github.com/danielgtaylor/huma/v2"
	"github.com/egor/k3s-base/internal/lib/httpx"
	"github.com/egor/k3s-base/internal/users/role"
)

type Handler struct {
	svc *role.Service
}

func NewHandler(svc *role.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Register(public, internal huma.API) {
	httpx.Register(public, huma.Operation{
		OperationID: "list-user-roles",
		Method:      http.MethodGet,
		Path:        "/v1/users/{id}/roles",
		Summary:     "Roles of a user, every user has the user role",
		Tags:        []string{"Roles"},
		Errors:      []int{http.StatusNotFound},
	}, httpx.AccessAdmin, h.list)
	httpx.Register(public, huma.Operation{
		OperationID: "grant-user-role",
		Method:      http.MethodPut,
		Path:        "/v1/users/{id}/roles/{role}",
		Summary:     "Grant a role to a user",
		Tags:        []string{"Roles"},
		Errors:      []int{http.StatusNotFound},
	}, httpx.AccessAdmin, h.grant)
	httpx.Register(public, huma.Operation{
		OperationID: "revoke-user-role",
		Method:      http.MethodDelete,
		Path:        "/v1/users/{id}/roles/{role}",
		Summary:     "Revoke a role from a user",
		Tags:        []string{"Roles"},
		Errors:      []int{http.StatusNotFound},
	}, httpx.AccessAdmin, h.revoke)
	httpx.RegisterInternal(internal, huma.Operation{
		OperationID: "internal-list-user-roles",
		Method:      http.MethodGet,
		Path:        "/internal/v1/users/{id}/roles",
		Summary:     "Roles of a user for other services",
		Tags:        []string{"Internal"},
		Errors:      []int{http.StatusNotFound},
	}, h.list)
}

func (h *Handler) list(ctx context.Context, in *GetRolesRequest) (*RolesResponse, error) {
	roles, err := h.svc.Roles(ctx, in.ID)
	if err != nil {
		return nil, err
	}
	return toRoles(roles), nil
}

func (h *Handler) grant(ctx context.Context, in *RoleRequest) (*RolesResponse, error) {
	roles, err := h.svc.Grant(ctx, in.ID, in.Role)
	if err != nil {
		return nil, err
	}
	return toRoles(roles), nil
}

func (h *Handler) revoke(ctx context.Context, in *RoleRequest) (*RolesResponse, error) {
	roles, err := h.svc.Revoke(ctx, in.ID, in.Role)
	if err != nil {
		return nil, err
	}
	return toRoles(roles), nil
}
