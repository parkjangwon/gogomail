# ACTIVE_TASK

## Current Status Summary

**3차 전수 감사 잔여 이슈 병렬 처리** ✅ COMPLETE
- System email call sites wired: admin invite creation sends `SendInvite`, invite acceptance sends `SendWelcome`, quota alert scheduler sends `SendQuotaAlert` and marks alerts notified.
- Operations readiness: `X-Request-ID` middleware, configurable DB pool sizing, scheduled AutoPurge, backup script/Compose cron profile, and OpenAPI refresh-token contract updated.
- Mail API user refresh tokens: migration 0112 creates `user_refresh_tokens`; access-token login returns `refresh_token`; `/api/v1/auth/refresh` rotates refresh tokens.
- Webmail launch gaps: forgot/reset password UI, server-synced signatures, Web Push SW/device registration, calendar edit/delete, and `.env.example` completed.
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
- `CalendarModals.tsx`: `EventEditModal` 컴포넌트 추가 (반복 이벤트 범위 선택 포함)
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
- [ ] 벤치마크 프레임워크 (메시지 1000+, 10000+ 시나리오)
- [x] 테스트 검증: `go test ./...` 통과

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
