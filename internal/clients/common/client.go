package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

var ErrAppConfigNotFound = errors.New("config not found")

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		http:    httpx.NewClient("common", 3*time.Second),
	}
}

func (c *Client) Bool(ctx context.Context, key string) (bool, error) {
	var v bool
	return v, c.config(ctx, key, &v)
}

func (c *Client) config(ctx context.Context, key string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/common/app-config/"+url.PathEscape(key), nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Code  string          `json:"code"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("common service returned %d: %w", resp.StatusCode, err)
	}
	switch {
	case resp.StatusCode == http.StatusOK:
		return json.Unmarshal(body.Value, dst)
	case body.Code == "app_config_not_found":
		return ErrAppConfigNotFound
	default:
		return fmt.Errorf("common service returned %d %s", resp.StatusCode, body.Code)
	}
}
