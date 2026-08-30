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

	log.Printf("[%s] connecting to Redis at %s", gatewayID, redisAddr)

	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	if err := rdb.Ping(context.Background()).Err(); err != nil {
		log.Fatalf("[%s] Redis ping failed: %v", gatewayID, err)
	}

	log.Printf("[%s] Redis OK", gatewayID)

	// capacity=10, rate=2 tokens/s, idle TTL=60 s
	limiter, err := ratelimit.NewRedisLimiter(rdb, 10, 2, 60*time.Second)
	if err != nil {
		log.Fatalf("[%s] failed to create rate limiter: %v", gatewayID, err)
	}

	backendAddr := envOr("BACKEND_ADDR", "http://localhost:9001")

	router, err := gw.NewRouter(map[string]string{
		"/api": backendAddr,
	})
	if err != nil {
		log.Fatalf("[%s] failed to create router: %v", gatewayID, err)
	}

	handler := gw.RateLimitMiddleware(limiter, router)

	log.Printf("[%s] listening on :8080", gatewayID)

	log.Fatal(http.ListenAndServe(":8080", handler))
}

