variable "aws_region" {
  description = "AWS region for the runner instance"
  type        = string
  default     = "us-east-1"
}

variable "vpc_id" {
  description = "VPC ID where the runner instance will be created"
  type        = string
}

variable "subnet_id" {
  description = "Subnet ID where the runner instance will be created"
  type        = string
  default     = null
}

variable "github_repository" {
  description = "GitHub repository in owner/repo format"
  type        = string
  default     = "ogulcanaydogan/LLM-SLO-eBPF-Toolkit"
}

variable "runner_pat_parameter_name" {
  description = "SSM SecureString parameter name that stores the GitHub PAT"
  type        = string
  default     = "/llm-slo/github/runner_pat"
}

variable "instance_type" {
  description = "Runner EC2 instance type"
  type        = string
  default     = "t3a.xlarge"
}

variable "root_volume_gb" {
  description = "Root volume size in GB"
  type        = number
  default     = 80
}

variable "runner_name_prefix" {
  description = "Prefix for ephemeral runner registrations"
  type        = string
  default     = "llm-slo-ebpf"
}

variable "runner_default_labels" {
  description = "Default labels added to every runner registration"
  type        = list(string)
  default     = ["self-hosted", "linux", "ebpf"]
}

variable "append_kernel_version_label" {
  description = "When true, runner auto-appends kernel-x-y label based on uname -r"
  type        = bool
  default     = true
}

variable "runner_profiles" {
  description = "Optional per-profile runner configuration map. If empty, one default runner is created from top-level variables."
  type = map(object({
    subnet_id          = optional(string)
    instance_type      = optional(string)
    root_volume_gb     = optional(number)
    ami_id             = optional(string)
    runner_name_prefix = optional(string)
    extra_labels       = optional(list(string))
  }))
  default = {}
}

variable "runner_version" {
  description = "GitHub Actions runner version"
  type        = string
  default     = "2.323.0"
}

variable "tags" {
  description = "Extra tags for AWS resources"
  type        = map(string)
  default     = {}
}
