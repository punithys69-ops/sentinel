package gateway

import (
	"fmt"
	"log"
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
// Fail-closed policy: when the limiter returns an error (e.g. Redis
// unreachable), the gateway responds with 500 rather than silently
// admitting the request without verification.
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
			log.Printf("client=%s error=%q status=500", key, err)
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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
