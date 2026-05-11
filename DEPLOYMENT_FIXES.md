# Deployment Fixes — Render Ready

## 🔧 What Was Fixed

### **Issue 1: Database Connection Race Condition**
- **Problem:** Render provisions PostgreSQL asynchronously. App was calling `logger.Fatal()` on first connection failure.
- **Solution:** Added `connectWithRetry()` function with exponential backoff (1s → 1.5s → 2.25s... max 30s).
- **Result:** App now waits up to ~90s for database to be ready instead of crashing immediately.

### **Issue 2: Migration File Path**
- **Problem:** Hardcoded `./migrations/001_init.sql` doesn't work reliably in Docker/Render containers.
- **Solution:** Added `readMigrationFile()` that tries 8 different paths:
  - `./migrations/001_init.sql` (relative)
  - `/app/migrations/001_init.sql` (Docker absolute)
  - `scripts/migrations/001_init.sql` (monorepo layout)
  - Executable-relative paths (for compiled binary)
- **Result:** Works in local dev, Docker, and Render deployment scenarios.

### **Issue 3: Graceful Degradation**
- **Already working:** Redis and Kafka are optional (app continues without them)
- **Verified:** Redis connection failure logs warning and app continues
- **Verified:** Kafka is disabled by default (KAFKA_ENABLED=false)

---

## ✅ Testing Checklist

### **1. Local Docker Test (5 minutes)**
```bash
# Run full stack
docker-compose up -d

# Wait 10 seconds for services to initialize
sleep 10

# Check app is running
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","db":"up","cache":"up"}

# Check readiness
curl http://localhost:8080/ready
# Expected: {"ready":true}

# Stop everything
docker-compose down
```

### **2. Local Go Test (Direct Build)**
```bash
# Install dependencies
go mod download

# Run migrations (if not already in DB)
# psql -U atlaspay -h localhost -d atlaspay -f scripts/migrations/001_init.sql

# Start the app
go run cmd/api-gateway/main.go

# In another terminal, test health
curl http://localhost:8080/health
```

### **3. Render Deployment Checklist**
- [ ] Push code to GitHub (includes all fixes in `cmd/api-gateway/main.go`)
- [ ] Connect Render to GitHub repo
- [ ] Render auto-detects `render.yaml` and provisions:
  - [ ] PostgreSQL database (free tier, 90 day limit)
  - [ ] Redis cache (free tier)
  - [ ] Go application (free tier, auto-scale 0-1 dyno)
- [ ] Wait 2-3 minutes for first deploy
- [ ] Check Render dashboard → Logs for:
  ```
  [INFO] starting API Gateway
  [INFO] database connection established successfully
  [INFO] auto-migration completed successfully
  [INFO] API Gateway listening
  ```
- [ ] Test via Render URL:
  ```bash
  curl https://{your-render-domain}/health
  # Expected: {"status":"healthy","db":"up","cache":"up"}
  ```

---

## 📊 What Each Fix Does

| Fix | Impact | Risk | Rollback |
|-----|--------|------|----------|
| DB retry logic | **HIGH** — fixes Render startup failures | **NONE** — backwards compatible | Trivial (just removes retry loop) |
| Migration file path | **HIGH** — enables multiple deployment scenarios | **LOW** — tries fallback paths, logs warnings | Trivial (reverts to single path) |
| Graceful degradation | **LOW** — already implemented, just verified | **NONE** — no changes | N/A |

---

## 🚀 Deployment Steps (TL;DR)

1. **Push to GitHub**
   ```bash
   git add .
   git commit -m "fix: add db retry and migration path handling for render deployment"
   git push
   ```

2. **Deploy via Render**
   - Go to [Render.com](https://render.com)
   - Click **New +** → **Blueprint**
   - Select your GitHub repo
   - Render auto-detects `render.yaml`
   - Click **Apply**
   - Wait 2-3 minutes

3. **Verify**
   ```bash
   curl https://{render-domain}/health
   ```

---

## 🔍 Diagnostics

### **If health check returns `"status":"degraded"`**
```json
{
  "status": "degraded",
  "db": "down",
  "cache": "not configured"
}
```
**This is OK!** It means:
- Database is connecting (it will retry and eventually connect)
- Cache is disabled (Redis not available yet, or REDIS_HOST not set)
- App is still running and serving requests

### **If health check times out or returns 502**
1. Check Render logs for:
   ```
   [ERROR] database connection failed
   [WARN] retrying database connection...
   ```
2. Wait another 30 seconds (DB provisioning can take 60+ seconds on Render free tier)
3. Retry the health check

### **If you see migration errors**
```
[WARN] failed to read migration file, skipping auto-migration
```
This is expected if running without schema. The app will log available paths and continue.

---

## 📝 Code Changes Summary

### **File: `cmd/api-gateway/main.go`**

**Added imports:**
- `fmt` — for formatted error messages
- `path/filepath` — for cross-platform path handling

**Modified:**
- `database.NewPostgresDB()` → `connectWithRetry()` — now has exponential backoff
- `os.ReadFile("./migrations/...")` → `readMigrationFile()` — now tries multiple paths

**Added functions:**
- `connectWithRetry()` — retries DB connection up to 10 times with exponential backoff
- `readMigrationFile()` — tries 8 different migration file paths

**No changes to:**
- Business logic (Order, Payment, Inventory services)
- API endpoints
- Configuration system
- Docker/Kubernetes manifests

---

## ✨ Next Steps for Interview

Once deployment is working:

1. **Show the live deployment:**
   - "This is a distributed payment platform running on Render, Postgres, and Redis"
   - Click `/health` endpoint to show system status
   - Explain the three layers: API Gateway, Services, Data Layer

2. **Walk through the saga pattern:**
   - Explain orchestrated saga (not choreography)
   - Show order/payment/inventory flow
   - Mention idempotency + compensation

3. **Highlight the resilience:**
   - "We added DB retry logic for cloud environments"
   - "Migration file paths work across local/Docker/Render"
   - "App gracefully degrades if Redis or Kafka unavailable"

---

## 🆘 Troubleshooting

| Issue | Diagnosis | Fix |
|-------|-----------|-----|
| `502 Bad Gateway` | App crashed or DB not ready | Wait 60s, check logs for `database connection failed` |
| `Connection refused` | Port binding issue | Check `SERVER_PORT=8080` in Render env vars |
| Migration errors | File not found | Check Dockerfile copies to `/app/migrations` ✅ |
| Cache down | Redis not available | Expected on free tier, app continues without cache |
| Kafka errors | Kafka broker not found | Expected (Kafka disabled on free tier), app continues without events |

---

*Updated: 2026-05-11 — For Render free tier deployment*
