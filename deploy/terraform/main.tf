# Configure the AWS provider
provider "aws" {
  region = var.aws_region
}

# Create a VPC for the project
resource "aws_vpc" "devlm_vpc" {
  cidr_block           = var.vpc_cidr
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-vpc"
      Environment = var.environment
    }
  )
}

# Create public subnets
resource "aws_subnet" "public_subnets" {
  count                   = length(var.public_subnet_cidrs)
  vpc_id                  = aws_vpc.devlm_vpc.id
  cidr_block              = var.public_subnet_cidrs[count.index]
  availability_zone       = element(var.availability_zones, count.index)
  map_public_ip_on_launch = true

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-public-subnet-${count.index + 1}"
      Type = "Public"
    }
  )
}

# Create private subnets
resource "aws_subnet" "private_subnets" {
  count             = length(var.private_subnet_cidrs)
  vpc_id            = aws_vpc.devlm_vpc.id
  cidr_block        = var.private_subnet_cidrs[count.index]
  availability_zone = element(var.availability_zones, count.index)

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-private-subnet-${count.index + 1}"
      Type = "Private"
    }
  )
}

# Create an Internet Gateway
resource "aws_internet_gateway" "devlm_igw" {
  vpc_id = aws_vpc.devlm_vpc.id

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-igw"
    }
  )
}

# Create a route table for public subnets
resource "aws_route_table" "public_rt" {
  vpc_id = aws_vpc.devlm_vpc.id

  route {
    cidr_block = "0.0.0.0/0"
    gateway_id = aws_internet_gateway.devlm_igw.id
  }

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-public-rt"
    }
  )
}

# Associate public subnets with the public route table
resource "aws_route_table_association" "public_subnet_routes" {
  count          = length(aws_subnet.public_subnets)
  subnet_id      = aws_subnet.public_subnets[count.index].id
  route_table_id = aws_route_table.public_rt.id
}

# Create NAT Gateways for private subnets
resource "aws_nat_gateway" "nat_gw" {
  count         = var.environment == "prod" ? length(var.public_subnet_cidrs) : 1
  allocation_id = aws_eip.nat_eip[count.index].id
  subnet_id     = aws_subnet.public_subnets[count.index].id

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-nat-gw-${count.index + 1}"
    }
  )

  depends_on = [aws_internet_gateway.devlm_igw]
}

# Create Elastic IPs for NAT Gateways
resource "aws_eip" "nat_eip" {
  count = var.environment == "prod" ? length(var.public_subnet_cidrs) : 1
  vpc   = true

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-nat-eip-${count.index + 1}"
    }
  )
}

# Create route tables for private subnets
resource "aws_route_table" "private_rt" {
  count  = length(var.private_subnet_cidrs)
  vpc_id = aws_vpc.devlm_vpc.id

  route {
    cidr_block     = "0.0.0.0/0"
    nat_gateway_id = var.environment == "prod" ? aws_nat_gateway.nat_gw[count.index].id : aws_nat_gateway.nat_gw[0].id
  }

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-private-rt-${count.index + 1}"
    }
  )
}

# Associate private subnets with the private route tables
resource "aws_route_table_association" "private_subnet_routes" {
  count          = length(aws_subnet.private_subnets)
  subnet_id      = aws_subnet.private_subnets[count.index].id
  route_table_id = aws_route_table.private_rt[count.index].id
}

# Security group for EC2 instances
resource "aws_security_group" "ec2_sg" {
  name        = "${var.project_name}-ec2-sg"
  description = "Security group for ${var.project_name} EC2 instances"
  vpc_id      = aws_vpc.devlm_vpc.id

  dynamic "ingress" {
    for_each = var.ec2_ingress_rules
    content {
      from_port   = ingress.value.from_port
      to_port     = ingress.value.to_port
      protocol    = ingress.value.protocol
      cidr_blocks = ingress.value.cidr_blocks
    }
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-ec2-sg"
    }
  )
}

# EC2 instance for Golang backend service
resource "aws_instance" "golang_backend" {
  count                  = var.ec2_instance_count
  ami                    = var.ec2_ami
  instance_type          = var.ec2_instance_type
  key_name               = var.ec2_key_name
  vpc_security_group_ids = [aws_security_group.ec2_sg.id]
  subnet_id              = element(aws_subnet.private_subnets[*].id, count.index)
  iam_instance_profile   = aws_iam_instance_profile.ec2_profile.name

  user_data = templatefile("${path.module}/user_data.tpl", {
    db_host    = aws_db_instance.postgres.endpoint
    redis_host = aws_elasticache_cluster.redis.cache_nodes[0].address
  })

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-golang-backend-${count.index + 1}"
    }
  )
}

# RDS instance for PostgreSQL database
resource "aws_db_instance" "postgres" {
  identifier        = "${var.project_name}-postgres"
  engine            = "postgres"
  engine_version    = var.rds_engine_version
  instance_class    = var.rds_instance_class
  allocated_storage = var.rds_allocated_storage
  storage_type      = "gp2"

  db_name  = var.db_name
  username = var.db_username
  password = var.db_password

  vpc_security_group_ids = [aws_security_group.rds_sg.id]
  db_subnet_group_name   = aws_db_subnet_group.postgres.name

  backup_retention_period = 7
  backup_window           = "03:00-04:00"
  maintenance_window      = "Mon:04:00-Mon:05:00"

  skip_final_snapshot = true
  multi_az            = var.environment == "prod" ? true : false

  storage_encrypted = true
  kms_key_id        = aws_kms_key.rds.arn

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-postgres"
    }
  )
}

# DB subnet group for RDS
resource "aws_db_subnet_group" "postgres" {
  name       = "${var.project_name}-postgres-subnet-group"
  subnet_ids = aws_subnet.private_subnets[*].id

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name} PostgreSQL DB subnet group"
    }
  )
}

# Security group for RDS
resource "aws_security_group" "rds_sg" {
  name        = "${var.project_name}-rds-sg"
  description = "Security group for ${var.project_name} RDS instance"
  vpc_id      = aws_vpc.devlm_vpc.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.ec2_sg.id, aws_security_group.ecs_sg.id]
  }

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-rds-sg"
    }
  )
}

# ElastiCache for Redis
resource "aws_elasticache_cluster" "redis" {
  cluster_id           = "${var.project_name}-redis"
  engine               = "redis"
  node_type            = var.elasticache_node_type
  num_cache_nodes      = var.elasticache_num_cache_nodes
  parameter_group_name = "default.redis6.x"
  port                 = 6379

  subnet_group_name  = aws_elasticache_subnet_group.redis.name
  security_group_ids = [aws_security_group.redis_sg.id]

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-redis"
    }
  )
}

# ElastiCache subnet group
resource "aws_elasticache_subnet_group" "redis" {
  name       = "${var.project_name}-redis-subnet-group"
  subnet_ids = aws_subnet.private_subnets[*].id
}

# Security group for ElastiCache
resource "aws_security_group" "redis_sg" {
  name        = "${var.project_name}-redis-sg"
  description = "Security group for ${var.project_name} ElastiCache Redis"
  vpc_id      = aws_vpc.devlm_vpc.id

  ingress {
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    security_groups = [aws_security_group.ec2_sg.id, aws_security_group.ecs_sg.id]
  }

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-redis-sg"
    }
  )
}

# ECS cluster
resource "aws_ecs_cluster" "devlm_cluster" {
  name = "${var.project_name}-cluster"

  setting {
    name  = "containerInsights"
    value = "enabled"
  }

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-ecs-cluster"
    }
  )
}

# ECS task definition for Python LLM service
resource "aws_ecs_task_definition" "python_llm" {
  family                   = "${var.project_name}-python-llm"
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = var.ecs_task_cpu
  memory                   = var.ecs_task_memory
  execution_role_arn       = aws_iam_role.ecs_execution_role.arn
  task_role_arn            = aws_iam_role.ecs_task_role.arn

  container_definitions = jsonencode([
    {
      name  = "${var.project_name}-python-llm"
      image = var.python_llm_image
      portMappings = [
        {
          containerPort = 8000
          hostPort      = 8000
        }
      ]
      environment = [
        {
          name  = "DB_HOST"
          value = aws_db_instance.postgres.endpoint
        },
        {
          name  = "REDIS_HOST"
          value = aws_elasticache_cluster.redis.cache_nodes[0].address
        }
      ]
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = "/ecs/${var.project_name}-python-llm"
          awslogs-region        = var.aws_region
          awslogs-stream-prefix = "ecs"
        }
      }
    }
  ])

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-python-llm-task"
    }
  )
}

# ECS service for Python LLM
resource "aws_ecs_service" "python_llm" {
  name            = "${var.project_name}-python-llm-service"
  cluster         = aws_ecs_cluster.devlm_cluster.id
  task_definition = aws_ecs_task_definition.python_llm.arn
  launch_type     = "FARGATE"
  desired_count   = var.ecs_service_desired_count

  network_configuration {
    subnets         = aws_subnet.private_subnets[*].id
    security_groups = [aws_security_group.ecs_sg.id]
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.python_llm.arn
    container_name   = "${var.project_name}-python-llm"
    container_port   = 8000
  }

  depends_on = [aws_lb_listener.front_end]

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-python-llm-service"
    }
  )
}

# Security group for ECS tasks
resource "aws_security_group" "ecs_sg" {
  name        = "${var.project_name}-ecs-sg"
  description = "Security group for ${var.project_name} ECS tasks"
  vpc_id      = aws_vpc.devlm_vpc.id

  ingress {
    from_port       = 8000
    to_port         = 8000
    protocol        = "tcp"
    security_groups = [aws_security_group.alb_sg.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = merge(
    var.tags,
    {
      Name = "${var.project_name}-ecs-sg"
    }
  )
}

# ALB for load balancing
resource "aws_lb" "devlm_alb" {
  name               = var.alb_name
  internal           = false
  load_balancer_type = "application"
  security_groups    = [aws_security_group.alb_sg.id]
  subnets            = aws_subnet.public_subnets[*].id

  enable_deletion_protection = var.environment == "prod" ? true : false

  access_logs {