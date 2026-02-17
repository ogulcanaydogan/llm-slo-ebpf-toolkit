# LLM SLO eBPF Toolkit Differentiation Strategy

## Goal
Define an honest, technical differentiation stance for a Kubernetes-first toolkit that improves LLM reliability/security operations through eBPF-grounded telemetry.

## Scope and Assumptions
- Primary environment: Kubernetes multi-tenant clusters.
- Integration posture: OpenTelemetry-compatible outputs.
- Product boundary: reliability/security diagnostics, not full APM replacement.

## Competitor and Adjacent Landscape (Top 10)

| Tool | What it does well | What it misses for this target |
|---|---|---|
| OpenTelemetry eBPF Instrumentation (OBI) | Zero-code protocol telemetry and strong OSS momentum | Generic signals; lacks opinionated LLM SLO semantics |
| Pixie | Rich K8s troubleshooting from eBPF data | Limited first-class LLM SLI and burn-rate model |
| Cilium + Hubble | Strong network identity, flow visibility, and policy context | Network-centric view, not full LLM SLO attribution pipeline |
| Tetragon | Runtime security events and policy enforcement telemetry | Security-event centric, not SLO-centric LLM workload diagnostics |
| Parca | Low-overhead continuous profiling | Profiling depth, but not request/token SLO decomposition |
| Coroot | Integrated eBPF observability plus SLO functions | Broad platform scope; LLM-specific semantics are not primary |
| Datadog USM | Production-grade service discovery and flow mapping | Closed product and less reproducible open benchmark posture |
| Elastic Universal Profiling | Broad production profiling with eBPF | Profiling-centric, weaker direct LLM transactional SLO framing |
| Odigos | Fast auto-instrumentation and OTel pipeline | Tracing-first; weaker kernel-level root-cause attribution by default |
| Langfuse (adjacent) | LLM traces/evals and application-level analytics | Does not provide kernel/network causality for infrastructure SLO failures |

## Honest Gap Assessment (What We Must Execute Well)
- eBPF portability across kernel versions can impact deployment reliability.
- Attribution claims are easy to overstate; must publish false-positive/false-negative rates.
- Kernel telemetry volume can be expensive without careful sampling and aggregation.
- LLM-specific metrics (for example TTFT) require robust parsing for diverse protocols/providers.

## Unique Wedge (3 Pillars)
1. Kernel-grounded LLM SLO telemetry with no-code coverage.
2. LLM-native SLI model: TTFT, token throughput, retrieval contribution, provider error classes.
3. Causal bridge: correlate eBPF events, OTel spans, and kube metadata for incident attribution.

## Differentiation by Buyer Role
- SRE: faster root-cause localization during latency/error budget burn.
- Platform: standardized SLO telemetry across inconsistent app instrumentation.
- Security: visibility into provider egress and runtime anomalies that impact reliability posture.

## Positioning Statement
LLM SLO eBPF Toolkit is a Kubernetes-first reliability observability layer that uses kernel-grounded data to explain why LLM SLOs are burning, even when application instrumentation is incomplete.

## Proof Requirements for Credibility
- Public fault-injection benchmark runs with raw attribution outputs.
- Versioned event schemas for interoperability.
- Transparent overhead measurements (CPU, memory, event rates) under load.
