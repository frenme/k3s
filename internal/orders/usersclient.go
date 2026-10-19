package orders

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

type UserChecker interface {
	Exists(ctx context.Context, id string) (bool, error)
}

type UsersClient struct {
	baseURL string
	http    *http.Client
}

func NewUsersClient(baseURL string) *UsersClient {
	return &UsersClient{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 3 * time.Second},
	}
}

func (c *UsersClient) Exists(ctx context.Context, id string) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/users/"+url.PathEscape(id), nil)
	if err != nil {
		return false, err
	}
	if rid := httpx.RequestIDFrom(ctx); rid != "" {
		req.Header.Set(httpx.RequestIDHeader, rid)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return false, err
	}
	defer func() { _ = resp.Body.Close() }()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("users service returned %d", resp.StatusCode)
	}
}
