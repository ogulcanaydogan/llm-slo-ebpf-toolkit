# Changelog

## Unreleased

## v0.2.0 - 2026-02-19

- Added release workflow container publishing for `agent` and `rag-service` images to GHCR with keyless cosign signing and provenance attestations.
- Added `e2e-evidence-report.yml` workflow to generate deterministic end-to-end evidence bundles and markdown reports.
- Added min-capability non-privileged deployment overlay under `deploy/k8s/min-capability` and security documentation (`docs/security/agent-min-capability-mode.md`).
- Expanded chaos matrix runner with optional real injectors (`REAL_INJECTORS=true`) for DNS delay, retransmit/loss, and CPU stress while preserving synthetic fallback.
- Added kernel compatibility matrix workflow (`kernel-compatibility-matrix.yml`) plus report generation scripts and published compatibility document (`docs/compatibility.md`).
- Completed RC burn-in: 3 RC cuts (rc.1–rc.3), 6+ weekly benchmark passes with M5 gates enforced, nightly integration stabilized. All go-no-go gates PASS or ENFORCED. GA decision: GO.

## v0.2.0-rc.2 - 2026-02-19
- Stabilized nightly privileged integration by replacing a terminating-pod-sensitive RAG readiness wait with deployment availability checks and safer port-forward cleanup.
- Hardened OTLP smoke and agent deployment path in CI by enforcing local image override usage and metrics-based ingestion gating.
- Added host-network DNS fix for the agent DaemonSet via `dnsPolicy: ClusterFirstWithHostNet`.
- Added `pr-privileged-ebpf-smoke` workflow to execute merge-oriented self-hosted eBPF smoke on trusted pull requests, with explicit fork PR isolation.
- Updated security and README CI documentation to reflect privileged PR smoke and runner-routing behavior.

## v0.2.0-rc.1 - 2026-02-18
- Rewrote README with full architecture diagrams, visual Mermaid explanations, competitive positioning quadrant, sequence diagram for TTFT attribution, and Gantt roadmap.
- Expanded Grafana dashboards from 1 panel to 17 panels across 3 dashboards (SLO Overview, Kernel Correlation, Incident Lab) with thresholds, units, and descriptions.
- Added all 3 dashboards to K8s ConfigMap for automatic Grafana provisioning.
- Added RAG demo service smoke test to nightly CI workflow (validates TTFT, tokens/sec, and trace_id in response).
- Expanded demo/rag-service README with streaming/non-streaming examples, expected output, PromQL queries, load profiles, and full flag reference.
- Added correlation quality evaluator (`cmd/correlationeval`, `pkg/correlation/evaluator.go`) with labeled dataset and CI quality gate.
- Added 6 CO-RE eBPF probes: DNS latency, TCP retransmit, runqueue delay, connect latency, TLS handshake, CPU steal — with shared event header and ring buffer emission.
- Added Go ring buffer consumer (`pkg/collector/ringbuf.go`) to decode kernel events into `schema.ProbeEventV1`.
- Added probe manager (`pkg/collector/probe_manager.go`) with lifecycle control, safety integration, and per-signal disable.
- Added BCC fallback stub (`pkg/collector/bcc_fallback.go`) for degraded-mode DNS+TCP collection.
- Updated `ebpf/bpf2go/gen.sh` to generate CO-RE bindings for all 7 programs (minimal + 6 signal probes).
- Added retry storm detector (`pkg/correlation/retry_storm.go`) with sliding-window per-pod burst detection.
- Added retrieval latency decomposition (`DecomposeRetrieval`) to OTel correlator for kernel-attributed retrieval breakdown.
- Added `llm.ebpf.retrieval.kernel_attributed_ms` and `llm.ebpf.tcp.retry_storm` semconv attributes.
- Expanded correlation labeled dataset from 20 to 55 pairs covering all 4 tiers, edge cases, and cross-service scenarios.
- Added 6th fault scenario `network_partition` with high connect latency, TCP retransmit storms, and DNS timeouts.
- Added Prometheus alerting rules (`deploy/observability/prometheus-alerts.yaml`) with 5 alerts: TTFT budget burn, error rate, correlation degraded, agent heartbeat stale, overhead high.
- Added 6 declarative incident scenario YAML definitions under `test/incident-lab/scenarios/`.
- Added `cfs_throttled_ms` signal mapping to OTel correlator with `llm.ebpf.cpu.cfs_throttled_ms` semconv attribute.
- Added weekly benchmark workflow (`.github/workflows/weekly-benchmark.yml`) running full 6-scenario matrix with baseline comparison and M5 gate execution.
- Added release workflow (`.github/workflows/release.yml`) with cross-compiled binaries, SHA-256 checksums, SBOM (syft), and provenance.
- Added M5 gate tool (`cmd/m5gate`, `pkg/releasegate`) enforcing B5 overhead, D3 variance, and E3 significance gates.
- Added AWS ephemeral eBPF runner infrastructure (`infra/runner/aws/`) with Terraform, SSM management, and HTTPS-only egress.
- Added runner preflight detection (`scripts/ci/check_ebpf_runner.sh`) with automatic synthetic fallback in CI.
- Added nightly and weekly CI conditional paths for self-hosted vs fallback execution.
- Updated go-no-go checklist: all M0–M5 gates PASS or ENFORCED. RC decision: GO.

## v0.1.0-alpha.5 - 2026-02-18
- Added OTLP/HTTP SLO event exporter (`pkg/otel/slo_event_exporter.go`) and wired `cmd/collector` + `cmd/agent` to support `--output otlp`.
- Added OTLP exporter tests and non-2xx failure handling coverage.
- Added observability baseline manifests (`deploy/observability`) with OTel collector, Prometheus, Tempo, and Grafana.
- Added kind observability smoke script (`test/integration-kind/observability-smoke.sh`) and make target (`kind-observability-smoke`).
- Added chaos automation scripts (`scripts/chaos/set_agent_mode.sh`, `scripts/chaos/run_fault_matrix.sh`) and make targets (`chaos-matrix`, `chaos-agent-otlp`).
- Extended fault replay scenarios with `memory_pressure`.

## v0.1.0-alpha.4 - 2026-02-18
- Added Kubernetes deployment skeleton for collector DaemonSet under `deploy/k8s`.
- Added `cmd/faultreplay` and `pkg/faultreplay` for multi-domain synthetic replay streams (`provider_throttle`, `dns_latency`, `cpu_throttle`, `mixed`).
- Added collector runtime flags for input/output mode, synthetic scenario generation, and stream mode (`--count=0`).
- Added `cmd/faultinject` harness to generate raw collector input streams for controlled fault scenarios.
- Added synthetic sample metadata enrichment (`node`, `fault_label`) in emitted SLO events.
- Extended benchmark harness to emit `report.md` as part of each artifact bundle.
- Added CI smoke path for fault injection -> collector normalization and replay -> benchmark bundle creation.

## v0.1.0-alpha.3 - 2026-02-17
- Added mixed-fault benchmark scenario (`mixed_faults`) combining `provider_throttle` and `dns_latency` labels.
- Added fixture-driven benchmark input flow using `pkg/benchmark/testdata/mixed_fault_samples.jsonl`.
- Expanded benchmark tests to validate mixed-domain confusion matrix output and external JSONL input ingestion.
- Updated quickstart docs with mixed-scenario and explicit-input benchmark commands.

## v0.1.0-alpha.2 - 2026-02-17
- Added attribution pipeline utilities for batch conversion, confusion matrix generation, and accuracy scoring.
- Added JSONL fault-sample loader and test coverage for loader and attribution math.
- Upgraded `cmd/attributor` with `--input`, `--out`, `--summary-out`, and `--confusion-out` outputs.
- Reworked benchmark harness to emit contract-aligned artifacts: `incident_predictions.csv`, `confusion-matrix.csv`, `collector_overhead.csv`, `attribution_summary.json`, and `provenance.json`.
- Added benchmark harness validation for unsupported scenarios and expanded quickstart commands.

## v0.1.0-alpha.1 - 2026-02-17
- Initialized Go repository with CI, tests, lint config, and benchmark scaffolding.
- Added collector pipeline producing schema-validated `slo-event` records.
- Added attribution mapper producing schema-validated `incident-attribution` records.
- Added provider-throttle scenario mapping and benchmark artifact generator.
- Added benchmark report template and output schema-compatible artifact generation.
- Added command-line tools: `collector`, `attributor`, and `benchgen`.
