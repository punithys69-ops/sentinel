package gateway

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/punithys69-ops/sentinel/internal/ratelimit"
)

// TestRateLimitMiddleware_Returns429 proves the middleware correctly returns
// HTTP 429 with a Retry-After header when the client's bucket is empty.
//
// Uses httptest so no real server is needed — the handler is exercised directly.
func TestRateLimitMiddleware_Returns429(t *testing.T) {
	// capacity=1, refillRate=0: one request allowed, then hard stop.
	limiter, err := ratelimit.NewLimiter(1, 0)
	if err != nil {
		t.Fatal(err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimitMiddleware(limiter, next)

	// --- Request 1: should pass through to next ---
	req1 := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req1.RemoteAddr = "127.0.0.1:10001"

	rec1 := httptest.NewRecorder()
	handler.ServeHTTP(rec1, req1)

	if rec1.Code != http.StatusOK {
		t.Fatalf("expected first request to return 200, got %d", rec1.Code)
	}

	// --- Request 2 (same client): bucket empty, must be rejected ---
	req2 := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	req2.RemoteAddr = "127.0.0.1:10001"

	rec2 := httptest.NewRecorder()
	handler.ServeHTTP(rec2, req2)

	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("expected 429, got %d", rec2.Code)
	}

	if rec2.Header().Get("Retry-After") == "" {
		t.Fatal("expected Retry-After header to be set")
	}
}

// TestRateLimitMiddleware_DifferentClientsAreIndependent proves that
// exhausting one client's bucket does not affect another client.
func TestRateLimitMiddleware_DifferentClientsAreIndependent(t *testing.T) {
	// capacity=1, refillRate=0.
	limiter, err := ratelimit.NewLimiter(1, 0)
	if err != nil {
		t.Fatal(err)
	}

	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := RateLimitMiddleware(limiter, next)

	// Exhaust client A.
	reqA := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	reqA.RemoteAddr = "10.0.0.1:10001"
	handler.ServeHTTP(httptest.NewRecorder(), reqA)

	// Client A second request → 429.
	reqA2 := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	reqA2.RemoteAddr = "10.0.0.1:10001"
	recA2 := httptest.NewRecorder()
	handler.ServeHTTP(recA2, reqA2)

	if recA2.Code != http.StatusTooManyRequests {
		t.Fatalf("client-a second request: expected 429, got %d", recA2.Code)
	}

	// Client B has never been seen — its own fresh bucket must allow it.
	reqB := httptest.NewRequest(http.MethodGet, "/api/test", nil)
	reqB.RemoteAddr = "10.0.0.2:10001"
	recB := httptest.NewRecorder()
	handler.ServeHTTP(recB, reqB)

	if recB.Code != http.StatusOK {
		t.Fatalf("client-b first request: expected 200, got %d", recB.Code)
	}
}
