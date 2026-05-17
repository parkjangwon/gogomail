# GoGoMail Docker Compose Deployments

This directory contains Docker Compose configurations for different deployment scales of GoGoMail.

## Deployment Configurations

### 1. Development (`docker-compose.dev.yml`)

**Target**: Local development on a single machine

**Components**:
- PostgreSQL (single instance)
- Redis (single instance)
- MinIO (single node)
- ClamAV (`clamd` + persistent signature DB volume)
- GoGoMail backend (hot-reload via mounted volume)

**Usage**:
```bash
docker-compose -f docker-compose.dev.yml up
```

**Features**:
- Hot reload for Go backend
- Direct database access for testing
- Attachment byte scanning through a separate ClamAV container at `clamav:3310`
- Bounded ClamAV scan admission: only attachment-bearing messages are scanned,
  with backend concurrency, timeout, max-byte, and circuit-breaker controls
- Development-friendly logging
- No authentication required

---

### 2. Small Deployment (`docker-compose.small.yml`)

**Target**: 1-5 servers, single data center

**Components**:
- PostgreSQL (single instance)
- Redis (single instance)
- MinIO (single node)
- GoGoMail backend (1 instance)

**Usage**:
```bash
# Copy .env.example to .env and customize
cp .env.example .env

# Start services
docker-compose -f docker-compose.small.yml up -d

# View logs
docker-compose -f docker-compose.small.yml logs -f

# Stop services
docker-compose -f docker-compose.small.yml down
```

**Features**:
- Minimal resource overhead
- Basic health checks
- Simple backup strategy (volume-based)
- Good for testing and small deployments

---

### 3. Medium Deployment (`docker-compose.medium.yml`)

**Target**: 5-50 servers, single or multiple data centers

**Components**:
- PostgreSQL primary + replica (streaming replication)
- Redis master + slave + Sentinel (automatic failover)
- MinIO 3-node distributed
- GoGoMail backend (2 instances behind Nginx LB)
- Nginx load balancer for backend and MinIO
- Prometheus monitoring

**Supporting Files Required**:
- `nginx-backend.conf` - Backend load balancing
- `nginx-minio.conf` - MinIO load balancing
- `sentinel-1.conf` - Redis Sentinel configuration
- `prometheus.yml` - Prometheus scrape config
- `init-scripts/postgresql-replication-setup.sh` - DB replication setup

**Usage**:
```bash
# Setup configuration
cp .env.example .env
# Edit .env with your values

# Create init script
chmod +x init-scripts/postgresql-replication-setup.sh

# Start services
docker-compose -f docker-compose.medium.yml up -d

# Check health
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready
curl http://localhost:9000/minio/health/live
curl http://localhost:9090 # Prometheus

# View logs
docker-compose -f docker-compose.medium.yml logs -f backend-1
```

**Features**:
- High availability setup
- Automatic failover (Sentinel)
- Load balancing (Nginx)
- Metrics collection (Prometheus)
- Multi-instance backend
- Distributed object storage (MinIO 3-node)

**Database Replication**:
The PostgreSQL replica streams from primary. To verify:
```bash
# In primary container
docker-compose -f docker-compose.medium.yml exec postgres-primary \
  psql -U gogomail -d gogomail -c \
  "SELECT pid, usename, application_name, client_addr, state FROM pg_stat_replication;"
```

**Redis Failover**:
Sentinel monitors Redis and triggers automatic failover. To test:
```bash
# Connect to Sentinel
docker-compose -f docker-compose.medium.yml exec redis-sentinel-1 \
  redis-cli -p 26379 info sentinel

# View current master
docker-compose -f docker-compose.medium.yml exec redis-sentinel-1 \
  redis-cli -p 26379 SENTINEL masters
```

---

### 4. Large Deployment (`docker-compose.large.yml`)

**Target**: 50+ servers, multi-region

**Components**:
- PostgreSQL 3-node cluster
- etcd 3-node cluster (distributed coordination)
- Redis 3-node cluster (cluster mode enabled)
- MinIO 6-node distributed
- GoGoMail backend (3 instances behind HAProxy)
- HAProxy (advanced load balancing + stats)
- Prometheus (90-day retention)
- Grafana (visualization)
- Elasticsearch + Logstash + Kibana (ELK stack)
- AWS CloudWatch logging (optional)

**Supporting Files Required**:
- `haproxy-large.cfg` - Advanced load balancing
- `prometheus-large.yml` - Comprehensive metrics
- `elasticsearch.yml` - Elasticsearch config
- `logstash.conf` - Log pipeline
- `grafana-provisioning/datasources.yml` - Data sources
- `grafana-provisioning/dashboard-provider.yml` - Dashboards
- `init-scripts/postgresql-cluster-setup.sh` - Cluster setup

**Usage**:
```bash
# Setup configuration
cp .env.example .env

# Edit .env with production values
# Set GRAFANA_ADMIN_PASSWORD, AWS credentials (optional), etc.

# Start services
docker-compose -f docker-compose.large.yml up -d

# Wait for cluster initialization (2-3 minutes)
sleep 180

# Verify cluster health
docker-compose -f docker-compose.large.yml exec etcd-1 \
  etcdctl --endpoints=localhost:2379 member list

# Access monitoring
# Prometheus: http://localhost:9090
# Grafana: http://localhost:3000 (admin/your-password)
# Kibana: http://localhost:5601
# HAProxy Stats: http://localhost:8404/stats
```

**Features**:
- Distributed architecture
- Multi-region ready
- Full clustering (PostgreSQL, Redis, etcd)
- Advanced load balancing (HAProxy)
- Comprehensive monitoring (Prometheus + Grafana)
- Centralized logging (ELK stack)
- CloudWatch integration (optional)
- 90-day metrics retention
- Container per service (no limits on scale)

**Cluster Status Commands**:
```bash
# PostgreSQL cluster
docker-compose -f docker-compose.large.yml exec postgres-node-1 \
  psql -U gogomail -d gogomail -c "SELECT version();"

# Redis cluster
docker-compose -f docker-compose.large.yml exec redis-node-1 \
  redis-cli cluster info

# etcd cluster
docker-compose -f docker-compose.large.yml exec etcd-1 \
  etcdctl --endpoints=localhost:2379 endpoint health

# MinIO cluster
curl -s http://localhost:9000/minio/v2/metrics/cluster | head -20
```

---

## Common Operations

### View Logs

```bash
# All services
docker-compose -f docker-compose.SCALE.yml logs

# Specific service
docker-compose -f docker-compose.SCALE.yml logs backend-1

# Follow logs
docker-compose -f docker-compose.SCALE.yml logs -f

# Last 100 lines
docker-compose -f docker-compose.SCALE.yml logs --tail 100
```

### Stop/Remove Volumes

```bash
# Stop services (keep volumes)
docker-compose -f docker-compose.SCALE.yml down

# Remove everything (including volumes)
docker-compose -f docker-compose.SCALE.yml down -v
```

### Scaling (Medium/Large)

```bash
# Medium: Add a third backend instance
docker-compose -f docker-compose.medium.yml up -d --scale backend=3

# Large: Scale Redis nodes (note: requires cluster reconfiguration)
docker-compose -f docker-compose.large.yml up -d redis-node-4
```

### Backup Strategies

**Development/Small**:
```bash
# Docker volumes backup
docker run --rm -v gogomail_postgres-data:/data -v $(pwd):/backup \
  alpine tar czf /backup/postgres-backup.tar.gz -C / data
```

**Medium**:
```bash
# PostgreSQL replica backup (non-blocking)
docker-compose -f docker-compose.medium.yml exec postgres-replica \
  pg_dump -U gogomail gogomail | gzip > gogomail-backup.sql.gz

# MinIO backup using mc
docker-compose -f docker-compose.medium.yml exec minio-1 \
  mc mirror local/gogomail /backup/gogomail
```

**Large**:
```bash
# Use PostgreSQL cluster backup tool
# Use MinIO multi-part snapshots
# Export Elasticsearch indices to S3
```

### Monitoring Access

**Prometheus**: `http://localhost:9090`
- Query: `http_requests_total{job="gogomail-backend"}`
- Alerts: `/alerts`

**Grafana** (Large only): `http://localhost:3000`
- Default: admin / (see .env GRAFANA_ADMIN_PASSWORD)
- Pre-configured datasources: Prometheus, Elasticsearch

**Kibana** (Large only): `http://localhost:5601`
- Logs: `gogomail-*` index pattern

**HAProxy Stats** (Large only): `http://localhost:8404/stats`

---

## Environment Variables

See `.env.example` for all available variables. Key settings:

```env
# Database
POSTGRES_PASSWORD=changeme
POSTGRES_REPL_PASSWORD=repl_password_change_me

# Object Storage
MINIO_ROOT_PASSWORD=changepassword123

# Backend
BACKEND_IMAGE=gogomail:latest
APP_LOG_LEVEL=info

# Large Deployment
GRAFANA_ADMIN_PASSWORD=admin
ELASTICSEARCH_MEMORY=512m
```

---

## Troubleshooting

### Services Won't Start

```bash
# Check logs
docker-compose -f docker-compose.SCALE.yml logs

# Inspect specific service
docker-compose -f docker-compose.SCALE.yml ps
docker-compose -f docker-compose.SCALE.yml inspect SERVICE_NAME
```

### Health Check Failures

```bash
# Check backend health
curl http://localhost:8080/health/live
curl http://localhost:8080/health/ready

# PostgreSQL connectivity
docker-compose -f docker-compose.SCALE.yml exec postgres \
  pg_isready -U gogomail

# Redis connectivity
docker-compose -f docker-compose.SCALE.yml exec redis \
  redis-cli ping

# MinIO connectivity
curl -I http://localhost:9000/minio/health/live
```

### Port Conflicts

If ports are already in use:
1. Edit docker-compose file and change port mappings
2. Or stop conflicting services: `lsof -i :PORT` → `kill -9 PID`

### Network Issues (Medium/Large)

```bash
# Check network connectivity
docker-compose -f docker-compose.SCALE.yml exec backend-1 \
  ping postgres-primary

# DNS resolution
docker-compose -f docker-compose.SCALE.yml exec backend-1 \
  nslookup redis-master
```

---

## Production Checklist

- [ ] Update all passwords in `.env`
- [ ] Configure SSL/TLS certificates in `certs/`
- [ ] Set `POSTGRES_PASSWORD`, `MINIO_ROOT_PASSWORD`, etc.
- [ ] Configure CloudWatch credentials (if using AWS)
- [ ] Set `GOGOMAIL_ENV=production`
- [ ] Disable development logging (`APP_LOG_LEVEL=warn`)
- [ ] Test failover scenarios
- [ ] Setup backup schedules
- [ ] Configure log rotation
- [ ] Monitor metrics in Prometheus/Grafana
- [ ] Setup alerting in Prometheus/Grafana
- [ ] Document recovery procedures

---

## Support

For issues or questions:
1. Check service logs: `docker-compose logs SERVICE_NAME`
2. Verify network connectivity between services
3. Check disk space and memory usage
4. Review configuration in `.env` and `*.conf` files
