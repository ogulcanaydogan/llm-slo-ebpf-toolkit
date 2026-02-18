#!/usr/bin/env bash
set -euo pipefail

NAMESPACE="${NAMESPACE:-llm-slo-system}"
DAEMONSET="${DAEMONSET:-llm-slo-agent}"
SCENARIO="${1:-mixed}"
OUTPUT_MODE="${2:-stdout}"
EVENT_KIND="${EVENT_KIND:-probe}"
SAMPLE_COUNT="${SAMPLE_COUNT:-0}"
SAMPLE_INTERVAL_MS="${SAMPLE_INTERVAL_MS:-1000}"
OTLP_ENDPOINT="${OTLP_ENDPOINT:-http://otel-collector.observability.svc.cluster.local:4318/v1/logs}"
OTLP_TIMEOUT_MS="${OTLP_TIMEOUT_MS:-5000}"
CAPABILITY_MODE="${CAPABILITY_MODE:-auto}"
DISABLE_SIGNALS="${DISABLE_SIGNALS:-}"
DISABLE_OVERHEAD_GUARD="${DISABLE_OVERHEAD_GUARD:-false}"

if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is required"
  exit 1
fi

kubectl -n "$NAMESPACE" set env daemonset/"$DAEMONSET" \
  CHAOS_SCENARIO="$SCENARIO" \
  EVENT_KIND="$EVENT_KIND" \
  OUTPUT_MODE="$OUTPUT_MODE" \
  SAMPLE_COUNT="$SAMPLE_COUNT" \
  SAMPLE_INTERVAL_MS="$SAMPLE_INTERVAL_MS" \
  OTLP_ENDPOINT="$OTLP_ENDPOINT" \
  OTLP_TIMEOUT_MS="$OTLP_TIMEOUT_MS" \
  CAPABILITY_MODE="$CAPABILITY_MODE" \
  DISABLE_SIGNALS="$DISABLE_SIGNALS" \
  DISABLE_OVERHEAD_GUARD="$DISABLE_OVERHEAD_GUARD"

kubectl -n "$NAMESPACE" rollout status daemonset/"$DAEMONSET" --timeout=180s
kubectl -n "$NAMESPACE" get pods -l app="$DAEMONSET"

echo "updated $DAEMONSET scenario=$SCENARIO output=$OUTPUT_MODE event_kind=$EVENT_KIND capability=$CAPABILITY_MODE"
