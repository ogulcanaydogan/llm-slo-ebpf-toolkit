# AWS Ephemeral Runner (`self-hosted,linux,ebpf`)

This Terraform stack provisions one EC2 host that runs a continuously re-registering **ephemeral** GitHub Actions runner.

## What It Creates
- 1x Ubuntu 22.04 EC2 instance (`t3a.xlarge` default)
- Encrypted 80GB gp3 root volume
- Security group with **no inbound** rules, egress TCP/443 only
- IAM role + instance profile:
  - `AmazonSSMManagedInstanceCore`
  - `ssm:GetParameter` on one SecureString path (PAT)
- cloud-init bootstrap for Docker + kind + kubectl + helm + runner service
  - bootstrap rewrites apt mirrors to `https://` so package install works with egress `443` only

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

## Validate Runner

```bash
gh api repos/ogulcanaydogan/LLM-SLO-eBPF-Toolkit/actions/runners \
  --jq '.runners[] | {name,status,labels:[.labels[].name]}'
```

Expected labels include:
- `self-hosted`
- `linux`
- `ebpf`

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
