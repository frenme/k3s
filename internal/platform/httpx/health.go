package httpx

import (
	"context"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Check func(ctx context.Context) error

type Health struct {
	ready  atomic.Bool
	checks map[string]Check
}

func NewHealth() *Health {
	h := &Health{checks: map[string]Check{}}
	h.ready.Store(true)
	return h
}

