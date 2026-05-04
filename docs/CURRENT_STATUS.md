# gogomail current status

Last updated: 2026-05-04

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
  suppression list, quota usage, domain DNS checks/history, and domain policy.
- Mail APIs for folders, messages, flags, bulk operations, drafts, send, and
  attachments, plus user-scoped sent-message delivery/bounce status.
- OpenAPI draft with route, request body, response envelope, operationId, and
  component reference drift tests.
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

Focus on turning the backend into a releasable webmail service platform:

1. Apply quota and policy enforcement at SMTP, Submission, Mail API, and
   delivery boundaries.
2. Apply runtime quota/policy enforcement at SMTP, Submission, Mail API, and
   delivery boundaries.
3. Strengthen domain onboarding with DKIM verification workflows and admin
   remediation UX contracts.
4. Improve Admin API observability for queue, delivery routes, and backpressure.
5. Keep OpenAPI and implementation synchronized for future generated clients.
