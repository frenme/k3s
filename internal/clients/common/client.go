package common

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

var ErrSettingNotFound = errors.New("setting not found")

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

func (c *Client) Bool(ctx context.Context, key string) (bool, error) {
	var v bool
	return v, c.setting(ctx, key, &v)
}

func (c *Client) setting(ctx context.Context, key string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/internal/v1/common/settings/"+url.PathEscape(key), nil)
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
	case body.Code == "setting_not_found":
		return ErrSettingNotFound
	default:
		return fmt.Errorf("common service returned %d %s", resp.StatusCode, body.Code)
	}
}
