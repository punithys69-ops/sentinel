package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
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

func envFloat(key string, fallback float64) float64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		log.Fatalf("invalid %s: %v", key, err)
	}
	return f
}

func main() {
	gatewayID := envOr("GATEWAY_ID", "gateway")
	redisAddr := envOr("REDIS_ADDR", "localhost:6379")
	upstreamAddr := envOr("UPSTREAM_ADDR", "http://localhost:9001")
	capacity := envFloat("RATE_CAPACITY", 10)
	refillRate := envFloat("RATE_REFILL", 2)

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
	log.Printf("rate limit: capacity=%.0f refill=%.0f/s", capacity, refillRate)

	limiter, err := ratelimit.NewRedisLimiter(rdb, capacity, refillRate, 60*time.Second, gatewayID)
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
