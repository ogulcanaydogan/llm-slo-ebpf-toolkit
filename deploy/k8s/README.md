# Kubernetes Agent Deployment

Apply agent manifests:

```bash
kubectl apply -k deploy/k8s
```

Delete agent manifests:

```bash
kubectl delete -k deploy/k8s
```

Notes:
- The DaemonSet uses a privileged security context for eBPF access.
- Update the container image in `deploy/k8s/daemonset.yaml` for your release.
- Default agent args run synthetic stream mode (`--count=0`) and expose heartbeat/health endpoints on port `2112`.
- Manifests are intended as a baseline and should be adapted to your cluster hardening policy.

Check emitted SLO events:

```bash
kubectl -n llm-slo-system logs -l app=llm-slo-agent --tail=20
```

Switch to OTLP output mode:

```bash
./scripts/chaos/set_agent_mode.sh mixed otlp
```
