# Benchmark Output Schemas

## File: `attribution_summary.json`
```json
{
  "run_id": "2026-02-17T20-00-00Z",
  "project": "llm-slo-ebpf-toolkit",
  "scenario": "dns_latency",
  "workload_profile": "rag_mixed",
  "environment": {
    "kubernetes_version": "1.31",
    "kernel_version": "6.x",
    "node_count": 3
  },
  "stats": {
    "sample_size": 0,
    "runs_count": 0,
    "confidence_level": 0.95
  },
  "metrics": {
    "detection_delay_seconds_median": 0.0,
    "detection_delay_ci95_low": 0.0,
    "detection_delay_ci95_high": 0.0,
    "attribution_macro_f1": 0.0,
    "attribution_macro_f1_ci95_low": 0.0,
    "attribution_macro_f1_ci95_high": 0.0,
    "false_positive_rate": 0.0,
    "false_negative_rate": 0.0,
    "abstain_rate": 0.0,
    "burn_rate_prediction_error": 0.0,
    "collector_cpu_overhead_pct": 0.0,
    "collector_memory_overhead_mb": 0.0,
    "events_per_second": 0.0,
    "dropped_events_rate": 0.0
  }
}
```

## File: `incident_predictions.csv`
Columns:
- `timestamp`
- `incident_id`
- `scenario`
- `fault_start_ts`
- `fault_end_ts`
- `predicted_fault_domain`
- `ground_truth_fault_domain`
- `confidence`
- `is_abstain`
- `detection_delay_seconds`
- `is_correct`

## File: `confusion_matrix.csv`
Columns:
- `actual_fault_domain`
- `predicted_fault_domain`
- `count`

## File: `class_metrics.csv`
Columns:
- `fault_domain`
- `precision`
- `recall`
- `f1`
- `support`

## File: `collector_overhead.csv`
Columns:
- `timestamp`
- `node`
- `collector_cpu_pct`
- `collector_memory_mb`
- `events_per_second`
- `dropped_events`

## File: `provenance.json`
```json
{
  "git_commit": "string",
  "collector_image_digest": "string",
  "kernel_config_hash": "string",
  "fault_harness_version": "string",
  "dataset_seed": 0,
  "benchmark_manifest_sha256": "string",
  "started_at": "RFC3339",
  "finished_at": "RFC3339"
}
```

## File: `report.md`
Required sections:
- run metadata (`run_id`, scenario, workload)
- core metrics (accuracy, detection delay, false positive/negative, overhead)
- artifact bundle list for traceability

## File: `m5_gate_summary.json`
Required top-level fields:
- `generated_at`
- `candidate_root`
- `baseline_root`
- `overhead` (`pass`, `threshold_pct`, `max_observed_pct`)
- `variance` (`pass`, per-scenario `variance_pct`)
- `significance` (`pass`, per-scenario `mann_whitney_p_value`, `bootstrap_delta_ci95`)
- `pass`

## File: `m5_gate_summary.md`
Required sections:
- B5 overhead verdict
- D3 rerun variance verdict
- E3 significance verdict
- failure details when `pass=false`

## Validation Rules
- Summary metrics must be recomputable from confusion/class metrics and raw incident predictions.
- CI fields must use the confidence level declared under `stats.confidence_level`.
- Any missing required artifact invalidates the benchmark run.
