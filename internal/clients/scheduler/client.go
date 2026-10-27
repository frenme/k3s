package scheduler

import (
	"context"
	"net/url"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
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
	api *httpx.ServiceClient
}

func New(baseURL string) *Client {
	return &Client{api: httpx.NewServiceClient("scheduler", baseURL, 30*time.Second)}
}

func (c *Client) Jobs(ctx context.Context) ([]Job, error) {
	list, err := httpx.Get[struct {
		Items []Job `json:"items"`
	}](ctx, c.api, "/internal/v1/scheduler/jobs")
	return list.Items, err
}

func (c *Client) Run(ctx context.Context, name string) (Run, error) {
	return httpx.Post[Run](ctx, c.api, "/internal/v1/scheduler/jobs/"+url.PathEscape(name)+"/run", nil)
}
