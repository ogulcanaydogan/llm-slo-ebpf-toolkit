# kind Bootstrap

Create cluster:

```bash
make kind-up
```

Delete cluster:

```bash
make kind-down
```

Defaults:
- cluster name: `llm-slo-lab`
- node count: 3 (1 control-plane, 2 workers)

Best-effort tool bootstrap:

```bash
./deploy/kind/bootstrap-tools.sh
# or AUTO_INSTALL=1 ./deploy/kind/bootstrap-tools.sh
```

Observability smoke:

```bash
make kind-observability-smoke
```
