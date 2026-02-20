# CD Gate Integration Guide

The CD gate validates SLO metrics from Prometheus before allowing deployments to proceed. It queries TTFT p95, error rate, and burn rate, returning pass/fail with detailed violation information.

## Usage

```bash
sloctl cdgate check \
  --config config/toolkit.yaml \
  --prometheus-url http://prometheus:9090 \
  --ttft-p95-ms 800 \
  --error-rate 0.05 \
  --burn-rate 2.0 \
  --output json
```

`sloctl cdgate check` reads defaults from `config/toolkit.yaml` (`cdgate.*`). Any CLI flag you pass overrides the config value for that invocation.

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All SLO metrics within thresholds (PASS) |
| 1 | One or more SLO metrics exceeded thresholds (FAIL) |
| 2 | Invalid arguments or usage error |

## Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--config` | `config/toolkit.yaml` | Toolkit config path used for default thresholds |
| `--prometheus-url` | `http://prometheus:9090` | Prometheus base URL |
| `--ttft-p95-ms` | `800` | TTFT p95 threshold (ms) |
| `--error-rate` | `0.05` | Error rate threshold (0-1) |
| `--burn-rate` | `2.0` | Burn rate threshold |
| `--fail-open` | `true` | Pass gate if Prometheus is unreachable |
| `--output` | `text` | Output format: `text` or `json` |
| `--timeout` | `10` | Query timeout in seconds |

## JSON Output

```json
{
  "pass": false,
  "violations": [
    {
      "metric": "ttft_p95_ms",
      "threshold": 800,
      "actual": 950.5
    }
  ],
  "timestamp": "2026-02-20T12:00:00Z"
}
```

## CI/CD Integration

### GitHub Actions

```yaml
- name: SLO gate check
  run: |
    go run ./cmd/sloctl cdgate check \
      --config config/toolkit.yaml \
      --prometheus-url ${{ secrets.PROMETHEUS_URL }} \
      --output json
```

### ArgoCD PreSync Hook

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: slo-gate
  annotations:
    argocd.argoproj.io/hook: PreSync
spec:
  template:
    spec:
      containers:
        - name: gate
          image: ghcr.io/ogulcanaydogan/llm-slo-ebpf-toolkit-agent:latest
          command: ["/app/sloctl", "cdgate", "check"]
          args:
            - "--prometheus-url=http://prometheus.monitoring:9090"
            - "--ttft-p95-ms=800"
            - "--error-rate=0.05"
      restartPolicy: Never
```

### Flux Pre-deployment Check

Add as a health check in your Flux Kustomization:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
spec:
  healthChecks:
    - apiVersion: batch/v1
      kind: Job
      name: slo-gate
```

## PromQL Queries

The CD gate uses these default PromQL queries:

| Metric | Query |
|--------|-------|
| TTFT p95 | `histogram_quantile(0.95, sum(rate(llm_slo_ttft_ms_bucket[5m])) by (le))` |
| Error rate | `sum(rate(llm_slo_errors_total[5m])) / sum(rate(llm_slo_requests_total[5m]))` |
| Burn rate | `llm_slo_burn_rate` |

## Configuration

### Via toolkit.yaml

```yaml
cdgate:
  enabled: true
  prometheus_url: http://prometheus:9090
  ttft_p95_ms: 800
  error_rate: 0.05
  burn_rate: 2.0
  fail_open: true
```

### Via Helm Values

```bash
helm install llm-slo-agent charts/llm-slo-agent \
  --set cdgate.enabled=true \
  --set cdgate.prometheusURL=http://prometheus:9090 \
  --set cdgate.ttftP95MS=800
```
