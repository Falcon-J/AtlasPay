# AWS EC2 Free Tier - Lean Deployment Guide

## 🎯 Quick Start (5 minutes)

### Prerequisites
- AWS Free Tier account
- EC2 key pair (.pem file)
- SSH client (built-in on macOS/Linux, Git Bash on Windows)

---

## 📋 Step 1: Launch EC2 Instance

### Via AWS Console
1. Go to **EC2 Dashboard** → **Instances** → **Launch Instances**
2. Select **Ubuntu 22.04 LTS** (free tier eligible)
3. Instance type: **t2.micro** (1 vCPU, 1GB RAM)
4. Security Group: Allow inbound:
   - **Port 22** (SSH) - from your IP only
   - **Port 8080** (App) - from 0.0.0.0/0
   - **Port 80** (Optional, for nginx reverse proxy)
5. Storage: **30GB** (free tier default)
6. Create/select key pair and download `.pem` file
7. **Launch Instance**

### Via Terraform (Optional)
```bash
cd deployments
terraform init
terraform apply -var="instance_type=t2.micro" -var="ami_name=ubuntu-22.04"
```

---

## 🔑 Step 2: Connect via SSH

### Windows (PowerShell / Git Bash)
```bash
# Set permissions on key
icacls "C:\path\to\key.pem" /inheritance:r /grant:r "%username%:F"

# Connect
ssh -i "C:\path\to\key.pem" ubuntu@YOUR_EC2_PUBLIC_IP
```

### macOS / Linux
```bash
chmod 600 ~/Downloads/key.pem
ssh -i ~/Downloads/key.pem ubuntu@YOUR_EC2_PUBLIC_IP
```

**Expected output:**
```
Welcome to Ubuntu 22.04 LTS (GNU/Linux 5.15.0-1234-aws x86_64)
ubuntu@ip-10-0-1-5:~$
```

---

## 🐳 Step 3: Install Docker

Inside EC2:

```bash
sudo apt update
sudo apt install -y docker.io docker-compose-v2 git

# Add user to docker group (avoid sudo)
sudo usermod -aG docker ubuntu

# Apply group changes
newgrp docker

# Verify
docker --version
docker compose version
```

---

## 📦 Step 4: Deploy AtlasPay

```bash
# Clone repo
git clone https://github.com/Falcon-J/AtlasPay.git
cd AtlasPay

# Start all services
docker compose up -d

# Check status
docker compose ps
```

**Expected output:**
```
NAME         STATUS          PORTS
postgres     Up (healthy)    5432/tcp
redis        Up (healthy)    6379/tcp
zookeeper    Up              2181/tcp
kafka        Up              9092/tcp
api          Up (healthy)    0.0.0.0:8080->8080/tcp
```

---

## ✅ Step 5: Verify Deployment

```bash
# Check API health
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","db":"up","cache":"up"}
```

**From your laptop:**
```bash
curl http://YOUR_EC2_PUBLIC_IP:8080/health
```

---

## 📊 Step 6: Monitor Services

```bash
# View real-time logs
docker compose logs -f

# Tail only API logs
docker compose logs -f api

# Check resource usage
docker stats
```

---

## 🎬 Service Status Commands

```bash
# View all services
docker compose ps

# Restart a service
docker compose restart api

# Stop all
docker compose down

# Start all
docker compose up -d

# View logs of specific service
docker compose logs postgres
```

---

## ⚠️ Memory Management (1GB RAM)

### If Kafka becomes unstable:
```bash
# Temporarily stop Kafka (app still works)
docker compose stop kafka zookeeper

# App will degrade gracefully (no event streaming)
curl http://localhost:8080/health
# {"status":"degraded","db":"up","cache":"up"}

# Restart when needed
docker compose start kafka zookeeper
```

### Memory usage by service:
- **PostgreSQL**: 100-200 MB
- **Redis**: 50-100 MB
- **Kafka + Zookeeper**: 300-400 MB (with -Xmx256M constraint)
- **API**: 100-150 MB
- **System overhead**: 100-150 MB
- **Total**: ~800-1000 MB (fits in 1GB)

---

## 🚀 Production Optimizations (Optional)

### 1. Nginx Reverse Proxy (HTTP/HTTPS)
```bash
# Install nginx
sudo apt install -y nginx

# Configure reverse proxy
sudo tee /etc/nginx/sites-available/atlaspay > /dev/null <<EOF
server {
    listen 80;
    server_name _;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
    }
}
EOF

# Enable and restart
sudo ln -sf /etc/nginx/sites-available/atlaspay /etc/nginx/sites-enabled/
sudo systemctl restart nginx
```

Access via **http://YOUR_EC2_PUBLIC_IP** (port 80)

### 2. HTTPS with Let's Encrypt
```bash
sudo apt install -y certbot python3-certbot-nginx
sudo certbot certonly --nginx -d yourdomain.com
```

### 3. CloudWatch Logs
```bash
# View system logs
tail -f /var/log/syslog | grep docker

# Or use EC2 CloudWatch integration (requires IAM role)
```

### 4. Auto-restart on reboot
```bash
# Docker compose services automatically restart
# (restart: unless-stopped in docker-compose.yml)

# Verify by rebooting
sudo reboot
```

---

## 📈 Performance Expectations

| Metric | Expected |
|--------|----------|
| Boot time | 2-3 minutes |
| Service startup | 30-45 seconds |
| API response time | <100ms (healthy) |
| CPU usage (idle) | 5-10% |
| CPU usage (load) | 30-50% |
| Memory usage | 900-950 MB |
| Disk I/O | Low |

---

## 🔒 Security Checklist

- [ ] SSH key stored securely (not in repo)
- [ ] Security group restricts SSH to your IP only
- [ ] Port 8080 open only if needed (or use firewall rules)
- [ ] Regularly rotate SSH keys
- [ ] Monitor EC2 console for unusual activity
- [ ] Set up EC2 Systems Manager Session Manager (browser-based SSH)

---

## 💰 Cost Control

### Monthly free tier limits (12 months):
- **EC2 t2.micro**: 750 hours/month = ~31 days continuous
- **Data transfer**: 100 GB/month outbound
- **EBS storage**: 30 GB/month

### After free tier expires:
- **EC2 t2.micro**: ~$0.0116/hour = $8.40/month
- **EBS storage**: ~$0.10/GB = $3.00/month for 30GB
- **Data transfer**: ~$0.09/GB over 100GB

**Tip**: Set up AWS Budgets to alert if costs exceed $10/month

---

## 🐛 Troubleshooting

### "Connection refused" on port 8080
```bash
# Check if API is running
docker compose ps api

# View API logs
docker compose logs api

# Restart API
docker compose restart api
```

### "Kafka exited with code 137" (Out of memory)
```bash
# This is expected on heavy load with 1GB RAM
# Stop Kafka temporarily
docker compose stop kafka zookeeper

# App continues with graceful degradation
```

### "PostgreSQL refused connection"
```bash
# Check PostgreSQL status
docker compose ps postgres

# View logs
docker compose logs postgres

# Restart
docker compose restart postgres
```

### "No space left on device"
```bash
# Check disk usage
df -h

# Clean up unused Docker images
docker system prune -a

# Remove old logs
docker logs --tail 0 -f
```

---

## 🔄 Useful Commands

```bash
# Inside EC2 SSH session
cd ~/AtlasPay

# Start fresh
docker compose down -v
docker compose up -d

# Monitor in real-time
watch -n 5 'docker stats --no-stream'

# Backup database
docker compose exec postgres pg_dump -U atlaspay atlaspay > backup.sql

# Restore database
docker compose exec -T postgres psql -U atlaspay atlaspay < backup.sql

# SSH into container
docker compose exec api sh

# Check specific service port
docker compose port api 8080

# Get EC2 public IP
curl http://169.254.169.254/latest/meta-data/public-ipv4
```

---

## 🎯 For Demo/Interview

### Recommended setup:
```bash
# Run full stack
docker compose up -d

# In a second terminal, tail logs
docker compose logs -f

# Access app
curl http://localhost:8080/health
```

### If Kafka causes issues:
```bash
# Stop Kafka
docker compose stop kafka zookeeper

# App still works
curl http://localhost:8080/health
# {"status":"degraded",...}
```

---

## 📞 Quick Reference

| Task | Command |
|------|---------|
| View all logs | `docker compose logs -f` |
| Restart API | `docker compose restart api` |
| Stop all | `docker compose down` |
| Check IP | `curl http://169.254.169.254/latest/meta-data/public-ipv4` |
| SSH reconnect | `ssh -i key.pem ubuntu@IP` |
| System stats | `free -h && df -h` |
| Docker stats | `docker stats` |

---

## ✨ Next Steps

1. **Test locally first**: `docker compose up` on your laptop
2. **Deploy to EC2**: Follow steps 1-5 above
3. **Share endpoint**: Give your IP to team for demos
4. **Monitor memory**: Use `docker stats` during load testing
5. **Setup domain** (optional): Point subdomain to EC2 Elastic IP

---

## 🆘 Support

If services keep crashing:

1. Check logs: `docker compose logs -f`
2. Free up memory: `docker compose stop kafka zookeeper`
3. Increase disk space: AWS Console → EBS → Expand volume
4. Restart EC2: AWS Console → Reboot Instance

**For 1GB RAM with all services + Kafka**: Expect occasional memory pressure. This is normal for free tier. App will gracefully degrade.
