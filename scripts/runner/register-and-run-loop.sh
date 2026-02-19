#!/usr/bin/env bash
set -euo pipefail

# Registers and runs an ephemeral GitHub Actions runner in a loop.
# Expected env vars:
#   GITHUB_REPOSITORY=owner/repo
#   RUNNER_PAT_PARAMETER_NAME=/llm-slo/github/runner_pat
# Optional env vars:
#   RUNNER_HOME=/opt/actions-runner
#   RUNNER_NAME_PREFIX=llm-slo-ebpf
#   RUNNER_DEFAULT_LABELS=self-hosted,linux,ebpf
#   RUNNER_EXTRA_LABELS=kernel-6-8
#   APPEND_KERNEL_VERSION_LABEL=true

GITHUB_REPOSITORY="${GITHUB_REPOSITORY:-}"
RUNNER_PAT_PARAMETER_NAME="${RUNNER_PAT_PARAMETER_NAME:-/llm-slo/github/runner_pat}"
RUNNER_HOME="${RUNNER_HOME:-/opt/actions-runner}"
RUNNER_NAME_PREFIX="${RUNNER_NAME_PREFIX:-llm-slo-ebpf}"
RUNNER_DEFAULT_LABELS="${RUNNER_DEFAULT_LABELS:-self-hosted,linux,ebpf}"
RUNNER_EXTRA_LABELS="${RUNNER_EXTRA_LABELS:-}"
APPEND_KERNEL_VERSION_LABEL="${APPEND_KERNEL_VERSION_LABEL:-true}"
RUNNER_PAT="${RUNNER_PAT:-}"

if [[ -z "$GITHUB_REPOSITORY" ]]; then
  echo "GITHUB_REPOSITORY is required (owner/repo)"
  exit 1
fi

cd "$RUNNER_HOME"

json_token() {
  python3 -c 'import json,sys; print((json.load(sys.stdin).get("token") or "").strip())'
}

fetch_pat() {
  if [[ -n "$RUNNER_PAT" ]]; then
    echo "$RUNNER_PAT"
    return 0
  fi
  if command -v aws >/dev/null 2>&1; then
    aws ssm get-parameter --name "$RUNNER_PAT_PARAMETER_NAME" --with-decryption --query 'Parameter.Value' --output text 2>/dev/null || true
    return 0
  fi
  return 0
}

fetch_token() {
  local pat="$1"
  curl -fsSL -X POST \
    -H "Accept: application/vnd.github+json" \
    -H "Authorization: Bearer ${pat}" \
    "https://api.github.com/repos/${GITHUB_REPOSITORY}/actions/runners/registration-token" | json_token
}

kernel_label() {
  local release major minor
  release="$(uname -r 2>/dev/null || true)"
  major="$(awk -F. '{print $1}' <<<"$release")"
  minor="$(awk -F. '{print $2}' <<<"$release")"
  if [[ "$major" =~ ^[0-9]+$ && "$minor" =~ ^[0-9]+$ ]]; then
    echo "kernel-${major}-${minor}"
  fi
}

build_labels() {
  local labels extra auto
  labels="${RUNNER_DEFAULT_LABELS:-self-hosted,linux,ebpf}"
  extra="${RUNNER_EXTRA_LABELS:-}"
  if [[ -n "$extra" ]]; then
    labels="${labels},${extra}"
  fi
  if [[ "${APPEND_KERNEL_VERSION_LABEL}" == "true" ]]; then
    auto="$(kernel_label)"
    if [[ -n "$auto" ]]; then
      labels="${labels},${auto}"
    fi
  fi
  tr '[:upper:]' '[:lower:]' <<<"$labels" \
    | awk -v RS=',' '{
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", $0);
        if (length($0) && !seen[$0]++) {
          out = out (out ? "," : "") $0
        }
      } END { print out }'
}

cleanup() {
  set +e
  if [[ -f .runner ]]; then
    PAT="$(fetch_pat)"
    if [[ -n "$PAT" && "$PAT" != "None" ]]; then
      REMOVE_TOKEN="$(curl -fsSL -X POST \
        -H "Accept: application/vnd.github+json" \
        -H "Authorization: Bearer ${PAT}" \
        "https://api.github.com/repos/${GITHUB_REPOSITORY}/actions/runners/remove-token" | json_token 2>/dev/null)"
      if [[ -n "$REMOVE_TOKEN" && "$REMOVE_TOKEN" != "null" ]]; then
        ./config.sh remove --token "$REMOVE_TOKEN" >/dev/null 2>&1 || true
      fi
    fi
  fi
}
trap cleanup EXIT INT TERM

while true; do
  PAT="$(fetch_pat)"
  if [[ -z "$PAT" || "$PAT" == "None" ]]; then
    echo "unable to fetch PAT (set RUNNER_PAT or configure aws ssm access)"
    sleep 15
    continue
  fi

  TOKEN="$(fetch_token "$PAT")"

  if [[ -z "$TOKEN" || "$TOKEN" == "null" ]]; then
    echo "unable to acquire registration token"
    sleep 15
    continue
  fi

  RUNNER_NAME="${RUNNER_NAME_PREFIX}-$(hostname)-$(date +%s)"
  RUNNER_LABELS="$(build_labels)"

  ./config.sh \
    --url "https://github.com/${GITHUB_REPOSITORY}" \
    --token "$TOKEN" \
    --name "$RUNNER_NAME" \
    --no-default-labels \
    --labels "$RUNNER_LABELS" \
    --work "_work" \
    --ephemeral \
    --unattended \
    --replace

  echo "runner registered: ${RUNNER_NAME}; waiting for one job"
  ./run.sh || true

  # Ephemeral runner auto-removes itself after one job, ensure local state is clean.
  ./config.sh remove --token "$TOKEN" >/dev/null 2>&1 || true
  rm -f .runner .credentials .credentials_rsaparams

  echo "runner loop complete; re-registering in 5s"
  sleep 5
done
