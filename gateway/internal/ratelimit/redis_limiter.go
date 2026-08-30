package ratelimit

import (
	"context"
	_ "embed"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:embed token_bucket.lua
var tokenBucketScript string

// redisScript is the preloaded Lua script handle.
// go-redis will SCRIPT LOAD on first use and EVALSHA on subsequent calls,
// falling back to EVAL automatically on NOSCRIPT errors.
var redisScript = redis.NewScript(tokenBucketScript)

// RedisLimiter is a distributed, per-client token bucket backed by Redis.
//
// The entire refill → check → consume → write cycle executes inside a single
// atomic Lua script, eliminating the GET/SET race described in Phase 2.2.
type RedisLimiter struct {
	client     *redis.Client
	capacity   float64
	refillRate float64
	ttl        time.Duration
}

// NewRedisLimiter creates a RedisLimiter.
//
//   - capacity   : maximum tokens a client may accumulate
//   - refillRate : tokens added per second
//   - ttl        : how long an idle client's bucket survives in Redis
func NewRedisLimiter(
	client *redis.Client,
	capacity, refillRate float64,
	ttl time.Duration,
) (*RedisLimiter, error) {
	if capacity <= 0 {
		return nil, ErrInvalidCapacity
	}

	if refillRate < 0 {
		return nil, ErrInvalidRefillRate
	}

	return &RedisLimiter{
		client:     client,
		capacity:   capacity,
		refillRate: refillRate,
		ttl:        ttl,
	}, nil
}

// bucketKey returns the Redis hash key for a client.
func bucketKey(clientKey string) string {
	return fmt.Sprintf("rate_limit:%s", clientKey)
}

// Allow checks whether this client may make one more request.
//
// Satisfies the RateLimiter interface.
// Returns (true, 0, nil) when allowed, or (false, retryAfter, nil) when rejected.
func (l *RedisLimiter) Allow(ctx context.Context, clientKey string) (bool, time.Duration, error) {
	nowMs := time.Now().UnixMilli()
	ttlMs := l.ttl.Milliseconds()

	result, err := redisScript.Run(
		ctx,
		l.client,
		[]string{bucketKey(clientKey)},
		l.capacity,
		l.refillRate,
		nowMs,
		ttlMs,
	).Int64Slice()

	if err != nil {
		return false, 0, fmt.Errorf("redis limiter: %w", err)
	}

	// Script returns: {allowed, retry_after_ms}
	allowed := result[0] == 1
	retryAfter := time.Duration(result[1]) * time.Millisecond

	return allowed, retryAfter, nil
}
