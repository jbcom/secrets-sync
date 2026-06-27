package cache

import (
	"context"
	"fmt"
	"testing"
	"time"
)

// compile-time: MemoryCache satisfies Cache.
var _ Cache = (*MemoryCache)(nil)

func TestMemorySetGet(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache(0)
	defer c.Close()

	if _, found, _ := c.Get(ctx, "k"); found {
		t.Fatal("empty cache should miss")
	}
	if err := c.Set(ctx, "k", []byte("v"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	v, found, err := c.Get(ctx, "k")
	if err != nil || !found || string(v) != "v" {
		t.Fatalf("get: v=%s found=%v err=%v", v, found, err)
	}
}

func TestMemoryExpiry(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache(0)
	defer c.Close()
	now := time.Unix(1000, 0)
	c.now = func() time.Time { return now }

	_ = c.Set(ctx, "k", []byte("v"), 10*time.Second)
	if _, found, _ := c.Get(ctx, "k"); !found {
		t.Fatal("should hit before expiry")
	}
	now = now.Add(11 * time.Second)
	if _, found, _ := c.Get(ctx, "k"); found {
		t.Fatal("should miss after expiry")
	}
}

func TestMemoryReturnsCopy(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache(0)
	defer c.Close()
	_ = c.Set(ctx, "k", []byte("abc"), time.Minute)
	v, _, _ := c.Get(ctx, "k")
	v[0] = 'X' // mutate the returned slice
	again, _, _ := c.Get(ctx, "k")
	if string(again) != "abc" {
		t.Fatalf("cache buffer was mutated through returned slice: %s", again)
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache(0)
	defer c.Close()
	_ = c.Set(ctx, "k", []byte("v"), time.Minute)
	_ = c.Delete(ctx, "k")
	if _, found, _ := c.Get(ctx, "k"); found {
		t.Fatal("deleted key should miss")
	}
}

func TestGetOrCompute(t *testing.T) {
	ctx := context.Background()
	c := NewMemoryCache(0)
	defer c.Close()

	calls := 0
	compute := func(context.Context) ([]byte, error) {
		calls++
		return []byte("computed"), nil
	}

	v, err := GetOrCompute(ctx, c, "k", time.Minute, compute)
	if err != nil || string(v) != "computed" {
		t.Fatalf("first: v=%s err=%v", v, err)
	}
	// Second call hits the cache; compute not invoked again.
	v, _ = GetOrCompute(ctx, c, "k", time.Minute, compute)
	if string(v) != "computed" || calls != 1 {
		t.Fatalf("expected 1 compute call, got %d", calls)
	}
}

func TestGetOrComputeNilCacheAlwaysComputes(t *testing.T) {
	calls := 0
	for range 3 {
		v, err := GetOrCompute(context.Background(), nil, "k", time.Minute, func(context.Context) ([]byte, error) {
			calls++
			return []byte("x"), nil
		})
		if err != nil || string(v) != "x" {
			t.Fatalf("nil-cache compute: v=%s err=%v", v, err)
		}
	}
	if calls != 3 {
		t.Fatalf("nil cache should compute every time, got %d calls", calls)
	}
}

func TestGetOrComputePropagatesError(t *testing.T) {
	c := NewMemoryCache(0)
	defer c.Close()
	_, err := GetOrCompute(context.Background(), c, "k", time.Minute, func(context.Context) ([]byte, error) {
		return nil, fmt.Errorf("boom")
	})
	if err == nil {
		t.Fatal("compute error should propagate")
	}
	// A failed compute must not poison the cache.
	if _, found, _ := c.Get(context.Background(), "k"); found {
		t.Fatal("failed compute should not be cached")
	}
}
