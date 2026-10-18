package cache

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/redis/go-redis/v9"
)

var lookups = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "cache_lookups_total",
	Help: "Cache lookups by result.",
}, []string{"result"})

type Cache struct {
	client *redis.Client
	prefix string
}

func New(client *redis.Client, prefix string) *Cache {
	if client == nil {
		return nil
	}
	return &Cache{client: client, prefix: prefix + ":"}
}

func (c *Cache) Delete(ctx context.Context, key string) {
	if c == nil {
		return
	}
	_ = c.client.Del(ctx, c.prefix+key).Err()
}

func Fetch[T any](ctx context.Context, c *Cache, key string, ttl time.Duration, load func() (T, error)) (T, error) {
	var zero T
	if c == nil {
		return load()
	}
	raw, err := c.client.Get(ctx, c.prefix+key).Bytes()
	switch {
	case err == nil:
		var v T
		if json.Unmarshal(raw, &v) == nil {
			lookups.WithLabelValues("hit").Inc()
			return v, nil
		}
		lookups.WithLabelValues("error").Inc()
	case errors.Is(err, redis.Nil):
		lookups.WithLabelValues("miss").Inc()
	default:
		lookups.WithLabelValues("error").Inc()
	}
	v, err := load()
	if err != nil {
		return zero, err
	}
	if data, err := json.Marshal(v); err == nil {
		_ = c.client.Set(ctx, c.prefix+key, data, ttl).Err()
	}
	return v, nil
}
