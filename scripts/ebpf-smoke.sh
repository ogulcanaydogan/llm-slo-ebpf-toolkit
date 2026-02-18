#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ "$(uname -s | tr '[:upper:]' '[:lower:]')" != "linux" ]]; then
  echo "ebpf smoke requires Linux" >&2
  exit 1
fi

if [[ "$(id -u)" -ne 0 ]]; then
  echo "ebpf smoke requires root privileges" >&2
  exit 1
fi

cd "$ROOT_DIR"
go run ./cmd/agent --probe-smoke

OBJ_FILE="$(find "$ROOT_DIR/ebpf/bpf2go" -maxdepth 1 -type f \( -name "*_bpfel.o" -o -name "*_bpfeb.o" \) | head -n 1 || true)"
PIN_PATH="/sys/fs/bpf/llm_slo_smoke"

if [[ -n "$OBJ_FILE" ]] && command -v bpftool >/dev/null 2>&1; then
  rm -rf "$PIN_PATH" || true
  mkdir -p "$PIN_PATH"
  bpftool prog loadall "$OBJ_FILE" "$PIN_PATH"
  rm -rf "$PIN_PATH"
  echo "loaded and unloaded probe object: $OBJ_FILE"
else
  echo "skipping bpftool load/unload smoke (missing generated object or bpftool)"
fi
