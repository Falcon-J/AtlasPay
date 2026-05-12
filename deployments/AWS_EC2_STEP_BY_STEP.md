# AtlasPay AWS EC2 Setup - Step by Step

## 📋 Prerequisites Checklist

- [ ] AWS Account created (Free Tier eligible)
- [ ] Email verified
- [ ] Payment method added
- [ ] SSH key pair created in AWS
- [ ] SSH client available (Windows: Git Bash or PuTTY, macOS/Linux: built-in)

---

## 🚀 STEP 1: Launch EC2 Instance

### 1.1 Go to AWS Console
```
1. Open https://console.aws.amazon.com
2. Search for "EC2" → Click "EC2" service
3. Click "Instances" in left sidebar
4. Click orange "Launch Instances" button
```

### 1.2 Select AMI (Amazon Machine Image)

**Choose from the free tier eligible options:**

Option A (Recommended - Latest):
- **Ubuntu Server 26.04 LTS (HVM), SSD Volume Type**
- AMI ID: `ami-0911...` (varies by region)
- ✅ Free tier eligible

Option B (Stable):
- **Ubuntu Server 24.04 LTS (HVM), SSD Volume Type**
- ✅ Free tier eligible

Option C (LTS):
- **Ubuntu Server 22.04 LTS (HVM), SSD Volume Type**
- ✅ Free tier eligible

**Select one → Click "Select"**

### 1.3 Choose Instance Type

```
Instance type: t2.micro
(Free tier: 750 hours/month)

vCPU: 1
Memory: 1 GB
EBS Storage: 8 GB (we'll increase to 30GB)

✅ Free tier eligible checkbox will show as checked
```

**Click "Next: Configure Instance Details"**

### 1.4 Configure Instance Details

```
Number of instances: 1
Network: vpc-xxxxx (default)
Subnet: subnet-xxxxx (default)
Auto-assign Public IP: Enable ✅
IAM instance profile: (leave default/none)
Monitoring: Enable detailed CloudWatch monitoring (optional)
```

**Click "Next: Add Storage"**

### 1.5 Add Storage

```
Volume type: Root volume (default)
Size: 30 GB (free tier allows up to 30GB)
Volume type: gp2 or gp3
Delete on termination: Yes ✅
Encrypted: Yes ✅ (free tier)
```

**Click "Next: Add Tags"**

### 1.6 Add Tags

```
Tag 1:
  Key: Name
  Value: AtlasPay-Demo

Tag 2:
  Key: Environment
  Value: Development

Tag 3:
  Key: Project
  Value: Interview
```

**Click "Next: Configure Security Group"**

### 1.7 Configure Security Group

**Create new security group:**

```
Security group name: atlaspay-ec2-sg
Description: Security group for AtlasPay demo

Inbound rules:
┌─────────────┬────────┬──────────┬──────────────────┬──────────────┐
│ Type        │ Protocol│ Port     │ Source           │ Description  │
├─────────────┼────────┼──────────┼──────────────────┼──────────────┤
│ SSH         │ TCP    │ 22       │ 0.0.0.0/0        │ SSH Access   │
│ HTTP        │ TCP    │ 80       │ 0.0.0.0/0        │ HTTP         │
│ Custom TCP  │ TCP    │ 8080     │ 0.0.0.0/0        │ App API      │
│ HTTPS       │ TCP    │ 443      │ 0.0.0.0/0        │ HTTPS (opt)  │
└─────────────┴────────┴──────────┴──────────────────┴──────────────┘

Outbound rules:
  All traffic allowed (default)
```

**Click "Review and Launch"**

### 1.8 Review

```
Summary:
✓ AMI: Ubuntu 22.04 LTS
✓ Instance type: t2.micro
✓ Storage: 30 GB
✓ Security group: atlaspay-ec2-sg
✓ Public IP: Will be assigned

⚠️ WARNING shown about port 22 open to 0.0.0.0/0 is OK for demo
(For production: restrict to your IP only)
```

**Click "Launch"**

### 1.9 Create/Select Key Pair

```
Key pair dropdown: "Create a new key pair"

Key pair name: atlaspay-demo
Key pair type: RSA
File format: .pem (for Linux/macOS) or .ppk (for PuTTY)

✅ Click "Create key pair"
(File downloads automatically: atlaspay-demo.pem)
```

**SAVE THIS FILE SECURELY** - You'll need it to SSH into EC2

### 1.10 Launch Instance

```
✅ Click "Launch Instances"

Wait 2-3 minutes...
You'll see: "Your instances are now launching"
```

---

## 🔍 STEP 2: Get EC2 Public IP

### 2.1 Go to Instances Dashboard

```
EC2 Console → Instances (left sidebar)
```

### 2.2 Find Your Instance

```
Look for instance named: AtlasPay-Demo
Status should be: "running" (green)
Instances should show: 1/1 (healthy)
```

### 2.3 Get Public IP

```
Click instance → Details tab → Public IPv4 address

Example: 54.123.45.67

Copy this value - you'll use it to access the app
```

### 2.4 Wait for Instance to be Ready

```
Status Checks should show: 2/2 checks passed
(This takes 1-2 minutes after "running" status)
```

---

## 🔑 STEP 3: Connect via SSH

### 3.1 Windows (PowerShell)

```powershell
# Connect to your EC2 instance
ssh -i "C:\Users\1552441\Documents\AtlasKPKP\AtlasKP.pem" ubuntu@52.23.219.80

# Type 'yes' when asked "Are you sure you want to continue connecting?"
```

### 3.2 macOS / Linux

```bash
# Step 1: Secure the key file
chmod 600 ~/Downloads/atlaspay-demo.pem

# Step 2: Connect
ssh -i ~/Downloads/atlaspay-demo.pem ubuntu@54.123.45.67

# Type 'yes' when asked about continuing
```

### 3.3 Windows (Git Bash if PowerShell doesn't work)

```bash
# Inside Git Bash:
ssh -i /c/Users/YourUsername/Downloads/atlaspay-demo.pem ubuntu@54.123.45.67
```

### Expected Output:

```
Welcome to Ubuntu 22.04 LTS (GNU/Linux 5.15.0-1234-aws x86_64)

ubuntu@ip-10-0-1-5:~$
```

✅ **You're now inside the EC2 instance at 52.23.219.80!**

---

## 🐳 STEP 4: Install Docker & Docker Compose

Inside the SSH session, run:

```bash
# Update system packages
sudo apt update
sudo apt upgrade -y

# Install Docker and Docker Compose
sudo apt install -y docker.io docker-compose-v2 git

# Add ubuntu user to docker group (avoid needing sudo)
sudo usermod -aG docker ubuntu

# Apply group membership
newgrp docker

# Verify installation
docker --version
docker compose version
```

Expected output:
```
Docker version 24.x.x, build xxxxxx
Docker Compose version 2.x.x
```

---

## 📦 STEP 5: Clone & Deploy AtlasPay

```bash
# Create directory (optional)
mkdir -p ~/atlaspay
cd ~/atlaspay

# Clone AtlasPay repository
git clone https://github.com/Falcon-J/AtlasPay.git .
cd ~/atlaspay

# Start all services
docker compose up -d

# Watch startup progress
docker compose logs -f
```

Expected output (after 30-45 seconds):
```
postgres        | database system is ready to accept connections
redis           | Ready to accept connections
zookeeper       | ...binding...
kafka           | [KafkaServer id=1] started...
api             | INF starting API Gateway port=8080
```

Press `Ctrl+C` to stop tailing logs

---

## ✅ STEP 6: Verify Services

### 6.1 Check Service Status

```bash
docker compose ps
```

Expected output:
```
NAME            STATUS              PORTS
postgres        Up (healthy)        5432/tcp
redis           Up (healthy)        6379/tcp
zookeeper       Up                  2181/tcp
kafka           Up                  9092/tcp
api             Up (healthy)        0.0.0.0:8080->8080/tcp
```

### 6.2 Test Health Endpoint (from EC2)

```bash
curl http://localhost:8080/health
```

Expected output:
```json
{"status":"healthy","db":"up","cache":"up"}
```

### 6.3 Test from Your Laptop

```powershell
# Your EC2 public IP: 52.23.219.80
curl http://52.23.219.80:8080/health
```

Expected output:
```json
{"status":"healthy","db":"up","cache":"up"}
```

✅ **Your app is live!**

---

## 📊 STEP 7: Monitor Services

### 7.1 View Real-Time Logs

```bash
# All services
docker compose logs -f

# Specific service
docker compose logs -f api
docker compose logs -f postgres

# Last 50 lines
docker compose logs --tail 50
```

### 7.2 Check Resource Usage

```bash
# Docker stats
docker stats

# System memory
free -h

# Disk space
df -h
```

Expected memory usage:
```
CONTAINER     MEM USAGE
postgres      150M
redis         80M
kafka         350M
api           120M
─────────────────────
TOTAL         ~800M / 1GB
```

### 7.3 Restart Services

```bash
# Restart all
docker compose restart

# Restart specific
docker compose restart api
docker compose restart postgres

# Stop all
docker compose down

# Start all
docker compose up -d
```

---

## 🎬 STEP 8: Access the Application

### From Your Laptop:

```
HTTP: http://52.23.219.80:8080
API Health: http://52.23.219.80:8080/health
```

### Test API Endpoints:

```bash
# Health check
curl http://52.23.219.80:8080/health

# Create order (example)
curl -X POST http://52.23.219.80:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id":"user123","total_amount":100.00}'

# View orders
curl http://52.23.219.80:8080/orders
```

---

## 🔧 STEP 9: Common Tasks

### Stop Services (but keep EC2 running)

```bash
cd ~/atlaspay
docker compose down
```

### Start Services Again

```bash
cd ~/atlaspay
docker compose up -d
```

### View API Logs Only

```bash
docker compose logs -f api --tail 100
```

### SSH Back Into EC2

```powershell
ssh -i "C:\Users\1552441\Documents\AtlasKPKP\AtlasKP.pem" ubuntu@52.23.219.80
```

### Restart EC2 Instance (from AWS Console)

```
EC2 Dashboard → Instances → Select instance → Instance State → Restart

Services will auto-restart (docker compose configured with restart: unless-stopped)
```

---

## 💰 STEP 10: Cost Management

### Set Up Billing Alert (Important!)

```
AWS Console → Billing → Budgets → Create budget

Budget name: AtlasPay Free Tier Monitor
Monthly limit: $10
Alert when: forecasted spend exceeds $5
```

### Monitor Usage

```
Billing Dashboard → EC2 Usage
Look for:
  ✓ t2.micro: 750 hours/month (you have plenty)
  ✓ EBS: 30 GB (you're using all 30GB free)
  ✓ Data transfer: 100 GB/month free (monitor this)
```

### Cleanup When Done (Optional)

```
EC2 Dashboard → Instances → Select → Instance State → Terminate

⚠️ This deletes the instance and cannot be undone!
(Data on EBS volume will be lost)
```

---

## ⚠️ STEP 11: Memory Management (Important for 1GB RAM)

### Monitor Memory Usage

```bash
free -h
```

If memory > 900 MB, Kafka may be unstable.

### Temporary Fix: Stop Kafka

```bash
docker compose stop kafka zookeeper
```

✅ App continues to work (graceful degradation):
```bash
curl http://localhost:8080/health
# {"status":"degraded","db":"up","cache":"up"}
```

### Restart Kafka When Needed

```bash
docker compose start kafka zookeeper
```

---

## 🐛 TROUBLESHOOTING

### Problem: "Connection refused" on port 8080

**Solution:**
```bash
# Check if API is running
docker compose ps api

# Check logs
docker compose logs api

# Restart API
docker compose restart api

# Wait 10 seconds and try again
sleep 10
curl http://localhost:8080/health
```

### Problem: "PostgreSQL refused connection"

**Solution:**
```bash
# Check PostgreSQL status
docker compose logs postgres

# Restart
docker compose restart postgres

# Wait 15 seconds and try again
sleep 15
curl http://localhost:8080/health
```

### Problem: "Cannot connect from laptop (External IP)"

**Solution:**
```
1. Verify security group allows port 8080:
   EC2 Dashboard → Security Groups → atlaspay-ec2-sg
   
2. Check inbound rule for port 8080 with source 0.0.0.0/0

3. Verify public IP is correct:
   EC2 Dashboard → Instances → Public IPv4 address

4. Test from EC2 first:
   curl http://localhost:8080/health
   
5. Then from laptop with public IP
```

### Problem: "No space left on device"

**Solution:**
```bash
df -h  # Check disk usage

# Clean up Docker (removes unused images/containers)
docker system prune -a

# Or expand EBS volume in AWS Console (requires downtime)
```

### Problem: "Out of memory" (Kafka crashed)

**Solution:**
```bash
# This is expected with 1GB RAM + all services running
docker compose stop kafka zookeeper

# App continues to work
curl http://localhost:8080/health
# {"status":"degraded",...}

# Restart Kafka later when needed
docker compose start kafka zookeeper
```

---

## ✨ Quick Reference

| Task | Command |
|------|---------|
| SSH into EC2 | `ssh -i key.pem ubuntu@IP` |
| View logs | `docker compose logs -f` |
| Restart API | `docker compose restart api` |
| Stop all | `docker compose down` |
| Start all | `docker compose up -d` |
| Check IP | `curl http://169.254.169.254/latest/meta-data/public-ipv4` |
| Check memory | `free -h` |
| Docker stats | `docker stats` |

---

## 📋 Deployment Checklist

- [ ] AWS account created & verified
- [ ] EC2 instance launched (t2.micro, Ubuntu 22.04 LTS)
- [ ] Security group created (SSH + port 8080 open)
- [ ] Key pair downloaded and secured
- [ ] SSH connection successful
- [ ] Docker & Docker Compose installed
- [ ] AtlasPay cloned from GitHub
- [ ] `docker compose up -d` executed
- [ ] All services showing healthy status
- [ ] Health endpoint returns 200 OK
- [ ] App accessible from laptop (external IP:8080)
- [ ] Billing alert set up
- [ ] Ready for demo/interview!

---

## 🎯 Demo Flow

When demoing to Harness team:

```bash
# 1. SSH into EC2
ssh -i "C:\Users\1552441\Documents\AtlasKPKP\AtlasKP.pem" ubuntu@52.23.219.80

# 2. Show services running
docker compose ps

# 3. Show health endpoint
curl http://localhost:8080/health

# 4. Show logs
docker compose logs -f api

# 5. From your laptop - test order creation
curl -X POST http://52.23.219.80:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"user_id":"demo","amount":99.99}'

# 6. Show architecture diagram from INTERVIEW_BREAKDOWN.md
```

---

## 📞 Get Help

If something fails:

1. **Check logs**: `docker compose logs -f`
2. **Check service status**: `docker compose ps`
3. **Check memory**: `free -h` and `docker stats`
4. **SSH reconnect**: `ssh -i key.pem ubuntu@IP`
5. **AWS console**: Check instance status & security groups

---

## 🎉 You're Ready!

```
✅ EC2 instance running (free tier)
   Instance IP: 52.23.219.80
   
✅ All services deployed (Docker Compose)
   - PostgreSQL, Redis, Kafka, Zookeeper, API Gateway
   
✅ App accessible from internet
   Access: http://52.23.219.80:8080
   Health: http://52.23.219.80:8080/health
   
✅ Interview material ready (INTERVIEW_BREAKDOWN.md)
✅ Demo ready for Harness team!

SSH Access:
ssh -i "C:\Users\1552441\Documents\AtlasKPKP\AtlasKP.pem" ubuntu@52.23.219.80

Cost: $0 (free tier, 12 months)
```
