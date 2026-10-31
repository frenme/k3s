package lock

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
)

var ErrHeld = errors.New("lock is held by another owner")

var releaseScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0`)

type Locker struct {
	client *redis.Client
	prefix string
	owner  string
}

func New(client *redis.Client, prefix, owner string) *Locker {
	if client == nil {
		return nil
	}
	return &Locker{client: client, prefix: prefix + ":lock:", owner: owner}
}

func (l *Locker) Acquire(ctx context.Context, name string, ttl time.Duration) (func(), error) {
	key := l.prefix + name
	ok, err := l.client.SetNX(ctx, key, l.owner, ttl).Result()
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrHeld
	}
	return func() {
		ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		_ = releaseScript.Run(ctx, l.client, []string{key}, l.owner).Err()
	}, nil
}
