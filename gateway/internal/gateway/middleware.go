package gateway

import (
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

// RateLimitMiddleware applies per-client rate limiting
// before passing the request to the next handler.
func RateLimitMiddleware(
	limiter *ratelimit.Limiter,
	next http.Handler,
) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := clientKey(r)

		allowed, err := limiter.Allow(key)
		if err != nil {
			http.Error(w, "rate limiter unavailable", http.StatusInternalServerError)
			return
		}

		if !allowed {
			retryAfter, err := limiter.RetryAfter(key)
			if err != nil {
				http.Error(w, "rate limiter unavailable", http.StatusInternalServerError)
				return
			}

			seconds := max(1, int(retryAfter+0.999999))

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
