# Kubernetes Collector Deployment

Apply collector manifests:

```bash
kubectl apply -k deploy/k8s
```

Delete collector manifests:

```bash
kubectl delete -k deploy/k8s
```

Notes:
- The DaemonSet uses a privileged security context for eBPF access.
- Update the container image in `deploy/k8s/daemonset.yaml` for your release.
- Default collector args run synthetic mixed-fault stream mode (`--count=0`) for baseline SLO signal generation.
- Manifests are intended as a baseline and should be adapted to your cluster hardening policy.

Check emitted SLO events:

```bash
kubectl -n llm-slo-system logs -l app=llm-slo-collector --tail=20
```
