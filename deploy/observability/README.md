# Local Observability Stack

This folder provides a local kind-compatible observability baseline:
- OTel Collector (OTLP/HTTP logs receiver)
- Prometheus
- Grafana

## Deploy

```bash
kubectl apply -k deploy/observability
```

## Verify

```bash
kubectl -n observability get pods
kubectl -n observability get svc
```

## Port forward

```bash
kubectl -n observability port-forward svc/grafana 3000:3000
kubectl -n observability port-forward svc/prometheus 9090:9090
```

Grafana default credentials:
- user: `admin`
- pass: `admin`

## Notes
- OTel Collector exposes OTLP/HTTP on `otel-collector.observability.svc:4318`.
- Prometheus scrapes agent heartbeat metrics and collector metrics.
- Dashboards are provisioned from configmaps.
- To send agent events to OTLP, switch output mode:
  - `kubectl -n llm-slo-system set env daemonset/llm-slo-agent OUTPUT_MODE=otlp OTLP_ENDPOINT=http://otel-collector.observability.svc.cluster.local:4318/v1/logs`
