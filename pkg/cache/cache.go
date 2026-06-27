// Package cache provides a small pluggable cache abstraction for expensive,
// re-derivable results (discovery output, ListSecrets results). An in-memory
// TTL cache ships by default; external backends (Redis, Memcached) implement
// the same interface for horizontally-scaled deployments that need a shared
// cache across replicas.
package cache

import (
	"context"
	"sync"
	"time"
)

// Cache is a string-keyed byte-value cache with per-entry TTL. Implementations
// must be safe for concurrent use. A cache miss returns (nil, false, nil); an
// error is reserved for backend failures.
type Cache interface {
	Get(ctx context.Context, key string) (value []byte, found bool, err error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Delete(ctx context.Context, key string) error
}

type entry struct {
	value   []byte
	expires time.Time
}

// MemoryCache is an in-process TTL cache. Expired entries are removed lazily on
// access and proactively by an optional janitor.
type MemoryCache struct {
	mu      sync.RWMutex
	items   map[string]entry
	now     func() time.Time
	stopCh  chan struct{}
	stopped bool
}

// NewMemoryCache returns an in-memory cache. If sweepInterval > 0 a background
// janitor evicts expired entries on that interval; call Close to stop it.
func NewMemoryCache(sweepInterval time.Duration) *MemoryCache {
	c := &MemoryCache{items: map[string]entry{}, now: time.Now, stopCh: make(chan struct{})}
	if sweepInterval > 0 {
		go c.janitor(sweepInterval)
	}
	return c
}

func (c *MemoryCache) janitor(interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
			c.sweep()
		}
	}
}

func (c *MemoryCache) sweep() {
	now := c.now()
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, e := range c.items {
		if !e.expires.IsZero() && now.After(e.expires) {
			delete(c.items, k)
		}
	}
}

// Get returns the value for key if present and unexpired.
func (c *MemoryCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.mu.RLock()
	e, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false, nil
	}
	if !e.expires.IsZero() && c.now().After(e.expires) {
		c.Delete(context.Background(), key)
		return nil, false, nil
	}
	// Return a copy so callers can't mutate the cached buffer.
	out := make([]byte, len(e.value))
	copy(out, e.value)
	return out, true, nil
}

// Set stores value under key for ttl (0 = no expiry).
func (c *MemoryCache) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	stored := make([]byte, len(value))
	copy(stored, value)
	var exp time.Time
	if ttl > 0 {
		exp = c.now().Add(ttl)
	}
	c.mu.Lock()
	c.items[key] = entry{value: stored, expires: exp}
	c.mu.Unlock()
	return nil
}

// Delete removes key.
func (c *MemoryCache) Delete(_ context.Context, key string) error {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
	return nil
}

// Close stops the janitor goroutine. It is safe to call more than once.
func (c *MemoryCache) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.stopped {
		close(c.stopCh)
		c.stopped = true
	}
}

// GetOrCompute is the cache-aside pattern: return the cached value for key, or
// invoke compute, store its result under key for ttl, and return it. A nil cache
// always computes (caching disabled), so callers need no nil checks. Cache read
// errors fall through to compute rather than failing the operation.
func GetOrCompute(ctx context.Context, c Cache, key string, ttl time.Duration, compute func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	if c != nil {
		if v, found, err := c.Get(ctx, key); err == nil && found {
			return v, nil
		}
	}
	v, err := compute(ctx)
	if err != nil {
		return nil, err
	}
	if c != nil {
		_ = c.Set(ctx, key, v, ttl)
	}
	return v, nil
}
