# Demo RAG Service

Deterministic streaming RAG stub with:
- seeded retrieval fixture selection
- OTel spans (`chat.request`, `chat.retrieval`, `chat.generation`)
- Prometheus SLO metrics (`TTFT`, `tokens/sec`, retrieval components)
- DNS correlation enrichment (`llm.ebpf.dns.latency_ms`) with confidence gate

## Local run

```bash
go run ./demo/rag-service --bind :8080 --metrics-bind :2113
```

Example request:

```bash
curl -sS -N http://localhost:8080/chat \
  -H 'content-type: application/json' \
  -d '{"prompt":"Explain DNS impact on TTFT","profile":"rag_medium","seed":42,"max_tokens":20,"stream":true}'
```

## Kubernetes run

```bash
kubectl apply -k demo/rag-service/k8s
```
