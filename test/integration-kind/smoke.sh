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
kubectl -n llm-slo-system get daemonset llm-slo-agent
make collector-smoke
