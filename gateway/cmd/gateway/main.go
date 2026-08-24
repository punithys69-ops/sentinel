package main

import (
	"log"
	"net/http"

	"github.com/punithys69-ops/sentinel/internal/gateway"
	"github.com/punithys69-ops/sentinel/internal/ratelimit"
)

func main() {
	router, err := gateway.NewRouter(map[string]string{
		"/api": "http://localhost:9001",
	})
	if err != nil {
		log.Fatal(err)
	}

	limiter, err := ratelimit.NewLimiter(5, 1)
	if err != nil {
		log.Fatal(err)
	}

	handler := gateway.RateLimitMiddleware(limiter, router)

	log.Println("Sentinel gateway listening on :8080")

	log.Fatal(http.ListenAndServe(":8080", handler))
}
