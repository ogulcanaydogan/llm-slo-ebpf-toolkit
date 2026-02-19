output "instance_ids" {
  description = "EC2 instance IDs for runner hosts keyed by profile"
  value       = { for profile, instance in aws_instance.runner : profile => instance.id }
}

output "instance_private_ips" {
  description = "Private IP addresses for runner hosts keyed by profile"
  value       = { for profile, instance in aws_instance.runner : profile => instance.private_ip }
}

output "ssm_start_sessions" {
  description = "SSM session commands keyed by profile"
  value = {
    for profile, instance in aws_instance.runner :
    profile => "aws ssm start-session --target ${instance.id} --region ${var.aws_region}"
  }
}

output "instance_id" {
  description = "Backward-compatible single instance ID (null when multiple profiles are defined)"
  value       = length(aws_instance.runner) == 1 ? values(aws_instance.runner)[0].id : null
}

output "instance_private_ip" {
  description = "Backward-compatible single private IP (null when multiple profiles are defined)"
  value       = length(aws_instance.runner) == 1 ? values(aws_instance.runner)[0].private_ip : null
}

output "ssm_start_session" {
  description = "Backward-compatible single SSM command (null when multiple profiles are defined)"
  value       = length(aws_instance.runner) == 1 ? "aws ssm start-session --target ${values(aws_instance.runner)[0].id} --region ${var.aws_region}" : null
}

output "validation_runner_labels" {
  description = "Expected baseline runner labels in GitHub"
  value       = concat(var.runner_default_labels, var.append_kernel_version_label ? ["kernel-x-y (auto)"] : [])
}

output "runner_profiles" {
  description = "Configured runner profiles"
  value       = keys(aws_instance.runner)
}

output "validation_gh_command" {
  description = "GitHub API command to verify runner status"
  value       = "gh api repos/${var.github_repository}/actions/runners --jq '.runners[] | {name,status,labels:[.labels[].name]}'"
}
