package gateway

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/punithys69-ops/sentinel/internal/ratelimit"
)

// clientKey returns a stable identifier for the caller.
//
// For Phase 1 we use the remote IP address.
// Later, this can be replaced or extended to support API keys.
func clientKey(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil {
		return host
	}

	// Fallback in case RemoteAddr is not host:port formatted.
	return r.RemoteAddr
}

// RateLimitMiddleware applies per-client rate limiting before passing the
// request to next. It accepts any ratelimit.RateLimiter, so the same
// middleware works with both the in-memory (Phase 1) and Redis (Phase 2)
// implementations.
//
// On rejection it writes:
//
//	HTTP 429 Too Many Requests
//	Retry-After: <seconds>
func RateLimitMiddleware(
	limiter ratelimit.RateLimiter,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientKey(r)

		// Use the request context so the limiter call is cancelled if the
		// client disconnects before we get a decision from Redis.
		allowed, retryAfter, err := limiter.Allow(r.Context(), key)
		if err != nil {
			http.Error(w, "rate limiter unavailable", http.StatusInternalServerError)
			return
		}

		if !allowed {
			// Convert duration → whole seconds, minimum 1.
			seconds := max(1, int(retryAfter.Seconds()+0.9999))

			w.Header().Set("Retry-After", strconv.Itoa(seconds))
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintln(w, "rate limit exceeded")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Allow calls the limiter with a background context.
// Useful in tests that don't have a live request context.
func allowWithBackground(l ratelimit.RateLimiter, key string) (bool, error) {
	allowed, _, err := l.Allow(context.Background(), key)
	return allowed, err
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
