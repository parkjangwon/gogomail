# gogomail current status

Last updated: 2026-05-27 (security hardening: password DoS cap, IdP error info leak)

## Post-remediation hardening round 2 (2026-05-27)

**Two more code-review findings fixed**
- **internal/maildb/me.go** + **internal/httpapi/password_reset.go**: Added 1024-byte cap on `new_password` in both the change-password and reset-password flows. Without the cap, a caller could submit a multi-megabyte string and force PBKDF2 210k iterations over it — a CPU DoS. IMAP already had the same cap (`maxIMAPAuthPasswordBytes = 4096`).
- **internal/httpapi/admin_ldap_sync.go**: Replaced three `writeError(w, 500, err.Error())` with generic messages + `slog.ErrorContext` log. The old code leaked LDAP/DB error strings (connection strings, hostnames, SQL details) in the HTTP response body.

## Post-remediation hardening (2026-05-27)

**Three follow-up improvements found during post-CSO code review**
- **internal/maildb/user_auth.go**: `upgradePasswordHash` goroutine now wraps context with 60-second timeout via `context.WithTimeout`; prevents goroutine leak if DB connection hangs during async hash upgrade
- **internal/bimi/bimi.go**: `FetchLogo` now calls `webhookguard.ValidateOutboundHTTPURL` before fetching, blocking SSRF attacks where an attacker-controlled `_bimi.domain` DNS record points to a private/internal IP; `LogoCache.allowPrivateNetwork` field added (false by default) for test override
- **internal/httpapi/webdav.go**: Two unbounded `io.ReadAll(r.Body)` calls (PROPFIND and PROPPATCH handlers) now use `io.LimitReader` capped at `maxJSONBodyBytes` (1 MB) to prevent large-body DoS

## Task 8: Next.js CSP nonce — webmail + console (2026-05-27)

**Replace `script-src 'unsafe-inline'` with per-request nonce-based CSP in both Next.js apps**
- **apps/webmail/src/middleware.ts** + **apps/console/src/middleware.ts**: Generate a base64 nonce (`Buffer.from(crypto.randomUUID()).toString('base64')`) on each request; set `Content-Security-Policy` header with `script-src 'self' 'nonce-<nonce>'` (no `'unsafe-inline'`); forward nonce to layout via `x-nonce` request header
- **apps/webmail/src/app/layout.tsx**: Import `headers` from `'next/headers'`; read `x-nonce` header into `nonce` variable; apply `nonce={nonce}` to the theme-init inline `<script>` tag
- **apps/webmail/next.config.ts** + **apps/console/next.config.ts**: Removed `scriptSrc` variable and `Content-Security-Policy` header block (now set dynamically by middleware); removed `isProduction` variable (was only used for scriptSrc); kept all other security headers unchanged
- **Acceptance criteria met**: `grep "'unsafe-inline'" apps/webmail/next.config.ts apps/console/next.config.ts` → 0 matches; nonce present in both middleware files and in webmail layout

## Task 7: Redis 기반 어드민 로그인 rate limiter (2026-05-27)

**Replace in-memory admin login rate limiter with Redis-backed distributed limiter**
- **internal/httpapi/admin_rate_limiter_redis.go** (new): `adminLoginRateLimiter` interface with `Middleware(next http.Handler) http.Handler`; `RedisAdminLoginLimiter` struct wrapping `ratelimit.RedisFixedWindowLimiter`; `NewRedisAdminLoginLimiter(client, limit, window)` constructor
- **internal/httpapi/admin_types.go**: Added `redisLoginClient *redis.Client` field to `adminRouteConfig`; added `WithRedisLoginLimiter(client *redis.Client) AdminRouteOption`
- **internal/httpapi/admin.go**: `registerAdminUtilityRoutes` now creates `loginLimiter` as `adminLoginRateLimiter` interface — uses `RedisAdminLoginLimiter` when `cfg.redisLoginClient != nil`, falls back to `AdminIPRateLimiter` (in-memory) otherwise
- **internal/httpapi/admin_auth.go**: `registerAuthAndAdminUserRoutes` parameter changed from `*AdminIPRateLimiter` → `adminLoginRateLimiter`
- **internal/app/run.go**: Appends `httpapi.WithRedisLoginLimiter(redisClient)` to `adminRouteOpts` when `redisClient != nil`
- **Test coverage**: 1274 tests pass across `internal/httpapi` and `internal/app`

## Task 6: 레거시 패스워드 해시 로그인 시 자동 업그레이드 (2026-05-27)

**Auto-upgrade legacy password hashes on successful login**
- **internal/auth/password.go**: Added `GenerateSalt(n int) []byte` (crypto/rand); added `VerifyPasswordHashResult(password, encoded string) (verified bool, needsUpgrade bool)` — returns `needsUpgrade=true` for `plain:` and `sha256:` formats
- **internal/maildb/user_auth.go**: `AuthenticateUser` now calls `VerifyPasswordHashResult` instead of `VerifyPasswordHash`; on `needsUpgrade=true` calls `upgradePasswordHash` (best-effort, non-blocking); `upgradePasswordHash` re-hashes with PBKDF2-SHA256 (210k rounds, 32-byte random salt) and updates `users.password_hash`
- **internal/auth/password_test.go**: Added `TestVerifyPasswordHashResult` (5 cases: plain match/mismatch, sha256 match/mismatch, pbkdf2 no upgrade)
- **Test coverage**: 602 tests pass across `internal/auth` and `internal/maildb`

## Task 5: JWT 내부 구현 → golang-jwt/jwt/v5 (2026-05-27)

**JWT library replacement**: Replaced hand-rolled HMAC-SHA256 JWT implementation with `golang-jwt/jwt/v5`
- **internal/auth/jwt.go**: Replaced manual `crypto/hmac`/`crypto/sha256`/base64 parsing with `jwt.ParseWithClaims` + `jwt.NewWithClaims`; public `Claims`/`TokenManager` interfaces unchanged
- Added `jwtInternalClaims` (embeds `jwt.RegisteredClaims`) as wire format; `Sign` maps `Claims` → `jwtInternalClaims`, `Verify` maps back
- Used `jwt.WithTimeFunc(m.now)` to inject mocked clock so tests using `manager.now` continue to pass
- Pre-validation retained: segment-size checks, manual header decode (alg+typ check), manual iat-future check — required by existing test suite
- `sign` unexported method kept for `signedTestToken` test helper
- Token format (HS256 HMAC-SHA256) unchanged — existing tokens remain valid
- **Test coverage**: All 47 `internal/auth` tests pass; all three extra test files pass

## Task 4: RDBMS identity provider SQL allowlist (2026-05-27)

**SQL injection prevention**: Admin-configured SQL queries validated to enforce SELECT-only operations
- **internal/idprovider/rdbms/provider.go**: Added `validateSourceQuery()` function with regex pattern to block SQL keywords (UNION, INSERT, UPDATE, DELETE, DROP, TRUNCATE, CREATE, ALTER, EXEC, EXECUTE, GRANT, REVOKE); enforces SELECT prefix, max 4096-byte limit, and allows trailing semicolon only
- **internal/idprovider/rdbms/provider.go** Connect(): Integrated query validation after db.Ping() for both UserQuery and GroupQuery; closes DB and returns error if validation fails
- **internal/idprovider/rdbms/provider_test.go**: Added TestValidateSourceQuery with 11 test cases covering empty query, valid SELECT statements (case-insensitive, leading space OK), injection patterns (UNION, INSERT, DROP with semicolon), length limits, and forbidden keywords
- **Test coverage**: 55 tests pass in rdbms package (added 11 new validation test cases)
- **Acceptance criteria met**: All 11 test cases pass; validation rejects empty/non-SELECT/injection queries; accepts valid SELECT with trailing semicolon; case-insensitive matching

## Task 3: Helm CHANGEME guard (2026-05-27)

**Helm deployment safety**: Added validation to prevent accidental deployment with placeholder secret values
- **helm/gogomail/templates/_helpers.tpl**: Added `gogomail.requireNotChangeme` helper function that validates secret values are not empty or contain "CHANGEME" placeholder; fails with descriptive error message if validation fails
- **helm/gogomail/templates/secret.yaml**: Added three guard calls before data block to validate GOGOMAIL_DM_MASTER_KEY, GOGOMAIL_AUTH_JWT_SECRET, and GOGOMAIL_ADMIN_TOKEN
- **Verification**: `helm template ./helm/gogomail` fails with "must be set to a non-placeholder value" error when using default values; `helm template` succeeds when all three secrets are set to real values via --set flags
- **Acceptance criteria met**: All three secrets fail helm template when containing CHANGEME, pass when set to real values

## Task 2: APNS private key file option (2026-05-27)

**Configuration enhancement**: Support for APNS private key from file path
- **internal/config/config.go**: Added APNsPrivateKeyFile field; Load() function reads file if path is specified, with file taking precedence over GOGOMAIL_APNS_PRIVATE_KEY env var
- **internal/config/config_file.go**: Added YAML support for apns_private_key_file configuration key
- **internal/config/validate.go**: Added validation to ensure file can be read and contains data
- **Test coverage**: Added 6 tests (TestLoadAPNsPrivateKeyFromEnvironmentVariable, TestLoadAPNsPrivateKeyFromFile, TestLoadAPNsPrivateKeyFilePathPrecedesEnvironmentVariable, TestValidateRejectsNonexistentAPNsPrivateKeyFile, TestValidateAcceptsAPNsPrivateKeyFromEnvironmentVariable, TestValidateRejectsEmptyAPNsPrivateKeyFile)
- **Impact**: Supports Kubernetes secret mounting patterns; backward compatible with existing GOGOMAIL_APNS_PRIVATE_KEY environment variable
- **Config test suite**: 368 passed (added 6 new tests)

## Task 1: Strip internal proxy headers (2026-05-27)

**Security hardening**: Strip inbound X-Gogomail-* request headers to prevent metering/billing spoofing
- **internal/httpapi/admin_middleware.go**: Added `StripInternalHeadersMiddleware` that removes 6 inbound X-Gogomail-* request headers before handler processing
- **internal/app/run.go**: Integrated StripInternalHeadersMiddleware right after RequestIDMiddleware
- **Test coverage**: `TestStripInternalHeadersMiddleware` (strips all 6 headers), `TestStripInternalHeadersMiddleware_PreservesOtherHeaders`
- **Headers stripped from requests**: X-Gogomail-Resolved-User-ID, X-Gogomail-Tenant-ID, X-Gogomail-Company-ID, X-Gogomail-Domain-ID, X-Gogomail-Principal-ID, X-Gogomail-API-Key-ID
- **Impact**: External callers cannot spoof internal metering or billing attribution headers

## Task 0: Secure defaults (2026-05-27)

**Security audit remediation**: Production-safe configuration defaults
- **internal/config/config.go** (line 371): Default GOGOMAIL_ENV changed from "development" to "production"
- **docker/docker-compose.scale.yml** (line 27): PostgreSQL sslmode default changed from "disable" to "require" (enforces encrypted connections)
- **Test suite hardening**: Added `setDevelopmentMode(t)` helper to ~40+ test functions in internal/config tests; fixed t.Parallel() conflicts by removing from tests that use t.Setenv()
- **cmd/gogomail/main_test.go**: Added GOGOMAIL_ENV environment setup to TestRunAcceptsStorageProfileConfigs, TestRunRejectsInvalidYAMLConfigBeforeAppStart, TestRunUsesAppModeEnvWhenModeFlagUnset, TestRunModeFlagOverridesAppModeEnv
- **Impact**: All 362 internal/config tests + 12 cmd/gogomail tests pass; production defaults now secure (no insecure auth allowed unless explicitly configured in development mode)

## Recent improvements (2026-05-27)

- **docker-compose.dev.yml**: `GOGOMAIL_DM_MASTER_KEY` → CHANGEME placeholder; Redis `requirepass dev-redis-password` + healthcheck updated; `GOGOMAIL_REDIS_PASSWORD` added to shared env anchor.
- **internal/app/run.go**: `context.Background()` → `context.WithoutCancel(ctx)` for OTel shutdown and HTTP graceful shutdown (Go 1.21+).
- **MessageList.tsx**: Filter/sort pipeline (`baseFiltered`, `afterLabelFilter`, `afterCategoryFilter`, `sortedBase`, `filteredMessages`, `threadCounts`, `pagedMessages`) wrapped in `useMemo`; `PAGE_SIZE`, `PULL_THRESHOLD`, `SKELETON_COUNT` extracted as module-level constants.
- **MessageRow.tsx**: Wrapped in `React.memo`; avatar `<img>` gets `loading="lazy" decoding="async"`.
- **useMailList.ts**: `foldersError` / `messagesError` states added; `.catch(() => {})` replaced with proper error capture.
- **useMessage.ts**: Module-level map cache replaced with TTL-aware cache (5 min); `error` state added.

## Recent refactoring

- Split `CalendarModals.tsx` (793 lines) into 6 files: `SubscriptionAddModal.tsx`, `CalendarManagementModal.tsx`, `EventCreateModal.tsx`, `EventEditModal.tsx`, `TodoCreateModal.tsx`, `calendarModalStyles.ts`; `CalendarModals.tsx` reduced to a 16-line re-export barrel with no logic changes.
- Extracted `useSettingsPrefs` hook from `SettingsView.tsx` (1098→563 lines); holds all 109 state variables and preference load/save effects.
- Extracted `useContactsBooks`, `useContactsEdit` hooks from `ContactsView.tsx` (723→~330 lines); hooks live in `apps/webmail/src/components/contacts/`.
- Extracted `useCalendarCreateForm`, `useCalendarEditForm`, and `useCalendarData` custom hooks from `CalendarView.tsx` (812→~340 lines); hooks live in `apps/webmail/src/components/calendar/`.
- Extracted `useInlineComposeSend` and `useInlineComposeAttachments` custom hooks from `InlineCompose.tsx` (477 lines); hooks live in `apps/webmail/src/components/reading-pane/`; pure code movement, no logic changes.
- Extracted `useDMModal`, `useMailLabels`, `useMailSession` hooks from `mail/page.tsx` (2302→2114 lines); hooks live in `apps/webmail/src/app/mail/`.
- Extracted `useMailSearch`, `useMailLayout` hooks from `mail/page.tsx` (2114→N lines); hooks live in `apps/webmail/src/app/mail/`.
- Extracted `useDriveUpload` (upload state, refs, pause/resume/cancel/scheduler) to `apps/webmail/src/components/drive/useDriveUpload.ts` and `useDriveSidebar` (folder tree state/loading) to `apps/webmail/src/components/drive/useDriveSidebar.ts`; DriveView.tsx reduced from 1412 → ~900 lines with no logic changes.
- Extracted `ComposeModal` (1309 lines) into three hooks: `useComposeDraft` (auto-save), `useComposeSlash` (slash command menu), `useComposeSend` (send lifecycle + countdown). No logic changes — pure code movement.
- Extracted `useMailThreads`, `useMailSettings`, `useMailToasts` hooks from `mail/page.tsx` (2085→1961 lines).
- Extracted `SpotlightSearch` pre-component helpers (`SpotlightItem`, `SYSTEM_ICONS`, `SCOPES`, `SpotlightT`, `sectionLabel`, `relativeTime`, `formatDriveSize`) to `apps/webmail/src/components/spotlight/spotlightHelpers.tsx`.
- Extracted `MessageList` helpers (`KO_KEYS`, `DateGroupKey`, `getDateGroup`) to `apps/webmail/src/components/message-list/messageListHelpers.ts`.
- Extracted `useReadingPaneAttachments`, `useReadingPaneMedia`, `useReadingPaneCalendar` hooks from `ReadingPane.tsx` (865→667 lines); hooks live in `apps/webmail/src/components/reading-pane/`.
- Extracted `useOrgTree` (org tree load, expand/collapse, search filtering), `useAddressBook` (address book load, contacts load, contacts search), and `useRecipients` (to/cc/bcc list management) from `OrgPickerModal.tsx` (620→457 lines); hooks live in `apps/webmail/src/components/org-picker/`.
- Extracted `useMessageListSelection` (bulk-select state, shift-click range, Escape/Ctrl+A/action-key shortcuts, row keyboard nav) and `useContactHoverCard` (hover card state + debounced timers) from `MessageList.tsx` (669→~520 lines); hooks live in `apps/webmail/src/components/message-list/`.
- Extracted `useMailCompose` (composeContext, openCompose/closeCompose, pendingCompose) and `useMailNav` (activeApp + localStorage/URL persist, activeFolderId, selectedMessageId) from `mail/page.tsx` (1961→1934 lines); hooks live in `apps/webmail/src/app/mail/`.
- Extracted `useComposeRecipients` (to/cc/bcc/from state, address loading, recent recipients) and `useComposeUI` (signature, emoji/org/send-dropdown toggles, confirmClose, imageResizeToolbar, trackOpens) from `ComposeModal.tsx` (1144→1115 lines); hooks live in `apps/webmail/src/components/compose/`.
- Extracted `useDriveNodes` (file/folder data loading, refresh, usage/quota) and `useDriveInteractions` (rename, menu, new-folder, drag/drop, selection, share) from `DriveView.tsx` (1197→1118 lines); hooks live in `apps/webmail/src/components/drive/`.

## Platform summary

GoGoMail is a production-grade self-hosted email platform written in Go
(single binary, 24 runtime modes). Key capabilities:

- **Protocols**: SMTP (inbound/outbound), IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP, JMAP (RFC 8620/8621 — all 20 methods implemented)
- **Mail security**: SPF, DKIM, DMARC, ARC, MTA-STS, TLS-RPT, DANE
- **Storage**: PostgreSQL (multi-tenant), Redis Streams (outbox), S3-compatible object storage
- **Auth**: JWT + TOTP MFA + refresh token rotation
- **Frontend**: Next.js 16 webmail SPA + admin console (TypeScript/TSX)
- **AI interface**: User MCP (124 tools) + Manage MCP (50 admin tools)
- **Monitoring**: Prometheus + Loki + Promtail + Grafana (provisioned dashboards)

## Completed milestones (2026-05)

| Date | Feature |
|------|---------|
| 2026-05-27 | Refactor(webmail): extracted pre-component helpers from apps/webmail/src/app/mail/page.tsx (lines 45–171) into mailPageHelpers.ts — 10 constants, 4 types, DM_RESIZE_HANDLES array, 10 functions; no logic changes |
| 2026-05-27 | Refactor(webmail): extract `RenderOrgTree`+`RenderOrgTreeProps` from `OrgPickerModal.tsx` into `OrgPickerTree.tsx`; extract `AVATAR_COLORS`, `avatarColor`, `initials`, `ParsedContact`, `ContactsSort`, `ContactsDensity`, `loadContactViewSettings`, `useContactsParsed` from `ContactsView.tsx` into `components/contacts/contactsViewHelpers.ts` |
| 2026-05-27 | Refactor(webmail): extract DriveView pre-component helpers (15 types/constants/functions, lines 22–320) into apps/webmail/src/components/drive/driveViewHelpers.ts; DriveView.tsx reduced by ~300 lines |
| 2026-05-27 | Refactor(webmail): extract SenderListTable from SettingsBlockedSection — blocked/allowed sender table+form JSX moved to shared SenderListTable.tsx (variant prop); SettingsBlockedSection.tsx reduced from 509→221 lines |
| 2026-05-27 | Refactor(webmail): extracted SettingsView blocked/vacation sections — SettingsBlockedSection.tsx (~280 lines: blocked senders, allowed senders, spam filter) and SettingsVacationSection.tsx (~100 lines) from SettingsView.tsx; SettingsView reduced from 1945→1465 lines |
| 2026-05-27 | Refactor: split 16 more large backend files — caldavgw/repository.go (2520→1107 lines, +3 files: repository_objects/sync/scheduling.go), storage/s3.go (2358→620 lines, +6 files: s3_request/sign/escape/validate/error/parse.go), carddavgw/repository.go (2325→1002 lines, +3 files: repository_vcard/objects/sync.go), maildb/messages.go (1876→trimmed, +messages_bulk.go), drive/repository.go (1854→trimmed, +repository_move/files.go), app/run.go (1820→trimmed, +run_workers/storage/metrics.go), httpapi/admin_helpers.go (1583→1120 lines, +admin_ldap_sync/rdbms_sync.go), carddavgw/xml.go (1476→1130 lines, +xml_filter/address_data.go), maildb/imap_copy_move.go (1444→1087 lines, +imap_expunge/uid_alloc.go), caldavgw/xml.go (1404→1116 lines, +xml_filter/calendar_data.go), httpapi/drive.go (1389→trimmed, +drive_share/download.go), dm/dm_store.go (1361→trimmed, +dm_store_keys/members/media.go), maildb/admin_api_usage_export.go (1230→deleted, +artifact/batch/manifest.go), maildb/attachment_upload_sessions.go (trimmed, +validate.go), maildb/admin_users.go (trimmed, +validate.go), dm/dm.go (1094→808 lines, +dm_attachments/membership.go), smtp/receiver.go (1092→737 lines, +receiver_auth/spool/defaults.go), config/validate.go (1082→777 lines, +validate_helpers.go); 6183 tests pass |
| 2026-05-27 | Refactor: split 5 large backend files — ldapgw/server.go (3287→745 lines, +5 files: server_pdu.go, server_search.go, server_attributes.go, server_controls.go, server_filter.go), maildb/imap_uid.go (3273→stub, +6 files: imap_mailboxes.go, imap_messages.go, imap_flags.go, imap_copy_move.go, imap_uid_backfill.go, imap_uid_helpers.go), caldavgw/handler.go (3178→308 lines, +5 files: handler_timezone/collection/objects/propfind/report.go), carddavgw/handler.go (2830→273 lines, +5 files: handler_collection/objects/propfind/report/sync.go), directory/repository.go (2922→~270 lines, +4 files: repository_aliases/search/memberships/delegations.go); 1974 tests pass |
| 2026-05-26 | Bug fix: internal/storage/s3.go — two pre-existing S3 signing bugs fixed: (1) escapeS3Segment now uses S3-strict unreserved-only encoding (was url.PathEscape which left @, =, ! unencoded, causing SignatureDoesNotMatch for paths with those chars); (2) s3PayloadHash computes actual SHA-256 body hash for HTTP endpoints (MinIO rejects UNSIGNED-PAYLOAD over plain HTTP); test infrastructure: pg_trgm installed in public schema via docker-entrypoint-initdb.d; webauthn test uses real UUID user |
| 2026-05-26 | HTTP client: webhook/urlguard.go *http.DefaultClient value copy replaced with explicit http.Client{} for clarity; timeout always set via DefaultOutboundTimeout |
| 2026-05-26 | Code structure: internal/imapgw/server_search.go (2170 lines) split into server_search.go (304), server_search_criteria.go (292), server_search_executor.go (901), server_search_match.go (692); server_fetch.go (2005 lines) split into server_fetch.go (478), server_fetch_body.go (549), server_fetch_envelope.go (1002); all files ≤1200 lines; 439 tests pass |
| 2026-05-26 | Code structure: internal/maildb/admin_api_usage.go split from 2361 lines into 5 domain files (admin_api_usage_quota.go, admin_api_usage_aggregate.go, admin_api_usage_ledger.go, admin_api_usage_retention.go, admin_api_usage_export.go); admin_api_usage.go reduced to 85 lines (shared utils); 549 tests pass |
| 2026-05-26 | Code structure: internal/mailservice/service.go split from 3415 lines into 10 domain files (service_folders.go, service_threads.go, service_search.go, service_imap.go, service_user.go, service_messages.go, service_drafts.go, service_attachments.go, service_delivery.go, service_helpers.go); service.go reduced to 244 lines; 305 tests pass |
| 2026-05-26 | Code structure: internal/httpapi/mail.go split from 3726 lines into 9 domain files (mail_auth.go, mail_folders.go, mail_messages.go, mail_threads.go, mail_drafts.go, mail_attachments.go, mail_push.go, mail_profile.go, mail_helpers.go); mail.go reduced to 502 lines; 1102 tests pass |
| 2026-05-26 | Code structure: internal/app/run.go split from 4048 lines into 10 subsystem files (run_imap.go, run_pop3.go, run_dav.go, run_ldap.go, run_scim.go, run_smtp.go, run_workers.go, run_search.go, run_push.go, run_delivery.go); run.go reduced to 1820 lines; 169 tests pass |
| 2026-05-26 | DM export: filename {ownerEmail}_{roomName}_{YYYYMMDDHHmmss}.txt; timestamps in user timezone (IANA via ?timezone= param); mail list reloads on web push (sw.js broadcasts mail_update to clients); recent search delete button in SpotlightSearch + SearchBar |
| 2026-05-26 | DM export filename UUID bug fixed: frontend now parses Content-Disposition header before creating blob URL; backend falls back to display_name (not UUID) when email is empty. Mail list double-poll race removed: additive setInterval in useMailList eliminated; single refresh() in page.tsx handles all polling. Mail reading pane: email dark/light toggle moved from body to MailActions toolbar (moon/sun icon); composeToAddress/blockSender buttons polished to production quality. DM export filename order: {roomName}_{ownerEmail}_{datetime}.txt. Reading pane: removed "메일 보내기" compose button from sender header; contact-save button remains |
| 2026-05-26 | Mail list refresh stale-closure fix: useMailList refresh() now uses useRef for both folderId and in-flight guard instead of useState — eliminates race where old closure (refreshing=false) could start a concurrent refresh between setRefreshing(true) and next render; folder-change guard discards in-flight results if folder switches mid-fetch; refresh() has empty deps and is created once per mount |
| 2026-05-26 | Inbox post-send refresh: ComposeModal gains onAfterSend prop; handleSuccessfulSend calls it immediately on success; page.tsx wires it to setTimeout(refresh, 1500) so backend has 1.5s to deliver before the inbox fetch fires — fixes inbox not updating after self-send when VAPID push is not configured |
| 2026-05-26 | Mail list rendering fix: handleRefresh now re-fetches threads in parallel alongside messages — threadViewEnabled is hardcoded true so getVisibleMailMessages always uses buildThreadMessages(threads); previous refresh() only updated messages/folders leaving threads stale, making refresh button and periodic poll appear non-functional; fix: listThreads() called on every non-virtual-folder refresh |
| 2026-05-26 | Config quality: TrustedProxyCIDRs changed from string to []string (loaded via splitCSV); Middleware/parseClientIP/isTrustedForwardingProxy accept []string, eliminating per-request splitting; validate.go gains GOGOMAIL_TRUSTED_PROXY_CIDRS and GOGOMAIL_SYSTEM_SMTP_ADDR validation |
| 2026-05-26 | Config centralization: os.Getenv removed from apikeys/middleware.go (trustedProxyCIDRs threaded as param), mailservice/systememail.go (NewSMTPSystemEmailSender replaces NewSMTPSystemEmailSenderFromEnv), httpapi/admin_auth.go (bootstrap creds via WithAdminBootstrap option); SystemEmailConfig + AdminBootstrapConfig named types added to config.Config |
| 2026-05-26 | HTTP client timeouts: replaced http.DefaultClient with 15s-timeout client in internal/sso (OIDC discovery + JWKS) and 30s-timeout client in internal/pushnotify (FCM/APNs/WebPush adapters); prevents goroutine leaks on unresponsive external services |
| 2026-05-26 | JMAP EventSource code quality: sseWriteEvent logs slog.Warn on marshal error, dead typesParam variable removed, fakeNotifier uses sync.Once for safe channel teardown, added TestEventSourcePingFormat + TestEventSourceDeliversStateChange; 62 tests pass |
| 2026-05-26 | JMAP Task 12: EventSource SSE push (RFC 8620 §7.3) — GET /jmap/eventsource/ with types/closeafter/ping params, initial state event, ping ticker, StateNotifier interface for live push, closeafter=state closes after first change; StateNotifier + Notifier field added to Deps; route registered in httpapi/jmap.go; 5 unit tests pass (60 total) |
| 2026-05-26 | FIDO2/WebAuthn MFA: go-webauthn/webauthn integration, 6 HTTP endpoints (register/begin, register/complete, authenticate/begin, authenticate/complete, credentials GET/DELETE), credential + challenge store, DB migration 0155 |
| 2026-05-26 | JMAP identity code quality: removed dead `wanted` map, create/destroy paths now return serverFail on SetWebmailPreferences error, destroy loop restructured to load-once/save-once eliminating read-modify-write race, update stub returns notImplemented instead of silent success |
| 2026-05-26 | JMAP auth hardening: userIDFromBearer now calls VerifyFull(ctx,token) instead of Verify(token) — session revocation enforced on every authenticated request; TestServeSessionRequiresAuth rewritten with real TokenManager asserting HTTP 401 |
| 2026-05-26 | Error wrapping: bare `return err` wrapped with fmt.Errorf context in internal/jmap (email_set.go — 5 occurrences in applyEmailPatch) and internal/mailservice/service.go exported methods (DeleteFolder, UnsubscribeIMAPMailboxName, DeleteIMAPMailbox, SetMessageFlag, MoveMessage, DeleteMessage, RestoreMessage, DeletePushDevice, DeleteDraft — 16 occurrences); 367 tests pass |
| 2026-05-26 | JMAP RFC compliance: Identity replyTo/bcc now marshal as null (not omitted), Identity/set destroy tracks and persists removals to preferences, SearchSnippet/get accepts filter param per RFC 8621 §7.1 |
| 2026-05-26 | JMAP code quality (EmailSubmission/VacationResponse): removed dead second GetMessage call post-SendDraft, json.Marshal errors handled for submission encoding, GetWebmailPreferences errors return notUpdated instead of silently using defaults, json.Marshal errors handled for vrRaw/newPrefs, proper JMAP patch semantics (RFC 8620 §3.3 key-iteration merge, leading-slash stripping); 55 tests pass |
| 2026-05-26 | JMAP RFC compliance (EmailSubmission/VacationResponse): identityId required validation, full RFC 8621 §7.2 submission fields (threadId, mailboxIds, envelope, sendAt RFC3339, undoStatus, deliveryStatus, dsnBlobIds, mdnBlobIds), newState diverges from oldState on create/update, VacationResponse/get ids filter (non-singleton → notFound), VacationResponse/set newState diverges on update; 53 tests pass |
| 2026-05-26 | JMAP Task 11: EmailSubmission/set, VacationResponse/get + /set — DraftSender interface in jmap package, EmailSubmission/set verifies draft exists then calls Sender.SendDraft (nil Sender → serverFail), VacationResponse stored in webmail preferences under `vacationResponse` key (singleton; create/destroy → forbidden), VacationResponse/set merges patch into stored struct; Sender DraftSender field added to Deps; 3 new methods registered in NewHandler; 4 unit tests pass (53 total) |
| 2026-05-26 | JMAP Task 10: Identity/get, Identity/set, SearchSnippet/get — primary identity from users table, custom identities stored in webmail preferences JSON under `identities` key, SearchSnippet/get fetches subject+TextBody preview via GetMessage; 4 unit tests pass |
| 2026-05-26 | JMAP Task 9: Email/changes, Email/copy, Email/import, Email/parse — modseq-based change tracking from imap_message_uid, MIME header parsing for Email/parse, Email/copy and Email/import stubs with proper RFC error types; 4 unit tests pass |
| 2026-05-26 | JMAP Task 8: Email/set — applyEmailPatch (keywords/$seen/$flagged/$draft, mailboxIds patch semantics), DeleteMessage for destroy, SaveDraft for create; 5 unit tests pass |
| 2026-05-26 | JMAP Task 7: Email/query, Email/queryChanges — text filter routes to SearchMessages, list filter routes to ListMessagesPage with keyword/flag mapping ($seen/$flagged); Email/queryChanges returns cannotCalculateChanges; 4 unit tests pass |
| 2026-05-26 | JMAP Task 6: Email/get real DB integration — messageDetailToJMAP property filtering, flagsToKeywords ($seen/$flagged/$draft), parseJMAPAddrs, body values/parts, requestTooLarge enforcement; 4 unit tests pass |
| 2026-05-26 | JMAP Task 5: Thread/get, Thread/changes — threadGetMethod calls ListThreadMessagesPage per threadID, notFound for missing threads; threadChangesMethod returns empty created/updated/destroyed with current modseq state; 3 unit tests pass |
| 2026-05-26 | JMAP Task 4: Mailbox/get, /query, /set, /changes — folderToMailbox conversion, JMAP role mapping (inbox/sent/drafts/trash/spam→junk/archive), CreateFolder/RenameFolder/DeleteFolder wired, conservative Mailbox/changes returning all IDs when state changes; 6 unit tests pass |
| 2026-05-26 | JMAP Task 3: State strings + blob upload/download — EmailStateFor/MailboxStateFor (modseq/session_version based), POST /jmap/upload, GET /jmap/download, jmap_blobs migration 0156; hardened error handling (DB insert failure returns 500, ErrNoRows-only fallback, Content-Disposition header per RFC 8620 §6.2) |
| 2026-05-26 | JMAP Task 2: BackReference resolution RFC 8620 §3.7 — resolveBackRefs/walkPath in internal/jmap/backref.go; wildcard (/list/*/id) and index (/list/0/id) path support; missing callID returns invalidResultReference error; wired into ServeAPI dispatch loop with prevResults map; 4 unit tests pass |
| 2026-05-26 | JMAP Task 1: Handler Deps + JWT auth + request validation — Deps{Repo,Store,Auth} struct, JWT Bearer auth on ServeSession/ServeAPI (401 when Auth set and token missing/invalid), using array validation (400 unknownCapability), maxCallsInRequest=16 (400 requestTooLarge), Method.Call now typed context.Context; 8 unit tests pass |
| 2026-05-26 | JMAP RFC 8620/8621 foundation: /.well-known/jmap session resource, POST /jmap/api dispatch, Email/get + Email/query stubs; 5 unit tests pass |
| 2026-05-26 | Refactor: split `internal/maildb/admin.go` (7579 lines) into 8 focused files — admin_users.go, admin_domains.go, admin_relay.go, admin_api_usage.go, admin_push.go, admin_outbox.go, admin_delivery.go, admin_suppression.go — admin.go reduced to 1498 lines (types/consts/validation); 549 tests pass |
| 2026-05-26 | Refactor: split `internal/imapgw/server.go` (9654 lines) into 13 focused files — server_conn.go, server_auth.go, server_capabilities.go, server_mailbox.go, server_list.go, server_idle.go, server_uid.go, server_search.go, server_fetch.go, server_store.go, server_copy_append.go, server_parse.go, server_dispatch.go — server.go reduced to 802 lines; 439 tests pass |
| 2026-05-26 | K8s deployment: 8 manifests (namespace, configmap, secret template, deployment, service, HPA, PDB, ingress) + README in `k8s/` |
| 2026-05-26 | DM room key rotation: `POST /api/v1/dm/rooms/{roomID}/rotate-key` — generates new AES-256-GCM key, atomically re-encrypts all message bodies and attachment paths |
| 2026-05-26 | Frontend cleanup: removed 63 console.log/error/warn calls from 29 console admin page files |
| 2026-05-26 | Global HTTP body limit already enforced via `MaxRequestBodyMiddleware(4MB)` applied to all routes |
| 2026-05-26 | Outbox relay horizontal scaling: WorkerCount (N goroutines per process, SKIP LOCKED prevents double-claim); ShardedPostgresStore (hashtext(partition_key) % N shard filter for partition-ordered multi-process scaling); config GOGOMAIL_OUTBOX_RELAY_{WORKER_COUNT,SHARD_TOTAL,SHARD_INDEX}; 24 tests pass |
| 2026-05-26 | Webmail unit tests: 3 new node --experimental-strip-types test scripts (check-compose-utils, check-send-result-label, check-stableid-timezone); pnpm test now runs all 6 scripts; composeUtils.ts import changed to `import type` |
| 2026-05-26 | OpenTelemetry tracing: `internal/observability/tracing.go` — TracerProvider, InitTracing (OTLP HTTP exporter), OTelHTTPMiddleware, StartSpan; config via GOGOMAIL_OTEL_{ENABLED,ENDPOINT,SERVICE_NAME,SERVICE_VERSION}; HTTP middleware wired into runHTTP + runOutboxRelay; 9 tests pass |
| 2026-05-26 | CI integration test stage: `docker/docker-compose.ci.yml` (Postgres 16, Redis 7, MinIO with tmpfs), `go-integration-test` job in CI runs full `go test ./...` against live services with `GOGOMAIL_TEST_*` env vars |
| 2026-05-26 | Delivery rate limit on by default (60/min), Redis-backed cross-process limiter (`RedisDomainRateLimiter`); backend selectable via `GOGOMAIL_DELIVERY_RATE_LIMIT_BACKEND` |
| 2026-05-26 | SMTP rate limiting per recipient domain: `InMemoryDomainRateLimiter` (fixed-window, per-minute), handler integration via `WithRateLimiter`, config via `GOGOMAIL_DELIVERY_RATE_LIMIT_*` env vars |
| 2026-05-26 | DM search scalability: paginated full-history scan (removed 1000-msg hard cap); search now iterates all room history in 200-msg pages until results found or history exhausted |
| 2026-05-26 | DM room export: TXT download from room header ⋯ menu (any participant; includes deleted/system messages); User MCP `gogomail_dm_export_room` tool (124 total) |
| 2026-05-26 | Doc optimization: deleted 12 completed/stale docs, trimmed backend-roadmap.md 7057→110 lines, removed 18 stale worktrees (1.4GB), freed 174MB from .git via gc |
| 2026-05-26 | JMAP session URL fix: APIUrl/DownloadUrl/UploadUrl/EventSourceUrl now point to /jmap/* (was /.well-known/jmap/* — 404 on all client operations); README.md and README.ko.md updated with JMAP client section |
| 2026-05-26 | JMAP integration: nil-safe DraftSender guard in jmapHandler (svc==nil → Sender=nil → graceful serverFail instead of panic) |
| 2026-05-26 | JMAP Task 13: Integration complete — DraftSender adapter wired (mailservice.Service→jmap.DraftSender), submission/vacationresponse capabilities in session resource; all 20 JMAP methods live |
| 2026-05-26 | JMAP hardening: replaced panic in mustRawString with safe strconv.AppendQuote fallback (rawString function); prevents HTTP handler goroutine death on encoding edge case; 62 internal/jmap tests pass |
| 2026-05-26 | VitePress AI Automation MCP guide (12 pages × 4 locales), expanded User MCP READMEs 147→1024 lines |
| 2026-05-26 | Codebase improvements (10 tasks): doc cleanup, security hardening, TypeScript domain splits (MCP tools, webmail API, UI components), Go package refactoring (httpapi/admin.go 8901→420 lines + 12 files, app/admin_service.go 1759→93 lines + 5 files) |
| 2026-05-25 | DM search candidate limit 10000→1000; system messages injectable |
| 2026-05-25 | Grafana default password removed; metrics interface{} replaced with typed interfaces |
| 2026-05-24 | User MCP DM tools (18 tools): rooms, messages, attachments, reactions, search |
| 2026-05-23 | Web push notifications + push device management |
| 2026-05-23 | Monitoring stack: Prometheus, Loki, Promtail, Grafana with provisioned dashboards |
| 2026-05-27 | Refactor: SettingsModalContent.tsx (568 lines) split into 9 sub-components under settings-modal/: MailboxSection, ComposeSection, ThemeSection, NotificationsSection, AccountSection, SecuritySection, ShortcutsSection, AdvancedSection, FiltersSection + shared sharedStyles.ts; root file reduced to 68 lines |
| 2026-05-27 | Refactor: SettingsView account/inbox/reading/compose/appearance/accessibility/contacts/drive/shortcuts extracted to separate components (1465 → 1098 lines) |
| 2026-05-22 | Admin console MFA enforcement |
| 2026-05-21 | Multiple query sargability improvements, LDAP batching, i18n cleanup |
| 2026-05-14 | DM instant messaging: rooms, encrypted messages, attachments, reactions, invites, Drive messages |
| 2026-05-10 | Web Push service worker, calendar edit/delete, password reset UI, server-synced signatures |
| 2026-05-07 | User MCP server: 123 tools across mail, DM, contacts, Drive, calendar, preferences |
| 2026-05-05 | Manage MCP server: 50 admin tools for Admin API, queue/health, org/security/spam policies |

## Architecture

See `docs/ARCHITECTURE.md` for the architecture overview.
See `docs/backend-roadmap.md` for the full feature roadmap.
See `docs/openapi.yaml` for the REST API spec.
See `PROJECT_HARNESS.md` for development workflow.

## Post-remediation hardening (2026-05-27)

**Three residual defects found in security branch and fixed:**
- **internal/httpapi/admin_rate_limiter_redis.go**: Redis rate limiter now fail-open on Redis error (warn log + allow) instead of fail-closed (429 for all logins)
- **internal/config/config.go**: APNS key-file loading now clears env-var value before file read — missing/unreadable file can no longer silently fall back to inline env var
- **internal/maildb/user_auth.go**: Legacy password hash upgrade now runs in a goroutine (`context.WithoutCancel`) — no longer blocks login response with 210k PBKDF2 iterations
