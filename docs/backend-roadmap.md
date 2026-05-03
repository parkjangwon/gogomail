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
37. The generated fallback Message-ID is inserted into the stored raw message, after leading Received trace headers when present, so downstream consumers see a complete RFC 5322 identity.
38. Submission mail without Message-ID gets a server-generated RFC 5322 Message-ID inserted into raw storage and outbound metadata.
39. Unsupported SMTP extensions are explicitly guarded: `REQUIRETLS`, DSN (`RET`, `ENVID`, `NOTIFY`, `ORCPT`), and `BINARYMIME` are rejected until their full end-to-end semantics are implemented.
40. SMTP session state now enforces MAIL-before-RCPT, resets envelope state after successful DATA, and clears receiver authentication on logout.
41. Queued delivery payloads validate RFC-shaped sender/recipient addresses, normalize them before transport, and deduplicate recipients before SMTP delivery attempt records are created.
42. Direct outbound SMTP now evaluates all MX candidates in priority order, fails over across hosts on transient connection/delivery errors, and treats RFC 7505 null MX as a permanent non-deliverable domain.
43. Delivery retry scheduling uses deterministic per-message jitter and max-delay caps so large retry waves spread out predictably without shared random state.
44. Delivery retry policy is configurable through environment variables for delay schedule, jitter ratio, and maximum delay so operators can tune small and large deployments without code changes.
45. Direct outbound SMTP applies a full-session deadline across connect, SMTP commands, STARTTLS, DATA streaming, and QUIT, with `GOGOMAIL_DELIVERY_TIMEOUT` for operator tuning.
46. SMTP receive now has a pluggable authentication verification boundary for SPF/DKIM/DMARC-style results, including a dedicated `authentication_checked` hook stage and result propagation to hook events and recorded messages.
47. SMTP receive exposes a lightweight metrics interface and emits accepted/rejected observations for MAIL, RCPT, and completed DATA/recorded transactions so production deployments can plug in Prometheus/OpenTelemetry adapters without coupling metrics to protocol logic.
48. Submission MTA uses the same metrics boundary for AUTH, MAIL, RCPT, and completed DATA/recorded transactions, keeping authenticated client submission observable without coupling it to a specific monitoring vendor.
49. Delivery worker now exposes its own metrics boundary for queued payload decode, transport delivery/failure/bounce, retry scheduling, and retry exhaustion, keeping outbound SMTP operations observable at scale.
50. Submission MTA now shares the normalized SMTP receive policy for max recipients and max message bytes, preventing authenticated clients from bypassing recipient-count guardrails.
51. SMTP receive and Submission MTA max-recipient/max-message-size policies are configurable through environment variables, allowing operators to tune abuse guardrails independently for inbound and authenticated submission traffic.
52. SMTP receive and Submission MTA protocol capability toggles are environment-configurable, keeping partially implemented extensions disabled by default while allowing controlled test deployments to enable them explicitly.
53. Inbound SMTP can run a real authentication verifier boundary for SPF, DKIM, and DMARC: SPF evaluates DNS TXT mechanisms (`ip4`, `ip6`, `include`, `a`, `mx`, `redirect`, `all`), DKIM verifies raw RFC 5322 messages through `go-msgauth`, and DMARC evaluates SPF/DKIM alignment against discovered policy.
54. Accepted inbound messages verified by the authentication boundary get an RFC-shaped `Authentication-Results` header inserted into stored `.eml`, giving audit, spam, indexing, and admin surfaces a durable standards-based signal.
55. DMARC enforcement is an optional hook (`monitor`, `quarantine`, `reject`) rather than hard-coded SMTP behavior, preserving local policy flexibility while allowing strict public-sector deployments to reject spoofed mail.
56. Spam relay integration has a dedicated hook adapter with accept/quarantine/reject/tempfail verdicts and shadow mode, so Rspamd/SpamAssassin/custom engines can attach at explicit SMTP stages without contaminating protocol code.
57. Bounce infrastructure now includes VERP return-path generation/parsing so delivery attempts can map future DSNs back to original recipients and message tokens without stateful heuristics.
58. Delivery Status Notification composition now emits multipart/report `message/delivery-status` payloads with reporting MTA, original envelope id, final recipient, action, enhanced status, remote MTA, diagnostic code, and last-attempt metadata.
59. Delivery workers expose an in-memory farm/domain concurrency throttler that can defer work by outbound farm and recipient domain, preparing the worker for safe large-provider delivery ramps.
60. Delivery throttling is runtime-configurable with default, farm, and domain concurrency limits, keeping small deployments simple while giving large operators per-destination control.
61. SMTP and delivery metrics interfaces have a concrete slog observability adapter and `GOGOMAIL_METRICS_BACKEND=slog` wiring, making protocol and worker behavior inspectable without forcing a vendor dependency.
62. Runtime configuration is validated at process startup for unsafe production auth, TLS file pairing, enum values, positive SMTP limits, retry jitter bounds, and unusable throttling settings.
63. SMTP hook events now carry the remote address through the result/event carrier, allowing optional external spam relay adapters to make policy calls without adding spam logic to SMTP core.
64. Mail API now supports folder listing, folder-scoped message lists, user folder create/rename/delete, message flag updates, message moves, soft deletes, attachment listing, attachment download, and no-store private attachment responses.
65. Message detail responses include attachment metadata, and folder list responses include total/unread counts for webmail state rendering.
66. Admin API now exposes domain and user listing read models alongside existing queue, delivery attempt, suppression, and DKIM key operations.
67. Mail API read paths have dedicated folder/message/attachment indexes to keep webmail list/detail operations efficient as mailbox volume grows.
68. Mail list responses now expose a stable page envelope with limit, has_more, and opaque next_cursor fields, keeping webmail list state deterministic while preserving the existing `messages` response key.
69. Compose/send contracts now distinguish new, reply, and forward intents; reply/forward sends require a source message and mark the source as answered/forwarded after successful queueing.
70. Draft save/update/delete and attachment-upload metadata HTTP contracts are now present, establishing the backend surface for webmail compose autosave and attachment linking.
71. Folder count responses now include starred counts, and Admin API can update user/domain status for basic operational management.
72. Draft save/update/delete now persists to PostgreSQL-backed `messages` rows with dedicated draft state, source message references, draft body storage, and Drafts folder creation.
73. Attachment upload metadata now persists to PostgreSQL with generated upload IDs, storage paths, user ownership, draft binding, and attachment metadata returned after draft saves.
74. Reply/forward source state is persisted for both drafts and sent messages, with ownership checks before source references are recorded.
75. Message list cursors now drive PostgreSQL seek pagination with limit-plus-one envelopes and UUID cursor validation, avoiding unstable ignored-cursor reads.
76. Mailbox folder counts pre-aggregate message state and add read/starred/cursor indexes to keep webmail navigation efficient at larger mailbox sizes.
77. Admin user/domain status writes normalize status values, and database constraints now protect persisted domain/user/message statuses and compose intents.
78. Draft-to-send now has an explicit service/repository transition: drafts are loaded as compose snapshots, sent through the normal outbound path, then marked deleted and linked to the created sent message for mailbox consistency.
79. Draft attachment uploads are moved to the created sent message during the draft-to-send transition and the sent message `has_attachment` flag is refreshed transactionally.

## Deferred until backend contracts stabilize

- Next.js shell/webmail/admin apps
- Kafka
- OpenSearch
- etcd
- Vault
- IMAP
- Push notifications
- Built-in spam filtering and pattern filtering; SMTP core should keep only pluggable boundaries and optional external relay adapters.
