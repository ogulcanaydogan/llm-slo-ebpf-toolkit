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

# If a kind cluster already exists (e.g. nightly CI pre-deploys), skip cluster
# creation and manifest application — go straight to the smoke assertions.
if kind get clusters 2>/dev/null | grep -q "llm-slo-lab"; then
  echo "kind cluster 'llm-slo-lab' already exists, skipping setup"
else
  make kind-up
  kubectl apply -k deploy/observability
  kubectl apply -k deploy/k8s
  if [[ -n "${AGENT_IMAGE:-}" ]]; then
    kubectl -n llm-slo-system set image daemonset/llm-slo-agent "agent=${AGENT_IMAGE}"
  fi
fi

kubectl -n observability rollout status deployment/otel-collector --timeout=180s
kubectl -n observability rollout status deployment/prometheus --timeout=180s
kubectl -n observability rollout status deployment/grafana --timeout=180s
kubectl -n llm-slo-system rollout status daemonset/llm-slo-agent --timeout=180s

./scripts/chaos/set_agent_mode.sh mixed otlp

# Wait for the rolling restart triggered by set_agent_mode.sh to complete,
# then give agents time to emit events to the OTel collector.
kubectl -n llm-slo-system rollout status daemonset/llm-slo-agent --timeout=180s
kubectl -n llm-slo-system get pods -l app=llm-slo-agent
sleep 15
if ! kubectl -n observability logs deployment/otel-collector --tail=1000 | grep -E "signal=|sli=|llm-slo-ebpf-toolkit|LogRecord|service\.name" >/dev/null; then
  echo "otel collector did not log expected agent events"
  echo "--- collector logs (last 40 lines) ---"
  kubectl -n observability logs deployment/otel-collector --tail=40 || true
  exit 1
fi

AGENT_POD="$(kubectl -n llm-slo-system get pods -l app=llm-slo-agent -o jsonpath='{.items[0].metadata.name}')"
if [[ -z "$AGENT_POD" ]]; then
  echo "failed to resolve agent pod"
  exit 1
fi

METRICS_PATH="/api/v1/namespaces/llm-slo-system/pods/${AGENT_POD}:2112/proxy/metrics"
METRICS_RAW="$(kubectl get --raw "$METRICS_PATH")"
echo "$METRICS_RAW" | grep -q "llm_slo_agent_signal_enabled" || {
  echo "missing llm_slo_agent_signal_enabled metric"
  exit 1
}
echo "$METRICS_RAW" | grep -q "llm_slo_agent_capability_mode" || {
  echo "missing llm_slo_agent_capability_mode metric"
  exit 1
}
echo "$METRICS_RAW" | grep -q "llm_slo_agent_event_kind{kind=\"probe\"} 1" || {
  echo "agent is not in probe mode"
  exit 1
}

echo "kind observability smoke passed"
