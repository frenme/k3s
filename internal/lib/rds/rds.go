package rds

import (
	"time"

	"github.com/redis/go-redis/v9"
)

func New(addr, password string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  300 * time.Millisecond,
		WriteTimeout: 300 * time.Millisecond,
		PoolSize:     20,
	})
}
