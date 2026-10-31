package users

import (
	"context"
	"net/url"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var ErrNotFound = httpx.Error{Code: "user_not_found"}

type User struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type roles struct {
	Roles []string `json:"roles"`
}

type Client struct {
	api *httpx.ServiceClient
}

func New(baseURL string) *Client {
	return &Client{api: httpx.NewServiceClient("users", baseURL, 3*time.Second)}
}

func (c *Client) Get(ctx context.Context, id string) (User, error) {
	return httpx.Get[User](ctx, c.api, "/internal/v1/users/"+url.PathEscape(id))
}

func (c *Client) Roles(ctx context.Context, id string) ([]string, error) {
	out, err := httpx.Get[roles](ctx, c.api, "/internal/v1/users/"+url.PathEscape(id)+"/roles")
	return out.Roles, err
}
