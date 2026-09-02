# Sentinel

A distributed rate-limiting API gateway built with Go, Redis, and Lua.

Sentinel enforces per-client rate limits across multiple gateway instances using an atomic token bucket algorithm. It was designed to solve a specific problem: when you horizontally scale your API gateway, each instance's in-memory rate limiter becomes independent, allowing clients to exceed their limit by distributing requests across instances.

## Architecture

```
                    Client
                      │
                      ▼
                    Nginx
                 /    |    \
                ▼     ▼     ▼
              GW1   GW2   GW3       ← independent Go processes
                \     |     /
                 \    |    /
                   Redis             ← shared state
                     │
                  Lua script         ← atomic decision
```

All gateway instances connect to one Redis instance. Each rate-limit decision (read → refill → check → consume → write) executes inside a single Lua script, which Redis runs atomically. This eliminates the race condition inherent in a naive GET/SET approach.

## Algorithm: Token Bucket

Each client gets a bucket with:
- **Capacity** — maximum burst size (e.g. 100 tokens)
- **Refill rate** — tokens added per second (e.g. 100/s)

When a request arrives:
1. Calculate tokens accrued since last refill
2. Cap at capacity
3. If tokens ≥ 1: consume one, allow the request
4. If tokens < 1: reject with `429 Too Many Requests` and `Retry-After` header

The same algorithm runs in two implementations:
- **In-memory** (`sync.Mutex`-based) — for single-process deployments
- **Redis + Lua** — for distributed deployments

Both satisfy the same `RateLimiter` interface:

```go
type RateLimiter interface {
    Allow(ctx context.Context, clientKey string) (bool, time.Duration, error)
}
```

## Why Lua?

A naive Redis approach is broken:

```
Gateway 1: GET count → 4        Gateway 2: GET count → 4
Gateway 1: count < 5? yes       Gateway 2: count < 5? yes
Gateway 1: SET count 5 ✅       Gateway 2: SET count 5 ✅
```

Both gateways admit a request, but only one token remained. This is a [TOCTOU](https://en.wikipedia.org/wiki/Time-of-check_to_time-of-use) race.

Redis executes Lua scripts atomically — no other command can interleave. The entire token bucket operation (read state → calculate refill → check → consume → write state) runs as one indivisible unit.

## Concurrency Proof

Sentinel was tested with 3 gateway instances behind Nginx, sharing a single Redis-backed Lua token bucket. The bucket was configured with **capacity=100 and no refill**, then exactly 10,000 requests were generated per trial using k6.

| Trial | Requests | Allowed | GW1 | GW2 | GW3 | Over-admissions |
|-------|----------|---------|-----|-----|-----|-----------------|
| 1     | 10,000   | 100     | 34  | 34  | 32  | **0** |
| 2     | 10,000   | 100     | 36  | 32  | 32  | **0** |
| 3     | 10,000   | 100     | 36  | 29  | 35  | **0** |
| 4     | 10,000   | 100     | 34  | 33  | 33  | **0** |
| 5     | 10,000   | 100     | 32  | 33  | 35  | **0** |

Across 5 trials (50,000 requests), exactly 100 were admitted per trial with zero over-admissions. Per-gateway counts confirm traffic was distributed across all three instances. Admission counts come from Redis counters incremented atomically inside the Lua script — the same execution that made the admission decision.

## Benchmarks

All numbers from actual runs. Raw k6 output is saved in `benchmarks/`.

**Configuration:** capacity=100, refill=100/s, 1000 req/s for 30 seconds.

| Metric | 1 Gateway | 3 Gateways + Nginx |
|--------|-----------|---------------------|
| Total requests | 30,001 | 30,000 |
| Sustained RPS | 1,000 | 1,000 |
| p50 latency | 504 µs | 589 µs |
| p90 latency | 688 µs | 848 µs |
| p95 latency | 783 µs | 1.02 ms |

Adding Nginx and 2 additional gateways increases median latency by ~85 µs — the cost of an extra network hop.

## Failure Handling

**Policy: fail-closed.** When Redis is unreachable, the gateway returns `500 Internal Server Error` instead of silently admitting traffic without rate-limit verification.

Redis timeouts are bounded:
- Dial: 2 seconds
- Read: 1 second
- Write: 1 second

**Tested by pausing Redis mid-traffic:**

| Phase | Status codes |
|-------|-------------|
| Normal (t=0–5s) | 200 / 429 |
| Redis paused (t=5–10s) | 500 |
| Recovered (t=10–20s) | 200 / 429 |

Gateway logs during failure: `error="redis limiter: i/o timeout" status=500`

Recovery is automatic — no restart required. When Redis becomes reachable again, the next request succeeds normally.

## Quick Start

```bash
# Start the full stack
docker compose up --build -d

# Test it
curl -i http://localhost:8080/api/test

# Run the concurrency proof
./benchmark.sh 5

# Run baseline benchmarks
./benchmark_baseline.sh

# Run Redis failure test
./benchmark_redis_failure.sh
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `GATEWAY_ID` | `gateway` | Instance identifier for logging and proof counters |
| `REDIS_ADDR` | `localhost:6379` | Redis connection address |
| `UPSTREAM_ADDR` | `http://localhost:9001` | Backend to proxy to |
| `RATE_CAPACITY` | `10` | Token bucket capacity |
| `RATE_REFILL` | `2` | Tokens added per second |

## Project Structure

```
sentinel/
├── gateway/
│   ├── cmd/
│   │   ├── gateway/main.go          # Gateway entrypoint
│   │   └── mockbackend/main.go      # Test backend
│   ├── internal/
│   │   ├── gateway/
│   │   │   ├── router.go            # Prefix-based reverse proxy
│   │   │   └── middleware.go         # Rate-limit middleware (fail-closed)
│   │   └── ratelimit/
│   │       ├── interface.go          # RateLimiter interface
│   │       ├── bucket.go             # In-memory token bucket
│   │       ├── limiter.go            # Per-client bucket manager
│   │       ├── redis_limiter.go      # Redis + Lua implementation
│   │       └── token_bucket.lua      # Atomic Lua script
│   ├── Dockerfile                    # Multi-stage gateway image
│   └── Dockerfile.mockbackend
├── nginx/
│   ├── nginx.conf                    # 3-gateway upstream
│   └── nginx.1gw.conf               # 1-gateway (benchmarking)
├── docker-compose.yml                # Full 3-gateway stack
├── docker-compose.1gw.yml            # Single-gateway stack
├── load-tests/k6/                    # k6 test scripts
├── benchmarks/                       # Raw results from actual runs
├── benchmark.sh                      # Concurrency proof runner
├── benchmark_baseline.sh             # 1-GW vs 3-GW benchmark
└── benchmark_redis_failure.sh        # Redis failure experiment
```

## Limitations

- **Single Redis instance.** No Redis Cluster or Sentinel (Redis HA) support. A Redis failure affects all gateway instances simultaneously.
- **No Redis Cluster Lua compatibility.** The Lua script accesses keys not declared in `KEYS[]` (proof counters). This works on single-instance Redis but would need adjustment for Redis Cluster.
- **IP-based client identification.** Clients are identified by IP address. Behind a proxy, all clients appear as one. API key-based identification would be needed for production.
- **No circuit breaker.** The gateway retries Redis on every request during failure. A circuit breaker would reduce load on a struggling Redis.
- **No metrics export.** No Prometheus/Grafana integration. Observability relies on structured logs.
- **Nginx-specific.** Load balancing depends on Nginx round-robin. Not tested with other load balancers.

## Future Work

- Redis Sentinel or Cluster for high availability
- API key-based client identification
- Circuit breaker pattern for Redis failures
- Prometheus metrics (`/metrics` endpoint)
- Per-route rate limits (different limits for different API paths)
- Sliding window counter as an alternative algorithm

## License

MIT
