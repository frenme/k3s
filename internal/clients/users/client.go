package users

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 3 * time.Second, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
}

func (c *Client) Get(ctx context.Context, id string) (User, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/users/"+url.PathEscape(id), nil)
	if err != nil {
		return User{}, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		var u User
		return u, json.NewDecoder(resp.Body).Decode(&u)
	case http.StatusNotFound:
		return User{}, ErrNotFound
	default:
		return User{}, fmt.Errorf("users service returned %d", resp.StatusCode)
	}
}
