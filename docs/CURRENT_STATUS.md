# gogomail current status

Last updated: 2026-05-26

## Platform summary

GoGoMail is a production-grade self-hosted email platform written in Go
(single binary, 24 runtime modes). Key capabilities:

- **Protocols**: SMTP (inbound/outbound), IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP, JMAP (RFC 8620/8621 foundation)
- **Mail security**: SPF, DKIM, DMARC, ARC, MTA-STS, TLS-RPT, DANE
- **Storage**: PostgreSQL (multi-tenant), Redis Streams (outbox), S3-compatible object storage
- **Auth**: JWT + TOTP MFA + refresh token rotation
- **Frontend**: Next.js 16 webmail SPA + admin console (TypeScript/TSX)
- **AI interface**: User MCP (124 tools) + Manage MCP (50 admin tools)
- **Monitoring**: Prometheus + Loki + Promtail + Grafana (provisioned dashboards)

## Completed milestones (2026-05)

| Date | Feature |
|------|---------|
| 2026-05-26 | FIDO2/WebAuthn MFA: go-webauthn/webauthn integration, 6 HTTP endpoints (register/begin, register/complete, authenticate/begin, authenticate/complete, credentials GET/DELETE), credential + challenge store, DB migration 0155 |
| 2026-05-26 | JMAP auth hardening: userIDFromBearer now calls VerifyFull(ctx,token) instead of Verify(token) — session revocation enforced on every authenticated request; TestServeSessionRequiresAuth rewritten with real TokenManager asserting HTTP 401 |
| 2026-05-26 | JMAP Task 3: State strings + blob upload/download — EmailStateFor/MailboxStateFor (modseq/session_version based), POST /jmap/upload, GET /jmap/download, jmap_blobs migration 0156 |
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
