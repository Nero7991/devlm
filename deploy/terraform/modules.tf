# modules.tf

# VPC Module
module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "~> 3.0"

  name = "${var.project_name}-vpc"
  cidr = var.vpc_cidr

  azs             = var.availability_zones
  private_subnets = var.private_subnet_cidrs
  public_subnets  = var.public_subnet_cidrs

  enable_nat_gateway     = var.vpc_enable_nat_gateway
  single_nat_gateway     = true
  enable_vpn_gateway     = false
  enable_dns_hostnames   = var.vpc_enable_dns_hostnames
  enable_dns_support     = true

  enable_flow_log                      = true
  create_flow_log_cloudwatch_log_group = true
  create_flow_log_cloudwatch_iam_role  = true
  flow_log_max_aggregation_interval    = 60
  flow_log_retention_in_days           = var.vpc_flow_logs_retention

  enable_s3_endpoint       = true
  enable_dynamodb_endpoint = true

  # Enable IPv6 support
  enable_ipv6 = var.vpc_enable_ipv6

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-vpc"
    }
  )
}

# Security Group Module
module "security_groups" {
  source  = "terraform-aws-modules/security-group/aws"
  version = "~> 4.0"

  name        = "${var.project_name}-sg"
  description = "Security group for ${var.project_name}"
  vpc_id      = module.vpc.vpc_id

  ingress_with_cidr_blocks = [
    {
      from_port   = 80
      to_port     = 80
      protocol    = "tcp"
      description = "HTTP"
      cidr_blocks = "0.0.0.0/0"
    },
    {
      from_port   = 443
      to_port     = 443
      protocol    = "tcp"
      description = "HTTPS"
      cidr_blocks = "0.0.0.0/0"
    },
    {
      from_port   = 22
      to_port     = 22
      protocol    = "tcp"
      description = "SSH"
      cidr_blocks = var.ssh_allowed_cidr
    }
  ]
  egress_rules = ["all-all"]

  tags = var.tags
}

# EC2 Module
module "ec2_instance" {
  source  = "terraform-aws-modules/ec2-instance/aws"
  version = "~> 3.0"

  name = "${var.project_name}-instance"

  ami                    = var.ec2_ami
  instance_type          = var.ec2_instance_type
  key_name               = var.ec2_key_name
  monitoring             = true
  vpc_security_group_ids = [module.security_groups.security_group_id]
  subnet_id              = module.vpc.public_subnets[0]

  user_data = templatefile("${path.module}/user_data.sh", {
    go_version = "1.16"
    repo_url   = "https://github.com/Nero7991/devlm.git"
  })

  tags = var.tags
}

# RDS Module
module "db" {
  source  = "terraform-aws-modules/rds/aws"
  version = "~> 3.0"

  identifier = "${var.project_name}-db"

  engine            = "postgres"
  engine_version    = var.rds_engine_version
  instance_class    = var.rds_instance_class
  allocated_storage = var.rds_allocated_storage

  db_name  = var.rds_db_name
  username = var.rds_username
  port     = var.db_port

  iam_database_authentication_enabled = true

  vpc_security_group_ids = [module.security_groups.security_group_id]

  maintenance_window = "Mon:00:00-Mon:03:00"
  backup_window      = "03:00-06:00"

  backup_retention_period = var.rds_backup_retention_period
  skip_final_snapshot     = true
  deletion_protection     = false

  subnet_ids = module.vpc.private_subnets

  family               = "postgres13"
  major_engine_version = "13"

  performance_insights_enabled          = true
  performance_insights_retention_period = var.rds_performance_insights_retention_period
  monitoring_interval                   = 60

  multi_az = var.rds_multi_az

  manage_master_user_password = true

  parameters = [
    {
      name  = "max_connections"
      value = "500"
    },
    {
      name  = "shared_buffers"
      value = "{DBInstanceClassMemory/4096}"
    }
  ]

  tags = var.tags
}

# ElastiCache Module
module "elasticache" {
  source  = "terraform-aws-modules/elasticache/aws"
  version = "~> 2.0"

  cluster_id           = "${var.project_name}-redis"
  engine               = "redis"
  node_type            = var.elasticache_node_type
  num_cache_nodes      = var.elasticache_num_cache_nodes
  parameter_group_name = var.elasticache_parameter_group_name
  port                 = 6379

  subnet_ids         = module.vpc.private_subnets
  security_group_ids = [module.security_groups.security_group_id]

  multi_az_enabled             = true
  automatic_failover_enabled   = true
  at_rest_encryption_enabled   = true
  transit_encryption_enabled   = true
  auth_token                   = var.redis_auth_token
  apply_immediately            = true
  auto_minor_version_upgrade   = true
  maintenance_window           = "sun:05:00-sun:06:00"
  snapshot_window              = "00:00-01:00"
  snapshot_retention_limit     = 7

  cluster_mode_enabled    = true
  replicas_per_node_group = 1
  num_node_groups         = 2

  tags = var.tags
}

# ECS Cluster Module
module "ecs" {
  source  = "terraform-aws-modules/ecs/aws"
  version = "~> 3.0"

  cluster_name = "${var.project_name}-cluster"

  cluster_configuration = {
    execute_command_configuration = {
      logging = "OVERRIDE"
      log_configuration = {
        cloud_watch_log_group_name = "/aws/ecs/${var.project_name}"
      }
    }
  }

  fargate_capacity_providers = {
    FARGATE = {
      default_capacity_provider_strategy = {
        weight = 50
        base   = 20
      }
    }
    FARGATE_SPOT = {
      default_capacity_provider_strategy = {
        weight = 50
      }
    }
  }

  services = {
    python_llm_service = {
      cpu    = var.ecs_task_cpu
      memory = var.ecs_task_memory

      desired_count                      = 2
      deployment_maximum_percent         = 200
      deployment_minimum_healthy_percent = 100

      container_definitions = [
        {
          name  = "python-llm-service"
          image = "${var.ecr_repository_url}:latest"
          portMappings = [
            {
              containerPort = var.ecs_container_port
              hostPort      = var.ecs_container_port
            }
          ]
          environment = [
            {
              name  = "REDIS_HOST"
              value = module.elasticache.redis_cache_nodes[0].address
            },
            {
              name  = "DB_HOST"
              value = module.db.db_instance_address
            }
          ]
          logConfiguration = {
            logDriver = "awslogs"
            options = {
              awslogs-group         = "/ecs/${var.project_name}"
              awslogs-region        = var.aws_region
              awslogs-stream-prefix = "ecs"
            }
          }
          healthCheck = {
            command     = ["CMD-SHELL", "curl -f http://localhost:${var.ecs_container_port}/health || exit 1"]
            interval    = 30
            timeout     = 5
            retries     = 3
            startPeriod = 60
          }
        }
      ]

      service_connect_configuration = {
        namespace = aws_service_discovery_http_namespace.this.arn
        service = {
          client_alias = {
            port     = 80
            dns_name = "python-llm-service"
          }
          port_name = "http"
        }
      }
    }
  }

  tags = var.tags
}

# ALB Module
module "alb" {
  source  = "terraform-aws-modules/alb/aws"
  version = "~> 6.0"

  name = "${var.project_name}-alb"

  load_balancer_type = "application"

  vpc_id          = module.vpc.vpc_id
  subnets         = module.vpc.public_subnets
  security_groups = [module.security_groups.security_group_id]

  target_groups = [
    {
      name_prefix      = "pref-"
      backend_protocol = "HTTP"
      backend_port     = 80
      target_type      = "ip"
      health_check = {
        enabled             = true
        interval            = 30
        path                = "/health"
        port                = "traffic-port"
        healthy_threshold   = 3
        unhealthy_threshold = 3
        timeout             = 6
        protocol            = "HTTP"
        matcher             = "200-399"
      }
    }
  ]

  http_tcp_listeners = [
    {
      port               = 80
      protocol           = "HTTP"
      target_group_index = 0
    }
  ]

  https_listeners = [
    {
      port               = 443
      protocol           = "HTTPS"
      certificate_arn    = var.alb_certificate_arn
      target_group_index = 0
    }
  ]

  https_listener_rules = [
    {
      https_listener_index = 0
      priority             = 1
      actions = [
        {
          type               = "forward"
          target_group_index = 0
        }
      ]
      conditions = [{
        host_headers = [var.route53_domain_name]
      }]
    }
  ]

  tags = var.tags
}

# S3 Bucket Module
module "s3_bucket" {
  source  = "terraform-aws-modules/s3-bucket/aws"
  version = "~> 2.0"

  bucket = var.s3_bucket_name
  acl    = "private"

  versioning = {
    enabled = var.s3_versioning
  }

  server_side_encryption_configuration = {
    rule = {
      apply_server_side_encryption_by_default = {
        sse_algorithm = "AES256"
      }
    }
  }

  lifecycle_rule = [
    {
      enabled = true
      transition = [
        {
          days          = 30
          storage_class = "INTELLIGENT_TIERING"
        },
        {
          days          = 60
          storage_class = "GLACIER"
        }
      ]
      expiration = {
        days = 90
      }
    }
  ]

  object_lock_configuration = {
    object_lock_enabled = "Enabled"
    rule = {
      default_retention = {
        mode = "GOVERNANCE"
        days = 30
      }
    }
  }

  attach_public_policy    = false
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true

  cors_rule = [
    {
      allowed_headers = ["*"]
      allowed_methods = ["GET", "HEAD"]
      allowed_origins = ["https://${var.route53_domain_name}"]
      expose_headers  = ["ETag"]
      max_age_seconds = 3000
    }
  ]

  tags = var.tags
}

# Route53 Module
module "route53" {
  source  = "terraform-aws-modules/route53/aws//modules/zones"
  version = "~> 2.0"

  zones = {
    "${var.route53_domain_name}" = {
      comment = "Domain for ${var.project_name}"
      tags    = var.tags
    }
  }

  records = [
    {
      name = ""
      type = "A"
      alias = {
        name    = module.alb.lb_dns_name
        zone_id = module.alb.lb_zone_id
      }
    },
    {
      name    = "www"
      type    = "CNAME"
      ttl     = 300
      records = ["${var.route53_domain_name}"]
    }
  ]

  tags = var.tags
}

# IAM Role Module
module "iam_role" {
  source  = "terraform-aws-modules/iam/aws//modules/iam-assumable-role"
  version = "~> 4.0"

  trusted_role_services = [
    "ec2.amazonaws.com",
    "ecs-tasks.amazonaws.com"
  ]

  role_name         = var.iam_role_name
  create_role       = true
  role_requires_mfa = false

  custom_role_policy_arns = [
    aws_iam_policy.s3_access.arn,
    aws_iam_policy.rds_access.arn,
    aws_iam_policy.ec2_access.arn,
    aws_iam_policy.cloudwatch_access.arn,
    aws_iam_policy.ecs_access.arn
  ]

  tags = var.tags
}

resource "aws_iam_policy" "s3_access" {
  name        = "${var.project_name}-s3-access"
  path        = "/"
  description = "IAM policy for S3 access"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "s3:GetObject",
          "s3:PutObject",
          "s3:ListBucket",
          "s3:DeleteObject"
        ]
        Resource = [
          module.s3_bucket.s3_bucket_arn,
          "${module.s3_bucket.s3_bucket_arn}/*"
        ]
      }
    ]
  })
}

resource "aws_iam_policy" "rds_access" {
  name        = "${var.project_name}-rds-access"
  path        = "/"
  description = "IAM policy for RDS access"

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "rds:DescribeDBInstances",
          "rds:DescribeDBClusters",
          "rds:ModifyDBInstance",
          "rds:CreateDBSnapshot"
        ]
        Resource = "*"
      }
    ]
  })
}

resource "aws_iam_policy" "ec2_access