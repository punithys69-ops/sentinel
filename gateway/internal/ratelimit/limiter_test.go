package ratelimit

import (
	"context"
	"testing"
)

// TestLimiter_ClientsHaveIndependentBuckets proves that different client keys
// produce separate, non-interfering token buckets.
func TestLimiter_ClientsHaveIndependentBuckets(t *testing.T) {
	limiter, err := NewLimiter(2, 0)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	if allowed, _, _ := limiter.Allow(ctx, "client-a"); !allowed {
		t.Fatal("client-a request 1 should be allowed")
	}

	if allowed, _, _ := limiter.Allow(ctx, "client-a"); !allowed {
		t.Fatal("client-a request 2 should be allowed")
	}

	if allowed, _, _ := limiter.Allow(ctx, "client-a"); allowed {
		t.Fatal("client-a request 3 should be rejected")
	}

	// Client B gets its own bucket — exhausting client-a must not affect it.
	if allowed, _, _ := limiter.Allow(ctx, "client-b"); !allowed {
		t.Fatal("client-b should have an independent bucket")
	}
}

// TestLimiter_SameClientSharesBucket proves that repeated calls with the
// same client key accumulate against a single shared bucket.
func TestLimiter_SameClientSharesBucket(t *testing.T) {
	limiter, err := NewLimiter(2, 0)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	if allowed, _, _ := limiter.Allow(ctx, "client-a"); !allowed {
		t.Fatal("first request should be allowed")
	}

	if allowed, _, _ := limiter.Allow(ctx, "client-a"); !allowed {
		t.Fatal("second request should be allowed")
	}

	if allowed, _, _ := limiter.Allow(ctx, "client-a"); allowed {
		t.Fatal("third request should be rejected")
	}
}

// TestLimiter_RetryAfterIsNonZeroOnRejection proves that when a request is
// rejected, a non-zero retry duration is returned.
func TestLimiter_RetryAfterIsNonZeroOnRejection(t *testing.T) {
	// capacity=1, refillRate=1: after one request, one token/sec refill.
	limiter, err := NewLimiter(1, 1)
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	// Consume the only token.
	limiter.Allow(ctx, "client-a") //nolint:errcheck

	// Second request should be rejected with a non-zero retry duration.
	allowed, retryAfter, err := limiter.Allow(ctx, "client-a")
	if err != nil {
		t.Fatal(err)
	}

	if allowed {
		t.Fatal("expected rejection")
	}

	if retryAfter <= 0 {
		t.Fatalf("expected non-zero retry duration, got %v", retryAfter)
	}
}
