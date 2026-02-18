# Demo RAG Service

Deterministic streaming RAG stub that simulates a retrieval-augmented generation workload with full observability instrumentation. Designed for local development, integration testing, and benchmark harness input.

## Features

- **OTel Spans**: `chat.request` (root), `chat.retrieval` (with latency breakdown), `chat.generation` (with token count)
- **Prometheus SLO Metrics**: TTFT, tokens/sec, retrieval component latencies (vectordb, network, dns), request rate by status
- **DNS Correlation Enrichment**: integrates with the eBPF correlator to enrich spans with `llm.ebpf.dns.latency_ms` when correlation confidence >= 0.70
- **Deterministic Output**: seeded RNG produces identical results for a given (prompt, seed, profile) triple
- **Three Load Profiles**: `chat_short` (fast), `rag_medium` (default), `context_long` (heavy retrieval)

## Local Run

```bash
go run ./demo/rag-service --bind :8080 --metrics-bind :2113
```

### Streaming Request

```bash
curl -sS -N http://localhost:8080/chat \
  -H 'content-type: application/json' \
  -d '{"prompt":"Explain DNS impact on TTFT","profile":"rag_medium","seed":42,"max_tokens":10,"stream":true}'
```

Expected output (NDJSON stream):

```json
{"type":"meta","request_id":"req-...","trace_id":"abc123...","profile":"rag_medium","selected_titles":["DNS Resolution","eBPF Probes"],"retrieval_vectordb_ms":62.0,"retrieval_network_ms":18.0,"retrieval_dns_ms":9.0}
{"type":"token","request_id":"req-...","trace_id":"abc123...","token":"explain","index":0}
{"type":"token","request_id":"req-...","trace_id":"abc123...","token":"dns","index":1}
...
{"type":"done","request_id":"req-...","trace_id":"abc123...","ttft_ms":85.3,"tokens_per_sec":22.1,"retrieval_vectordb_ms":62.0,"retrieval_network_ms":18.0,"retrieval_dns_ms":9.0}
```

### Non-Streaming Request

```bash
curl -sS http://localhost:8080/chat \
  -H 'content-type: application/json' \
  -d '{"prompt":"Explain DNS impact on TTFT","profile":"rag_medium","seed":42,"max_tokens":10,"stream":false}'
```

Expected output:

```json
{
  "request_id": "req-...",
  "trace_id": "abc123...",
  "profile": "rag_medium",
  "selected_titles": ["DNS Resolution", "eBPF Probes"],
  "response": "explain dns impact on ttft reliability signal trace kernel latency attribution",
  "ttft_ms": 85.3,
  "tokens_per_sec": 22.1,
  "retrieval_vectordb_ms": 62.0,
  "retrieval_network_ms": 18.0,
  "retrieval_dns_ms": 9.0
}
```

## OTLP Export

Send traces to an OTel Collector:

```bash
go run ./demo/rag-service --bind :8080 --metrics-bind :2113 \
  --otlp-endpoint localhost:4317
```

Or set the environment variable:

```bash
OTEL_EXPORTER_OTLP_ENDPOINT=localhost:4317 go run ./demo/rag-service
```

## Kubernetes Deployment

```bash
kubectl apply -k demo/rag-service/k8s
kubectl -n default port-forward svc/rag-service 8080:8080
```

## Prometheus Metrics

| Metric | Type | Description |
|---|---|---|
| `llm_slo_ttft_ms` | histogram | Time-to-first-token (ms) |
| `llm_slo_tokens_per_sec` | histogram | Token generation throughput |
| `llm_slo_retrieval_vectordb_ms` | histogram | Vector DB latency component |
| `llm_slo_retrieval_network_ms` | histogram | Network latency component |
| `llm_slo_retrieval_dns_ms` | histogram | DNS latency component |
| `llm_slo_requests_total` | counter | Request count by `{status, profile}` |
| `llm_slo_correlation_total` | counter | Correlation decisions by `{tier, enriched}` |

### Example PromQL Queries

```promql
# TTFT p95
histogram_quantile(0.95, sum(rate(llm_slo_ttft_ms_bucket[5m])) by (le))

# Token throughput p50
histogram_quantile(0.50, sum(rate(llm_slo_tokens_per_sec_bucket[5m])) by (le))

# Error rate
sum(rate(llm_slo_requests_total{status!="ok"}[5m])) / sum(rate(llm_slo_requests_total[5m]))

# Correlation enrichment rate
sum(rate(llm_slo_correlation_total{enriched="true"}[5m])) / sum(rate(llm_slo_correlation_total[5m]))
```

## Load Profiles

| Profile | TTFT Range | Cadence | Retrieval Weight | Use Case |
|---|---|---|---|---|
| `chat_short` | 25-40 ms | 25-35 ms | Light | Quick chat, low retrieval |
| `rag_medium` | 30-50 ms | 30-45 ms | Medium | Standard RAG workload |
| `context_long` | 50-80 ms | 45-65 ms | Heavy | Large context, deep retrieval |

## Flags

| Flag | Default | Description |
|---|---|---|
| `--bind` | `:8080` | HTTP API bind address |
| `--metrics-bind` | `:2113` | Prometheus metrics bind address |
| `--service-name` | `rag-service` | OTel `service.name` resource attribute |
| `--otlp-endpoint` | (empty) | OTLP gRPC endpoint; uses stdout exporter when empty |
| `--fixtures` | `demo/rag-service/fixtures/corpus.json` | Path to corpus fixture file |
