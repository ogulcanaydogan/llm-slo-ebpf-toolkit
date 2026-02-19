#!/usr/bin/env bash
set -euo pipefail

# Bootstraps one ephemeral GitHub runner host.
# Expected env vars:
#   GITHUB_REPOSITORY=owner/repo
#   RUNNER_PAT_PARAMETER_NAME=/path/to/ssm/param
# Optional env vars:
#   RUNNER_VERSION=2.323.0
#   RUNNER_HOME=/opt/actions-runner
#   RUNNER_DEFAULT_LABELS=self-hosted,linux,ebpf
#   RUNNER_EXTRA_LABELS=kernel-6-8

if ! command -v aws >/dev/null 2>&1; then
  echo "aws cli is required"
  exit 1
fi

GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-}"
RUNNER_PAT_PARAMETER_NAME="${RUNNER_PAT_PARAMETER_NAME:-/llm-slo/github/runner_pat}"
RUNNER_VERSION="${RUNNER_VERSION:-2.323.0}"
RUNNER_HOME="${RUNNER_HOME:-/opt/actions-runner}"
RUNNER_PAT="${RUNNER_PAT:-}"

if [[ -z "$GITHUB_REPOSITORY" ]]; then
  echo "GITHUB_REPOSITORY is required (owner/repo)"
  exit 1
fi

mkdir -p "$RUNNER_HOME"
cd "$RUNNER_HOME"

if [[ ! -x ./config.sh ]]; then
  arch="x64"
  os="linux"
  pkg="actions-runner-${os}-${arch}-${RUNNER_VERSION}.tar.gz"
  url="https://github.com/actions/runner/releases/download/v${RUNNER_VERSION}/${pkg}"
  echo "downloading actions runner ${RUNNER_VERSION}"
  curl -fsSL "$url" -o "$pkg"
  tar xzf "$pkg"
fi

echo "installing runner dependencies"
if [[ -x ./bin/installdependencies.sh ]]; then
  sudo ./bin/installdependencies.sh || true
fi

json_token() {
  python3 -c 'import json,sys; print((json.load(sys.stdin).get("token") or "").strip())'
}

PAT="$RUNNER_PAT"
if [[ -z "$PAT" ]]; then
  PAT="$(aws ssm get-parameter --name "$RUNNER_PAT_PARAMETER_NAME" --with-decryption --query 'Parameter.Value' --output text)"
fi
if [[ -z "$PAT" || "$PAT" == "None" ]]; then
  echo "failed to fetch PAT (set RUNNER_PAT or SSM parameter: $RUNNER_PAT_PARAMETER_NAME)"
  exit 1
fi

TOKEN="$(curl -fsSL -X POST \
  -H "Accept: application/vnd.github+json" \
  -H "Authorization: Bearer ${PAT}" \
  "https://api.github.com/repos/${GITHUB_REPOSITORY}/actions/runners/registration-token" | json_token)"

if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
  echo "failed to acquire registration token"
  exit 1
fi

echo "bootstrap done (token acquired). use register-and-run-loop.sh for service runtime"
