# Super-Powerful SMTP Optimization Roadmap

## Goal
Build an end-to-end SMTP system with:
- 🚀 Monster performance & scalability (handle massive mail traffic)
- 🔒 Extreme stability (bulk mail doesn't impact regular users)
- ✅ RFC strict compliance (5322, 5321, 5891, 3461, etc.)
- 🔄 Mode-based multiplexing (inbound, outbound, delivery in single instance)
- 🌍 Server farm capability (horizontal scaling, state sharing)

---

## Current State (Audit)

### Existing Architecture ✓
- **inbound-mta** (receive): Accepts SMTP from remote MTAs
- **outbound-mta** (submission): Client submission + relay
- **delivery-worker**: Sends outbound mail to recipient domains
- **event-worker**: Async event processing (indexing, push, audit)

### Current Config & Settings
```
SMTPMaxConnections:      0 (unlimited) ⚠️
SMTPMaxRecipients:       100
SMTPMaxMessageBytes:     25MB (25*1024*1024)
SMTPReadTimeout:         30s
SMTPWriteTimeout:        30s
```

### Implemented Safety Features ✓
- Backpressure control (pressure.go)
- Deduplication (Redis SET NX)
- Rate limiting (per-domain, per-IP)
- Authentication/authorization
- SMTP authentication header insertion
- DSN (Delivery Status Notification)
- DKIM signing support
- Spam filter (Milter protocol)

---

## Gaps & Issues

### 1. Connection Pool Management
**Issue**: SMTPMaxConnections=0 (unlimited) can lead to resource exhaustion
**Impact**: Under high load, OS can hit file descriptor limits
**Fix**: 
- [ ] Set sensible defaults: 10,000 for inbound-mta, 5,000 for submission
- [ ] Add connection state tracking (active, idle, draining)
- [ ] Implement graceful connection draining on shutdown
- [ ] Add connection pool metrics (current, max, average lifetime)

### 2. Backpressure & Load Shedding
**Issue**: No per-recipient-domain rate limiting; bulk senders can trigger slow/failed recipients affecting all users
**Impact**: One problematic domain blocks the entire delivery queue
**Fix**:
- [ ] Per-recipient-domain delivery isolation (separate queue per domain)
- [ ] Domain-scoped retry policy (exponential backoff per domain)
- [ ] Circuit breaker pattern (fail-fast for consistently failing domains)
- [ ] Reject new submissions if delivery queue is too deep
- [ ] Add delivery metrics per domain (success rate, avg latency, retry depth)

### 3. Bulk Mail Isolation
**Issue**: Bulk senders compete with regular users for resources
**Impact**: Bulk mail delays regular user sends/receives
**Fix**:
- [ ] Separate bulk submission queue (e.g., `bulk-submission` mode)
- [ ] Bulk queue has lower CPU/IO priority (nice, cgroup limits)
- [ ] Bulk senders authenticated as bulk_user role (rate limited to 100 msg/sec)
- [ ] Regular users (default role) get priority scheduling

### 4. Delivery Concurrency Control
**Issue**: No limit on concurrent delivery attempts per domain
**Impact**: Hammer remote servers; get rate-limited or blocked
**Fix**:
- [ ] Per-domain concurrency limit (default 10, configurable)
- [ ] Global delivery concurrency pool (e.g., 1000 total)
- [ ] Adaptive rate limiting (reduce concurrency if remote server 4xx)
- [ ] Per-domain backoff tracking (via outbox `available_at`)

### 5. Memory & GC Pressure
**Issue**: Processing 25MB messages at scale creates GC pressure
**Impact**: Latency spikes during GC; throughput drops
**Fix**:
- [ ] Stream-based message processing (avoid loading full .eml into memory)
- [ ] Implement chunked reading for large attachments
- [ ] Add GC metrics (pause time, frequency)
- [ ] Tune GOGC for mail workers (e.g., GOGC=75)

### 6. SMTP Timeouts & Resilience
**Issue**: Fixed 30s timeout doesn't account for slow networks/servers
**Impact**: Legitimate slow servers get disconnected; lost mail
**Fix**:
- [ ] Adaptive timeout based on domain reputation
- [ ] Separate idle timeout (300s) from command timeout (30s)
- [ ] Add slow client detection (disconnect if < 1KB/s for 10s)
- [ ] Add metrics: timeout_count, slow_client_count per inbound-mta

### 7. Mode Multiplexing in Single Instance
**Issue**: Currently modes must be separate binaries (poor dev/test experience)
**Impact**: Hard to run all-in-one for testing; need multiple processes
**Fix**:
- [ ] Add `--mode=multi` that runs inbound-mta + submission + delivery concurrently
- [ ] Graceful shutdown of all modes together
- [ ] Shared metrics/logging across modes
- [ ] Use goroutine pools, not separate OS processes

### 8. Server Farm / Horizontal Scaling
**Issue**: No built-in coordination for multiple instances
**Impact**: Can't scale horizontally; no failover
**Fix**:
- [ ] Redis-based delivery queue coordination (heartbeat, ownership tracking)
- [ ] Session replication for submission auth (Redis backend option)
- [ ] Distributed rate limiter (via Redis)
- [ ] Health check endpoint (`GET /health`)
- [ ] Graceful drain + rejoin for rolling updates

### 9. Monitoring & Observability
**Issue**: No end-to-end latency metrics; audit logs don't capture SMTP timeline
**Impact**: Can't diagnose performance bottlenecks
**Fix**:
- [ ] Add SMTP phase timestamps to audit logs (received_at → parsed_at → stored_at → delivered_at)
- [ ] Track end-to-end mail latency (submission → delivery)
- [ ] Add metrics for SMTP command latency (RCPT, DATA, etc.)
- [ ] Implement tracing (jaeger/otlp) for delivery chain
- [ ] Add dashboard for real-time mail flow (msgs/sec, avg latency, queue depth)

### 10. RFC Compliance Verification
**Issue**: No systematic testing of all RFC requirements
**Impact**: Edge cases cause interop failures
**Fix**:
- [ ] Add RFC 5322 header parsing tests (all edge cases)
- [ ] RFC 5321 SMTP command tests (pipelining, timeout, quit)
- [ ] RFC 3461 DSN tests (NOTIFY, RET, ORCPT)
- [ ] RFC 6376 DKIM tests (header canonicalization, signature validation)
- [ ] RFC 5891 SMTPUTF8 tests (international domains/addresses)

---

## Implementation Phases

### Phase 1: Connection & Concurrency Control (Critical)
**Goal**: Prevent resource exhaustion under load
- Set sensible connection limits
- Add connection pool metrics
- Implement per-domain delivery concurrency limits
- Circuit breaker for failing domains

**Files**:
- config/config.go - add SMTPConnectionLimit, DeliveryDomainConcurrency
- internal/smtp/server.go - add connection tracking
- internal/delivery/worker.go - per-domain queue + concurrency control
- metrics - connection/delivery queue metrics

### Phase 2: Bulk Mail Isolation (High Impact)
**Goal**: Bulk senders don't affect regular users
- Separate bulk-submission mode
- Role-based rate limiting (bulk_user vs regular)
- Priority scheduling

**Files**:
- config/config.go - add BulkMode, BulkRateLimit
- internal/smtp/submission.go - role-based routing
- internal/maildb/user.go - bulk_user role support

### Phase 3: Memory & GC Optimization (Performance)
**Goal**: Handle 25MB messages without GC pauses
- Stream-based processing
- Chunked reading
- GC tuning

**Files**:
- internal/smtp/receiver.go - stream DATA reading
- internal/storage/ - chunked upload
- Dockerfile / startup script - GOGC tuning

### Phase 4: Mode Multiplexing (Dev Experience)
**Goal**: Run multiple modes in single instance
- --mode=multi option
- Coordinated shutdown
- Shared logging

**Files**:
- cmd/gogomail/main.go - multi-mode support
- internal/app/run.go - refactor mode functions

### Phase 5: Server Farm & HA (Scalability)
**Goal**: Horizontal scaling + failover
- Redis coordination
- Distributed rate limiting
- Health checks
- Graceful drain

**Files**:
- internal/delivery/ - Redis-based queue coordination
- internal/smtp/ - health check handler
- deployment/ - Kubernetes manifests with readiness probes

### Phase 6: Observability (Debugging)
**Goal**: Diagnose performance issues
- End-to-end latency metrics
- SMTP phase timestamps in audit logs
- Tracing support
- Real-time dashboard

**Files**:
- internal/audit/ - add SMTP phase times
- internal/smtp/ - command latency metrics
- docs/MONITORING.md - setup guide

### Phase 7: RFC Compliance Hardening (Quality)
**Goal**: Pass all RFC compliance tests
- Systematic RFC testing
- Edge case handling
- Interop verification

**Files**:
- internal/smtp/*_test.go - RFC test suite expansion
- Makefile - RFC compliance check target

---

## Success Criteria

### Performance Targets
- [ ] Handle 100,000 msg/day (1+ msg/sec sustained)
- [ ] Inbound latency: < 2s from MAIL FROM → stored
- [ ] Delivery latency: < 60s median (p99: < 300s)
- [ ] No GC pause > 100ms under sustained load

### Stability Targets
- [ ] 99.99% uptime (1 outage/month max)
- [ ] No mail loss (zero data corruption)
- [ ] Bulk mail queueing doesn't block regular user ops

### Scaling Targets
- [ ] Single instance: 10K concurrent SMTP connections
- [ ] Server farm: Linear scaling up to 10 instances
- [ ] Redis as sole distributed coordination point

---

## Next Steps

1. **Immediately** (blocking super-powerful SMTP):
   - [ ] Phase 1: Set SMTPMaxConnections default to 10,000
   - [ ] Implement per-domain delivery concurrency limits
   - [ ] Add circuit breaker for failing domains

2. **This sprint**:
   - [ ] Phase 2: Bulk mail isolation mode
   - [ ] Phase 3: Memory optimization (chunked reading)

3. **Next sprint**:
   - [ ] Phase 4: Mode multiplexing
   - [ ] Phase 5: Server farm basics (Redis queue coordination)

4. **Ongoing**:
   - [ ] Phase 6: Observability (continuous)
   - [ ] Phase 7: RFC compliance (continuous)
