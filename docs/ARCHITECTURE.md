# Architecture

## Overview

The eBPF + LLM Inference SLO Toolkit is a Kubernetes-native observability system that captures kernel-level signals via eBPF probes and correlates them with OpenTelemetry traces to attribute SLO violations in LLM inference workloads to specific infrastructure fault domains.

The system operates as a three-stage pipeline:

1. **Collection** — eBPF probes capture 9 kernel signals (DNS latency, TCP retransmits, runqueue delay, connect latency, TLS handshake, CPU steal, memory reclaim latency, disk I/O latency, syscall latency) from each node.
2. **Correlation** — A tiered confidence model joins kernel signals to OTel spans using trace IDs, process identity, connection tuples, or service locality.
3. **Attribution** — Bayesian multi-fault classification computes posterior probabilities over 8 fault domains, producing ranked hypotheses with measurable confidence.

```
                    ┌──────────────────────────────────┐
                    │          KERNEL SPACE             │
                    │                                   │
                    │  kprobe/udp_sendmsg (DNS)         │
                    │  tracepoint/tcp_retransmit_skb    │
                    │  tracepoint/sched_switch (runq)   │
                    │  kprobe/tcp_v4_connect             │
                    │  kprobe/ssl_do_handshake (TLS)    │
                    │  /proc/stat polling (CPU steal)   │
                    │  tp/vmscan (mem reclaim)          │
                    │  tp/block (disk I/O)              │
                    │  kprobe/ksys_read|write (syscall) │
                    │                                   │
                    └─────────┬─────────────────────────┘
                              │ BPF ring buffer (mmap)
                              ▼
                    ┌──────────────────────────────────┐
                    │       llm-slo-agent (DaemonSet)  │
                    │                                   │
                    │  ProbeManager → RingBufReader     │
                    │  OverheadGuard + RateLimiter      │
                    │  ProbeEventV1 / SLOEvent emit     │
                    └─────────┬─────────────────────────┘
                              │ OTLP/HTTP, JSONL, stdout
                              ▼
          ┌───────────────────┼────────────────────────┐
          ▼                   ▼                         ▼
   OTel Collector      Correlation Engine        Collector CLI
   (Prometheus)        (4-tier join)             (normalize)
          │                   │                         │
          └───────────────────┼─────────────────────────┘
                              ▼
                    ┌──────────────────────────────────┐
                    │   Bayesian Attribution Engine     │
                    │  P(fault|signals) → hypotheses    │
                    │  confusion matrix + evidence      │
                    └─────────┬─────────────────────────┘
                              │
              ┌───────────┬───┼───────────┬──────────────┐
              ▼           ▼   ▼           ▼              ▼
         Grafana      Webhook  Incident   Benchmark   CD Gate
         (dashboards) (HMAC)   Reports    Artifacts   (Prometheus)
```

## Package Structure

### Command-Line Tools (`cmd/`)

| Binary | Description |
|--------|-------------|
| `agent` | Node-level eBPF probe collector. Runs as K8s DaemonSet. Emits SLO and probe events via OTLP, JSONL, or stdout. |
| `collector` | SLO event normalization pipeline. Accepts raw samples or generates synthetic data. |
| `attributor` | Fault-to-incident attribution. Classifies SLO violations into fault domains with confusion matrix output. |
| `benchgen` | Benchmark artifact generator. Produces reproducible test bundles with provenance. |
| `faultreplay` | Multi-domain fault scenario replayer for deterministic benchmark streams. |
| `faultinject` | Raw fault injection harness for controlled scenario testing. |
| `correlationeval` | Correlation quality gate evaluator. Validates precision/recall against labeled dataset. |
| `m5gate` | M5 GA gate enforcement. Evaluates B5 overhead, D3 variance, and E3 significance gates. |
| `sloctl` | CLI toolkit: `prereq check` validates kernel eBPF support; `cdgate check` enforces Prometheus-based SLO gates. |
| `loadgen` | Synthetic load generator for deterministic JSONL request traces. |
| `schemavalidate` | JSON schema contract validator for SLO, attribution, probe, and config schemas. |

### Library Packages (`pkg/`)

| Package | Purpose |
|---------|---------|
| `collector` | Core collection: synthetic sample generation, ring buffer consumer, probe manager, BCC fallback, kernel event decoding |
| `releasegate` | M5 gate calculations: overhead (B5), rerun variance (D3), Mann-Whitney + bootstrap CI + Cliff's delta (E3) |
| `signals` | Kernel signal models, capability modes, constants, deterministic generation |
| `otel` | OTLP/HTTP exporters for SLO and probe events |
| `otel/processor/ebpfcorrelator` | OTel correlator processor for signal-to-span enrichment using 4-tier confidence model |
| `correlation` | Confidence matching, retry storm detection, retrieval latency decomposition, quality evaluator |
| `benchmark` | Benchmark harness, artifact generation, report templating |
| `attribution` | Bayesian multi-fault attribution, confusion matrix, partial/coverage accuracy, rule-based mapper |
| `webhook` | HMAC-SHA256 signed webhook delivery with PagerDuty, Opsgenie, and generic payload formats |
| `cdgate` | Prometheus-based SLO gate evaluation (TTFT p95, error rate, burn rate) for CD pipelines |
| `safety` | Overhead guard, rate limiter, backpressure controls |
| `prereq` | Environment prerequisite checks (Go version, eBPF support, libbpf, kernel) |
| `schema` | JSON schema validator, v1 SLO/attribution types, v1alpha1 probe event types |
| `slo` | SLO burn-rate calculation, error budget math, TTFT and token metrics |
| `faultreplay` | Multi-domain fault scenario generation engine |
| `toolkitcfg` | Configuration YAML loader and defaults |
| `semconv` | Semantic conventions (`llm.ebpf.*` attribute names) |

## Key Types

### SLOEvent (`pkg/schema/types.go`)

The normalized event envelope emitted by the collector:

```go
type SLOEvent struct {
    EventID   string            `json:"event_id"`
    Timestamp time.Time         `json:"timestamp"`
    Cluster   string            `json:"cluster"`
    Namespace string            `json:"namespace"`
    Workload  string            `json:"workload"`
    Service   string            `json:"service"`
    RequestID string            `json:"request_id"`
    TraceID   string            `json:"trace_id,omitempty"`
    SLIName   string            `json:"sli_name"`    // ttft_ms, token_rate, etc.
    SLIValue  float64           `json:"sli_value"`
    Unit      string            `json:"unit"`
    Status    string            `json:"status"`       // ok, error, timeout
    Labels    map[string]string `json:"labels,omitempty"`
}
```

### ProbeEventV1 (`pkg/schema/types.go`)

The normalized probe envelope emitted by the node agent:

```go
type ProbeEventV1 struct {
    TSUnixNano int64      `json:"ts_unix_nano"`
    Signal     string     `json:"signal"`      // dns_latency_ms, tcp_retransmits_total, etc.
    Node       string     `json:"node"`
    Namespace  string     `json:"namespace"`
    Pod        string     `json:"pod"`
    Container  string     `json:"container"`
    PID        int        `json:"pid"`
    TID        int        `json:"tid"`
    ConnTuple  *ConnTuple `json:"conn_tuple,omitempty"`
    Value      float64    `json:"value"`
    Unit       string     `json:"unit"`
    Status     string     `json:"status"`
    TraceID    string     `json:"trace_id,omitempty"`
    SpanID     string     `json:"span_id,omitempty"`
    Errno      *int       `json:"errno,omitempty"`
    Confidence *float64   `json:"confidence,omitempty"`
}
```

### IncidentAttribution (`pkg/schema/types.go`)

The normalized attribution envelope:

```go
type IncidentAttribution struct {
    IncidentID           string            `json:"incident_id"`
    Timestamp            time.Time         `json:"timestamp"`
    Cluster              string            `json:"cluster"`
    Namespace            string            `json:"namespace,omitempty"`
    Service              string            `json:"service"`
    PredictedFaultDomain string            `json:"predicted_fault_domain"`
    Confidence           float64           `json:"confidence"`
    FaultHypotheses      []FaultHypothesis `json:"fault_hypotheses,omitempty"`
    Evidence             []Evidence        `json:"evidence"`
    SLOImpact            SLOImpact         `json:"slo_impact"`
    TraceIDs             []string          `json:"trace_ids,omitempty"`
    RequestIDs           []string          `json:"request_ids,omitempty"`
}
```

## eBPF Programs

All programs reside under `ebpf/c/` and use libbpf CO-RE (Compile Once, Run Everywhere). Events are emitted via `BPF_MAP_TYPE_RINGBUF` using a shared header (`llm_slo_event.h`).

| Program | Hook Type | Signal |
|---------|-----------|--------|
| `dns_latency.bpf.c` | kprobe/udp_sendmsg + kretprobe/udp_recvmsg | DNS resolution latency (ms) |
| `tcp_retransmit.bpf.c` | tracepoint/tcp/tcp_retransmit_skb | TCP packet retransmit count |
| `runqueue_delay.bpf.c` | tracepoint/sched/sched_switch | CPU scheduler runqueue delay (ns) |
| `connect_latency.bpf.c` | kprobe/tcp_v4_connect | TCP connection establishment time (ms) |
| `tls_handshake.bpf.c` | kprobe/ssl_do_handshake | TLS handshake duration (ms) |
| `cpu_steal.bpf.c` | /proc/stat polling (userspace) | Hypervisor CPU steal time (%) |
| `mem_reclaim.bpf.c` | tracepoint/vmscan/mm_vmscan_direct_reclaim_{begin,end} | Memory reclaim latency (ms) |
| `disk_io_latency.bpf.c` | tracepoint/block/block_rq_{issue,complete} | Block device I/O latency (ms) |
| `syscall_latency.bpf.c` | kprobe/kretprobe ksys_read + ksys_write | Read/write syscall latency (ms) |
| `minimal.bpf.c` | tracepoint/sys_enter_write | Minimal CO-RE validation probe |
| `hello_sys_enter_write.bpf.c` | tracepoint/sys_enter_write | Hello-world syscall counter for smoke tests |

### Common Event Structure

```c
struct llm_slo_event {
    __u32 pid;
    __u32 tid;
    __u64 timestamp_ns;
    __u32 signal_type;      // LLM_SLO_SIGNAL_DNS, LLM_SLO_SIGNAL_TCP_RETRANSMIT, etc.
    __u64 value_ns;
    __u16 conn_src_port;
    __u16 conn_dst_port;
    __u32 conn_dst_ip;
    __i32 errno_val;
};
```

### Kernel Compatibility

- **Core Full** (`core_full`): Kernel >= 5.8 with BTF. All 9 signals.
- **BCC Degraded** (`bcc_degraded`): Kernel >= 4.4. DNS + TCP retransmit only.
- Detection: agent checks `/sys/kernel/btf/vmlinux` at startup; `sloctl prereq check` provides manual verification.

## Correlation Engine

The correlation engine (`pkg/otel/processor/ebpfcorrelator/`) joins kernel probe signals to OTel spans using a 4-tier confidence model:

| Tier | Match Strategy | Confidence | Window |
|------|---------------|------------|--------|
| `trace_id_exact` | Exact trace_id match | 1.00 | configurable |
| `pod_pid_100ms` | Same pod + PID within 100ms | 0.90 | 100ms |
| `pod_conn_250ms` | Same pod + connection tuple within 250ms | 0.80 | 250ms |
| `svc_node_500ms` | Same service + node within 500ms | 0.65 | 500ms |

**Enrichment threshold**: Only correlations with confidence >= 0.70 enrich production spans. The 0.65 tier contributes diagnostic counters only.

**Fanout control**: Maximum 3 signals per span (sorted by confidence, then temporal proximity). Prevents correlation storms in high-signal environments.

**Retry storm detection**: Sliding-window burst detection per pod identifies retransmit storms that may indicate cascading failures.

**Retrieval decomposition**: DNS + connect + TLS latency decomposition mapped to `llm.ebpf.retrieval.kernel_attributed_ms`.

## Configuration

### Configuration File (`config/toolkit.yaml`)

```yaml
apiVersion: toolkit.llm-slo.dev/v1alpha1
kind: ToolkitConfig
signal_set:
  - dns_latency_ms
  - tcp_retransmits_total
  - runqueue_delay_ms
  - connect_latency_ms
  - tls_handshake_ms
  - cpu_steal_pct
  - mem_reclaim_latency_ms
  - disk_io_latency_ms
  - syscall_latency_ms
sampling:
  events_per_second_limit: 10000
  burst_limit: 20000
correlation:
  window_ms: 2000
otlp:
  endpoint: http://otel-collector:4317
safety:
  max_overhead_pct: 5
webhook:
  enabled: false
  url: ""
  secret: ""
  format: generic          # generic | pagerduty | opsgenie
  timeout_ms: 5000
cdgate:
  enabled: false
  prometheus_url: http://prometheus:9090
  ttft_p95_ms: 800
  error_rate: 0.05
  burn_rate: 3.0
```

Schema validation enforced by `config/toolkit.schema.json`. Configuration loads via `pkg/toolkitcfg` with CLI flag overrides.

## Deployment Topology

### Agent DaemonSet (`deploy/k8s/`)

The agent runs as a privileged DaemonSet on every node with:

- `hostPID: true` — access host `/proc` for CPU sampling
- `hostNetwork: true` — host network for DNS/TCP probes
- `dnsPolicy: ClusterFirstWithHostNet` — resolve cluster services from host network
- Capabilities: `BPF`, `SYS_ADMIN`, `SYS_RESOURCE`, `NET_ADMIN`
- Volume mounts: `/sys` (ro), `/proc` (ro), `/lib/modules` (ro), `/sys/fs/bpf` (rw), config (ro)

### Min-Capability Profile (`deploy/k8s/min-capability/`)

Kustomize overlay for environments that reject privileged pods:

- `privileged: false`, `allowPrivilegeEscalation: false`
- Capabilities reduced to `BPF`, `SYS_ADMIN`, `SYS_RESOURCE`
- `hostPID: false`, `hostNetwork: false`
- Signal set reduced to DNS + TCP retransmit only

### Observability Stack (`deploy/observability/`)

- **OTel Collector**: Receives OTLP/HTTP logs from agent, exports to Prometheus
- **Prometheus**: Scrapes agent metrics on port 2112, evaluates 5 alert rules
- **Grafana**: 17 panels across 3 dashboards (SLO Overview, Kernel Correlation, Incident Lab)
- **Tempo**: Distributed tracing backend

## CI/CD Pipeline

| Workflow | Trigger | Purpose |
|----------|---------|---------|
| `ci.yml` | Push/PR | Build, lint, test, schema validation, correlation gate, fault-injection smoke, helm lint, webhook smoke, CD gate smoke, multi-fault attribution test |
| `pr-privileged-ebpf-smoke.yml` | PR (labeled) | Privileged eBPF smoke on self-hosted runner |
| `nightly-ebpf-integration.yml` | Nightly schedule | Full eBPF integration in kind cluster |
| `weekly-benchmark.yml` | Weekly schedule | 7 scenarios (incl. mixed_multi) x 3 reruns with M5 gate evaluation |
| `e2e-evidence-report.yml` | Manual | Deterministic evidence bundle generation |
| `kernel-compatibility-matrix.yml` | Weekly | Multi-kernel compatibility probing |
| `release.yml` | Tag push (`v*`) | Cross-compiled binaries, container images, Helm chart OCI push, SBOM, cosign signing, provenance |
| `runner-health.yml` | Daily | Self-hosted runner health monitoring |

## Design Decisions

### 1. Multi-Tier Confidence Model

Single-strategy correlation fails in distributed systems where trace propagation is inconsistent. The 4-tier model gracefully degrades from exact trace_id match (1.0) through process-level (0.90) and connection-level (0.80) to service-level (0.65), with an enrichment threshold at 0.70 that prevents noisy low-confidence data from polluting production spans.

### 2. Overhead Guard with Signal Shedding

eBPF probes add measurable overhead. The agent enforces a hard CPU ceiling (3% GA, 5% dev) with automatic signal disabling. When overhead exceeds the budget, probes are disabled in cost order: TLS > runqueue > connect > CPU steal > DNS > TCP retransmit. This prevents the observability system from degrading the workloads it monitors.

### 3. Ring Buffer Event Delivery

`BPF_MAP_TYPE_RINGBUF` provides lock-free, FIFO, single-mmap event delivery from kernel to userspace. This avoids the per-CPU overhead of older perf buffers and provides natural backpressure — full buffers drop oldest events rather than blocking producers.

### 4. BCC Fallback for Older Kernels

Production clusters vary widely in kernel version. libbpf CO-RE requires kernel >= 5.8 with BTF. For kernels 4.4-5.7, a BCC fallback provides DNS + TCP retransmit signals. Auto-detection at startup ensures no deployment failures on older hosts.

### 5. M5 Statistical Release Gates

Four independent gates prevent regression. B5 (overhead) ensures the agent stays light. D3 (variance) ensures reproducibility. E3 (significance via Mann-Whitney + bootstrap CI + Cliff's delta) catches real TTFT regressions while ignoring noise. Baseline provenance tracking prevents comparing incompatible runs.

### 6. Schema Validation at Every Stage

JSON schema validation (`docs/contracts/v1/`, `docs/contracts/v1alpha1/`) runs at collection, correlation, and attribution. CI enforces `make schema-validate`. Schemas serve as the contract between pipeline stages and enable independent component evolution.

### 7. Bayesian Multi-Fault Attribution

Rule-based single-fault attribution breaks down when multiple faults co-occur (e.g., DNS latency + CPU throttling). The Bayesian engine computes P(fault|signals) = P(fault) × ∏P(signal_i|fault) / Z over all 8 fault domains simultaneously, returning ranked `FaultHypothesis` entries. This enables operators to see that an incident has, for example, 60% network_dns + 30% cpu_throttle rather than a single hard classification. Multi-fault evaluation uses `PartialAccuracy` (top-1 in expected set) and `CoverageAccuracy` (hypothesis coverage above threshold).

### 8. Webhook Exporter

Incident attributions are delivered to external systems via webhook with HMAC-SHA256 signing (`X-Webhook-Signature: sha256=...`). Three payload formats are supported: generic JSON (raw `IncidentAttribution`), PagerDuty Events API v2, and Opsgenie Alert API. Exponential backoff retry (3 attempts) handles transient failures; 4xx errors are non-retryable.

### 9. CD Gate

The CD gate (`sloctl cdgate check`) queries Prometheus instant API for TTFT p95, error rate, and burn rate. If any metric exceeds configured thresholds, the gate fails with exit code 1, blocking deployment. `--fail-open` allows degraded Prometheus to pass rather than block. This integrates with ArgoCD PreSync hooks, Flux health checks, and GitHub Actions CI steps.

### 10. Helm Chart

The Helm chart (`charts/llm-slo-agent/`) produces identical resources to the kustomize deployment but with full parameterization via `values.yaml`. All agent flags, signal sets, webhook, and CD gate configuration are templatable. OCI registry publication enables `helm install` from GHCR.
