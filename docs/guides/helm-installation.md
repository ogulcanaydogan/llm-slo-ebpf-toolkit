# Helm Installation Guide

## Prerequisites

- Kubernetes cluster (1.24+)
- Helm 3.12+
- Linux nodes with kernel 5.10+ and BTF support

## Install from OCI Registry

```bash
helm install llm-slo-agent \
  oci://ghcr.io/ogulcanaydogan/charts/llm-slo-agent \
  --version 0.3.0 \
  --namespace llm-slo-system \
  --create-namespace
```

## Install from Local Chart

```bash
helm install llm-slo-agent charts/llm-slo-agent \
  --namespace llm-slo-system \
  --create-namespace
```

## Configuration

All configuration is managed via `values.yaml`. Key settings:

### OTLP Endpoint

```yaml
otlp:
  endpoint: "http://otel-collector.observability.svc.cluster.local:4318/v1/logs"
  timeoutMS: "5000"
```

### Signal Set

```yaml
toolkit:
  signalSet:
    - dns_latency_ms
    - tcp_retransmits_total
    - runqueue_delay_ms
    - connect_latency_ms
    - tls_handshake_ms
    - cpu_steal_pct
    - mem_reclaim_latency_ms
    - disk_io_latency_ms
    - syscall_latency_ms
```

### Webhook (disabled by default)

```yaml
webhook:
  enabled: true
  url: "https://events.pagerduty.com/v2/enqueue"
  secret: "your-hmac-secret"
  format: "pagerduty"
```

### Resources

```yaml
resources:
  requests:
    cpu: 100m
    memory: 128Mi
  limits:
    cpu: 500m
    memory: 512Mi
```

## Upgrade

```bash
helm upgrade llm-slo-agent charts/llm-slo-agent \
  --namespace llm-slo-system
```

## Uninstall

```bash
helm uninstall llm-slo-agent --namespace llm-slo-system
```

## Validation

```bash
# Lint the chart
helm lint charts/llm-slo-agent

# Render templates without installing
helm template test-release charts/llm-slo-agent

# Run Helm test after install
helm test llm-slo-agent --namespace llm-slo-system
```
