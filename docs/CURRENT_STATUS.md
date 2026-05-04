# gogomail current status

Last updated: 2026-05-04 (updated after platform hardening sprint)

## Current phase

gogomail is in the backend platform hardening phase.

The project has moved beyond SMTP-only development. SMTP remains a critical
RFC-sensitive core, but current work should balance:

- tenant/domain operations
- Admin API
- Mail API contracts
- delivery routing and observability
- DNS/DKIM/domain onboarding
- quota and policy enforcement
- OpenAPI drift prevention

Actual Next.js frontend implementation has not started. Before creating or
substantially implementing frontend apps, ask the user for frontend-specific
guidance.

## Completed or materially advanced

- SMTP receive engine with real TCP integration coverage.
- Authenticated Submission MTA with STARTTLS and SMTPS support.
- Outbound SMTP delivery with direct MX, smart-host, TLS policy, retry, and
  partial recipient failure handling.
- DSN/bounce handling with RFC 3461/3464-oriented metadata, null reverse-path,
  `NOTIFY=NEVER`, deterministic outbox dedupe, and loop-risk reduction.
- Shared high-performance-minded EML parsing boundary under `internal/message`.
- PostgreSQL metadata model for companies, domains, users, folders, messages,
  attachments, outbox, audit logs, DKIM keys, trusted relays, delivery routes,
  domain DNS checks, and policy-bearing domain settings.
- Admin APIs for domains, users, quotas, DKIM keys, trusted relays, delivery
  routes, delivery route resolution, queue stats, delivery attempts,
  suppression list, quota usage, domain DNS checks/history, backpressure
  inspection/update, domain policy, per-domain stats, DKIM DNS verification,
  delivery route runtime counters, and exhausted delivery attempts.
- Mail APIs for folders, messages, flags, bulk operations, drafts, send, and
  attachments, thread lists/thread messages, plus user-scoped sent-message
  delivery/bounce status.
- Inbound parsing now extracts RFC `In-Reply-To`/`References`; inbound and
  reply/forward outbound persistence inherit local thread IDs when matching
  source messages exist.
- Reply composition writes RFC `In-Reply-To`/`References` headers into outgoing
  `.eml` messages.
- Mail API exposes a first Postgres-backed search endpoint for message metadata
  and draft text, with an FTS index for small deployments.
- Mail API send/draft-send applies domain outbound policy in enforce mode for
  recipient-count and composed-message-size guardrails.
- Per-domain inbound policy enforced at SMTP receive and Submission MTA (max
  recipients, max message size, inbound mode).
- Hierarchical quota ledger enforced at mail storage write/delete boundaries:
  company, domain, and user usage counters are updated atomically in the same
  PostgreSQL transaction. User quota source is tracked as `default|custom`, and
  domain default user quota updates apply to default-following users while
  preserving custom overrides.
- Product quota direction is company pool → domain allocation → user unified
  storage allowance. User quota should cover mailbox, attachments, future Drive,
  and other user-owned storage features.
- API metering direction is agreed for future SaaS operations: collect usage
  dimensions early through an async middleware/event boundary, but keep
  billing/rate-limit enforcement policy-driven and disabled by default.
- DKIM key DNS verification workflow with `dns_verified_at` persistence.
- Delivery route runtime counters (`RouteCounters`) with Admin API exposure.
- Retry exhaustion hook: `mail.delivery_exhausted` outbox event emitted and
  `delivery_attempts` row with status `exhausted` written when all retries fail.
- DMARC reject policy enforcement at SMTP receive (`DMARCEnforce` flag).
- SMTPUTF8 declared correctly on outbound MAIL FROM for all internationalized
  addresses (RFC 6531 compliance fix).
- OpenAPI draft with route, request body, response envelope, operationId, and
  component reference drift tests.  All schemas kept in sync with Go types.
- Backend release verification script and SMTP release runbook.
- Public GitHub repository:
  <https://github.com/parkjangwon/gogomail>

## Explicitly not started

- Next.js shell/webmail/admin frontend implementation.
- Built-in spam scoring or pattern filtering.
- IMAP/POP3.
- OpenSearch indexing.
- Kafka migration.
- etcd/Vault production control plane.
- Push notification worker.

## Important guardrails

- Implemented SMTP features must strictly follow the relevant email RFCs.
- Do not advertise SMTP extensions until end-to-end semantics are implemented
  and tested.
- Do not turn SMTP core into a spam engine. Spam relay/filtering belongs behind
  explicit hooks/adapters.
- Keep hot paths streaming and allocation-aware.
- Preserve domain-as-tenant isolation.
- Commit by feature and push after completed work.

## Latest direction

The platform hardening sprint completed the following:

- Mailbox quota enforcement (receive, send, delete)
- Per-domain SMTP inbound policy (max recipients, max message size)
- DKIM DNS verification workflow
- Delivery route runtime counters
- Retry exhaustion events and Admin API exposure
- SMTPUTF8 outbound RFC 6531 fix
- DMARC reject policy enforcement hook
- Domain aggregate stats endpoint
- OpenAPI schema expansion (DKIMKey, DeliveryAttempt, DKIMKeyDNSVerification)
- Hierarchical quota ledger first implementation: company/domain/user Admin
  quota APIs, user quota source, domain default user quota propagation, and
  aggregate quota enforcement for mail writes/deletes.

Next focus areas:

1. Search indexing boundary for received message bodies and future OpenSearch.
2. Extend hierarchical quota ledger to attachments and add reconciliation jobs.
3. IMAP gateway design and implementation planning.
4. Search result highlighting/ranking once indexing boundary exists.
5. Push notification hook for FCM/APNs (pluggable pipeline stage).
6. API metering middleware boundary for future SaaS billing/abuse analytics.
7. Frontend planning and API contract review before webmail implementation.
