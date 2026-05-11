# AWS EC2 Free Tier Deployment for AtlasPay
# Requires: AWS CLI configured, Terraform installed

terraform {
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

# Variables
variable "aws_region" {
  default = "us-east-1"
  description = "AWS region (us-east-1 has most free tier offerings)"
}

variable "instance_type" {
  default = "t2.micro"
  description = "EC2 instance type (t2.micro is free tier eligible)"
}

variable "environment" {
  default = "production"
}

# VPC
resource "aws_vpc" "atlaspay" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name = "atlaspay-vpc"
  }
}

# Public Subnet
resource "aws_subnet" "public" {
  vpc_id                  = aws_vpc.atlaspay.id
  cidr_block              = "10.0.1.0/24"
  availability_zone       = data.aws_availability_zones.available.names[0]
  map_public_ip_on_launch = true

  tags = {
    Name = "atlaspay-public-subnet"
  }
}

# Private Subnet (for RDS)
resource "aws_subnet" "private" {
  vpc_id            = aws_vpc.atlaspay.id
  cidr_block        = "10.0.2.0/24"
  availability_zone = data.aws_availability_zones.available.names[1]

  tags = {
    Name = "atlaspay-private-subnet"
  }
}

# Internet Gateway
resource "aws_internet_gateway" "atlaspay" {
  vpc_id = aws_vpc.atlaspay.id

  tags = {
    Name = "atlaspay-igw"
  }
}

# Route Table for Public Subnet
resource "aws_route_table" "public" {
  vpc_id = aws_vpc.atlaspay.id

  route {
    cidr_block      = "0.0.0.0/0"
    gateway_id      = aws_internet_gateway.atlaspay.id
  }

  tags = {
    Name = "atlaspay-public-rt"
  }
}

resource "aws_route_table_association" "public" {
  subnet_id      = aws_subnet.public.id
  route_table_id = aws_route_table.public.id
}

# Security Group for EC2
resource "aws_security_group" "ec2" {
  name        = "atlaspay-ec2-sg"
  description = "Security group for AtlasPay EC2 instance"
  vpc_id      = aws_vpc.atlaspay.id

  ingress {
    from_port   = 80
    to_port     = 80
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 8080
    to_port     = 8080
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 22
    to_port     = 22
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]  # CHANGE: Restrict to your IP
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "atlaspay-ec2-sg"
  }
}

# Security Group for RDS
resource "aws_security_group" "rds" {
  name        = "atlaspay-rds-sg"
  description = "Security group for AtlasPay RDS"
  vpc_id      = aws_vpc.atlaspay.id

  ingress {
    from_port       = 5432
    to_port         = 5432
    protocol        = "tcp"
    security_groups = [aws_security_group.ec2.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags = {
    Name = "atlaspay-rds-sg"
  }
}

# RDS Subnet Group
resource "aws_db_subnet_group" "atlaspay" {
  name       = "atlaspay-db-subnet-group"
  subnet_ids = [aws_subnet.public.id, aws_subnet.private.id]

  tags = {
    Name = "atlaspay-db-subnet-group"
  }
}

# RDS PostgreSQL (Free Tier: db.t3.micro, 20GB storage)
resource "aws_db_instance" "atlaspay" {
  identifier     = "atlaspay-db"
  engine         = "postgres"
  engine_version = "15.3"
  instance_class = "db.t3.micro"
  
  allocated_storage = 20
  max_allocated_storage = 100  # Auto-scaling enabled

  db_name  = "atlaspay"
  username = "atlaspay"
  password = random_password.db_password.result

  db_subnet_group_name   = aws_db_subnet_group.atlaspay.name
  vpc_security_group_ids = [aws_security_group.rds.id]

  publicly_accessible    = false
  skip_final_snapshot    = false
  final_snapshot_identifier = "atlaspay-db-final-snapshot-${formatdate("YYYY-MM-DD-hhmm", timestamp())}"

  multi_az               = false  # Free tier: single AZ
  backup_retention_period = 7
  storage_encrypted      = true

  tags = {
    Name = "atlaspay-db"
  }
}

# Random password for RDS
resource "random_password" "db_password" {
  length  = 16
  special = true
}

# IAM Role for EC2 (to access CloudWatch, SSM)
resource "aws_iam_role" "ec2_role" {
  name = "atlaspay-ec2-role"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ec2.amazonaws.com"
      }
    }]
  })
}

# IAM Policy for CloudWatch logs
resource "aws_iam_role_policy" "cloudwatch_policy" {
  name = "atlaspay-cloudwatch-policy"
  role = aws_iam_role.ec2_role.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect = "Allow"
      Action = [
        "logs:CreateLogGroup",
        "logs:CreateLogStream",
        "logs:PutLogEvents"
      ]
      Resource = "arn:aws:logs:*:*:*"
    }]
  })
}

# IAM Instance Profile
resource "aws_iam_instance_profile" "ec2_profile" {
  name = "atlaspay-ec2-profile"
  role = aws_iam_role.ec2_role.name
}

# Data source for latest Amazon Linux 2 AMI
data "aws_ami" "amazon_linux_2" {
  most_recent = true
  owners      = ["amazon"]

  filter {
    name   = "name"
    values = ["amzn2-ami-hvm-*-x86_64-gp2"]
  }

  filter {
    name   = "virtualization-type"
    values = ["hvm"]
  }
}

# Data source for availability zones
data "aws_availability_zones" "available" {
  state = "available"
}

# User data script to configure EC2
locals {
  user_data = templatefile("${path.module}/user-data.sh", {
    db_host     = aws_db_instance.atlaspay.endpoint
    db_user     = aws_db_instance.atlaspay.username
    db_password = random_password.db_password.result
    db_name     = aws_db_instance.atlaspay.db_name
  })
}

# EC2 Instance
resource "aws_instance" "atlaspay" {
  ami           = data.aws_ami.amazon_linux_2.id
  instance_type = var.instance_type

  subnet_id                   = aws_subnet.public.id
  vpc_security_group_ids      = [aws_security_group.ec2.id]
  iam_instance_profile        = aws_iam_instance_profile.ec2_profile.name
  associate_public_ip_address = true

  user_data = base64encode(local.user_data)

  root_block_device {
    volume_type           = "gp3"
    volume_size           = 30
    delete_on_termination = true
    encrypted             = true
  }

  tags = {
    Name = "atlaspay-ec2"
  }

  depends_on = [aws_db_instance.atlaspay]
}

# Elastic IP
resource "aws_eip" "atlaspay" {
  instance = aws_instance.atlaspay.id
  domain   = "vpc"

  tags = {
    Name = "atlaspay-eip"
  }

  depends_on = [aws_internet_gateway.atlaspay]
}

# Outputs
output "ec2_public_ip" {
  value       = aws_eip.atlaspay.public_ip
  description = "Public IP of AtlasPay EC2 instance"
}

output "ec2_public_dns" {
  value       = aws_instance.atlaspay.public_dns
  description = "Public DNS of AtlasPay EC2 instance"
}

output "rds_endpoint" {
  value       = aws_db_instance.atlaspay.endpoint
  description = "RDS endpoint"
}

output "rds_host" {
  value       = split(":", aws_db_instance.atlaspay.endpoint)[0]
  description = "RDS hostname"
}

output "db_password" {
  value       = random_password.db_password.result
  sensitive   = true
  description = "RDS password (stored in terraform.tfstate, keep secure!)"
}
