package v1

import "github.com/egor/k3s-base/internal/users/model"

type RolesBody struct {
	Roles []string `json:"roles"`
}

type RolesResponse struct {
	Body RolesBody
}

type GetRolesRequest struct {
	ID string `path:"id"`
}

type RoleRequest struct {
	ID   string `path:"id"`
	Role string `path:"role" enum:"user,admin"`
}

func toRoles(roles []model.Role) *RolesResponse {
	out := &RolesResponse{Body: RolesBody{Roles: make([]string, 0, len(roles))}}
	for _, r := range roles {
		out.Body.Roles = append(out.Body.Roles, string(r))
	}
	return out
}
