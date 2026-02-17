# LLM SLO eBPF Toolkit Killer Demo Stories

## Demo 1: 5-Minute Install, First SLO Dashboard
### Setup
- Deploy DaemonSet collector in a sample Kubernetes cluster.
- Run synthetic LLM request load against two namespaces.

### What to show
- Auto-discovered LLM service map.
- Baseline RED metrics plus TTFT distribution.
- Zero-code instrumentation for initial visibility.

### Win condition
- Useful SLO dashboard appears without app code changes.

## Demo 2: TTFT Spike Autopsy
### Setup
- Inject DNS or egress latency for one workload path.

### What to show
- TTFT SLO burn detected in near real-time.
- Attribution points to network degradation path.
- Affected namespaces/services identified.

### Win condition
- Operator can isolate likely root cause in minutes.

## Demo 3: Noisy Neighbor Fairness Incident
### Setup
- Saturate CPU/cgroup for one tenant workload.

### What to show
- Token throughput degradation for neighboring workloads.
- Burn-rate alert with culprit workload correlation.
- Resource-level evidence connected to request-level impact.

### Win condition
- Platform team can act on concrete offender attribution.

## Demo 4: Provider 429 Storm Separation
### Setup
- Simulate upstream provider throttling errors.

### What to show
- Distinguish upstream saturation from in-cluster bottlenecks.
- Error budget burn segmented by failure class.
- Suggested mitigation path (backoff/routing/cap adjustments).

### Win condition
- Incident responders avoid misdiagnosing cluster internals.

## Demo 5: Canary Release SLO Gate
### Setup
- Compare stable vs canary workloads under same test profile.

### What to show
- Automated regression detection on TTFT/tail latency/error rate.
- Confidence signal for rollback or promotion.
- Artifacted report for change-review trail.

### Win condition
- Release pipeline blocks reliability regressions with measurable criteria.
