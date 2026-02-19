#!/usr/bin/env bash
set -euo pipefail

INPUT_DIR="${1:-artifacts/compatibility}"
OUT_FILE="${2:-docs/compatibility.md}"
RUN_ID="${RUN_ID:-manual}"

mkdir -p "$(dirname "$OUT_FILE")"

read_field() {
  local file="$1"
  local expr="$2"
  local value
  if [[ ! -f "$file" ]]; then
    echo "n/a"
    return 0
  fi
  value="$(jq -r "$expr" "$file" 2>/dev/null || true)"
  if [[ -z "$value" || "$value" == "null" ]]; then
    echo "n/a"
    return 0
  fi
  echo "$value"
}

render_row() {
  local label="$1"
  local file="$2"
  local status="$3"
  local kernel="$4"
  local btf="$5"
  local prereq="$6"
  local probe="$7"
  printf '| `%s` | %s | `%s` | `%s` | `%s` | `%s` |\n' "$label" "$status" "$kernel" "$btf" "$prereq" "$probe"
}

K515="$INPUT_DIR/kernel-5-15.json"
K68="$INPUT_DIR/kernel-6-8.json"

K515_STATUS="$(read_field "$K515" '.status // "available"')"
K515_KERNEL="$(read_field "$K515" '.kernel_release')"
K515_BTF="$(read_field "$K515" '.btf_available')"
K515_PREREQ="$(read_field "$K515" '.prereq.status')"
K515_PROBE="$(read_field "$K515" '.probe_smoke.status')"

K68_STATUS="$(read_field "$K68" '.status // "available"')"
K68_KERNEL="$(read_field "$K68" '.kernel_release')"
K68_BTF="$(read_field "$K68" '.btf_available')"
K68_PREREQ="$(read_field "$K68" '.prereq.status')"
K68_PROBE="$(read_field "$K68" '.probe_smoke.status')"

GENERATED_AT="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

cat > "$OUT_FILE" <<EOF
# Kernel Compatibility Matrix

This page tracks compatibility checks for privileged eBPF execution across supported runner kernel profiles.

- Generated at (UTC): ${GENERATED_AT}
- Source run: \`${RUN_ID}\`
- Report source directory: \`${INPUT_DIR}\`

## Matrix

| Profile Label | Availability | Kernel Release | BTF | \`sloctl prereq\` | \`agent --probe-smoke\` |
|---|---|---|---|---|---|
EOF

render_row "kernel-5-15" "$K515" "$K515_STATUS" "$K515_KERNEL" "$K515_BTF" "$K515_PREREQ" "$K515_PROBE" >> "$OUT_FILE"
render_row "kernel-6-8" "$K68" "$K68_STATUS" "$K68_KERNEL" "$K68_BTF" "$K68_PREREQ" "$K68_PROBE" >> "$OUT_FILE"

cat >> "$OUT_FILE" <<'EOF'

## Interpretation

- `available`: matrix job ran on a runner matching the profile label.
- `unavailable`: no online runner with the requested label was detected in preflight.
- `prereq.status=pass`: local kernel/tooling/capability checks passed for that runner.
- `probe_smoke.status=pass`: probe loader smoke succeeded (or `skipped` when root privileges were unavailable).

## Notes

- These checks are intended as compatibility signals, not full performance regressions.
- Full SLO/perf and incident reproducibility gates remain in weekly benchmark workflows.
EOF
