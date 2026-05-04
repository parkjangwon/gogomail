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
- Mail API send/draft-send applies domain outbound policy in enforce mode for
  recipient-count and composed-message-size guardrails.
- Per-domain inbound policy enforced at SMTP receive and Submission MTA (max
  recipients, max message size, inbound mode).
- Mailbox quota enforced at SMTP receive, Submission, and delete flows.
  Quota is decremented atomically on delete.
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

Next focus areas:

1. Search indexing readiness (Postgres FTS first, OpenSearch adapter later).
2. Message thread assignment on receive/send using RFC `References` and
   `In-Reply-To`.
3. IMAP gateway design and implementation planning.
4. Push notification hook for FCM/APNs (pluggable pipeline stage).
5. Attachment scanning hook (pluggable pipeline stage).
6. Frontend planning and API contract review before webmail implementation.
