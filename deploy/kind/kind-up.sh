#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLUSTER_NAME="${CLUSTER_NAME:-llm-slo-lab}"
KIND_CONFIG="${KIND_CONFIG:-$ROOT_DIR/deploy/kind/kind-config.yaml}"

if ! command -v kind >/dev/null 2>&1; then
  echo "kind is not installed. Install kind first." >&2
  exit 1
fi
if ! command -v kubectl >/dev/null 2>&1; then
  echo "kubectl is not installed. Install kubectl first." >&2
  exit 1
fi

if kind get clusters | grep -Fxq "$CLUSTER_NAME"; then
  echo "kind cluster '$CLUSTER_NAME' already exists"
else
  kind create cluster --name "$CLUSTER_NAME" --config "$KIND_CONFIG" --wait 120s
fi

kubectl cluster-info --context "kind-$CLUSTER_NAME" >/dev/null
kubectl get nodes --context "kind-$CLUSTER_NAME"
echo "kind cluster '$CLUSTER_NAME' is ready"
