# Contributing to eBPF + LLM Inference SLO Toolkit

Thank you for considering a contribution. This document covers the build prerequisites, development workflow, and conventions used in this project.

## Prerequisites

| Tool | Version | Purpose |
|------|---------|---------|
| Go | >= 1.22 | Build and test |
| Make | any | Task runner |
| Docker | >= 24 | Container builds |
| kind | >= 0.20 | Local Kubernetes cluster |
| Helm | >= 3.14 | Chart linting and templating |
| jq | any | JSON processing in scripts |

Optional (for eBPF development on Linux):

| Tool | Version | Purpose |
|------|---------|---------|
| clang | >= 15 | BPF C compilation |
| llvm-strip | >= 15 | Strip debug info from BPF objects |
| bpftool | any | BTF and skeleton generation |
| libbpf-dev | >= 1.0 | CO-RE headers |

## Getting Started

```bash
git clone https://github.com/ogulcanaydogan/llm-slo-ebpf-toolkit.git
cd llm-slo-ebpf-toolkit
go build ./...
go test ./...
```

## Common Make Targets

```bash
make test                # Run all Go tests
make lint                # golangci-lint
make schema-validate     # Validate JSON schemas and contract samples
make correlation-gate    # Run correlation quality gate
make bench               # Generate benchmark artifacts
make helm-lint           # Lint Helm chart
make helm-template       # Template Helm chart
make cdgate-smoke        # CD gate smoke test
make bench-multi         # Multi-fault benchmark scenario
```

## Running the Full CI Locally

```bash
go build ./...
go test ./...
make schema-validate
make correlation-gate
make helm-lint
make helm-template
```

## Project Layout

- `cmd/` — CLI entry points (one directory per binary).
- `pkg/` — Library packages (collector, attribution, correlation, signals, webhook, cdgate, etc.).
- `ebpf/` — eBPF C source and generated Go bindings.
- `charts/` — Helm chart.
- `config/` — Default `toolkit.yaml` and JSON schema for config validation.
- `deploy/` — Kubernetes manifests (kustomize overlays, observability stack, alerts).
- `docs/` — Architecture, contracts, release notes, strategy documents.
- `test/` — Integration and incident lab test fixtures.
- `scripts/` — CI and chaos automation scripts.

## Commit Conventions

This project follows conventional commit prefixes:

| Prefix | Use |
|--------|-----|
| `feat:` | New feature or capability |
| `fix:` | Bug fix |
| `chore:` | Maintenance, deps, config |
| `docs:` | Documentation only |
| `refactor:` | Code restructure (no behavior change) |
| `test:` | Adding or updating tests |

Keep the first line under 72 characters. Use imperative mood ("add" not "added").

## Branch Naming

```
feature/<description>
fix/<description>
chore/<description>
docs/<description>
```

## Pull Request Process

1. Fork the repository and create a feature branch.
2. Ensure `go build ./...` and `go test ./...` pass.
3. Run `make schema-validate` and `make correlation-gate` locally.
4. Write a concise PR title and description explaining the change.
5. One approval required before merge.

## Code Style

- Follow standard Go formatting (`gofmt`).
- Use `golangci-lint` for static analysis.
- Keep packages small and focused — one responsibility per package.
- All public APIs must have doc comments.
- Schema-validated contracts live under `docs/contracts/`.

## Testing

- Unit tests live alongside their packages (`*_test.go`).
- Integration tests requiring Kubernetes run in `test/integration-kind/`.
- eBPF tests requiring a Linux host with BTF use synthetic fallback in CI on non-Linux runners.
- Benchmark artifacts are generated via `cmd/benchgen` and validated in `pkg/benchmark/`.

## License

By contributing, you agree that your contributions will be licensed under the MIT License.
