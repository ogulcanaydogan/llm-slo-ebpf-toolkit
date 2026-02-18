#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ "$(uname -s | tr '[:upper:]' '[:lower:]')" != "linux" ]]; then
  echo "kind integration smoke skipped: linux required"
  exit 0
fi

if ! command -v kind >/dev/null 2>&1; then
  echo "kind integration smoke skipped: kind not installed"
  exit 0
fi

cd "$ROOT_DIR"
make kind-up
kubectl apply -k deploy/k8s
kubectl -n llm-slo-system rollout status daemonset/llm-slo-agent --timeout=180s

AGENT_POD="$(kubectl -n llm-slo-system get pods -l app=llm-slo-agent -o jsonpath='{.items[0].metadata.name}')"
if [[ -z "$AGENT_POD" ]]; then
  echo "failed to resolve agent pod"
  exit 1
fi

METRICS_PATH="/api/v1/namespaces/llm-slo-system/pods/${AGENT_POD}:2112/proxy/metrics"
METRICS_RAW="$(kubectl get --raw "$METRICS_PATH")"
echo "$METRICS_RAW" | grep -q "llm_slo_agent_event_kind{kind=\"probe\"} 1" || {
  echo "expected probe mode metric on agent"
  exit 1
}
echo "$METRICS_RAW" | grep -q "llm_slo_agent_signal_enabled" || {
  echo "expected signal toggle metrics on agent"
  exit 1
}

make collector-smoke
