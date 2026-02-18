# eBPF Build Pipeline (M0)

## CO-RE generation
Run on Linux host:

```bash
cd ebpf/bpf2go
go generate ./...
```

This flow will:
1. Generate `ebpf/headers/vmlinux.h` from host BTF.
2. Compile `ebpf/c/minimal.bpf.c`.
3. Emit Go bindings in `ebpf/bpf2go`.

## Smoke check
Use agent smoke mode:

```bash
go run ./cmd/agent --probe-smoke
```

This verifies that privileged eBPF map creation works.

## BCC fallback
Fallback scripts for non-BTF hosts are under `ebpf/bcc-fallback/` and currently cover:
- DNS latency
- TCP retransmits
