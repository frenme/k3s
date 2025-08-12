package rds

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

func New(addr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  300 * time.Millisecond,
		WriteTimeout: 300 * time.Millisecond,
		PoolSize:     20,
	})
}

func Ping(c *redis.Client) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		return c.Ping(ctx).Err()
	}
}
