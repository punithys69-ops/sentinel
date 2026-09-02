#!/usr/bin/env bash
# benchmark_baseline.sh — Run 1-GW and 3-GW baseline benchmarks.
#
# Usage: ./benchmark_baseline.sh
#
# Prerequisites: docker compose, k6, redis-cli

set -euo pipefail

RESULTS_DIR="benchmarks"
mkdir -p "$RESULTS_DIR"

echo "=============================================="
echo " Sentinel Baseline Benchmarks"
echo " 1000 req/s × 30s per experiment"
echo "=============================================="

# ─── 1-GATEWAY ─────────────────────────────────────────────────────────────
echo ""
echo "──── Experiment 1: Single Gateway ────"
echo ""

docker compose -f docker-compose.1gw.yml down --remove-orphans 2>/dev/null || true
docker compose down --remove-orphans 2>/dev/null || true

RATE_CAPACITY=100 RATE_REFILL=100 docker compose -f docker-compose.1gw.yml up -d --build 2>&1 | tail -5
sleep 5

echo "Verifying stack..."
curl -sf http://localhost:8080/api/test > /dev/null && echo "Stack OK" || { echo "Stack not ready"; exit 1; }

redis-cli FLUSHDB > /dev/null 2>&1
sleep 1

echo "Running 1-GW benchmark..."
k6 run load-tests/k6/baseline_bench.js 2>&1 | tee "$RESULTS_DIR/results_1gw.txt"

docker compose -f docker-compose.1gw.yml down --remove-orphans 2>&1 | tail -1

# ─── 3-GATEWAY ─────────────────────────────────────────────────────────────
echo ""
echo "──── Experiment 2: Three Gateways ────"
echo ""

RATE_CAPACITY=100 RATE_REFILL=100 docker compose up -d --build 2>&1 | tail -5
sleep 5

echo "Verifying stack..."
curl -sf http://localhost:8080/api/test > /dev/null && echo "Stack OK" || { echo "Stack not ready"; exit 1; }

redis-cli FLUSHDB > /dev/null 2>&1
sleep 1

echo "Running 3-GW benchmark..."
k6 run load-tests/k6/baseline_bench.js 2>&1 | tee "$RESULTS_DIR/results_3gw.txt"

docker compose down --remove-orphans 2>&1 | tail -1

echo ""
echo "=============================================="
echo " Done. Results saved to:"
echo "   $RESULTS_DIR/results_1gw.txt"
echo "   $RESULTS_DIR/results_3gw.txt"
echo "=============================================="
