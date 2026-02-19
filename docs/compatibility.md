# Kernel Compatibility Matrix

This page tracks compatibility checks for privileged eBPF execution across supported runner kernel profiles.

- Generated at (UTC): 2026-02-19T20:49:48Z
- Source run: `22199345650`
- Report source directory: `artifacts/compatibility`

## Matrix

| Profile Label | Availability | Kernel Release | BTF | `sloctl prereq` | `agent --probe-smoke` |
|---|---|---|---|---|---|
| `kernel-5-15` | unavailable | `n/a` | `n/a` | `n/a` | `n/a` |
| `kernel-6-8` | unavailable | `n/a` | `n/a` | `n/a` | `n/a` |

## Interpretation

- `available`: matrix job ran on a runner matching the profile label.
- `unavailable`: no online runner with the requested label was detected in preflight.
- `prereq.status=pass`: local kernel/tooling/capability checks passed for that runner.
- `probe_smoke.status=pass`: probe loader smoke succeeded (or `skipped` when root privileges were unavailable).

## Notes

- These checks are intended as compatibility signals, not full performance regressions.
- Full SLO/perf and incident reproducibility gates remain in weekly benchmark workflows.
