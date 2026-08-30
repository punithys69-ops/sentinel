package ratelimit

import (
	"context"
	"time"
)

// RateLimiter is the single interface both the in-memory and Redis-backed
// implementations satisfy. Middleware and tests depend on this, not on
// any concrete type.
//
// Allow returns:
//   - (true,  0,           nil) — request is allowed
//   - (false, retryAfter,  nil) — rejected; retryAfter is how long to wait
//   - (false, 0,           err) — limiter is unavailable
type RateLimiter interface {
	Allow(ctx context.Context, clientKey string) (allowed bool, retryAfter time.Duration, err error)
}
