package ratelimit

import (
	"testing"
	"time"
)

// TestTokenBucket_BurstLimit proves a client can burst up to
// exactly the configured capacity, but not beyond it.
func TestTokenBucket_BurstLimit(t *testing.T) {
	bucket, err := NewTokenBucket(5, 0)
	if err != nil {
		t.Fatal(err)
	}

	// Exactly the capacity should be allowed.
	for i := 1; i <= 5; i++ {
		if !bucket.Allow() {
			t.Fatalf("request %d should have been allowed", i)
		}
	}

	// The bucket is now empty.
	if bucket.Allow() {
		t.Fatal("6th request should have been rejected")
	}
}

// TestTokenBucket_RefillsOverTime proves the lazy-refill math is correct.
//
// capacity=1, refillRate=10 tokens/sec → 1 token every 100ms.
// After sleeping 150ms the bucket should have:
//
//	min(1, 0 + 0.15×10) = min(1, 1.5) = 1
func TestTokenBucket_RefillsOverTime(t *testing.T) {
	bucket, err := NewTokenBucket(1, 10)
	if err != nil {
		t.Fatal(err)
	}

	// Bucket starts full.
	if !bucket.Allow() {
		t.Fatal("first request should have been allowed")
	}

	// No token remains.
	if bucket.Allow() {
		t.Fatal("second immediate request should have been rejected")
	}

	// 150ms × 10 tokens/sec = 1.5 tokens, capped at capacity 1.
	time.Sleep(150 * time.Millisecond)

	if !bucket.Allow() {
		t.Fatal("request should have been allowed after refill")
	}
}

// TestTokenBucket_NeverExceedsCapacity proves the capacity ceiling holds
// even when elapsed time would mathematically produce more tokens.
//
// capacity=3, refillRate=100 tokens/sec.
// After 100ms: 100×0.1 = 10 tokens, but min(10, 3) = 3.
func TestTokenBucket_NeverExceedsCapacity(t *testing.T) {
	bucket, err := NewTokenBucket(3, 100)
	if err != nil {
		t.Fatal(err)
	}

	// Give it enough time to refill, but capacity must still be 3.
	time.Sleep(100 * time.Millisecond)

	allowed := 0

	for i := 0; i < 5; i++ {
		if bucket.Allow() {
			allowed++
		}
	}

	if allowed != 3 {
		t.Fatalf("expected 3 allowed requests, got %d", allowed)
	}
}

// TestTokenBucket_ConcurrentAccess proves the mutex prevents over-counting
// when 200 goroutines race against a bucket that only has 100 tokens.
func TestTokenBucket_ConcurrentAccess(t *testing.T) {
	bucket, err := NewTokenBucket(100, 0)
	if err != nil {
		t.Fatal(err)
	}

	const requests = 200

	results := make(chan bool, requests)

	for i := 0; i < requests; i++ {
		go func() {
			results <- bucket.Allow()
		}()
	}

	allowed := 0

	for i := 0; i < requests; i++ {
		if <-results {
			allowed++
		}
	}

	if allowed != 100 {
		t.Fatalf("expected exactly 100 allowed requests, got %d", allowed)
	}
}
