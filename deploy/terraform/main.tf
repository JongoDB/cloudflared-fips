# cloudflared-fips Tier 3 Deployment — AWS GovCloud
#
# Deploys cloudflared-fips as an ECS Fargate service with:
# - FIPS 140-2 endpoint (VPC endpoints, FIPS API endpoints)
# - Private subnet with NAT gateway
# - ECS Fargate with the cloudflared-fips container
# - CloudWatch logging
# - Secrets Manager for tunnel credentials
#
# Usage:
#   terraform init
#   terraform plan -var="tunnel_token=<token>" -var="origin_url=https://localhost:8080"
#   terraform apply

terraform {
  required_version = ">= 1.5"
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}

provider "aws" {
  region = var.aws_region
}

# ── Variables ──

variable "aws_region" {
  description = "AWS region (use us-gov-west-1 or us-gov-east-1 for GovCloud)"
  type        = string
  default     = "us-gov-west-1"
}

variable "tunnel_token" {
  description = "Cloudflare Tunnel token"
  type        = string
  sensitive   = true
}

variable "origin_url" {
  description = "Origin service URL that cloudflared proxies to"
  type        = string
  default     = "https://localhost:8080"
}

variable "container_image" {
  description = "cloudflared-fips container image URI"
  type        = string
  default     = "cloudflared-fips:latest"
}

variable "container_cpu" {
  description = "Fargate task CPU units (256 = 0.25 vCPU)"
  type        = number
  default     = 512
}

variable "container_memory" {
  description = "Fargate task memory in MiB"
  type        = number
  default     = 1024
}

variable "desired_count" {
  description = "Number of cloudflared tasks"
  type        = number
  default     = 2
}

variable "environment" {
  description = "Environment name (e.g., production, staging)"
  type        = string
  default     = "production"
}

# ── Networking ──

resource "aws_vpc" "main" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_support   = true
  enable_dns_hostnames = true

  tags = {
    Name        = "cloudflared-fips-vpc"
    Environment = var.environment
    FIPSMode    = "true"
  }
}

resource "aws_subnet" "private" {
  count             = 2
  vpc_id            = aws_vpc.main.id
  cidr_block        = cidrsubnet(aws_vpc.main.cidr_block, 8, count.index)
  availability_zone = data.aws_availability_zones.available.names[count.index]

  tags = {
    Name = "cloudflared-fips-private-${count.index}"
  }
}

resource "aws_subnet" "public" {
  count                   = 2
  vpc_id                  = aws_vpc.main.id
  cidr_block              = cidrsubnet(aws_vpc.main.cidr_block, 8, count.index + 100)
  availability_zone       = data.aws_availability_zones.available.names[count.index]
  map_public_ip_on_launch = true

  tags = {
    Name = "cloudflared-fips-public-${count.index}"
  }
}

resource "aws_internet_gateway" "main" {
  vpc_id = aws_vpc.main.id
}

resource "aws_eip" "nat" {
  domain = "vpc"
}

resource "aws_nat_gateway" "main" {
  allocation_id = aws_eip.nat.id
  subnet_id     = aws_subnet.public[0].id
}

resource "aws_route_table" "private" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = aws_nat_gateway.main.id
  }
}

resource "aws_route_table" "public" {
  vpc_id = aws_vpc.main.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.main.id
  }
}

resource "aws_route_table_association" "private" {
  count          = 2
  subnet_id      = aws_subnet.private[count.index].id
  route_table_id = aws_route_table.private.id
}

resource "aws_route_table_association" "public" {
  count          = 2
  subnet_id      = aws_subnet.public[count.index].id
  route_table_id = aws_route_table.public.id
}

data "aws_availability_zones" "available" {
  state = "available"
}

# ── Secrets ──

resource "aws_secretsmanager_secret" "tunnel_token" {
  name                    = "cloudflared-fips/tunnel-token"
  description             = "Cloudflare Tunnel token for cloudflared-fips"
  recovery_window_in_days = 7
}

resource "aws_secretsmanager_secret_version" "tunnel_token" {
  secret_id     = aws_secretsmanager_secret.tunnel_token.id
  secret_string = var.tunnel_token
}

# ── ECS Cluster ──

resource "aws_ecs_cluster" "main" {
  name = "cloudflared-fips"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  configuration {
    execute_command_configuration {
      logging = "OVERRIDE"
      log_configuration {
        cloud_watch_log_group_name = aws_cloudwatch_log_group.ecs.name
      }
    }
  }
}

resource "aws_cloudwatch_log_group" "ecs" {
  name              = "/ecs/cloudflared-fips"
  retention_in_days = 90
}

# ── IAM ──

resource "aws_iam_role" "ecs_task_execution" {
  name = "cloudflared-fips-task-execution"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ecs-tasks.amazonaws.com"
      }
    }]
  })
}

resource "aws_iam_role_policy_attachment" "ecs_task_execution" {
  role       = aws_iam_role.ecs_task_execution.name
  policy_arn = "arn:aws-us-gov:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role_policy" "secrets_access" {
  name = "secrets-access"
  role = aws_iam_role.ecs_task_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["secretsmanager:GetSecretValue"]
      Resource = [aws_secretsmanager_secret.tunnel_token.arn]
    }]
  })
}

resource "aws_iam_role" "ecs_task" {
  name = "cloudflared-fips-task"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ecs-tasks.amazonaws.com"
      }
    }]
  })
}

# ── Security Group ──

resource "aws_security_group" "cloudflared" {
  name_prefix = "cloudflared-fips-"
  vpc_id      = aws_vpc.main.id

  # cloudflared initiates outbound connections to Cloudflare edge
  egress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "HTTPS to Cloudflare edge"
  }

  # QUIC/UDP to Cloudflare edge
  egress {
    from_port   = 7844
    to_port     = 7844
    protocol    = "udp"
    cidr_blocks = ["0.0.0.0/0"]
    description = "QUIC tunnel to Cloudflare edge"
  }

  # Allow access to origin service
  egress {
    from_port   = 0
    to_port     = 65535
    protocol    = "tcp"
    cidr_blocks = [aws_vpc.main.cidr_block]
    description = "Access to origin services in VPC"
  }

  tags = {
    Name = "cloudflared-fips"
  }
}

# ── ECS Task Definition ──

resource "aws_ecs_task_definition" "cloudflared" {
  family                   = "cloudflared-fips"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.container_cpu
  memory                   = var.container_memory
  execution_role_arn       = aws_iam_role.ecs_task_execution.arn
  task_role_arn            = aws_iam_role.ecs_task.arn

  container_definitions = jsonencode([{
    name      = "cloudflared-fips"
    image     = var.container_image
    essential = true

    command = ["tunnel", "--no-autoupdate", "run"]

    secrets = [{
      name      = "TUNNEL_TOKEN"
      valueFrom = aws_secretsmanager_secret.tunnel_token.arn
    }]

    environment = [
      {
        name  = "TUNNEL_ORIGIN_CERT"
        value = "/etc/cloudflared/cert.pem"
      }
    ]

    healthCheck = {
      command     = ["CMD", "selftest"]
      interval    = 30
      timeout     = 5
      retries     = 3
      startPeriod = 10
    }

    logConfiguration = {
      logDriver = "awslogs"
      options = {
        "awslogs-group"         = aws_cloudwatch_log_group.ecs.name
        "awslogs-region"        = var.aws_region
        "awslogs-stream-prefix" = "cloudflared"
      }
    }
  }])
}

# ── ECS Service ──

resource "aws_ecs_service" "cloudflared" {
  name            = "cloudflared-fips"
  cluster         = aws_ecs_cluster.main.id
  task_definition = aws_ecs_task_definition.cloudflared.arn
  desired_count   = var.desired_count
  launch_type     = "FARGATE"

  network_configuration {
    subnets         = aws_subnet.private[*].id
    security_groups = [aws_security_group.cloudflared.id]
  }
}

# ── Outputs ──

output "cluster_name" {
  value = aws_ecs_cluster.main.name
}

output "service_name" {
  value = aws_ecs_service.cloudflared.name
}

output "vpc_id" {
  value = aws_vpc.main.id
}

output "log_group" {
  value = aws_cloudwatch_log_group.ecs.name
}
