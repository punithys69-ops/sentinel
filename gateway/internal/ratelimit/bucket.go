package ratelimit

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalidCapacity   = errors.New("capacity must be greater than 0")
	ErrInvalidRefillRate = errors.New("refill rate cannot be negative")
)

// TokenBucket is a thread-safe, in-memory token bucket.
//
// capacity   = maximum number of tokens the bucket can hold
// tokens     = current number of available tokens
// refillRate = tokens added per second
type TokenBucket struct {
	mu         sync.Mutex
	capacity   float64
	tokens     float64
	refillRate float64
	lastRefill time.Time
}

// NewTokenBucket creates a bucket starting at full capacity.
func NewTokenBucket(capacity, refillRate float64) (*TokenBucket, error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}

	if refillRate < 0 {
		return nil, ErrInvalidRefillRate
	}

	now := time.Now()

	return &TokenBucket{
		capacity:   capacity,
		tokens:     capacity,
		refillRate: refillRate,
		lastRefill: now,
	}, nil
}

// Allow reports whether one request is allowed.
//
// If at least one token is available, one token is consumed
// and Allow returns true. Otherwise it returns false.
func (b *TokenBucket) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()

	if b.tokens < 1 {
		return false
	}

	b.tokens--
	return true
}

// RetryAfter returns an estimate of how long to wait until
// one token becomes available.
func (b *TokenBucket) RetryAfter() time.Duration {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.refill()

	if b.tokens >= 1 || b.refillRate == 0 {
		return 0
	}

	seconds := (1 - b.tokens) / b.refillRate

	return time.Duration(seconds * float64(time.Second))
}

// refill updates the bucket according to elapsed time.
//
// The caller must hold b.mu.
func (b *TokenBucket) refill() {
	now := time.Now()

	elapsed := now.Sub(b.lastRefill).Seconds()

	if elapsed <= 0 {
		return
	}

	b.tokens += elapsed * b.refillRate

	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}

	b.lastRefill = now
}
