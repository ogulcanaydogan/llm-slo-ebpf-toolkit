# eBPF Agent Threat Model

## Scope

This document covers the threat model for the eBPF agent DaemonSet component of the LLM SLO Toolkit. The agent runs with elevated privileges on Kubernetes worker nodes to attach kernel probes and collect system-level telemetry.

## System Context

The agent operates at the kernel/userspace boundary:

```
┌─────────────────────────────────────────────────┐
│                  Kubernetes Node                 │
│                                                  │
│  ┌─────────────────────────────────────────────┐ │
│  │           Kernel Space                       │ │
│  │  kprobes, tracepoints, ring buffer           │ │
│  │  /sys/fs/bpf, /proc, /sys                   │ │
│  └──────────┬──────────────────────────────────┘ │
│             │ BPF syscall + mmap                  │
│  ┌──────────▼──────────────────────────────────┐ │
│  │     llm-slo-agent container                  │ │
│  │  CAP_BPF + CAP_SYS_ADMIN + CAP_SYS_RESOURCE │ │
│  │  hostPID, hostNetwork                        │ │
│  └──────────┬──────────────────────────────────┘ │
│             │ OTLP/HTTP (port 4318)               │
│  ┌──────────▼──────────────────────────────────┐ │
│  │     OTel Collector (cluster-internal)        │ │
│  └─────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────┘
```

## Privilege Requirements

### Full Mode (`deploy/k8s/daemonset.yaml`)

| Privilege | Justification | Risk |
|-----------|--------------|------|
| `privileged: true` | Simplifies eBPF program loading and kernel access | Full host access; container escape equivalent |
| `CAP_BPF` | Load and manage eBPF programs (Linux >= 5.8) | Can read arbitrary kernel memory via BPF maps |
| `CAP_SYS_ADMIN` | Attach kprobes/tracepoints, manage BPF ring buffers, access `/sys/fs/bpf` | Broad capability; can modify kernel state |
| `CAP_SYS_RESOURCE` | Remove memory limits on eBPF maps, access BPF filesystem | Can exceed cgroup memory limits |
| `CAP_NET_ADMIN` | Network-specific probe attachment | Can modify network configuration |
| `hostPID: true` | Access host `/proc` for CPU sampling and PID-based correlation | Can see all host processes |
| `hostNetwork: true` | Observe host-level DNS/TCP traffic for probe collection | Can observe all host network traffic |

### Min-Capability Mode (`deploy/k8s/min-capability/`)

| Privilege | Change | Impact |
|-----------|--------|--------|
| `privileged: false` | Drop blanket privilege | Reduced attack surface |
| `allowPrivilegeEscalation: false` | Prevent escalation | Hardens container boundary |
| Capabilities: `BPF`, `SYS_ADMIN`, `SYS_RESOURCE` only | Drop `NET_ADMIN` | No network modification |
| `hostPID: false` | No host process visibility | Lose per-process correlation |
| `hostNetwork: false` | No host network access | Lose host-level DNS/TCP probes |
| Signal set: DNS + TCP retransmit only | 2 of 9 signals | Reduced attribution coverage |

## Threat Categories

### T1: Container Escape via BPF

**Risk**: An attacker who compromises the agent container could use BPF capabilities to read arbitrary kernel memory, attach malicious probes, or escape the container boundary.

**Mitigations**:
- Agent binary is statically compiled with no shell or package manager in the container image
- Container image is signed with cosign (keyless OIDC via GitHub Actions) and attestation-verified
- SBOM (syft SPDX) published with every release for vulnerability scanning
- Min-capability mode removes `privileged: true` and `allowPrivilegeEscalation: false`
- Resource limits enforced: 500m CPU, 512Mi memory

**Residual risk**: Medium. BPF + SYS_ADMIN capabilities inherently provide kernel access. Mitigated by image signing and reduced-privilege profiles.

### T2: Sensitive Data Exposure via Probes

**Risk**: eBPF probes observe kernel-level events including network traffic, process scheduling, and connection metadata. This could expose sensitive data (DNS queries, connection targets, process names).

**Mitigations**:
- Probes capture timing and count metrics only, not payload content
- DNS probe records latency in milliseconds, not query content
- TCP probe records retransmit counts, not packet data
- TLS probe records handshake duration, not cryptographic material
- Connection tuple (src/dst IP:port) is the most granular network data collected
- Events are exported via OTLP to cluster-internal OTel collector only

**Residual risk**: Low. Connection tuples reveal communication patterns but not content.

### T3: Resource Exhaustion (Denial of Service)

**Risk**: Runaway eBPF probes or ring buffer consumers could exhaust CPU or memory on the host node, degrading workloads.

**Mitigations**:
- **Overhead guard** (`pkg/safety/overhead_guard.go`): Monitors agent CPU via `/proc` sampling. Automatically disables probes when overhead exceeds budget (3% GA, 5% dev).
- **Rate limiter** (`pkg/safety/rate_limiter.go`): Hard cap at 10,000 events/second with 20,000 burst. Excess events dropped with counter metric.
- **Signal shedding order**: Probes disabled in cost order (TLS > runqueue > connect > CPU steal > DNS > TCP retransmit) to preserve highest-value signals.
- **Resource limits**: K8s resource requests (100m CPU, 128Mi) and limits (500m CPU, 512Mi) enforced.
- **M5 B5 gate**: Weekly CI benchmark verifies overhead stays <= 3%.

**Residual risk**: Low. Multiple independent safety layers with automatic degradation.

### T4: Supply Chain Compromise

**Risk**: A compromised build or dependency could inject malicious code into the agent binary or container image.

**Mitigations**:
- Container images built in GitHub Actions with pinned action versions
- Keyless cosign signing via GitHub Actions OIDC (no long-lived signing keys)
- Build provenance attestation via `actions/attest-build-provenance@v2`
- SBOM generated with syft in SPDX format for every release
- SHA-256 checksums published for all release binaries
- Go module checksums verified via `go.sum`

**Residual risk**: Low. Standard supply chain hardening with cryptographic verification.

### T5: Lateral Movement from CI Runner

**Risk**: Self-hosted CI runners with privileged access could be compromised to attack production infrastructure.

**Mitigations**:
- **Ephemeral instances**: One job per runner lifecycle; instance destroyed after completion
- **Network isolation**: Egress TCP/443 only (HTTPS to GitHub and package mirrors). No inbound rules.
- **No SSH**: SSM-only management, no SSH keys or ports
- **Credential scoping**: PAT stored in AWS SSM Parameter Store (SecureString). IAM role limited to `ssm:GetParameter` on single path.
- **Encrypted storage**: 80GB gp3 root volume with encryption at rest
- **No production credentials**: Runner has no access to production clusters or secrets

**Residual risk**: Low. Air-gapped from production with no lateral movement path.

### T6: Privilege Escalation via Volume Mounts

**Risk**: Agent mounts host filesystems (`/sys`, `/proc`, `/lib/modules`, `/sys/fs/bpf`) which could be used for privilege escalation.

**Mitigations**:
- `/sys`, `/proc`, `/lib/modules` mounted read-only
- Only `/sys/fs/bpf` is read-write (required for eBPF map pinning and ring buffer creation)
- Min-capability mode does not change volume mounts but removes `privileged: true`
- Agent code only reads from `/proc/stat` (CPU steal) and `/sys/kernel/btf/vmlinux` (BTF detection)

**Residual risk**: Medium. Write access to `/sys/fs/bpf` allows eBPF map manipulation. Inherent to eBPF operation.

## Attack Surface Summary

| Surface | Full Mode | Min-Capability Mode |
|---------|-----------|-------------------|
| Kernel memory (via BPF) | Exposed | Exposed (reduced signal set) |
| Host processes (via hostPID) | Visible | Not visible |
| Host network (via hostNetwork) | Observable | Not observable |
| Network modification (NET_ADMIN) | Possible | Not possible |
| Privilege escalation | Possible (privileged) | Blocked (allowPrivilegeEscalation: false) |
| Host filesystem write | /sys/fs/bpf only | /sys/fs/bpf only |

## Recommendations

1. **Production**: Use min-capability mode unless full signal coverage is required. Accept the reduced 2-signal set for lower attack surface.
2. **Staging/Lab**: Use full privileged mode for complete 9-signal coverage and incident lab exercises.
3. **Image verification**: Always verify cosign signatures before deploying: `cosign verify --certificate-identity-regexp='.*' --certificate-oidc-issuer='https://token.actions.githubusercontent.com' ghcr.io/ogulcanaydogan/llm-slo-ebpf-toolkit-agent:v0.2.0`
4. **Network policy**: Apply Kubernetes NetworkPolicy to restrict agent egress to OTel collector only.
5. **RBAC**: Agent ServiceAccount has read-only access (list nodes, get configmaps, read pod metadata). Do not extend.
6. **Monitoring**: Track `llm_slo_agent_cpu_overhead_pct` and `llm_slo_agent_dropped_events_total` to detect anomalous agent behavior.
