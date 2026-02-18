# Changelog

## Unreleased
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
