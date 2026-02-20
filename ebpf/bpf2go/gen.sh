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

BPF2GO="go run github.com/cilium/ebpf/cmd/bpf2go@v0.16.0"
CFLAGS="-O2 -g -Wall -I../headers -I../c"

cd "$ROOT_DIR/ebpf/bpf2go"

# Minimal smoke probe (build test only)
$BPF2GO -cc clang -cflags "$CFLAGS" Minimal ../c/minimal.bpf.c

# Signal probes
$BPF2GO -cc clang -cflags "$CFLAGS" DNSLatency ../c/dns_latency.bpf.c
$BPF2GO -cc clang -cflags "$CFLAGS" TCPRetransmit ../c/tcp_retransmit.bpf.c
$BPF2GO -cc clang -cflags "$CFLAGS" RunqueueDelay ../c/runqueue_delay.bpf.c
$BPF2GO -cc clang -cflags "$CFLAGS" ConnectLatency ../c/connect_latency.bpf.c
$BPF2GO -cc clang -cflags "$CFLAGS" TLSHandshake ../c/tls_handshake.bpf.c
$BPF2GO -cc clang -cflags "$CFLAGS" CPUSteal ../c/cpu_steal.bpf.c
$BPF2GO -cc clang -cflags "$CFLAGS" MemReclaim ../c/mem_reclaim.bpf.c
$BPF2GO -cc clang -cflags "$CFLAGS" DiskIOLatency ../c/disk_io_latency.bpf.c
$BPF2GO -cc clang -cflags "$CFLAGS" SyscallLatency ../c/syscall_latency.bpf.c
$BPF2GO -cc clang -cflags "$CFLAGS" HelloSysEnterWrite ../c/hello_sys_enter_write.bpf.c

echo "generated CO-RE bindings for 11 programs in ebpf/bpf2go"
