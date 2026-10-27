package v1

import (
	"time"

	"github.com/egor/k3s-base/internal/admin/storage"
)

type User struct {
	ID        string    `json:"id" format:"uuid"`
	Email     string    `json:"email" format:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

func toUser(u storage.User) User {
	return User{ID: u.ID, Email: u.Email, Name: u.Name, CreatedAt: u.CreatedAt}
}

type ListRequest struct {
	Limit  int    `query:"limit" default:"50" minimum:"1" maximum:"500"`
	Cursor string `query:"cursor" maxLength:"200"`
}

type ListUsersResponse struct {
	Body struct {
		Items      []User `json:"items"`
		NextCursor string `json:"next_cursor,omitempty"`
	}
}
