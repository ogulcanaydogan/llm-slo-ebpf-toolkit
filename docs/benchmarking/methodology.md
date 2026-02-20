# Benchmark Methodology

## Overview

The toolkit uses a multi-gate statistical framework to evaluate release quality. Benchmarks run weekly in CI across 6 fault scenarios with 3 reruns each, producing artifacts that feed into four independent release gates.

## Fault Scenarios

Each scenario injects controlled SLI degradation with deterministic parameters:

| Scenario | Primary Signals | Key Assertions |
|----------|-----------------|----------------|
| `dns_latency` | DNS 220ms, connect 130ms | TTFT p95 > 800ms, DNS p95 > 150ms |
| `cpu_throttle` | Runqueue 28ms, CFS 170ms, steal 9% | Tokens p50 < 15, runqueue p95 > 20ms |
| `provider_throttle` | Connect 45ms, TLS 55ms, connect errors | TTFT p95 > 800ms, error rate > 10% |
| `memory_pressure` | CFS 90ms, runqueue 14ms | CFS p95 > 60ms, runqueue p95 > 10ms |
| `network_partition` | Connect 350ms, retransmits 12, DNS 180ms | TTFT p95 > 800ms, error rate > 20% |
| `mixed` | Round-robin through all 5 faults | Attribution precision >= 90%, recall >= 85% |

Scenarios are defined declaratively in `test/incident-lab/scenarios/*.yaml` with fixed seeds (default: 42), load profiles, and 3-phase execution (warmup, inject, recovery).

## Deterministic Harness

All benchmark runs share:

- **Seed**: 42 (deterministic sample generation)
- **Load profile**: `rag_mixed_20rps`
- **Phases**: 3 (warmup, fault injection, recovery)
- **Samples per run**: 24 fault samples
- **Minimum repetitions**: 3 reruns per scenario for variance gate; 10 for distribution claims

The harness (`pkg/benchmark/harness.go`) generates:

| Artifact | Format | Content |
|----------|--------|---------|
| `raw_samples.jsonl` | JSONL | Fault injection samples |
| `incident_predictions.csv` | CSV | Attribution predictions |
| `confusion_matrix.csv` | CSV | Prediction accuracy matrix |
| `collector_overhead.csv` | CSV | Agent CPU/memory metrics |
| `attribution_summary.json` | JSON | Aggregated accuracy, overhead, false positive/negative rates |
| `provenance.json` | JSON | Git commit, image digest, kernel config hash, timestamps |
| `report.md` | Markdown | Human-readable summary |

## Release Gates (M5)

The `m5gate` tool (`cmd/m5gate`, `pkg/releasegate`) evaluates four independent gates. All must pass for a release candidate to proceed.

### Gate B5: Collector CPU Overhead

**Threshold**: <= 3% (GA), <= 5% (development)

Validates that the eBPF agent's CPU overhead stays within budget:

- Loads `collector_overhead.csv` from each scenario/run directory
- Computes per-node p95 CPU overhead
- Computes mean CPU overhead across all samples
- Fails if any node p95 or overall mean exceeds threshold

### Gate D3: Rerun Variance

**Threshold**: <= 10% coefficient of variance

Validates reproducibility across reruns:

- Requires minimum 3 reruns per scenario
- Measures coefficient of variance (CV%) for three metrics:
  - TTFT P95 (Time To First Token at 95th percentile)
  - Tokens P50 (token throughput at 50th percentile)
  - Error rate mean
- CV% computed as: `(stddev / mean) * 100`
- Fails if any metric exceeds 10% variance in any scenario

### Gate E3: Significance (Regression Detection)

**Threshold**: TTFT regression > 5% with statistical significance

Detects real performance regressions using three non-parametric methods:

1. **Mann-Whitney U test** — Rank-based test comparing candidate vs. baseline TTFT distributions. Handles ties with average rank method and continuity correction. Two-tailed p-value via normal CDF. Threshold: p < 0.05.

2. **Bootstrap confidence interval** — Resamples candidate and baseline 1000 times (seed: 42). Computes quantile delta for each iteration. Extracts 95% CI from sorted deltas (2.5th and 97.5th percentiles). Fails if CI lower bound > 0 (indicating consistent degradation).

3. **Cliff's delta** — Non-parametric effect size: `(greater - lower) / (n1 * n2)`. Range: [-1, 1]. Threshold for practical significance: |delta| >= 0.147.

All three conditions must hold simultaneously for the gate to fail, reducing false positives.

**Minimum sample requirement**: 30 samples per scenario for both candidate and baseline. Runs with fewer samples are flagged as inconclusive rather than failing.

### Baseline Gate: Provenance

Validates that candidate and baseline runs are independent:

- Baseline manifest (`manifest.json`) tracks `source_ref`, `source_commit`, `generated_at`
- Candidate must not match baseline source (prevents no-op self-comparisons)
- If no prior stable tag exists, first GA anchor tag is used as baseline reference

## Weekly Benchmark Workflow

The weekly CI workflow (`.github/workflows/weekly-benchmark.yml`) orchestrates:

1. **Runner detection**: Check for `self-hosted,linux,ebpf` runner availability
2. **Baseline generation**: 36 samples per scenario from previous stable tag
3. **Candidate runs**: 6 scenarios x 3 reruns = 18 benchmark runs
4. **M5 gate evaluation**: `make m5-gate` against all artifacts
5. **Report publication**: Summary artifact with pass/fail per gate

When no self-hosted runner is available, the workflow falls back to synthetic mode and marks results as `fallback-synthetic-no-self-hosted-ebpf`.

## Reporting

Gate evaluation produces a machine-readable JSON summary (`m5_gate_summary.json`) and a human-readable report (`m5_gate_summary.md`) containing:

- Per-gate pass/fail with threshold values
- Per-scenario variance metrics (CV% for TTFT, tokens, error rate)
- Overhead measurements (node p95, mean, sample count)
- Significance test results (Mann-Whitney p-value, bootstrap CI bounds, Cliff's delta)
- Baseline provenance information
- Candidate reference and commit

## Experimental Design for Publication

For publishable results (per `docs/benchmarks/llm-slo-attribution-accuracy.md`):

- **Minimum repetitions**: 10 runs per profile (3 is minimum for CI gates)
- **Three treatments**: baseline (app-only), treatment A (eBPF-only), treatment B (combined eBPF + correlation)
- **Required outputs**: Confusion matrices, precision/recall/F1, abstain rates, provenance
- **CI reporting**: CI95 and class balance published in every report
- **Significance rule**: Results with fewer than 10 runs are marked exploratory and cannot gate a release
