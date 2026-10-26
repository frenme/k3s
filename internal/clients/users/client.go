package users

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    httpx.NewClient("users", 3*time.Second),
	}
}

func (c *Client) Get(ctx context.Context, id string) (User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/users/"+url.PathEscape(id), nil)
	if err != nil {
		return User{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		var u User
		return u, json.NewDecoder(resp.Body).Decode(&u)
	}
	var e struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&e)
	if e.Code == "user_not_found" {
		return User{}, ErrNotFound
	}
	return User{}, fmt.Errorf("users service returned %d %s", resp.StatusCode, e.Code)
}
