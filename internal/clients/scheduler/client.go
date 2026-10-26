package scheduler

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

var (
	ErrJobNotFound = errors.New("job not found")
	ErrJobRunning  = errors.New("job is already running")
)

type Job struct {
	Name           string    `json:"name"`
	Schedule       string    `json:"schedule"`
	TimeoutSeconds int64     `json:"timeout_seconds"`
	NextRun        time.Time `json:"next_run"`
}

type Run struct {
	Job        string `json:"job"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error"`
}

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string) *Client {
	return &Client{baseURL: baseURL, http: httpx.NewClient("scheduler", 30*time.Second)}
}

func (c *Client) Jobs(ctx context.Context) ([]Job, error) {
	var out struct {
		Items []Job `json:"items"`
	}
	return out.Items, c.do(ctx, http.MethodGet, "/internal/v1/scheduler/jobs", &out)
}

func (c *Client) Run(ctx context.Context, name string) (Run, error) {
	var out Run
	return out, c.do(ctx, http.MethodPost, "/internal/v1/scheduler/jobs/"+url.PathEscape(name)+"/run", &out)
}

func (c *Client) do(ctx context.Context, method, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		return json.NewDecoder(resp.Body).Decode(dst)
	}
	var e struct {
		Code string `json:"code"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&e)
	switch e.Code {
	case "job_not_found":
		return ErrJobNotFound
	case "job_running":
		return ErrJobRunning
	}
	return fmt.Errorf("scheduler service returned %d %s", resp.StatusCode, e.Code)
}
