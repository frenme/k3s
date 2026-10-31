package auth

import (
	"context"
	"sync"
	"time"
)

type RoleSource interface {
	Roles(ctx context.Context, userID string) ([]string, error)
}

type rolesEntry struct {
	roles   []string
	expires time.Time
}

type rolesCall struct {
	done  chan struct{}
	roles []string
	err   error
}

type CachedRoles struct {
	src RoleSource
	ttl time.Duration
	max int
	now func() time.Time

	mu      sync.Mutex
	entries map[string]rolesEntry
	calls   map[string]*rolesCall
}

func NewCachedRoles(src RoleSource, ttl time.Duration, max int) *CachedRoles {
	return &CachedRoles{
		src:     src,
		ttl:     ttl,
		max:     max,
		now:     time.Now,
		entries: map[string]rolesEntry{},
		calls:   map[string]*rolesCall{},
	}
}

func (c *CachedRoles) Roles(ctx context.Context, userID string) ([]string, error) {
	c.mu.Lock()
	if e, ok := c.entries[userID]; ok && c.now().Before(e.expires) {
		c.mu.Unlock()
		return e.roles, nil
	}
	if call, ok := c.calls[userID]; ok {
		c.mu.Unlock()
		select {
		case <-call.done:
			return call.roles, call.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	call := &rolesCall{done: make(chan struct{})}
	c.calls[userID] = call
	c.mu.Unlock()

	call.roles, call.err = c.src.Roles(context.WithoutCancel(ctx), userID)

	c.mu.Lock()
	delete(c.calls, userID)
	if call.err == nil {
		if len(c.entries) >= c.max {
			c.evict()
		}
		c.entries[userID] = rolesEntry{roles: call.roles, expires: c.now().Add(c.ttl)}
	}
	c.mu.Unlock()
	close(call.done)
	return call.roles, call.err
}

func (c *CachedRoles) evict() {
	now := c.now()
	for id, e := range c.entries {
		if !now.Before(e.expires) {
			delete(c.entries, id)
		}
	}
	for id := range c.entries {
		if len(c.entries) < c.max {
			return
		}
		delete(c.entries, id)
	}
}
