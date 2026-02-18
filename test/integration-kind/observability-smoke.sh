#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ "$(uname -s | tr '[:upper:]' '[:lower:]')" != "linux" ]]; then
  echo "kind observability smoke skipped: linux required"
  exit 0
fi

for tool in kind kubectl; do
  if ! command -v "$tool" >/dev/null 2>&1; then
    echo "kind observability smoke skipped: $tool not installed"
    exit 0
  fi
done

cd "$ROOT_DIR"
make kind-up
kubectl apply -k deploy/observability
kubectl apply -k deploy/k8s

kubectl -n observability rollout status deployment/otel-collector --timeout=180s
kubectl -n observability rollout status deployment/prometheus --timeout=180s
kubectl -n observability rollout status deployment/grafana --timeout=180s
kubectl -n llm-slo-system rollout status daemonset/llm-slo-agent --timeout=180s

./scripts/chaos/set_agent_mode.sh mixed otlp

sleep 8
if ! kubectl -n observability logs deployment/otel-collector --tail=400 | grep -E "sli=|llm-slo-ebpf-toolkit" >/dev/null; then
  echo "otel collector did not log expected SLO events"
  exit 1
fi

echo "kind observability smoke passed"
