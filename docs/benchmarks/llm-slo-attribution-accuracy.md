# Benchmark Spec: LLM SLO Attribution Accuracy

## Purpose
Measure whether kernel-grounded telemetry improves LLM incident detection and fault attribution quality versus app-instrumentation-only baselines, while staying inside operational overhead budgets.

## Hypothesis
`eBPF + OTel correlation` reduces detection delay and improves fault-domain attribution accuracy relative to app-only observability, with bounded CPU/memory/event overhead.

## Comparison Conditions
1. Baseline: application instrumentation only.
2. Treatment A: eBPF toolkit only.
3. Treatment B: combined mode (application instrumentation + eBPF toolkit).

## Baseline Selection Rule
- Preferred comparison set: `main` vs `v0.2-rc` vs previous stable tag.
- If no previous stable tag exists, use first GA anchor tag as baseline.
- All reports must state which baseline rule was used.

## Workload Profiles
- Chat-only LLM service profile.
- RAG-enabled profile with retrieval dependency.
- Mixed-tenant concurrent profile.

## Fault Injection Matrix
- DNS latency injection.
- Packet loss and intermittent egress drop.
- CPU throttling (cgroup quotas).
- Memory pressure / reclaim pressure.
- Provider 429/5xx bursts.
- Retrieval backend latency inflation.

## Primary Metrics
- Detection delay from fault start to alert.
- Multiclass fault-domain attribution precision/recall/F1.
- False positive and false negative incident rates.
- Abstain rate (low-confidence no-decision cases).
- Burn-rate prediction error.
- Collector CPU, memory, event throughput, and drop rate.

## Success Thresholds (Initial)
- Median detection delay improvement >= 30% vs baseline.
- Attribution macro-F1 >= 0.85 on single-fault scenarios.
- False positive rate <= 10%.
- Abstain rate <= 15% on single-fault scenarios.
- Collector CPU overhead <= 5% per node for development and RC validation.
- Collector CPU overhead <= 3% per node for GA release gate.
- Collector memory overhead <= 250 MB per node in benchmark profile.
- Full 6-signal requirement applies to CO-RE reference kernels.
- BCC fallback runs are explicitly marked degraded capability and cannot claim full-signal coverage.

## Statistical Reporting Requirements
- Publish confusion matrix and per-class precision/recall/F1.
- Publish 95% confidence intervals for detection delay and macro-F1.
- Publish run count, sample size, and class balance per scenario.
- Publish abstain/uncertainty distribution by fault type.
- Minimum repetitions for release claims: >=10 runs per profile/condition.
- Fewer than 10 runs must be labeled exploratory and are non-gating.

## Measurement Method
- Time-synchronized run controller marks fault start/end events.
- Ground truth labels recorded in fault manifest.
- Attribution evaluated as multiclass classification plus abstain class.
- Report includes scenario-level and aggregate metrics.

## Reproducibility Protocol
- Pin kernel, Kubernetes, and collector image versions.
- Publish exact fault injection scripts and seed values.
- Capture raw outputs and report-generation code.
- Re-run on fixed weekly profile to track drift.
- Use nightly reduced-duration smoke scenarios for cost/runtime control.
- Keep canonical full-duration scenario matrix in weekly benchmark runs.

## Required Artifacts Per Run
- `artifacts/events/*.jsonl`
- `artifacts/metrics/*.csv`
- `artifacts/confusion-matrix.csv`
- `artifacts/summary/attribution_summary.json`
- `artifacts/report.md`
- `artifacts/environment-manifest.yaml`

## Failure Criteria (When to Reject a Run)
- Missing ground truth manifest or seed values.
- Missing confusion matrix or class-level metrics.
- Unexplained mismatch between summary and raw recomputed metrics.
- Collector overhead metrics missing for any node in test profile.

## Known Limitations
- Multi-fault concurrency can reduce attribution certainty.
- Kernel differences may alter event availability and timing behavior.
- Provider-side opacity can limit causal resolution without API-level telemetry.
