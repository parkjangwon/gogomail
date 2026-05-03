# gogomail backend roadmap

## Phase 0: backend foundation

- Single Go binary: `gogomail --mode=<component>`
- Environment-based config loader
- Liveness/readiness HTTP endpoints
- Mail address normalization utility
- Test-first baseline

## Phase 1: receive and read mail

Target outcome:

> SMTP로 메일을 넣으면 원문이 저장되고, REST API로 메일 목록/상세를 조회할 수 있다.

Shared mail parsing lives in `internal/message` so SMTP, Mail API, future IMAP, and future POP3 use the same RFC 5322/MIME parsing behavior.

SMTP receive exposes explicit pipeline hook stages:

- `backpressure_checked`
- `spooled`
- `parsed`
- `dedup_checked`
- `stored`
- `recorded`

Future modules such as spam relay, image conversion, FCM/push enqueue, audit logging, attachment scanning, and indexing should attach to these explicit stages instead of being hard-coded into the SMTP engine.

SMTP receive policy is explicit and should grow as the receive boundary matures:

- max recipients per message
- max message bytes
- future per-IP/per-domain rate limits
- future queue/backpressure checks

Received mail persistence follows the Outbox Pattern:

- Store `.eml` first through the storage backend.
- Insert `messages` metadata and a `mail.event` / `mail.stored` outbox row in one PostgreSQL transaction.
- `gogomail --mode=outbox-relay` claims pending outbox rows and publishes them to Redis Streams.
- Redis stream consumers route events by payload `event` name so optional modules can plug in their own handlers.
- `gogomail --mode=event-worker` consumes `mail.event` and records `mail.stored` as `mail.received` audit logs.
- Keep indexing, push notifications, audit fan-out, and queue publishing asynchronous consumers of the outbox/event stream.

Implementation order:

1. PostgreSQL migrations for company/domain/user/address/folder/message.
2. Storage backend interface with local and Minio implementations.
3. SMTP receive path using `go-smtp`.
4. Recipient validation against `user_addresses`.
5. `.eml` persistence, shared EML parsing, and `MessageRecorder` metadata handoff.
6. PostgreSQL-backed recorder for `messages` insert.
7. Redis SET NX duplicate detection.
8. PostgreSQL outbox event creation for stored mail.
9. Mail API list/detail/folder endpoints.
10. RFC 5322/MIME outbound text composer and outbound farm classifier.
11. Delivery worker consumes `mail.outbound.general` and hands queued `.eml` messages to a pluggable SMTP transport.
12. Delivery attempts are recorded for delivered/failed recipients as retry and bounce groundwork.
13. Failed delivery attempts can schedule delayed retries through outbox `available_at`.
14. SMTP 4xx/5xx responses are classified so 5xx hard bounces stop retry while temporary failures retry.
15. Hard-bounced recipients are added to suppression list and blocked before future send enqueue.
16. Delivery outcomes emit `mail.delivered`, `mail.bounced`, or `mail.delivery_failed` events for audit and future admin streams.
17. Admin API exposes queue stats, delivery attempts, and suppression list read models.
18. Admin API can retry outbox events and remove suppression entries.
19. Admin API supports optional bearer/admin-token protection through `GOGOMAIL_ADMIN_TOKEN`.
20. Mail API supports optional HS256 JWT verification through `GOGOMAIL_AUTH_JWT_SECRET`.
21. SMTP sessions support AUTH PLAIN hooks and optional auth-required mode for future Submission MTA.
22. `outbound-mta` runs an authenticated SMTP Submission server that accepts client mail, verifies the envelope sender belongs to the authenticated user, stores the raw `.eml`, and records it through the existing outbound queue/outbox path.
23. Submission TLS policy is configurable: STARTTLS uses configured certificate/key files, insecure AUTH is convenient in development but disabled by default when `GOGOMAIL_ENV=production`.
24. Submission SMTP exposes explicit hook stages (`authenticated`, `mail_from`, `rcpt`, `spooled`, `parsed`, `stored`, `recorded`) so DKIM signing, image conversion, audit fan-out, indexing, and notification enqueue can attach without being hard-coded into the protocol engine.
25. Delivery SMTP supports a streaming message transform chain before DATA, providing the insertion point for DKIM signing, custom headers, tenant policy transforms, and future content processing without loading the whole `.eml` into memory by default.
26. DKIM has a dedicated delivery transformer boundary so a standards-compliant signer can be plugged into the pre-DATA transform chain without coupling cryptographic signing to SMTP transport code.
27. DKIM key metadata is persisted per domain with admin APIs for list/create/deactivate; private keys are intentionally omitted from list views and reserved for signer-only repository access.
28. Delivery worker can enable DKIM signing with `GOGOMAIL_DKIM_ENABLED=true`; it looks up the active domain key and plugs the RFC 6376 signer into the pre-DATA transform chain.
29. Direct outbound SMTP supports STARTTLS policy via `GOGOMAIL_DELIVERY_TLS_MODE` (`opportunistic`, `require`, `disable`), defaulting to opportunistic MTA-to-MTA TLS.
30. SMTPUTF8 is intentionally not advertised and `MAIL FROM SMTPUTF8` is rejected until full RFC 6531/6532 address/header/storage support is implemented, preventing accidental partial EAI behavior.
31. Shared EML parsing now caps extracted text body bytes by default and exposes `TextBodyTruncated`, keeping metadata parsing allocation-bounded for large messages while preserving raw `.eml` storage.
32. SMTP receive/submission hot paths parse headers, addresses, and attachment metadata with `SkipTextBody`, deferring body preview extraction to read/search paths to reduce unnecessary allocations.
33. EML parsing caps collected attachment metadata and reports `AttachmentsTruncated`, preventing pathological MIME part counts from growing metadata memory unbounded.
34. Edge MTA prepends an RFC-shaped `Received` trace header before parsing/storing accepted mail, while tests can keep it disabled for raw storage assertions.
35. Submission MTA also prepends a trace header using `with ESMTPA`, preserving authenticated client submission provenance before outbound queueing.
36. Inbound messages without RFC `Message-ID` get a deterministic internal fallback ID for deduplication and metadata consistency.

## Deferred until backend contracts stabilize

- Next.js shell/webmail/admin apps
- Kafka
- OpenSearch
- etcd
- Vault
- IMAP
- Push notifications
