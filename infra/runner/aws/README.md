# AWS Ephemeral Runner (`self-hosted,linux,ebpf`)

This Terraform stack provisions one or more EC2 hosts that run continuously re-registering **ephemeral** GitHub Actions runners.

## What It Creates
- 1x Ubuntu 22.04 EC2 instance (`t3a.xlarge` default), or multiple profile-specific instances
- Encrypted 80GB gp3 root volume
- Security group with **no inbound** rules, egress TCP/443 only
- IAM role + instance profile:
  - `AmazonSSMManagedInstanceCore`
  - `ssm:GetParameter` on one SecureString path (PAT)
- cloud-init bootstrap for Docker + kind + kubectl + helm + runner service
  - bootstrap rewrites apt mirrors to `https://` so package install works with egress `443` only
  - bootstrap installs AWS CLI v2 fallback when distro package is unavailable
  - runner labels include baseline `self-hosted,linux,ebpf` and can auto-append `kernel-x-y`

## Prerequisites
1. Terraform >= 1.6
2. AWS credentials with IAM/EC2/SSM permissions
3. PAT stored in SSM Parameter Store (SecureString):

```bash
aws ssm put-parameter \
  --name /llm-slo/github/runner_pat \
  --type SecureString \
  --value '<github_pat>' \
  --overwrite
```

PAT should allow repo runner registration for your repository.

## Deploy

```bash
cd infra/runner/aws
terraform init
terraform apply \
  -var 'vpc_id=vpc-xxxx' \
  -var 'subnet_id=subnet-xxxx' \
  -var 'github_repository=ogulcanaydogan/LLM-SLO-eBPF-Toolkit'
```

## Deploy Dual Kernel Profiles (example)

Create `terraform.tfvars`:

```hcl
aws_region        = "us-east-1"
vpc_id            = "vpc-xxxx"
github_repository = "ogulcanaydogan/LLM-SLO-eBPF-Toolkit"
runner_name_prefix = "llm-slo-ebpf"

runner_profiles = {
  kernel_5_15 = {
    subnet_id    = "subnet-aaaa"
    ami_id       = "ami-5-15"
    extra_labels = ["kernel-5-15"]
  }
  kernel_6_8 = {
    subnet_id    = "subnet-bbbb"
    ami_id       = "ami-6-8"
    extra_labels = ["kernel-6-8"]
  }
}
```

Then apply:

```bash
terraform apply
```

## Validate Runner

```bash
gh api repos/ogulcanaydogan/LLM-SLO-eBPF-Toolkit/actions/runners \
  --jq '.runners[] | {name,status,labels:[.labels[].name]}'
```

Expected labels include:
- `self-hosted`
- `linux`
- `ebpf`
- kernel label:
  - auto detected: `kernel-x-y` (from `uname -r`)
  - optional explicit profile labels: `kernel-5-15`, `kernel-6-8`

Quick profile coverage check:

```bash
cd <repo-root>
RUNNER_STATUS_TOKEN="$(gh auth token)" \
GITHUB_REPOSITORY=ogulcanaydogan/LLM-SLO-eBPF-Toolkit \
./scripts/ci/check_runner_profiles.sh --profiles kernel-5-15,kernel-6-8 --out /tmp/runner-profiles.json

jq . /tmp/runner-profiles.json
```

## Validate Full Privileged Path
Trigger weekly workflow and confirm `full-benchmark-matrix` starts (not fallback):

```bash
gh workflow run weekly-benchmark.yml
```

Then inspect run jobs:

```bash
gh run list --workflow weekly-benchmark.yml --limit 1
gh run view <run-id> --log
```

## Operations
- Session access (no SSH): use SSM.

```bash
aws ssm start-session --target <instance-id>
```

- Runner service status:

```bash
sudo systemctl status gha-ephemeral-runner.service
journalctl -u gha-ephemeral-runner.service -n 200 --no-pager
```

## Notes
- This is a fast-evidence single-host setup. For scale/HA, migrate to autoscaled runner sets in a later phase.
- If your subnet is private, ensure NAT egress to GitHub endpoints and package mirrors.
