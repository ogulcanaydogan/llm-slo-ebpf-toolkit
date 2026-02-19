#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<EOF
Usage: $0 [--profiles csv] [--out path]

Environment:
  RUNNER_STATUS_TOKEN or GITHUB_TOKEN  GitHub token with actions:read
                                      (optional when gh CLI is authenticated)
  GITHUB_REPOSITORY                    owner/repo
  RUNNER_BASE_LABELS                   baseline label csv (default: self-hosted,linux,ebpf)
EOF
}

profiles_csv="kernel-5-15,kernel-6-8"
out_file=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --profiles)
      profiles_csv="${2:-}"
      shift 2
      ;;
    --out)
      out_file="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

api_token="${RUNNER_STATUS_TOKEN:-${GITHUB_TOKEN:-}}"
if [[ -z "${GITHUB_REPOSITORY:-}" ]]; then
  echo "GITHUB_REPOSITORY is required" >&2
  exit 1
fi

base_labels_csv="${RUNNER_BASE_LABELS:-self-hosted,linux,ebpf}"
IFS=',' read -r -a profiles <<<"$profiles_csv"
IFS=',' read -r -a base_labels <<<"$base_labels_csv"

for i in "${!profiles[@]}"; do
  profiles[$i]="$(echo "${profiles[$i]}" | tr '[:upper:]' '[:lower:]' | xargs)"
done
for i in "${!base_labels[@]}"; do
  base_labels[$i]="$(echo "${base_labels[$i]}" | tr '[:upper:]' '[:lower:]' | xargs)"
done

api_url="https://api.github.com/repos/${GITHUB_REPOSITORY}/actions/runners?per_page=100"
tmp_body="$(mktemp)"
http_code=""
reason="api_not_attempted"
base_online_count=0
profiles_json='{}'

for profile in "${profiles[@]}"; do
  profiles_json="$(jq \
    --arg profile "$profile" \
    '. + {($profile): {count: 0, available: false}}' <<<"$profiles_json")"
done

if [[ -n "$api_token" ]]; then
  http_code="$(curl -sS -o "$tmp_body" -w "%{http_code}" \
    -H "Authorization: Bearer ${api_token}" \
    -H "Accept: application/vnd.github+json" \
    "$api_url" || true)"
  if [[ "$http_code" == "200" ]]; then
    reason="api_success"
    body="$(cat "$tmp_body")"
  else
    reason="api_http_${http_code:-error}"
    body=""
  fi
elif command -v gh >/dev/null 2>&1; then
  if gh api "repos/${GITHUB_REPOSITORY}/actions/runners?per_page=100" > "$tmp_body" 2>/dev/null; then
    reason="gh_api_success"
    body="$(cat "$tmp_body")"
  else
    reason="gh_api_error"
    body=""
  fi
else
  reason="missing_token_and_gh_auth"
  body=""
fi

if [[ -n "$body" ]]; then
  base_filter='
    select(.status == "online")
  '
  for base_label in "${base_labels[@]}"; do
    base_filter="${base_filter} | select(any(.labels[]?; (.name | ascii_downcase) == \"${base_label}\"))"
  done

  base_online_count="$(jq "[.runners[] | ${base_filter}] | length" <<<"$body")"

  for profile in "${profiles[@]}"; do
    count="$(jq "[.runners[] | ${base_filter} | select(any(.labels[]?; (.name | ascii_downcase) == \"${profile}\"))] | length" <<<"$body")"
    profiles_json="$(jq \
      --arg profile "$profile" \
      --argjson count "$count" \
      '. + {($profile): {count: $count, available: ($count > 0)}}' <<<"$profiles_json")"
  done
fi
rm -f "$tmp_body"

result_json="$(jq -n \
  --arg repository "$GITHUB_REPOSITORY" \
  --arg timestamp "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  --arg reason "$reason" \
  --argjson total_online "$base_online_count" \
  --argjson profiles "$profiles_json" \
  --argjson base_labels "$(printf '%s\n' "${base_labels[@]}" | jq -R . | jq -s .)" \
  '{
    repository: $repository,
    timestamp_utc: $timestamp,
    reason: $reason,
    base_labels: $base_labels,
    total_online_ebpf_runners: $total_online,
    profiles: $profiles
  }')"

echo "$result_json"

if [[ -n "$out_file" ]]; then
  mkdir -p "$(dirname "$out_file")"
  echo "$result_json" > "$out_file"
fi

if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  {
    echo "reason=$reason"
    echo "total_online_ebpf_runners=$base_online_count"
    for profile in "${profiles[@]}"; do
      safe_key="$(echo "$profile" | tr '-' '_')"
      count="$(jq -r --arg p "$profile" '.profiles[$p].count // 0' <<<"$result_json")"
      echo "count_${safe_key}=$count"
    done
  } >> "$GITHUB_OUTPUT"
fi
