package health

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

type Target struct {
	Name string
	URL  string
}

type Status struct {
	Name    string
	Healthy bool
}

func ParseTargets(spec string) ([]Target, error) {
	var targets []Target
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, url, ok := strings.Cut(part, "=")
		if !ok || name == "" || !strings.HasPrefix(url, "http") {
			return nil, fmt.Errorf("invalid health target %q", part)
		}
		targets = append(targets, Target{Name: name, URL: strings.TrimSuffix(url, "/")})
	}
	return targets, nil
}

type Service struct {
	targets []Target
	client  *http.Client
}

func NewService(targets []Target) *Service {
	return &Service{
		targets: targets,
		client:  &http.Client{Timeout: 2 * time.Second, Transport: otelhttp.NewTransport(http.DefaultTransport)},
	}
}

func (s *Service) Check(ctx context.Context) []Status {
	statuses := make([]Status, len(s.targets))
	var wg sync.WaitGroup
	for i, t := range s.targets {
		wg.Go(func() {
			statuses[i] = Status{Name: t.Name, Healthy: s.ready(ctx, t.URL)}
		})
	}
	wg.Wait()
	return statuses
}

func (s *Service) ready(ctx context.Context, url string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url+"/readyz", nil)
	if err != nil {
		return false
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}
