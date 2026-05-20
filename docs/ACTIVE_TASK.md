# ACTIVE_TASK

## Current Status Summary

**SaaS Pre-Launch Security & Integrity Hardening** ✅ COMPLETE
- Security (Critical/High): SAML XML signature verification, OIDC RS256/JWKS verification,
  removed hardcoded admin credentials, company_admin multi-tenant isolation enforced,
  admin mock APIs replaced with real DB, login rate limiting, IMAP brute-force protection,
  user enumeration timing fix, SCIM constant-time compare, MFA VerifyFull, legacy hash block
- Business logic: vacation body/subject field unification, draft-to-send atomicity,
  MaxUsers/MaxDomains limit enforcement at creation endpoints
- Infrastructure: production validation for JWT secret/admin token/DB sslmode,
  Redis password support, Docker pg-hba.conf created, Redis requirepass in compose
- Data integrity: batchlock panic recovery + lock release, configstore audit log errors,
  createSystemFolders error propagation, S3 large-upload timeout fix
- Frontend: ErrorBoundary generic message, filter rules "not implemented" banner,
  console pages fetch error UI

**Push Notify & Webhook Hardening** ✅ COMPLETE
- APNs: ES256 JWT (provider token auth) with 45-min caching — replaces raw bearer token
- WebPush: RFC 8292 VAPID headers (vapid t=<jwt>,k=<pubkey>) — replaces missing auth
- WebhookDispatcher: company config-backed dispatcher for mail.received/sent/bounced
  with HMAC-SHA256 signatures, SSRF guard, best-effort delivery
- Config vars: GOGOMAIL_APNS_KEY_ID, GOGOMAIL_APNS_TEAM_ID, GOGOMAIL_APNS_PRIVATE_KEY,
  GOGOMAIL_APNS_BUNDLE_ID, GOGOMAIL_WEBPUSH_VAPID_PUBLIC_KEY,
  GOGOMAIL_WEBPUSH_VAPID_PRIVATE_KEY, GOGOMAIL_WEBPUSH_CONTACT_EMAIL,
  GOGOMAIL_WEBHOOK_DISPATCH_ENABLED (default: true)
- Fixed pre-existing test breakage in auth/password_test.go and maildb/admin_contract_test.go
  caused by allowLegacy=false enforcement; updated to allowLegacy=true for admin APIs

**Console Admin MFA Implementation** 🔄 IN PROGRESS
- Task 1 ✅ DONE: Added `AdminMFARequired` config field (env: `GOGOMAIL_ADMIN_MFA_REQUIRED`)
- Tasks 2-9: Frontend/backend MFA enrollment, verification, CLI break-glass, security settings UI

**TASK-089: Protocol Gateway Hardening** ✅ COMPLETE (5987 tests)
- All 3 phases implemented and verified
- Buffer pooling, metrics export, health checks, graceful degradation
- Ready for production deployment

**TASK-091: Runtime Config Store Hardening** ✅ COMPLETE
- Fixed `Resolve()` to respect `locked` flag: locked entry at any ancestor stops the walk
- Fixed `Resolve()` to walk the company parent chain (root → ... → company → domain → user)
- Fixed `listenNotifications()`: was calling `pg_notify` in a loop (sending, not receiving); replaced with `pgx.Conn.WaitForNotification()` via `sql.Conn.Raw()`
- Fixed `collectSubtree()`: removed duplicate append that caused repeated entries
- Fixed `Propagate()`: `PropagateDomains` now queries actual domain IDs under the company; `PropagateSubtree` now includes domains of all subtree companies
- Added `parentOf map[string]string` field for O(depth) company chain resolution
- `loadCompanyTree()` now builds both `companyTree` (parent→children) and `parentOf` (child→parent) in one pass
- `Set()` and `Delete()` now write to `config_change_log` audit table
- 16 unit tests covering locked resolution, tree traversal, cycle protection, fan-out, concurrency

**TASK-090: Message Storage & Delivery Optimization** 🔄 IN PROGRESS
- Phase 1 (Database Query Optimization): Partial indexes, UUID array hydration, and batch lookup improvements implemented
- Phase 2 (Bulk Delivery Batching): Same-domain batch planning, runtime batch-size tuning, observability, and benchmarks implemented
- Phase 3 (Message Caching Layer): Parsed EML body LRU cache with runtime tuning and cache snapshots implemented

**Infrastructure & Storage Hardening** ✅ COMPLETE
- Task 1 (EML GC): Added `LookupDeleteableStoragePaths` and `LookupExpungeStoragePaths` to maildb; service layer now performs two-phase GC (lookup before DB delete, delete from store after commit) for `DeleteMessage`, `BulkDeleteMessages`, `BulkDeleteThreads`, and `ExpungeIMAPMessages`. Reference-count check prevents deletion of paths shared by IMAP COPY.
- Task 2 (Drive quota): Added `drive_nodes` (active files) to both quota reconciliation CTEs (`ListQuotaReconciliation` and `quotaActualCTE`).
- Task 3 (CORS): Added `CORSMiddleware` to `httpapi/admin.go`; wired via `GOGOMAIL_CORS_ALLOWED_ORIGINS` env var; handles OPTIONS preflight; added `cors_allowed_origins` config file key.
- Task 4 (CSP): Added `Content-Security-Policy` header to `SecurityHeadersMiddleware`.
- Task 5 (Prometheus /metrics): Added `ObserveLDAP` + `ObserveRFCNonCompliance` to `PrometheusAdapter`; wired as "prometheus" backend in `smtpMetrics`/`deliveryMetrics`/`ldapMetrics`; `serveMetrics` goroutine exposes `/metrics` on `GOGOMAIL_METRICS_ADDR` (default :9090) when `GOGOMAIL_METRICS_BACKEND=prometheus`.

---

## TASK-090: Message Storage & Delivery Optimization (Bulk Mail Performance)

### 목표

대량 메일 발신 환경에서의 데이터베이스 성능, 메시지 저장 효율성, 대량 발신 배치 처리 최적화.
`internal/maildb`의 메시지 쿼리, `internal/delivery`의 배치 처리, 메시지 스토리지 서브시스템의 성능 개선.

현재 문제점:
- 대량 발신 시 메시지 메타데이터 조회 성능 저하
- 배치 처리 없이 개별 메시지 처리로 인한 데이터베이스 왕복 증가
- 자주 접근하는 메시지 데이터 캐싱 미구현

### 구현 대상

Go Backend (`internal/`):
- 메시지 조회 쿼리 인덱싱 검증: 수신자, 상태, 발송 시간 기반 조회 최적화
- 배치 메시지 처리: 대량 발신 시 N+1 쿼리 제거
- 메시지 캐싱: Redis 또는 메모리 기반 LRU 캐시 (옵션)
- 대량 메일 벤치마크: 1000+, 10000+, 100000+ 메시지 발신 성능 측정
- 지연된 재시도 최적화: 배치 스케줄링 개선

### 단계별 계획

**Phase 1: Database Query Optimization (진행 중 / 일부 구현)**
- 메시지 조회 쿼리 분석: EXPLAIN ANALYZE로 현재 성능 조사
- 누락된 인덱스 추가: delivery_state, scheduled_at, recipient_count 기준 인덱스
- 배치 조회 함수 최적화: ListOutboundMessages(), GetMessagesByID()
- 벤치마크: 단일 쿼리 vs 배치 조회 성능 비교
- 목표: 대량 조회 시 쿼리 개수 50% 감소

**Phase 2: Bulk Delivery Batching (진행 중 / 일부 구현)**
- 멀티 수신자 메시지 배치 처리: 같은 도메인 수신자 묶음 발송
- 배치 크기 튜닝: 메모리/성능 트레이드오프
- 벤치마크: 배치 vs 개별 발송 처리량 비교
- 목표: 대량 발신 처리량 2배 이상 향상

**Phase 3: Message Caching Layer (진행 중 / 일부 구현)**
- 자주 접근하는 메시지 메타데이터 캐싱
- Redis 기반 캐시 (선택사항) 또는 메모리 LRU 캐시
- 캐시 무효화 전략: 메시지 상태 변경 시 자동 무효화
- 목표: 메시지 조회 레이턴시 30-50% 감소

### 진행 상황

**TASK-090 진행 중: Message Storage & Delivery Optimization**

구현 대상:
- [ ] EXPLAIN ANALYZE로 메시지 조회 쿼리 성능 분석
- [ ] 누락된 인덱스 생성 (delivery_state, scheduled_at, recipient_count)
- [ ] ListOutboundMessages 최적화 (N+1 제거)
- [ ] GetMessagesByID 배치 조회 함수 작성
- [ ] 벤치마크 프레임워크 (메시지 1000+, 10000+ 시나리오)
- [ ] 테스트 검증: go test ./... 통과

최근 진행:
- `ListMessagesByIDs` hydration을 `unnest($2::uuid[]) WITH ORDINALITY` 기반으로 바꿔 JSON 배열 파싱을 제거함
- `ListMessageIDsForThreads`와 `BulkSetThreadFlag`도 UUID 배열 `unnest` 경로로 바꿔 thread 배치 처리의 JSON 파싱을 제거함
- `ListThreadMessagesPage`는 `COALESCE(thread_id, id)` 비교를 UUID 친화적인 `thread_id = ... OR id = ...`로 분해함
- IMAP UID copy/expunge/move/hydrate 경로도 typed array unnest로 바꿔 요청당 JSON 직렬화 비용을 제거함
- `imapUIDArray` 1k/10k 벤치마크를 추가해 UID 전처리 비용을 추적할 수 있게 함
- IMAP mailbox lookup normalization도 `strings.Fields` 대신 로컬 공백 정리 스캐너를 쓰도록 바꿔 `SELECT`/`LIST` alias 처리의 토큰 슬라이스 비용을 줄임
- active 메시지/스레드 lookup용 partial index migration을 추가함
- Phase 2 배송 경로에 명시적인 `RecipientBatch` 계획 함수를 추가해 같은 도메인 수신자를 결정적 순서로 묶고, 1k/10k/100k 수신자 배치 계획 벤치마크를 추가함
- 최근 배치 계획 벤치마크 샘플: 1k/10도메인 ~49.1 us/op, 10k/100도메인 ~567.5 us/op, 100k/1k도메인 ~6.0 ms/op
- `GOGOMAIL_DELIVERY_RECIPIENT_BATCH_SIZE`를 추가해 대량 수신자 도메인 배치를 운영 환경에서 조정할 수 있게 하고, 기본값은 기존 수신자 한도와 맞춘 100으로 둠
- 배치 vs 개별 발송 벤치마크를 추가함: 100 수신자/10 도메인 기준 배치 경로는 10 SMTP transaction/op, 개별 경로는 100 SMTP transaction/op
- Phase 3 메시지 상세 조회에 bounded LRU EML body parse cache를 추가해 동일 storage path 반복 조회 시 storage read/parse를 재사용함
- `GOGOMAIL_MESSAGE_BODY_CACHE_ENTRIES`와 `GOGOMAIL_MESSAGE_BODY_CACHE_TTL`을 추가해 Mail API/IMAP/POP3의 parsed body cache 용량과 TTL을 운영 환경에서 조정하거나 비활성화할 수 있게 함
- message body cache snapshot에 enabled/entries/capacity/TTL/hit/miss/eviction/expired 카운터를 추가해 read-path 캐시 효과와 만료 정리를 낮은 cardinality로 관찰할 수 있게 함
- cache write 경로가 새 body 삽입 전에 만료 항목을 정리하도록 해 stale body가 capacity를 장시간 점유하지 않게 함
- Delivery observability sink에 route pool과 bounded recipient-count bucket을 추가해 대량 발송 배치 효과를 낮은 cardinality로 추적할 수 있게 함
- 백업/복구 리허설 스크립트를 추가해 PostgreSQL dump를 scratch DB에 복원하고 migration metadata를 확인한 뒤 기본적으로 scratch DB를 삭제하게 함
- `verify-backend-release.sh`가 `GOGOMAIL_RESTORE_REHEARSAL_DATABASE_URL` 설정 시 백업/복구 리허설을 릴리즈 검증 단계에 포함하도록 연결함
- `GOGOMAIL_SECURITY_VERIFY=1` 설정 시 `verify-backend-release.sh`가 `go vet ./...`와 설치된 `govulncheck ./...`를 보안 릴리즈 게이트로 실행하도록 함
- 프론트엔드 릴리즈 검증 스크립트를 추가해 webmail/console type-check와 helper test를 기본 실행하고, E2E/build는 명시 환경변수로 켤 수 있게 함

다음 단계: Phase 2 (Bulk Delivery Batching) 구현

### 검증

- `go test ./...` 통과
- `go build ./...` 성공
- 집중 검증: `go test ./internal/mailservice -run 'TestGetMessage(CachesParsedBodyByStoragePath|ReportsMessageBodyCacheStats|CanDisableMessageBodyCache)'`, `go test ./internal/config -run 'TestLoad|TestValidateRejectsInvalidMessageBodyCacheSettings|TestValidateRejectsNonpositiveDeliveryRecipientBatchSize'`, `go test ./internal/app -run '^$'` 통과
- 벤치마크 결과 기록 (쿼리/sec, 레이턴시, 메모리)
