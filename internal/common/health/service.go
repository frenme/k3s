package health

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/egor/k3s-base/internal/lib/httpx"
)

const (
	KindService = "service"
	KindInfra   = "infra"

	cacheTTL = 2 * time.Second
)

type Target struct {
	Name string
	URL  string
}

type Probe struct {
	Name string
	Run  func(ctx context.Context) error
}

type Result struct {
	Name      string
	Kind      string
	Healthy   bool
	LatencyMS int64
	Error     string
	Details   map[string]string
}

type Report struct {
	CheckedAt time.Time
	Results   []Result
}

func (r Report) Healthy() bool {
	for _, res := range r.Results {
		if !res.Healthy {
			return false
		}
	}
	return true
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
	probes  []Probe
	client  *http.Client

	mu     sync.Mutex
	cached Report
}

func NewService(targets []Target, probes ...Probe) *Service {
	return &Service{
		targets: targets,
		probes:  probes,
		client:  httpx.NewClient("health", 2*time.Second, httpx.WithoutResilience()),
	}
}

func (s *Service) Check(ctx context.Context) Report {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.cached.CheckedAt.IsZero() && time.Since(s.cached.CheckedAt) < cacheTTL {
		return s.cached
	}
	results := make([]Result, len(s.targets)+len(s.probes))
	var wg sync.WaitGroup
	for i, t := range s.targets {
		wg.Go(func() { results[i] = s.checkService(ctx, t) })
	}
	for i, p := range s.probes {
		wg.Go(func() { results[len(s.targets)+i] = checkProbe(ctx, p) })
	}
	wg.Wait()
	s.cached = Report{CheckedAt: time.Now().UTC(), Results: results}
	return s.cached
}

func checkProbe(ctx context.Context, p Probe) Result {
	start := time.Now()
	res := Result{Name: p.Name, Kind: KindInfra, Healthy: true}
	if err := p.Run(ctx); err != nil {
		res.Healthy, res.Error = false, err.Error()
	}
	res.LatencyMS = time.Since(start).Milliseconds()
	return res
}

func (s *Service) checkService(ctx context.Context, t Target) Result {
	start := time.Now()
	res := Result{Name: t.Name, Kind: KindService}
	defer func() { res.LatencyMS = time.Since(start).Milliseconds() }()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, t.URL+"/readyz", nil)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	resp, err := s.client.Do(req)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		res.Healthy = true
		return res
	}
	res.Error = fmt.Sprintf("readyz returned %d", resp.StatusCode)
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	var details map[string]string
	if json.Unmarshal(body, &details) == nil {
		res.Details = details
	}
	return res
}
