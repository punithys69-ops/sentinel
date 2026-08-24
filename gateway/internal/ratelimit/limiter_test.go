package ratelimit

import "testing"

// TestLimiter_ClientsHaveIndependentBuckets proves that different client keys
// produce separate, non-interfering token buckets.
func TestLimiter_ClientsHaveIndependentBuckets(t *testing.T) {
	limiter, err := NewLimiter(2, 0)
	if err != nil {
		t.Fatal(err)
	}

	if allowed, _ := limiter.Allow("client-a"); !allowed {
		t.Fatal("client-a request 1 should be allowed")
	}

	if allowed, _ := limiter.Allow("client-a"); !allowed {
		t.Fatal("client-a request 2 should be allowed")
	}

	if allowed, _ := limiter.Allow("client-a"); allowed {
		t.Fatal("client-a request 3 should be rejected")
	}

	// Client B gets its own bucket — exhausting client-a must not affect it.
	if allowed, _ := limiter.Allow("client-b"); !allowed {
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

	if allowed, _ := limiter.Allow("client-a"); !allowed {
		t.Fatal("first request should be allowed")
	}

	if allowed, _ := limiter.Allow("client-a"); !allowed {
		t.Fatal("second request should be allowed")
	}

	if allowed, _ := limiter.Allow("client-a"); allowed {
		t.Fatal("third request should be rejected")
	}
}
