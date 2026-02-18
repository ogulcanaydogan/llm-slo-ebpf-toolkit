# LLM SLO eBPF Toolkit

Kubernetes-first reliability and security observability toolkit for LLM workloads using eBPF-grounded telemetry.

## Quick Start
1. Install Go 1.23+.
2. Validate prerequisites:
```bash
go run ./cmd/sloctl prereq check
```
3. Run build + tests:
```bash
make build
make test
```
4. Start local kind cluster:
```bash
make kind-up
```
5. Run agent sample output:
```bash
go run ./cmd/agent --count 3 --output stdout
```
6. Run deterministic RAG demo service:
```bash
go run ./demo/rag-service --bind :8080 --metrics-bind :2113
```
7. Run collector sample output:
```bash
go run ./cmd/collector
```
8. Run attribution sample output:
```bash
go run ./cmd/attributor --out -
```
9. Deploy local observability stack:
```bash
kubectl apply -k deploy/observability
```
10. Generate benchmark skeleton artifacts:
```bash
go run ./cmd/benchgen --out artifacts/benchmarks --scenario provider_throttle
```
11. Generate mixed-fault benchmark artifacts:
```bash
go run ./cmd/benchgen --out artifacts/benchmarks-mixed --scenario mixed_faults
```
12. Run benchmark using explicit JSONL fault stream:
```bash
go run ./cmd/benchgen --out artifacts/benchmarks-input --input pkg/benchmark/testdata/mixed_fault_samples.jsonl
```
13. Generate replay samples for multi-domain faults:
```bash
go run ./cmd/faultreplay --scenario mixed --count 30 --out artifacts/fault-replay/fault_samples.jsonl
```
14. Build benchmark/report bundle from replay samples:
```bash
go run ./cmd/benchgen --out artifacts/benchmarks-replay --scenario mixed_faults --input artifacts/fault-replay/fault_samples.jsonl
```
15. Generate collector raw samples via fault injection harness:
```bash
go run ./cmd/faultinject --scenario mixed --count 24 --out artifacts/fault-injection/raw_samples.jsonl
```
16. Normalize injected raw samples with collector pipeline:
```bash
go run ./cmd/collector --input artifacts/fault-injection/raw_samples.jsonl --output jsonl --output-path artifacts/collector/slo-events.jsonl
```
17. Send agent events directly to OTLP collector:
```bash
go run ./cmd/agent --count 3 --output otlp --otlp-endpoint http://127.0.0.1:4318/v1/logs
```
18. Run fault-matrix benchmark hooks:
```bash
make chaos-matrix
```
19. Run kind observability smoke:
```bash
make kind-observability-smoke
```
20. Tear down kind cluster:
```bash
make kind-down
```

## Kubernetes Deployment Skeleton
```bash
kubectl apply -k deploy/k8s
```

## Differentiation Artifacts
- `docs/strategy/differentiation-strategy.md`
- `docs/strategy/why-this-exists-security-sre.md`
- `docs/strategy/killer-demo-stories.md`
- `docs/strategy/v0.2-build-plan.md`
- `docs/strategy/v0.2-go-no-go-checklist.md`
- `docs/benchmarks/llm-slo-attribution-accuracy.md`
- `docs/benchmarks/output-schema.md`
- `docs/contracts/v1/slo-event.schema.json`
- `docs/contracts/v1/incident-attribution.schema.json`
- `docs/contracts/v1alpha1/probe-event.schema.json`
- `config/toolkit.schema.json`
- `docs/security/self-hosted-runner-baseline.md`
- `docs/research/landscape-sources.md`

## Positioning Snapshot
- Audience: SRE, platform, and security engineering teams operating LLM workloads on Kubernetes.
- Core claim: kernel-grounded telemetry closes instrumentation blind spots and improves SLO incident attribution.
- Wedge: LLM-specific SLI semantics + causal mapping from network/runtime events to user-facing SLO burn.

## Immediate Next Steps
1. Implement against `docs/strategy/v0.2-build-plan.md` and track release gates in `docs/strategy/v0.2-go-no-go-checklist.md`.
2. Replace synthetic generator defaults with real probe-fed signal ingestion in `cmd/agent`.
3. Add alert routing and SLO burn-rate dashboard panels for incident triage.
4. Extend nightly workflow to publish matrix benchmark trend diffs as markdown.
