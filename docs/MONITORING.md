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
# Sent as POST form-encoded body — Prometheus accepts both GET and POST
curl -s "http://localhost:9090/api/v1/query_range" \
  --data-urlencode "query=<PromQL>" \
  --data-urlencode "start=$(date -v-1H +%s 2>/dev/null || date -d '1 hour ago' +%s)" \
  --data-urlencode "end=$(date +%s)" \
  --data-urlencode "step=30s" | jq .
```

### Key PromQL expressions

```promql
# HTTP request rate (req/s) by route
sum(rate(gogomail_http_request_duration_seconds_count[5m])) by (route)

# HTTP 5xx error rate by route
sum(rate(gogomail_http_request_duration_seconds_count{status=~"5.."}[5m])) by (route)

# HTTP p95 latency by route
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
| `gogomail_smtp_session_duration_seconds` | _(histogram, no extra labels)_ |
| `gogomail_smtp_rfc_noncompliance_total` | `rfc5322`, `rfc5321` |
| `gogomail_delivery_events_total` | `stage`, `result`, `farm`, `route_pool`, `recipient_bucket` |
| `gogomail_ldap_events_total` | `operation`, `result` |

---

## Loki: Log Query API

Base URL: `http://localhost:3100/loki/api/v1`

Timestamps are Unix **nanoseconds**. Use Python for cross-platform precision:

```bash
START=$(python3 -c "import time; print(int((time.time()-3600)*1e9))")
END=$(python3 -c "import time; print(int(time.time()*1e9))")
```

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

# Cleanup/rollback delete failures that can leave orphaned storage objects
{service="backend"} | json | level="WARN" |~ "cleanup|rollback|delete"

# Drive cleanup failure records are surfaced through admin APIs; pair those
# records with warning logs from the same time window.
{service="backend"} | json | level="WARN" |~ "drive.*cleanup|cleanup.*drive"

# SCIM external IdP status synchronization failures
{service="backend"} | json | level="WARN" |~ "SCIM|UpdateUserStatus"

# Fail-open API metering sink failures
{service="backend"} | json | level="WARN" |~ "metering|api usage"

# Remote API usage signer lifecycle and request failures
{service="remote-signer"} | json

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

`request_id` is stored as Loki structured metadata — not a label — so it doesn't pollute
the label index. Query via JSON filter:

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

# Check datasource health (uid-based endpoint, available in Grafana 9.4+)
curl -s http://admin:admin@localhost:3000/api/datasources/uid/prometheus/health | jq .
curl -s http://admin:admin@localhost:3000/api/datasources/uid/loki/health | jq .

# List dashboards
curl -s http://admin:admin@localhost:3000/api/search | jq '.[] | {uid, title}'

# Get GoGoMail overview dashboard JSON
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
