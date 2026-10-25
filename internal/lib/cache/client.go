package cache

import (
	"time"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

func NewClient(addr, password string) *redis.Client {
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     password,
		DialTimeout:  500 * time.Millisecond,
		ReadTimeout:  300 * time.Millisecond,
		WriteTimeout: 300 * time.Millisecond,
		PoolSize:     20,
	})
	_ = redisotel.InstrumentTracing(client)
	return client
}
