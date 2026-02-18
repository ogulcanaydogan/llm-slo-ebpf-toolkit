# LLM SLO eBPF Toolkit

Kubernetes-first reliability and security observability toolkit for LLM workloads using eBPF-grounded telemetry.

## Quick Start
1. Install Go 1.23+.
2. Run `make test`.
3. Run collector sample output:
```bash
go run ./cmd/collector
```
4. Run attribution sample output:
```bash
go run ./cmd/attributor --out -
```
5. Generate benchmark skeleton artifacts:
```bash
go run ./cmd/benchgen --out artifacts/benchmarks --scenario provider_throttle
```
6. Generate mixed-fault benchmark artifacts:
```bash
go run ./cmd/benchgen --out artifacts/benchmarks-mixed --scenario mixed_faults
```
7. Run benchmark using explicit JSONL fault stream:
```bash
go run ./cmd/benchgen --out artifacts/benchmarks-input --input pkg/benchmark/testdata/mixed_fault_samples.jsonl
```
8. Generate replay samples for multi-domain faults:
```bash
go run ./cmd/faultreplay --scenario mixed --count 30 --out artifacts/fault-replay/fault_samples.jsonl
```
9. Build benchmark/report bundle from replay samples:
```bash
go run ./cmd/benchgen --out artifacts/benchmarks-replay --scenario mixed_faults --input artifacts/fault-replay/fault_samples.jsonl
```

## Kubernetes Deployment Skeleton
```bash
kubectl apply -k deploy/k8s
```

## Differentiation Artifacts
- `docs/strategy/differentiation-strategy.md`
- `docs/strategy/why-this-exists-security-sre.md`
- `docs/strategy/killer-demo-stories.md`
- `docs/benchmarks/llm-slo-attribution-accuracy.md`
- `docs/benchmarks/output-schema.md`
- `docs/contracts/v1/slo-event.schema.json`
- `docs/contracts/v1/incident-attribution.schema.json`
- `docs/research/landscape-sources.md`

## Positioning Snapshot
- Audience: SRE, platform, and security engineering teams operating LLM workloads on Kubernetes.
- Core claim: kernel-grounded telemetry closes instrumentation blind spots and improves SLO incident attribution.
- Wedge: LLM-specific SLI semantics + causal mapping from network/runtime events to user-facing SLO burn.

## Immediate Next Steps
1. Build minimal DaemonSet collector prototype and emit schema-compliant events.
2. Add fault-injection harness for attribution benchmark scenarios.
3. Publish baseline attribution report under `docs/benchmarks/reports/`.
