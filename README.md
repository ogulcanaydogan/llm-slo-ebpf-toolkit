# LLM SLO eBPF Toolkit

**Kernel-grounded observability for LLM reliability engineering.**

A Kubernetes-native toolkit that uses eBPF to capture kernel-level signals — DNS latency, TCP retransmits, scheduling delays, connection timing, TLS handshakes, and CPU steal — and correlates them with OpenTelemetry traces and Kubernetes workload identity to produce causal incident attributions for LLM service-level objectives.

Production LLM systems fail in ways that application instrumentation alone cannot explain. When a retrieval-augmented generation service violates its time-to-first-token SLO, the root cause may be DNS resolution delays, provider throttling, noisy-neighbor CPU contention, or network packet loss — none of which are visible in application traces. This toolkit closes that observability gap by fusing kernel telemetry with application context, enabling SRE teams to attribute SLO violations to specific fault domains with measurable confidence.

## Problem

LLM inference workloads on Kubernetes exhibit failure modes that cross multiple infrastructure layers simultaneously. A single user-facing latency spike can originate from any layer — and often multiple layers interact:

```mermaid
graph TD
    USER["User Request<br/>(TTFT SLO: 800ms)"]

    subgraph INFRA["Infrastructure Layers Where Failures Hide"]
        direction TB
        NET["Network Layer<br/>DNS delays · TCP retransmits · TLS overhead"]
        COMPUTE["Compute Layer<br/>CPU throttling · runqueue contention · memory pressure"]
        PROVIDER["Provider Layer<br/>HTTP 429 rate limits · 5xx capacity errors"]
        RETRIEVAL["Retrieval Layer<br/>Vector DB latency · embedding service degradation"]
    end

    USER --> NET
    USER --> COMPUTE
    USER --> PROVIDER
    USER --> RETRIEVAL

    NET -->|"invisible to<br/>app traces"| BLIND["Observability<br/>Blind Spot"]
    COMPUTE -->|"invisible to<br/>app traces"| BLIND
    PROVIDER -->|"partially visible"| APP["App-Level<br/>Tracing"]
    RETRIEVAL -->|"partially visible"| APP

    BLIND -->|"this toolkit<br/>closes the gap"| SOLUTION["Kernel-Grounded<br/>Attribution"]
    APP --> SOLUTION

    style BLIND fill:#dc3545,color:#fff,stroke:#dc3545
    style SOLUTION fill:#198754,color:#fff,stroke:#198754
    style USER fill:#0d6efd,color:#fff,stroke:#0d6efd
```

Existing observability tools address subsets of this problem. Application-level tracing captures request flow but misses kernel-level causality. Network-focused eBPF tools (Cilium/Hubble, Pixie) provide infrastructure visibility but lack LLM-specific SLI semantics. Profiling tools (Parca, Elastic) measure resource consumption without linking it to SLO burn behaviour.

No existing open-source tool combines kernel-grounded signal collection with LLM-native SLI decomposition and causal attribution in a single pipeline.

## Approach

The toolkit operates as a three-stage pipeline deployed alongside LLM workloads:

```mermaid
graph LR
    subgraph S1["Stage 1: Collection"]
        EBPF["eBPF Probes<br/>6 kernel signals"]
    end

    subgraph S2["Stage 2: Correlation"]
        CORR["Correlation Engine<br/>confidence ≥ 0.70"]
    end

    subgraph S3["Stage 3: Attribution"]
        ATTR["Attribution Engine<br/>fault-domain classification"]
    end

    KERNEL["Linux Kernel<br/>tracepoints · kprobes"] --> EBPF
    OTEL["OTel Spans<br/>+ K8s Identity"] --> CORR
    EBPF -->|"raw signals"| CORR
    CORR -->|"enriched events"| ATTR
    ATTR --> OUT1["SLO Events (v1)"]
    ATTR --> OUT2["Incident Reports"]
    ATTR --> OUT3["Confusion Matrix"]

    style S1 fill:#e8f4f8,stroke:#0d6efd
    style S2 fill:#fff3cd,stroke:#ffc107
    style S3 fill:#d1e7dd,stroke:#198754
    style KERNEL fill:#6c757d,color:#fff,stroke:#6c757d
```

### Stage 1: Kernel Signal Collection

Six eBPF programs attach to kernel tracepoints and kprobes using libbpf CO-RE (Compile Once, Run Everywhere) for portability across kernel versions. A BCC fallback path supports older hosts without BTF (BPF Type Format) support. Signals collected:

| Signal | Source | LLM Relevance |
|---|---|---|
| DNS latency | `kprobe/udp_sendmsg` | Retrieval backend and provider endpoint resolution |
| TCP retransmits | `tracepoint/tcp/tcp_retransmit_skb` | Network-layer contribution to TTFT degradation |
| Runqueue delay | `tracepoint/sched/sched_switch` | CPU contention from noisy neighbours |
| Connect latency | `kprobe/tcp_v4_connect` | Provider API connection overhead |
| TLS handshake time | `kprobe/ssl_do_handshake` | Encryption cost in provider communication |
| CPU steal | `/proc/stat` polling | Hypervisor-level resource contention |

The agent runs as a Kubernetes DaemonSet with configurable sampling and a safety governor that enforces a hard CPU overhead ceiling (development: 5%, production: 3%).

```mermaid
graph TB
    subgraph USERSPACE["User Space"]
        AGENT["eBPF Agent<br/>(Go, DaemonSet)"]
        SAFETY["Safety Governor<br/>overhead ≤ 3% CPU"]
        MAPS["eBPF Maps<br/>(ring buffer)"]
    end

    subgraph KERNELSPACE["Kernel Space"]
        DNS_PROBE["kprobe: udp_sendmsg<br/>→ DNS latency"]
        TCP_PROBE["tracepoint: tcp_retransmit_skb<br/>→ TCP retransmits"]
        SCHED_PROBE["tracepoint: sched_switch<br/>→ runqueue delay"]
        CONN_PROBE["kprobe: tcp_v4_connect<br/>→ connect latency"]
        TLS_PROBE["kprobe: ssl_do_handshake<br/>→ TLS handshake"]
    end

    subgraph HARDWARE["Hardware / Hypervisor"]
        PROC["/proc/stat polling<br/>→ CPU steal %"]
    end

    DNS_PROBE --> MAPS
    TCP_PROBE --> MAPS
    SCHED_PROBE --> MAPS
    CONN_PROBE --> MAPS
    TLS_PROBE --> MAPS
    PROC --> AGENT

    MAPS --> AGENT
    AGENT --> SAFETY
    SAFETY -->|"within budget"| OUTPUT["→ Correlation Engine"]
    SAFETY -->|"over budget"| SHED["→ Shed events<br/>(backpressure)"]

    style KERNELSPACE fill:#f8d7da,stroke:#dc3545
    style USERSPACE fill:#d1e7dd,stroke:#198754
    style HARDWARE fill:#e2e3e5,stroke:#6c757d
    style SAFETY fill:#fff3cd,stroke:#ffc107
```

### Stage 2: Correlation Engine

Collected kernel signals are joined with OpenTelemetry spans using a tiered confidence model:

| Tier | Join Key | Window | Confidence |
|---|---|---|---|
| Exact | `trace_id` | In-window | 1.00 |
| Process | `pod` + `pid` | ≤ 100 ms | 0.90 |
| Connection | `pod` + `conn_tuple` | ≤ 250 ms | 0.80 |
| Service | `service` + `node` | ≤ 500 ms | 0.65 |

Only correlations at confidence ≥ 0.70 enrich spans. The 0.65 tier contributes to diagnostic views only, preventing low-confidence data from polluting attribution outputs. The correlation quality gate enforces precision ≥ 0.90 and recall ≥ 0.85 against a labeled evaluation dataset in CI.

```mermaid
graph LR
    SIG["Kernel Signal<br/>(e.g. DNS latency spike)"]
    SPAN["OTel Span<br/>(e.g. chat.retrieval)"]

    SIG --> T1{"trace_id<br/>match?"}
    T1 -->|"Yes"| C1["1.00 · Exact"]
    T1 -->|"No"| T2{"pod+pid<br/>≤100ms?"}
    T2 -->|"Yes"| C2["0.90 · Process"]
    T2 -->|"No"| T3{"pod+conn<br/>≤250ms?"}
    T3 -->|"Yes"| C3["0.80 · Connection"]
    T3 -->|"No"| T4{"svc+node<br/>≤500ms?"}
    T4 -->|"Yes"| C4["0.65 · Service"]
    T4 -->|"No"| DROP["No correlation"]

    C1 --> ENRICH["Enrich Span"]
    C2 --> ENRICH
    C3 --> ENRICH
    C4 --> DIAG["Diagnostic Only<br/>(below 0.70 threshold)"]
    DROP --> DISCARD["Discard"]

    SPAN --> T1

    style C1 fill:#198754,color:#fff
    style C2 fill:#198754,color:#fff
    style C3 fill:#198754,color:#fff
    style C4 fill:#ffc107,color:#000
    style DROP fill:#dc3545,color:#fff
    style ENRICH fill:#198754,color:#fff
    style DIAG fill:#ffc107,color:#000
    style DISCARD fill:#dc3545,color:#fff
```

### Stage 3: Attribution and SLO Diagnostics

Correlated events feed an attribution engine that classifies SLO violations into fault domains (network, compute, provider, retrieval) and produces structured outputs:

- **SLO events** aligned to a stable v1 JSON schema with LLM-specific SLIs: time-to-first-token (TTFT), token throughput, request latency, error rate, retrieval latency
- **Incident attributions** with fault-domain classification, confidence scores, and evidence chains
- **Confusion matrices** with per-fault precision, recall, and F1 scores
- **Provenance metadata** for audit and reproducibility

All outputs export via OTLP/HTTP to standard OpenTelemetry backends and via JSONL for offline analysis.

## Architecture

```mermaid
graph TB
    subgraph K8S["Kubernetes Cluster"]
        direction TB

        subgraph WORKLOADS["LLM Workloads"]
            LLM["LLM Service<br/>(OTel spans)"]
            RAG["RAG Service<br/>(OTel spans)"]
            EMB["Embedding<br/>Service"]
        end

        subgraph AGENT["eBPF Agent · DaemonSet"]
            direction TB
            subgraph PROBES["Kernel Probes"]
                P1["DNS<br/>Latency"]
                P2["TCP<br/>Retrans"]
                P3["Sched<br/>Delay"]
                P4["Connect<br/>Latency"]
                P5["TLS<br/>Handshake"]
                P6["CPU<br/>Steal"]
            end
            CORR["Correlation Engine<br/>confidence ≥ 0.70"]
            ATTR["Attribution Engine<br/>fault-domain classification"]
        end

        subgraph OBS["Observability Backend"]
            PROM["Prometheus"]
            TEMPO["Tempo"]
            GRAF["Grafana"]
            OTELC["OTel Collector"]
        end
    end

    LLM & RAG & EMB -->|"OTel traces"| CORR
    P1 & P2 & P3 & P4 & P5 & P6 -->|"kernel signals"| CORR
    CORR -->|"enriched events"| ATTR
    ATTR -->|"OTLP/HTTP"| OTELC
    OTELC --> PROM & TEMPO
    PROM & TEMPO --> GRAF

    style K8S fill:#f8f9fa,stroke:#dee2e6
    style WORKLOADS fill:#e8f4f8,stroke:#0d6efd
    style AGENT fill:#d1e7dd,stroke:#198754
    style PROBES fill:#f8d7da,stroke:#dc3545
    style OBS fill:#fff3cd,stroke:#ffc107
```

### Example: Attributing a TTFT SLO Violation

This sequence shows how the toolkit traces a user-facing latency spike to its kernel-level root cause:

```mermaid
sequenceDiagram
    participant User
    participant RAG as RAG Service
    participant VDB as Vector DB
    participant Kernel as Linux Kernel
    participant Agent as eBPF Agent
    participant Corr as Correlation Engine
    participant Attr as Attribution Engine

    User->>RAG: POST /chat (expects TTFT < 800ms)
    RAG->>VDB: embedding lookup

    Note over Kernel: DNS resolution to vector-db.svc<br/>takes 450ms (normally 2ms)
    Kernel-->>Agent: dns_latency_ms = 450

    VDB-->>RAG: embeddings (delayed)
    RAG-->>User: first token at 1200ms (SLO violated)

    Note over RAG: OTel span: chat.retrieval<br/>duration = 1180ms

    Agent->>Corr: kernel signal (DNS spike, pod, pid, timestamp)
    RAG->>Corr: OTel span (chat.retrieval, trace_id, pod)

    Corr->>Corr: Match: pod+pid within 100ms<br/>confidence = 0.90

    Corr->>Attr: enriched event (span + DNS evidence)
    Attr->>Attr: Classify → fault domain: NETWORK<br/>sub-cause: DNS resolution delay

    Note over Attr: Output: SLO event (v1 schema)<br/>fault_domain: network<br/>evidence: dns_latency_ms = 450<br/>confidence: 0.90<br/>sli_violated: ttft > 800ms
```

## CI/CD Pipeline

Every pull request and nightly run passes through a multi-stage quality pipeline:

```mermaid
graph LR
    subgraph PR["Pull Request Gates"]
        BUILD["Build<br/>go build ./..."]
        LINT["Lint<br/>golangci-lint"]
        UNIT["Unit Tests<br/>go test ./..."]
        SCHEMA["Schema<br/>Validation"]
        CORR_GATE["Correlation<br/>Quality Gate<br/>P≥0.90 R≥0.85"]
        FAULT["Fault Injection<br/>Smoke"]
        OTLP_SMOKE["OTLP Export<br/>Smoke"]
    end

    subgraph NIGHTLY["Nightly (Self-Hosted Linux)"]
        KIND["kind Cluster<br/>Deploy"]
        EBPF_PRIV["Privileged eBPF<br/>Smoke"]
        OBS_SMOKE["Observability<br/>Stack Validation"]
        INCIDENT["Incident Smoke<br/>DNS · Retransmit · CPU"]
    end

    subgraph WEEKLY["Weekly"]
        FULL_MATRIX["Full Scenario<br/>Matrix"]
        REGRESSION["Release-Tag<br/>Regression"]
        REPORT["Benchmark<br/>Report Publish"]
    end

    PR --> NIGHTLY --> WEEKLY

    style PR fill:#e8f4f8,stroke:#0d6efd
    style NIGHTLY fill:#fff3cd,stroke:#ffc107
    style WEEKLY fill:#d1e7dd,stroke:#198754
```

## Key Results

Results from the current benchmark suite using controlled fault injection on a 3-node kind cluster:

| Metric | Value |
|---|---|
| Attribution accuracy (mixed faults) | 100% |
| Detection delay median | 2.50 s |
| False positive rate | 0.00 |
| False negative rate | 0.00 |
| Burn-rate prediction error | 0.07 |
| Collector CPU overhead | 2.20% |
| Collector memory overhead | 120 MB |
| Correlation precision | ≥ 0.90 (CI-gated) |
| Correlation recall | ≥ 0.85 (CI-gated) |

Benchmark artifacts (confusion matrices, predictions, provenance) are published with every CI run for independent verification.

## Technical Stack

- **Language**: Go 1.23
- **eBPF**: libbpf CO-RE (primary), BCC fallback for non-BTF hosts
- **Orchestration**: Kubernetes (kind for development, any conformant cluster for production)
- **Telemetry**: OpenTelemetry SDK, OTLP/HTTP exporters, Prometheus client
- **Observability**: Grafana, Prometheus, Tempo, OpenTelemetry Collector
- **Schemas**: JSON Schema for contract stability (v1 SLO events, v1 incident attributions, v1alpha1 probe events)
- **CI/CD**: GitHub Actions with nightly eBPF integration on self-hosted Linux runners

## Quick Start

### Prerequisites
- Go 1.23+
- Docker (for kind cluster)
- kubectl

### Build and Test
```bash
# Validate prerequisites
go run ./cmd/sloctl prereq check

# Build and run unit tests
make build && make test

# Run correlation quality gate
make correlation-gate
```

### Local Cluster Deployment
```bash
# Start 3-node kind cluster
make kind-up

# Deploy agent DaemonSet
kubectl apply -k deploy/k8s

# Deploy observability stack (Prometheus + Tempo + Grafana + OTel Collector)
kubectl apply -k deploy/observability

# Run observability smoke test
make kind-observability-smoke

# Tear down
make kind-down
```

### Agent and Collector
```bash
# Run agent with OTLP export
go run ./cmd/agent --count 3 --output otlp --otlp-endpoint http://127.0.0.1:4318/v1/logs

# Run collector with fault injection input
go run ./cmd/faultinject --scenario mixed --count 24 --out artifacts/fault-injection/raw_samples.jsonl
go run ./cmd/collector --input artifacts/fault-injection/raw_samples.jsonl --output jsonl --output-path artifacts/collector/slo-events.jsonl

# Run deterministic RAG demo service
go run ./demo/rag-service --bind :8080 --metrics-bind :2113
```

### Benchmarks and Fault Injection
```bash
# Generate fault replay samples
go run ./cmd/faultreplay --scenario mixed --count 30 --out artifacts/fault-replay/fault_samples.jsonl

# Build benchmark report from replay samples
go run ./cmd/benchgen --out artifacts/benchmarks-replay --scenario mixed_faults --input artifacts/fault-replay/fault_samples.jsonl

# Run chaos fault matrix
make chaos-matrix
```

## Project Structure

```
cmd/
├── agent/           # eBPF data collection agent (DaemonSet)
├── collector/       # SLO event normalization pipeline
├── attributor/      # Fault-to-incident attribution engine
├── benchgen/        # Benchmark artifact generator
├── faultreplay/     # Synthetic fault scenario streams
├── faultinject/     # Raw fault injection harness
├── correlationeval/ # Correlation quality gate evaluator
├── sloctl/          # CLI prerequisite checker
└── loadgen/         # Load generation utility

pkg/
├── schema/          # v1 JSON schema types (SLOEvent, IncidentAttribution, ProbeEvent)
├── collector/       # Sample generation and normalization
├── attribution/     # Fault classification and confusion matrix
├── correlation/     # Span-signal correlation with confidence tiers
├── otel/            # OTLP/HTTP exporters for SLO events
├── benchmark/       # Benchmark harness and artifact generation
├── faultreplay/     # Multi-domain fault scenario engine
├── signals/         # Kernel signal models
├── slo/             # SLO burn-rate calculation
├── safety/          # Overhead guards and rate limiters
├── semconv/         # Semantic conventions (llm.ebpf.*)
└── toolkitcfg/      # Configuration management

ebpf/
├── c/               # eBPF C programs (CO-RE)
├── bcc-fallback/    # BCC scripts for non-BTF hosts
└── headers/         # vmlinux.h and helpers

deploy/
├── k8s/             # DaemonSet, ConfigMap, RBAC
├── observability/   # Prometheus, Tempo, Grafana, OTel Collector
└── kind/            # Local cluster configuration

docs/
├── contracts/       # Stable v1 and alpha schema definitions
├── benchmarks/      # Benchmark methodology and reports
├── strategy/        # Differentiation analysis and roadmap
└── research/        # Competitive landscape sources
```

## Differentiation

This toolkit occupies a specific position in the observability landscape that no existing tool addresses:

```mermaid
quadrantChart
    title Observability Tool Positioning
    x-axis "Generic Telemetry" --> "LLM-Specific SLIs"
    y-axis "App-Level Only" --> "Kernel-Grounded"

    This Toolkit: [0.90, 0.92]
    OTel eBPF: [0.20, 0.70]
    Pixie: [0.15, 0.75]
    Cilium/Hubble: [0.10, 0.80]
    Coroot: [0.30, 0.60]
    Parca: [0.10, 0.65]
    Datadog USM: [0.25, 0.55]
    Tetragon: [0.05, 0.85]
```

| Capability | This Toolkit | OTel eBPF | Pixie | Cilium/Hubble | Coroot |
|---|---|---|---|---|---|
| LLM-specific SLI decomposition (TTFT, token throughput) | Yes | No | No | No | No |
| Kernel-to-span causal attribution | Yes | Partial | Partial | No | Partial |
| Fault-domain incident classification | Yes | No | No | No | Limited |
| Reproducible benchmark methodology | Yes | No | No | No | No |
| Confidence-gated correlation | Yes | No | No | No | No |
| Overhead-gated safety controls | Yes | No | Partial | N/A | Partial |

**Three pillars of differentiation**:

1. **No-code kernel telemetry baseline** — captures infrastructure signals even when application instrumentation is partial or absent, reducing dependency on per-team tracing maturity.

2. **LLM-native SLI semantics** — models time-to-first-token, token throughput collapse, provider error classification, and retrieval contribution as first-class SLIs rather than generic HTTP metrics.

3. **Causal attribution with measured confidence** — correlates kernel events with application traces and Kubernetes identity to produce fault-domain hypotheses with published precision/recall/F1, treating uncertainty as a first-class output rather than hiding it.

## Methodology and Reproducibility

The project follows publishable benchmark standards with a controlled experimental pipeline:

```mermaid
graph TD
    subgraph INJECT["Fault Injection (Controlled)"]
        F1["DNS Latency"]
        F2["CPU Throttling"]
        F3["Provider 429/5xx"]
        F4["Memory Pressure"]
        F5["Egress Packet Loss"]
        F6["Mixed Faults"]
    end

    subgraph HARNESS["Benchmark Harness"]
        REPLAY["Fault Replay<br/>seed: 42"]
        LOAD["Load Profile<br/>rag_mixed_20rps"]
        PHASES["Phases<br/>baseline 10m → fault 10m → recovery 5m"]
    end

    subgraph MEASURE["Measurement"]
        CM["Confusion Matrix<br/>per-fault P/R/F1"]
        OVERHEAD["Overhead Metrics<br/>CPU %, memory MB"]
        PROV["Provenance<br/>run ID, git SHA, timestamps"]
    end

    subgraph GATE["CI Quality Gate"]
        PREC["Precision ≥ 0.90"]
        REC["Recall ≥ 0.85"]
        OVER["Overhead ≤ 3%"]
        REPS["≥ 10 repetitions<br/>CI95 intervals"]
    end

    INJECT --> HARNESS
    HARNESS --> MEASURE
    MEASURE --> GATE
    GATE -->|"pass"| MERGE["Merge Allowed"]
    GATE -->|"fail"| BLOCK["Merge Blocked"]

    style INJECT fill:#f8d7da,stroke:#dc3545
    style HARNESS fill:#e8f4f8,stroke:#0d6efd
    style MEASURE fill:#fff3cd,stroke:#ffc107
    style GATE fill:#d1e7dd,stroke:#198754
    style MERGE fill:#198754,color:#fff
    style BLOCK fill:#dc3545,color:#fff
```

- **Controlled fault injection**: deterministic scenarios for DNS latency, CPU throttling, provider rate limiting, memory pressure, egress packet loss, and mixed-fault conditions
- **Statistical requirements**: minimum 10 repetitions per profile, CI95 confidence intervals, class balance reporting
- **Transparency**: confusion matrices, abstain rates, and provenance metadata published with every benchmark run
- **Reproducibility**: seeded random generation (seed: 42), canonical load profiles, and versioned fault manifests
- **CI-enforced quality gates**: correlation precision ≥ 0.90 and recall ≥ 0.85 block merges when violated

This methodology is designed to meet the evidentiary standards of peer-reviewed systems research, enabling independent verification of all attribution accuracy claims.

## Roadmap

```mermaid
gantt
    title v0.2 Release Plan
    dateFormat  YYYY-MM-DD
    axisFormat  %b %d

    section Bootstrap
    M0 Buildable repo + CI + kind          :done, m0, 2026-02-17, 2026-02-23

    section Demo
    M1 Streaming RAG + baseline SLI        :active, m1, 2026-02-24, 2026-03-02

    section Signals
    M2 Six CO-RE signals · overhead ≤5%    :m2, 2026-03-03, 2026-03-16

    section Correlation
    M3 Precision ≥0.90 · Recall ≥0.85     :m3, 2026-03-17, 2026-03-23

    section Incident Lab
    M4 Six scenarios · dashboards · alerts :m4, 2026-03-24, 2026-03-30

    section Release
    M5 v0.2 GA · overhead ≤3% · SBOM      :m5, 2026-03-31, 2026-04-13
```

| Milestone | Status | Key Deliverables |
|---|---|---|
| M0: Bootstrap | Complete | Buildable repo, kind cluster, CI pipeline, DaemonSet heartbeat |
| M1: Demo + OTel | In progress | Streaming RAG demo, baseline TTFT/tokens-per-second, first correlated signal |
| M2: Signals v1 | Planned | Six CO-RE signals, schema validation, safety toggles, overhead ≤ 5% |
| M3: Correlation | Planned | Production correlator, retrieval decomposition, retry storm detection |
| M4: Incident Lab | Planned | Six deterministic scenarios, Grafana dashboards, alerting rules |
| M5: Bench + Release | Planned | v0.2 GA, overhead ≤ 3%, signed artifacts with SBOM and provenance |

## Documentation

| Document | Description |
|---|---|
| [Differentiation Strategy](docs/strategy/differentiation-strategy.md) | Competitive analysis against 10 adjacent tools with gap assessment |
| [v0.2 Build Plan](docs/strategy/v0.2-build-plan.md) | Milestone schedule, hard gates, and acceptance criteria |
| [Go/No-Go Checklist](docs/strategy/v0.2-go-no-go-checklist.md) | Release gate tracking for contract, signal, correlation, and benchmark readiness |
| [Why This Exists](docs/strategy/why-this-exists-security-sre.md) | Problem statement for SRE, platform, and security teams |
| [Demo Stories](docs/strategy/killer-demo-stories.md) | Five demo scenarios with measurable win conditions |
| [SLO Event Schema](docs/contracts/v1/slo-event.schema.json) | Stable v1 contract for SLO event outputs |
| [Attribution Schema](docs/contracts/v1/incident-attribution.schema.json) | Stable v1 contract for incident attribution outputs |
| [Benchmark Specification](docs/benchmarks/llm-slo-attribution-accuracy.md) | Experimental design, statistical requirements, and publishability criteria |
| [Benchmark Reports](docs/benchmarks/reports/) | Latest attribution accuracy, overhead, and regression results |
| [Configuration Schema](config/toolkit.schema.json) | Toolkit configuration reference |
| [Runner Security Baseline](docs/security/self-hosted-runner-baseline.md) | CI runner isolation and credential management |

## Licence

MIT
