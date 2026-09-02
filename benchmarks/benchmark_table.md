# Sentinel Benchmark Results

All numbers from actual runs on a MacBook. Not fabricated.

## Configuration

| Parameter | Value |
|-----------|-------|
| Capacity | 100 tokens |
| Refill rate | 100 tokens/s |
| Load | 1,000 req/s × 30s |
| k6 executor | `constant-arrival-rate` |

## Baseline: 1 Gateway vs 3 Gateways

| Metric | 1 Gateway | 3 Gateways |
|--------|-----------|------------|
| Total requests | 30,001 | 30,000 |
| RPS (sustained) | 1,000 | 1,000 |
| Admitted | 3,140 | 3,102 |
| Rejected (429) | 26,861 | 26,898 |
| **p50 latency** | **504 µs** | **589 µs** |
| p90 latency | 688 µs | 848 µs |
| p95 latency | 783 µs | 1.02 ms |
| avg latency | 784 µs | 804 µs |

> Adding Nginx + 2 additional gateways increases median latency by ~85 µs —
> the cost of an extra network hop through the Nginx reverse proxy layer.

## Concurrency Proof (no refill, capacity=100)

| Trial | Requests | Allowed | Over-admissions |
|-------|----------|---------|-----------------|
| 1 | 10,000 | 100 | 0 |
| 2 | 10,000 | 100 | 0 |
| 3 | 10,000 | 100 | 0 |
| 4 | 10,000 | 100 | 0 |
| 5 | 10,000 | 100 | 0 |

## Redis Failure Experiment (fail-closed)

| Phase | Duration | Expected | Observed |
|-------|----------|----------|----------|
| Normal (t=0–5s) | 5s | 200/429 | 200/429 ✅ |
| Redis paused (t=5–10s) | 5s | 500 | 500 ✅ |
| Recovered (t=10–20s) | 10s | 200/429 | 200/429 ✅ |

Totals: 1,963 × 200, 1,165 × 429, 512 × 500

Gateway logs during failure show: `error="redis limiter: i/o timeout" status=500`
