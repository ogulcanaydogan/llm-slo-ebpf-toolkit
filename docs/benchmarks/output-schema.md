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
  "metrics": {
    "detection_delay_seconds_median": 0.0,
    "attribution_accuracy": 0.0,
    "false_positive_rate": 0.0,
    "false_negative_rate": 0.0,
    "burn_rate_prediction_error": 0.0,
    "collector_cpu_overhead_pct": 0.0,
    "collector_memory_overhead_mb": 0.0
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
- `detection_delay_seconds`
- `is_correct`

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
  "started_at": "RFC3339",
  "finished_at": "RFC3339"
}
```
