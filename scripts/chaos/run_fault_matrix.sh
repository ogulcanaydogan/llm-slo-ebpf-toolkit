#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
OUT_DIR="${OUT_DIR:-$ROOT_DIR/artifacts/benchmarks-matrix}"
COUNT="${COUNT:-24}"
SCENARIOS=(provider_throttle dns_latency cpu_throttle memory_pressure network_partition mixed)

mkdir -p "$OUT_DIR"

for scenario in "${SCENARIOS[@]}"; do
  scenario_dir="$OUT_DIR/$scenario"
  mkdir -p "$scenario_dir"

  echo "==> scenario: $scenario"
  go run "$ROOT_DIR/cmd/faultinject" \
    --scenario "$scenario" \
    --count "$COUNT" \
    --out "$scenario_dir/raw_samples.jsonl"

  go run "$ROOT_DIR/cmd/collector" \
    --input "$scenario_dir/raw_samples.jsonl" \
    --output jsonl \
    --output-path "$scenario_dir/slo_events.jsonl"

  go run "$ROOT_DIR/cmd/faultreplay" \
    --scenario "$scenario" \
    --count "$COUNT" \
    --out "$scenario_dir/fault_samples.jsonl"

  bench_scenario="$scenario"
  if [[ "$scenario" == "mixed" ]]; then
    bench_scenario="mixed_faults"
  fi

  go run "$ROOT_DIR/cmd/benchgen" \
    --out "$scenario_dir" \
    --scenario "$bench_scenario" \
    --input "$scenario_dir/fault_samples.jsonl"

done

echo "matrix artifacts created under $OUT_DIR"
