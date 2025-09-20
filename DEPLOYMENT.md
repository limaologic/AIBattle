# Reverse Challenge System - Deployment Guide

## 📋 Prerequisites

### System Requirements
- **Go**: Version 1.21 or later
- **SQLite**: For database storage
- **ngrok**: For public callback URLs (development)
- **Operating System**: Linux, macOS, or Windows

### Install Go (if not already installed)

**Windows:**
1. Download from https://golang.org/dl/
2. Run installer and follow instructions
3. Verify: `go version`

**Linux/macOS:**
```bash
# Using package manager (Ubuntu)
sudo apt update
sudo apt install golang-go

# Or download binary
wget https://go.dev/dl/go1.21.x.linux-amd64.tar.gz
sudo tar -C /usr/local -xzf go1.21.x.linux-amd64.tar.gz
export PATH=$PATH:/usr/local/go/bin
```

### Install ngrok
```bash
# Download from https://ngrok.com/download
# Or using package managers:

# Windows (choco)
choco install ngrok

# macOS (brew)
brew install ngrok

# Ubuntu
snap install ngrok
```

## 🚀 Quick Setup

### 1. Clone and Dependencies
```bash
git clone <repository>
cd reverse-challenge-system

# Download dependencies
go mod download
go mod tidy
```

### 2. Environment Configuration
```bash
# Copy example environment
cp .env.example .env

# Edit configuration
nano .env  # or your preferred editor
```

**Critical Settings:**
```bash
# IMPORTANT: Set these before starting
PUBLIC_CALLBACK_HOST=https://your-ngrok-url.ngrok.io
SHARED_SECRET_KEY=your-very-secure-secret-key-change-this
```

### 3. Start Public Tunnel (Development)
```bash
# In terminal 1
ngrok http 8080

# Copy the https://xxx.ngrok.io URL
# Update PUBLIC_CALLBACK_HOST in .env with this URL
```

### 4. Start Services
```bash
# Terminal 2 - Challenger Service
go run cmd/challenger/main.go

# Terminal 3 - Solver Service  
go run cmd/solver/main.go
```

### 5. Test the System
```bash
# Terminal 4 - Send test challenges
go run examples/send_challenge.go
```

## 🔧 Production Deployment

### Docker Deployment

**Dockerfile.challenger:**
```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o challenger cmd/challenger/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite
WORKDIR /root/
COPY --from=builder /app/challenger .
CMD ["./challenger"]
```

**Dockerfile.solver:**
```dockerfile  
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o solver cmd/solver/main.go

FROM alpine:latest
RUN apk --no-cache add ca-certificates sqlite
WORKDIR /root/
COPY --from=builder /app/solver .
CMD ["./solver"]
```

**docker-compose.yml:**
```yaml
version: '3.8'

services:
  challenger:
    build:
      context: .
      dockerfile: Dockerfile.challenger
    ports:
      - "8080:8080"
    environment:
      - CHALLENGER_HOST=0.0.0.0
      - CHALLENGER_PORT=8080
      - PUBLIC_CALLBACK_HOST=https://your-domain.com
      - SHARED_SECRET_KEY=${SHARED_SECRET_KEY}
      - CHALLENGER_DB_PATH=/data/challenger.db
    volumes:
      - challenger_data:/data
    restart: unless-stopped

  solver:
    build:
      context: .
      dockerfile: Dockerfile.solver
    ports:
      - "8081:8081"
    environment:
      - SOLVER_HOST=0.0.0.0
      - SOLVER_PORT=8081
      - SOLVER_WORKER_COUNT=8
      - SHARED_SECRET_KEY=${SHARED_SECRET_KEY}
      - SOLVER_DB_PATH=/data/solver.db
    volumes:
      - solver_data:/data
    restart: unless-stopped

volumes:
  challenger_data:
  solver_data:
```

## 🔌 gRPC Bridge（外部解題服務整合）

Solver 可選擇開啟 gRPC Bridge，供外部程式（如 Python LLM/ML 工作者）提交答案。預設關閉，僅在使用 build tag `grpcbridge` 時啟用。

- 建置：
  - 先產生 Go 端 stubs：`make proto-gen-go`
  - 加入 gRPC 相依（需網路）：`make grpc-deps`
  - 以標籤建置：`make build-solver-grpc`
- 執行：
  - `SOLVER_GRPC_BRIDGE_ADDR=:9090 ./bin/solver`（未設定時預設 `:9090`）
- Python 範例客戶端：
  - 生成 Python stubs：`make proto-gen-py`
  - 送出答案（Mock LLM 回覆）：
    - `python examples/grpc/client.py --challenge-id ch_123 --job-id solver_job_ch_123 --answer "MOCK_ANSWER" --target localhost:9090`

安全性建議（正式環境）：
- 將 gRPC Bridge 侷限於私網/服務網段，或置於 mTLS 反向代理之後（envoy/nginx stream）。
- 規劃呼叫端驗證（metadata token、mTLS SAN 檢查）。
- 設定 gRPC timeout、監控與告警。

### Systemd Service (Linux)

**challenger.service:**
```ini
[Unit]
Description=Reverse Challenge System - Challenger
After=network.target

[Service]
Type=simple
User=rcs
WorkingDirectory=/opt/reverse-challenge-system
ExecStart=/opt/reverse-challenge-system/challenger
Restart=on-failure
RestartSec=5
Environment=LOG_LEVEL=info

[Install]
WantedBy=multi-user.target
```

**solver.service:**
```ini
[Unit]
Description=Reverse Challenge System - Solver  
After=network.target

[Service]
Type=simple
User=rcs
WorkingDirectory=/opt/reverse-challenge-system
ExecStart=/opt/reverse-challenge-system/solver
Restart=on-failure
RestartSec=5
Environment=LOG_LEVEL=info

[Install]
WantedBy=multi-user.target
```

### Reverse Proxy (nginx)

```nginx
# /etc/nginx/sites-available/rcs-challenger
server {
    listen 80;
    server_name challenger.yourdomain.com;
    return 301 https://$server_name$request_uri;
}

server {
    listen 443 ssl http2;
    server_name challenger.yourdomain.com;
    
    ssl_certificate /path/to/cert.pem;
    ssl_certificate_key /path/to/key.pem;
    
    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        # Important: preserve original request
        proxy_buffering off;
        proxy_request_buffering off;
    }
}
```

## 🔍 Monitoring & Logging

### Health Check Endpoints
- Challenger: `https://challenger.domain.com/healthz`
- Solver: `https://solver.domain.com/healthz`
- Readiness: `https://*/readyz` (checks database)

### Log Aggregation
```bash
# Using journalctl for systemd services
journalctl -u challenger.service -f
journalctl -u solver.service -f

# Using Docker logs
docker-compose logs -f challenger
docker-compose logs -f solver
```

### Database Monitoring
```sql
-- Monitor challenge volume
SELECT 
    DATE(created_at) as date,
    COUNT(*) as challenges
FROM challenges 
GROUP BY DATE(created_at) 
ORDER BY date DESC;

-- Monitor success rates  
SELECT 
    status,
    COUNT(*) as count,
    AVG(compute_time_ms) as avg_compute_ms
FROM results 
GROUP BY status;

-- Monitor retry attempts
SELECT 
    attempt_count,
    COUNT(*) as challenges
FROM pending_challenges 
GROUP BY attempt_count;
```

## 🔒 Security Checklist

### HTTPS Configuration
- [ ] Valid SSL certificates installed
- [ ] HTTP redirects to HTTPS  
- [ ] HSTS headers configured
- [ ] Callback URLs validated against whitelist

### Authentication
- [ ] Strong HMAC keys generated (32+ chars)
- [ ] Keys rotated regularly
- [ ] Clock synchronization configured (NTP)
- [ ] Nonce storage cleaned regularly

### Network Security
- [ ] Firewall rules configured
- [ ] Rate limiting implemented
- [ ] DDoS protection enabled
- [ ] VPN/private networks for inter-service communication

### Application Security
- [ ] Request size limits configured
- [ ] SQL injection protections (using parameterized queries)
- [ ] Input validation on all endpoints
- [ ] Error messages don't leak sensitive info

## 📊 Performance Tuning

### Database Optimization
```sql
-- Enable WAL mode for better concurrency
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA cache_size=10000;
PRAGMA temp_store=memory;
```

### Go Service Optimization
```bash
# Build with optimizations
go build -ldflags="-w -s" -o challenger cmd/challenger/main.go
go build -ldflags="-w -s" -o solver cmd/solver/main.go

# Runtime tuning
export GOGC=100
export GOMAXPROCS=8
```

### Worker Pool Tuning
```bash
# Adjust based on available resources
SOLVER_WORKER_COUNT=16  # For CPU-bound tasks
SOLVER_WORKER_COUNT=64  # For I/O-bound tasks
```

## 🚨 Troubleshooting

### Common Issues

**HMAC Authentication Failures:**
```bash
# Check system clocks are synchronized
timedatectl status  # Linux
w32tm /query /status  # Windows

# Verify configuration
grep -i hmac .env
```

**Callback Failures:**
```bash
# Test callback URL accessibility
curl -I https://your-callback-host.com/healthz

# Check ngrok status
ngrok status
```

**Database Locks:**
```sql
-- Check for long-running transactions
PRAGMA compile_options;  -- Verify WAL mode
.mode column
.headers on
PRAGMA table_info(challenges);
```

### Log Analysis
```bash
# Look for authentication errors
grep "signature.*failed" logs/challenger.log

# Monitor retry patterns
grep "retry.*attempt" logs/solver.log

# Check callback success rates  
grep "callback.*status" logs/solver.log | grep -c "200"
```

## 📈 Scaling Considerations

### Horizontal Scaling
- Use external database (PostgreSQL/MySQL)
- Implement service discovery
- Add load balancer with sticky sessions for callbacks
- Use Redis for shared nonce storage

### Vertical Scaling
- Increase worker pool size
- Optimize database connection pools
- Tune Go garbage collector
- Use faster storage (SSD/NVMe)

## 🔄 Backup & Recovery

### Database Backup
```bash
# SQLite backup
sqlite3 challenger.db ".backup challenger-backup-$(date +%Y%m%d).db"
sqlite3 solver.db ".backup solver-backup-$(date +%Y%m%d).db"

# Automated backup script
#!/bin/bash
backup_dir="/backup/rcs"
timestamp=$(date +%Y%m%d_%H%M%S)
sqlite3 /data/challenger.db ".backup ${backup_dir}/challenger_${timestamp}.db"
sqlite3 /data/solver.db ".backup ${backup_dir}/solver_${timestamp}.db"
find ${backup_dir} -name "*.db" -mtime +7 -delete
```

### Configuration Backup
```bash
# Backup environment and config
tar -czf rcs-config-backup.tar.gz .env *.service nginx/sites-available/
```

This deployment guide provides comprehensive instructions for both development and production environments. Adjust configuration values based on your specific infrastructure and security requirements.
