package v1

import "time"

type StatusBody struct {
	Status string `json:"status" enum:"ok,unhealthy"`
}

type HealthResponse struct {
	Status int
	Body   StatusBody
}

type Check struct {
	Name      string            `json:"name"`
	Kind      string            `json:"kind" enum:"service,infra"`
	Healthy   bool              `json:"healthy"`
	LatencyMS int64             `json:"latency_ms"`
	Error     string            `json:"error,omitempty"`
	Details   map[string]string `json:"details,omitempty"`
}

type DetailsResponse struct {
	Status int
	Body   struct {
		StatusBody
		CheckedAt time.Time `json:"checked_at"`
		Checks    []Check   `json:"checks"`
	}
}
