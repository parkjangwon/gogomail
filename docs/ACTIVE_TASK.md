# ACTIVE_TASK

## Current Status Summary

**3차 전수 감사 잔여 이슈 병렬 처리** ✅ COMPLETE
- System email call sites wired: admin invite creation sends `SendInvite`, invite acceptance sends `SendWelcome`, quota alert scheduler sends `SendQuotaAlert` and marks alerts notified.
- Operations readiness: `X-Request-ID` middleware, configurable DB pool sizing, scheduled AutoPurge, backup script/Compose cron profile, and OpenAPI refresh-token contract updated.
- Mail API user refresh tokens: migration 0112 creates `user_refresh_tokens`; access-token login returns `refresh_token`; `/api/v1/auth/refresh` rotates refresh tokens.
- Webmail launch gaps: forgot/reset password UI, server-synced signatures/filter rules/quick reply templates, Web Push SW/device registration, calendar edit/delete, and `.env.example` completed.
- Console launch gaps: audit-log cursor pagination, delivery-attempt filters/feedback, TS helper cleanup completed.
- Verification: `go test ./...`, `pnpm --dir apps/webmail type-check`, `pnpm --dir apps/console type-check`.

**DMARC Quarantine Folder Routing & User Refresh Tokens** ✅ COMPLETE
- DMARC quarantine: `enforceDMARCPolicy` in `internal/smtp/authentication.go` now returns `(DMARCEnforcementResult, error)`; `Quarantine=true` when `p=quarantine` and DMARC fails
- Receiver: `internal/smtp/receiver.go` reads `dmarcResult.Quarantine` and sets `FolderSystemType="spam"` on `ReceivedMessage` so delivery routes to the Spam folder via `deliveryFolderID`
- Mailauth enforcement comment in `internal/mailauth/enforcement.go` clarified: quarantine-mode hook continues to pass, real routing done at receiver level
- Migration 0112: `user_refresh_tokens` table (user_id FK, token_hash BYTEA, expires_at, revoked_at, created_at)
- `internal/maildb/user_refresh_tokens.go`: `CreateUserRefreshToken` (32-byte random, SHA-256 hash, 30-day TTL), `RotateUserRefreshToken` (validate + revoke + reissue in single TX)
- `POST /api/v1/auth/token`: response now includes `refresh_token` field when `RefreshTokenStore` is configured
- `POST /api/v1/auth/refresh`: new endpoint — validates refresh token, rotates it, returns new JWT + new refresh token
- `internal/httpapi/mail.go`: `UserRefreshTokenStore` interface + `MailRouteOptions.RefreshTokenStore` field
- `internal/app/run.go`: `repository` wired as `RefreshTokenStore`

**Webmail: 캘린더 이벤트 편집/삭제 UI + WebPush Service Worker** ✅ COMPLETE
- `EventPopover.tsx`: 팝오버에 "편집"/"삭제" 버튼 추가
- `CalendarModals.tsx`: `EventEditModal` 컴포넌트 추가 (반복 일정은 현재 지원되는 전체 시리즈 편집으로 명시)
- `CalendarView.tsx`: `openEditModal`, `handleEditSubmit`, `handleDeleteEvent` 핸들러 추가; `updateCalendarEvent` API 연동
- `api.ts`: `updateCalendarEvent(calendarId, objectName, uid, req)` 함수 추가
- `api.ts`: `registerWebPushDevice(subscription)` 함수 추가 (`POST /api/v1/push-devices`)
- `public/sw.js`: WebPush Service Worker 생성 (push 이벤트 → showNotification, notificationclick → openWindow)
- `SettingsView.tsx`: 알림 허용 시 SW 등록 + `pushManager.subscribe()` + 백엔드 구독 전송
- `mail/page.tsx`: 마운트 시 알림 허용이면 SW 자동 등록
- `.env.example`: `NEXT_PUBLIC_VAPID_PUBLIC_KEY=` 이미 포함
- TypeScript 오류 없음

**Webmail UI: 비밀번호 재설정 & 이메일 서명 서버 저장** ✅ COMPLETE
- `/forgot-password` 페이지: 이메일 입력 → `POST /api/auth/password-reset/request` → 성공 메시지 표시
- `/reset-password?token=<hex>` 페이지: 새 비밀번호 입력 → `POST /api/auth/password-reset/confirm` → 로그인 redirect
- 로그인 페이지에 "비밀번호를 잊으셨나요?" 링크 추가 (`/forgot-password`)
- Next.js API 프록시 라우트 2종 추가: `src/app/api/auth/password-reset/{request,confirm}/route.ts`
- `SettingsView.tsx` 서명 저장: localStorage + `setPreferences({ signatures: { default: signature } })` 서버 동기화
- 서명 로드: `getPreferences()`에서 `signatures.default` 우선, localStorage 폴백
- TypeScript 타입 오류 없음 (기존 CalendarView.tsx 오류 제외)

**3차 감사 수정 사항** ✅ COMPLETE
- Inbound MTA STARTTLS: `runReceiveMTA`에서 `smtpTLSConfig` 호출 누락 수정 — 인바운드 포트 25에서 STARTTLS 활성화
- IMAP 백필 OOM 방지: `backfillIMAPMailboxUIDsTx` 쿼리에 `LIMIT 1000` 추가 — 대형 메일박스 잠금 전체 로드 방지
- `audit_logs.user_id` 인덱스 추가 (migration 0110) — user_id 필터 full table scan 방지

**3차 감사 즉시 수정 사항** ✅ COMPLETE
- Console: `companies/"[id]"/alerts/page.tsx` → `companies/[id]/alerts/page.tsx` 라우팅 버그 수정 (페이지가 404였음)
- Login rate limit: `POST /api/v1/auth/token`에 10/min per-IP 제한 추가 (기존: 없음)
- Password reset rate limit: `POST /api/v1/auth/password-reset/request`에 5/15min per-IP 제한 추가
- JSON 로그: production 환경에서 `NewTextHandler` → `NewJSONHandler`로 교체

**Critical Security Fixes** ✅ COMPLETE
- `internal/maildb/user_auth.go`: `AuthenticateUser` now JOINs `companies` table and returns `ErrCompanySuspended` if company status is `suspended`; callers in `mail.go` map this to HTTP 403
- `internal/httpapi/admin.go`: Added `requiresCompanyAccess` checks to all mutation endpoints (PATCH/DELETE/POST companies, bulk-import/export, company config/*, domain status/quota/DELETE/settings/config/*, user DELETE/status/password-hash)

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

**Password Reset & System Email Infrastructure** ✅ COMPLETE
- Migration 0109: `password_reset_tokens` table (user_id FK, token_hash BYTEA, expires_at, used_at)
- `internal/maildb/password_reset.go`: CreatePasswordResetToken, GetPasswordResetToken, MarkTokenUsed, ResetUserPassword, GenerateResetToken
- `internal/mailservice/systememail.go`: SystemEmailSender interface + SMTPSystemEmailSender (log-only when GOGOMAIL_SYSTEM_EMAIL_FROM unset)
- `internal/httpapi/password_reset.go`: POST /api/v1/auth/password-reset/request + confirm, MaildbPasswordResetAdapter
- Wired in internal/app/run.go alongside existing mail API routes
- 17 new unit tests (maildb + httpapi); 5936 total tests passing

**Security/RFC Hardening (OIDC, SAML, SMTP)** ✅ COMPLETE
- OIDC: `alg=none` and non-RS256 algorithms now rejected when no clientSecret is configured
- OIDC: audience (`aud`) claim rejected when clientID not configured but token carries non-empty aud
- SAML: DigestValue verification added to `VerifySAMLSignature` — compares SHA-256 of Assertion
  bytes against the DigestValue in SignedInfo, blocking XSW attacks
- SMTP: Received header now uses `\r\n` line endings (RFC 5321 §2.3.8 CRLF compliance)

**Console Admin MFA Implementation** ✅ COMPLETE
- Task 1 ✅ DONE: Added `AdminMFARequired` config field (env: `GOGOMAIL_ADMIN_MFA_REQUIRED`)
- Task 2 ✅ DONE: Added MFA status check to admin login with `mfa_required` and `mfa_setup_required` flows
- Task 3 ✅ DONE: Added `/admin/v1/auth/mfa/` endpoints (`verify`, `status`, `setup`, `setup/confirm`, `DELETE`)
- Task 4 ✅ DONE: Added `admin` CLI break-glass command `gogomail admin mfa-reset --email <email>`
- Task 5 ✅ DONE: Registered MFA routes in admin router and wired `adminMFAStore` / `adminMFARequired` / config resolver
- Task 6 ✅ DONE: Implemented console login MFA step and recovery-code support
- Task 7 ✅ DONE: Added full MFA flow in console security settings (setup, confirm, disable)
- Task 8 ✅ DONE: Added localStorage-based `console_mfa_setup_required` gate in company layout
- Task 9 ✅ DONE: Added runtime policy integration and status/refresh behavior for forced setup

**Admin API Security & Pagination Hardening** ✅ COMPLETE
- Replaced `err.Error()` in HTTP 500 responses with generic "internal server error" + slog server-side logging
  - admin.go: 143 occurrences (all `writeError(w, http.StatusInternalServerError, err.Error())`)
  - alerts.go: 9 occurrences (`http.Error(w, err.Error(), http.StatusInternalServerError)`)
  - webdav.go: 9 occurrences (`http.Error(w, "..."+err.Error(), http.StatusInternalServerError)`)
- Added `has_more` field to 5 list endpoints: domains, users, audit-logs, outbox-events, delivery-attempts
  - DB functions now support `ProbeMore: true` (limit+1 probe pattern, same as companies)
  - Interfaces updated: `ListDomains`, `ListUsers`, `ListOutboxEvents`, `ListAuditLogs`, `ListDeliveryAttempts` now return `([]T, bool, error)`
  - All call sites updated across admin.go, app/run.go, idprovider/database/provider.go, test files

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

**Email Protocol Compliance Hardening** ✅ COMPLETE
- Task 1 (DMARC): New domain default DMARC policy changed from `p=none` to `p=quarantine`
  (`internal/httpapi/admin.go` → `defaultDmarcSpfPolicy()`)
- Task 2 (List-Unsubscribe): Outbound bulk sends (≥5 recipients) now auto-inject
  `List-Unsubscribe` and `List-Unsubscribe-Post` headers if not already present
  (`internal/smtp/submission.go` → `bulkRecipientThreshold`, `messageHasListUnsubscribe`)
- Task 3 (SCIM PATCH): Implemented RFC 7644 `PATCH /scim/v2/Users/{id}` endpoint
  supporting `replace` op on `active` (path-targeted and path-less value object form);
  `SCIMUserService.PatchSCIMUser()` added to interface and `maildbSCIMUserService`;
  `ServiceProviderConfig` updated to report `patch.supported: true`

**Infrastructure Improvements** ✅ COMPLETE
- Task 1 (DB 커넥션 풀 환경변수화): `Config`에 `DBMaxOpenConns`/`DBMaxIdleConns`/`DBConnMaxLifetime` 필드 추가; `database.Open`에 `Options` 가변인자 추가; `internal/app/run.go` 24곳 호출부 모두 cfg 값 전달로 교체
  - env: `GOGOMAIL_DB_MAX_OPEN_CONNS` (기본 20), `GOGOMAIL_DB_MAX_IDLE_CONNS` (기본 5), `GOGOMAIL_DB_CONN_MAX_LIFETIME` (기본 30m)
- Task 2 (X-Request-ID 미들웨어): `httpapi.RequestIDMiddleware`, `RequestIDFromContext`, `RequestIDAttr` 추가; `run.go` HTTP 핸들러 체인 최외곽에 삽입
- Task 3 (DB 백업 스크립트): `scripts/backup.sh` (pg_dump→gzip, 보존 기간 자동 삭제, S3 업로드 옵션); `docker/docker-compose.backup.yml` (alpine crond, 매일 03:00 UTC)

**TASK-090: Message Storage & Delivery Optimization** 🔄 IN PROGRESS
- Phase 1 (Database Query Optimization): Partial indexes, UUID array hydration, and batch lookup improvements implemented
- Phase 2 (Bulk Delivery Batching): Same-domain batch planning, runtime batch-size tuning, observability, and benchmarks implemented
- Phase 3 (Message Caching Layer): Parsed EML body LRU cache with runtime tuning and cache snapshots implemented
- Mailbox submission isolation hardening: bulk sender role is now correctly bound in both inbound and submission paths, and bulk limiter now yields/pauses between denied attempts to prevent starvation under abusive loops (`TestSubmissionBulkIsolation` now passes).
- Organization chart user-unit lookup placeholder removed: `GetUserUnits` now resolves active user assignments through repository-backed joins, with an active-member index migration.
- Organization hierarchy member loading no longer performs one query per unit; `GetHierarchy` now batch-loads active members for all units and groups them in memory.
- Admin user bulk status updates no longer call the single-user update path in a loop; the endpoint now uses one repository bulk update and enforces company_admin company scope.

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
- [x] 누락된 인덱스 생성 (outbox + delivery_attempts 조회 경로): `0113_delivery_attempt_indexes.sql`, `0114_outbox_query_indexes.sql`
- [ ] ListOutboundMessages 최적화 (N+1 제거)
- [ ] GetMessagesByID 배치 조회 함수 작성
- [x] 벤치마크 프레임워크 (메시지 1000+, 10000+ 시나리오)
- [x] 배송 기록 경로 벤치마크: `BenchmarkRecordAttemptBatchBulkVsIndividual`로 bulk vs individual recorder 호출 수/할당 추적
- [x] 테스트 검증: `go test ./...` 통과

최근 진행:
- `ListMessagesByIDs` hydration을 `unnest($2::uuid[]) WITH ORDINALITY` 기반으로 바꿔 JSON 배열 파싱을 제거함
- `ListMessageIDsForThreads`와 `BulkSetThreadFlag`도 UUID 배열 `unnest` 경로로 바꿔 thread 배치 처리의 JSON 파싱을 제거함
- `ListThreadMessagesPage`는 `COALESCE(thread_id, id)` 비교를 UUID 친화적인 `thread_id = ... OR id = ...`로 분해함
- `ListMessagesPage`도 folder 필터가 있을 때만 `messages.folder_id = $2::uuid` 조건을 추가하고, 폴더 없는 전체 목록에서는 folder predicate를 제거해 메시지 페이지 조회의 optional `OR` predicate를 줄임
- `ListMessagesPage`의 read/starred/attachment 필터도 제공된 경우에만 직접 predicate를 추가하고, 필터 없는 목록에서는 nullable boolean optional `OR` predicate를 제거함
- `ListMessagesPage` cursor predicate도 커서가 있을 때만 newest/oldest 방향에 맞는 tuple comparison을 추가하고, 첫 페이지 목록에서는 pagination optional `OR`를 제거함
- `SearchMessages`도 folder 필터가 있을 때만 `folder_id = $3::uuid` 조건을 추가하고, 전체 메일 검색에서는 folder predicate를 제거해 메시지 검색 조회의 optional `OR` predicate를 줄임
- `SearchMessages`/`SearchDrafts`의 attachment 필터도 제공된 경우에만 typed boolean predicate를 추가하고, attachment-agnostic 검색에서는 predicate를 제거해 optional `OR` predicate를 줄임
- `SearchMessages`/`SearchDrafts`의 cursor predicate도 커서가 있을 때만 tuple comparison을 추가하고, 첫 페이지 검색에서는 predicate를 제거해 pagination optional `OR`를 줄임
- thread list의 read/starred/attachment 필터도 제공된 경우에만 직접 predicate를 추가하고, 필터 없는 목록에서는 nullable boolean optional `OR` predicate를 제거함
- thread list cursor predicate도 커서가 있을 때만 newest/oldest 방향에 맞는 tuple comparison을 추가하고, 첫 페이지 목록에서는 pagination optional `OR`를 제거함
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
- Outbox pending/processing 재시도 경로에서 `FOR UPDATE SKIP LOCKED` 대상 집합을 후보 집합(CTE)으로 분리해 정렬/제한 후 일괄 업데이트하도록 변경해 잠재적 잠금 경합과 비효율적 쿼리를 완화함
- 메시지/재시도 조회 경로용 인덱스 마이그레이션 `0113_delivery_attempt_indexes.sql`, `0114_outbox_query_indexes.sql`를 추가해 `status/attempted_at/topic/partition` 필터 경로의 인덱스 커버리지를 강화함
- 조직도 서비스의 `GetUserUnits` placeholder를 제거하고 `organization_members` → `organization_units` 활성 배정 조회로 구현했으며, `0115_organization_member_active_user_index.sql`로 사용자별 active membership lookup을 보강함
- 조직도 `GetHierarchy`의 per-unit `GetMembersInUnit` N+1 조회를 `ListMembersInUnits` 단일 배치 조회로 바꾸고, `0116_organization_members_active_unit_index.sql`로 active unit-member lookup을 보강함
- `/admin/v1/users/bulk`의 사용자 상태 변경을 단건 `UpdateUserStatus` 반복 호출에서 `BulkUpdateUserStatus` 단일 UPDATE 경로로 바꾸고, company_admin 요청은 `CompanyID` 필터로 자기 회사 사용자만 갱신되도록 강화함
- 백업/복구 리허설 스크립트를 추가해 PostgreSQL dump를 scratch DB에 복원하고 migration metadata를 확인한 뒤 기본적으로 scratch DB를 삭제하게 함
- `verify-backend-release.sh`가 `GOGOMAIL_RESTORE_REHEARSAL_DATABASE_URL` 설정 시 백업/복구 리허설을 릴리즈 검증 단계에 포함하도록 연결함
- `GOGOMAIL_SECURITY_VERIFY=1` 설정 시 `verify-backend-release.sh`가 `go vet ./...`와 설치된 `govulncheck ./...`를 보안 릴리즈 게이트로 실행하도록 함
- 프론트엔드 릴리즈 검증 스크립트를 추가해 webmail/console type-check와 helper test를 기본 실행하고, E2E/build는 명시 환경변수로 켤 수 있게 함
- 웹메일의 캘린더 UID, 임시 첨부 ID, 토스트 ID, 필터 규칙 ID, vCard fallback filename 생성을 `Math.random()`에서 `crypto.randomUUID()` 기반 공통 헬퍼로 교체해 충돌 가능성과 비결정적 fallback을 줄임
- outbox relay가 배치 claim 후 이벤트별 `MarkDone`/`MarkFailed` UPDATE를 반복하던 경로에 선택적 `BatchStore`를 추가하고, PostgreSQL store는 성공/실패 상태를 각각 단일 UPDATE로 묶어 배치 크기 N 기준 DB 상태 갱신 왕복을 최대 N회에서 1~2회로 줄임
- delivery handler가 다수 수신자 attempt를 수신자별 `RecordAttempt` 트랜잭션으로 기록하던 경로에 선택적 `BulkRecorder`를 추가하고, PostgreSQL recorder는 delivery_attempts/outbox event/suppression insert를 배열 `unnest` 기반 배치 INSERT로 묶어 기록 왕복과 트랜잭션 수를 줄임
- retry exhausted 기록도 동일한 delivery_attempts 배치 INSERT를 재사용해, 대량 수신자 메시지가 최종 소진될 때 수신자별 INSERT를 반복하지 않도록 함
- outbox pending claim SQL에서 UNION 후보 산출과 실제 `FOR UPDATE OF o SKIP LOCKED` row lock 단계를 분리해 PostgreSQL row locking 제약과 동시 relay 경합에서 더 안정적으로 동작하게 함
- delivery attempt recorder 벤치마크를 추가해 100 수신자 기준 bulk path가 `1.000 record_calls/op`, fallback individual path가 `100.0 record_calls/op`임을 추적할 수 있게 함
- `BenchmarkRecordAttemptBatchBulkVsIndividual`: 100 수신자 기준 bulk recorder `~3.18 ns/op`, `1.000 record_calls/op`; fallback individual recorder `~1051 ns/op`, `100.0 record_calls/op`
- `BulkSetMessageFlag`, `BulkMoveMessages`, `deleteIMAPUIDRowsForMessages`가 JSON 배열 확장 대신 `unnest($2::uuid[])` typed array를 사용하도록 바꿔 메시지 bulk mutation의 JSON 인코딩/파싱 비용을 제거함
- `BenchmarkBulkMessageIDsArrayValue`: UUID array 준비 비용 1k `~69.3 us/op`, 10k `~689.3 us/op` 기준선을 추가함
- `BulkDeleteMessages`, `LookupDeleteableStoragePaths`, `BulkRestoreMessages`, `BulkMoveThreads`, `BulkRestoreThreads`, `BulkDeleteThreads`도 typed UUID array 기반 CTE로 전환해 메시지/스레드 bulk 작업의 `$2::jsonb` ID 확장을 제거함
- inbound/submission thread resolution과 draft attachment binding 조회가 `array_position($2, ...)` 대신 `unnest(... WITH ORDINALITY)`를 사용하도록 바꿔 PostgreSQL이 후보 배열을 행마다 재스캔하지 않게 함
- `TestResolveThreadIDSQLUsesOrdinalityArray`, `TestAttachmentsByIDsSQLUsesUuidOrdinality`를 추가해 reply threading/attachment lookup SQL이 ordinality 기반 typed array 경로를 유지하도록 고정함
- thread list newest/oldest 쿼리에서 `SELECT * FROM thread_summaries`를 제거하고 API 응답에 필요한 10개 컬럼만 명시해 스레드 목록 read model projection을 고정함
- `TestThreadListSQLUsesLatestMessagePreview`가 명시 컬럼 projection과 `SELECT *` 금지를 함께 검증하도록 확장함
- thread list newest/oldest 쿼리도 folder 필터가 있을 때만 `messages.folder_id = $8::uuid` 조건을 추가하고, 전체 스레드 목록에서는 folder predicate를 제거해 스레드 목록 조회의 optional `OR` predicate를 줄임
- delivery worker가 TLS-RPT collector domain을 `localhost` 고정값으로 두지 않고 `GOGOMAIL_SMTP_DOMAIN`에서 주입하도록 바꿔 운영 TLS report identity가 실제 MTA 도메인을 쓰게 함
- `TestDirectSMTPTransportTLSReportDomainCanBeConfigured`를 추가해 TLS-RPT report domain override가 collector에 반영되는지 검증함
- outbox relay `MarkFailedBatch` SQL이 `SELECT *` 대신 `unnest` 결과의 `id,last_error` 컬럼을 명시 투영하도록 바꿔 배치 실패 갱신 경로의 SQL shape를 고정함
- 웹메일 preferences 저장을 서버 기존 값과 병합한 뒤 PUT하도록 바꿔 서명/필터/일반 설정 저장이 서로의 키를 지우지 않게 했고, `SettingsView`와 사이드바 `SettingsModal` 모두 서버값을 로컬 캐시에 반영해 메일 목록의 클라이언트 필터 적용 경로와 설정 화면이 같은 규칙/서명/일반 설정을 보게 함
- `TestMarkFailedBatchSQLProjectsUnnestColumns`를 추가해 outbox batch failure CTE가 명시 projection을 유지하는지 검증함
- draft attachment binding이 첨부 ID마다 UPDATE를 반복하지 않고 `unnest($3::uuid[])` typed-array 단일 UPDATE로 바인딩하도록 바꿔 draft save/update 첨부 N+1 왕복을 제거함
- `TestBindDraftAttachmentsSQLUsesSingleTypedArrayUpdate`를 추가해 draft attachment binding SQL이 단일 typed-array update 경로를 유지하도록 고정함
- attachment upload session finalization의 `target` CTE가 `SELECT *` 대신 attachment 생성과 draft refresh에 필요한 컬럼만 잠그고 전달하도록 축소함
- `TestFinalizeAttachmentUploadSessionSQLProjectsTargetColumns`를 추가해 finalize CTE가 wide projection으로 되돌아가지 않도록 고정함
- audit log integrity checker의 최근 로그 subquery가 `SELECT *` 대신 hash-chain 검증에 필요한 audit columns만 읽도록 좁혀 운영 무결성 점검 쿼리 projection을 축소함
- `TestAuditLogIntegrityQueryProjectsRecentColumns`를 추가해 audit integrity query가 explicit projection을 유지하도록 고정함
- attachment upload session 만료 정리가 세션별 `UPDATE`와 quota decrement를 반복하지 않고 `unnest($1::uuid[])`/`unnest($1::uuid[], $2::bigint[])` 기반 batched UPDATE로 세션 상태와 user/domain/company quota를 한 번씩 갱신하도록 최적화함
- `TestExpireAttachmentUploadSessionsSQLUsesBatchUpdates`를 추가해 stale upload cleanup이 per-session write loop로 되돌아가지 않도록 고정함
- attachment share link 목록 조회도 attachment/status 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 공유 링크 운영 조회의 optional `OR` predicate를 제거함
- attachment upload session 목록 조회도 user/draft/status 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 대량 첨부 업로드 운영 조회의 optional `OR` predicate를 제거함
- bulk delete/IMAP EXPUNGE storage GC lookup이 대상 메시지마다 `messages ref`를 correlated `COUNT(*)`로 재스캔하지 않고, target storage paths를 먼저 뽑은 뒤 grouped `ref_counts` CTE로 공유 여부를 판정하도록 바꿈
- `TestStoragePathLookupSQLUsesGroupedReferenceCounts`를 추가해 bulk delete와 IMAP EXPUNGE storage path lookup이 grouped reference-count CTE를 유지하도록 고정함
- legacy attachment upload cleanup도 stale attachment마다 `UPDATE attachments`와 quota decrement를 반복하지 않고 typed UUID-array batch update와 공용 aggregated quota decrement CTE를 쓰도록 바꿈
- `TestExpireStaleAttachmentUploadsSQLUsesBatchUpdates`를 추가하고 attachment upload session cleanup도 같은 `decrementUserQuotasSQL` 회귀 가드를 공유하도록 정리함
- thread list read path가 `COALESCE(thread_id, id)` thread key와 `message_at` expression으로 active messages를 집계하는 쿼리 shape에 맞춰 `0117_thread_list_indexes.sql` partial expression indexes를 추가함
- `TestThreadListIndexMigrationMatchesThreadQueries`를 추가해 thread list index migration이 folder-scoped/unscoped active thread-key lookup을 계속 지원하도록 고정함
- delivery attempt 생성과 exhausted event payload 생성이 수신자마다 DSN recipient 옵션을 선형 검색하지 않고, 요청당 한 번 만든 normalized address map을 재사용하도록 바꿔 대량 수신자 처리의 O(n²) DSN lookup 비용을 제거함
- `TestDSNRecipientOptionsByAddressPreservesFirstNormalizedMatch`를 추가해 DSN recipient option map이 기존 first-match 의미와 빈 주소 제외를 유지하도록 고정함
- retry dedupe key 생성이 `fmt.Sprint`/`strings.Join` 중간 문자열을 만들지 않고 사전 크기 계산된 `strings.Builder`에 직접 attempt/recipient key를 쓰도록 바꿔 대량 retry scheduling CPU/할당 비용을 낮춤
- `BenchmarkRetryDedupeKey`에 1k/10k 수신자 케이스를 추가해 대량 수신자 retry key 생성 비용을 계속 추적할 수 있게 함
- Drive rename/move/upload-session 생성 SQL의 마지막 CTE read가 `SELECT * FROM updated/inserted`를 쓰지 않고 API 응답에 필요한 컬럼을 명시하도록 좁혀 storage read/write 경로의 projection 회귀를 줄임
- `TestDriveRepositorySQLAvoidsWideCTEProjection`을 추가해 Drive repository SQL이 wide CTE projection으로 되돌아가지 않도록 고정함
- Drive node 목록 조회가 query/type/parent scope 조건을 제공된 경우에만 WHERE에 추가하도록 바꿔 대형 폴더와 전체 Drive 검색에서 optional `OR` predicate와 `NULLIF` parent guard를 피하게 함
- Drive upload session 목록 조회도 status 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 대량 업로드 운영 조회의 optional `OR` predicate를 제거함
- IMAP unknown command/UID subcommand 응답이 내부 구현 상태처럼 보이는 `not implemented` 대신 클라이언트-facing `unsupported command`/`unsupported UID command`를 반환하도록 바꿔 프로토콜 오류 문구를 제품 수준으로 정리함
- UID syntax validation 테스트를 갱신해 미지원 UID 하위명령이 인증/선택 상태보다 먼저 안정적으로 거부되는 동작을 유지하도록 확인함
- CardDAV/CalDAV REPORT fallback과 directory principal-kind fallback도 내부 구현 상태처럼 보이는 `not implemented` 대신 `unsupported` 오류를 반환하도록 정리함
- CardDAV/CalDAV `reportResponses` 단위 테스트를 추가해 unsupported REPORT 오류가 제품-facing 문구를 유지하고 `not implemented`를 노출하지 않도록 고정함
- Organization sync no-op adapter가 성공으로 기록되던 경로를 제거함: adapter 미구성 시 `ErrOrgChartSyncNotConfigured`로 실패 로그를 남기고 HTTP sync endpoint는 501을 반환하며, batch worker는 미구성 sync job을 등록하지 않음
- LDAP sync API가 미구성 상태에서 `pending` 성공처럼 보이던 경로를 제거함: sync run은 `failed`로 기록하고 `ErrSyncNotConfigured`를 반환하며 admin HTTP endpoint는 501로 노출함
- LDAP 직접 provider read/sync 메서드도 내부 placeholder 문구 대신 typed `ErrProviderNotConfigured`/`ErrReadUnavailable`/`ErrSyncNotConfigured`를 반환하도록 정리함
- Console 조직 페이지의 외부 동기화 상태 안내도 `placeholder`/`not available yet` 내부 용어 대신 “연동 설정 필요/미활성” 제품 문구와 지역화된 badge label을 사용하도록 정리함
- 누락되어 404였던 RDBMS sync admin routes를 등록하고, 외부 provider 미구성 상태를 `pending`으로 숨기지 않고 typed `ErrSyncNotConfigured` 실패로 기록하며 admin HTTP endpoint가 501을 반환하도록 정리함
- RDBMS membership sync도 provider schema가 membership query를 지원하지 않는 상태에서 성공 no-op으로 보이지 않도록 `ErrMembershipSyncUnsupported` 실패로 명시함
- Partial delivery attempt 기록이 실패 수신자마다 DSN recipient map을 재생성하지 않고 요청당 한 번 만든 map을 재사용하도록 바꿔 대량 부분 실패 기록의 O(n²) 비용을 제거함
- `GetMessage` detail path가 `HasAttachment=false`인 메시지에서 attachment list 조회를 건너뛰도록 바꿔 첨부 없는 읽기 경로의 DB 왕복을 줄이고, cache hit/miss 벤치마크를 추가함
- `BenchmarkGetMessageBodyCache`: miss `~7.83 us/op`, `10979 B/op`, `85 allocs/op`; hit `~933.6 ns/op`, `568 B/op`, `9 allocs/op`
- README/README.ko, Docker env example, console/webmail env examples, backend release readiness 문서를 최신 운영 env 그룹(성능, 백업/복구, push/webhook, storage, API usage, system email)과 동기화함
- BIMI VMC URL 존재만으로 `vmcVerified=true`를 반환하던 stub 동작을 제거해 실제 인증서 검증 전에는 VMC를 검증됨으로 표시하지 않도록 고정하고, logo cache hash 계산을 실제 SHA-256 body hash로 수정함
- 웹메일 캘린더 반복 일정 편집 모달에서 지원되지 않는 "이 이벤트만" 선택지를 제거하고, 현재 동작이 전체 반복 시리즈 편집임을 명시해 제출 후 unsupported 오류가 뜨는 어색한 흐름을 없앰
- 웹메일 Web Push 등록이 VAPID public key를 문자열 그대로 `PushManager.subscribe()`에 넘기지 않고 표준 `Uint8Array` applicationServerKey로 변환하도록 고정하고, 메일 화면 진입 시 알림 권한 프롬프트를 자동 표시하지 않도록 설정 화면의 명시적 opt-in 흐름으로 정리함
- 웹메일 빠른 답장 템플릿을 브라우저 localStorage 전용 상태에서 서버 preferences 동기화 상태로 승격하고, Compose/Settings/Spotlight가 같은 normalized template cache를 보도록 정리함
- 관리자 outbox event 목록 조회가 `NULLIF(...) OR ...` 조건으로 인덱스 선택성을 흐리지 않도록, 제공된 필터만 WHERE에 추가하는 sargable 동적 쿼리로 전환함
- 관리자 delivery attempt 목록 조회도 status/since/domain/message/farm/sender 필터를 제공된 조건만 포함하는 sargable WHERE로 전환해 대형 배송 기록 테이블에서 선택적 조회가 인덱스를 더 잘 타도록 정리함
- delivery attempt 통계와 exhausted 목록도 같은 공통 sargable WHERE builder를 재사용하도록 바꿔, 대형 배송 기록 테이블을 읽는 list/stats/exhausted 운영 화면이 모두 optional `OR` predicate를 피하게 함
- 관리자 company 목록 조회도 status 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 SaaS 테넌트 운영 목록의 optional `OR` predicate를 제거함
- Push notification attempt 목록과 통계 조회도 동적 sargable WHERE로 전환해 Web Push 운영 기록이 커져도 message/user/platform/device/provider 필터가 인덱스 친화적인 쿼리 모양을 유지하게 함
- 도메인 DNS check 이력 조회도 status/since 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 도메인 운영 화면의 DNS 검증 이력 조회가 optional `OR` predicate를 피하게 함
- API usage daily/monthly 집계 조회도 tenant/company/domain/user/key/principal/auth/method/route/status/time 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 SaaS 사용량/청구 분석 화면의 대형 집계 조회가 인덱스 친화적인 쿼리 모양을 유지하게 함
- API usage export batch 목록 조회도 tenant/principal/status/window 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 청구/사용량 export handoff 조회가 optional `OR` predicate를 피하게 함
- DKIM key 목록 조회도 domain/status 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 도메인 인증 운영 화면의 optional `OR` predicate를 제거함
- Directory alias 목록 조회도 domain/target/query/active 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 대형 주소록에서 alias 운영 조회가 optional `OR` predicate를 피하게 함
- Directory organization tree 조회도 domain 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 조직도 로딩 경로가 optional `OR` predicate를 피하게 함
- Directory alias/user email resolve 조회도 `ActiveOnly`가 켜진 경우에만 active 상태 조건을 추가하도록 바꿔 주소/사용자 exact lookup이 boolean optional `OR` predicate를 피하게 함
- Directory principal ID resolve 조회도 user/org/group/resource 각각 `ActiveOnly`가 켜진 경우에만 active 상태 조건을 추가하도록 바꿔 delegation/membership 검증 경로의 boolean optional `OR` predicate를 제거함
- Directory group membership 목록 조회도 group/member/role/active 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 대형 그룹 멤버십 운영 조회가 optional `OR` predicate를 피하게 함
- Directory direct/effective group membership 검증 조회도 `ActiveOnly`가 켜진 경우에만 active 상태 조건을 삽입하도록 바꿔 멤버십 validation hot path의 boolean optional `OR` predicate를 제거함
- Directory direct delegation 검증 조회도 `ActiveOnly`가 켜진 경우에만 active 상태 조건을 삽입하도록 바꿔 delegation validation 경로의 boolean optional `OR` predicate를 제거함
- Directory delegation 목록 조회도 owner/delegate/scope/role/active 필터를 제공된 경우에만 WHERE에 추가하도록 바꿔 대형 delegation 운영 조회가 optional `OR` predicate를 피하게 함
- Directory principal search 조회도 요청된 principal kind branch만 UNION에 포함하고 domain/org/query/active 필터를 제공된 경우에만 추가하도록 바꿔 주소록 검색이 optional `OR` predicate와 불필요한 branch scan을 피하게 함

**System Email Connections & AutoPurge** ✅ COMPLETE
- `internal/httpapi/admin.go`: Added `systemEmail mailservice.SystemEmailSender` and `publicBaseURL string` fields to `adminRouteConfig`; added `WithSystemEmailSender` and `WithPublicBaseURL` `AdminRouteOption` constructors
- `POST /admin/v1/users/{id}/invite`: After token creation, fetches user+domain in background goroutine, sends `SendInvite` email with URL `{publicBaseURL}/admin/invite/{token}/accept`
- `POST /admin/invite/{token}/accept`: After accepting, sends `SendWelcome` email in background goroutine; failure is non-fatal
- `internal/app/run.go`: Wired `WithSystemEmailSender` and `WithPublicBaseURL` into `adminRouteOpts` for the admin API
- `migrations/0111_quota_alerts_notified_at.sql`: Added `notified_at timestamptz` column to `quota_alerts`
- `internal/maildb/quota_alerts.go`: Added `PendingQuotaAlertEmail` struct, `GetPendingQuotaAlertEmails()`, and `MarkQuotaAlertNotified()` methods
- `internal/app/run.go` quota-alert-check job: After scanning alerts, fetches pending user-scope alert emails, calls `SendQuotaAlert`, marks `notified_at` on success
- `internal/maildb/autopurge.go` (new): `GetCompaniesWithAutoPurge()` reads retention policies from `runtime_config`; `PurgeExpiredTrashMessages()` deletes messages in trash folders older than retention days; `PurgeExpiredAuditLogs()` deletes expired `audit_logs` rows
- `internal/config/config.go`: Added `AutoPurgeEnabled bool` (env: `GOGOMAIL_AUTO_PURGE_ENABLED`, default false)
- `internal/app/run.go`: `auto-purge` batch job (24h interval) iterates companies with `auto_purge_enabled=true` and purges trash messages + audit logs per-company retention settings

**Redis Sentinel Failover & Alert Dispatcher** ✅ COMPLETE
- `internal/config/config.go`: 새 필드 `RedisSentinelAddrs` (env: `GOGOMAIL_REDIS_SENTINEL_ADDRS`), `RedisMasterName` (env: `GOGOMAIL_REDIS_MASTER_NAME`, 기본: "mymaster") 추가
- `internal/config/config.go`: 알림 설정 필드 `AlertEmailTo`, `AlertEmailFrom`, `AlertSMTPAddr`, `AlertWebhookURL` 추가
- `internal/app/run.go`: `newRedisClient(cfg)` 헬퍼 함수 추가 — Sentinel 주소가 있으면 `redis.NewFailoverClient` 사용, 없으면 `redis.NewClient` 사용
- `internal/app/run.go`: 모든 `redis.NewClient(...)` 호출 (12곳) → `newRedisClient(cfg)` 으로 교체
- `internal/alert/dispatcher.go`: `Dispatcher` 인터페이스 구현체 3종 추가:
  - `EmailDispatcher`: `net/smtp` 기반 이메일 알림 (SMTP 주소, From/To 설정 가능)
  - `WebhookDispatcher`: Slack incoming webhook 호환 JSON POST
  - `MultiDispatcher`: 여러 dispatcher 팬아웃

**Draft Optimistic Locking** ✅ COMPLETE
- `ErrDraftConflict` sentinel added to `internal/maildb`
- `SaveDraftRequest.IfUpdatedAt` (maildb + mailservice) propagates a last-known timestamp
- `updateDraft` adds `AND draft_updated_at = $16` predicate when `IfUpdatedAt` is non-zero;
  if 0 rows affected it distinguishes conflict from not-found with a secondary SELECT
- PATCH handler returns HTTP 409 + `{"error":{"code":"conflict",...}}` on conflict
- No DB migration required — uses the existing `draft_updated_at` column

**OpenSearch Korean Nori Analyzer** ✅ COMPLETE
- `SearchIndexOpenSearchKoreanAnalyzer bool` config field (env: `GOGOMAIL_OPENSEARCH_KOREAN_ANALYZER`)
- When enabled, `openSearchIndexDefinition` adds `"analysis":{"analyzer":{"korean":{"type":"nori","decompound_mode":"mixed"}}}` to settings and sets `analyzer: "korean"` on `subject` and `body_text` mappings
- Requires the OpenSearch `analysis-nori` plugin; safe to leave disabled otherwise

다음 단계: Phase 2 (Bulk Delivery Batching) 구현

### 검증

- `go test ./...` 통과
- `go build ./...` 성공
- 집중 검증: `go test ./internal/mailservice -run 'TestGetMessage(CachesParsedBodyByStoragePath|ReportsMessageBodyCacheStats|CanDisableMessageBodyCache)'`, `go test ./internal/config -run 'TestLoad|TestValidateRejectsInvalidMessageBodyCacheSettings|TestValidateRejectsNonpositiveDeliveryRecipientBatchSize'`, `go test ./internal/app -run '^$'` 통과
- 벤치마크 결과 기록 (쿼리/sec, 레이턴시, 메모리)
