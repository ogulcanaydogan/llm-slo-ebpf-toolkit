# Self-Hosted Runner Security Baseline (Privileged eBPF CI)

Scope: nightly and weekly Linux jobs that require privileged eBPF and kind integration.

## Required Controls
1. Ephemeral runners only.
- One job, one runner lifecycle.
- Runner is destroyed after each workflow.

2. Isolated execution environment.
- Dedicated node pool or standalone host class for CI runners.
- No co-location with production workloads.

3. Credential minimization.
- Short-lived tokens only.
- No long-lived cloud keys on runner disk.
- Least-privilege repository and artifact permissions.

4. Artifact and log hygiene.
- Scrub secrets and environment material before artifact upload.
- Reject uploads that contain kubeconfigs, tokens, or private keys.

5. Network policy posture.
- Restrict egress to required package mirrors, registry, and GitHub endpoints.
- Block lateral movement to internal non-CI systems.

6. Host hardening.
- Keep kernel and container runtime patched.
- Enforce audit logging for privileged job startup, teardown, and artifact upload actions.

## Workflow Requirements
- PR CI on GitHub-hosted runners must not require privileged eBPF execution.
- Privileged jobs run only on trusted branches and scheduled workflows.
- Weekly full benchmark jobs must publish provenance metadata for traceability.
