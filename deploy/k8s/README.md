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
- Manifests are intended as a baseline and should be adapted to your cluster hardening policy.
