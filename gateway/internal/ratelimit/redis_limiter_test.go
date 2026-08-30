package ratelimit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

// TestRedisLimiter_Integration runs against a real Redis instance.
//
// It is skipped automatically when Redis is not reachable, so it is safe
// to run in environments without Redis (e.g. CI without a sidecar).
func TestRedisLimiter_Integration(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available at localhost:6379 (%v) — skipping integration test", err)
	}

	// Use a unique key per test run so parallel runs don't interfere.
	testKey := fmt.Sprintf("test:integration:%d", time.Now().UnixNano())

	// Clean up after ourselves regardless of outcome.
	t.Cleanup(func() {
		client.Del(ctx, "rate_limit:"+testKey)
	})

	limiter, err := NewRedisLimiter(client, 3, 1, 60*time.Second, "test")
	if err != nil {
		t.Fatal(err)
	}

	// --- Burst: exactly 3 requests should be allowed ---
	for i := 1; i <= 3; i++ {
		allowed, _, err := limiter.Allow(ctx, testKey)
		if err != nil {
			t.Fatalf("request %d: unexpected error: %v", i, err)
		}

		if !allowed {
			t.Fatalf("request %d should have been allowed (burst capacity = 3)", i)
		}
	}

	// --- 4th request must be rejected ---
	allowed, retryAfter, err := limiter.Allow(ctx, testKey)
	if err != nil {
		t.Fatalf("request 4: unexpected error: %v", err)
	}

	if allowed {
		t.Fatal("4th request should have been rejected (bucket empty)")
	}

	if retryAfter <= 0 {
		t.Fatalf("expected non-zero Retry-After, got %v", retryAfter)
	}

	t.Logf("4th request rejected — Retry-After: %v", retryAfter)

	// --- Refill: advance by 1 second and expect one token ---
	time.Sleep(1100 * time.Millisecond)

	allowed, _, err = limiter.Allow(ctx, testKey)
	if err != nil {
		t.Fatalf("post-refill request: unexpected error: %v", err)
	}

	if !allowed {
		t.Fatal("post-refill request should have been allowed (1 token refilled)")
	}

	t.Log("post-refill request allowed — refill math verified")
}

// TestRedisLimiter_ClientsAreIndependent proves two different client keys
// get independent buckets in Redis.
func TestRedisLimiter_ClientsAreIndependent(t *testing.T) {
	client := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer client.Close()

	ctx := context.Background()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Redis not available — skipping: %v", err)
	}

	nano := time.Now().UnixNano()
	keyA := fmt.Sprintf("test:iso:a:%d", nano)
	keyB := fmt.Sprintf("test:iso:b:%d", nano)

	t.Cleanup(func() {
		client.Del(ctx, "rate_limit:"+keyA, "rate_limit:"+keyB)
	})

	limiter, err := NewRedisLimiter(client, 1, 1, 60*time.Second, "test")
	if err != nil {
		t.Fatal(err)
	}

	// Exhaust client A.
	limiter.Allow(ctx, keyA) //nolint:errcheck

	allowedA, _, _ := limiter.Allow(ctx, keyA)
	if allowedA {
		t.Fatal("client-a second request should be rejected")
	}

	// Client B must have its own fresh bucket.
	allowedB, _, err := limiter.Allow(ctx, keyB)
	if err != nil {
		t.Fatal(err)
	}

	if !allowedB {
		t.Fatal("client-b should have an independent bucket in Redis")
	}
}
