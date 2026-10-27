package httpx

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type ServiceClient struct {
	name    string
	baseURL string
	http    *http.Client
}

func NewServiceClient(name, baseURL string, timeout time.Duration) *ServiceClient {
	return &ServiceClient{name: name, baseURL: baseURL, http: NewClient(name, timeout)}
}

func Get[T any](ctx context.Context, c *ServiceClient, path string) (T, error) {
	return call[T](ctx, c, http.MethodGet, path, nil)
}

func Post[T any](ctx context.Context, c *ServiceClient, path string, body any) (T, error) {
	return call[T](ctx, c, http.MethodPost, path, body)
}

func call[T any](ctx context.Context, c *ServiceClient, method, path string, body any) (T, error) {
	var out T
	var payload io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return out, err
		}
		payload = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, payload)
	if err != nil {
		return out, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return out, c.unavailable(err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			return out, c.unavailable(fmt.Errorf("%s returned %d with an unreadable body: %w", c.name, resp.StatusCode, err))
		}
		return out, nil
	}

	var apiErr Error
	_ = json.NewDecoder(resp.Body).Decode(&apiErr)
	if resp.StatusCode >= http.StatusInternalServerError || apiErr.Code == "" {
		return out, c.unavailable(fmt.Errorf("%s returned %d %s", c.name, resp.StatusCode, apiErr.Code))
	}
	apiErr.Status = resp.StatusCode
	return out, apiErr
}

func (c *ServiceClient) unavailable(cause error) error {
	return fmt.Errorf("%w: %w", ErrUnavailable.With(c.name), cause)
}
