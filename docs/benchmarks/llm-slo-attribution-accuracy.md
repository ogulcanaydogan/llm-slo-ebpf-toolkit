# Benchmark Spec: LLM SLO Attribution Accuracy

## Purpose
Evaluate how accurately and quickly the toolkit detects and localizes LLM SLO degradations compared with app-instrumentation-only baselines.

## Hypothesis
Kernel-grounded telemetry plus OTel correlation improves incident detection latency and root-cause attribution accuracy with acceptable runtime overhead.

## Comparison Conditions
1. Application instrumentation only (baseline).
2. eBPF toolkit only.
3. Combined mode (app instrumentation + eBPF toolkit).

## Workloads
- Chat-only service profile.
- RAG-enabled profile with retrieval dependency.
- Mixed tenant profile under concurrent load.

## Fault Injection Matrix
- DNS latency injection.
- Packet loss and intermittent egress drops.
- CPU throttling (cgroup quotas).
- Memory pressure and reclaim pressure.
- Provider 429/5xx error bursts.
- Retrieval backend latency inflation.

## Primary Metrics
- Detection delay (seconds) from fault start to alert.
- Attribution accuracy (correctly identified primary fault domain).
- False positive and false negative incident rates.
- SLO burn-rate prediction error.
- Runtime overhead (collector CPU/memory/event throughput).

## Success Thresholds (Initial)
- Median detection delay improved by >=30% vs baseline.
- Attribution accuracy >=85% on single-fault scenarios.
- False positive rate <=10% across test matrix.
- Collector CPU overhead <=5% per node under target load.
- Collector memory overhead <=250 MB per node in benchmark profile.

## Measurement Method
- Time-synchronized run controller marks fault begin/end.
- Ground truth labels are recorded in fault manifest.
- Attribution output evaluated as multiclass classification.
- Report includes confusion matrix, per-class precision/recall/F1.

## Reproducibility Protocol
- Pin kernel, Kubernetes, and collector image versions.
- Publish exact fault injection scripts and seed values.
- Capture all raw event outputs and report generation code.
- Re-run weekly on same cluster profile for trend stability.

## Deliverables Per Run
- `artifacts/events/*.jsonl`
- `artifacts/metrics/*.csv`
- `artifacts/confusion-matrix.csv`
- `artifacts/report.md`
- `artifacts/environment-manifest.yaml`

## Known Limitations
- Multi-fault concurrency can reduce attribution certainty.
- Kernel differences may change event availability and timing.
- Some provider-side metrics may remain opaque without API-level instrumentation.
