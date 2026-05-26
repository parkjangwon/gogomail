# gogomail current status

Last updated: 2026-05-26

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
| 2026-05-26 | Code structure: internal/imapgw/server_search.go (2170 lines) split into server_search.go (304), server_search_criteria.go (292), server_search_executor.go (901), server_search_match.go (692); server_fetch.go (2005 lines) split into server_fetch.go (478), server_fetch_body.go (549), server_fetch_envelope.go (1002); all files ≤1200 lines; 439 tests pass |
| 2026-05-26 | Code structure: internal/maildb/admin_api_usage.go split from 2361 lines into 5 domain files (admin_api_usage_quota.go, admin_api_usage_aggregate.go, admin_api_usage_ledger.go, admin_api_usage_retention.go, admin_api_usage_export.go); admin_api_usage.go reduced to 85 lines (shared utils); 549 tests pass |
| 2026-05-26 | Code structure: internal/mailservice/service.go split from 3415 lines into 10 domain files (service_folders.go, service_threads.go, service_search.go, service_imap.go, service_user.go, service_messages.go, service_drafts.go, service_attachments.go, service_delivery.go, service_helpers.go); service.go reduced to 244 lines; 305 tests pass |
| 2026-05-26 | Code structure: internal/httpapi/mail.go split from 3726 lines into 9 domain files (mail_auth.go, mail_folders.go, mail_messages.go, mail_threads.go, mail_drafts.go, mail_attachments.go, mail_push.go, mail_profile.go, mail_helpers.go); mail.go reduced to 502 lines; 1102 tests pass |
| 2026-05-26 | Code structure: internal/app/run.go split from 4048 lines into 10 subsystem files (run_imap.go, run_pop3.go, run_dav.go, run_ldap.go, run_scim.go, run_smtp.go, run_workers.go, run_search.go, run_push.go, run_delivery.go); run.go reduced to 1820 lines; 169 tests pass |
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
