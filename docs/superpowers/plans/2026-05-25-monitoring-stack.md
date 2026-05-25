# Monitoring Stack (Prometheus + Loki + Grafana) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers-extended-cc:subagent-driven-development (recommended) or superpowers-extended-cc:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Docker Compose overlay that adds Prometheus + Loki + Grafana + Promtail to any existing gogomail compose environment, with Prometheus and Loki APIs directly accessible for AI agent queries.

**Architecture:** A single `docker-compose.monitoring.yml` overlay file extends the `backend` service with metrics env vars and adds a `gogomail-monitoring` bridge network that all monitoring services share. Promtail reads container logs via the Docker socket and ships JSON logs to Loki with `service`, `component`, `level` labels. Prometheus scrapes `backend:9091/metrics`. Grafana is pre-provisioned with both datasources and a GoGoMail overview dashboard.

**Tech Stack:** Docker Compose overlay, prom/prometheus:v2.53.1, grafana/loki:3.1.0, grafana/promtail:3.1.0, grafana/grafana:11.1.0, Go backend Prometheus adapter (existing).

---

## File Map

| Action | Path |
|--------|------|
| Create | `docker/docker-compose.monitoring.yml` |
| Create | `docker/prometheus-monitoring.yml` |
| Create | `docker/loki-config.yml` |
| Create | `docker/promtail-config.yml` |
| Create | `docker/grafana-provisioning/datasources/datasources.yml` |
| Create | `docker/grafana-provisioning/dashboards/provider.yml` |
| Create | `docker/grafana-provisioning/dashboards/gogomail-overview.json` |
| Delete | `docker/grafana-provisioning/datasources.yml` (was never loaded — wrong path) |
| Delete | `docker/grafana-provisioning/dashboard-provider.yml` (was never loaded — wrong path) |
| Create | `docs/MONITORING.md` |

---

### Task 1: Docker Compose Overlay + Config Files

**Goal:** Four config files that define the monitoring service topology and give the backend its metrics endpoint.

**Files:**
- Create: `docker/docker-compose.monitoring.yml`
- Create: `docker/prometheus-monitoring.yml`
- Create: `docker/loki-config.yml`
- Create: `docker/promtail-config.yml`

**Acceptance Criteria:**
- [ ] `docker compose -f docker/docker-compose.dev.yml -f docker/docker-compose.monitoring.yml config` exits 0 and emits valid YAML
- [ ] `docker compose -f docker/docker-compose.small.yml -f docker/docker-compose.monitoring.yml config` exits 0
- [ ] Merged config shows `backend` service has `GOGOMAIL_METRICS_BACKEND: prometheus` and `GOGOMAIL_METRICS_ADDR: :9091`
- [ ] Merged config shows `backend` service on both its original network and `gogomail-monitoring`

**Verify:** `docker compose -f docker/docker-compose.dev.yml -f docker/docker-compose.monitoring.yml config | grep -E "GOGOMAIL_METRICS|gogomail-monitoring"` → prints both env vars and network name

**Steps:**

- [ ] **Step 1: Create `docker/docker-compose.monitoring.yml`**

```yaml
# docker/docker-compose.monitoring.yml
# Monitoring overlay: Prometheus + Loki + Promtail + Grafana
#
# Usage:
#   docker compose -f docker/docker-compose.dev.yml   -f docker/docker-compose.monitoring.yml up -d
#   docker compose -f docker/docker-compose.small.yml -f docker/docker-compose.monitoring.yml up -d
#
# Agent API surface:
#   Prometheus:  http://localhost:9090/api/v1/query?query=<PromQL>
#   Loki:        http://localhost:3100/loki/api/v1/query_range?query=<LogQL>&start=<ns>&end=<ns>&limit=100
#   Grafana UI:  http://localhost:3000  (admin / $GRAFANA_PASSWORD)

services:
  # Extend backend: enable Prometheus metrics on :9091
  backend:
    environment:
      GOGOMAIL_METRICS_BACKEND: prometheus
      GOGOMAIL_METRICS_ADDR: ":9091"
    networks:
      - gogomail-monitoring

  # Prometheus — scrapes backend:9091/metrics every 15s
  prometheus:
    image: prom/prometheus:v2.53.1
    container_name: gogomail-prometheus
    restart: unless-stopped
    ports:
      - "${PROMETHEUS_PORT:-9090}:9090"
    volumes:
      - ./prometheus-monitoring.yml:/etc/prometheus/prometheus.yml:ro
      - prometheus-monitoring-data:/prometheus
    command:
      - --config.file=/etc/prometheus/prometheus.yml
      - --storage.tsdb.path=/prometheus
      - --storage.tsdb.retention.time=30d
      - --web.enable-lifecycle
      - --web.enable-admin-api
    networks:
      - gogomail-monitoring

  # Loki — log aggregation, filesystem storage
  loki:
    image: grafana/loki:3.1.0
    container_name: gogomail-loki
    restart: unless-stopped
    ports:
      - "${LOKI_PORT:-3100}:3100"
    volumes:
      - ./loki-config.yml:/etc/loki/local-config.yaml:ro
      - loki-monitoring-data:/loki
    command: -config.file=/etc/loki/local-config.yaml
    networks:
      - gogomail-monitoring

  # Promtail — ships Docker container logs to Loki via Docker socket
  promtail:
    image: grafana/promtail:3.1.0
    container_name: gogomail-promtail
    restart: unless-stopped
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - promtail-positions:/tmp
      - ./promtail-config.yml:/etc/promtail/config.yml:ro
    command: -config.file=/etc/promtail/config.yml
    depends_on:
      - loki
    networks:
      - gogomail-monitoring

  # Grafana — dashboards pre-provisioned with Prometheus + Loki
  grafana:
    image: grafana/grafana:11.1.0
    container_name: gogomail-grafana
    restart: unless-stopped
    ports:
      - "${GRAFANA_PORT:-3000}:3000"
    environment:
      GF_SECURITY_ADMIN_USER: admin
      GF_SECURITY_ADMIN_PASSWORD: "${GRAFANA_PASSWORD:-admin}"
      GF_USERS_ALLOW_SIGN_UP: "false"
      GF_AUTH_ANONYMOUS_ENABLED: "true"
      GF_AUTH_ANONYMOUS_ORG_ROLE: Viewer
      GF_SECURITY_ALLOW_EMBEDDING: "true"
    volumes:
      - grafana-monitoring-data:/var/lib/grafana
      - ./grafana-provisioning:/etc/grafana/provisioning:ro
    depends_on:
      - prometheus
      - loki
    networks:
      - gogomail-monitoring

volumes:
  prometheus-monitoring-data:
  loki-monitoring-data:
  grafana-monitoring-data:
  promtail-positions:

networks:
  gogomail-monitoring:
    driver: bridge
```

- [ ] **Step 2: Create `docker/prometheus-monitoring.yml`**

```yaml
# Prometheus config for monitoring overlay.
# Scrapes backend:9091/metrics — port set via GOGOMAIL_METRICS_ADDR in the overlay.

global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    deployment: gogomail

scrape_configs:
  - job_name: gogomail-backend
    static_configs:
      - targets: ["backend:9091"]
    metrics_path: /metrics
    scrape_interval: 15s
```

- [ ] **Step 3: Create `docker/loki-config.yml`**

```yaml
# Loki single-node config. Filesystem storage, 30d retention.
# Structured metadata enabled so request_id is queryable without being a label.

auth_enabled: false

server:
  http_listen_port: 3100
  grpc_listen_port: 9096

common:
  instance_addr: 127.0.0.1
  path_prefix: /loki
  storage:
    filesystem:
      chunks_directory: /loki/chunks
      rules_directory: /loki/rules
  replication_factor: 1
  ring:
    kvstore:
      store: inmemory

query_range:
  results_cache:
    cache:
      embedded_cache:
        enabled: true
        max_size_mb: 100

schema_config:
  configs:
    - from: "2024-01-01"
      store: tsdb
      object_store: filesystem
      schema: v13
      index:
        prefix: index_
        period: 24h

limits_config:
  allow_structured_metadata: true
  volume_enabled: true
  retention_period: 720h   # 30 days

compactor:
  working_directory: /loki/compactor
  retention_enabled: true
  delete_request_store: filesystem
```

- [ ] **Step 4: Create `docker/promtail-config.yml`**

```yaml
# Promtail config.
# Discovers all Docker Compose containers via Docker socket.
# Parses structured JSON logs (slog output) to extract labels:
#   service  — docker compose service name (backend, event-worker, etc.)
#   component — from JSON field (next-api, smtp, delivery, ldap, ...)
#   level    — from JSON field (INFO, WARN, ERROR)
# request_id is promoted to Loki structured metadata for efficient search
# without becoming a high-cardinality label.

server:
  http_listen_port: 9080
  grpc_listen_port: 0

positions:
  filename: /tmp/positions.yaml

clients:
  - url: http://loki:3100/loki/api/v1/push

scrape_configs:
  - job_name: docker
    docker_sd_configs:
      - host: unix:///var/run/docker.sock
        refresh_interval: 5s
        filters:
          - name: label
            values: ["com.docker.compose.project"]
    relabel_configs:
      # container label: "/gogomail-backend-dev" → "backend"
      - source_labels: ["__meta_docker_container_label_com_docker_compose_service"]
        target_label: service
      # project label
      - source_labels: ["__meta_docker_container_label_com_docker_compose_project"]
        target_label: project
      # stream (stdout / stderr)
      - source_labels: ["__meta_docker_container_log_stream"]
        target_label: stream
    pipeline_stages:
      # Parse the log line as JSON (slog JSON format).
      # Non-JSON lines (e.g. air reload messages in dev) are passed through unchanged.
      - json:
          expressions:
            component: component
            level: level
            request_id: request_id
      # Promote component and level to Loki labels (low cardinality).
      - labels:
          component:
          level:
      # Promote request_id to structured metadata (high cardinality — not a label).
      - structured_metadata:
          request_id:
```

- [ ] **Step 5: Verify compose config merges correctly**

```bash
cd /path/to/gogomail

docker compose \
  -f docker/docker-compose.dev.yml \
  -f docker/docker-compose.monitoring.yml \
  config | grep -E "GOGOMAIL_METRICS|gogomail-monitoring"
```

Expected output contains lines like:
```
GOGOMAIL_METRICS_BACKEND: prometheus
GOGOMAIL_METRICS_ADDR: :9091
gogomail-monitoring: null
```

- [ ] **Step 6: Commit**

```bash
git add docker/docker-compose.monitoring.yml \
        docker/prometheus-monitoring.yml \
        docker/loki-config.yml \
        docker/promtail-config.yml
git commit -m "feat(monitoring): add docker compose monitoring overlay

Prometheus + Loki + Promtail + Grafana overlay.
- backend extended with GOGOMAIL_METRICS_BACKEND=prometheus, GOGOMAIL_METRICS_ADDR=:9091
- Prometheus scrapes backend:9091/metrics every 15s, 30d retention
- Loki single-node filesystem, 30d retention, structured metadata on
- Promtail discovers all compose containers via Docker socket; extracts
  component/level labels and request_id structured metadata from slog JSON"
```

---

### Task 2: Grafana Provisioning (Datasources + Dashboard)

**Goal:** Grafana auto-loads Prometheus and Loki datasources and a GoGoMail overview dashboard on first start, with no manual UI setup required.

**Files:**
- Create: `docker/grafana-provisioning/datasources/datasources.yml`
- Create: `docker/grafana-provisioning/dashboards/provider.yml`
- Create: `docker/grafana-provisioning/dashboards/gogomail-overview.json`
- Delete: `docker/grafana-provisioning/datasources.yml` (root-level — never loaded by Grafana)
- Delete: `docker/grafana-provisioning/dashboard-provider.yml` (root-level — never loaded by Grafana)

**Acceptance Criteria:**
- [ ] `docker compose -f docker/docker-compose.dev.yml -f docker/docker-compose.monitoring.yml up -d` starts all 5 monitoring services
- [ ] `curl -s http://admin:admin@localhost:3000/api/datasources | jq '.[].name'` returns `"Prometheus"` and `"Loki"`
- [ ] `curl -s http://admin:admin@localhost:3000/api/datasources/name/Prometheus/health` returns `{"status":"OK",...}`
- [ ] `curl -s http://admin:admin@localhost:3000/api/datasources/name/Loki/health` returns `{"status":"OK",...}`
- [ ] `curl -s http://admin:admin@localhost:3000/api/dashboards/uid/gogomail-overview | jq '.dashboard.title'` returns `"GoGoMail Overview"`

**Verify:** (run after `docker compose up -d` and 30s wait)
```bash
curl -s http://admin:admin@localhost:3000/api/datasources | jq '.[].name'
```
→ `"Prometheus"` and `"Loki"` on separate lines

**Steps:**

- [ ] **Step 1: Create directory structure**

```bash
mkdir -p docker/grafana-provisioning/datasources
mkdir -p docker/grafana-provisioning/dashboards
```

- [ ] **Step 2: Remove old root-level files**

```bash
git rm docker/grafana-provisioning/datasources.yml
git rm docker/grafana-provisioning/dashboard-provider.yml
```

- [ ] **Step 3: Create `docker/grafana-provisioning/datasources/datasources.yml`**

UIDs are hardcoded (`prometheus`, `loki`) so the dashboard JSON can reference them reliably.

```yaml
apiVersion: 1

datasources:
  - name: Prometheus
    uid: prometheus
    type: prometheus
    access: proxy
    orgId: 1
    url: http://prometheus:9090
    isDefault: true
    jsonData:
      timeInterval: 15s
      httpMethod: POST
    editable: true

  - name: Loki
    uid: loki
    type: loki
    access: proxy
    orgId: 1
    url: http://loki:3100
    isDefault: false
    jsonData:
      maxLines: 1000
      derivedFields:
        - name: TraceID
          matcherRegex: '"request_id":"([^"]+)"'
          url: '/explore?orgId=1&left={"datasource":"loki","queries":[{"expr":"{service=~\".+\"} |= \"${__value.raw}\"","refId":"A"}],"range":{"from":"now-1h","to":"now"}}'
          datasourceUid: loki
    editable: true
```

- [ ] **Step 4: Create `docker/grafana-provisioning/dashboards/provider.yml`**

```yaml
apiVersion: 1

providers:
  - name: gogomail
    orgId: 1
    folder: GoGoMail
    type: file
    disableDeletion: false
    updateIntervalSeconds: 30
    allowUiUpdates: true
    options:
      path: /etc/grafana/provisioning/dashboards
      foldersFromFilesStructure: false
```

- [ ] **Step 5: Create `docker/grafana-provisioning/dashboards/gogomail-overview.json`**

```json
{
  "title": "GoGoMail Overview",
  "uid": "gogomail-overview",
  "schemaVersion": 39,
  "version": 1,
  "refresh": "30s",
  "time": { "from": "now-1h", "to": "now" },
  "timepicker": {},
  "panels": [
    {
      "id": 1,
      "type": "timeseries",
      "title": "HTTP Request Rate (req/s)",
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 0 },
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "targets": [
        {
          "refId": "A",
          "expr": "sum(rate(gogomail_http_request_duration_seconds_count[5m])) by (route)",
          "legendFormat": "{{route}}"
        }
      ],
      "fieldConfig": {
        "defaults": { "unit": "reqps", "color": { "mode": "palette-classic" } },
        "overrides": []
      },
      "options": { "tooltip": { "mode": "multi" } }
    },
    {
      "id": 2,
      "type": "timeseries",
      "title": "HTTP p95 Latency (s)",
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 0 },
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "targets": [
        {
          "refId": "A",
          "expr": "histogram_quantile(0.95, sum(rate(gogomail_http_request_duration_seconds_bucket[5m])) by (le, route))",
          "legendFormat": "p95 {{route}}"
        }
      ],
      "fieldConfig": {
        "defaults": { "unit": "s", "color": { "mode": "palette-classic" } },
        "overrides": []
      },
      "options": { "tooltip": { "mode": "multi" } }
    },
    {
      "id": 3,
      "type": "timeseries",
      "title": "HTTP Error Rate (5xx, req/s)",
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 8 },
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "targets": [
        {
          "refId": "A",
          "expr": "sum(rate(gogomail_http_request_duration_seconds_count{status=~\"5..\"}[5m])) by (route)",
          "legendFormat": "5xx {{route}}"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "reqps",
          "color": { "mode": "fixed", "fixedColor": "red" }
        },
        "overrides": []
      },
      "options": { "tooltip": { "mode": "multi" } }
    },
    {
      "id": 4,
      "type": "timeseries",
      "title": "SMTP Events (per/s)",
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 8 },
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "targets": [
        {
          "refId": "A",
          "expr": "sum(rate(gogomail_smtp_events_total[5m])) by (stage, result)",
          "legendFormat": "{{stage}}/{{result}}"
        }
      ],
      "fieldConfig": {
        "defaults": { "unit": "ops", "color": { "mode": "palette-classic" } },
        "overrides": []
      },
      "options": { "tooltip": { "mode": "multi" } }
    },
    {
      "id": 5,
      "type": "timeseries",
      "title": "Delivery Events (per/s)",
      "gridPos": { "h": 8, "w": 12, "x": 0, "y": 16 },
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "targets": [
        {
          "refId": "A",
          "expr": "sum(rate(gogomail_delivery_events_total[5m])) by (stage, result)",
          "legendFormat": "{{stage}}/{{result}}"
        }
      ],
      "fieldConfig": {
        "defaults": { "unit": "ops", "color": { "mode": "palette-classic" } },
        "overrides": []
      },
      "options": { "tooltip": { "mode": "multi" } }
    },
    {
      "id": 6,
      "type": "timeseries",
      "title": "LDAP Events (per/s)",
      "gridPos": { "h": 8, "w": 12, "x": 12, "y": 16 },
      "datasource": { "type": "prometheus", "uid": "prometheus" },
      "targets": [
        {
          "refId": "A",
          "expr": "sum(rate(gogomail_ldap_events_total[5m])) by (operation, result)",
          "legendFormat": "{{operation}}/{{result}}"
        }
      ],
      "fieldConfig": {
        "defaults": { "unit": "ops", "color": { "mode": "palette-classic" } },
        "overrides": []
      },
      "options": { "tooltip": { "mode": "multi" } }
    },
    {
      "id": 7,
      "type": "logs",
      "title": "Backend Logs",
      "gridPos": { "h": 12, "w": 24, "x": 0, "y": 24 },
      "datasource": { "type": "loki", "uid": "loki" },
      "targets": [
        {
          "refId": "A",
          "expr": "{service=\"backend\"} | json",
          "legendFormat": ""
        }
      ],
      "options": {
        "showTime": true,
        "showLabels": true,
        "showCommonLabels": false,
        "wrapLogMessage": false,
        "prettifyLogMessage": false,
        "enableLogDetails": true,
        "dedupStrategy": "none",
        "sortOrder": "Descending"
      }
    }
  ]
}
```

- [ ] **Step 6: Verify Grafana loads provisioning**

Start the stack:
```bash
docker compose \
  -f docker/docker-compose.dev.yml \
  -f docker/docker-compose.monitoring.yml \
  up -d

# Wait ~30s for Grafana to initialize
sleep 30
```

Check datasources:
```bash
curl -s http://admin:admin@localhost:3000/api/datasources | jq '.[].name'
```
Expected:
```
"Prometheus"
"Loki"
```

Check datasource health:
```bash
curl -s http://admin:admin@localhost:3000/api/datasources/name/Prometheus/health | jq .
curl -s http://admin:admin@localhost:3000/api/datasources/name/Loki/health | jq .
```
Both should return `{"status":"OK",...}`. If Loki returns 503, wait another 30s for Loki to finish startup.

Check dashboard:
```bash
curl -s http://admin:admin@localhost:3000/api/dashboards/uid/gogomail-overview | jq '.dashboard.title'
```
Expected: `"GoGoMail Overview"`

- [ ] **Step 7: Verify metrics are flowing (requires backend to be running with metrics enabled)**

```bash
# Should return Prometheus text format with gogomail_* metrics
curl -s http://localhost:9090/api/v1/targets | jq '.data.activeTargets[0].health'
```
Expected: `"up"`

```bash
# Instant query — request rate (may be 0 if no traffic yet, but must not error)
curl -s "http://localhost:9090/api/v1/query?query=gogomail_http_request_duration_seconds_count" | jq '.status'
```
Expected: `"success"`

- [ ] **Step 8: Commit**

```bash
git add docker/grafana-provisioning/
git commit -m "feat(monitoring): add Grafana provisioning (Prometheus + Loki datasources, overview dashboard)

- Move datasources and dashboard provider to correct subdirectories
  (root-level files were silently ignored by Grafana)
- Add Loki datasource with derived field linking request_id to log explorer
- Add GoGoMail overview dashboard: HTTP rate/latency/errors, SMTP,
  delivery, LDAP metric panels + live backend log panel
- Datasource UIDs hardcoded (prometheus, loki) for stable dashboard refs"
```

---

### Task 3: Agent Query Reference (MONITORING.md)

**Goal:** A single reference document that lets an AI agent query the monitoring stack without guessing — covers startup, all key PromQL and LogQL patterns, and the HTTP API call format.

**Files:**
- Create: `docs/MONITORING.md`

**Acceptance Criteria:**
- [ ] File exists at `docs/MONITORING.md`
- [ ] Contains working Prometheus HTTP API query example with correct `curl` syntax
- [ ] Contains working Loki HTTP API query example with correct `curl` syntax
- [ ] Covers `request_id` trace query (the primary agent debugging pattern)
- [ ] Covers all four metric families (`http`, `smtp`, `delivery`, `ldap`)

**Verify:** `test -f docs/MONITORING.md && grep -q "request_id" docs/MONITORING.md && grep -q "loki/api/v1" docs/MONITORING.md && echo OK`
→ `OK`

**Steps:**

- [ ] **Step 1: Create `docs/MONITORING.md`**

```markdown
# GoGoMail Monitoring

Prometheus + Loki + Grafana overlay for gogomail Docker deployments.

## Starting the stack

```bash
# Development
docker compose -f docker/docker-compose.dev.yml -f docker/docker-compose.monitoring.yml up -d

# Small / production
docker compose -f docker/docker-compose.small.yml -f docker/docker-compose.monitoring.yml up -d
```

Service ports (all localhost):

| Service    | Port | Purpose                       |
|------------|------|-------------------------------|
| Prometheus | 9090 | Metrics query API             |
| Loki       | 3100 | Log query API                 |
| Grafana    | 3000 | Dashboards UI + HTTP API      |

---

## Prometheus: Metrics API

Base URL: `http://localhost:9090/api/v1`

### Instant query

```bash
curl -s "http://localhost:9090/api/v1/query?query=<PromQL>" | jq .
```

### Range query

```bash
curl -s "http://localhost:9090/api/v1/query_range" \
  --data-urlencode "query=<PromQL>" \
  --data-urlencode "start=$(date -v-1H +%s)" \
  --data-urlencode "end=$(date +%s)" \
  --data-urlencode "step=30" | jq .
```

### Key PromQL expressions

```promql
# HTTP request rate (req/s) by route
sum(rate(gogomail_http_request_duration_seconds_count[5m])) by (route)

# HTTP 5xx error rate by route
sum(rate(gogomail_http_request_duration_seconds_count{status=~"5.."}[5m])) by (route)

# HTTP p50/p95/p99 latency by route
histogram_quantile(0.95, sum(rate(gogomail_http_request_duration_seconds_bucket[5m])) by (le, route))

# SMTP event rate by stage and result
sum(rate(gogomail_smtp_events_total[5m])) by (stage, result)

# SMTP session duration p95
histogram_quantile(0.95, sum(rate(gogomail_smtp_session_duration_seconds_bucket[5m])) by (le))

# Delivery event rate by stage and result
sum(rate(gogomail_delivery_events_total[5m])) by (stage, result)

# LDAP event rate by operation and result
sum(rate(gogomail_ldap_events_total[5m])) by (operation, result)
```

### Metric labels reference

| Metric | Labels |
|--------|--------|
| `gogomail_http_request_duration_seconds` | `method`, `route`, `status` |
| `gogomail_smtp_events_total` | `stage`, `result` |
| `gogomail_smtp_session_duration_seconds` | _(none)_ |
| `gogomail_smtp_rfc_noncompliance_total` | `rfc5322`, `rfc5321` |
| `gogomail_delivery_events_total` | `stage`, `result`, `farm`, `route_pool`, `recipient_bucket` |
| `gogomail_ldap_events_total` | `operation`, `result` |

---

## Loki: Log Query API

Base URL: `http://localhost:3100/loki/api/v1`

Timestamps are Unix nanoseconds. Use `$(date +%s)000000000` for current time.

### Instant log query

```bash
curl -s -G "http://localhost:3100/loki/api/v1/query" \
  --data-urlencode 'query={service="backend"} | json' \
  --data-urlencode "limit=50" | jq .
```

### Range log query

```bash
START=$(python3 -c "import time; print(int((time.time()-3600)*1e9))")
END=$(python3 -c "import time; print(int(time.time()*1e9))")

curl -s -G "http://localhost:3100/loki/api/v1/query_range" \
  --data-urlencode 'query={service="backend"} | json | level="ERROR"' \
  --data-urlencode "start=${START}" \
  --data-urlencode "end=${END}" \
  --data-urlencode "limit=100" | jq .
```

### Key LogQL expressions

```logql
# All backend logs, JSON parsed
{service="backend"} | json

# Errors only
{service="backend"} | json | level="ERROR"

# Trace a specific request end-to-end (primary debugging pattern)
{service=~".+"} | json | request_id="<paste-request-id-here>"

# HTTP access logs only
{service="backend", component="next-api"} | json

# SMTP component logs
{service="backend", component="smtp"} | json

# Delivery worker logs
{service="backend", component="delivery"} | json

# Errors in the last hour, with message field
{service="backend"} | json | level="ERROR" | line_format "{{.msg}}"

# Filter by status code in HTTP access logs
{service="backend"} | json | status >= 500

# Count error rate over time
sum(count_over_time({service="backend"} | json | level="ERROR" [5m]))
```

### Label reference

| Label | Values | Source |
|-------|--------|--------|
| `service` | `backend`, `event-worker`, `outbox-relay`, `delivery-worker` | Docker Compose service name |
| `component` | `next-api`, `smtp`, `delivery`, `ldap`, `imap`, `pop3` | JSON field from slog |
| `level` | `INFO`, `WARN`, `ERROR`, `DEBUG` | JSON field from slog |
| `project` | Compose project name | Docker Compose label |
| `stream` | `stdout`, `stderr` | Container stream |

### Structured metadata (request_id)

`request_id` is stored as Loki structured metadata (not a label). Query via JSON filter:

```logql
{service=~".+"} | json | request_id="req-abc123"
```

This is the primary end-to-end trace pattern: one `request_id` correlates
the Next.js proxy log, the Go HTTP access log, and all downstream component logs.

---

## Grafana API

Base URL: `http://localhost:3000`
Auth: `admin` / `$GRAFANA_PASSWORD` (default: `admin`)

```bash
# List datasources
curl -s http://admin:admin@localhost:3000/api/datasources | jq '.[].name'

# Check datasource health
curl -s http://admin:admin@localhost:3000/api/datasources/name/Prometheus/health | jq .

# List dashboards
curl -s http://admin:admin@localhost:3000/api/search | jq '.[] | {uid, title}'

# Get GoGoMail overview dashboard
curl -s http://admin:admin@localhost:3000/api/dashboards/uid/gogomail-overview | jq '.dashboard.title'
```

---

## Stopping the stack

```bash
docker compose \
  -f docker/docker-compose.dev.yml \
  -f docker/docker-compose.monitoring.yml \
  down

# To also remove volumes (wipes all metrics/log history):
docker compose \
  -f docker/docker-compose.dev.yml \
  -f docker/docker-compose.monitoring.yml \
  down -v
```
```

- [ ] **Step 2: Verify file**

```bash
test -f docs/MONITORING.md \
  && grep -q "request_id" docs/MONITORING.md \
  && grep -q "loki/api/v1" docs/MONITORING.md \
  && grep -q "9090" docs/MONITORING.md \
  && echo OK
```
Expected: `OK`

- [ ] **Step 3: Commit**

```bash
git add docs/MONITORING.md
git commit -m "docs: add MONITORING.md — agent query reference for Prometheus + Loki

Full HTTP API examples for PromQL and LogQL, label reference,
request_id trace pattern, Grafana API cheatsheet, and stack
start/stop commands."
```
