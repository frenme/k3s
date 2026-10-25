package v1

import (
	"time"

	"github.com/egor/k3s-base/internal/users/model"
)

type User struct {
	ID        string    `json:"id" format:"uuid"`
	Email     string    `json:"email" format:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func toUser(u model.User) User {
	return User{ID: u.ID, Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt}
}

type ListUsersRequest struct {
	Limit int `query:"limit" default:"50" minimum:"1" maximum:"500"`
}

type ListUsersResponse struct {
	Body struct {
		Items []User `json:"items"`
	}
}

type GetUserRequest struct {
	ID string `path:"id"`
}

type UserResponse struct {
	Body User
}

type CreateUserRequest struct {
	Body struct {
		Email string `json:"email" maxLength:"254"`
		Name  string `json:"name" minLength:"1" maxLength:"200"`
	}
}
