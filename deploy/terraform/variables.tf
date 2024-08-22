# Terraform variables for DevLM project

variable "aws_region" {
  description = "The AWS region to deploy resources"
  type        = string
  default     = "us-west-2"
  validation {
    condition     = can(regex("^[a-z]{2}-[a-z]+-[1-9]$", var.aws_region))
    error_message = "Must be a valid AWS region name."
  }
}

variable "project_name" {
  description = "The name of the project"
  type        = string
  default     = "devlm"
  validation {
    condition     = can(regex("^[a-z0-9-]+$", var.project_name))
    error_message = "Project name must contain only lowercase letters, numbers, and hyphens."
  }
}

variable "environment" {
  description = "The deployment environment (e.g., dev, staging, prod)"
  type        = string
  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "Environment must be one of: dev, staging, prod."
  }
}

variable "vpc_cidr" {
  description = "The CIDR block for the VPC"
  type        = string
  default     = "10.0.0.0/16"
  validation {
    condition     = can(cidrhost(var.vpc_cidr, 0))
    error_message = "Must be a valid IPv4 CIDR block."
  }
}

variable "public_subnet_cidrs" {
  description = "List of CIDR blocks for public subnets"
  type        = list(string)
  default     = ["10.0.1.0/24", "10.0.2.0/24", "10.0.3.0/24"]
  validation {
    condition     = length(var.public_subnet_cidrs) > 0
    error_message = "At least one public subnet CIDR must be provided."
  }
}

variable "private_subnet_cidrs" {
  description = "List of CIDR blocks for private subnets"
  type        = list(string)
  default     = ["10.0.10.0/24", "10.0.11.0/24", "10.0.12.0/24"]
  validation {
    condition     = length(var.private_subnet_cidrs) > 0
    error_message = "At least one private subnet CIDR must be provided."
  }
}

variable "availability_zones" {
  description = "List of availability zones to use for subnets"
  type        = list(string)
  default     = ["us-west-2a", "us-west-2b", "us-west-2c"]
}

variable "tags" {
  description = "A map of tags to add to all resources"
  type        = map(string)
  default = {
    Project     = "DevLM"
    Environment = "dev"
    ManagedBy   = "Terraform"
    CostCenter  = "DevOps"
    Team        = "Infrastructure"
  }
}

variable "ec2_instance_type" {
  description = "EC2 instance type for the Golang backend service"
  type        = string
  default     = "t3.small"
  validation {
    condition     = can(regex("^[tcrmi][3-6][a-z]?\\.(nano|micro|small|medium|large|xlarge|[248]xlarge)$", var.ec2_instance_type))
    error_message = "Must be a valid EC2 instance type."
  }
}

variable "ec2_instance_count" {
  description = "Number of EC2 instances to launch"
  type        = number
  default     = 2
  validation {
    condition     = var.ec2_instance_count > 0
    error_message = "EC2 instance count must be greater than 0."
  }
}

variable "rds_instance_class" {
  description = "RDS instance class for PostgreSQL database"
  type        = string
  default     = "db.t3.small"
}

variable "rds_allocated_storage" {
  description = "Allocated storage for RDS instance (in GB)"
  type        = number
  default     = 20
  validation {
    condition     = var.rds_allocated_storage >= 20 && var.rds_allocated_storage <= 65536
    error_message = "RDS allocated storage must be between 20 GB and 65536 GB."
  }
}

variable "rds_engine_version" {
  description = "PostgreSQL engine version for RDS"
  type        = string
  default     = "14.7"
}

variable "elasticache_node_type" {
  description = "ElastiCache node type for Redis"
  type        = string
  default     = "cache.t3.micro"
}

variable "elasticache_num_cache_nodes" {
  description = "Number of cache nodes in the ElastiCache cluster"
  type        = number
  default     = 2
  validation {
    condition     = var.elasticache_num_cache_nodes > 0
    error_message = "Number of cache nodes must be greater than 0."
  }
}

variable "ecs_task_cpu" {
  description = "CPU units for ECS task definition"
  type        = number
  default     = 512
  validation {
    condition     = var.ecs_task_cpu > 0
    error_message = "ECS task CPU must be greater than 0."
  }
}

variable "ecs_task_memory" {
  description = "Memory for ECS task definition (in MiB)"
  type        = number
  default     = 1024
  validation {
    condition     = var.ecs_task_memory > 0
    error_message = "ECS task memory must be greater than 0."
  }
}

variable "alb_name" {
  description = "Name for the Application Load Balancer"
  type        = string
  default     = "devlm-alb"
}

variable "route53_domain_name" {
  description = "Domain name for Route53 hosted zone"
  type        = string
  default     = "example.com"
  validation {
    condition     = can(regex("^([a-zA-Z0-9][a-zA-Z0-9-]{0,61}[a-zA-Z0-9]\\.)+[a-zA-Z]{2,}$", var.route53_domain_name))
    error_message = "Must be a valid domain name."
  }
}

variable "s3_bucket_name" {
  description = "Name of the S3 bucket for storing project files"
  type        = string
  default     = "devlm-project-files"
  validation {
    condition     = can(regex("^[a-z0-9.-]{3,63}$", var.s3_bucket_name))
    error_message = "S3 bucket name must be between 3 and 63 characters, contain only lowercase letters, numbers, hyphens, and periods, and start and end with a letter or number."
  }
}

variable "s3_versioning" {
  description = "Enable versioning for S3 bucket"
  type        = bool
  default     = true
}

variable "rds_backup_retention_period" {
  description = "Number of days to retain RDS backups"
  type        = number
  default     = 7
  validation {
    condition     = var.rds_backup_retention_period >= 0 && var.rds_backup_retention_period <= 35
    error_message = "RDS backup retention period must be between 0 and 35 days."
  }
}

variable "rds_multi_az" {
  description = "Enable multi-AZ deployment for RDS"
  type        = bool
  default     = true
}

variable "ecs_desired_count" {
  description = "Desired number of ECS tasks to run"
  type        = number
  default     = 2
  validation {
    condition     = var.ecs_desired_count > 0
    error_message = "ECS desired count must be greater than 0."
  }
}

variable "ecs_deployment_min_percent" {
  description = "Minimum healthy percent for ECS deployment"
  type        = number
  default     = 50
  validation {
    condition     = var.ecs_deployment_min_percent >= 0 && var.ecs_deployment_min_percent <= 100
    error_message = "ECS deployment minimum percent must be between 0 and 100."
  }
}

variable "ecs_deployment_max_percent" {
  description = "Maximum percent for ECS deployment"
  type        = number
  default     = 200
  validation {
    condition     = var.ecs_deployment_max_percent >= 100
    error_message = "ECS deployment maximum percent must be greater than or equal to 100."
  }
}

variable "alb_ssl_policy" {
  description = "SSL policy for ALB HTTPS listener"
  type        = string
  default     = "ELBSecurityPolicy-TLS-1-2-2017-01"
}

variable "cloudwatch_log_retention" {
  description = "Number of days to retain CloudWatch logs"
  type        = number
  default     = 30
  validation {
    condition     = can(index([0, 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653], var.cloudwatch_log_retention))
    error_message = "CloudWatch log retention must be one of the allowed values: 0, 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653."
  }
}

variable "iam_role_name" {
  description = "Name of the IAM role for EC2 instances"
  type        = string
  default     = "devlm-ec2-role"
}

variable "vpc_flow_logs_retention" {
  description = "Number of days to retain VPC flow logs"
  type        = number
  default     = 14
  validation {
    condition     = can(index([0, 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653], var.vpc_flow_logs_retention))
    error_message = "VPC flow logs retention must be one of the allowed values: 0, 1, 3, 5, 7, 14, 30, 60, 90, 120, 150, 180, 365, 400, 545, 731, 1827, 3653."
  }
}

variable "waf_web_acl_name" {
  description = "Name of the WAF Web ACL"
  type        = string
  default     = "devlm-web-acl"
}

variable "cloudfront_enabled" {
  description = "Enable CloudFront distribution"
  type        = bool
  default     = false
}

variable "kms_key_alias" {
  description = "Alias for the KMS key used for encryption"
  type        = string
  default     = "alias/devlm-encryption-key"
}

variable "rds_db_name" {
  description = "Name of the PostgreSQL database"
  type        = string
  default     = "devlm_db"
}

variable "rds_username" {
  description = "Username for the RDS PostgreSQL instance"
  type        = string
  sensitive   = true
}

variable "rds_password" {
  description = "Password for the RDS PostgreSQL instance"
  type        = string
  sensitive   = true
}

variable "elasticache_parameter_group_name" {
  description = "Name of the ElastiCache parameter group"
  type        = string
  default     = "default.redis6.x"
}

variable "ecs_container_port" {
  description = "Port exposed by the Docker image for ECS tasks"
  type        = number
  default     = 8080
}

variable "alb_certificate_arn" {
  description = "ARN of the SSL certificate for ALB HTTPS listener"
  type        = string
  default     = "arn:aws:acm:us-west-2:123456789012:certificate/abcdef12-3456-7890-abcd-ef1234567890"
}

variable "cloudfront_price_class" {
  description = "CloudFront distribution price class"
  type        = string
  default     = "PriceClass_100"
  validation {
    condition     = contains(["PriceClass_100", "PriceClass_200", "PriceClass_All"], var.cloudfront_price_class)
    error_message = "CloudFront price class must be one of: PriceClass_100, PriceClass_200, PriceClass_All."
  }
}

variable "waf_rule_group_priority" {
  description = "Priority for WAF rule group"
  type        = number
  default     = 1
}

variable "vpc_enable_dns_hostnames" {
  description = "Enable DNS hostnames in VPC"
  type        = bool
  default     = true
}

variable "vpc_enable_nat_gateway" {
  description = "Enable NAT Gateway in VPC"
  type        = bool
  default     = true
}

variable "ecs_task_execution_role_name" {
  description = "Name of the IAM role for ECS task execution"
  type        = string
  default     = "devlm-ecs-task-execution-role"
}

variable "ecs_task_role_name" {
  description = "Name of the IAM role for ECS tasks"
  type        = string
  default     = "devlm-ecs-task-role"
}

variable "ecs_service_name" {
  description = "Name of the ECS service"
  type        = string
  default     = "devlm-service"
}

variable "ecs_cluster_name" {
  description = "Name of the ECS cluster"
  type        = string
  default     = "devlm-cluster"
}

variable "ecr_repository_name" {
  description = "Name of the ECR repository"
  type        = string
  default     = "devlm-repository"
}

variable "cloudfront_origin_access_identity_comment" {
  description = "Comment for CloudFront origin access identity"
  type        = string
  default     = "DevLM CloudFront OAI"
}

variable "waf_ip_rate_limit" {
  description = "Maximum number of requests allowed from an IP in 5 minutes"
  type        = number
  default     = 100
  validation {
    condition     = var.waf_ip_rate_limit > 0
    error_message = "WAF IP rate limit must be greater than 0."
  }
}

variable "vpc_enable_ipv6" {
  description = "Enable IPv6 in VPC"
  type        = bool
  default     = false
}

variable "rds_performance_insights_enabled" {
  description = "Enable Performance Insights for RDS"
  type        = bool
  default     = true
}

variable "rds_performance_insights_retention_period" {
  description = "Retention period for RDS Performance Insights in days"
  type        = number
  default     = 7
  validation {
    condition     = var.rds_performance_insights_retention_period == 7 || var.rds_performance_insights_retention_period == 731
    error_message = "RDS Performance Insights retention period must be either 7 or