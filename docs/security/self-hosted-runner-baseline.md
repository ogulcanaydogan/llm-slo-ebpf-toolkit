# Self-Hosted Runner Security Baseline (Privileged eBPF CI)

Scope: nightly and weekly Linux jobs that require privileged eBPF and kind integration.

Implementation reference for the single-runner fast-evidence setup:
- `infra/runner/aws/main.tf`
- `infra/runner/aws/cloud-init.yaml`
- `scripts/runner/register-and-run-loop.sh`

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
- Standard PR CI on GitHub-hosted runners must not require privileged eBPF execution.
- Dedicated PR privileged smoke workflow runs on self-hosted `linux+ebpf` runners for trusted (same-repo) pull requests.
- Fork pull requests must not execute privileged self-hosted jobs.
- Weekly full benchmark jobs must publish provenance metadata for traceability.
- Nightly and weekly workflows must execute runner preflight and switch to synthetic fallback mode when no online `self-hosted+linux+ebpf` runner is available.

## Online Validation
Use GitHub API to verify runner registration state and labels:

```bash
gh api repos/ogulcanaydogan/LLM-SLO-eBPF-Toolkit/actions/runners \
  --jq '.runners[] | {name,status,labels:[.labels[].name]}'
```

Expected outcome:
- at least one runner reports `status: "online"`
- labels include `self-hosted`, `linux`, and `ebpf`
