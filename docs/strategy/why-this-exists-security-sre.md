# Why LLM SLO eBPF Toolkit Exists (Security and SRE Narrative)

SRE organizations depend on telemetry quality, but the hardest production systems often have the least consistent instrumentation. LLM workloads make this worse: each request can span ingress, application code, retrieval systems, policy layers, and external model providers. When SLOs degrade, teams often get contradictory signals from logs, traces, and vendor dashboards.

LLM SLO eBPF Toolkit exists to provide ground truth from the kernel outward. It does not replace application instrumentation; it makes operations safer when instrumentation is missing, delayed, or incomplete.

By using eBPF-derived signals at protocol and runtime boundaries, the toolkit can provide baseline visibility quickly without per-service code changes. That matters for SRE response time and for security posture, because blind spots directly increase incident duration.

The second problem is semantic mismatch. Traditional RED/USE metrics are necessary but insufficient for LLM workloads. Operators need decomposed indicators tied to user experience and model economics: time-to-first-token, provider tail latency, token throughput collapse, retrieval latency contribution, and policy/guardrail overhead.

Without these LLM-specific slices, teams guess root cause and apply expensive mitigations that may not improve reliability. The toolkit aims to close that loop by correlating kernel-level signals with OTel spans and Kubernetes workload identity.

This project is intentionally Kubernetes-first and open benchmark-oriented. Security and platform teams can inspect what is collected, how it is processed, and what overhead it introduces. The benchmark model uses repeatable fault injection so attribution claims are measurable, not anecdotal.

There are real constraints: kernel compatibility, event volume management, and attribution uncertainty in multi-fault conditions. The toolkit treats these as first-class engineering concerns and makes uncertainty explicit in reports.

Why this exists now: LLM systems are entering critical production paths faster than reliability instrumentation standards are maturing. Teams need an open, practical toolkit that can produce trustworthy SLO diagnostics and shorten mean time to resolution.
