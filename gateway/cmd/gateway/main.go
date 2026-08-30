package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	gw "github.com/punithys69-ops/sentinel/internal/gateway"
	"github.com/punithys69-ops/sentinel/internal/ratelimit"
)

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	gatewayID := envOr("GATEWAY_ID", "gateway")
	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	upstreamAddr := envOr("UPSTREAM_ADDR", "http://localhost:9001")

	log.SetFlags(log.LstdFlags)
	log.SetPrefix("[" + gatewayID + "] ")

	log.Printf("connecting to Redis at %s", redisAddr)

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("Redis ping failed: %v", err)
	}

	log.Printf("Redis OK")

	// capacity=10, refill=2 tokens/s, idle TTL=60s.
	// These could also be read from environment variables in a later phase.
	limiter, err := ratelimit.NewRedisLimiter(rdb, 10, 2, 60*time.Second)
	if err != nil {
		log.Fatalf("failed to create rate limiter: %v", err)
	}

	router, err := gw.NewRouter(map[string]string{
		"/api": upstreamAddr,
	})
	if err != nil {
		log.Fatalf("failed to create router: %v", err)
	}

	handler := gw.RateLimitMiddleware(limiter, router)

	log.Printf("listening on :8080 → upstream %s", upstreamAddr)

	log.Fatal(http.ListenAndServe(":8080", handler))
}
