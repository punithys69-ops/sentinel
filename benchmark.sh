#!/usr/bin/env bash
# benchmark.sh — Run the concurrency proof experiment.
#
# Usage:
#   ./benchmark.sh [trials]     (default: 5)
#
# Prerequisites:
#   - docker compose stack running (RATE_CAPACITY=100, RATE_REFILL=0)
#   - k6 installed
#   - redis-cli available

set -euo pipefail

TRIALS=${1:-5}
RESULTS_DIR="benchmarks"
mkdir -p "$RESULTS_DIR"

echo "=============================================="
echo " Sentinel Concurrency Proof"
echo " Trials:    $TRIALS"
echo " Requests:  10,000 per trial"
echo " VUs:       200"
echo " Capacity:  100 (no refill)"
echo "=============================================="
echo ""

summary_file="$RESULTS_DIR/concurrency_summary.json"
echo "[" > "$summary_file"

for trial in $(seq 1 "$TRIALS"); do
    echo "──────────────────────────────────────────────"
    echo " Trial $trial / $TRIALS"
    echo "──────────────────────────────────────────────"

    # 1. Flush all rate-limit and proof keys.
    redis-cli FLUSHDB > /dev/null 2>&1

    # 2. Wait a beat for Redis to settle.
    sleep 1

    # 3. Run k6.
    output_file="$RESULTS_DIR/concurrency_trial_${trial}.txt"
    k6 run --quiet load-tests/k6/concurrency_proof.js 2>&1 | tee "$output_file"

    # 4. Read authoritative admission count from Redis.
    proof_total=$(redis-cli GET proof:allowed 2>/dev/null || echo "0")
    proof_gw1=$(redis-cli GET proof:allowed:gateway1 2>/dev/null || echo "0")
    proof_gw2=$(redis-cli GET proof:allowed:gateway2 2>/dev/null || echo "0")
    proof_gw3=$(redis-cli GET proof:allowed:gateway3 2>/dev/null || echo "0")

    # Handle nil responses from Redis.
    [ "$proof_total" = "" ] || [ "$proof_total" = "(nil)" ] && proof_total=0
    [ "$proof_gw1" = "" ]   || [ "$proof_gw1" = "(nil)" ]   && proof_gw1=0
    [ "$proof_gw2" = "" ]   || [ "$proof_gw2" = "(nil)" ]   && proof_gw2=0
    [ "$proof_gw3" = "" ]   || [ "$proof_gw3" = "(nil)" ]   && proof_gw3=0

    over_admissions=$((proof_total - 100))
    [ "$over_admissions" -lt 0 ] && over_admissions=0

    echo ""
    echo " Redis proof counters:"
    echo "   total allowed : $proof_total"
    echo "   gateway1      : $proof_gw1"
    echo "   gateway2      : $proof_gw2"
    echo "   gateway3      : $proof_gw3"
    echo "   over-admissions: $over_admissions"
    echo ""

    # 5. Append to JSON summary.
    comma=""
    [ "$trial" -gt 1 ] && comma=","
    cat >> "$summary_file" <<EOF
${comma}{
  "trial": $trial,
  "requests": 10000,
  "allowed": $proof_total,
  "rejected": $((10000 - proof_total)),
  "gateway1": $proof_gw1,
  "gateway2": $proof_gw2,
  "gateway3": $proof_gw3,
  "over_admissions": $over_admissions
}
EOF

    sleep 2
done

echo "]" >> "$summary_file"

echo ""
echo "=============================================="
echo " All $TRIALS trials complete."
echo " Results: $RESULTS_DIR/"
echo " Summary: $summary_file"
echo "=============================================="
echo ""
cat "$summary_file"
