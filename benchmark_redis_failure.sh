#!/usr/bin/env bash
# benchmark_redis_failure.sh — Prove fail-closed behavior when Redis dies.
#
# Timeline:
#   t=0s   k6 starts (200 req/s for 20s)
#   t=5s   Redis paused (simulating network partition)
#   t=10s  Redis unpaused (recovery)
#   t=20s  k6 ends
#
# Expected behavior:
#   t=0-5s    200/429 responses (normal)
#   t=5-10s   500 responses (fail-closed)
#   t=10-20s  200/429 responses (recovered)

set -euo pipefail

RESULTS_DIR="benchmarks"
mkdir -p "$RESULTS_DIR"

echo "=============================================="
echo " Redis Failure Experiment (fail-closed)"
echo " 200 req/s × 20s"
echo " Redis paused at t+5s, unpaused at t+10s"
echo "=============================================="

# Ensure a clean 3-GW stack.
docker compose down --remove-orphans 2>/dev/null || true
RATE_CAPACITY=100 RATE_REFILL=100 docker compose up -d --build 2>&1 | tail -5
sleep 5

echo "Verifying stack..."
curl -sf http://localhost:8080/api/test > /dev/null && echo "Stack OK" || { echo "Stack not ready"; exit 1; }

redis-cli FLUSHDB > /dev/null 2>&1
sleep 1

# Start k6 in the background.
echo ""
echo "Starting k6 (200 req/s for 20s)..."
k6 run load-tests/k6/redis_failure.js > "$RESULTS_DIR/redis_failure.txt" 2>&1 &
K6_PID=$!

# Wait 5 seconds, then pause Redis.
sleep 5
echo "[t+5s]  Pausing Redis..."
docker compose pause redis 2>&1

# Wait 5 seconds, then unpause Redis.
sleep 5
echo "[t+10s] Unpausing Redis..."
docker compose unpause redis 2>&1

# Wait for k6 to finish.
echo "Waiting for k6 to complete..."
wait $K6_PID || true

echo ""
echo "──── Results ────"
cat "$RESULTS_DIR/redis_failure.txt"

echo ""
echo "=============================================="
echo " Saved to: $RESULTS_DIR/redis_failure.txt"
echo "=============================================="

# Show gateway error logs during the failure window.
echo ""
echo "──── Gateway error logs ────"
docker compose logs gateway1 gateway2 gateway3 2>&1 | grep -i "error" | tail -20
