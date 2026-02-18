#!/usr/bin/env bash
set -euo pipefail

CLUSTER_NAME="${CLUSTER_NAME:-llm-slo-lab}"

if ! command -v kind >/dev/null 2>&1; then
  echo "kind is not installed." >&2
  exit 1
fi

if kind get clusters | grep -Fxq "$CLUSTER_NAME"; then
  kind delete cluster --name "$CLUSTER_NAME"
  echo "kind cluster '$CLUSTER_NAME' deleted"
else
  echo "kind cluster '$CLUSTER_NAME' does not exist"
fi
