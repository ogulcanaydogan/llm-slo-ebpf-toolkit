# Completed Work

This document tracks all implemented milestones, artifacts, and deliverables for the eBPF + LLM Inference SLO Toolkit through v0.2.0 GA.

## Release History

| Version | Date | Type | Key Deliverables |
|---------|------|------|-----------------|
| v0.1.0-alpha.1 | 2026-02-17 | Alpha | Repository init, collector, attributor, benchgen, CI |
| v0.1.0-alpha.2 | 2026-02-17 | Alpha | Attribution pipeline, confusion matrices, JSONL loader |
| v0.1.0-alpha.3 | 2026-02-17 | Alpha | Mixed-fault scenarios, fixture-driven benchmarks |
| v0.1.0-alpha.4 | 2026-02-18 | Alpha | K8s DaemonSet, faultreplay, faultinject, streaming mode |
| v0.1.0-alpha.5 | 2026-02-18 | Alpha | OTLP exporter, observability stack, kind smoke tests |
| v0.2.0-rc.1 | 2026-02-18 | RC | 6 CO-RE probes, correlation engine, incident lab, M5 gates, release pipeline |
| v0.2.0-rc.2 | 2026-02-19 | RC | CI stabilization, OTLP hardening, DNS fix, PR smoke workflow |
| v0.2.0-rc.3 | 2026-02-19 | RC | Kind startup hardening, runner health workflow |
| v0.2.0 | 2026-02-19 | GA | Container images, e2e evidence, min-capability, real chaos, kernel compat |

## Milestone Completion

### M0: Bootstrap (Complete)

- Go module with CI build/lint/unit pipeline
- kind 3-node cluster bootstrap
- DaemonSet heartbeat from agent to collector
- `make build` and `make test` green on clean environment

### M1: Demo + OTel (Complete)

- OTel Collector, Prometheus, Tempo, Grafana deployed via kustomize
- 17 Grafana panels across 3 dashboards:
  - SLO Overview: TTFT distribution, token throughput, error rate, SLO burn
  - Kernel Correlation: Signal heatmap, confidence distribution, retry storms
  - Incident Lab: Fault timeline, attribution accuracy, overhead tracking
- RAG demo service with TTFT, tokens/sec, and trace_id telemetry
- OTLP/HTTP log exporter for SLO and probe events
- Observability smoke test in kind cluster

### M2: Signals v1 (Complete)

- 6 CO-RE eBPF probes implemented:
  - `dns_latency.bpf.c` — kprobe/udp_sendmsg
  - `tcp_retransmit.bpf.c` — tracepoint/tcp_retransmit_skb
  - `runqueue_delay.bpf.c` — tracepoint/sched_switch
  - `connect_latency.bpf.c` — kprobe/tcp_v4_connect
  - `tls_handshake.bpf.c` — kprobe/ssl_do_handshake
  - `cpu_steal.bpf.c` — /proc/stat polling
- Shared ring buffer event format (`llm_slo_event.h`)
- Go ring buffer consumer (`pkg/collector/ringbuf.go`)
- Probe manager with lifecycle control and per-signal disable
- BCC fallback for non-BTF kernels (DNS + TCP retransmit)
- Overhead guard with automatic signal shedding
- Rate limiter (10,000 EPS default, 20,000 burst)
- Overhead measured at 2.20% (below 5% dev gate)

### M3: Correlation (Complete)

- 4-tier confidence model:
  - `trace_id_exact`: 1.00
  - `pod_pid_100ms`: 0.90
  - `pod_conn_250ms`: 0.80
  - `svc_node_500ms`: 0.65
- Enrichment threshold: 0.70
- Max join fanout: 1:3
- Retry storm detector with sliding-window burst detection
- Retrieval latency decomposition (DNS + connect + TLS)
- Quality gate: P=1.00, R=1.00 on 55 labeled pairs
- `llm.ebpf.retrieval.kernel_attributed_ms` semconv attribute
- `llm.ebpf.tcp.retry_storm` semconv attribute
- `llm.ebpf.cpu.cfs_throttled_ms` semconv attribute

### M4: Incident Lab (Complete)

- 6 deterministic fault scenarios:
  - `dns_latency`, `cpu_throttle`, `provider_throttle`
  - `memory_pressure`, `network_partition`, `mixed`
- Declarative scenario YAML definitions
- Optional real injectors: DNS delay (tc netem), retransmit/loss, CPU stress (stress-ng)
- Synthetic fallback for environments without root
- 5 Prometheus alerting rules:
  - TTFT budget burn rate
  - Error rate threshold
  - Correlation degraded
  - Agent heartbeat stale
  - Overhead high

### M5: Benchmark + Release (Complete)

- M5 gate tool (`cmd/m5gate`, `pkg/releasegate`):
  - B5: Collector overhead <= 3%
  - D3: Rerun variance <= 10%
  - E3: Mann-Whitney + bootstrap CI + Cliff's delta
- Weekly benchmark workflow: 6 scenarios x 3 reruns
- Release pipeline with cross-compiled binaries (linux-amd64, darwin-arm64)
- SHA-256 checksums, SBOM (syft SPDX), provenance JSON
- Container images published to GHCR with cosign signing and provenance attestations
- E2E evidence report workflow for auditable test bundles

## Artifact Inventory

### Source Code

| Category | Count |
|----------|-------|
| Go source files | 73 |
| Go test files | 24 |
| eBPF C programs | 8 |
| Total Go lines (non-test) | ~5,700 |
| Total test lines | ~2,500 |
| Total eBPF C lines | ~550 |

### Command-Line Tools

10 binaries cross-compiled for linux-amd64 and darwin-arm64: `agent`, `collector`, `attributor`, `benchgen`, `faultreplay`, `faultinject`, `correlationeval`, `m5gate`, `sloctl`, `loadgen`.

### Library Packages

14 packages under `pkg/`: `collector`, `releasegate`, `signals`, `otel`, `correlation`, `benchmark`, `attribution`, `safety`, `prereq`, `schema`, `slo`, `faultreplay`, `toolkitcfg`, `semconv`.

### Kubernetes Manifests

~30 YAML files under `deploy/`:
- Agent DaemonSet stack (7 resources + min-capability overlay)
- Observability stack (OTel Collector, Prometheus, Tempo, Grafana)
- 4 Grafana dashboard ConfigMaps (17 panels)
- 5 Prometheus alert rules
- kind cluster config

### CI/CD Workflows

8 GitHub Actions workflows: `ci.yml`, `pr-privileged-ebpf-smoke.yml`, `nightly-ebpf-integration.yml`, `weekly-benchmark.yml`, `e2e-evidence-report.yml`, `kernel-compatibility-matrix.yml`, `release.yml`, `runner-health.yml`.

### Infrastructure

- AWS ephemeral runner: Terraform + cloud-init (t3a.xlarge, 80GB encrypted gp3, SSM-managed, HTTPS-only egress)
- Runner preflight detection with automatic synthetic fallback

### Documentation

~28 markdown files across:
- Strategy: differentiation, build plan, go-no-go checklist, demo stories
- Research: landscape sources with 10 competitor references
- Contracts: v1 SLO event schema, v1 incident attribution schema, v1alpha1 probe event schema
- Benchmarks: experimental design, output schema
- Security: min-capability mode, self-hosted runner baseline, threat model
- Releases: v0.2.0 GA notes, v0.2.0-rc.1 notes
- Architecture: full system architecture document
- Compatibility: kernel compatibility matrix

### Container Images

- `ghcr.io/ogulcanaydogan/llm-slo-ebpf-toolkit-agent` — eBPF agent DaemonSet
- `ghcr.io/ogulcanaydogan/llm-slo-ebpf-toolkit-rag-service` — RAG demo service
- Keyless cosign signing and provenance attestations via GitHub Actions OIDC

## Go-No-Go Gate Status (v0.2.0 GA)

All 23 gates across 6 sections PASS or ENFORCED:

| Section | Gates | Status |
|---------|-------|--------|
| A. Contract/API | A1-A4 | All PASS |
| B. Signal/Runtime | B1-B4 PASS, B5 ENFORCED | All PASS/ENFORCED |
| C. Correlation | C1-C5 | All PASS |
| D. Reproducibility | D1-D2 PASS, D3 ENFORCED, D4 PASS | All PASS/ENFORCED |
| E. Statistics | E1-E2 PASS, E3 ENFORCED, E4 PASS | All PASS/ENFORCED |
| F. CI/CD/Security | F1-F5 | All PASS |

## Git History

- 37 commits on main branch
- 13 merged pull requests
- 4 release tags (rc.1, rc.2, rc.3, GA)
- 3 RC burn-in cycles with 6+ weekly benchmark passes
