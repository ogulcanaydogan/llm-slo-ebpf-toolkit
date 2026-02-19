terraform {
  required_version = ">= 1.5.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

data "aws_caller_identity" "current" {}

data "aws_ami" "ubuntu_2204" {
  most_recent = true
  owners      = ["099720109477"] # Canonical

  filter {
    name   = "name"
    values = ["ubuntu/images/hvm-ssd/ubuntu-jammy-22.04-amd64-server-*"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

locals {
  base_tags = {
    project    = "llm-slo-ebpf-toolkit"
    role       = "github-runner"
    capability = "ebpf"
    managed_by = "terraform"
  }
  tags = merge(local.base_tags, var.tags)

  default_runner_profile = {
    default = {
      subnet_id          = var.subnet_id
      instance_type      = var.instance_type
      root_volume_gb     = var.root_volume_gb
      ami_id             = null
      runner_name_prefix = var.runner_name_prefix
      extra_labels       = []
    }
  }

  runner_profiles = length(var.runner_profiles) > 0 ? var.runner_profiles : local.default_runner_profile
}

check "runner_subnet_configuration" {
  assert {
    condition     = length(var.runner_profiles) > 0 || (var.subnet_id != null && trim(var.subnet_id) != "")
    error_message = "subnet_id must be set when runner_profiles is empty."
  }

  assert {
    condition = alltrue([
      for _, profile in var.runner_profiles :
      ((var.subnet_id != null && trim(var.subnet_id) != "") || (try(profile.subnet_id, null) != null && trim(try(profile.subnet_id, "")) != ""))
    ])
    error_message = "Each runner profile must define subnet_id or use top-level subnet_id."
  }
}

data "aws_iam_policy_document" "assume_ec2" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ec2.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "runner" {
  name               = "llm-slo-ephemeral-runner-role"
  assume_role_policy = data.aws_iam_policy_document.assume_ec2.json
  tags               = local.tags
}

resource "aws_iam_role_policy_attachment" "ssm_core" {
  role       = aws_iam_role.runner.name
  policy_arn = "arn:aws:iam::aws:policy/AmazonSSMManagedInstanceCore"
}

data "aws_iam_policy_document" "runner_ssm_param" {
  statement {
    sid = "ReadRunnerPAT"
    actions = [
      "ssm:GetParameter"
    ]
    resources = [
      "arn:aws:ssm:${var.aws_region}:${data.aws_caller_identity.current.account_id}:parameter${var.runner_pat_parameter_name}"
    ]
  }
}

resource "aws_iam_policy" "runner_ssm_param" {
  name   = "llm-slo-runner-ssm-param"
  policy = data.aws_iam_policy_document.runner_ssm_param.json
  tags   = local.tags
}

resource "aws_iam_role_policy_attachment" "runner_ssm_param" {
  role       = aws_iam_role.runner.name
  policy_arn = aws_iam_policy.runner_ssm_param.arn
}

resource "aws_iam_instance_profile" "runner" {
  name = "llm-slo-ephemeral-runner-profile"
  role = aws_iam_role.runner.name
  tags = local.tags
}

resource "aws_security_group" "runner" {
  name        = "llm-slo-ephemeral-runner-sg"
  description = "No inbound access; HTTPS-only egress for GitHub runner"
  vpc_id      = var.vpc_id

  egress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = local.tags
}

resource "aws_instance" "runner" {
  for_each = local.runner_profiles

  ami = try(each.value.ami_id, "") != "" ? each.value.ami_id : data.aws_ami.ubuntu_2204.id

  instance_type               = coalesce(try(each.value.instance_type, null), var.instance_type)
  subnet_id                   = coalesce(try(each.value.subnet_id, null), var.subnet_id)
  vpc_security_group_ids      = [aws_security_group.runner.id]
  iam_instance_profile        = aws_iam_instance_profile.runner.name
  associate_public_ip_address = true

  metadata_options {
    http_endpoint               = "enabled"
    http_tokens                 = "required"
    http_put_response_hop_limit = 1
    instance_metadata_tags      = "enabled"
  }

  root_block_device {
    encrypted   = true
    volume_size = coalesce(try(each.value.root_volume_gb, null), var.root_volume_gb)
    volume_type = "gp3"
  }

  user_data = templatefile("${path.module}/cloud-init.yaml", {
    github_repository         = var.github_repository
    runner_pat_parameter_name = var.runner_pat_parameter_name
    runner_name_prefix = coalesce(
      try(each.value.runner_name_prefix, null),
      length(var.runner_profiles) > 0 ? "${var.runner_name_prefix}-${each.key}" : var.runner_name_prefix
    )
    runner_version              = var.runner_version
    runner_default_labels_csv   = join(",", var.runner_default_labels)
    runner_extra_labels_csv     = join(",", try(each.value.extra_labels, []))
    append_kernel_version_label = var.append_kernel_version_label ? "true" : "false"
  })

  tags = merge(local.tags, {
    Name           = length(var.runner_profiles) > 0 ? "llm-slo-ephemeral-runner-${each.key}" : "llm-slo-ephemeral-runner"
    runner_profile = each.key
  })
}
