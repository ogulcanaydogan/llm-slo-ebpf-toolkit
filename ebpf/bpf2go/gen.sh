#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ "$(uname -s | tr '[:upper:]' '[:lower:]')" != "linux" ]]; then
  echo "ebpf generation must run on Linux with BTF support." >&2
  exit 1
fi

for bin in bpftool clang go; do
  if ! command -v "$bin" >/dev/null 2>&1; then
    echo "missing dependency: $bin" >&2
    exit 1
  fi
done

mkdir -p "$ROOT_DIR/ebpf/headers"
if [[ -e /sys/kernel/btf/vmlinux ]]; then
  bpftool btf dump file /sys/kernel/btf/vmlinux format c > "$ROOT_DIR/ebpf/headers/vmlinux.h"
else
  echo "missing /sys/kernel/btf/vmlinux" >&2
  exit 1
fi

cd "$ROOT_DIR/ebpf/bpf2go"
go run github.com/cilium/ebpf/cmd/bpf2go@v0.16.0 \
  -cc clang \
  -cflags "-O2 -g -Wall -I../headers" \
  Minimal \
  ../c/minimal.bpf.c

echo "generated CO-RE bindings in ebpf/bpf2go"
