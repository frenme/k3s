package common

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var ErrNotFound = httpx.Error{Code: "app_config_not_found"}

type Client struct {
	api *httpx.ServiceClient
}

func New(baseURL string) *Client {
	return &Client{api: httpx.NewServiceClient("common", baseURL, 3*time.Second)}
}

func (c *Client) Bool(ctx context.Context, key string) (bool, error) {
	entry, err := httpx.Get[struct {
		Value bool `json:"value"`
	}](ctx, c.api, "/internal/v1/common/app-config/"+url.PathEscape(key))
	if errors.Is(err, ErrNotFound) {
		return false, fmt.Errorf("app config %q is not set", key)
	}
	return entry.Value, err
}
