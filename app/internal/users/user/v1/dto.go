package v1

import (
	"time"

	"github.com/egor/k3s-base/internal/lib/pagination"
	"github.com/egor/k3s-base/internal/users/model"
)

type User struct {
	ID        string    `json:"id" format:"uuid"`
	Email     string    `json:"email" format:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type ListUsersRequest struct {
	pagination.Params
}

type ListUsersResponse struct {
	Body pagination.Page[User]
}

type GetUserRequest struct {
	ID string `path:"id"`
}

type UserResponse struct {
	Body User
}

type CreateUserRequest struct {
	Body struct {
		Email string `json:"email" format:"email" maxLength:"254"`
		Name  string `json:"name" pattern:"\\S" maxLength:"200"`
	}
}

func toUser(u model.User) User {
	return User{ID: u.ID, Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt}
}
