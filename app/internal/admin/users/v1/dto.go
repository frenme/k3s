package v1

import (
	"time"

	"github.com/egor/k3s-base/internal/admin/storage"
	"github.com/egor/k3s-base/internal/lib/pagination"
)

type User struct {
	ID        string    `json:"id" format:"uuid"`
	Email     string    `json:"email" format:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type ListRequest struct {
	pagination.Params
}

type ListUsersResponse struct {
	Body pagination.Page[User]
}

func toUser(u storage.User) User {
	return User{ID: u.ID, Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt}
}
