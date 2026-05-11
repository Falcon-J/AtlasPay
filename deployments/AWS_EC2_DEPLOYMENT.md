# AWS EC2 Free Tier Deployment Guide for AtlasPay

## 📋 Prerequisites

### AWS Account Setup
1. **Create AWS Account** (if not already done)
   - Sign up at https://aws.amazon.com/free
   - Verify email & phone
   - Add payment method (won't be charged for free tier)

2. **Install AWS CLI**
   ```powershell
   # Windows - Download from https://awscli.amazonaws.com/AWSCLIV2.msi
   # Or via PowerShell
   msiexec.exe /i https://awscli.amazonaws.com/AWSCLIV2.msi
   ```

3. **Configure AWS Credentials**
   ```powershell
   aws configure
   # Enter: Access Key ID, Secret Access Key, default region (us-east-1), output format (json)
   ```

4. **Install Terraform**
   ```powershell
   # Download from https://www.terraform.io/downloads
   # Or via Chocolatey: choco install terraform
   ```

---

## 🚀 Deployment Steps

### Step 1: Initialize Terraform

```powershell
cd deployments
terraform init
```

This downloads the AWS provider plugin and initializes the Terraform working directory.

### Step 2: Review Infrastructure Plan

```powershell
terraform plan
```

**Expected output**: Shows all resources to be created:
- VPC with public/private subnets
- Security groups (EC2 + RDS)
- RDS PostgreSQL instance (db.t3.micro, 20GB)
- EC2 instance (t2.micro, Amazon Linux 2)
- Elastic IP for static access

### Step 3: Deploy Infrastructure

```powershell
terraform apply
```

When prompted, type `yes` to confirm.

**Expected timeline**: 5-10 minutes
- VPC & subnets: ~30s
- RDS provisioning: 5-8 min (longest step)
- EC2 instance: ~1 min
- Elastic IP: ~10s

### Step 4: Get Outputs

After successful deployment, Terraform prints:

```
Outputs:

ec2_public_ip = "54.123.45.67"
ec2_public_dns = "ec2-54-123-45-67.compute-1.amazonaws.com"
rds_endpoint = "atlaspay-db.c12345abcd.us-east-1.rds.amazonaws.com:5432"
rds_host = "atlaspay-db.c12345abcd.us-east-1.rds.amazonaws.com"
db_password = "generated-random-password-here"
```

**Save these values!** You'll need them for accessing the application.

### Step 5: Wait for EC2 Setup

The EC2 instance runs a user-data script that:
- Installs Docker & Docker Compose
- Clones AtlasPay repo from GitHub
- Starts services (API Gateway, Redis, Kafka)
- Waits for RDS database to be ready

**Timeline**: 3-5 minutes after EC2 starts

### Step 6: Verify Deployment

```powershell
# Test health endpoint
curl http://54.123.45.67:8080/health
# or
Invoke-WebRequest -Uri http://54.123.45.67:8080/health -UseBasicParsing

# Expected response:
# {"status":"healthy","db":"up","cache":"up"}
```

### Step 7: SSH into EC2 (optional)

```powershell
# Create key pair in AWS Console, then:
ssh -i "your-key.pem" ec2-user@54.123.45.67

# Inside EC2:
cd /opt/atlaspay
docker-compose -f docker-compose.ec2.yml logs -f api-gateway
```

---

## 📊 Architecture Overview

```
┌─────────────────────────────────────────┐
│         AWS EC2 (t2.micro)              │
│  ┌─────────────────────────────────────┐│
│  │ AtlasPay API Gateway (Docker)       ││
│  │  - Port 8080 (HTTP)                 ││
│  │  - Health check: /health            ││
│  └─────────────────────────────────────┘│
│  ┌─────────────────────────────────────┐│
│  │ Redis 7 (Docker)                    ││
│  │  - Cache for orders/inventory       ││
│  └─────────────────────────────────────┘│
│  ┌─────────────────────────────────────┐│
│  │ Kafka 7.5 + Zookeeper (Docker)      ││
│  │  - Event streaming for sagas        ││
│  └─────────────────────────────────────┘│
└─────────────────────────────────────────┘
         ↓ (Private network)
┌─────────────────────────────────────────┐
│    AWS RDS PostgreSQL (db.t3.micro)    │
│    - Multi-AZ: Off (free tier)          │
│    - Backup retention: 7 days           │
│    - 20GB storage, auto-scaling to 100GB│
└─────────────────────────────────────────┘
```

---

## 💰 Cost Estimation (12-Month Free Tier)

| Service | Free Tier | Cost/Month |
|---------|-----------|-----------|
| EC2 t2.micro | 750 hours | $0 |
| RDS db.t3.micro | Single-AZ, 20GB | $0 |
| EBS storage | 30GB | $0 |
| Data transfer | 100GB/month | $0* |
| Elastic IP | 1 per running instance | $0 |
| **TOTAL** | | **$0** |

*After 12 months or if limits exceeded, charges apply (~$0.09/hour for t2.micro EC2, ~$0.017/GB for data transfer)

---

## 🔒 Security Considerations

### Current Configuration (Development)
```hcl
ingress {
  from_port   = 22
  to_port     = 22
  protocol    = "tcp"
  cidr_blocks = ["0.0.0.0/0"]  # ⚠️ OPEN TO WORLD
}
```

### Production Recommendations

1. **Restrict SSH Access**
   ```hcl
   cidr_blocks = ["YOUR_IP/32"]  # Replace with your IP
   ```

2. **Add ALB (Application Load Balancer)**
   ```hcl
   # For HTTPS, SSL termination
   ```

3. **Use AWS Secrets Manager** for database password
   ```hcl
   # Instead of terraform.tfstate
   ```

4. **Enable VPC Flow Logs**
   ```hcl
   # Monitor network traffic
   ```

5. **Setup CloudWatch Alarms**
   ```hcl
   # Alert on CPU > 80%, disk space low, etc.
   ```

---

## 🐛 Troubleshooting

### Issue: "No space left on device" after deployment

**Cause**: t2.micro with 30GB root volume fills up quickly if Docker images accumulate

**Fix**:
```bash
# SSH into EC2
docker system prune -a  # Remove unused images
df -h  # Check disk space
```

### Issue: RDS database not connecting

**Cause**: RDS endpoint not ready, network timeouts

**Status**: Wait 5-8 minutes for RDS to fully provision
```powershell
# Check RDS status in AWS Console
# Or via CLI:
aws rds describe-db-instances --db-instance-identifier atlaspay-db
```

### Issue: Health check returns 503 (Service Unavailable)

**Cause**: Redis or Kafka not healthy yet

**Status**: Expected during first startup (30-60s), app has graceful degradation
```bash
docker-compose -f docker-compose.ec2.yml logs redis
docker-compose -f docker-compose.ec2.yml logs kafka
```

### Issue: Terraform plan fails with "Access Denied"

**Cause**: AWS credentials not configured or lacking permissions

**Fix**:
```powershell
aws configure  # Re-check credentials
aws sts get-caller-identity  # Verify access
```

---

## 🧹 Cleanup (Destroy Infrastructure)

### ⚠️ WARNING: This will delete ALL resources

```powershell
cd deployments
terraform destroy
```

When prompted, type `yes` to confirm.

**Saves**: RDS creates a final snapshot before deletion (can be recovered)

---

## 📈 Monitoring & Logs

### CloudWatch Logs
```powershell
# View EC2 startup logs
aws logs tail /aws/ec2/atlaspay-api-gateway --follow

# View RDS logs
aws logs tail /aws/rds/database/atlaspay-db --follow
```

### SSH into EC2 & View Docker Logs
```bash
# Connect
ssh -i "your-key.pem" ec2-user@54.123.45.67

# View app logs
cd /opt/atlaspay
docker-compose -f docker-compose.ec2.yml logs -f api-gateway

# View all services
docker-compose -f docker-compose.ec2.yml logs -f
```

### AWS Console
1. Open https://console.aws.amazon.com
2. Navigate to:
   - **EC2 Dashboard** → Instances → Find `atlaspay-ec2`
   - **RDS Dashboard** → Databases → Find `atlaspay-db`
   - **CloudWatch** → Logs → View container logs

---

## 🔄 CI/CD Integration (Optional)

To auto-deploy from GitHub on push:

### Option 1: GitHub Actions + CodeDeploy
```yaml
# .github/workflows/deploy-aws.yml
name: Deploy to AWS EC2

on:
  push:
    branches: [main]

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - name: Deploy to EC2
        run: |
          ssh -i ${{ secrets.AWS_KEY }} ec2-user@${{ secrets.EC2_IP }} \
            'cd /opt/atlaspay && git pull && docker-compose -f docker-compose.ec2.yml up -d'
```

### Option 2: AWS CodePipeline (Full CI/CD)
See [AWS_CICD_SETUP.md] (create if needed)

---

## 📱 Accessing the Application

### Public URL
```
http://54.123.45.67:8080
http://ec2-54-123-45-67.compute-1.amazonaws.com:8080
```

### API Endpoints
- **Health Check**: `GET /health`
- **Order API**: `GET/POST /orders`
- **Payment API**: `GET/POST /payments`
- **Inventory API**: `GET/POST /inventory`

### With Domain Name (Optional)
1. Register domain (Route 53, Namecheap, etc.)
2. Create Route 53 A record pointing to Elastic IP
3. Use ACM certificate for HTTPS

---

## ✅ Deployment Checklist

- [ ] AWS Account created & verified
- [ ] AWS CLI installed & configured
- [ ] Terraform installed
- [ ] Ran `terraform init`
- [ ] Reviewed `terraform plan` output
- [ ] Ran `terraform apply` and saved outputs
- [ ] Waited 10 minutes for full provisioning
- [ ] Health check returned 200 OK
- [ ] SSH access verified (if needed)
- [ ] Saved Elastic IP and RDS endpoint
- [ ] Noted DB password location (terraform.tfstate)

---

## 📞 Support

For issues:
1. Check AWS Console for resource status
2. View EC2 system logs: Instances → Select instance → Monitor tab
3. SSH in and check Docker logs
4. Review Terraform state: `terraform show`
5. Verify security groups allow required ports

---

## 🎯 Next Steps

1. **Setup HTTPS**: Use AWS Certificate Manager + Application Load Balancer
2. **Add Monitoring**: CloudWatch alarms, X-Ray tracing
3. **Setup Backups**: RDS automated backups already enabled (7 days)
4. **Domain & DNS**: Route 53 + custom domain name
5. **Auto-scaling**: Monitor CPU usage, scale if needed after free tier ends
