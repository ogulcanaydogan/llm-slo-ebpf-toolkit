# Changelog

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
