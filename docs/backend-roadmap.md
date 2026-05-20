# gogomail backend roadmap

## Recent launch-readiness closures

- Mail API user sessions now have rotating refresh tokens backed by hashed `user_refresh_tokens`, reducing forced daily re-login while preserving single-use refresh semantics.
- HTTP runtime has request ID propagation and configurable PostgreSQL pool sizing across app DB open paths.
- System transactional emails are wired for admin invites, invite acceptance welcome mail, and user quota alerts.
- Retention AutoPurge has a scheduled execution path guarded by `GOGOMAIL_AUTO_PURGE_ENABLED` and company retention policy config.
- Operations backup coverage now includes a `pg_dump` backup script and Compose cron profile, with optional S3 upload.
- Webmail pre-launch gaps closed: password reset UI, server-synced signatures, Web Push service worker registration, and calendar edit/delete controls.
- Console pre-launch gaps closed: audit-log cursor pagination, delivery-attempt filters/feedback, and targeted TypeScript cleanup.

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
34. Password reset groundwork stores a per-user `recovery_email` shared by webmail settings and admin user management, and domain settings now carry `password_reset_token_ttl_minutes` so future reset tokens can use a domain-specific expiry.
35. Edge MTA prepends an RFC-shaped `Received` trace header before parsing/storing accepted mail, while tests can keep it disabled for raw storage assertions.
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
80. A backend release readiness checklist now tracks webmail API coverage, attachment storage, draft-to-send consistency, validation, admin CRUD, API error stability, required verification, and explicitly deferred spam/vendor modules.
81. Admin API now exposes domain and user detail endpoints, improving CRUD completeness for operational consoles without starting frontend implementation.
82. HTTP list endpoints reject malformed and nonpositive `limit` parameters instead of silently defaulting them, keeping API contracts explicit for OpenAPI generation.
83. Attachment uploads now cap multipart request bodies, verify declared size against stored bytes, sanitize fallback storage paths, and reject newline-bearing filenames/MIME types.
84. Service info and readiness responses now expose backend contract metadata and structured checks for deployment automation.
85. Backend API contract documentation now records response envelopes, auth modes, pagination behavior, and intentionally deferred frontend/spam modules as OpenAPI preparation.
86. Backend release hardening added stricter admin domain/user validation, constant-time admin token comparison, and trimmed development user identifiers.
87. Mail API now supports bounded bulk message flag, folder move, and soft-delete actions with duplicate ID rejection for stable webmail list operations.
88. Attachment lifecycle cleanup can expire stale uploading records, delete expired storage objects through the configured store, and use a partial cleanup index safely outside migration transactions.
89. Draft-to-send now carries draft attachment state into the outgoing message record, keeping sent-message `has_attachment` behavior consistent with attachment handoff.
90. SMTP DSN extension options (`RET`, `ENVID`, `NOTIFY`, `ORCPT`) now flow through inbound/submission session state, hook events, and recorder payloads so future RFC 3461 DSN generation can use negotiated envelope metadata.
91. SMTPUTF8 address handling is stricter: internationalized addresses require explicit server support and a transaction that declared `MAIL FROM ... SMTPUTF8`, preventing accidental partial EAI acceptance.
92. Inbound SMTP accepts RFC 5321 null reverse-path mail for DSN/bounce delivery while keeping transaction state separate from the envelope sender string.
93. Direct outbound SMTP continues DATA when at least one RCPT in a domain group is accepted, avoiding unnecessary rejection of valid recipients after partial RCPT failures.
94. SMTP storage paths sanitize tenant/user/message path segments before writing `.eml` files, preserving deterministic layout without allowing generated IDs to alter directory boundaries.
95. Outbound route definitions now support explicit SMTP ports and include the port in route pool keys, preparing Smart Host/gateway relay routing without collapsing distinct connection pools.
96. SMTP server runtime guardrails now expose configurable read/write timeouts, max message bytes, and max recipients, wired through environment config for edge and submission modes.
97. DSN composition preserves RFC 3464 `Original-Recipient` per-recipient fields and sanitizes DSN machine-readable values to prevent header injection in generated bounce reports.
98. DSN recipient status composition validates RFC-shaped actions and enhanced status codes before emitting `message/delivery-status` payloads.
99. Direct outbound SMTP reports partial RCPT delivery precisely: accepted recipients are recorded as delivered, permanent rejected recipients bounce, and only temporary rejected recipients are scheduled for retry.
100. Direct outbound SMTP treats partial DATA success as terminal for an MX host, preventing failover from duplicating already-accepted recipients on later MX candidates.
101. Smart-host route normalization accepts common `host:port` input while still keying pools by the normalized host and effective route port.
102. SMTP DSN extension handling now validates supported `RET` and `NOTIFY` values, including the RFC rule that `NOTIFY=NEVER` cannot be combined with other notification requests.
103. DSN composition sanitizes MIME boundary tokens and recipient machine-readable fields, preventing generated bounce reports from carrying header or boundary injection.
104. Smart-host routes now support SMTP AUTH credentials and separate route pool keys by authenticated username, preparing gateway relay delivery without sharing connections across identities.
105. Optional inbound SMTP AUTH now emits the same `authenticated` hook/metric boundary as submission, keeping AUTH-required receive/relay deployments observable.
106. SMTP receive and Submission MTA reject repeated AUTH attempts after a session is already authenticated, matching RFC 4954 state expectations and avoiding accidental identity replacement mid-session.
107. SMTP DSN `ENVID` and `ORCPT` inputs are syntax-guarded before being copied into hook/recorder payloads, preventing malformed DSN identity metadata from entering downstream bounce workflows.
108. DSN `NOTIFY` values are normalized and deduplicated as they flow through SMTP session state, keeping hook and recorder payloads stable for future DSN generation.
109. Smart-host route pool keys now include both SMTP AUTH username and identity/authzid, preventing distinct delegated relay identities from sharing pooled connections.
110. Outbound MX candidates are lowercased, de-duplicated, and stripped of DNS trailing dots before connection attempts, reducing duplicate delivery work and pool fragmentation.
111. Temporary MX lookup failures now produce a temporary delivery failure instead of falling back to the bare domain, preventing unsafe delivery attempts during DNS outages.
112. Direct SMTP delivery rejects jobs that have no deliverable recipients instead of silently reporting success.
113. Invalid smart-host route ports are clamped back to valid host-derived or default SMTP ports before connection and pool-key use.
114. New `MAIL FROM` commands reset prior RCPT/DSN transaction state for receive and submission sessions, preventing envelope leakage across SMTP transactions.
115. DSN composition sanitizes generated/returned Message-ID values as well as emitted headers, keeping bounce metadata safe from header injection.
116. DSN address headers sanitize envelope address values before formatting From/To headers in generated reports.
117. Runtime validation rejects nonpositive SMTP read/write and outbound delivery timeouts so protocol servers and delivery workers always start with bounded I/O deadlines.
118. Runtime validation rejects nonpositive DKIM verification limits, keeping authentication verification allocation/runtime guardrails enabled.
119. `mail.stored` events now carry SMTP envelope sender, DSN options, and Authentication-Results metadata so downstream audit/index/notification workers can use protocol-stage signals without reparsing SMTP session state.
120. Submission DSN options now flow into `mail.queued` events and are decoded by delivery jobs, preserving RFC 3461 envelope metadata across submission, outbox, and delivery boundaries.
121. Direct outbound SMTP now aggregates delivery outcomes across recipient domains, so successfully delivered domains are recorded as delivered while failed domains can bounce or retry independently.
122. Partial delivery retries now keep only DSN recipient metadata for temporary-failure recipients, preventing already-delivered or permanently bounced recipient metadata from leaking into retry jobs.
123. Delivery worker queue decoding validates and normalizes DSN payload fields at the queue trust boundary, including `RET`, `NOTIFY`, recipient addresses, and newline-bearing identity fields.
124. SMTP receive and Submission MTA reject declared `SIZE` values that exceed policy during `MAIL FROM`, avoiding unnecessary DATA streaming for known-oversized messages.
125. Delivery observability now reports all-permanent partial failures as bounced instead of deferred, making retry dashboards reflect actual terminal outcomes.
126. DSN report composition now aligns enhanced status classes with recipient actions (`2.x.x` delivered/relayed/expanded, `4.x.x` delayed, `5.x.x` failed) before emitting `message/delivery-status`.
127. Delivery jobs now support RFC 5321 null reverse-path senders for DSN/bounce traffic while still validating non-empty envelope senders.
128. Outbound farm values are normalized both when `mail.queued` events are recorded and when delivery jobs are decoded, preventing malformed farm payloads from creating invalid topics or route keys.
129. DSN report composition now enforces RFC-shaped enhanced status code classes/field widths and caps generated MIME boundary length, preventing malformed `message/delivery-status` reports from leaking into bounce traffic.
130. Delivery queue decoding trims and validates storage paths plus message identity fields before storage lookup, keeping malformed queue metadata out of hot delivery and attempt/metric paths.
131. DSN `NOTIFY` values are canonicalized at SMTP/session and delivery queue boundaries, so equivalent RFC 3461 options produce stable hook/event/retry payloads.
132. Duplicate delivery DSN recipient metadata is merged by normalized recipient address, preserving the first ORCPT and canonical union of notify requests for accurate partial retry/bounce metadata.
133. Direct outbound SMTP normalizes recipient domains before grouping, reducing duplicate MX/route work caused by case or DNS trailing-dot differences.
134. SMTP DATA success is treated as terminal even if QUIT later fails, preventing retry-induced duplicate delivery after a remote MTA has already accepted the message.
135. Smart-host route pool keys strip DNS trailing dots from host names, preventing otherwise identical routes from fragmenting connection pools.
136. SMTP receive and submission sessions now clear stale envelope state as soon as a new `MAIL FROM` command is received, even when that command is rejected, preventing failed second MAIL commands from reusing previous RCPT state.
137. Delivery attempt metadata now records normalized recipient domains and truncates long error messages at UTF-8 boundaries for safer PostgreSQL/audit storage.
138. Delivery throttling now emits retry scheduled and retry exhausted metrics, making deferred outbound farm/domain backpressure visible through the existing observability boundary.
139. `inbound-mta` now starts a real SMTP receive server with its own `GOGOMAIL_INBOUND_SMTP_ADDR`, rather than acting as a placeholder mode, moving Phase 1 closer to distinct Edge and post-filter receive boundaries.
140. SMTP server runtime extension toggles now propagate SMTPUTF8, REQUIRETLS, DSN, and BINARYMIME support into the underlying protocol server for both edge/inbound and submission modes.
141. Delivery queue DSN metadata rejects duplicate recipient NOTIFY merges that would combine `NEVER` with other notification requests.
142. Smart-host route normalization now handles bracketed IPv6 literals with and without explicit ports without fragmenting host lists or route pool keys.
143. SMTP DSN `ENVID` and `ORCPT` xtext validation now enforces RFC 3461 `+HH` escaping and rejects raw `=` or malformed plus escapes.
144. Outbound SMTP delivery now forwards DSN envelope options (`RET`, `ENVID`, `NOTIFY`, `ORCPT`) when the remote server advertises DSN support.
145. Queued delivery DSN `ORCPT` metadata is validated at the delivery trust boundary as an RFC-shaped typed xtext recipient value before it can reach SMTP command generation.
146. Null reverse-path delivery suppresses outbound DSN request options, reducing DSN/bounce loop risk while preserving regular recipient delivery.
147. Outbound SMTP DSN command generation has wire-level tests that verify advertised DSN support produces RFC-shaped MAIL and RCPT parameters.
148. `inbound-mta` can enforce a trusted relay CIDR policy through `GOGOMAIL_INBOUND_TRUSTED_RELAYS`, rejecting untrusted post-filter SMTP clients before envelope state is accepted.
149. Authenticated submission can optionally expose an implicit TLS SMTPS listener through `GOGOMAIL_SUBMISSION_SMTPS_ADDR`, reusing the configured SMTP certificate/key pair and running alongside the STARTTLS submission listener.
150. Smart-host route normalization strips invalid bare `host:port` suffixes before pool-key and connection use, preventing malformed operator input from becoming impossible host names.
151. Null reverse-path delivery suppresses both MAIL-level and RCPT-level outbound DSN request parameters, reducing bounce/DSN loop risk.
152. Trusted relay authorization accepts remote address strings with TCP ports as well as bare IP addresses, keeping inbound relay policy robust across listener/test adapters.
153. Submission listener addresses are trimmed before server startup so environment whitespace does not create invalid STARTTLS or SMTPS bind addresses.
154. Routed delivery now inherits the global delivery TLS policy when a smart-host route does not override it, preventing `require` deployments from accidentally downgrading routed traffic to opportunistic TLS.
155. Delivery workers can route all outbound mail through a configurable smart host using `GOGOMAIL_DELIVERY_SMARTHOST`, including explicit ports, optional route-specific TLS mode, and SMTP AUTH credentials.
156. Smart-host delivery supports implicit TLS relay connections through `GOGOMAIL_DELIVERY_SMARTHOST_IMPLICIT_TLS`, enabling port 465 gateway operation while keeping STARTTLS and implicit TLS route pools separate.
157. Delivery attempt events now carry DSN envelope metadata (`RET`, `ENVID`, `NOTIFY`, `ORCPT`) so downstream bounce/audit workers can generate RFC 3461-aware notifications without reparsing queue payloads.
158. Delivery attempts preserve enhanced SMTP status codes from remote replies (for example `5.1.1` and `4.7.1`) and include them in delivery events for more accurate DSN/bounce reporting.
159. SMTP server startup rejects implicit TLS listeners without TLS configuration, making SMTPS misconfiguration fail early with a clear operational error.
160. Event stream routing supports ordered fan-out handlers, allowing audit, DSN generation, indexing, notification, and future custom hooks to attach to the same mail lifecycle event without hard-coded feature coupling.
161. Hard bounce events now generate RFC 3464 multipart/report DSN messages, store them as `.eml`, and enqueue null reverse-path outbound delivery back to the original envelope sender while honoring `NOTIFY=NEVER`.
162. Delivery audit details now include the original envelope sender so operators can trace sender-recipient delivery outcomes without reparsing queued payloads.
163. Null reverse-path DSN bounces no longer create suppression-list entries, preventing system-generated bounce traffic from contaminating user outbound suppression policy.
164. Bounce DSN generation now uses deterministic storage paths, deterministic RFC-safe Message-IDs, and outbox dedupe keys so event-worker retries do not fan out duplicate DSN queue rows.
165. DSN postmaster identity is runtime-configurable and validated at startup, supporting production-grade bounce branding without risking malformed DSN From headers.
166. SMTP runtime configuration rejects unsafe `GOGOMAIL_SMTP_DOMAIN` values before startup, protecting SMTP banners, Received headers, and DSN reporting identities from malformed host names.
167. Scheduled delivery retries now normalize outbound farm topics, deduplicate retry outbox rows by message/attempt/recipient set, and keep retry error storage UTF-8 safe.
168. Outbox relay failure recording now truncates long errors at UTF-8 boundaries, keeping retry/failed-state diagnostics safe for PostgreSQL and audit surfaces.
169. SMTP receive and authenticated submission now have real TCP protocol integration tests that exercise go-smtp server wiring, raw `.eml` storage, AUTH PLAIN submission, wire-level policy rejections, and implicit TLS SMTPS delivery.
170. SMTP server TLS configuration is cloned and hardened to TLS 1.2+, and implicit TLS startup now fails fast when no static or dynamic server certificate source is configured.
171. Trusted relay CIDR authorization now handles IPv4-mapped IPv6 remote addresses, preserving relay policy correctness across dual-stack listener adapters.
172. Bounce DSN event decoding validates enhanced status, DSN notify, and newline-bearing DSN metadata before composing or queueing a generated RFC 3464 report.
173. Direct SMTP delivery preserves per-recipient permanent/temporary classes when all RCPT commands fail, while still failing over to the next MX when every rejected recipient is only temporarily deferred.
174. Authenticated Submission is now covered by STARTTLS wire-level tests: AUTH succeeds after STARTTLS and is rejected before TLS when insecure AUTH is disabled.
175. SMTP wire-level tests now assert unsupported MAIL extensions are rejected and EHLO capability advertisement matches enabled/disabled extension toggles for DSN, SMTPUTF8, and BINARYMIME.
176. SMTP wire-level DSN support now verifies `RET`, `ENVID`, `NOTIFY`, and `ORCPT` options flow through a real TCP SMTP session into recorder payloads, including go-smtp decoded typed-recipient handling.
177. Trusted relay policy now has TCP protocol coverage proving untrusted clients are rejected at the MAIL boundary before envelope acceptance.
178. SMTP receive soak coverage now exercises repeated DATA transactions on one connection, guarding session reset behavior under long-lived client sessions.
179. Outbound SMTP DSN wire coverage now confirms DSN parameters are suppressed when a remote peer does not advertise DSN support.
180. Backend release readiness documentation now calls out SMTP extension, TLS/SMTPS, trusted relay, DSN/bounce, and same-connection soak verification before a release cut.
181. PostgreSQL-backed release integration tests can run migrations in an isolated temporary schema and verify draft-to-send, attachment handoff, sent-message state, DSN payload persistence, and outbound outbox enqueue behavior against real SQL.
182. Admin console and webmail client contracts now align with the backend OpenAPI shapes for folders, messages, compose intents, domains, users, roles, DNS checks, scoped company user loading, SSO/auth policy, reports, organization hierarchy, audit logs, API keys, alerts, identity-provider placeholders, and statistics read models.
182. PostgreSQL outbox integration tests now verify `available_at` claiming, retry-to-pending behavior, retry exhaustion, and UTF-8-safe failure diagnostics against the migrated schema.
183. Trusted relay tests now explicitly cover empty relay policy defaults and malformed remote address rejection, tightening final inbound relay boundary verification.
184. Same-connection SMTP soak coverage now verifies DSN `RET`/`ENVID`/`NOTIFY`/`ORCPT` state does not leak into a later transaction on the same TCP session.
185. SMTP backend release operations now have a dedicated runbook for PostgreSQL verification, same-connection soak, STARTTLS, SMTPS, trusted relay policy, and outbound DSN/bounce smoke checks.
186. SMTP RCPT DSN recipient state is isolated per normalized recipient, with repeated RCPT commands replacing stored DSN metadata with the latest `NOTIFY`/`ORCPT` options for deterministic RFC 3461 handling.
187. Delivery throttling now has a shared `ThrottleCounter` lease boundary, preserving process-local throttling while giving server-farm deployments a clean integration point for cluster-wide farm/domain concurrency coordination.
188. Delivery throttling can use Redis-backed atomic lease counters through `GOGOMAIL_DELIVERY_THROTTLE_BACKEND=redis`, enforcing farm/domain concurrency budgets across delivery worker processes.
189. Delivery handlers now expose an adaptive domain backoff boundary and in-memory policy so temporary recipient-domain failures can defer later jobs for that domain without slowing unrelated domains or permanent-bounce handling.
190. Adaptive delivery domain backoff is runtime-configurable through environment/YAML settings and can be enabled in delivery workers with validated base/max delay windows.
191. Adaptive delivery domain backoff can use Redis-backed per-domain TTL state through `GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_BACKEND=redis`, sharing provider tempfail backoff across delivery worker processes.
192. Adaptive delivery domain backoff can now scope tempfail pressure by normalized delivery farm plus recipient domain through `GOGOMAIL_DELIVERY_DOMAIN_BACKOFF_SCOPE=farm_domain`, isolating bulk ramps from transactional/general delivery.
193. Admin role list/create endpoints now use the persisted RBAC schema instead of mock role data, including company-scoped role listing, custom role creation, permission counts, and active assignment counts.
194. Admin auth/session routes now use signed JWT access/refresh tokens, validate bearer sessions with `session_version`, refresh access tokens from refresh JWTs, and revoke sessions on logout by incrementing `session_version`.
195. Admin user management now exposes `DELETE /admin/v1/users/{id}` as a safe disable-and-revoke operation, completing the backend CRUD surface without hard-deleting user rows.
196. Admin organization management routes are now wired into the admin runtime with the real orgchart service/repository, exposing unit CRUD, hierarchy, membership, and sync operations behind admin auth.
186. Shared submission authentication now requires the owning company, domain, and user to all be active, preventing suspended tenants from authenticating through SMTP submission, IMAP, POP3, CardDAV, or other shared protocol adapters.
186. DSN queue and bounce-event trust boundaries now reject malformed RFC 3461 xtext metadata before outbound SMTP command generation or RFC 3464 report composition.
187. Attachment storage-path contracts now reject unsafe caller-provided paths and sanitize generated attachment object path segments before writing to storage.
188. SMTP release verification now covers `NOTIFY=NEVER` over a real TCP SMTP session and controlled outbound SMTP sink recipient-classification behavior.
189. Backend API contract metadata is centralized in code and guarded against OpenAPI drift, keeping service info and generated client contracts aligned.
190. Company-scoped console user tools now derive user lists from the company's domains before querying `/users`, preventing user config and MFA management screens from mixing tenants.
191. Console organization webhooks and notification templates now match backend persistence contracts: webhooks use generated secrets, and notification templates edit `subject`, `body`, and `enabled` fields only.
192. Webmail settings no longer call the nonexistent mailbox import `/messages/restore` route; mailbox export remains local, while backend-supported message restore stays on trash message endpoints.
193. Backend tenant-scoped user summaries now share the company-domain user enumeration path for bulk export, SCIM status, and security posture, preventing unscoped user counts or CSV rows.
194. The console `/admin/v1` proxy now preserves backend 204 no-body responses and download headers like `/api/admin`, keeping direct admin-v1 pages aligned with backend response contracts.
195. Admin OpenAPI webhook and notification-template schemas now match backend behavior: generated webhook secrets are response-only, webhook tests include `status_code`, and templates use `subject`, `body`, and `enabled`.
196. Browser smoke hardening now normalizes admin-console `/companies/default/...` routes to the real company UUID before tenant-scoped data loads, renders structured webmail API errors as user-readable strings, casts draft `scheduled_at` JSONB parameters explicitly, and restores draft-send recipients from the persisted `address` JSON key so compose draft-save/send preparation works against PostgreSQL.
197. Admin console i18n hardening now routes visible console copy through the shared locale catalogs for English, Korean, Japanese, and Simplified Chinese, including dashboard, tenant/domain/user management, security, organization, compliance, operations, and system health surfaces.
198. Admin console navigation hardening now builds side-navigation links from the active company route, synchronizes company context on route changes, and falls back from stalled client routing to browser navigation so menu clicks do not appear unresponsive during SPA transitions.
199. Admin console data-list hardening now routes table/list surfaces through a shared `DataTable` wrapper with consistent content scrolling, default client-side search, and default pagination where page-specific controls are not already present.
200. Parsed EML body caching is runtime-tunable with `GOGOMAIL_MESSAGE_BODY_CACHE_ENTRIES` and `GOGOMAIL_MESSAGE_BODY_CACHE_TTL`, and the mail service exposes cache hit/miss/eviction/expired snapshots for operational read-path visibility while pruning expired entries before fresh writes.
190. A backend-only OpenAPI 3.1 draft now documents the current mail/admin API surface without starting frontend implementation.
191. OpenAPI route coverage is now tested against registered Go HTTP routes, including health probes, so backend handlers cannot drift silently from the contract.
192. OpenAPI request bodies now describe the current JSON and multipart mutation payloads, giving future generated webmail/admin clients a stable backend contract before frontend implementation starts.
193. OpenAPI component references, message flag enums, and list limit bounds are now guarded by tests, tightening SDK-generation readiness without changing the SMTP core or starting frontend work.
194. HTTP list endpoints now reject over-200 limits at the API boundary, aligning runtime behavior with the OpenAPI `1 <= limit <= 200` contract before generated clients rely on it.
195. OpenAPI operations now expose stable lower-camel `operationId` values and reusable default Error responses, improving generated-client naming and error-envelope handling.
196. Backend release verification now has a single script entrypoint for Go tests, module tidy diff checks, optional PostgreSQL integration tests, and final git status inspection.
197. Admin backend now persists and exposes trusted relay CIDR management, moving inbound SMTP relay policy from environment-only operation toward auditable platform control.
198. DKIM key creation can now derive the DNS TXT public-key record from the private key when operators omit `public_key_dns`, reducing domain setup mistakes while keeping private keys out of list responses.
199. Domain DNS verification now has a backend checker and Admin API envelope for MX, SPF, DMARC, and active DKIM TXT records, preparing domain onboarding without starting frontend implementation.
198. Admin backend now persists and exposes delivery gateway/smart-host route management, letting operators define exact, wildcard, and default outbound routes with TLS, pool, and SMTP AUTH settings while keeping SMTP core focused on protocol boundaries.
199. Delivery workers can opt into PostgreSQL-backed route lookup with `GOGOMAIL_DELIVERY_ROUTE_BACKEND=postgres`; exact, wildcard, and default routes map into the existing delivery router boundary and fall back to direct MX delivery when no active route matches.
200. Domain DNS verification now has a persisted history endpoint and domain list/detail summaries expose the latest DNS check status/timestamp, so admin consoles can show onboarding progress without triggering fresh DNS lookups.
201. Mail API exposes a user-scoped sent-message delivery status endpoint that summarizes delivery attempts as pending/retrying/delivered/partial/failed/bounced without leaking other tenants' attempts.
202. Admin API can inspect and update the shared Redis-backed SMTP backpressure state with structured level/reason/until metadata while preserving legacy string-state compatibility for existing receive nodes.
203. Domain policy now has a runtime read helper and Mail API send/draft-send enforces outbound recipient-count and composed-message-size guardrails when a domain policy is set to `outbound_mode=enforce`.
204. Mail API exposes thread list and thread-message read models using `COALESCE(thread_id, id)` so existing unthreaded mail renders as conversations while future RFC References/In-Reply-To assignment can improve grouping.
205. Message parsing and persistence now use RFC `In-Reply-To`/`References` and reply/forward source messages to assign `thread_id`, improving conversation grouping while preserving user-scoped tenant isolation.
206. Reply composition now writes RFC `In-Reply-To` and `References` headers into outgoing `.eml` messages using the source message thread, preserving conversation threading for remote recipients and future IMAP clients.
207. Mail API exposes a small-deployment search endpoint backed by Postgres FTS over active-message metadata, while full received-body indexing remains reserved for the future index worker/OpenSearch boundary.
208. Quota roadmap is hierarchical and SaaS-oriented: company owns the contracted storage pool, domains receive allocations within the company pool, and users receive unified personal storage usable across mailbox, attachments, future Drive, and other user-owned storage. Domain default user quota changes should update default-following users while preserving custom user overrides.

204. Mailbox quota is enforced atomically at SMTP receive, Submission MTA, and Mail API delete flows using a PostgreSQL row-level lock on the user row; the SMTP layer returns RFC-correct 452 4.2.2 when the mailbox is full.
205. Per-domain inbound SMTP policy (max recipients per message, max message bytes, inbound mode) is enforced at the SMTP receive and Submission boundaries without leaking policy logic into protocol core; the `DomainPolicyLookup` interface keeps the SMTP engine decoupled from `maildb`.
206. DKIM key DNS verification workflow: operators can trigger `POST /admin/v1/dkim-keys/{id}/verify-dns`, which runs a targeted DNS lookup, persists the result to `domain_dns_checks`, and sets `dns_verified_at` on the key when the record matches.
207. Delivery route runtime counters (`RouteCounters`) track per-pool delivered/failed/retried/exhausted since process start and are exposed via `GET /admin/v1/delivery-routes/counters` when configured.
208. Retry exhaustion hook: when all delivery retries for a message are exhausted, an `exhausted` status row is written to `delivery_attempts` and a `mail.delivery_exhausted` outbox event is emitted; `GET /admin/v1/delivery-attempts/exhausted` lists these for operator triage and can scope them by recipient domain and RFC3339 lower-bound timestamp.
209. SMTPUTF8 declared correctly on outbound MAIL FROM whenever the sender or any recipient contains non-ASCII bytes and the remote server advertises SMTPUTF8, complying with RFC 6531 Section 3.3; also fixes a typo where RCPT TO responses were checked against status 25 instead of 250.
210. DMARC reject policy enforcement is opt-in via `ReceiverOptions.DMARCEnforce`; when enabled, messages failing DMARC with `p=reject` are refused with SMTP 550 5.7.1, while quarantine policy messages are delivered with the policy visible in the Authentication-Results header.
211. Admin API now exposes per-domain aggregate statistics (`GET /admin/v1/domains/{id}/stats`): active/total user counts, inbound/outbound/active message counts, storage used/limit bytes, 24-hour delivery outcomes, and suppression list size.
212. OpenAPI schemas expanded: DKIMKey now includes `dns_verified_at` and `status` enum; DeliveryAttempt includes `status` enum with `exhausted`; DKIM DNS verify response references typed `DKIMKeyDNSVerification` and `DNSRecordCheck` schemas; `ExhaustedAttemptsEnvelope` added.
213. Hierarchical quota ledger first implementation: Admin API exposes company quota read/update, user records track `quota_source=default|custom`, domain quota updates can propagate `default_user_quota` to default-following users, and mail write/delete transactions atomically adjust company, domain, and user ledgers.
214. API metering is a planned platform capability: collect company/domain/user/api-key, route, method, status, latency, response size, and timestamp asynchronously for SaaS billing, rate-limit policy, abuse detection, and operations dashboards without adding synchronous hot-path writes.
215. Attachment uploads now participate in the hierarchical quota ledger: upload metadata creation reserves bytes from company/domain/user counters, stale upload cleanup releases those bytes, stored objects are deleted after DB cleanup, and Mail API quota exhaustion is surfaced as HTTP 507 `insufficient_storage`.
216. Admin quota read models now expose remaining capacity, child allocated quota, allocatable quota, and over-allocation flags so operators can see company/domain/user pressure before writes fail.
217. Admin API exposes a read-only quota reconciliation report (`GET /admin/v1/quota-reconciliation`) comparing company/domain/user ledger counters against active message bytes plus uploading/stored attachment bytes.
218. Received-message body search has an asynchronous indexing boundary: `search-index-worker` consumes `mail.stored`, reads raw `.eml` from storage, extracts bounded plain text through `internal/message`, and upserts `message_search_documents`.
219. Postgres-backed search includes indexed received body text while preserving the existing `GET /api/v1/search` response envelope; OpenSearch, highlighting, and ranking remain behind the same future search contract.
220. API metering has a disabled-by-default HTTP middleware boundary with async fail-open recording and a `slog` sink, preparing future durable usage aggregation without synchronous enforcement.
221. Reply-thread candidate lookup and draft attachment lookup now use ordinality-preserving typed-array joins, removing `array_position($2, ...)` rescans from hot message ingestion and draft-send preparation paths.
222. Thread list read models now project only the API response columns from aggregated thread summaries, keeping mailbox conversation listing narrower and guarded against future `SELECT *` drift.
223. Delivery TLS-RPT report identity now follows the configured SMTP domain instead of a hard-coded localhost collector domain, keeping outbound TLS reporting aligned with production MTA identity.
224. Outbox relay failed-batch updates now project typed `unnest` input columns explicitly, keeping the high-throughput failure status path narrow and guarded against `SELECT *` drift.
225. Draft attachment binding now uses one typed-array batch update instead of one UPDATE per attachment ID, removing an N+1 database round trip from draft save/update preparation.
226. Attachment upload session finalization now locks and carries only the columns required to create the attachment row and refresh draft state, narrowing the finalize CTE on the upload hot path.
227. Audit log integrity scans now project only the columns needed for hash-chain verification in the recent-log subquery, reducing operational scan width while preserving tamper detection.
221. API metering can emit durable `api.usage` events through the generic outbox on topic `api.event`, keeping request handling fail-open while giving future aggregation workers a persistent event source.
222. Quota reconciliation corrections can be explicitly applied by operators through `POST /admin/v1/quota-reconciliation/corrections`; corrections lock the affected quota hierarchy and set counters from message/attachment source rows.
223. Domain outbound policy includes `max_attachment_bytes`, and Mail API attachment reservation/direct upload enforce it before quota reservation or object storage writes.
224. Attachment scanning has a disabled-by-default hook adapter outside SMTP core, allowing metadata-first attachment scanners to attach at the parsed stage without adding spam or vendor logic to protocol paths.
225. SMTP receive now looks up inbound domain policy for every recipient in a multi-recipient transaction, aggregates the strictest enforced limits for RCPT and DATA, and tempfails policy lookup errors instead of accepting recipients under stale or missing policy.
226. SMTP receive deletes a just-written raw `.eml` object if the stored hook, recorder, or mailbox-quota path fails before the database record commits, preventing orphaned object-storage data after failed DATA.
227. Authenticated SMTP Submission now carries all available `user_addresses` from DB authentication into `SubmissionUser` and authorizes `MAIL FROM` against that set, allowing owned additional sender addresses without permitting unrelated envelope senders.
228. Authenticated SMTP Submission now deletes a just-written submitted `.eml` object if the stored hook, submitted recorder, or mailbox-quota path fails before database commit.
229. IMAP authentication now consumes the DB authentication result's `must_change_password` state and rejects users that must rotate their password before opening protocol sessions.
230. API metering aggregation has a first worker/read-model boundary: `api-metering-worker` consumes `api.usage` events from `api.event`, upserts `api_usage_daily`, and Admin API exposes `GET /admin/v1/api-usage/daily`.
231. ADR 0004 captures the API metering aggregation boundary: HTTP remains fail-open, aggregation is disabled by default, daily aggregates are operational read models, and billing-grade idempotency is deferred.
232. Search results now have opt-in relevance ordering, rank scores, and bounded headline snippets through `sort=relevance`, `include_rank=true`, and `include_highlights=true`, while the default response remains date sorted.
## Phase 2: Protocol Gateways

Target outcome:

> IMAP, CalDAV, CardDAV 프로토콜 게이트웨이. SMTP 코어와 분리된 서비스형 게이트웨이.

`internal/imapgw`, `internal/caldavgw`, `internal/carddavgw`는 SMTP/Mail API와 분리된 독립 게이트웨이입니다. 각 게이트웨이는 서비스 어댑터 경계를 통해 `maildb`, `mailservice`, `storage`를 사용합니다.

233. `internal/imapgw` establishes a dependency-light IMAP gateway boundary with native DTOs/interfaces, UID-oriented mailbox state, RFC 3501 system flag mapping, mailbox helpers, and explicit deferral of `\Deleted`/EXPUNGE semantics.
234. ADR 0005 records that IMAP will be a separate gateway over stable mailbox/message interfaces rather than protocol code embedded into Mail API, SMTP, or `maildb` internals.
235. Push notification enqueue has a first async worker boundary: `push-notification-worker` consumes committed `mail.stored` events, routes them through `internal/pushnotify`, and supports a disabled-by-default `slog` sink while FCM/APNs delivery remains adapter work.
236. IMAP listener runtime now exposes configurable read, write, and IDLE timeouts and applies connection deadlines around command reads, response flushes, STARTTLS handshakes, and IDLE waits.
237. IMAP `AUTHENTICATE PLAIN` now honors plaintext TLS policy before decoding SASL initial responses, keeping `LOGINDISABLED`/`PRIVACYREQUIRED` enforcement ahead of credential payload handling.
238. IMAP failed `SELECT`/`EXAMINE` reselect attempts now leave the previous selected mailbox and subscription intact until the replacement mailbox is successfully resolved and subscribed.
239. IMAP `COPY` now validates destination mailbox existence before short-circuiting empty source UID sets, preserving `[TRYCREATE]` semantics for empty `$` SEARCHRES copies.
240. POP3 authentication now consumes the same `must_change_password` submission-auth state as IMAP and rejects password-rotation-required users before opening a maildrop session.
231. API metering aggregation now writes both daily and monthly Postgres read models from the same `api.usage` event, with Admin API exposing `GET /admin/v1/api-usage/monthly` for plan/billing analysis groundwork.
232. Mail API now supports user-scoped push device registration, listing, and soft deletion for APNs, FCM, and Web Push tokens; raw tokens are write-only while response envelopes expose only a short token suffix for diagnostics.
233. The push notification worker now resolves bounded active device targets from PostgreSQL before invoking its sink, so future FCM/APNs/Web Push adapters receive explicit per-device targets without touching SMTP hot paths.
234. Push notification candidates are now persisted to `push_notification_attempts` after worker sink enqueue succeeds, creating an operator audit trail for per-device notification fan-out before vendor delivery adapters are enabled.
235. Admin API exposes `GET /admin/v1/push-notification-attempts` with limit, status, and user filters so operators can inspect notification fan-out without querying PostgreSQL directly.
236. Push notification candidate recording returns the persisted attempt id to sink targets, giving future FCM/APNs/Web Push adapters a stable row for delivery outcome updates.
237. `internal/pushnotify` now includes a Postgres outcome updater for queued, delivered, failed, and invalid-token statuses, completing the internal write path future vendor sinks need.
238. Invalid-token push notification outcomes now soft-delete the matching user device in the same transaction as the attempt update, preparing automatic token hygiene for future vendor sinks.
239. Admin API exposes `GET /admin/v1/push-notification-stats`, summarizing active devices and push notification attempt statuses for release operations dashboards.
240. `mail.stored` event payloads now include `schema_version=2026-05-04.mail-stored.v1`, making the audit/search/push worker event contract explicit and regression-tested.
241. Audit, search indexing, and push notification consumers now reject unsupported explicit `mail.stored` schema versions while accepting legacy versionless events, reducing silent downstream drift.
242. Push notification worker now records `queued` outcomes only after a sink handoff succeeds, keeping failed sink handoffs visible as `candidate` attempts for retry and operations review.
242. IMAP UID storage is now explicit: `imap_mailbox_state` persists UIDVALIDITY, UIDNEXT, and highest MODSEQ, while `imap_message_uid` persists mailbox-local message UID and MODSEQ.
243. `maildb` can ensure mailbox IMAP UID state and assign stable message UIDs transactionally, preparing an adapter for `internal/imapgw` without starting an IMAP TCP server.
244. `maildb` now exposes first IMAP mailbox adapter methods that list/get folders as `internal/imapgw.Mailbox` DTOs while ensuring UIDVALIDITY/UIDNEXT state.
245. `maildb` can list mailbox messages as `internal/imapgw.MessageSummary` DTOs, assigning missing mailbox-local UIDs and mapping envelope/flag fields for future IMAP FETCH/LIST flows.
246. IMAP fetch groundwork can resolve active messages by mailbox UID and stream the raw stored `.eml` body through `mailservice` without parsing or copying it into memory.
247. IMAP STORE groundwork can mutate `\Seen`, `\Flagged`, and `\Answered` through the existing message flag JSON while advancing message/mailbox MODSEQ only for actual flag changes.
248. IMAP UID backfill can assign missing mailbox-local UIDs to existing active messages in bounded stable-order batches, preparing mailboxes before a live protocol listener is enabled.
249. Mail API move/delete paths now remove stale IMAP message UID rows in the same transaction, preventing mailbox-local UIDs from leaking across folders before IMAP MOVE/EXPUNGE semantics exist.
250. Optional PostgreSQL integration tests now cover IMAP UID backfill and move invalidation against migrated schema when `GOGOMAIL_TEST_DATABASE_URL` is configured.
251. `internal/imapgw` now includes a dependency-light in-memory mailbox event broker for future IMAP IDLE fan-out, with non-blocking delivery to avoid write-path backpressure.
252. EML parser tests now assert the bounded text reader stops after the max+1 truncation probe, and benchmarks cover truncated large-body reads for hot-path regression tracking.
253. API metering outbox events now include `schema_version=2026-05-04.api-usage.v1` and deterministic `event_id` values, while the aggregate consumer rejects unsupported schema versions.
254. API metering aggregation now claims `event_id` values in `api_usage_events` before daily/monthly upserts, preventing replayed usage events from double-counting operational totals.
255. API metering events now use `schema_version=2026-05-04.api-usage.v2` with tenant/company/domain/user/API-key/principal/auth-source dimensions, while the worker remains backward-compatible with v1 events.
256. API metering daily/monthly aggregates are keyed by identity dimensions so usage from different tenants, principals, API keys, or auth sources does not merge into the same operational total.
257. API metering can enrich Mail API usage from JWT claims and classify configured Admin API token access without coupling the metering package to the auth package.
258. `internal/searchindex` now has an OpenSearch writer adapter behind the existing indexing interface, using idempotent document IDs based on gogomail message IDs.
259. `search-index-worker` can select the OpenSearch writer with explicit endpoint/index configuration while preserving the existing Postgres read-side search contract.
260. The OpenSearch writer can bootstrap a strict message index mapping for identity fields, tenant/user filters, subject/body text, timestamps, and bounded body metadata.
258. `search-index-worker` can optionally bootstrap the OpenSearch index mapping on startup through `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_BOOTSTRAP=true`.
259. OpenSearch query-side groundwork can search user-scoped indexed documents and return ranked gogomail message IDs for later metadata hydration.
260. `maildb` can hydrate ordered search message IDs into active `MessageSummary` rows, preparing OpenSearch read-side results to preserve the existing Mail API response envelope.
261. `mailservice` can compose OpenSearch relevance ID hits with Postgres summary hydration when the current API search contract can be preserved, falling back to Postgres for unsupported filters/highlights.
262. Mail API app wiring can inject the OpenSearch search source when `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`, enabling safe relevance-search rollout with Postgres fallback for unsupported contract features.
263. OpenSearch indexed message documents now include parsed sender and attachment presence fields, preparing from/attachment search-filter parity with the Postgres search contract.
264. OpenSearch relevance search can apply from, subject, and attachment filters before Postgres metadata hydration.
265. OpenSearch relevance search can return subject/from/body highlights and map them into the existing Mail API `search_highlights` response shape.
265a. OpenSearch relevance search now supports RFC3339 timestamp range filtering on `received_at` via `since` (gte) and `until` (lte) query parameters, enabling date-bounded message discovery.
266. `mail.stored` events and OpenSearch message documents now carry `folder_id`, allowing relevance searches to apply Mail API folder filters before Postgres summary hydration instead of falling back to Postgres for folder-scoped queries.
267. Optional OpenSearch integration coverage can create a disposable index, bootstrap mappings, index folder-scoped documents, refresh, and verify folder-aware relevance search against a real backend when `GOGOMAIL_TEST_OPENSEARCH_URL` is set.
268. Search index worker startup logs now include non-secret backend diagnostics, including OpenSearch index name and bootstrap state, so operators can confirm search backend selection from logs without exposing endpoint credentials.
269. OpenSearch writer/searcher calls now use `GOGOMAIL_SEARCH_INDEX_OPENSEARCH_TIMEOUT`, giving operators an explicit external backend timeout instead of relying on a hidden adapter default.
270. OpenSearch documents now include a lower-cased `from_addr_lc` keyword field and sender filters query that field, preserving Postgres-like case-insensitive `from` filtering for relevance searches.
271. OpenSearch documents now include a lower-cased `subject_lc` keyword field and subject filters query that field, preserving Postgres-like case-insensitive substring filtering while keeping the analyzed subject field for ranking/highlighting.
272. OpenSearch highlight fragments are now filtered to marked snippets and bounded by count and UTF-8-safe byte length before they enter the Mail API response shape.
273. Mail API OpenSearch hydration now deduplicates repeated external hit IDs before Postgres summary loading while preserving the first rank/highlight result for deterministic search responses.
274. Backend release readiness now documents an OpenSearch rollout smoke path covering backend selection, bootstrap, timeout, and non-secret worker startup diagnostics.
275. The IMAP mailbox event broker now carries user IDs on events and delivers only to matching user+mailbox subscribers, preserving tenant isolation before future IDLE fan-out is wired to mutation paths.
276. `mailservice.StoreIMAPFlags` now publishes optional IMAP mailbox `flags` events after successful repository mutations, giving future IDLE sessions a first live-update source without starting a protocol listener.
277. Mail API single and bulk flag mutations now look up existing IMAP UID rows and publish optional mailbox `flags` events for UID-visible messages after database updates succeed.
278. Mail API single and bulk move mutations now publish optional mailbox `expunge` events for previously UID-visible source messages after database moves succeed and stale IMAP UID rows are invalidated.
279. Mail API single and bulk delete mutations now publish optional mailbox `expunge` events for previously UID-visible messages after soft-delete succeeds and stale IMAP UID rows are invalidated.
280. `mailservice` now exposes IMAP mailbox/message listing and mailbox-event subscription methods, keeping the future protocol listener on the service boundary instead of reaching into `maildb` internals.
281. `mailservice` now exposes bounded IMAP UID backfill through the same service boundary, preparing future operator/bootstrap modes without coupling them directly to `maildb`.
282. IMAP mailbox event publication from service mutations is now best-effort after successful database updates, preventing future IDLE fan-out failures from making committed mail writes look failed to clients.
283. `mailservice.IMAPStoreAdapter` now satisfies `imapgw.Store`, giving a future IMAP protocol listener a narrow adapter over service methods instead of direct repository access.
284. Backend release readiness now records the IMAP backend boundary state: service adapter, UID backfill, user-scoped event broker, best-effort flag/move/delete events, and focused verification commands.
284a. IMAP `SEARCH`/`UID SEARCH` can opportunistically prefilter simple conjunctions of supported text/date criteria with OpenSearch candidate IDs when the search index backend is enabled, while preserving the existing mailbox-order evaluator and falling back for unsupported boolean, flag, sequence-set, or header-heavy queries.
285. API metering now records immutable `api_usage_ledger` rows before updating daily/monthly aggregate read models, preserving event-level usage payloads for future billing/export workflows.
286. Admin API now exposes bounded API usage ledger list, NDJSON export, and stats endpoints with tenant/principal/time filters so operators can inspect and export event-level usage without depending on aggregates.
287. API usage ledger schema now enforces status, positive request count, nonnegative byte/latency, and JSON object payload constraints at the database boundary.
288. API usage export batches now persist manifest checkpoints with fixed filter windows and event/request/byte/latency totals, so downstream billing or warehouse exports can replay a known ledger slice by batch ID.
289. Admin API now exposes API usage export batch create/list/detail endpoints plus saved-batch NDJSON replay, keeping export checkpointing explicit and bounded.
290. API usage export batches can now register external artifact metadata with object key, SHA-256, byte count, event count, and JSON metadata while staying vendor-neutral.
291. Admin API now exposes API usage export artifact create/list/detail endpoints, and artifact rows are deduplicated per batch by object key and SHA-256 for idempotent handoff.
292. API usage export batches now support canonical SHA-256 manifest digests over saved batch metadata plus registered artifacts, giving downstream billing/export jobs a stable integrity primitive before external signing is wired.
293. Admin API now exposes manifest digest create/list/detail and verification endpoints so operators can confirm expected versus actual canonical digest values before handoff.
294. API usage export batches can now be written as NDJSON artifacts through a vendor-neutral streaming writer that computes byte count and SHA-256 while writing to object storage.
295. Admin API now exposes server-side export artifact write and download endpoints, with retry-friendly artifact registration and best-effort object cleanup when metadata registration fails.
296. API usage export artifact writing now streams full saved batch windows without the Admin API list cap, while export ordering indexes support stable `(event_timestamp, event_id)` scans.
297. Admin API now exposes stored API usage artifact verification, recomputing byte count and SHA-256 from object storage before billing or warehouse handoff.
298. API usage export manifest digests can now be signed through a disabled-by-default local-HMAC signer with explicit key IDs and persisted signature rows.
299. Admin API now exposes manifest signature create/list/detail/verification endpoints, preserving a vendor-neutral shape for future KMS or asymmetric signer backends.
300. Admin API now exposes API usage export handoff readiness for a saved batch, summarizing artifact coverage, latest manifest digest/signature state, operational readiness, and billing readiness without creating new export artifacts or signatures.
301. API usage export handoff readiness now supports explicit deep verification that streams artifacts, checks manifest artifact coverage, verifies digest/signature evidence, and returns `verified_billing_ready` separately from metadata-only billing readiness.
302. API usage export manifest signature verification now uses an `ExportManifestSignatureVerifier` boundary parallel to the signer, with local-HMAC as the first adapter and a clear replacement point for future production verification.
303. Admin API now exposes API usage export capability inspection so operators can see signer/verifier configuration and production billing readiness support before running export handoff workflows.
304. API usage export manifest signing now supports a disabled-by-default local-Ed25519 backend with base64 keypair validation, persisted `ed25519` signatures, backend/algorithm compatibility checks, and local-only billing readiness semantics.
305. Admin API now exposes API usage ledger retention readiness, a read-only cutoff report that blocks future archive/delete work unless candidate ledger rows are covered by a completed export batch with matching tenant/principal filters, no later-recorded candidate rows, and artifact/digest/signature evidence.
306. API usage export now has an operator runbook covering capability checks, saved export batches, artifact/digest/signature verification, deep handoff readiness, and retention-readiness gates.
307. API usage export manifest signing now supports a `remote-ed25519` backend that calls an HTTPS signer endpoint, verifies the returned Ed25519 signature locally with the configured public key, and can satisfy production signature readiness without coupling gogomail to a vendor KMS SDK.
308. Postgres and OpenSearch relevance search now share metadata-first weighting, boosting subject and sender matches above indexed body text with regression coverage on both backend query shapes.
309. The shared event worker now handles `mail.stored` events with an IMAP UID assignment handler, ensuring received active messages get mailbox-local UIDs asynchronously after SMTP storage commits.
310. `GET /api/v1/search` now explicitly stays scoped to active messages; draft search is deferred to a future dedicated contract so Postgres and OpenSearch relevance behavior remain aligned.
311. Admin push notification stats can now be scoped by `user_id` and RFC3339 `since`, letting operators compare active devices and recent fan-out attempt outcomes for one user's delivery troubleshooting without querying PostgreSQL directly.
312. Admin push notification attempt lists can now also be scoped by RFC3339 `since`, matching the stats endpoint so operators can drill from recent status counts into the corresponding attempt rows.
313. Admin delivery attempt lists can now be filtered by delivery status, recipient domain, and RFC3339 `since`, giving operators a bounded recent-window view for retry, bounce, destination-domain, and exhaustion triage.
314. OpenAPI now documents and tests the API usage ledger `tenant_id`, `principal_id`, `from`, and `to` filters accepted by the Admin API, preventing generated billing/export clients from losing runtime-supported query scope.
315. OpenAPI now exposes exhausted delivery attempts through a reusable `ExhaustedAttempts` response component and contract test, keeping terminal retry triage envelopes aligned with generated-client response handling.
316. Admin delivery attempt stats now summarize total attempts, unique messages, unique recipients, and delivered/failed/bounced/exhausted buckets with the same status, recipient-domain, and RFC3339 `since` filters as the attempt list, giving operators a compact retry/bounce dashboard primitive.
317. Admin API now exposes read-only outbox event metadata with bounded topic/status/RFC3339 `since` filters, letting operators inspect stuck async work without returning raw payload bodies.
318. Admin queue stats now distinguish ready pending work, delayed pending work, stale processing locks, oldest ready time, and next available retry time so operators can separate backlog from scheduled delay.
319. Shared EML parsing now caps total MIME parts through `ParseOptions.MaxParts` and reports `PartsTruncated`, preventing pathological part counts from forcing unbounded parser iteration on SMTP, Mail API, search indexing, and future IMAP hot paths.
320. Delivery attempt lists now order by `attempted_at DESC, id DESC`, making admin retry/bounce views and user-scoped sent-message delivery status deterministic when multiple attempts share the same timestamp.
321. Shared EML parsing now treats inline parts with filenames and non-text inline parts as attachment metadata without reading their bodies, improving `has_attachment` accuracy for MIME messages that do not use `Content-Disposition: attachment`.
322. Outbound RFC 5322 text composition rejects CR/LF-bearing subject, display-name, email, and explicit Message-ID inputs before header serialization, tightening header-injection safety for send and draft-send paths.
323. Admin outbox event metadata lists can now filter by partition key as well as topic, status, and RFC3339 creation lower bound, making message/batch-specific async troubleshooting possible without exposing payload bodies.
324. Admin outbox event metadata lists now return UTF-8-safe bounded `last_error` previews, keeping operational list responses lightweight while preserving the full stored error for future detail views.
325. Admin outbox event detail now returns full event metadata and full stored `last_error` by id without exposing the JSON payload body, giving operators a safe drill-down path from bounded list previews.
326. Admin outbox retry now trims path IDs and rejects blank IDs at the HTTP boundary before reaching repository mutation logic.
327. Admin delivery-route counter and DKIM DNS verification responses now use reusable OpenAPI response components and drift tests, keeping operator API envelopes explicit for generated clients.
328. Delivery route runtime counter snapshots now sort pools deterministically, keeping admin operator dashboards and API tests stable across map iteration order.
329. CalDAV and CardDAV Basic Auth now reject `must_change_password` users, keeping DAV clients aligned with SMTP submission, IMAP, and POP3 temporary-password rotation policy.
330. CardDAV vCard object validation now requires CRLF line endings, rejecting LF-only or mixed-newline contact payloads before they can be persisted.
331. CardDAV `address-data` partial projections now retain vCard `UID` lines even when the client requests a narrow property set, preserving contact identity in filtered REPORT responses.
332. CardDAV `addressbook-query` negated parameter text matches now evaluate every parameter value, preventing multi-value parameters such as `TYPE=home,work` from satisfying a forbidden `home` filter.
333. CardDAV `addressbook-query` candidate optimization now declines LIKE wildcard or escape-bearing seed text, falling back to exact in-memory filtering over the broad walker.
334. CardDAV sync change pruning now explicitly preserves the current address book `sync_token` in both dry-run and delete paths, preventing retention cleanup from expiring the latest collection marker.
335. CalDAV storage validation tests now pin the split between iTIP scheduling payloads and persisted calendar object resources by rejecting `METHOD`-bearing VCALENDAR bodies at `UpsertObject`.
336. CalDAV sync change pruning now explicitly preserves the current calendar `sync_token` in both dry-run and delete paths, matching the CardDAV retention guard for latest collection markers.
329. Shared EML parsing now exposes a `MaxHeaderBytes` option wired into go-message header parsing, letting hot-path callers bound pathological RFC 5322 header blocks alongside body, attachment, and part limits.
330. Push notification attempt outcomes can now persist provider message IDs and provider status codes, preparing the async push pipeline for FCM/APNs/Web Push adapters without exposing a public mutation API or coupling vendor behavior to SMTP writes.
331. Delivery attempt rows now persist sender, RFC 3463 enhanced status, and RFC 3461 DSN `RET`/`ENVID`/`NOTIFY`/`ORCPT` metadata, and Admin/user delivery status reads expose those diagnostics for bounce and retry triage.
332. API usage ledger retention runs now persist audit rows for blocked, dry-run, and destructive attempts, including filters, counts, deleted rows, and the readiness snapshot used for the decision.
332. Migration filenames now have test coverage for unique numeric versions, and the message search index migration has been moved to an idempotent latest-version file to avoid duplicate goose version ambiguity.
333. OpenAPI contract tests now guard documented query parameters for operational triage APIs, including delivery attempts, exhausted attempts, push attempts/stats, outbox events, and API usage export handoff readiness.
334. Delivery enhanced-status extraction now recognizes RFC 3463 codes embedded in multiline SMTP replies and bracketed diagnostics while still requiring the code class to match the SMTP reply class.
335. Shared EML parser coverage now pins RFC 2047 encoded-word decoding for subjects and address display names, protecting internationalized header behavior used by SMTP receive, Mail API, search indexing, and future IMAP.
336. DSN composition now folds long header and `message/delivery-status` fields to keep generated bounce reports within RFC line-length limits while preserving injection sanitization.
337. IMAP mailbox event broker tests now pin non-blocking fan-out, context-cancel cleanup, and canceled-publish rejection, protecting the future IDLE foundation from slow-subscriber regressions.
338. OpenAPI request-body contract tests now cover company quota updates and API usage export artifact create/write endpoints, preventing generated Admin clients from losing required mutation schemas.
339. Shared EML parsing now tolerates unknown charsets on multipart text parts when go-message can still return a usable part body, preserving bounded raw body extraction instead of failing the whole parse.
340. Migration file tests now also require contiguous numeric versions, preventing release builds from silently skipping a migration number after renames or parallel agent work.
341. Domain statistics now use a reusable OpenAPI response component and envelope drift tests, keeping `GET /admin/v1/domains/{id}/stats` aligned with generated Admin clients.
342. OpenAPI contract tests now derive path parameters from every documented route and require matching operation parameters, preventing generated clients from silently losing IDs on nested Mail/Admin API endpoints.
343. Nested OpenAPI path parameters for attachment downloads and API usage export digest/signature routes now use reusable components and drift tests, keeping generated clients consistent across deeply nested operational endpoints.
344. The delivery worker now wires the Postgres recorder as the retry exhaustion hook, so exhausted retries persist terminal delivery-attempt diagnostics and emit the documented `mail.delivery_exhausted` outbox event at runtime.
345. OpenAPI DKIM key creation now matches the Go/Admin API contract by documenting optional `public_key_dns` and rejecting the unsupported `active` request field in drift tests.
346. OpenAPI quota update requests are now split by company, domain, and user scope, documenting domain `default_user_quota` and user `quota_source` while keeping generated Admin clients aligned with Go request types.
347. OpenAPI response envelope drift tests now cover company list/detail, delivery route counters, and DKIM DNS verification envelopes, closing gaps in generated Admin client response guarantees.
348. OpenAPI query-parameter drift tests now cover common Admin list and lookup filters for companies, domains, users, quota usage, trusted relays, delivery routes, DKIM keys, and DNS check history.
349. Shared EML text-body truncation now backs up to a valid UTF-8 boundary before storing bounded text previews, protecting Mail API/search/IMAP consumers from malformed strings on multibyte bodies.
350. Shared EML attachment filename metadata is now basename-normalized, control-character cleaned, UTF-8-safe, and capped before storage/API/search consumers see it.
351. Outbound RFC 5322 text composition now folds long headers, keeping generated `From`/`To`/`Subject`/thread metadata lines within mail line-length limits while preserving CR/LF injection rejection.
352. Outbound RFC 5322 text composition now rejects malformed explicit `Message-ID` values and drops malformed thread IDs before writing `In-Reply-To`/`References` headers.
353. DSN composition now validates typed `Original-Recipient`/`Final-Recipient` address-type fields at the package boundary while preserving normalized bare RFC822 recipients.
354. The IMAP `mail.stored` notification handler can now publish UID-bearing `EXISTS` mailbox events after successful async UID assignment, preparing the future IDLE listener without adding work to SMTP receive.
355. Admin push notification attempt triage can now filter by platform, device id, provider status, and provider message id in addition to status/user/since, improving vendor-outcome troubleshooting readiness.
356. Redis event consumers now acknowledge malformed stream entries after logging decode failures, preventing poison messages from pinning worker progress while preserving retry behavior for handler failures.
357. OpenAPI drift tests now pin Admin domain/user and delivery-route status enums against backend validators, preventing generated clients from drifting on supported lifecycle states.
358. Project continuity docs now reflect the autonomous release-readiness hardening sprint, including parser hygiene, outbound/DSN RFC tightening, IMAP notification readiness, push outcome filters, Redis poison-message handling, and expanded OpenAPI guardrails.
359. Redis event consumers can now reclaim idle pending stream messages with per-worker claim-idle settings, allowing event/search/API-metering/push/delivery workers to recover work left pending by crashed consumers.
360. OpenAPI contract tests now guard non-JSON download/export responses, including NDJSON API usage exports and binary attachment downloads, so generated clients do not treat streamed bytes as JSON envelopes.
361. Push notification workers now treat queued-outcome recording failures after sink success as operational warnings instead of handler failures, reducing duplicate push risk from Redis event redelivery while preserving candidate audit rows.
362. DSN composition can now include an optional sanitized `text/rfc822-headers` original-message header part, preparing RFC 3464 `RET=HDRS` bounce reports without exposing header injection.
363. User-scoped sent-message delivery status now treats failed attempts with `4.x.x` enhanced status codes as retrying, so temporary SMTP failures do not appear as terminal failures in Mail API responses.
364. Shared EML parsing now caps retained address-list and `References` metadata with explicit truncation flags, preventing oversized headers from expanding downstream storage, search, and threading metadata unboundedly.
365. Admin token checks now compare SHA-256 digests of trimmed token values, keeping the authorization comparison fixed-length while preserving existing bearer and `X-Admin-Token` behavior.
366. Admin delivery-route creation now rejects impossible TLS/auth combinations, including implicit TLS with disabled TLS mode and password-only authentication, before invalid relay routes can be stored.
367. Event and delivery Redis worker configuration now rejects nonpositive consumer count/block settings at startup, matching the existing push, API-metering, and search-index guardrails before unusable stream consumers can run.
368. OpenAPI drift tests now pin Mail API search query parameters, including relevance sorting, rank/highlight toggles, attachment filtering, and metadata filters, protecting generated client search controls from contract regression.
369. Admin delivery-route status and delete handlers now trim route IDs at the HTTP boundary before service calls and response envelopes, keeping operator mutations consistent with repository validation.
370. Admin API now exposes bounded IMAP mailbox UID backfill by user/mailbox, giving operators a bootstrap path for future IMAP enablement without starting an IMAP protocol listener.
371. Push notification workers now mark candidate attempts `failed` with the sink error when sink handoff fails, preserving operator diagnostics while still returning the handler error for Redis stream retry.
372. Push notification target resolution now drops malformed targets with blank device IDs, blank tokens, or unsupported platforms before sink handoff, keeping future vendor adapters behind a cleaner boundary.
373. API metering admin-token identity classification now compares SHA-256 digests of trimmed token values, matching the hardened Admin API authorization path while preserving bearer and `X-Admin-Token` classification.
374. API usage ledger retention-readiness now rejects future cutoff timestamps at the HTTP boundary, preventing operators from marking still-open accounting windows ready for future archive/delete jobs.
375. OpenAPI drift tests now pin API usage retention-readiness query parameters and document the non-future cutoff guardrail, keeping generated archive/delete tooling aligned with the safety gate.
376. Direct multipart attachment uploads now classify over-limit HTTP request envelopes as 413 `payload_too_large` while preserving 400 responses for malformed multipart bodies, keeping generated clients aligned with upload retry/error handling.
377. Mail API path identifiers and direct-upload `draft_id` fields are now whitespace-normalized at the HTTP boundary before service dispatch, reducing accidental client formatting drift without pushing cleanup into service/storage layers.
378. OpenAPI drift tests now pin attachment reservation and direct-upload HTTP 413 Error responses, protecting generated clients from losing size-cap failure modeling.
379. Mail and Admin API JSON request handlers now reject trailing JSON tokens before service dispatch, preventing malformed multi-object bodies from being partially accepted.
380. Attachment download responses now include a safe ASCII `filename` fallback plus RFC 5987-style UTF-8 `filename*` parameter, preserving internationalized attachment filenames for webmail clients without relaxing header-injection guards.
381. Attachment downloads now sanitize the stored MIME type at the HTTP boundary, falling back to `application/octet-stream` for blank or newline-bearing values before response headers are written.
382. OpenAPI now documents attachment download `Content-Disposition` and `Cache-Control: no-store` headers and drift-tests them alongside the binary response media type.
383. API usage artifact downloads now sanitize stored content type and SHA-256 response headers before streaming export objects, preserving billing handoff integrity without trusting persisted header values blindly.
384. API usage ledger NDJSON exports, batch replay exports, and stored artifact downloads now return `Cache-Control: no-store`, with OpenAPI drift coverage for generated billing/export clients.
385. Attachment downloads, API usage NDJSON exports, and stored usage artifact downloads now return `X-Content-Type-Options: nosniff`, with OpenAPI drift coverage to keep browser-facing stream responses from being MIME-sniffed.
386. API metering auth-source dimensions now normalize to the fixed known set `anonymous|bearer|admin_token|query_user_id|unknown`, folding unexpected resolver values to `unknown` before ledger and aggregate storage to prevent billing dimension cardinality drift.
387. API metering durable event metrics now clamp negative request bytes, response bytes, and latency to zero and default nonpositive request counts to one before ledger and aggregate storage, keeping replayed usage payloads inside database accounting constraints.
388. API metering durable events now require nonblank method/route keys and HTTP-like status codes before ledger and aggregate storage, preventing malformed usage payloads from polluting billing/export dimensions.
389. API metering outbox payload generation now clamps negative byte and latency values before deterministic event IDs are generated, aligning source emission with worker-side ledger/aggregate normalization.
390. API metering middleware now falls back to `METHOD /path` when no `http.ServeMux` route pattern is available, keeping durable usage event route keys nonblank even around custom handlers or unmatched routes.
391. API usage export batch creation now requires explicit RFC3339 `from`/`to` ledger windows at the Admin API boundary, preventing accidental all-ledger checkpoints and keeping generated clients aligned through OpenAPI drift coverage.
392. The OpenAPI thread-list operation no longer leaks API usage export filters into generated Mail API clients, with drift coverage pinning the route to its actual `limit` query contract.
393. Mail API JWT verification now validates signed token headers, accepting only HS256 and optional JWT `typ` values before trusting bearer claims.
394. JWT signing and verification now whitespace-normalize `user_id`/`sub` identities and reject blank identities, keeping Mail API bearer scoping from accepting formatting-only subjects.
395. API metering request identity extraction now trims tenant/company/domain/user/API-key/principal dimensions and treats blank bearer headers as anonymous unless another auth signal is present.
396. JWT verification now rejects tokens whose `iat` is more than one minute in the future, reducing acceptance of misissued bearer tokens while allowing modest clock skew.
397. Attachment and API usage artifact downloads now parse stored media types before writing `Content-Type`, falling back to safe defaults for malformed MIME values.
398. Attachment download `Content-Disposition` filenames are now bounded before header emission, preserving UTF-8 filename support without allowing oversized stored names to bloat responses.
399. Mail and Admin API JSON body decoding is now capped at 1 MiB before parsing while preserving the existing trailing-token rejection guard.
400. Admin API domain query identifiers for user listing, DKIM key listing, and delivery-route resolution are now trimmed before service dispatch, matching the existing path-id normalization stance.
401. Mail API search query, folder, sender, and subject filters are now trimmed before backend dispatch, keeping Postgres/OpenSearch search behavior aligned when clients send accidental whitespace.
402. API usage ledger, export batch, and retention-readiness tenant/principal query filters are now trimmed before billing/export service dispatch.
403. Admin API outbox event topic, partition key, and status filters are now trimmed before operational queue inspection.
404. Admin API delivery-attempt status and recipient-domain filters are now trimmed before retry/bounce inspection.
405. Admin API push-notification attempt and stats filters are now trimmed before device/provider troubleshooting queries.
406. Mail API push-device registration now normalizes user, platform, token, and label fields before validation/storage while keeping raw tokens write-only in responses.
407. Mail compose draft/save/send requests now normalize user/source/from/address and attachment identifier fields before repository, storage, suppression, and outbound composition work.
408. Attachment upload reservation and direct-upload service requests now normalize user, draft, filename, MIME type, and storage-path metadata before quota, storage, and repository work.
409. Attachment list/download and draft-delete service methods now trim user, message, attachment, and draft identifiers before repository/storage work.
410. Single-message flag, move, and delete service methods now trim user/message/flag and folder identifiers before repository mutation and IMAP event fan-out.
411. Bulk flag, move, and delete service methods now trim user/message/flag and folder identifiers before repository mutation, IMAP UID lookup, and mailbox event fan-out.
412. Folder, message-list, thread-list, and message-detail service reads now trim user, folder, thread, message, and folder-name inputs before repository work.
413. Mail search service queries now normalize user, text, folder, sender, subject, and sort inputs before Postgres or OpenSearch dispatch.
414. Push-device list and delete service methods now trim user and device identifiers before repository work.
415. Message delivery-status and reply source-thread service lookups now trim user, message, and source-message identifiers before repository work.
416. Stale attachment-upload cleanup now validates its time window and limit at the service boundary before repository cleanup/object deletion work.
417. Message, thread, and push-device list service methods now normalize list limits to the documented message-list bounds before repository work.
418. OpenAPI contract tests now pin the push-device list `limit` query parameter so generated clients keep pagination controls for device management.
419. IMAP service methods now trim user/mailbox identifiers and normalize list/backfill limits before repository, storage, broker, or mailbox-event work.
420. Domain policy service lookups now trim domain and user identifiers before repository policy reads for outbound and attachment enforcement.
421. The audit `mail.stored` consumer now trims event, tenant, recipient, subject, storage, and timestamp fields and rejects CR/LF-bearing message identifiers before audit-log persistence.
422. Delivery-status audit consumers now trim event, tenant, sender, recipient, farm, status, error, and timestamp fields and reject CR/LF-bearing message identifiers before audit-log persistence.
423. Event routing now trims registered and payload event names and rejects CR/LF-bearing event names before worker dispatch.
424. Redis stream event decoding now trims outbox id, partition key, and payload fields and rejects blank metadata before handler dispatch.
425. Delivery outcome and exhausted outbox event payloads now trim message, tenant, farm, sender, recipient, error, and DSN metadata before event persistence.
426. Redis outbox publishing now trims event id, topic, partition key, and payload metadata and rejects invalid topics or non-JSON payloads before stream writes.
427. API metering durable event decoding now rejects CR/LF-bearing method, route, event-id, tenant, company, domain, user, API-key, and principal dimensions before ledger/aggregate storage.
428. API metering outbox production now rejects CR/LF-bearing method, route, event-id, tenant, company, domain, user, API-key, and principal dimensions before durable usage-event insertion.
429. API usage aggregate storage now validates method, route, event-id, schema-version, identity dimensions, and HTTP-like status before direct store writes.
430. Backend release verification now fails on a dirty worktree after standard tests and ignores local OpenChrome session artifacts, turning final status inspection into an enforceable release gate.
431. Push notification target resolution now drops CR/LF-bearing device IDs or tokens before sink handoff, preparing vendor adapters to receive cleaner target metadata.
432. Push notification candidate and provider-outcome diagnostics now truncate at UTF-8 boundaries before Postgres storage, preserving valid Admin API text for internationalized subjects and vendor messages.
433. Search indexing now rejects ambiguous `mail.stored` storage paths that would be changed by path cleaning, preventing traversal-shaped event payloads from opening a different `.eml` object key.
434. Search indexing now caps `mail.stored` event `References` metadata before document construction, aligning event payload handling with parser metadata bounds.
435. The OpenSearch indexing adapter now bounds UTF-8 metadata fields and reference arrays before document submission, keeping direct adapter calls aligned with worker/parser metadata limits.
436. OpenSearch relevance queries now bound UTF-8 search/filter text and escape wildcard metacharacters in sender/subject filters before submission, preserving literal Mail API filter semantics.
437. OpenSearch relevance hit IDs are now bounded and CR/LF-guarded before Postgres hydration, keeping external search responses from sending malformed message IDs into repository lookups.
438. OpenSearch relevance response decoding is now capped before JSON parsing, preventing oversized backend responses from allocating unbounded highlight or hit payloads in the Mail API path.
439. API usage export artifact writes now reject ambiguous object keys that would change during path cleaning, including duplicate separators, dot segments, backslashes, and parent traversal.
440. API usage export manifest digesting now rejects unsupported explicit manifest schema versions before canonical digest evidence is generated.
441. API usage export manifest signing now rejects blank, CR/LF-bearing, or oversized key IDs for local and remote signer metadata before signature evidence is returned.
442. Admin API DKIM key deactivate and DNS-verify path identifiers are now trimmed before service dispatch and response envelopes, aligning DKIM operations with other Admin route ID handling.
443. Admin API suppression-list and trusted-relay delete path identifiers are now trimmed before service dispatch and response envelopes, aligning operator delete routes with other Admin ID handling.
444. Admin API company, domain, and user quota/status/policy mutation path identifiers are now trimmed before service dispatch and response envelopes, keeping operator mutations tolerant of incidental URL whitespace.
445. Draft save/update validation now enforces the same attachment-count cap as immediate send, preventing oversized compose payloads from reaching draft attachment binding.
446. Compose request validation now rejects CR/LF-bearing recipient display names and emails before draft persistence or outbound header composition.
447. Compose request validation now rejects CR/LF-bearing explicit sender hints and subjects before draft persistence or outbound header composition.
448. Bulk mailbox mutation validation now rejects CR/LF-bearing or oversized message/folder identifiers before repository mutation and IMAP event fan-out.
449. Single-message mutation/read and attachment-read service methods now reject blank, CR/LF-bearing, or oversized message/folder/attachment identifiers before repository, storage, or IMAP UID lookup work.
450. Draft save/delete/send and reply/forward compose validation now reject blank, CR/LF-bearing, or oversized draft/source-message identifiers before repository dispatch.
451. Mail search service validation now rejects CR/LF-bearing or oversized query/filter fields before Postgres fallback or OpenSearch relevance dispatch.
452. Attachment reservation and direct-upload validation now rejects CR/LF-bearing or oversized draft identifiers before quota reservation or object writes.
453. User folder create/rename validation now rejects blank, path-bearing, CR/LF-bearing, or oversized names, and folder rename/delete reject unsafe folder identifiers before repository dispatch.
454. Push-device delete validation now rejects blank, CR/LF-bearing, or oversized device identifiers before repository dispatch.
455. Folder-scoped message lists and thread-message reads now reject unsafe folder/thread identifiers before repository work.
456. Message-list cursor decoding now rejects oversized opaque cursor strings before base64 decode and JSON parsing.
457. Admin outbox event topic, partition-key, and status filters now reject CR/LF-bearing or oversized values before service dispatch.
458. Admin delivery-attempt list, stats, and exhausted filters now reject CR/LF-bearing or oversized values before service dispatch.
459. Admin push-notification attempt and stats filters now reject CR/LF-bearing or oversized values before service dispatch.
460. Admin API usage ledger/export/stats/export-batch/retention tenant/principal filters now reject CR/LF-bearing or oversized values before service dispatch.
461. Admin user-list, IMAP UID backfill, DKIM key-list, and delivery-route resolution query filters now reject CR/LF-bearing or oversized values before service dispatch.
462. API usage export batch, artifact, manifest-digest, and manifest-signature path identifiers now reject blank, CR/LF-bearing, or oversized values before service dispatch.
463. Admin company, domain, and user detail/mutation path identifiers now reject blank, CR/LF-bearing, or oversized values before service dispatch.
464. Admin IMAP UID backfill mailbox IDs, outbox event/retry IDs, DKIM key IDs, suppression IDs, trusted-relay IDs, and delivery-route IDs now reject blank, CR/LF-bearing, or oversized values before service dispatch.
465. Mail API development `user_id` query fallback values now reject CR/LF-bearing or oversized identifiers before service dispatch.
466. Mail API folder, thread, message, draft, attachment, and push-device path identifiers now reject blank, CR/LF-bearing, or oversized values before service dispatch.
467. Mail API message-list `folder_id` and search text/filter query parameters now reject CR/LF-bearing or oversized values before service dispatch.
468. Mail API bearer JWT `user_id` and `sub` identities now reject CR/LF-bearing or oversized values during signing and verification before request scoping.
469. Mail API bearer JWT verification now rejects oversized token, header, payload, and signature segments before base64 decoding.
470. Mail and Admin API authentication headers now reject oversized `Authorization` and `X-Admin-Token` values before bearer parsing, JWT decoding, or token comparison.
471. Password hash verification now rejects oversized stored hashes, excessive PBKDF2 iteration counts, and oversized PBKDF2 salt/key metadata before expensive derivation or decoded allocation.
472. Mail API search control query values and direct multipart attachment `draft_id` fields now reject CR/LF-bearing or oversized values at the HTTP boundary before service dispatch.
473. VERP return-path parsing now rejects oversized addresses, local parts, tokens, and encoded recipients before base64 decoding DSN recipient metadata.
474. API usage export Ed25519 signer/verifier key configuration now rejects oversized base64 public/private keys before decoding.
475. API usage export manifest signer configuration now rejects CR/LF-bearing or oversized key IDs and remote signer tokens, and local HMAC signing rejects oversized secrets before MAC generation.
476. API usage export HMAC and Ed25519 signature verification now rejects incorrectly sized signature hex before decoding.
477. Remote Ed25519 manifest signer responses now reject oversized bodies and trailing JSON tokens before signature evidence is accepted.
478. OpenSearch relevance response decoding now rejects oversized bodies and trailing JSON tokens before search hits are accepted.
479. API metering default request identity extraction now drops CR/LF-bearing or oversized dimensions and avoids classifying unsafe auth headers or `user_id` query values as bearer/admin/query-user traffic.
480. API metering middleware route-key extraction now drops CR/LF-bearing or oversized ServeMux patterns and fallback paths before sink dispatch.
481. Eventstream routing and Redis stream decoding now reject CR/LF-bearing or oversized event names, stream metadata, and payloads before worker fan-out.
482. IMAP UID assignment event decoding now rejects CR/LF-bearing or oversized message, user, and folder IDs before UID work or mailbox event fan-out.
483. Push notification `mail.stored` event decoding now rejects CR/LF-bearing or oversized message/user IDs before target resolution or candidate fan-out.
484. Search indexing `mail.stored` event decoding now rejects oversized message/user IDs and storage paths before stored EML objects are opened.
485. Mail receive audit event decoding now rejects CR/LF-bearing or oversized message IDs before immutable audit log construction.
486. Delivery status audit event decoding now rejects CR/LF-bearing or oversized message IDs before immutable audit log construction.
487. Delivery `mail.queued` decoding now rejects oversized message identities and storage paths before SMTP transport or message storage access.
488. Delivery `mail.queued` DSN option decoding now rejects oversized `original_recipient` values before retry/delivery attempt recording.
489. Delivery `mail.queued` decoding now rejects oversized recipient and DSN-recipient arrays before normalization, routing, or retry bookkeeping.
490. Attachment scanner hook rejection/tempfail reasons are now CR/LF-stripped and UTF-8 safely bounded before they are surfaced as SMTP hook errors.
491. Redis duplicate-message detection now uses fixed-length hashed dedup keys so raw message IDs or recipient addresses cannot create oversized Redis keys.
492. Attachment scanning can now be enabled with `GOGOMAIL_ATTACHMENT_SCAN_BACKEND=webhook`, wiring a bounded HTTP scanner into Edge, Inbound, and Submission MTA app boundaries while remaining disabled by default.
493. Push notification workers can now use `GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webhook` to POST raw-token targets and candidate attempt IDs to an external push gateway while keeping first-party FCM/APNs/Web Push adapters outside the core.
494. Attachment scanner webhooks can now send an optional bounded bearer token, with CR/LF-bearing or oversized token configuration rejected before SMTP hook wiring.
495. Push notification webhooks can now send an optional bounded bearer token, with CR/LF-bearing or oversized token configuration rejected before worker sink wiring.
496. Attachment scanner and push notification webhook URLs now must be HTTPS in production, while still allowing HTTP endpoints for local development and private test harnesses.
497. README and example configuration now document attachment-scan and push-notification webhook backends, optional bearer tokens, timeouts, and production HTTPS requirements for operator rollout.
498. Webhook integration contracts now document attachment scanner and push gateway JSON payloads, authentication, HTTPS requirements, bounded response behavior, and push attempt-state semantics for external adapter rollout.
499. Push notification webhooks now bound and normalize message, recipient, subject, timestamp, and target metadata before JSON serialization, and drop malformed direct-call targets before external gateway handoff.
500. Attachment scanner webhooks now bound and normalize message, address, subject, recipient, and attachment metadata before JSON serialization, cap recipient and attachment arrays, and clamp negative message sizes to zero.
501. Push notification target resolution now drops oversized device IDs and tokens before candidate recording or sink handoff, and bounds optional labels/token suffixes at UTF-8 boundaries for cleaner provider adapter inputs.
502. Push-device create/update validation now rejects invalid-UTF-8, CR/LF-bearing, or oversized user and token metadata before repository upsert, keeping raw provider tokens bounded at the storage boundary.
503. Admin push-notification attempt and stats repository filters now reject invalid-UTF-8, CR/LF-bearing, or oversized direct-call values before SQL dispatch, aligning database access with HTTP query guardrails.
504. Push notification outcome recording now rejects invalid-UTF-8, CR/LF-bearing, or oversized attempt IDs before SQL update dispatch, keeping future provider adapters from sending unsafe attempt keys into storage.
505. Push notification candidate recording now rejects invalid-UTF-8, CR/LF-bearing, or oversized message/user/device/company/domain IDs before SQL insert dispatch, and rejects unsupported platforms at the recorder boundary.
506. Admin API now exposes `PATCH /admin/v1/push-notification-attempts/{id}/outcome` so authenticated operators or external push gateways can record queued, delivered, failed, or invalid-token outcomes with bounded provider diagnostics.
507. Admin API now exposes `GET /admin/v1/push-notification-attempts/{id}` for single push-attempt troubleshooting, with OpenAPI envelope coverage for generated operator clients.
508. Admin push-notification attempt listing now supports a bounded `message_id` filter so operators can inspect fan-out and provider outcomes for one stored message.
509. Push notification worker outcome recording now delegates to the shared `maildb` outcome update path used by the Admin API, keeping queued/delivered/failed/invalid-token validation and invalid-token device deletion consistent across internal sinks and provider callbacks.
510. Admin push-notification stats now supports a bounded `message_id` filter, matching attempt-list drilldowns so operators can summarize one stored message's fan-out outcomes before inspecting individual attempts.
511. Admin push-notification stats now supports a platform filter limited to `apns`, `fcm`, and `webpush`, letting operators isolate provider-platform fan-out failures without querying raw attempts first.
512. Admin push-notification stats now supports a bounded `device_id` filter, letting operators summarize one device's push lifecycle before drilling into individual attempt records.
513. `attachment-cleanup-worker` now runs stale attachment-upload cleanup as a configurable operational mode, expiring old `uploading` rows in bounded batches and deleting their stored objects through the mail service.
514. Attachment cleanup can run once and exit via `GOGOMAIL_ATTACHMENT_CLEANUP_RUN_ONCE=true`, supporting CronJob or timer-driven deployments without requiring a long-running worker.
515. Stale attachment cleanup now reports stored-object delete failures instead of silently swallowing them, while treating missing objects as idempotently cleaned so operators can see real storage cleanup drift.
516. Stale attachment cleanup now uses an attachment-specific 1000-row batch cap instead of the shared message-list pagination limit, keeping `GOGOMAIL_ATTACHMENT_CLEANUP_BATCH_SIZE` meaningful for operational sweeps.
517. Admin API now exposes `POST /admin/v1/attachment-cleanup/runs` for authenticated on-demand stale upload cleanup with an explicit non-future RFC3339 cutoff and bounded batch size.
518. Admin attachment cleanup runs now support `dry_run` previews that return total and batch-limited stale upload candidate counts before destructive cleanup.
519. Admin API now exposes `POST /admin/v1/attachment-cleanup/candidates` so operators can inspect the bounded stale upload candidate set before running cleanup.
520. Attachment cleanup candidate previews now include total and batch-limited candidate counts alongside the bounded candidate list.
521. Mail API now exposes user-scoped pending attachment upload cancellation, releasing reserved quota and deleting any stored upload object before stale cleanup is needed.
522. Mail API now exposes attachment upload capability discovery with current limits and supported modes, including an explicit `resumable_chunked_uploads=false` until that contract lands.
523. Draft attachment binding and send handoff now require `uploading` attachments, preventing canceled or deleted uploads from being rebound to drafts or moved onto sent messages.
524. Canceling a draft-bound attachment upload now refreshes the draft `has_attachment` cache in the same transaction as quota release.
525. Pending attachment upload cancellation now validates user and attachment identifiers at the service boundary, with HTTP regression coverage for unsafe cancel path IDs.
526. Pending attachment cancellation now clears any draft binding while marking the upload deleted, avoiding stale draft links in deleted attachment rows.
527. OpenAPI attachment status documentation now matches persisted API values (`uploading`, `stored`, `deleted`) and rejects the obsolete `active` enum in contract tests.
528. Attachment upload capabilities now lock the runtime byte-limit constants to the HTTP response and OpenAPI schema through regression tests for generated clients.
529. ADR 0007 defines resumable/chunked attachment uploads as explicit upload sessions with quota reservation, adapter-owned staged chunks, final attachment rows after assembly, and bounded cleanup.
530. `attachment_upload_sessions` migration prepares future resumable upload state with declared/received byte counters, lifecycle status, expiry, checksum, storage adapter metadata, and cleanup indexes.
531. `maildb` can create resumable attachment upload sessions and reserve declared bytes in the shared quota ledger transactionally, with optional PostgreSQL integration coverage.
532. `maildb` can cancel resumable attachment upload sessions and release declared quota reservations exactly once, rejecting repeated cancellation of already terminal sessions.
533. `maildb` can expire stale resumable attachment upload sessions in bounded batches, marking them `expired` and releasing declared quota reservations.
534. `mailservice` exposes resumable upload session create/cancel/expire methods over the repository boundary while preserving metadata validation, max-size checks, and domain attachment policy enforcement.
535. `attachment-cleanup-worker` now expires stale resumable attachment upload sessions during the normal bounded cleanup sweep so abandoned sessions release reserved quota without a separate worker.
536. Mail API exposes resumable attachment upload session create/read/cancel endpoints with explicit quota reservation while keeping chunked upload capability disabled until receive/finalize contracts land.
537. Attachment upload capabilities now advertise upload session create/cancel support separately from full resumable chunk support so clients can stage integration safely.
538. Upload session creation now rejects already-expired `expires_at` values at the service boundary before any quota reservation is attempted.
539. Upload session creation now caps client-requested expiries to a 24-hour service TTL and advertises that limit through attachment upload capabilities.
540. Mail API can store a complete upload session body, persist it under session-scoped storage, and record received bytes plus SHA-256 digest without creating the final attachment row.
541. Upload session finalization converts a ready stored session body into the normal pending attachment row without double-reserving quota and marks the session finalized.
542. Upload session cancellation now deletes staged session bodies when present, keeping storage cleanup aligned with quota release.
543. Upload session expiry now deletes staged session bodies when present, keeping worker-driven cleanup aligned with quota release.
544. Optional PostgreSQL integration coverage now verifies upload session finalization creates an attachment row without double-reserving quota.
545. Optional PostgreSQL integration coverage now verifies duplicate upload session finalization does not change quota or create extra attachment rows.
546. Upload session body storage now maps over-limit HTTP request bodies to the shared 413 `payload_too_large` envelope before storage work.
547. Upload session body storage now has regression coverage that terminal sessions are rejected before storage writes or repository body-recording.
548. Upload session body storage now accepts an optional `X-Content-SHA256` precondition and rejects checksum mismatches before recording staged body metadata.
549. Attachment upload capabilities now advertise upload session checksum precondition support separately from body storage and finalization support.
550. Optional PostgreSQL integration coverage now verifies upload session finalization rejects unstored bodies without changing quota or creating empty attachment rows.
551. OpenAPI contract tests now lock the upload session body `X-Content-SHA256` header so generated clients keep the integrity precondition.
552. Upload session body storage now explicitly rejects `Content-Range` requests while full range-aware chunk semantics remain disabled.
553. Upload session finalization now verifies staged object existence, byte count, and SHA-256 before creating the attachment row.
554. Admin attachment cleanup run and dry-run responses now include stale upload-session candidate, limited, and expired counts, aligning operator previews with the background worker's full cleanup scope.
555. Admin attachment cleanup candidate previews now include bounded stale upload-session rows alongside legacy attachment-upload rows, giving operators row-level visibility before cleanup releases quota reservations.
556. Upload session body replacement now writes each retry to a distinct staged object path before repository metadata update, preserving the previously recorded body if the database update fails and best-effort cleaning the previous staged body after successful replacement.
557. Admin API usage ledger retention runs can now dry-run or delete a bounded batch of immutable ledger rows only after reusing the retention-readiness gate, with destructive runs requiring explicit `confirm_ready`.
558. Optional PostgreSQL integration coverage now verifies API usage retention runs preserve blocked candidates, keep dry-runs read-only, delete only the requested ready batch, and leave newer ledger rows intact.
559. Admin API now exposes list/detail reads for API usage ledger retention-run audit rows, making blocked, dry-run, and destructive ledger purge attempts inspectable after execution.
560. Quota reconciliation corrections now record bounded audit-log detail for dry-run and applied attempts, including before/after drift counts and samples in the same correction transaction.
561. Admin API now exposes bounded audit-log list/detail reads with operational filters, making persisted audit records inspectable without direct database access.
562. Domain DNS check and quota reconciliation correction audit records now reuse the shared audit hash-chain writer, avoiding empty-hash audit rows in operator-visible trails.
563. Admin API now exposes a bounded audit-log integrity check that recomputes row hashes and reports hash-chain breaks for recent audit windows.
564. `api-usage-retention-worker` now runs bounded API usage ledger retention on an interval or once-and-exit, dry-run by default and guarded by the existing readiness/confirm gates.
565. Destructive `api-usage-retention-worker` configuration now requires a production-oriented `remote-ed25519` export manifest signer backend in addition to explicit `confirm_ready`.
566. API usage export capabilities now advertise bounded retention-run support, retention-worker support, and the remote-key requirement for destructive worker purges.
567. API usage ledger retention now rejects future cutoffs inside the repository boundary, aligning worker/direct-call behavior with the Admin API future-cutoff guard.
568. Draft search now has a separate compose-focused Mail API contract, keeping active-message search and OpenSearch relevance semantics separate from draft body/recipient lookup.
569. `gogomail --mode=imap` now initializes a service-backed IMAP gateway scaffold with a process-local mailbox event broker while keeping the TCP IMAP protocol listener deferred.
570. `gogomail --mode=all-in-one` now registers Mail API and Admin API routes in one HTTP process, making the documented single-node mode usable for local release smoke tests.
571. Admin user creation and password-hash rotation now accept validated `password_hash` values, giving operators a supported path to create and maintain SMTP Submission-capable local users without direct database writes.
572. Admin user read models now expose `password_configured`, giving operators safe visibility into Submission login readiness without returning password hashes.
573. Admin user listing now supports a bounded `password_configured=true|false` filter, letting operators find Submission-ready or not-yet-configured local users without direct database queries.
574. Admin user listing now also supports a status filter over `active`, `suspended`, and `disabled`, aligning list triage with the existing user status lifecycle.
575. Admin DKIM key listing now supports `status=active|inactive`, letting operators inspect current signing keys or retired keys without direct database queries.
576. Admin suppression-list reads now support bounded domain, email, and reason filters, letting operators triage bounce suppression state without direct database queries.
577. Outbound SMTP now fails closed for internationalized envelope addresses when a remote MTA does not advertise SMTPUTF8, preventing accidental RFC 6531 option leakage to non-EAI peers.
578. Admin domain listing now supports company, lifecycle status, and latest DNS-status filters, letting operators triage tenant onboarding and suspended domains without client-side full-list scans.
579. Admin delivery-route listing now supports status, farm, and domain-pattern filters, making route audits and incident triage possible without client-side full-list scans.
580. Shared EML parsing now caps retained subject, address, message-id, and reference metadata at UTF-8 boundaries and drops oversized message IDs instead of storing malformed partial IDs, bounding parser output before SMTP receive, Mail API, search indexing, and future IMAP consumers persist it.
581. Admin company listing now supports lifecycle status filters, letting operators isolate active, suspended, or disabled tenant accounts without client-side full-list scans.
582. Delivery `mail.queued` decoding now rejects ambiguous, absolute, parent-traversal, backslash-bearing, or non-`.eml` storage object keys before workers open queued message bodies, aligning delivery storage hygiene with search indexing.
583. OpenSearch indexing now rejects blank, CR/LF-bearing, or oversized message IDs before building `_doc/{id}` URLs and uses the same cleaned ID in JSON payload metadata.
584. Admin delivery-attempt list, stats, and exhausted-attempt reads now support bounded message-id, farm, and sender filters, letting operators triage one failed message, sender, or delivery farm without direct SQL.
585. Admin API usage daily/monthly aggregate reads now support bounded tenant, company, domain, user, API-key, principal, auth-source, method, route, status, and time-window filters, making aggregate billing/incident triage possible without global scans.
586. Admin quota usage pressure reads now support scope, domain, over-limit, and over-allocation filters, letting operators isolate quota hot spots without scanning every company/domain/user row client-side.
587. Attachment upload-session staged object paths are now validated as relative `upload-sessions/` keys before repository persistence and before service-side storage reads/deletes, hardening finalize/cancel/expiry flows against corrupted stored paths.
588. Mailservice now validates DB-returned message and attachment storage paths before body reads or cleanup deletes, preventing corrupted rows from sending absolute, traversal, newline, backslash-bearing, or blank required object keys to storage adapters.
589. Local storage now enforces a shared strict object-key validator before reads, writes, and deletes, rejecting non-canonical relative keys such as duplicate separators, dot segments, absolute paths, traversal, newlines, and backslashes.
590. Admin trusted relay listing now supports bounded CIDR and description filters, letting operators audit inbound relay policy without client-side full-list scans.
591. Admin domain DNS-check history now supports summary-status and RFC3339 `since` filters, letting operators inspect recent onboarding and deliverability failures without re-querying DNS or scanning every persisted check.
592. Admin API usage export batch listing now supports bounded tenant, principal, status, and export-window filters, letting operators find covering manifests for handoff and retention checks without global scans.
593. HTTP readiness can now include runtime database and Redis dependency probes for Mail/Admin/API-metered modes and returns a degraded 503 readiness response when an injected dependency check fails.
594. Admin attachment upload-session listing now supports bounded user, draft, and lifecycle-status filters, giving operators direct visibility into pending, uploading, finalized, canceled, or expired resumable sessions.
595. Trusted relay create/delete mutations now persist hash-chain admin audit rows in the same database transaction as the relay policy change, making inbound relay administration tamper-evident through the Admin audit surface.
596. Delivery route create/status/delete mutations now persist hash-chain admin audit rows in the same database transaction as the gateway policy change, while excluding relay authentication passwords from audit detail.
597. DKIM key create/upsert, deactivate, and DNS-verification mutations now persist hash-chain admin audit rows in the same database transaction as the key lifecycle change, while excluding private key material from audit detail.
598. Domain and user lifecycle status updates now persist hash-chain admin audit rows in the same database transaction as the status change, scoped by company/domain identifiers for tenant forensics.
599. Company, domain, and user quota mutations now persist hash-chain admin audit rows in the same database transaction as the quota change, including domain default user quota propagation counts for quota forensics.
600. Domain policy mutations now persist hash-chain admin audit rows in the same database transaction as the policy change, preserving inbound/outbound mode and size guardrail evidence for enforcement forensics.
601. Domain/user provisioning and user password-hash rotation now persist hash-chain admin audit rows in the same database transaction as the change, while exposing password readiness without leaking password hash material.
602. Bounce DSN generation now honors RFC 3461 `RET=HDRS` by carrying safe original `.eml` storage paths through delivery events and attaching bounded sanitized original headers as `text/rfc822-headers` in generated RFC 3464 reports.
603. Migration file guardrails now require every SQL migration to declare explicit goose Up/Down sections, and the legacy API-usage, push, IMAP, and audit-index migrations have been normalized to that structure without changing applied SQL.
604. Runtime database readiness now checks the applied goose migration version against the latest local SQL migration before reporting database probes healthy, preventing stale schemas from passing `/health/ready` on ping alone.
605. Admin backpressure overrides now persist bounded hash-chain audit rows after Redis state changes, recording previous/current SMTP pressure levels as durable evidence for receive-throttle operations.
606. Admin suppression-list deletes now persist hash-chain audit rows in the same transaction as the delete, preserving deliverability-control removal evidence for operator forensics.
607. Admin outbox retry now persists a hash-chain audit row in the same transaction as the retry reset, preserving previous event status, attempts, and bounded error evidence before operator replay.
608. Admin push-notification outcome updates now persist hash-chain audit rows in the same transaction as provider-status changes and invalid-token device cleanup, without including raw push tokens or token suffixes.
609. Admin attachment cleanup runs now persist bounded hash-chain audit rows after stale upload and upload-session expiry sweeps, recording cutoff, normalized limit, expired counts, and ID samples without storage paths.
610. Admin IMAP UID backfill now persists a hash-chain audit row in the same transaction as UID assignment, recording mailbox/user scope, normalized limit, assigned count, and a bounded message/UID sample.
611. Admin API-usage export batch creation now persists a hash-chain audit row in the same transaction as the batch, recording tenant/principal scope, export window, counts, byte totals, latency totals, and export format.
612. Admin API-usage export artifact creation/upsert now persists a hash-chain audit row in the same transaction as artifact persistence, recording object key, storage backend, content type, byte/event counts, and SHA-256 digest evidence.
613. Admin API-usage export manifest digest and signature creation now persist hash-chain audit rows in the same transaction as the evidence rows, recording bounded digest/signature evidence without copying raw manifests, metadata, or full signature material.
614. Admin API-usage ledger retention runs now persist hash-chain audit rows in the same transaction as run records and destructive deletes, recording dry-run, blocked, no-op, and completed outcomes with bounded readiness evidence.
615. Mail/Admin HTTP readiness now probes local storage with a write/read/delete cycle and rejects unsupported HTTP storage backends at startup instead of silently wiring local storage.
616. SMTP, Submission, Delivery, Event, Search Index, IMAP scaffold, attachment cleanup, and HTTP runtimes now share storage backend validation, preventing unsupported object-storage settings from silently using the local adapter.
617. The shared HTTP server now has configurable and validated read, write, idle, read-header, and maximum-header guardrails for Mail/Admin/API-metered modes.
618. Mail and Admin API JSON request decoding now rejects unknown object fields before service dispatch, making generated-client and OpenAPI drift visible as HTTP 400 errors.
619. API error responses now return `Cache-Control: no-store` and `X-Content-Type-Options: nosniff`, and the reusable OpenAPI error response documents both headers.
620. Mail and Admin API scalar query parameters now reject duplicate values before dispatch, preventing HTTP parameter pollution ambiguity for user IDs, limits, booleans, timestamps, and operational filters.
621. Direct multipart attachment uploads now reject repeated `draft_id` or `file` parts and no longer accept `draft_id` from the URL query string, preventing ambiguous form metadata before storage writes.
622. Successful Mail/Admin JSON, health, and service-info responses now return `X-Content-Type-Options: nosniff`, aligning browser-facing API envelopes with error, NDJSON, and download response hardening.
623. Successful Mail/Admin JSON envelopes now return `Cache-Control: no-store` through the shared writer, preventing sensitive message, audit, usage, and control responses from being cached.
624. Mail JWT and Admin token authentication now reject repeated credential headers, and Admin routes reject mixed `X-Admin-Token` plus bearer credentials before dispatch.
625. Upload session body storage now rejects repeated `Content-Range` or `X-Content-SHA256` control headers before reading or storing the request body.
626. Mail and Admin API JSON mutation bodies now require `Content-Type: application/json`, accepting normal media-type parameters but rejecting missing or non-JSON content types before dispatch.
627. Shared storage object path validation now enforces total-key and per-segment byte caps before local storage, mailservice body reads, cleanup deletes, and artifact/storage-key callers reach storage adapters.
628. Shared audit-log normalization now bounds scalar metadata and JSON detail size before hash computation or database insertion, keeping every audit producer on the same persistence guardrails.
629. Mail and Admin API JSON mutation bodies now reject repeated `Content-Type` headers before JSON decoding, preventing ambiguous media-type interpretation at the HTTP boundary.
630. Shared EML parsing now pre-bounds oversized structured address and message-id-list headers before list parsing, preserving parser hot-path memory guardrails in addition to retained metadata caps.
631. Shared EML parsing now pre-bounds oversized Subject headers before RFC 2047 decoding, preserving parser hot-path memory guardrails while retaining normal encoded-subject support.
632. Mail API read/search/list routes now reject unknown query parameter names before dispatch, making generated-client typos visible as HTTP 400 responses instead of silently ignoring them.
633. Admin company/domain/DNS-check/user list routes now reject unknown query parameter names before dispatch, keeping core operator filters aligned with the documented contract.
634. Admin API usage aggregate, ledger, retention, export-batch, artifact, manifest-digest, and manifest-signature routes now reject unknown query parameter names before dispatch, including unexpected query strings on detail, download, verification, and mutation routes with no query controls.
635. Mail API draft-search, attachment capability/session/download, and push-device list routes now reject unknown query parameter names before dispatch, extending generated-client typo detection beyond the primary mailbox read routes.
636. Admin queue, outbox, audit, backpressure, quota, attachment-session, delivery-attempt, push-notification, suppression-list, trusted-relay, delivery-route, and DKIM read routes now reject unknown query parameter names before dispatch, extending generated-client typo detection across operator read surfaces.
637. Mail API read and bodyless mutation routes now reject request bodies and `Content-Type` headers before dispatch, preventing ignored JSON or multipart metadata on resource reads, deletes, draft-send, upload-session finalization, capability discovery, downloads, and push-device list/delete operations.
638. Admin GET/DELETE routes and bodyless Admin POST commands now reject request bodies and `Content-Type` headers before dispatch, preventing ignored payloads on operator reads, deletes, route verification, retry, IMAP UID backfill, API-usage export-batch creation, and manifest digest/signature creation.
639. Redis stream consumers now inspect Redis pending delivery counts and move repeatedly handler-failing messages into a durable dead-letter stream before acknowledging the original event, preventing one poison event from pinning event/search/API-metering/push/delivery workers forever while still allowing transient handler retries first.
640. Event, search-index, API-metering, push-notification, and delivery workers now expose per-worker Redis consumer max-delivery and dead-letter-stream configuration, making poison-event handling tunable without code changes.
641. IMAP UID assignment now reuses existing UIDs only when the message is still active in the same user/mailbox and treats stale moved/deleted `mail.stored` events as no-ops, preventing cross-mailbox UID reuse and permanent retries from legitimate mailbox races.
642. Mail API thread list and per-thread message reads now support opaque cursor pagination with `limit`, `has_more`, and `next_cursor`, making large conversation mailboxes navigable without client-side full-list scans.
643. Mail API draft search now supports opaque cursor pagination with `limit`, `has_more`, and `next_cursor`, making large compose draft lists navigable without loading every active draft.
644. OpenAPI now wires the development-only `user_id` fallback parameter into every user-scoped Mail operation that can use it when JWT auth is disabled, keeping generated local/all-in-one clients aligned with runtime request scoping.
645. Retry-exhausted delivery events now carry recipient-level RFC 3461 DSN metadata and safe original-message storage paths into the event worker, letting exhausted temporary failures generate sender-facing RFC 3464 failure DSNs with deterministic dedupe while user delivery status classifies terminal `exhausted` attempts as failed.
646. Mail API mutation routes now reject unknown query parameter names before dispatch, and JSON-backed compose, draft, attachment-reservation, and send mutations honor the documented development-only `user_id` query fallback when JWT auth is disabled.
647. Admin bodyless command/delete routes now reject unknown query parameter names before dispatch for IMAP UID backfill, DKIM DNS verification, outbox retry, DKIM deactivation, suppression deletion, trusted-relay deletion, and delivery-route deletion, preventing ignored operator intent flags on sensitive actions.
648. Admin JSON mutation routes now reject unknown query parameter names before dispatch for tenant quota, domain/user lifecycle and policy, backpressure, attachment cleanup, quota correction, push outcome, trusted-relay, delivery-route, and DKIM-key mutations.
649. OpenAPI contract tests now parse `docs/openapi.yaml` as YAML and reject stale documented routes that are not registered by the Go HTTP mux, catching generated-client blocking spec syntax errors and obsolete endpoint contracts.
650. Bounce DSN generation now honors RFC 3461 `RET=FULL` by attaching a bounded, parse-validated original `.eml` as `message/rfc822` in generated RFC 3464 reports while preserving existing `RET=HDRS` header-only behavior.
651. Health and service-info GET routes now reject request bodies and `Content-Type` headers before returning probe or contract metadata responses, keeping unauthenticated release probes aligned with bodyless HTTP read semantics.
652. Authentication-Results trace header formatting now strips control characters and bounds authserv-id, reason, domain, and identifier metadata before formatting SPF/DKIM/DMARC results, preventing verifier diagnostics from injecting or bloating stored headers.
653. Health and service-info GET routes now reject unknown query parameter names, making release probe and metadata endpoint typos visible as HTTP 400 instead of silently ignored inputs.
654. Runtime config validation now restricts `GOGOMAIL_ENV` to `development`, `test`, or `production`, preventing environment typos from silently bypassing production-only safety gates.
655. Runtime config validation now restricts Redis-backed deduplication, recipient rate limiting, and SMTP backpressure backend selectors to `none` or `redis`, preventing typos from silently disabling operational controls.
656. Runtime config validation now checks HTTP, SMTP, inbound SMTP, Submission, and optional SMTPS listener addresses as TCP `host:port` values, surfacing bind configuration mistakes before runtime listener setup.
657. Runtime config validation now requires delivery retry delay schedules and maximum delay caps to be positive durations, preventing malformed retry configuration from exhausting retries immediately or scheduling jobs in the past.
658. Runtime config validation now checks OpenSearch endpoints as HTTP(S) URLs with hosts when `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`, failing malformed search backend configuration before worker/search adapter construction.
659. Runtime config validation now checks OpenSearch index names with the adapter's unsafe-character and reserved-prefix guardrails when `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`, failing invalid index configuration before worker/search setup.
660. Runtime config validation now checks `GOGOMAIL_DELIVERY_SMTP_HELLO` as a non-empty whitespace-free hostname, surfacing outbound SMTP EHLO configuration mistakes before delivery worker startup.
661. Runtime config validation now requires RCPT rate-limit and outbox relay batch, poll, and max-attempt settings to be positive, surfacing relay/limit misconfiguration before workers start.
662. Redis-backed RCPT rate-limit keys now normalize remote addresses to the remote host/IP bucket instead of the full `ip:port`, preventing source-port churn from bypassing recipient abuse controls.
663. Authenticated Submission now applies enforcing per-domain recipient caps during `RCPT TO`, not only after `DATA`, giving clients earlier SMTP feedback before message streaming/spooling.
664. Mail API detail reads that auto-mark unread messages as read now publish best-effort IMAP `flags` events for UID-visible messages after a successful read-flag write, keeping future IMAP subscribers aligned with webmail reads.
665. Runtime config validation now requires Redis worker stream, group, and consumer-name settings for event, search-index, API-metering, push-notification, and delivery workers to be non-empty, CR/LF-free, and size-bounded, surfacing worker identity mistakes before consumer construction.
666. `eventstream.NewRedisConsumer` now trims and validates Redis stream, group, and consumer identifiers as required, CR/LF-free, size-bounded values, keeping direct adapter callers aligned with runtime config validation.
667. OpenSearch writer/searcher construction now trims usernames while preserving password bytes, and rejects CR/LF-bearing or oversized endpoint credentials before BasicAuth request headers can be generated.
668. Runtime config validation now rejects CR/LF-bearing or oversized OpenSearch username/password configuration when `GOGOMAIL_SEARCH_INDEX_BACKEND=opensearch`, surfacing credential formatting mistakes before worker/search setup.
669. OpenSearch index/bootstrap/search status-error diagnostics now collapse backend response bodies into bounded one-line UTF-8 previews, preventing CR/LF-bearing backend errors from leaking into logs or API diagnostics.
670. Runtime config validation now rejects static smart-host password-only auth plus CR/LF-bearing or oversized auth username, password, and identity values, matching Admin delivery-route guardrails before delivery worker startup.
671. Admin delivery-route creation now rejects oversized farm, SMTP hello, pool, description, and relay auth identity/username/password metadata before route storage or audit work.
672. Remote Ed25519 manifest signer status-error diagnostics now collapse signer response bodies into bounded one-line UTF-8 previews, preventing CR/LF-bearing external signer errors from leaking into export/billing diagnostics.
673. Attachment scanner and push-notification webhook senders now reject CR/LF-bearing
    configured tokens or endpoints and collapse non-2xx response bodies into bounded
    one-line UTF-8 previews before surfacing endpoint failures.
674. OpenSearch writer construction now rejects CR/LF-bearing direct endpoint
     values before URL parsing, keeping adapter calls aligned with startup config
     endpoint validation.
675. Runtime config now loads and validates `GOGOMAIL_IMAP_ADDR` as required TCP
     listener metadata for the IMAP scaffold, preparing future protocol listener
     wiring without opening the port yet.
676. `mailservice.IMAPStoreAdapter` now satisfies `imapgw.MailboxSessionStore`
     for SELECT-style mailbox state, service-backed COPY/MOVE/EXPUNGE, and
     event subscriptions.
677. Runtime storage wiring now supports `GOGOMAIL_STORAGE_BACKEND=s3` and
     `GOGOMAIL_STORAGE_BACKEND=minio` through a standard SigV4 S3-compatible
     adapter with endpoint, region, bucket, prefix, credential, session-token,
     and path-style settings, while preserving local filesystem/NFS storage as
     the default backend.
678. Optional S3-compatible integration coverage can now exercise real
     `PUT`/`GET`/`DELETE` round trips against MinIO or AWS S3 when
     `GOGOMAIL_TEST_S3_ENDPOINT`, bucket, and credential environment variables
     are configured.
678a. Drive runtime storage wiring now supports explicit compatibility labels
      through `GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS`, allowing staged
      local/NFS-to-S3-compatible migrations to serve historical
      `storage_backend` row labels through the active store only when
      operators intentionally opt into that label map.
679. ADR 0008 now records IMAP authentication and session semantics: protocol
     auth should use a dedicated adapter over local user password hashes, JWT
     stays HTTP-only, production auth requires TLS policy review, and
     `\Deleted`/EXPUNGE/MOVE stay separated from gogomail soft-delete and
     ordinary Mail API folder-move semantics.
680. `mailservice.NewIMAPAuthenticatorAdapter` now maps the existing
     Submission/local-password authentication boundary into `imapgw.Session`
     values, giving the future listener a protocol-native authenticator without
     coupling IMAP to JWT middleware.
681. `mailservice.NewIMAPBackendAdapter` now composes the protocol authenticator
     with the service-backed store/session adapter, giving the future TCP
     listener a single `imapgw.Backend` boundary.
682. Runtime config now loads and validates IMAP TLS certificate/key paths plus
     `GOGOMAIL_IMAP_ALLOW_INSECURE_AUTH`, preventing production IMAP auth from
     being enabled with cleartext credential policy.
683. IMAP runtime TLS helper groundwork can load IMAP-specific certificate/key
     files with TLS 1.2 minimum and derive the server name from the IMAP
     listener host before falling back to `GOGOMAIL_SMTP_DOMAIN`.
684. S3-compatible storage status-error diagnostics now collapse backend
     response bodies into bounded one-line UTF-8 previews, preventing
     CR/LF-bearing object-store errors from leaking into readiness or storage
     operation diagnostics.
685. S3-compatible bucket names are now validated with shared adapter/config
     guardrails before runtime wiring, surfacing uppercase, undersized,
     slash-bearing, or punctuation-adjacent deployment mistakes before storage
     calls.
686. S3-compatible regions are now validated with shared adapter/config
     guardrails before SigV4 signing, rejecting blank, whitespace-bearing,
     slash-bearing, or uppercase region values before object-storage requests
     are created.
687. S3-compatible object prefixes are now validated as canonical relative
     object-key prefixes during config validation, surfacing duplicate
     separators, dot segments, traversal, or backslash mistakes before adapter
     construction.
688. IMAP runtime now builds listener-ready server options containing address,
     backend, TLS config, and insecure-auth policy while still deferring the
     actual TCP protocol server.
689. `internal/imapgw.NewServer` now provides a protocol-server lifecycle shell
     with listener option validation, backend requirement checks, and
     TLS/insecure-auth policy enforcement before the IMAP command parser is
     wired.
690. The IMAP server shell can now serve an initial connection greeting plus
     unauthenticated `CAPABILITY`, `NOOP`, `LOGIN`, and `LOGOUT` responses,
     giving TCP clients a bounded RFC-shaped handshake/auth surface before
     mailbox commands are enabled.
691. Authenticated IMAP `SELECT` now maps to `imapgw.MailboxSessionStore`,
     returning permanent flags, `EXISTS`, `UIDVALIDITY`, `UIDNEXT`, and
     read-write completion metadata from the service-backed mailbox state.
692. Authenticated IMAP `LIST` now maps to the service-backed mailbox list and
     returns sanitized quoted mailbox names with hierarchy delimiters.
693. Authenticated IMAP `STATUS` now maps to service-backed mailbox state and
     returns `MESSAGES`, `UIDNEXT`, `UIDVALIDITY`, and `UNSEEN` metadata.
694. IMAP command parsing now supports basic quoted strings with backslash
     escapes, allowing common quoted `LOGIN` credentials and mailbox atoms while
     rejecting malformed quoted controls.
695. Authenticated selected-mailbox `UID FETCH` can now return UID, flags,
     RFC822 size metadata, and `BODY[]` literals streamed from the
     service-backed raw message fetch boundary.
696. Authenticated selected-mailbox `UID STORE` now maps `FLAGS`, `+FLAGS`, and
     `-FLAGS` for supported system flags to the service-backed flag mutation
     boundary and returns updated flag metadata.
697. `gogomail --mode=imap` now opens the configured TCP listener and serves the
     IMAP server shell with greeting, `CAPABILITY`, `NOOP`, `LOGIN`, `LIST`,
     `SELECT`, metadata/body `UID FETCH`, `UID STORE`, and `LOGOUT`, while
     broader body-section FETCH and IDLE work remains deferred.
698. IMAP listener creation now uses a TLS listener whenever IMAP TLS config is
     present, keeping the runtime listener path aligned with the authentication
     policy guardrails.
699. IMAP `UID FETCH` and `UID STORE` responses now use RFC 3501 message
     sequence numbers for untagged `FETCH` data while preserving the requested
     UID inside the response attributes; `RFC822.SIZE` is treated as metadata,
     not a body-fetch request.
700. IMAP `CAPABILITY` responses now reflect connection state by dropping
     `AUTH=PLAIN` after authentication, and the command parser rejects
     unsupported literal tokens instead of treating them as atoms.
701. IMAP `AUTHENTICATE PLAIN` now supports the standard continuation flow with
     SASL PLAIN base64 decoding, cancellation handling, and post-auth
     capability state updates.
702. IMAP `UID FETCH` now accepts bounded numeric UID sets and ranges, including
     comma-separated and reverse range forms, and treats `BODY.PEEK[]` as a body
     fetch request without changing the stored flag state.
703. IMAP `FETCH` now accepts bounded message sequence sets, including `*`,
     resolves them through the selected mailbox message list, and returns the
     same RFC-shaped untagged `FETCH` metadata/body responses as `UID FETCH`.
704. IMAP `EXAMINE` now shares the mailbox selection response path with
     `SELECT`, returns `[READ-ONLY]`, and prevents `UID STORE` mutations while a
     mailbox is selected read-only.
705. IMAP `CHECK` and `CLOSE` now cover the selected-mailbox lifecycle: `CHECK`
     completes as a safe synchronization no-op, and `CLOSE` clears selected
     state without expunging while `\Deleted` semantics remain unsupported.
706. IMAP `STATUS` now validates requested status data items and returns only
     the requested RFC-shaped fields, including `RECENT`, instead of always
     emitting a fixed mailbox metadata set.
707. IMAP `LIST` now applies basic mailbox pattern matching for exact names,
     `*`, and `%`, and matches against sanitized wire names before emitting
     quoted mailbox responses.
708. IMAP `SELECT` and `EXAMINE` now emit `[PERMANENTFLAGS]` response codes,
     advertising writable flags for read-write selections and no permanent
     flags for read-only `EXAMINE` state.
709. IMAP `UID STORE` now supports `FLAGS.SILENT`, `+FLAGS.SILENT`, and
     `-FLAGS.SILENT`, applying the same flag mutations while suppressing
     untagged `FETCH` echo responses.
710. IMAP `FETCH`/`UID FETCH` can now include `INTERNALDATE` and RFC-shaped
     `ENVELOPE` attributes from the service-backed message summary, giving
     clients enough structured metadata for mailbox list rendering.
711. IMAP `FETCH`/`UID FETCH` now keep a conservative single-part
     `BODYSTRUCTURE` fallback when only message headers are available, while
     metadata-only structure fetches can reopen the bounded raw message stream
     for richer MIME tree serialization.
712. IMAP `FETCH`/`UID FETCH` can now stream bounded header-only literals for
     `BODY[HEADER]`, `BODY.PEEK[HEADER]`, and `RFC822.HEADER`, stopping at the
     RFC message header/body separator instead of returning the full message.
713. IMAP `FETCH`/`UID FETCH` now supports `BODY[TEXT]`, `BODY.PEEK[TEXT]`, and
     `RFC822.TEXT` section literals by streaming only the message body after the
     header separator.
714. IMAP `UID STORE` now accepts bounded UID sets and ranges, applying flag
     mutations across multiple UIDs in one command and returning per-message
     untagged flag updates when not using `.SILENT`.
715. IMAP `NOOP` now drains queued selected-mailbox events from the mailbox
     event broker, emitting untagged `EXISTS` and flag `FETCH` updates as a
     polling-friendly synchronization path before full IDLE support.
716. IMAP now advertises and accepts `IDLE`, entering continuation mode and
     draining queued selected-mailbox events when the client sends `DONE`.
717. IMAP `SEARCH ALL`, `SEARCH UID <set>`, and `UID SEARCH ALL` now resolve
     against the selected mailbox message list, returning sequence numbers or
     UIDs according to RFC command mode.
718. IMAP `FETCH`/`UID FETCH` now supports bounded partial full-body literals
     for `BODY[]<offset.count>` and `BODY.PEEK[]<offset.count>`, streaming only
     the requested byte window.
719. IMAP `SEARCH` and `UID SEARCH` now support common flag criteria: `SEEN`,
     `UNSEEN`, `FLAGGED`, `UNFLAGGED`, `ANSWERED`, `UNANSWERED`, `DRAFT`, and
     `UNDRAFT`.
720. IMAP `FETCH`/`UID FETCH` now supports `BODY[HEADER.FIELDS (...)]` and
     `BODY.PEEK[HEADER.FIELDS (...)]`, returning bounded header literals with
     only the requested fields and continuations.
721. IMAP `FETCH`/`UID FETCH` now supports `BODY[HEADER.FIELDS.NOT (...)]` and
     `BODY.PEEK[HEADER.FIELDS.NOT (...)]`, returning bounded header literals
     with requested fields excluded.
722. IMAP `SEARCH` and `UID SEARCH` now support `SINCE`, `BEFORE`, and `ON`
     date criteria over message `INTERNALDATE`, using RFC-style `DD-Mon-YYYY`
     criteria parsing.
723. IMAP `SEARCH` and `UID SEARCH` now support basic `FROM` and `SUBJECT`
     substring criteria over selected-mailbox message summaries.
724. IMAP now supports authenticated `NAMESPACE`, advertising a personal
     namespace with `/` hierarchy delimiter for client mailbox discovery.
725. IMAP now supports authenticated `LSUB` over the same mailbox pattern
     matching path as `LIST`, returning subscribed-compatible mailbox
     responses.
726. IMAP now accepts authenticated `SUBSCRIBE` and `UNSUBSCRIBE` after mailbox
     existence checks, keeping client subscription flows unblocked.
727. IMAP now advertises and supports the RFC 2971-style `ID` command, returning
     a bounded server identity response for client compatibility diagnostics.
728. IMAP now advertises and supports `UNSELECT`, clearing selected-mailbox
     state and event subscriptions without invoking `CLOSE`/EXPUNGE semantics.
729. IMAP `EXPUNGE` and `UID EXPUNGE` now delete only messages marked with the
     IMAP-specific `\Deleted` flag, emit RFC-shaped untagged sequence-number
     `EXPUNGE` responses, remove stale mailbox UID rows, and publish
     best-effort expunge events.
730. IMAP now supports `STARTTLS` on plaintext listeners with configured TLS,
     advertising it only before authentication and removing it after the
     connection upgrades.
731. IMAP `IDLE` now streams selected-mailbox events while the client waits,
     sending untagged `EXISTS` and flag `FETCH` updates before `DONE` completes
     the command.
732. IMAP `SEARCH` and `UID SEARCH` now support address-list criteria for `TO`,
     `CC`, and `BCC` alongside existing `FROM` and `SUBJECT` summary searches.
733. IMAP `SEARCH` and `UID SEARCH` now support sent-date criteria
     `SENTSINCE`, `SENTBEFORE`, and `SENTON` over message envelope dates.
734. IMAP `SEARCH` and `UID SEARCH` now support RFC 3501 `LARGER` and
     `SMALLER` size criteria over message `RFC822.SIZE` metadata.
735. IMAP `SEARCH` and `UID SEARCH` can combine supported criteria with the RFC
     default AND semantics, including `ALL` plus flag, date, size, address, and
     UID filters.
736. IMAP `SEARCH` and `UID SEARCH` now support RFC `NOT` and binary `OR`
     criteria composition over the supported predicate set.
737. IMAP `FETCH` and `UID FETCH` now support standard `FAST`, `ALL`, and
     `FULL` macros, including the non-extensible `BODY` attribute used by
     `FULL`.
738. IMAP now advertises `SASL-IR` before authentication and accepts
     `AUTHENTICATE PLAIN` initial responses for compatible clients that avoid
     an extra SASL continuation round trip.
739. IMAP plaintext sessions now advertise `LOGINDISABLED` and reject
     `LOGIN`/`AUTHENTICATE` with `[PRIVACYREQUIRED]` when insecure auth is
     disabled before STARTTLS.
740. IMAP `SEARCH` and `UID SEARCH` now support bounded `BODY` and `TEXT`
     raw-message criteria scans, with `BODY` excluding the RFC 5322 header
     block.
741. IMAP `SEARCH` and `UID SEARCH` now support bounded RFC
     `HEADER <field> <value>` criteria scans over the raw message header block.
742. IMAP `COPY` and `UID COPY` now resolve source sequence/UID sets, validate
     the destination mailbox, duplicate active message metadata and attachment
     rows transactionally, assign fresh destination mailbox UIDs, and publish
     best-effort destination `EXISTS` events through the service boundary.
743. IMAP non-UID `STORE` now accepts bounded sequence sets/ranges and maps
     them to the same service-backed flag mutation boundary as `UID STORE`.
744. IMAP non-UID `STORE` now supports `.SILENT` flag mutation modes and
     suppresses untagged flag echo responses for those requests.
745. IMAP `CREATE`, `DELETE`, and `RENAME` now delegate to the service folder
     boundary for authenticated flat user-mailbox management, resolving wire
     names before destructive or rename operations and preserving the existing
     folder validation/storage constraints.
746. IMAP `SELECT` and `EXAMINE` now emit RFC-shaped untagged `RECENT` counts
     alongside `EXISTS`, `UIDVALIDITY`, and `UIDNEXT`.
747. IMAP `FETCH` and `UID FETCH` now support conservative single-part text
     literals for `BODY[1]` and `BODY.PEEK[1]`.
748. IMAP `FETCH` and `UID FETCH` now answer conservative single-part MIME
     header requests for `BODY[1.MIME]` and `BODY.PEEK[1.MIME]`.
749. IMAP non-UID `FETCH` now uses the same bounded header literal path as
     `UID FETCH` for `BODY[HEADER]` and `RFC822.HEADER`.
750. IMAP `STARTTLS` completion now includes an updated `[CAPABILITY ...]`
     response code for the post-TLS command surface.
751. IMAP `APPEND` now returns an explicit unsupported `NO` response while
     mailbox import semantics remain deferred.
752. `gogomail --mode=imap` now runs a dedicated Redis consumer group for
     committed `mail.stored` events and publishes UID-bearing `EXISTS` updates
     into the process-local mailbox event broker for live IDLE sessions.
753. IMAP single-part `BODY`/`BODYSTRUCTURE` responses now derive content type,
     parameters, content-transfer-encoding, ID, and description from bounded raw
     message headers instead of always reporting text/plain defaults.
754. `internal/message` now exposes a bounded streaming MIME-structure parser
     that walks multipart trees, preserves raw transfer-encoding metadata,
     counts body octets/lines, and avoids retaining attachment payloads for
     future IMAP `BODYSTRUCTURE` serialization.
755. IMAP metadata-only `BODYSTRUCTURE` fetches now use the streaming
     MIME-structure parser to return multipart child order, subtype, parameters,
     transfer encodings, dispositions, body octets, and text line counts without
     retaining attachment payloads.
756. IMAP combined `BODYSTRUCTURE` plus literal body/header fetches can reopen
     the raw message for MIME metadata while preserving the original reader for
     literal streaming, so common preview/header fetch batches keep rich
     structure responses.
757. IMAP `BODYSTRUCTURE` now emits RFC 3501-shaped `message/rfc822` body
     fields, including encapsulated message header-derived envelope metadata,
     parsed nested body structure, and line counts, instead of serializing
     attached messages as generic basic parts.
758. IMAP `FETCH`/`UID FETCH` can now return RFC 3501-shaped
     `BODY[n.HEADER]` and `BODY[n.TEXT]` literals for `message/rfc822` parts,
     including forwarded-message attachments inside multipart messages.
759. IMAP `FETCH`/`UID FETCH` can now return
     `BODY[n.HEADER.FIELDS (...)]` and `BODY[n.HEADER.FIELDS.NOT (...)]`
     subsets for `message/rfc822` parts, so clients can preview
     forwarded-message headers without fetching whole nested headers.
760. IMAP `FETCH`/`UID FETCH` can now follow multipart body-part numbering
     inside top-level `message/rfc822` parts, including nested part MIME
     headers such as `BODY[1.2]` and `BODY[1.2.MIME]`.
761. IMAP literal-fetch regression coverage now includes multipart messages
     that attach a `message/rfc822` whose encapsulated body is itself
     multipart, guarding forwarded-message paths such as `BODY[2.2]` and
     `BODY[2.2.MIME]`.
762. Malformed encapsulated `message/rfc822` literals now degrade gracefully for
     nested section fetches, returning an empty header section and raw text
     bytes instead of failing the whole IMAP `FETCH`.
763. The shared MIME-structure parser now descends into `message/rfc822` parts
     while counting the encapsulated message bytes/lines and capturing bounded
     envelope metadata, so forwarded-message attachments expose nested body
     metadata without retaining payloads.
764. IMAP `CAPABILITY` now advertises `NAMESPACE` alongside the implemented
     namespace command so client discovery matches the supported command
     surface.
765. IMAP `SEARCH` and `UID SEARCH` now accept `CHARSET US-ASCII` and
     `CHARSET UTF-8` prefixes and return an RFC-shaped `[BADCHARSET]` response
     for unsupported search charsets.
766. IMAP `STORE`/`UID STORE` can persist the IMAP-specific `\Deleted` flag
     separately from gogomail's soft-delete status, and `FETCH`/`SEARCH` expose
     that flag through `FLAGS`, `DELETED`, and `UNDELETED`.
767. IMAP `SEARCH` and `UID SEARCH` now accept `RECENT`, `OLD`, and `NEW`
     criteria at the parser boundary.
768. IMAP `SEARCH` and `UID SEARCH` now support `KEYWORD` and `UNKEYWORD`
     criteria with validated keyword atoms for the `$Forwarded` compatibility
     keyword.
769. IMAP `SEARCH` and `UID SEARCH` now accept sequence-set criteria such as
     `2:*`, letting clients intersect standard search predicates with selected
     mailbox sequence ranges.
770. IMAP `LIST "" ""` and `LSUB "" ""` now return the hierarchy root with
     `\Noselect` and `/` delimiter metadata for clients that probe namespace
     delimiters through LIST-compatible commands.
771. IMAP `SEARCH` and `UID SEARCH` now accept parenthesized search-key groups,
     combining grouped predicates with RFC default AND semantics and allowing
     grouped operands inside `OR`.
772. IMAP `FETCH` and `UID FETCH` now support bounded partial section literals
     for common `BODY[HEADER]`, `BODY[TEXT]`, `BODY[1]`, and `BODY[1.MIME]`
     requests.
773. IMAP `FETCH` and `UID FETCH` now support bounded top-level multipart
     body-section literals such as `BODY[1]` and `BODY[2]`, allowing clients
     to read individual MIME parts without fetching the full message.
774. IMAP `FETCH` and `UID FETCH` now stream actual multipart child MIME
     headers for `BODY[n.MIME]` and `BODY.PEEK[n.MIME]` requests when the
     selected part exists.
775. IMAP `FETCH` and `UID FETCH` now support bounded nested multipart
     body-section literals such as `BODY[1.2]` with a capped MIME part path
     depth.
776. IMAP `FETCH` and `UID FETCH` now support bounded partial windows over
     `BODY[HEADER.FIELDS (...)]`, `BODY.PEEK[HEADER.FIELDS (...)]`,
     `BODY[HEADER.FIELDS.NOT (...)]`, and
     `BODY.PEEK[HEADER.FIELDS.NOT (...)]` literals.
777. IMAP `FETCH` and `UID FETCH` now support bounded partial windows over
     multipart body-section literals such as `BODY.PEEK[2]<4.4>`.
778. IMAP mailbox lookup now resolves wire names such as `INBOX` and
     `Archive/2026` to the stored mailbox ID before selected-mailbox state is
     used by follow-up commands.
779. IMAP `EXAMINE` now passes read-only selection intent through the backend
     `SelectMailboxRequest`, letting service adapters distinguish read-only
     sessions from writable `SELECT`.
780. IMAP `SELECT` and `EXAMINE` now establish mailbox event subscriptions
     before emitting selected-mailbox response data, avoiding ambiguous partial
     selection state when subscription setup fails.
781. IMAP `COPY` and `UID COPY` now have protocol, service, and PostgreSQL
     repository coverage for standards-shaped cross-mailbox copy semantics:
     source UIDs remain stable, destination copies receive fresh mailbox-local
     UIDs, and copied rows pass through quota accounting.
782. IMAP `CREATE`, `DELETE`, and `RENAME` now have protocol and service
     adapter coverage over the existing folder CRUD boundary, improving
     standards-shaped mailbox management compatibility while keeping hierarchy
     semantics constrained by the current flat folder model.
783. IMAP `\Deleted` is now a first-class protocol flag in the gateway and
     repository flag store, giving `EXPUNGE` a standards-shaped marker without
     conflating it with gogomail's soft-delete message status.
784. IMAP `EXPUNGE` and `UID EXPUNGE` now have protocol, service, repository,
     and optional PostgreSQL integration coverage for deleting only
     `\Deleted`-marked active messages while preserving sequence-number wire
     semantics.
785. IMAP `MOVE` and `UID MOVE` now have protocol, service, repository, and
     optional PostgreSQL integration coverage for RFC-shaped source expunge
     semantics: source sequence sets resolve through the selected mailbox,
     destination mailboxes are validated, active messages move folders
     transactionally, source UID rows are removed, and destination mailboxes
     assign fresh local UIDs.
786. IMAP now advertises `UIDPLUS` and returns RFC 4315-style `[COPYUID ...]`
     response codes for `COPY` and `UID COPY` when the repository returns
     destination UIDs, improving client synchronization without guessing copied
     message identities.
787. IMAP `CLOSE` now follows RFC selected-mailbox semantics by silently
     expunging `\Deleted` messages for writable selections before clearing
     selected state, while `EXAMINE` read-only selections close without
     destructive work or untagged `EXPUNGE` responses.
788. IMAP `SELECT` and `EXAMINE` now emit optional RFC-shaped `[UNSEEN n]`
     response codes by resolving the first unread message sequence number from
     mailbox summaries instead of confusing unseen counts with sequence numbers.
789. IMAP command reading consumes bounded synchronizing literals with a
     continuation response, preserves the literal as the final parsed command
     field, and keeps the connection framed for APPEND-style command literals.
790. IMAP `APPEND` now has a protocol-to-backend request boundary carrying the
     destination mailbox, literal body reader, and size, with the service adapter
     returning an explicit unsupported error until repository/storage append
     semantics are implemented.
791. IMAP `APPEND` request parsing now accepts RFC-shaped optional flag lists and
     internal date-time values before the literal, preserving client-supplied
     initial flags and INTERNALDATE metadata in the backend request boundary.
792. IMAP `APPEND` success responses now have a UIDPLUS-ready result boundary
     carrying UIDVALIDITY and the assigned message UID, allowing successful
     backend storage to emit `[APPENDUID uidvalidity uid]` as required for
     strong client synchronization.
793. `mailservice` now routes IMAP `APPEND` through an append-capable
     repository boundary when present, trims authenticated user/mailbox ids, and
     publishes best-effort destination `EXISTS` events for successful append
     results while preserving explicit unsupported responses for deployments
     without repository-backed APPEND storage.
794. IMAP `APPEND` now has a service-backed storage path: the service spools
     and size-checks synchronizing literals, parses RFC message metadata,
     stores the raw `.eml` through the configured storage backend, and the
     `maildb` repository inserts message metadata, quota ledger increments,
     `mail.stored` outbox work, mailbox-local UID assignment, and UIDVALIDITY in
     one transaction.
795. IMAP `APPEND` now maps missing destination mailboxes to an RFC-shaped
     `NO [TRYCREATE]` response code, improving compatibility with clients that
     can create the target mailbox and retry the append.
796. IMAP `APPEND` now maps quota-ledger `mailbox full` failures to a
     client-visible `NO [OVERQUOTA]` response code, so standards-aware clients
     can distinguish quota exhaustion from generic append failures.
797. IMAP `APPEND` now rejects commands without a synchronizing literal as a
     syntax `BAD` response instead of reporting the command as unsupported,
     reflecting that APPEND is implemented but requires an RFC-shaped literal
     payload.
798. IMAP `APPEND` persistence now returns the appended message sequence number
     and uses it as the `EXISTS` event message count when available, giving
     selected-mailbox IDLE/NOOP listeners a precise mailbox size instead of
     only an inferred increment.
799. IMAP `COPY`, `UID COPY`, `MOVE`, and `UID MOVE` now map missing
     destination mailboxes to RFC-shaped `NO [TRYCREATE]` responses, matching
     the APPEND behavior and improving retry/create flows in standards-aware
     clients.
800. IMAP selected-mailbox event draining now suppresses stale or duplicate
     `EXISTS` events when an exact message count is present, reducing noisy
     follow-up NOOP/IDLE updates after commands that already reported the same
     mailbox size.
801. IMAP mailbox DTOs now carry durable `highest_modseq`, and `SELECT`,
     `EXAMINE`, and `STATUS` can expose `[HIGHESTMODSEQ ...]` /
     `HIGHESTMODSEQ` metadata for clients that use mod-sequence based sync
     hints.
802. IMAP `FETCH` and `UID FETCH` now return RFC 4551-shaped `MODSEQ (n)`
     attributes when requested, mapping durable per-message IMAP mod-sequences
     through the gateway.
803. IMAP `SEARCH` and `UID SEARCH` now support RFC 4551-shaped `MODSEQ`
     criteria, including optional metadata entry/type arguments, and append
     `(MODSEQ n)` with the highest matched mod-sequence for non-empty results.
804. IMAP `FETCH` and `UID FETCH` now support RFC 4551-shaped `CHANGEDSINCE`
     modifiers, returning only messages with greater per-message mod-sequences
     and implicitly including `MODSEQ` response attributes.
805. IMAP sessions now become CONDSTORE-aware after implemented mod-sequence
     enabling commands, so subsequent flag `FETCH` event/STORE echo responses
     include `MODSEQ` attributes for client cache coherence.
806. IMAP `STORE` and `UID STORE` now support RFC 4551-shaped
     `(UNCHANGEDSINCE n)` modifiers with transactional per-message mod-sequence
     checks, partial success for passing messages, and `[MODIFIED uid-set]` /
     `[MODIFIED sequence-set]` responses for stale flag writes.
807. Conditional IMAP `STORE`/`UID STORE` response and service event paths now
     filter modified stale UIDs out of successful `FETCH` echoes and mailbox
     flag notifications, keeping mixed-success CONDSTORE updates clean across
     backend adapters.
808. IMAP `SELECT` and `EXAMINE` now accept the RFC 4551-shaped `(CONDSTORE)`
     parameter and mark the session CONDSTORE-aware.
809. IMAP `CAPABILITY` now advertises `CONDSTORE`, making the implemented
     RFC 4551 durable mod-sequence sync surface discoverable by standard IMAP
     clients.
810. IMAP now advertises `ENABLE` and accepts RFC 5161-shaped
     `ENABLE CONDSTORE`, allowing clients to mark a session CONDSTORE-aware
     before mailbox selection while leaving the capability list stable.
811. IMAP `LIST` now emits RFC 6154 special-use attributes for system folder
     roles such as Drafts, Sent, Trash, Junk, Archive, All, and Flagged so
     standards-aware clients can auto-detect default mailbox roles.
812. IMAP now advertises `SPECIAL-USE` and accepts RFC 6154 extended
     `LIST (SPECIAL-USE)` / `RETURN (SPECIAL-USE)` forms, filtering special
     role discovery requests while keeping normal `LIST` output compatible.
813. Development storage portability is now documented in
     `docs/storage-backends.md`, and `docker/docker-compose.dev.yml` includes a
     `minio-init` one-shot service that creates the default local `gogomail`
     bucket for MinIO-backed runs.
814. S3-compatible storage URL generation now has regression coverage for
     virtual-hosted-style requests with URL-sensitive object keys, preventing
     double-escaped paths before SigV4 canonical request signing.
815. IMAP selected-mailbox event draining now renders sequence-bearing
     `MailboxEventExpunge` notifications as untagged `EXPUNGE` responses for
     `NOOP`/`IDLE` clients, keeping live deletion state aligned.
816. Existing IMAP UID lookup now returns mailbox sequence numbers, letting
     Mail API move/delete expunge events become renderable for live
     `NOOP`/`IDLE` clients even after the committed mutation removes source UID
     rows.
817. IMAP `MOVE` and `UID MOVE` now reassign source mailbox UID rows to fresh
     destination mailbox UIDs inside the move transaction and return UIDPLUS
     `[COPYUID ...]` mappings before source `EXPUNGE` responses, aligning the
     MOVE path with RFC 6851/UIDPLUS-compatible client expectations.
818. IMAP `MOVE` and `UID MOVE` now advance the selected source mailbox
     highest mod-sequence and emit `[HIGHESTMODSEQ ...]` metadata alongside
     the move response path, keeping the advertised CONDSTORE surface aligned
     with RFC 6851 mod-sequence expectations.
819. IMAP `CAPABILITY` now advertises RFC 3348 `CHILDREN` alongside the
     existing `\HasNoChildren` LIST attributes, keeping mailbox hierarchy
     discovery signals consistent for standards-aware clients.
820. IMAP `LIST` now derives RFC 3348 `\HasChildren` / `\HasNoChildren`
     attributes from mailbox parent/path metadata instead of marking every
     mailbox leaf-only, giving hierarchical clients accurate expansion hints.
821. IMAP `SUBSCRIBE` and `UNSUBSCRIBE` now persist mailbox subscription names
     through the service/repository boundary, and `LSUB` returns the saved set
     while retaining deleted mailbox names with `\Noselect` and honoring the
     RFC 3501 `%` hierarchy parent response case.
822. IMAP `ID` now validates RFC 2971-shaped `NIL` or bounded field/value
     parameter lists, rejecting duplicate fields, oversized field names or
     values, and malformed argument shapes before returning gogomail server
     identity.
823. IMAP `MOVE` and `UID MOVE` now allow the selected mailbox as the
     destination when COPY to that mailbox is allowed, creating a fresh
     same-mailbox message UID before expunging the source UID as required by
     RFC 6851.
824. IMAP `CAPABILITY` now advertises RFC 5819 `LIST-STATUS`, and extended
     `LIST ... RETURN (STATUS (...))` emits requested `STATUS` data after each
     matching selectable mailbox, reducing standards-aware client folder-list
     round trips.
824a. IMAP extended `LIST` now accepts no-op `RETURN (CHILDREN)` requests and
      combinations such as `RETURN (CHILDREN STATUS (...))`, keeping clients
      that explicitly request advertised CHILDREN metadata compatible while
      preserving existing `\HasChildren` / `\HasNoChildren` response
      attributes.
825. IMAP `CAPABILITY` now advertises RFC 4731 `ESEARCH`; `SEARCH RETURN (...)`
     and `UID SEARCH RETURN (...)` return single untagged `ESEARCH` responses
     with requested `MIN`, `MAX`, compact `ALL`, `COUNT`, UID indicators, and
     CONDSTORE `MODSEQ` data.
826. IMAP `CAPABILITY` now advertises RFC 5182 `SEARCHRES`; `SEARCH RETURN
     (SAVE)` stores selected-session search results so `$` can be reused by
     subsequent sequence-set and UID-set commands without sending result sets
     back through the client.
826a. IMAP `SORT`/`UID SORT` and `THREAD`/`UID THREAD` now accept leading
      `RETURN (SAVE)`, save successful matched result sets for subsequent `$`
      reuse, clear save-requested `NO` failures, and keep tagged `BAD`
      malformed save attempts non-mutating.
826b. IMAP direct `ESEARCH` commands now return an explicit tagged `BAD`
      explaining that RFC 7377 `MULTISEARCH` is required, preserving the
      distinction between RFC 4731 `SEARCH RETURN (...)` support and the
      future multi-mailbox `ESEARCH` command surface.
827. IMAP `CAPABILITY` now advertises RFC 8438 `STATUS=SIZE`; `STATUS` and
     `LIST-STATUS` can return per-mailbox active message octet totals from
     repository aggregate metadata without per-message `RFC822.SIZE` fetches.
828. IMAP `CAPABILITY` now advertises RFC 5256 `SORT`; `SORT` and `UID SORT`
     reuse selected-mailbox search evaluation, enforce mandatory `US-ASCII` and
     `UTF-8` charset support, and return sorted sequence-number or UID result
     sets for standard arrival, sent-date, address, base-subject, and size
     ordering.
829. IMAP `CAPABILITY` now advertises RFC 5256 `THREAD=ORDEREDSUBJECT`;
     `THREAD ORDEREDSUBJECT` and `UID THREAD ORDEREDSUBJECT` reuse
     selected-mailbox search evaluation and return ordered-subject thread trees
     without advertising the more complex `REFERENCES` algorithm before its
     Message-ID normalization and ancestry rules are implemented.
830. IMAP RFC 5256 base-subject extraction now decodes RFC 2047 encoded-word
     subjects before reply/forward artifact removal, improving
     internationalized `SORT SUBJECT` and `THREAD ORDEREDSUBJECT` compatibility.
831. IMAP mailbox mutation guardrails now enforce RFC 3501 `INBOX` special
     cases by rejecting `CREATE INBOX`, `DELETE INBOX`, and generic
     `RENAME INBOX` until the required message-moving rename semantics are
     implemented.
832. Service-backed IMAP message summaries now hydrate stored `To`, `Cc`, and
     `Bcc` address JSON into RFC-shaped ENVELOPE address lists, keeping real
     repository-backed `FETCH ENVELOPE`, address search, and RFC 5256 address
     sort behavior aligned with persisted message metadata.
833. IMAP `FETCH` and `UID FETCH` now apply RFC 3501 `\Seen` side effects for
     successful `BODY[...]`, `RFC822`, and `RFC822.TEXT` literal reads while
     preserving `BODY.PEEK[...]` and `RFC822.HEADER` as non-mutating preview
     requests.
834. IMAP `FETCH` and `UID FETCH` now preserve RFC 3501 `RFC822`,
     `RFC822.HEADER`, and `RFC822.TEXT` response data item names on the wire
     instead of rewriting them to their internal `BODY[...]` equivalents.
835. IMAP `SUBSCRIBE` can now persist missing mailbox names so `LSUB` can
     report them with `\Noselect`, preserving subscription state across mailbox
     migration, deletion, and delayed creation flows.
836. IMAP mailbox names now follow RFC 3501 modified UTF-7 at the protocol
     boundary: `LIST`/`LSUB` decode reference and pattern arguments before
     matching and encode non-ASCII names or ampersands on the wire, while
     `SELECT`, `EXAMINE`, `STATUS`, `APPEND`, `COPY`, `MOVE`, `CREATE`,
     `DELETE`, `RENAME`, `SUBSCRIBE`, and `UNSUBSCRIBE` reject raw 8-bit and
     malformed modified UTF-7 instead of leaking wire names into storage.
837. IMAP `BODYSTRUCTURE` regression coverage now includes multipart messages
     that attach a `message/rfc822` whose encapsulated body is itself
     multipart, guarding nested `MESSAGE/RFC822` serialization for forwarded
     message compatibility.
838. IMAP now advertises `LITERAL+` and accepts bounded non-synchronizing
     command literals such as `APPEND ... {n+}` without an extra continuation
     round trip, while preserving synchronizing literal framing for
     conservative clients.
839. IMAP empty flag-lists are accepted where RFC-shaped clients can send them:
     `APPEND ()` stores without initial flags, `STORE FLAGS ()` clears
     supported flags, and empty `+FLAGS ()`/`-FLAGS ()` are successful no-ops.
840. IMAP selected-mailbox `APPEND` now prefers the backend-returned appended
     message sequence number for the untagged `EXISTS` count, falling back to a
     local increment only when precise sequence metadata is unavailable.
841. IMAP selected-mailbox `COPY` and same-mailbox `MOVE` now prefer
     backend-returned destination message sequence numbers for untagged
     `EXISTS` counts, falling back to local increments only when precise
     metadata is unavailable.
842. IMAP selected-mailbox `EXPUNGE` events delivered through `NOOP` or `IDLE`
     now adjust saved SEARCHRES `$` sequence numbers the same way explicit
     `EXPUNGE` commands do, keeping subsequent `$` reuse aligned with visible
     mailbox state.
843. IMAP `EXAMINE` setup failures now return `NO EXAMINE failed` instead of
     `NO SELECT failed`, keeping tagged failure responses aligned with the
     selected-mailbox command clients actually issued.
844. IMAP malformed recognized `UID` subcommands now reach their
     command-specific validators, so incomplete or structurally invalid
     `UID SEARCH`, `UID SORT`, `UID THREAD`, `UID FETCH`, `UID STORE`,
     `UID EXPUNGE`, and `UID COPY` produce precise tagged `BAD` responses
     before authentication/selected-state checks instead of a generic
     UID-dispatch failure.
845. IMAP missing-mailbox failures for `SELECT`, `EXAMINE`, `STATUS`,
     `DELETE`, and `RENAME` now return tagged `[NONEXISTENT]` response codes
     instead of generic command failures, preserving machine-readable
     absent-folder state for standards-aware clients.
846. IMAP `SELECT` and `EXAMINE` now emit `[UIDNOTSTICKY]` when selected
     mailbox state marks UIDs as non-sticky, keeping UIDPLUS-adjacent clients
     aware of mailbox UID persistence guarantees.
847. IMAP selected-state no-argument commands `CHECK`, `CLOSE`, `UNSELECT`,
     and `EXPUNGE` now reject extra arguments with tagged `BAD` responses
     instead of ignoring malformed input, protecting destructive expunge
     handling from ambiguous client commands.
848. IMAP any-state no-argument commands `CAPABILITY`, `NOOP`, and `LOGOUT`
     now reject extra arguments with tagged `BAD` responses instead of silently
     accepting malformed commands or ending sessions for malformed logout
     attempts.
849. IMAP `STATUS` now requires a parenthesized status item list, rejecting
     malformed `STATUS mailbox MESSAGES`-style requests before mailbox metadata
     lookup.
850. IMAP `LIST ... RETURN (STATUS (...))` now also requires a parenthesized
     status item list, rejecting malformed `RETURN (STATUS MESSAGES)` before
     mailbox listing work.
851. IMAP command dispatch now rejects malformed tags containing atom-special
     characters with untagged `BAD` responses before command handling,
     avoiding ambiguous tagged replies for invalid client command tags.
852. IMAP command parsing now rejects control characters inside unquoted atoms,
     matching the existing quoted-string control-character guardrail before
     command dispatch.
853. IMAP read-only selected-state mutation handling now validates malformed
     `STORE`, `MOVE`, `UID STORE`, `UID MOVE`, and `UID EXPUNGE` commands before
     returning `NO mailbox is read-only` for valid mutation attempts, keeping
     syntax errors precise for standards-aware clients.
854. IMAP read-only mutation validation now covers full command syntax before
     the read-only response, including invalid UID/sequence sets, unsupported
     STORE modes/flags, and modified UTF-7 destination mailbox names, while
     avoiding backend mutation work for syntactically valid `EXAMINE` attempts.
855. IMAP mailbox mutation handling now rejects generic `RENAME ... INBOX`
     attempts before backend folder mutation, keeping INBOX special semantics
     out of ordinary rename flows alongside existing create/delete/source-INBOX
     guardrails.
856. IMAP command dispatch now validates command and UID subcommand atoms before
     routing, rejecting atom-special-bearing command names as malformed syntax
     instead of reporting them as unknown commands.
857. IMAP authenticated `UID` dispatch now validates missing or malformed
     subcommands before selected-mailbox state, keeping syntax errors precise
     while still returning `NO mailbox must be selected` for valid UID commands
     issued outside selected state.
858. IMAP authenticated `UID` dispatch now also rejects unknown UID subcommands
     before selected-mailbox state, so unsupported UID command names are not
     hidden behind mailbox-selection errors.
859. IMAP selected-state command handlers now validate obvious malformed
     `FETCH`, `STORE`, `COPY`, `MOVE`, `SEARCH`, `SORT`, and `THREAD` syntax
     before returning `NO mailbox must be selected` for valid commands issued
     outside selected state.
860. IMAP selected-state no-argument commands now reject extra arguments on
     `CHECK`, `IDLE`, `CLOSE`, `UNSELECT`, and `EXPUNGE` before returning
     authentication or selected-mailbox state errors for well-formed commands.
861. IMAP `STARTTLS` now validates its no-argument syntax before TLS
     availability and authentication-state checks, preserving precise `BAD`
     diagnostics for malformed upgrade attempts.
862. IMAP authenticated `UID` dispatch now validates state-independent
     subcommand arity and destination mailbox-name syntax before selected
     mailbox state for `FETCH`, `STORE`, `EXPUNGE`, `COPY`, and `MOVE`.
863. IMAP `LOGIN` and `AUTHENTICATE` now validate malformed argument shape
     before plaintext `[PRIVACYREQUIRED]` responses on TLS-required listeners,
     while syntactically valid but unsupported SASL mechanisms return tagged
     `NO` so mechanism-probing clients can fall back cleanly.
864. IMAP mailbox management and subscription commands now validate malformed
     `LIST`, `LSUB`, `CREATE`, `DELETE`, `RENAME`, `SUBSCRIBE`, and
     `UNSUBSCRIBE` argument shape or modified UTF-7 mailbox names before
     authentication failures, preserving precise tagged `BAD` diagnostics while
     valid unauthenticated commands still return `NO authentication required`.
865. IMAP selected-mailbox discovery commands now validate malformed
     `NAMESPACE`, `SELECT`, `EXAMINE`, and `STATUS` argument shape, CONDSTORE
     options, status item lists, or modified UTF-7 mailbox names before
     authentication failures, preserving precise tagged `BAD` diagnostics while
     valid unauthenticated commands still return `NO authentication required`.
866. IMAP selected-state no-argument commands now validate extra arguments on
     `CHECK`, `IDLE`, `CLOSE`, `UNSELECT`, and `EXPUNGE` before authentication
     failures too, preserving precise tagged `BAD` diagnostics while valid
     unauthenticated commands still return `NO authentication required`.
867. IMAP selected-state action commands now validate malformed `FETCH`,
     `STORE`, `COPY`, and `MOVE` arity or modified UTF-7 destination mailbox
     names before authentication failures too, preserving precise tagged `BAD`
     diagnostics while valid unauthenticated commands still return
     `NO authentication required`.
868. IMAP search-oriented selected-state commands now validate malformed
     `SEARCH`, `SORT`, and `THREAD` argument shape, return options, sort
     argument lists, and thread argument lists before authentication failures
     too, preserving precise tagged `BAD` diagnostics while valid
     unauthenticated commands still return `NO authentication required`.
869. IMAP `UID` dispatch now validates missing, malformed, unknown, or
     state-independent malformed subcommands before authentication failures too,
     preserving precise tagged `BAD` diagnostics while valid unauthenticated UID
     commands still return `NO authentication required`.
870. IMAP `APPEND` now validates missing literals, malformed append options, and
     modified UTF-7 mailbox names before authentication failures too, while
     valid unauthenticated appends still consume the RFC literal and return
     `NO authentication required` before backend storage.
871. IMAP `ENABLE` now validates missing capability arguments before
     authentication failures too, while valid unauthenticated enable attempts
     still return `NO authentication required` without mutating session feature
     state.
872. S3-compatible bucket validation now rejects IP-address-shaped names plus
     AWS-reserved bucket prefixes and suffixes at adapter/config validation
     time, aligning gogomail's object-storage startup checks with current AWS
     general purpose bucket naming restrictions.
873. S3-compatible endpoint validation now rejects userinfo, query strings,
     fragments, non-HTTP schemes, CR/LF-bearing target text, and
     non-canonical base paths before adapter construction, keeping SigV4
     signing and object addressing deterministic across AWS S3, MinIO, and
     compatible providers.
874. S3-compatible request construction now automatically uses path-style
     addressing for dotted bucket names on HTTPS endpoints, avoiding AWS S3
     virtual-hosted TLS wildcard certificate mismatches while preserving
     virtual-hosted requests for ordinary bucket names by default.
875. IMAP subscription canonicalization now preserves hierarchy delimiters,
     quoting, and internal spacing while retaining case-insensitive matching,
     preventing distinct subscribed mailbox names from silently collapsing into
     one `LSUB` row.
876. IMAP quoted-string response formatting now preserves ordinary internal
     spacing while escaping quotes/backslashes and cleaning controls, preventing
     mailbox names, FETCH metadata, and MIME parameters from being rewritten on
     output.
877. S3-compatible object key escaping now preserves literal `+` characters as
     `%2B` in segment-escaped request paths, preventing plus-bearing mail
     object keys from being rewritten as spaces and keeping SigV4 canonical
     paths aligned with stored object identity.
878. S3-compatible PUT requests now set deterministic `Content-Length` values
     for seekable upload bodies without buffering object contents, improving
     AWS S3/MinIO-compatible behavior for file-backed mail and attachment
     writes while preserving streaming-first storage hot paths.
879. IMAP `LIST`/`LSUB` CHILDREN metadata now infers immediate parent
     mailboxes from nested `FullPath` values when `ParentID` is absent,
     preserving `\HasChildren` responses for deeper hierarchies such as
     `Projects/2026/Jan`.
880. S3-compatible secret access keys and session tokens now reject spaces,
     tabs, and line breaks during adapter construction, making copied
     environment/config credential mistakes fail before runtime S3
     authentication attempts.
881. Local/NFS-style storage writes now stage data through unique temporary
     files in the destination directory before `rename`, avoiding fixed `.tmp`
     collisions while preserving atomic object replacement semantics.
882. Local/NFS-style storage deletes now treat already-missing objects as
     success, aligning lifecycle cleanup semantics with S3-compatible object
     deletion across storage backends.
883. IMAP `APPEND`, `STORE`, and `UID STORE` flag-list parsing now rejects
     unparenthesized or unbalanced flag lists instead of silently trimming stray
     parentheses before backend mutation.
884. IMAP message sequence sets now reject sequence numbers above the selected
     mailbox size with tagged `BAD` responses, preserving RFC 3501 bounds
     behavior for `FETCH`, `STORE`, `COPY`, and `MOVE` sequence arguments.
885. S3-compatible deletes now treat `404 Not Found` as already-cleaned
     success, aligning compatible-provider lifecycle cleanup with local/NFS
     idempotent delete semantics.
886. IMAP quoted-string parsing now rejects adjacent tokens after a closing
     quote and unsupported backslash escapes before authentication or backend
     work, aligning command tokenization with RFC 3501 quoted-special handling.
887. IMAP mailbox wire-name formatting now preserves ordinary internal spacing
     while still collapsing control-character runs, preventing folder
     list/status responses from changing distinct user-visible mailbox names.
888. IMAP UID `FETCH`, `STORE`, `COPY`, `MOVE`, and `EXPUNGE` commands now
     resolve `*` UID sequence ranges against selected-mailbox UIDs, so common
     client requests such as `UID FETCH 1:*` include the last visible UID
     without expanding through non-existent UID gaps.
889. IMAP `SEARCH UID <sequence-set>` and `UID SEARCH UID <sequence-set>` now
     resolve `*` UID ranges against the selected mailbox's visible UIDs,
     aligning search-key filtering with UID command range handling.
890. IMAP command tag validation now rejects `+` in tags before command
     routing, matching RFC 3501 tag grammar and avoiding ambiguity with
     continuation protocol markers.
891. IMAP `SEARCH`/`UID SEARCH` date criteria now reject malformed date atoms
     that still contain quote characters after command parsing, preventing
     broken `SINCE 05-May-2026"` style inputs from being silently normalized.
892. IMAP command tokenization now rejects embedded quote characters inside
     unquoted atoms while preserving escaped quotes inside proper quoted
     strings, keeping RFC 3501 atom and quoted-string handling separate.
893. IMAP parenthesized `SEARCH`/`UID SEARCH` groups now reject empty `()`
     groups instead of treating them as match-all, while preserving valid
     `(ALL)` groups.
894. IMAP `SEARCH`/`UID SEARCH` `MODSEQ` numeric thresholds now reject
     malformed values that still contain quote characters after command
     parsing, preventing broken `MODSEQ 20"` style inputs from being silently
     normalized.
895. IMAP `SEARCH`/`UID SEARCH` `MODSEQ` entry types now reject malformed
     atoms that still contain quote characters after command parsing,
     preventing broken `MODSEQ "/flags/\\Seen" all" 17` style inputs from
     being silently normalized.
896. Runtime config validation now rejects whitespace-bearing S3-compatible
     secret access keys and session tokens for both `s3` and `minio` backends,
     matching adapter construction guardrails before readiness probes or
     runtime object-storage authentication.
897. IMAP RFC 2971 `ID` parameter-list parsing now rejects unsupported quoted
     escapes and adjacent quoted tokens without whitespace, while preserving
     valid escaped quoted-special characters inside ID strings.
898. IMAP `SEARCH`/`UID SEARCH` `LARGER` and `SMALLER` size criteria now
     require digit-only RFC 3501 number atoms, rejecting signed values such as
     `+20` instead of silently treating them as valid sizes.
899. IMAP mod-sequence numeric inputs now require digit-only atoms across
     `SEARCH MODSEQ`, `FETCH CHANGEDSINCE`, and conditional `STORE`
     `UNCHANGEDSINCE`, rejecting signed values such as `+17`.
900. IMAP UID and message sequence-set numbers now require digit-only atoms,
     rejecting signed values such as `UID FETCH +7` and `FETCH +1` before
     command execution.
901. IMAP MIME body-part paths and partial body fetch windows now require
     digit-only number atoms, rejecting signed forms such as `BODY[+1]` and
     `BODY[]<+12.34>` before fetch processing, with offset/count values capped
     to the unsigned 32-bit IMAP `number` range and MIME part path numbers
     held to the same bound. Padded MIME path atoms are rejected instead of
     trimmed before section lookup.
902. IMAP `SEARCH`, `SORT`, and `THREAD` charset arguments now reject
     malformed atoms that still contain quote characters after command parsing,
     preventing broken values such as `UTF-8"` from being silently normalized.
903. IMAP `THREAD` algorithm arguments now reject malformed atoms that still
     contain quote characters after command parsing, preventing broken values
     such as `ORDEREDSUBJECT"` from being silently normalized.
904. IMAP `SEARCH`/`UID SEARCH` text, body, and header string arguments now
     reject malformed atoms that still contain quote characters after command
     parsing, preventing broken values such as `SUBJECT IMAP"` from being
     normalized.
905. IMAP `SEARCH`/`UID SEARCH` `KEYWORD` and `UNKEYWORD` criteria now reject
     malformed keyword atoms that still contain quote characters after command
     parsing, preventing broken values such as `KEYWORD custom"` from being
     silently normalized.
906. IMAP command tokenization now rejects dangling quote characters at the end
     of unquoted atoms, preventing broken commands such as `SUBJECT IMAP"` and
     `LIST "" INBOX"` from reaching command-specific normalization while
     preserving valid escaped quotes inside proper quoted strings.
907. IMAP `SEARCH` text arguments now preserve valid RFC quoted-special escaped
     quotes from proper quoted strings, so standards-shaped searches such as
     `SUBJECT "Project \"Q2\""` remain compatible while malformed atom quotes
     are rejected by command parsing.
908. IMAP `FETCH`/`UID FETCH` `HEADER.FIELDS` and `HEADER.FIELDS.NOT` lists now
     validate RFC-shaped header field names instead of trimming stray brackets
     or accepting IMAP atom-specials, rejecting malformed requests such as
     `HEADER.FIELDS ([Subject])`.
909. IMAP `FETCH`/`UID FETCH` `CHANGEDSINCE` now requires the RFC-shaped
     parenthesized modifier form and rejects bare or over-closed variants such
     as `FETCH 7 FLAGS CHANGEDSINCE 17`.
910. IMAP `FETCH`/`UID FETCH` macros now remain valid only as standalone macro
     arguments, rejecting malformed list usage such as `FETCH 1 (FAST)` or
     `UID FETCH 7 (FLAGS FAST)`.
911. IMAP `STORE`/`UID STORE` `UNCHANGEDSINCE` now requires the RFC-shaped
     parenthesized modifier form and rejects malformed over-closed values such
     as `(UNCHANGEDSINCE 27))`.
912. IMAP `FETCH`/`UID FETCH` data items now reject over-parenthesized tokens
     before item normalization, preventing malformed requests such as `FETCH 1
     ((FLAGS))` and `UID FETCH 7 BODY.PEEK[]))` from being repaired.
913. S3-compatible access key IDs now reject spaces, tabs, and line breaks
     during config validation and adapter construction, preventing copied
     credential mistakes from being silently trimmed before SigV4 signing.
914. S3-compatible bucket validation now requires bucket names to start and end
     with a letter or digit, rejecting dot-edge names such as `.gogomail` and
     `gogomail.` before adapter construction.
915. S3-compatible endpoint base paths now reject encoded path separators such
     as `%2F` and `%5C`, preventing ambiguous SigV4 canonical paths and object
     addressing before adapter construction.
916. Shared local and S3-compatible storage writes now reject nil `Put` bodies
     before filesystem or HTTP request work, preventing accidental empty object
     writes and keeping storage adapter behavior consistent.
917. IMAP UID and message sequence-set expansion now accepts client-scale
     ranges such as `1:1000` and `1:*` while retaining an explicit expansion
     cap, reducing false `BAD` responses during mailbox synchronization.
918. IMAP UID set resolution now intersects authenticated selected-mailbox UID
     ranges with visible message UIDs even when the range does not contain `*`,
     so sparse requests such as `UID FETCH 1:999` skip missing UIDs instead of
     failing the whole command.
919. S3-compatible storage requests now reject canceled contexts before
     object-key validation, SigV4 signing, or HTTP dispatch, aligning
     cancellation behavior with local/NFS storage and avoiding wasted request
     setup.
920. IMAP UID set resolution now also intersects comma-separated selected
     UID sets with visible message UIDs, so sparse client requests such as
     `UID FETCH 1,7,999` return existing messages and skip missing UIDs instead
     of failing the whole command.
921. IMAP `STORE` and `UID STORE` now honor the selected mailbox's advertised
     `[PERMANENTFLAGS]`, rejecting valid-but-unpermitted system flag mutations
     before backend dispatch and keeping flag storage behavior aligned with
     RFC-shaped selection metadata. Empty add/remove flag lists remain no-ops,
     while empty replacement is rejected when no permanent flags are permitted.
922. Local/NFS and S3-compatible readiness probes now bound the
     verification-object read to the exact expected body size plus one byte,
     preventing malformed or proxy-inflated probe responses from allocating
     unbounded memory during health checks while still detecting body
     mismatches.
922a. Local/NFS and S3-compatible readiness probes now also `Stat` the
      verification object and compare canonical key plus byte size before
      cleanup, catching broken filesystem metadata or S3 `HEAD` paths before
      the instance reports ready.
922b. Local/NFS and S3-compatible readiness probes now also issue a short
      `GetRange` against the verification object, catching broken filesystem
      seek/range handling or S3 `Range` response compatibility before partial
      Drive, attachment, or IMAP object workflows report ready.
923. S3-compatible `PUT`, failed `GET`, and `DELETE` responses now drain a
     small bounded response-body window before close, improving HTTP connection
     reuse for ordinary S3/MinIO responses while preventing oversized bodies
     from stalling cleanup.
924. Local/NFS storage configuration now requires a non-empty bounded
     `GOGOMAIL_MAILSTORE_ROOT` without line breaks when
     `GOGOMAIL_STORAGE_BACKEND=local`, surfacing broken filesystem roots during
     config validation before runtime storage probes.
925. `GOGOMAIL_STORAGE_ROOT` is now accepted as a storage-focused compatibility
     alias for `GOGOMAIL_MAILSTORE_ROOT`, with the legacy mailstore variable
     taking precedence when both are set, keeping documented local/NFS setup
     snippets aligned with runtime config loading.
926. App-level S3-compatible storage option construction is now isolated and
     covered, pinning `GOGOMAIL_STORAGE_BACKEND=minio` to path-style requests,
     preserving ordinary S3 virtual-hosted defaults, and honoring the explicit
     `GOGOMAIL_STORAGE_S3_FORCE_PATH_STYLE=true` override.
927. IMAP `BODY[TEXT]`, `BODY.PEEK[TEXT]`, and `RFC822.TEXT` section literal
     reads now enforce the same bounded text-literal ceiling as other IMAP
     body fetch paths, rejecting oversized section bodies before unbounded
     allocation.
928. IMAP partial fetch windows now reject zero-length counts such as
     `BODY[]<12.0>`, aligning parser validation with RFC 3501 `nz-number`
     grammar before fetch processing.
929. IMAP partial fetch windows now reject trailing characters after the
     closing `>` in tokens such as `BODY[]<12.34>BAD`, preventing malformed
     body-section requests from being silently repaired.
930. Attachment-scan webhooks, push-notification webhooks, and OpenSearch
     writer/searcher calls now share bounded response drain-and-close cleanup,
     improving HTTP connection reuse for external adapters without allowing
     oversized response bodies to stall cleanup.
931. Remote Ed25519 API-usage export manifest signer calls now use the shared
     bounded response drain-and-close helper, improving keep-alive reuse for
     external KMS-backed signer services without unbounded cleanup reads.
932. IMAP `MOVE` and `UID MOVE` now place UIDPLUS `[COPYUID ...]` mappings in
     the final tagged OK response instead of an untagged OK, preserving source
     `EXPUNGE` updates while matching RFC 6851 client expectations for move
     completion metadata.
933. Attachment upload reservation and upload-session body service validation
     now reject CR/LF-bearing or oversized user identifiers before quota,
     storage, or repository work, aligning user-id guardrails with draft,
     attachment, and session identifiers.
934. IMAP `APPEND` internaldate parsing now accepts RFC 3501 space-padded
     one-digit date-days such as `" 5-May-2026 ..."`, improving compatibility
     with clients that emit the formal `date-day-fixed` grammar.
935. IMAP APPEND service validation now rejects CR/LF-bearing or oversized
     user and mailbox identifiers before repository lookup, spooling, parsing,
     storage, or quota work, keeping direct service callers aligned with the
     protocol boundary's defensive posture.
936. IMAP service-backed `STORE`, `COPY`, `MOVE`, and `EXPUNGE` mutations now
     reject CR/LF-bearing or oversized user and mailbox identifiers before
     repository mutation dispatch or mailbox event publication, extending the
     direct-call guardrails beyond APPEND.
937. IMAP service-backed read/list/subscription/backfill operations now reject
     CR/LF-bearing or oversized user and mailbox identifiers before repository
     reads, storage opens, event subscriptions, or UID backfill work.
938. S3-compatible request construction now automatically uses path-style
     addressing for localhost and IP-address endpoints, avoiding
     `bucket.localhost`/`bucket.127.0.0.1` drift for local MinIO and other local
     compatible object stores even when the generic `s3` backend is used.
939. Folder list/create/rename/delete service methods now reject CR/LF-bearing
     or oversized user identifiers, and create/rename reject unsafe folder
     names, before repository work, keeping Mail API folder operations aligned
     with IMAP mailbox-management guardrails.
940. IMAP service-backed `FETCH`, `STORE`, `COPY`, `MOVE`, and `EXPUNGE` calls
     now reject zero UIDs before repository or storage work, keeping direct
     service callers aligned with RFC 3501's positive UID model.
941. IMAP service-backed `STORE`, `COPY`, and `MOVE` calls now reject empty UID
     sets before repository work, while `EXPUNGE` preserves nil UID sets for
     `CLOSE`-style "all deleted messages" semantics.
942. IMAP `SEARCH` and `UID SEARCH` date criteria now accept one-digit
     date-day atoms such as `SINCE 5-May-2026` while preserving malformed quote
     rejection for broken atoms such as `SINCE 05-May-2026"`, improving client
     compatibility without weakening syntax guardrails. Search date-month atoms
     are canonicalized ASCII-case-insensitively before parsing, so common
     uppercase or lowercase month literals remain compatible.
943. IMAP `SEARCH RETURN (SAVE)` now clears the selected-session `$` result
     when the save-requested search fails with tagged `NO`, while tagged `BAD`
     searches leave the previous result untouched, aligning the advertised
     SEARCHRES capability with RFC 5182 failure semantics.
944. IMAP shared fetch failure paths now preserve the issued command name in
     tagged `NO` responses, so regular `FETCH` failures do not surface as
     `UID FETCH failed` while `UID FETCH` keeps UID-specific wording.
945. S3-compatible access key IDs, secret access keys, and session tokens now
     reject oversized direct adapter inputs using the same bounds as startup
     config validation, keeping SigV4 request construction bounded even when
     `NewS3Store` is called outside the app config path.
946. S3-compatible endpoint base paths are now segment-escaped with the same
     literal `+` preservation as object keys, keeping reverse-proxy/base-path
     deployments aligned with SigV4 canonical request paths across AWS S3,
     MinIO, and strict compatible providers.
947. Local/NFS-style storage writes now honor context cancellation during body
     copy, cleaning staged temp objects and avoiding partial object commits,
     aligning local/NFS cancellation semantics with S3-compatible request
     cancellation.
948. Mail API now exposes `GET /api/v1/webmail/capabilities`, giving future
     production webmail clients a stable bootstrap contract for backend
     version, available/planned modules including future Drive, list and bulk
     limits, supported message flags, compose/search capabilities, attachment
     upload modes, and push-device platforms.
949. Admin API now exposes `GET /admin/v1/console/capabilities`, giving future
     production operator consoles a stable bootstrap contract for backend
     version, available/planned modules including future Drive, list and
     cleanup/retention limits, tenant/domain/user controls, operational triage
     surfaces, API usage/export, IMAP UID backfill, admin auth behavior, and a
     redacted storage backend profile that identifies the configured
     local/NFS, MinIO, or AWS S3-compatible runtime without leaking secrets or
     host-local filesystem roots.
950. Mail API now exposes `GET /api/v1/mailbox/overview`, giving production
     webmail chrome a user-scoped aggregate read for total, unread, starred,
     and stored-size counters plus system-folder ID shortcuts without requiring
     each frontend client to duplicate folder-list aggregation.
951. Mail API message list pagination now accepts optional `read=true|false`
     and `starred=true|false` filters, enabling fast unread/read/starred
     production webmail views while preserving folder scoping, opaque cursors,
     and bounded boolean query validation.
952. Mail API thread list pagination now accepts optional `read=true|false`
     and `starred=true|false` filters, enabling conversation-level unread,
     fully-read, starred, and unstarred quick views while preserving opaque
     cursor pagination and bounded boolean query validation.
953. Mail API message and thread list pagination now also accepts optional
     `has_attachment=true|false`, enabling attachment-presence quick views for
     both flat mailbox lists and conversation lists while preserving existing
     opaque cursor and boolean validation behavior.
954. Mail API thread list pagination now accepts a bounded `folder_id` filter,
     enabling folder-scoped conversation views for system and custom folders
     while retaining opaque cursor pagination and thread-level quick filters.
955. Mail API message and thread list pagination now accepts
     `sort=newest|oldest`, enabling explicit newest-first and oldest-first
     production webmail list controls while preserving bounded validation and
     opaque cursor pagination.
956. Mail API message and thread summaries now expose a required bounded
     `preview` string from the asynchronous search-document read model,
     enabling production webmail list body context without opening stored EML
     objects on the list hot path.
957. Mail API now exposes bounded thread-level bulk flag updates, enabling
     conversation-list read/starred/answered/forwarded actions while using the
     updated message IDs for best-effort IMAP flag event fanout.
958. Mail API now exposes bounded thread-level folder moves, enabling
     conversation-list archive/move workflows while validating destination
     folders, invalidating affected IMAP UID rows transactionally, and
     publishing best-effort expunge events from the pre-move UID snapshot.
959. Mail API now exposes bounded thread-level soft deletes, enabling
     conversation-list delete workflows while invalidating affected IMAP UID
     rows, decrementing stored-byte quota transactionally, and publishing
     best-effort expunge events from the pre-delete UID snapshot.
960. Shared storage now exposes object `Stat` across local/NFS and
     S3-compatible backends, using filesystem metadata locally and signed
     S3 `HEAD` requests remotely so future Drive, lifecycle, and verification
     paths can inspect object size/metadata without streaming bodies.
961. Shared storage now exposes object `Copy` across local/NFS and
     S3-compatible backends, using atomic temporary-file commits locally and
     signed S3 server-side copy remotely so future Drive and lifecycle
     workflows can duplicate objects without caller-side body streaming.
962. Mail API now exposes single-message and bounded bulk message restore for
     soft-deleted messages, clearing `deleted_at` while re-checking and
     re-incrementing the hierarchical quota ledger before restored messages
     become active again.
963. Mail API now exposes bounded thread-level restore for soft-deleted
     conversations, giving production webmail clients a quota-protected
     conversation recovery primitive alongside thread delete/move/flag actions.
964. Mail API restore flows now bridge back into IMAP by best-effort assigning
     UIDs to restored active messages and publishing IMAP `EXISTS` events,
     improving live client compatibility after webmail recovery actions.
965. Shared storage now exposes bounded prefix `List` across local/NFS and
     S3-compatible backends, using local directory walks and signed
     `ListObjectsV2` remotely so future Drive, lifecycle, and reconciliation
     workflows can page through object metadata without backend-specific code.
966. Shared storage now exposes object `Move` across local/NFS and
     S3-compatible backends, using filesystem rename semantics locally and
     signed S3 server-side copy plus source delete remotely so future Drive and
     lifecycle workflows can relocate objects through one backend-neutral
     contract while respecting S3's non-atomic rename model.
967. Shared storage now provides a bounded `DeletePrefix` helper over
     validated prefix `List` pages and idempotent object deletes, giving future
     Drive folder deletion, attachment lifecycle, and reconciliation jobs a
     cursor-driven cleanup path that stays portable across local/NFS, MinIO,
     AWS S3, and strict S3-compatible backends.
968. Drive backend groundwork now has ADR 0009, a `drive_nodes` PostgreSQL
     metadata table, and internal node validation for file/folder names, node
     types, and lifecycle status. Drive bytes remain in the shared storage
     interface, while metadata, folder hierarchy, active sibling uniqueness,
     and trash/delete lifecycle live in the database/service boundary.
969. Drive now has a first internal repository mutation for creating active
     folder nodes, deriving company/domain scope from the active user row,
     validating optional active parent folders, and relying on the
     `drive_nodes` active sibling uniqueness constraint before Drive HTTP APIs
     are exposed.
970. Drive now has an internal file-finalize repository boundary that validates
     storage backend/path/checksum metadata, verifies the object through the
     shared storage `Stat` contract, and increments the company/domain/user
     quota ledger in the same transaction as the `drive_nodes` file insert.
971. Drive now has an internal node-list repository read model for bounded
     active/trashed/deleted folder contents with folder-first stable ordering,
     preparing future Drive list views before any HTTP API or frontend surface
     is exposed.
972. Drive now has an internal trash repository mutation that marks an active
     file/folder and its active descendants as trashed in one transaction,
     preserving object bytes and quota usage for future restore or delayed
     permanent deletion workflows.
973. Drive now has an internal restore repository mutation that marks a
     trashed file/folder and its trashed descendants active again in one
     transaction, clears `trashed_at`, and lets the active sibling uniqueness
     constraint guard name conflicts before HTTP/API surfaces expose restore.
974. Drive now has an internal permanent-delete repository mutation that marks
     a trashed file/folder and its trashed descendants deleted, decrements the
     company/domain/user quota ledger for deleted file bytes in the same
     transaction, and returns storage object references for backend-specific
     cleanup/reconciliation.
975. Drive now has a backend-object cleanup helper for permanent-delete object
     references. It validates backend/path input, de-duplicates repeated
     references, honors cancellation, deletes through configured storage
     stores, and reports progress-preserving failures for retry/reconciliation
     layers.
976. Drive now has a small internal service layer that composes repository
     permanent-delete with backend object cleanup, returning both committed
     metadata/quota state and cleanup progress so future HTTP handlers and
     workers can expose retryable storage cleanup failures cleanly.
977. Drive now has canonical object path builders for staged uploads,
     committed node objects, and user cleanup prefixes under
     `drive/users/{user_id}/...`, with path-segment-safe ID validation before
     storage paths are emitted.
978. Drive permanent-delete cleanup failures now have a PostgreSQL retry
     record boundary. Structured cleanup errors can be recorded with
     user/node/object context, pending failures are de-duplicated per
     backend/path, repeat failures increment attempts, and diagnostic text is
     one-line/UTF-8 bounded for future worker and operator surfaces.
979. Drive cleanup-failure records now have bounded repository list and resolve
     methods with status/user filters, oldest-first pending ordering, limit
     caps, and pending-only resolution so retry workers and admin surfaces can
     inspect and close cleanup drift without direct SQL access.
980. Drive now has an internal cleanup retry service method that lists pending
     cleanup-failure records, deletes referenced objects through configured
     storage stores, resolves successful records, and re-records failed
     attempts with fresh bounded diagnostics.
981. Drive cleanup retry can now run as a first-class backend worker mode,
     `drive-cleanup-worker`, with validated interval/batch/run-once
     configuration and shared storage adapter wiring for local/NFS, MinIO, and
     S3-compatible deployments.
982. Mail API now exposes first Drive HTTP routes for bounded node listing,
     folder creation, trash, restore, and permanent delete. The routes reuse
     the existing user auth/fallback path, call the Drive repository/service
     boundaries, and are documented in OpenAPI with stable response envelopes.
983. Mail API now exposes `POST /api/v1/drive/files/finalize`, allowing a
     previously staged object to become quota-accounted Drive file metadata via
     the shared storage `Stat` contract and Drive file-finalize repository
     boundary.
984. Mail API now exposes `PUT /api/v1/drive/files/staged/{upload_id}/body`,
     streaming bounded Drive object bodies to local/NFS, MinIO, or
     S3-compatible storage, deriving canonical staging keys, and returning
     size/SHA-256 metadata for file finalization.
985. Mail API now exposes `PATCH /api/v1/drive/nodes/{id}/name`, letting
     production Drive clients rename active files and folders through the
     repository normalization and active sibling uniqueness boundary.
986. Mail API now exposes `PATCH /api/v1/drive/nodes/{id}/parent`, letting
     production Drive clients move active files and folders into destination
     folders or back to root while rejecting self/subtree cycles.
987. Mail API now exposes `GET /api/v1/drive/nodes/{id}` with bounded status
     filtering, giving production Drive clients a stable single-node metadata
     refresh path after edits and selections.
988. Drive upload sessions now have a PostgreSQL metadata table and
     `internal/drive` validation contract for upload identity, parent folder,
     declared size, MIME type, storage backend, lifecycle status, and bounded
     expiration before resumable Drive upload APIs are exposed.
989. Drive upload-session creation now has a repository/service boundary for
     recording pending sessions under active users and optional active parent
     folders with backend-neutral storage metadata and bounded expiration.
990. Mail API now exposes `POST /api/v1/drive/upload-sessions`, letting
     production Drive clients create pending upload-session envelopes with
     declared size, storage backend, MIME type, parent folder, and optional
     RFC3339 expiration before body transfer and finalization.
991. Mail API now exposes `GET /api/v1/drive/upload-sessions/{id}`, giving
     production Drive clients a stable upload-session status refresh path for
     retry and future resumable-upload state.
992. Mail API now exposes `DELETE /api/v1/drive/upload-sessions/{id}`,
     allowing production Drive clients to cancel pending, uploading, or failed
     sessions explicitly instead of waiting for expiry cleanup.
993. Drive upload-session body storage now has service/repository boundaries
     that stream retry bodies to distinct canonical object paths, enforce
     declared size and optional SHA-256 checks, record storage metadata, and
     clean failed or superseded objects best-effort through shared storage.
994. Mail API now exposes `PUT /api/v1/drive/upload-sessions/{id}/body`,
     wiring retry-safe Drive upload-session body storage to clients with
     optional `X-Content-SHA256` verification and explicit `Content-Range`
     rejection until chunked upload semantics are specified.
995. Drive upload-session finalization now has an atomic repository/service
     boundary that locks a writable session, verifies stored object size
     through shared storage, increments quota, inserts Drive file metadata, and
     marks the session finalized in one transaction.
996. Mail API now exposes `POST /api/v1/drive/upload-sessions/{id}/finalize`,
     completing the production-facing full-body Drive upload-session path from
     session creation through body storage into quota-accounted file metadata.
997. Webmail capabilities now advertise Drive node operations, Drive
     upload-session create/read/cancel/body/finalize support, checksum
     preconditions, and Drive upload size/TTL limits so production clients can
     bootstrap Drive controls without copying backend constants.
998. Drive upload sessions now have bounded repository/service expiry, marking
     stale pending/uploading/failed sessions expired and deleting stored
     session body objects from the configured storage backend after metadata
     expiry.
999. `drive-cleanup-worker` now runs Drive upload-session expiry before
     permanent-delete object cleanup retries on each tick, keeping abandoned
     session bodies and object cleanup drift out of request paths.
1000. Mail API now exposes `GET /api/v1/drive/upload-sessions` with bounded
      status and limit filters, plus webmail capability discovery for the list
      surface, so production Drive upload managers can recover in-progress
      session state.
1001. Admin API now exposes `GET /admin/v1/drive-upload-sessions` with
      required user scope plus bounded status/limit filters, and admin
      capabilities mark Drive upload-session inspection available for operator
      consoles.
1002. Drive node listing now supports a bounded `q` name filter on both Mail
      and Admin API list surfaces, with case-insensitive normalization and
      literal wildcard handling inside the selected parent/status scope.
1003. Admin API now exposes `GET /admin/v1/drive-nodes` with required user
      scope plus parent/status/name/limit filters, giving operator consoles a
      bounded Drive inventory view through the existing node read model.
1004. Admin API now exposes `GET /admin/v1/drive-nodes/{id}` with required
      user scope and lifecycle status filtering, giving operator consoles a
      bounded single-node metadata inspection view.
1005. Admin API now exposes `GET /admin/v1/drive-usage` with required user
      scope, giving operator consoles Drive quota, node-count, byte-count, and
      pending upload-session dashboard summaries.
1006. Mail API now exposes `GET /api/v1/drive/usage`, and webmail
      capabilities advertise the usage summary surface so production Drive
      panels can render per-user quota and storage cards without admin routes.
1007. Admin API now exposes `POST /admin/v1/drive-upload-cleanup/candidates`
      for stale Drive upload-session cleanup counts and bounded candidate rows,
      giving operator consoles a non-destructive preview before worker cleanup.
1008. Admin API now exposes `POST /admin/v1/drive-upload-cleanup/runs` for
      explicit audited one-shot stale Drive upload-session expiry with
      candidate counts and expired session rows.
1009. Admin API now exposes `GET /admin/v1/drive-cleanup-failures` with
      bounded user/status/limit filters so operator consoles can inspect
      pending or resolved Drive backend object cleanup drift.
1010. Admin API now exposes `POST /admin/v1/drive-cleanup-failures/{id}/resolve`
      for audited operator closure of pending Drive cleanup failures after
      external object cleanup verification.
1011. Admin API now exposes `POST /admin/v1/drive-cleanup-failures/retry-runs`
      for audited bounded retry of pending Drive object cleanup failures, with
      scanned/deleted/resolved/failed counts for operator consoles.
1012. IMAP `SEARCH` and `UID SEARCH` `KEYWORD`/`UNKEYWORD` now map the
      existing webmail `forwarded` message state to a searchable IMAP keyword,
      improving client compatibility while durable arbitrary keyword storage
      remains deferred.
1013. IMAP now exposes the webmail `forwarded` state as a first-class
      `$Forwarded` keyword in `FETCH FLAGS`, accepts it through permitted
      `STORE`/`UID STORE` flag mutations, and persists it through the existing
      message flag JSON model.
1014. Mail API now exposes `GET /api/v1/drive/nodes/{id}/download` for active
      Drive files, streaming object bytes from the configured local/NFS,
      MinIO, or S3-compatible backend with safe attachment/no-store/nosniff
      headers and webmail `node_download` capability discovery.
1015. Mail API now exposes `HEAD /api/v1/drive/nodes/{id}/download`, reusing
      the same active file and storage-object validation to return download
      headers without opening or transferring the file body.
1016. Shared storage now exposes backend-neutral `GetRange` partial object
      reads for local/NFS and S3-compatible backends, and Mail API Drive
      downloads accept one satisfiable `Range: bytes=...` request, returning
      `206 Partial Content` plus `Content-Range` for resumable download and
      media-preview clients. Webmail capabilities advertise
      `node_range_download`.
1017. IMAP `ENABLE` now validates malformed capability atoms before
      authentication and before mutating session extension state, so RFC
      5161-shaped syntax errors remain distinct from well-formed unsupported
      capabilities or unauthenticated enable attempts.
1018. Drive download, byte-range download, and `HEAD` download metadata
      responses now emit sanitized `X-Gogomail-Drive-SHA256` when the file node
      carries a recorded whole-object digest, giving webmail and automation
      clients a backend-neutral integrity signal across local/NFS, MinIO, and
      S3-compatible storage.
1019. Mail API now exposes
      `HEAD /api/v1/messages/{id}/attachments/{attachment_id}/download`,
      reusing message, attachment, and storage-object validation to return safe
      download headers and object-backed `Content-Length` without streaming
      attachment bytes.
1020. Mail API now exposes `POST /api/v1/drive/nodes/{id}/copy` for active
      Drive files, copying object bytes through the backend-neutral storage
      `Copy` contract, creating quota-accounted file metadata, advertising
      `copy_nodes` to webmail clients, and cleaning up the copied object when
      metadata creation fails.
1021. Drive file copy now records cleanup-failure rows when metadata creation
      fails after an object copy and the copied object cannot be deleted,
      preserving operator-visible storage drift instead of losing it as a
      best-effort cleanup warning.
1022. Drive file copy now preallocates the destination node UUID and uses it
      for both the committed object path and inserted `drive_nodes.id`, keeping
      copied-object keys aligned with metadata identifiers while ordinary
      upload/finalize paths may continue using database-generated IDs.
1023. Drive file write endpoints now map quota exhaustion to HTTP 507
      `insufficient_storage` for upload-session finalization, staged-object
      finalization, and file copy, giving webmail clients a distinct storage
      pressure signal instead of a generic bad request.
1024. IMAP `AUTHENTICATE PLAIN` cancellation now returns the RFC 3501 tagged
      `BAD` completion while leaving the unauthenticated session usable for
      follow-up commands.
1025. IMAP SASL PLAIN decoding now rejects mismatched non-empty authorization
      identities until delegated authorization is explicitly modeled in the
      backend authenticator boundary, while still accepting omitted or matching
      identities.
1026. IMAP `LOGIN` and `AUTHENTICATE` failures now include RFC 5530
      `[AUTHENTICATIONFAILED]` response codes, improving common client and
      migration-tool handling of invalid credentials.
1027. Drive folder creation SQL now uses only the bound request parameters,
      fixing production folder-create placeholder drift before recursive Drive
      copy work builds on it.
1028. Drive node copy now supports bounded active folder-tree copies in
      addition to files, preserving backend-neutral object copy semantics,
      quota-accounted file metadata, and cleanup on partial-copy failure.
1029. Drive file finalize, upload-session cleanup/retry-body replacement,
      permanent-delete cleanup, cleanup-failure retry, download, and copy paths
      now enforce the owning user's `drive/users/{user_id}/...` object prefix
      before storage adapter access, tightening tenant isolation for local/NFS,
      MinIO, and S3-backed deployments.
1030. Drive cleanup-failure recording now rejects object paths outside the
      owning user's `drive/users/{user_id}/...` prefix, keeping retry queues
      tenant-scoped at ingestion instead of only at retry time.
1031. Drive node list APIs now accept `sort=name|updated|created|size` for
      webmail and admin surfaces while preserving folder-first ordering,
      enabling production browser controls without frontend-specific coupling.
1032. Admin Drive node listing now accepts `all_parents=true` for whole-user
      Drive inventory search while rejecting ambiguous `parent_id` combinations,
      giving operator consoles a single backend query for broad file lookup.
1033. Drive node list APIs now accept `node_type=folder|file` on webmail and
      admin surfaces, with webmail capabilities advertising supported node
      types for production Drive filters.
1034. IMAP command reading now supports bounded literals in non-final command
      positions and multiple literals in one command, aligning literalized
      LOGIN/string arguments with the advertised `LITERAL+` capability.
1034a. IMAP command reading now applies the command-literal memory cap to the
       cumulative literal payloads in a single command, keeping multi-literal
       `LOGIN`, RFC 2971 `ID`, LIST, or search-style commands standards-shaped
       without letting one connection goroutine exceed the per-command memory
       ceiling through several individually valid literals.
1035. Webmail Drive node listing now accepts `all_parents=true` for whole-user
      Drive search/list views while rejecting ambiguous `parent_id`
      combinations, giving production compose file pickers and Drive browsers a
      backend-backed broad search mode without client-side folder crawling.
1036. Drive share-link metadata now has a PostgreSQL-backed boundary plus
      authenticated Mail API create/list/revoke routes. Raw tokens are returned
      only on creation, while persisted state stores token hashes, short
      suffixes, permission, expiry, and revoke status for future public link
      resolution and compose-side Drive insertion.
1037. CalDAV work now has ADR 0010, a `caldav` runtime scaffold, and an
      `internal/caldavgw` gateway boundary for RFC/WebDAV standards, advertised
      DAV tokens, principal paths, calendar-home paths, calendar collections,
      and `.ics` object paths before storage or public protocol handlers are
      enabled.
1038. CalDAV storage groundwork now has `caldav_calendars` and
      `caldav_calendar_objects` migrations plus gateway validation for calendar
      metadata, supported top-level iCalendar components, object UIDs, strong
      ETags, sync-token derivation, and bounded `.ics` object bodies.
1039. CalDAV WebDAV XML groundwork now has bounded namespace-aware parsing for
      PROPFIND bodies, safe `Depth` header values, `allprop` `include`
      properties, and core REPORT root classification for CalDAV
      `calendar-query`, `calendar-multiget`, `free-busy-query`, and WebDAV
      `sync-collection`, with explicit body, property, href, and nesting
      limits before method handlers are advertised.
1040. CalDAV storage now has a PostgreSQL repository boundary for calendar
      create/list/get and calendar-object upsert/list/get/soft-delete, with
      `.ics` resource-name validation, UID/component checks, strong ETag
      generation, optional observed-ETag guards, object-size limits, and
      transactional calendar sync-token bumps for future WebDAV sync handlers.
1041. CalDAV object validation now wraps `github.com/emersion/go-ical` for RFC
      5545 iCalendar decoding, deriving or verifying UID/component metadata
      from `.ics` bodies while rejecting missing/duplicate UIDs, multiple
      supported top-level calendar components, and excessive
      component/property counts before storage.
1041a. CalDAV calendar-object writes now preflight duplicate active iCalendar
      UIDs within the same calendar before SQL upsert, returning predictable
      repository/handler errors while the PostgreSQL partial unique index stays
      as the final concurrency guard.
1041b. CalDAV calendar-object upsert now maps PostgreSQL unique-index races for
      active object names or iCalendar UIDs into stable repository errors, so
      concurrent writes keep developer-readable CalDAV failure semantics
      instead of exposing raw driver details.
1042. CalDAV WebDAV response groundwork now has a reusable `multistatus`
      builder with per-property `propstat` statuses plus principal,
      calendar-home, calendar-collection, and calendar-object discovery
      properties needed by future PROPFIND and REPORT handlers.
1043. CalDAV now has an internal `OPTIONS`/`PROPFIND` discovery handler
      boundary over a pluggable discovery store, including DAV capability
      headers, safe depth handling, user/path scope checks, and multistatus
      discovery responses for principals, calendar homes, collections, and
      objects before the public listener is enabled.
1044. The PostgreSQL CalDAV repository now satisfies the discovery store
      boundary with active principal lookup and calendar/object list/get
      adapters, preparing `PROPFIND` runtime wiring without exposing the public
      listener yet.
1045. CalDAV Basic authentication groundwork now reuses the existing
      authenticated Submission password verifier boundary, requires TLS or an
      HTTPS forwarding signal unless explicitly allowed for development, and
      resolves authenticated user IDs for future native-client runtime wiring.
1046. CalDAV runtime configuration now has `GOGOMAIL_CALDAV_ADDR` and
      `GOGOMAIL_CALDAV_ALLOW_INSECURE_AUTH`, with production validation
      rejecting insecure Basic-auth operation before listener wiring is enabled.
1047. `gogomail --mode=caldav` now starts a dedicated HTTP listener backed by
      the CalDAV PostgreSQL repository and Basic-auth resolver, while full
      client-ready compatibility remains gated on sync-collection,
      free-busy/scheduling, recurrence semantics, and broader compatibility
      tests.
1048. CalDAV REPORT parsing now validates core handler preconditions for
      `calendar-query`, `calendar-multiget`, `free-busy-query`, and
      `sync-collection`, including required filters, hrefs, UTC time ranges,
      supported `sync-level=1`, and bounded sync limits before handler logic.
1049. CalDAV now implements a first `REPORT calendar-multiget` handler for
      authenticated calendar collections, returning requested object ETags and
      `calendar-data` bodies in multistatus responses while rendering missing
      hrefs as per-resource 404 propstats.
1049a. CalDAV REPORT responses now honor nested RFC 4791 `calendar-data`
       projection requests for `VCALENDAR` and child component property
       selection across `calendar-multiget`, `calendar-query`, and
       `sync-collection`, while preserving required RFC 5545 structure
       properties needed for valid encoded iCalendar objects.
1050. CalDAV now handles authenticated calendar object `GET`, `HEAD`, `PUT`,
      and `DELETE`, returning strong ETags and text/calendar bodies, enforcing
      bounded iCalendar object validation, and honoring `If-Match` /
      `If-None-Match` preconditions before repository upsert or soft delete.
1051. CalDAV now implements `REPORT calendar-query` for authenticated calendar
      collections, returning requested object ETags and `calendar-data` in
      multistatus responses while applying RFC 5545-backed VEVENT time-range
      overlap filtering when clients send CalDAV time-range filters.
1051a. CalDAV `REPORT calendar-query` now rejects unsupported CalDAV filter
       elements with the RFC 4791 `CALDAV:supported-filter` precondition
       instead of silently skipping unimplemented predicates and returning a
       misleadingly broad success response.
1051b. CalDAV handler authorization now separates authenticated actor user ID
       from resource owner user ID and can resolve delegated read/write/manage
       checks through a pluggable access authorizer before owner-scoped store
       reads or mutations run.
1051c. `gogomail --mode=caldav` now wires that access authorizer through
       Directory active principal resolution, `DelegatedAccessAuthorizer`, and
       the shared audit repository, giving CalDAV cross-user access checks the
       same auditable delegation boundary planned for CardDAV, Drive, mailbox
       sharing, and resource calendars.
1051d. CalDAV delegated `PROPFIND` privilege discovery now consumes the
       access-policy decision's WebDAV privilege mapping, so read-only
       delegates see only `DAV:read`, stronger delegates see privileges scoped
       to the requested home, collection, or object resource, and weaker
       delegated sessions no longer receive owner-level static capabilities.
1051e. Directory/Identity now exposes a bounded `SearchPrincipals` repository
       boundary over users, organizations, groups, and resources. Requests are
       company-scoped, optionally domain/organization-scoped, kind-filtered,
       result-limited, and SQL-wildcard escaped, giving CalDAV attendee and
       resource lookup, Contacts/CardDAV autocomplete, shared inbox targeting,
       and admin consoles one platform principal-search contract instead of
       product-local lookup logic.
1052. CalDAV now implements a conservative RFC 6578 `REPORT sync-collection`
      handler for authenticated calendar collections: initial empty-token sync
      returns active objects plus a top-level collection sync token, current
      tokens return only the token, stale-but-known tokens return
      deltas/tombstones, unknown or expired tokens return a DAV
      `valid-sync-token` precondition error, and truncating limits are rejected
      until continuation support exists.
1053. CalDAV now implements RFC 4791-shaped `REPORT free-busy-query` for
      authenticated calendar collections, returning `200 OK` `text/calendar`
      `VFREEBUSY` bodies instead of multistatus XML. The first implementation
      honors `Depth: 1` child VEVENTs, clips periods to the requested UTC
      time-range, skips transparent/cancelled events, maps tentative events to
      `BUSY-TENTATIVE`, coalesces same-type overlaps, and rejects duplicate
      free-busy time ranges before handler work.
1054. CalDAV now implements `MKCALENDAR` for authenticated calendar collection
      Request-URIs whose calendar segment is a UUID, parsing bounded
      namespace-aware creation properties for display name, description, and
      CalendarServer/Apple color, creating the collection at the requested URI,
      and returning `201 Created` with `Location`. The `Allow` header advertises
      `MKCALENDAR` again only after those semantics exist. Unsupported creation
      properties and invalid supported property values now fail atomically
      before creation with RFC 4791 `207 Multi-Status` and a
      `C:mkcalendar-response` body; failed properties return `403 Forbidden` or
      `409 Conflict`, and dependent properties return `424 Failed Dependency`.
      Non-empty `C:mkcalendar` XML bodies now require the RFC 4791 `DAV:set` /
      `DAV:prop` shape instead of silently accepting unknown structural
      children or self-closing bodies, and whitespace-only non-empty bodies are
      rejected as malformed while truly absent bodies retain compatibility.
      Creation success and property-failure
      responses now include `Cache-Control: no-store, no-cache`, preserving the
      existing conservative cache block while satisfying RFC 4791 `no-cache`.
      Explicit non-empty request-body `Content-Type` headers are validated
      before XML parsing: malformed or duplicate values return `400`, non-XML
      media types return `415`, and absent headers remain accepted for client
      compatibility. Unsupported properties from arbitrary XML namespaces are
      preserved in `MKCALENDAR` failure responses with a scoped fallback
      namespace declaration instead of surfacing as `500`. `PROPPATCH` now uses
      the same atomic property failure model for unsupported properties and
      protected `DAV:displayname` removal attempts, returning `403 Forbidden`
      for failed properties and `424 Failed Dependency` for mutable properties
      in the same request. `PROPPATCH` instructions also reject unknown
      structural children inside `DAV:set` and `DAV:remove` before property
      handling, preserving unsupported-property failure semantics only inside
      `DAV:prop`. The `MaxWebDAVProperties` parser bound is now enforced
      across the entire `propertyupdate` request, counting supported,
      unsupported, and protected properties across split `DAV:prop` blocks.
      Repeated mutable properties keep document-order final-value semantics,
      while success and failed-dependency `propstat` responses report each
      repeated property name once. `DAV:remove` property elements are now
      required to be empty name markers; text values or nested XML children are
      rejected as malformed requests before mutation or property-failure
      handling. Empty `DAV:set` and `DAV:remove` instructions are rejected even
      when sibling instructions contain valid properties, and each instruction
      now accepts exactly one `DAV:prop` child per the RFC 4918 `set (prop)` /
      `remove (prop)` grammar. `DAV:prop xml:lang` is persisted for
      `DAV:displayname` and `CALDAV:calendar-description`, returned on
      `PROPFIND` and successful `PROPPATCH`, and cleared with removed
      description values. `MKCALENDAR` creation now stores the same
      `xml:lang` metadata and exposes it on the newly created collection's
      `PROPFIND` responses. Migration guardrails assert 0097 keeps the CalDAV
      language columns, constraints, and Down cleanup aligned with repository
      SQL, and optional PostgreSQL integration coverage now verifies migrated
      create/get/update round-trips plus raw column persistence. Negative
      PostgreSQL coverage bypasses application validation and asserts migration
      0097 rejects invalid language values with the expected check constraints.
      PROPPATCH parsing now distinguishes absent `xml:lang` from explicit empty
      `xml:lang=""`, preserving existing CalDAV language tags on text-only
      updates unless clients explicitly clear them. Explicit empty language
      clearing is covered through handler and PostgreSQL repository tests, and
      empty stored language values are emitted without `xml:lang` attributes.
      Unsupported/protected-property rollback tests now assert attempted
      language changes are not persisted and repository updates are not called
      on RFC 4918 atomic failure responses. Conditional failure coverage now
      asserts `If-Match`, `If-None-Match`, and `If-Unmodified-Since`
      preconditions reject before body reads, leaving attempted language
      mutations unapplied. Conditional success coverage now asserts matching
      collection `If-Match` preserves omitted language tags and `If-Match: *`
      carries observed collection ETags alongside explicit language updates.
      WebDAV `If` header coverage now asserts matching collection conditions
      preserve omitted language tags and failing conditions reject before body
      reads with language mutations unapplied. Optional PostgreSQL integration
      tests now verify observed collection ETag success preserves omitted
      language tags and stale observed ETags reject attempted text/language
      mutations before persistence. Tagged WebDAV `If` header coverage now
      asserts exact collection path tags preserve omitted language tags and
      non-matching or stale-ETag tagged lists reject before body reads. WebDAV
      `If` `Not` condition-list coverage now asserts negated stale ETags allow
      text-only updates with omitted language preservation while negated current
      ETags reject before body reads. Multi-list WebDAV `If` coverage now
      asserts a later matching condition list can satisfy the precondition
      after an earlier stale list, while all-failed lists reject before body
      reads. Malformed WebDAV `If` coverage now asserts parser errors return
      HTTP 400 before body reads and before attempted language mutations,
      including line breaks, unterminated lists/tokens, empty lists, and
      unsupported conditions. Absolute URI tagged `If` coverage now asserts
      HTTP(S) resource tags match by collection path, preserving omitted
      language tags on matching paths and rejecting non-matching paths before
      body reads. State-token `If` coverage now documents the no-lock-store
      semantics: bare state tokens reject before body reads, while negated
      state tokens allow text-only updates and preserve omitted language tags.
      Compound WebDAV `If` coverage now asserts multiple conditions inside one
      list are conjunctive for both omitted-language success and pre-body-read
      rejection. Repeated HTTP `If` header coverage now asserts joined header
      values preserve WebDAV condition-list semantics for omitted-language
      success and pre-body-read failure. Repeated malformed WebDAV `If`
      coverage now asserts later malformed relevant lists still return HTTP
      400 before body reads even when an earlier list already matched.
      Irrelevant tagged malformed `If` coverage now asserts condition-list
      syntax is validated before resource-tag relevance, returning HTTP 400
      before body reads for non-matching tagged malformed lists. Trailing
      WebDAV `If` coverage now rejects tokens left after a valid condition
      list with HTTP 400 before body reads. Malformed prefix `If` coverage now
      rejects non-empty condition-list prefixes unless they are valid
      `<resource-tag>` forms. Empty resource-tag `If` coverage now asserts
      `<>` prefixes return HTTP 400 before body reads. Resource-tag suffix
      coverage now rejects raw `<` or `>` inside tag content with HTTP 400.
      ADR 0014 defines slug alias design for future implementation.
1055. CalDAV now implements `DELETE` for authenticated calendar collection
      paths, soft-deleting the collection and its active child objects in one
      repository transaction while keeping calendar-home and cross-user deletes
      forbidden. Incremental sync tombstones/change logs remain the next
      compatibility step before stale-token clients can receive deletion
      deltas.
1056. CalDAV now has a durable sync-change log for RFC 6578-style
      `sync-collection` deltas. Calendar creation and object upsert/delete
      paths record sync markers in the same mutation transaction, older
      calendars get a baseline marker on first object change, stale-but-known
      sync tokens return changed object properties or response-level 404
      tombstones, and unknown tokens still return DAV `valid-sync-token`.
1057. CalDAV now handles RFC 6764-style discovery by redirecting
      `/.well-known/caldav` to `/caldav/` and serving authenticated root
      `PROPFIND /caldav/` discovery for `current-user-principal`,
      `principal-collection-set`, and `calendar-home-set`.
1058. CalDAV now handles WebDAV `PROPPATCH` for authenticated calendar
      collections, parsing bounded namespace-aware `propertyupdate` bodies for
      `DAV:displayname`, `CALDAV:calendar-description`, and
      CalendarServer/Apple `calendar-color`. The handler rejects object/home
      targets, keeps `displayname` non-removable, updates metadata through a
      small repository boundary, refreshes the collection sync token, and
      appends a durable `collection-updated` sync marker.
1059. CalDAV release status is explicitly experimental/backend-only until
      recurrence expansion, scheduling, sync retention, collection-deletion
      deltas, native-client compatibility testing, and Directory/Identity,
      Contacts/CardDAV, Notification & Sync, Search, and Policy/Audit
      boundaries are established for public calendar semantics. Calendar must
      not evolve into an isolated CRUD subsystem with its own private principal
      model.
1060. CalDAV collection discovery now exposes WebDAV `supported-report-set`
      for the REPORT methods implemented by the gateway today:
      `calendar-query`, `calendar-multiget`, `free-busy-query`, and
      `sync-collection`. Future scheduling/timezone reports remain
      unadvertised until their full RFC semantics and backend boundaries exist.
1060a. CalDAV collection discovery now includes the `sync-collection`
       supported-report only when the runtime store implements the sync-change
       interface, matching the DAV header token gate and avoiding false
       native-client sync discovery on limited stores.
1061. CalDAV `REPORT calendar-query` now applies simple top-level
      `comp-filter` component selection using stored object `component_type`
      metadata before time-range/body matching, so requests for `VTODO`,
      `VEVENT`, `VJOURNAL`, or `VFREEBUSY` do not return unrelated object
      types and do not require reparsing every `.ics` body for component
      filtering.
1062. CalDAV `REPORT calendar-multiget` now validates requested hrefs against
      the REPORT request resource. Collection-scoped multiget only returns
      objects from the same collection, calendar-home multiget may resolve
      authenticated same-user calendar-object hrefs, and out-of-scope hrefs
      render WebDAV 404 propstats without returning object metadata.
1063. CalDAV `PROPFIND` now emits WebDAV `owner`, `creationdate`, and
      `getlastmodified` metadata for resources where gogomail has exact stored
      state. Owner hrefs point at the authenticated principal, creation dates
      are UTC RFC3339 values, and last-modified values use HTTP-date format.
1064. CalDAV calendar object `GET` and `HEAD` now support HTTP
      `If-None-Match` revalidation against stored strong ETags, returning
      `304 Not Modified` without a body when the client representation is
      current.
1065. CalDAV calendar object `PUT` now validates explicit `Content-Type`
      headers at the protocol boundary, accepting `text/calendar` with
      parameters and rejecting incompatible media types with HTTP 415 before
      bounded iCalendar parsing.
1066. CalDAV calendar object `PUT` now enforces `If-Match: *` as an
      existing-resource precondition, returning HTTP 412 for missing objects
      rather than creating a new `.ics` resource through an overwrite-only
      request.
1067. CalDAV calendar object `PUT` now evaluates specific ETag preconditions
      before body reads: stale `If-Match` values and matching
      `If-None-Match` values fail with HTTP 412 before iCalendar parsing or
      repository mutation.
1068. CalDAV calendar object `GET` and `HEAD` now evaluate stale `If-Match`
      before `If-None-Match` cache revalidation, and object `DELETE` now uses
      shared strong ETag list matching so comma-listed conditional-delete
      headers interoperate with WebDAV clients.
1069. CalDAV calendar object `DELETE` now treats `If-Match: *` as an
      existing-resource precondition, returning HTTP 412 for missing `.ics`
      resources before attempting repository deletion.
1070. CalDAV calendar object `GET` and `HEAD` now emit `Last-Modified` from
      stored object update time and honor `If-Modified-Since` revalidation with
      second-precision comparisons, reducing unnecessary body streaming for
      timestamp-valid native-client caches.
1071. CalDAV calendar object `PUT` and `DELETE` now honor
      `If-Unmodified-Since` against stored object update time before reading
      request bodies or mutating repository state, returning HTTP 412 for stale
      timestamp-based overwrite/delete preconditions.
1072. S3-compatible `GetRange` now bounds the returned reader to the validated
      requested byte length even when a provider returns an oversized
      `206 Partial Content` body, aligning remote range reads with local/NFS
      semantics for Drive, attachment, and IMAP partial-read callers.
1073. CalDAV calendar object `GET` and `HEAD` now honor
      `If-Unmodified-Since` before cache revalidation, returning HTTP 412 for
      stale timestamp read preconditions instead of incorrectly falling through
      to `If-None-Match` or `If-Modified-Since`.
1074. S3-compatible `GetRange` now validates the provider's `Content-Range`
      header against the requested byte window before returning the bounded
      response reader, closing mismatched partial responses early so Drive,
      attachment, and IMAP range callers do not consume the wrong bytes.
1075. S3-compatible `GetRange` now enforces exact partial-response body length:
      matching `Content-Range` headers with truncated bodies surface
      `io.ErrUnexpectedEOF` to callers instead of silently returning a short
      Drive, attachment, or IMAP range read.
1076. S3-compatible `GetRange` now drains a small bounded remainder when a
      successfully consumed range reader is closed, improving HTTP connection
      reuse for oversized partial responses while preserving the exact
      caller-visible byte window.
1077. S3-compatible `GetRange` now also bounded-drains unread range bytes when
      callers close early, improving connection reuse for preview/cancel paths
      without allowing unbounded cleanup reads.
1078. IMAP `STATUS` and LIST-STATUS parsing now rejects duplicate status data
      items before mailbox metadata lookup, keeping RFC-shaped status item
      lists deterministic and preventing duplicate response pairs.
1079. CalDAV `MKCALENDAR` now rejects non-UUID creation path IDs before reading
      or parsing the XML body when no active collection already exists at that
      path, preserving the UUID-only creation contract while keeping invalid
      create attempts cheap.
1080. CalDAV calendar collection `DELETE` now evaluates `If-Unmodified-Since`
      and `If-Match: *` preconditions before deleting a collection and its
      children, preventing stale native-client deletes.
1081. CalDAV collection `PROPPATCH` now shares the collection precondition gate,
      rejecting stale `If-Unmodified-Since` metadata edits before reading or
      parsing XML request bodies.
1082. CalDAV `REPORT` now validates malformed Depth headers and rejects
      `Depth: infinity` before reading XML request bodies, keeping unsupported
      WebDAV traversal semantics out of calendar-query, calendar-multiget,
      sync-collection, and free-busy-query hot paths.
1083. CalDAV `calendar-multiget` now accepts HTTP(S) absolute URI hrefs from
      native clients by normalizing only the URI path through the existing
      CalDAV parser and same-user/same-collection scope checks, while rejecting
      userinfo-bearing authorities, query, fragment, opaque, non-HTTP(S), or
      unsafe href forms.
1084. IMAP RFC 2971 `ID` parameter-list parsing now rejects quote and backslash
      atom-special characters inside unquoted ID tokens, keeping raw ID
      argument parsing aligned with RFC 3501 atom/quoted-string boundaries
      while preserving escaped quoted-special characters inside quoted strings.
1085. IMAP RFC 2971 `ID` unquoted field/value tokens now reuse the common IMAP
      atom validator, rejecting literal markers, response specials, wildcard
      specials, quoted specials, and controls consistently with command/tag
      atom handling.
1085a. IMAP RFC 2971 `ID` parameter-list parsing now accepts bounded
       synchronizing and non-synchronizing string literals inside the
       parenthesized field/value list, while missing or unused literal payloads
       remain syntax errors.
1086. Shared storage object path, prefix, and list-cursor validation now rejects
      invalid UTF-8 before local/NFS or S3-compatible adapter use, keeping
      object keys, S3 URLs, SigV4 canonical paths, logs, and cleanup cursors
      text-stable across backends.
1087. S3-compatible `ListObjectsV2` decoding now rejects truncated pages that
      omit a continuation token, preventing Drive, lifecycle, and reconciliation
      scans from accepting a page that cannot be advanced safely.
1088. S3-compatible `ListObjectsV2` key decoding no longer trims
      provider-returned object keys before prefix/object-path validation,
      preventing distinct whitespace-bearing S3 keys from being silently
      normalized into canonical gogomail object paths.
1088a. S3-compatible `ListObjectsV2` pages now reject provider responses that
       return more matching objects than the requested bounded page size,
       keeping S3, MinIO, and local/NFS pagination under the same storage
       contract.
1089. S3-compatible `Stat` and `List` now bound and sanitize provider-returned
      `Content-Type` and ETag metadata before exposing `ObjectInfo`, dropping
      unsafe multiline, invalid UTF-8, or oversized metadata while preserving
      object identity and size for Drive, lifecycle, and reconciliation
      consumers.
1090. CalDAV free-busy generation now ingests stored `VFREEBUSY` source
      objects by parsing `FREEBUSY` period lists, including UTC start/end and
      start/duration forms, clipping them to the requested range, and feeding
      them through the existing same-type coalescing path for RFC 4791
      `free-busy-query` responses.
1091. CalDAV calendar collections now advertise strong collection `getetag`
      values derived from collection sync state and evaluate specific
      comma-listed `If-Match` preconditions for collection `DELETE` and
      `PROPPATCH`, while preserving `If-Match: *` as an existing-collection
      guard.
1092. Directory/Identity now has a first protocol-neutral
      `internal/directory` principal resolver for bounded active user principal
      lookup over user/domain/company state, and CalDAV discovery delegates to
      that shared boundary instead of embedding its own private active-user
      query.
1093. Directory/Identity principal resolution now also supports organization
      principals over the existing organization/domain/company model, preparing
      organization calendar and policy scopes without exposing shared-calendar
      or resource-booking semantics publicly.
1094. Directory/Identity storage now defines protocol-neutral group, resource,
      alias, and group-membership tables, and the shared principal resolver can
      load group and resource principals for future shared inbox, resource
      calendar, admin directory, and delegated access workflows.
1095. Directory/Identity can resolve normalized email aliases to target user,
      organization, group, or resource principals, with active aliases enforced
      as globally unique normalized addresses for predictable mail, attendee,
      and admin-console lookup semantics.
1096. Directory/Identity can check direct active group membership for user,
      organization, group, and resource principals, creating an auditable read
      boundary before recursive membership expansion, delegation, or resource
      booking policy are exposed.
1097. Directory/Identity effective group-membership checks now expand nested
      groups through a bounded recursive query with an explicit depth cap and
      cycle guard, preventing unbounded principal graph traversal before the
      result is used for delegated access or resource-booking policy.
1098. CardDAV groundwork has started with ADR 0012 and `internal/carddavgw`,
      defining RFC/WebDAV/CardDAV standards names, DAV capability tokens,
      canonical principal/address-book/contact-object paths, `.vcf` resource
      validation, and safe relative or HTTP(S) absolute href parsing before any
      contacts CRUD or public CardDAV listener is exposed.
1099. CardDAV storage groundwork now has PostgreSQL `carddav_addressbooks`,
      `carddav_contact_objects`, and `carddav_addressbook_changes` tables with
      user-scoped active uniqueness, strong ETag, sync-token, status, size, and
      `.vcf` body constraints, plus `internal/carddavgw` metadata validators for
      address-book names/descriptions, contact object names/UIDs, strong ETags,
      object-size limits, and sync-token derivation.
1100. CardDAV address-book repository methods now create/list/get collections
      behind active user/domain/company scope, normalize names, bound list
      limits, and insert durable `addressbook-created` change rows in the same
      transaction as collection creation.
1101. CardDAV vCard validation now performs bounded RFC 6350-oriented vCard
      4.0 checks for contact objects, including BEGIN/END structure, exactly
      one VERSION, required UID/FN, folded content-line handling, line/body
      caps, and nested VCARD rejection before contact-object repository writes
      are exposed.
1102. CardDAV contact-object repository methods now upsert/list/get/delete
      active `.vcf` resources under active address-book scope, enforce vCard
      UID alignment, compute strong ETags, honor optional observed ETags before
      overwrite, refresh address-book sync tokens, and record
      `contact-upserted`/`contact-deleted` changes in the same transaction as
      the object mutation.
1103. CardDAV REPORT parsing now recognizes bounded `addressbook-query`,
      `addressbook-multiget`, and WebDAV `sync-collection` request bodies,
      collecting requested properties, hrefs, sync token/level, result limits,
      and the first text-match filter while rejecting malformed, oversized,
      deeply nested, or unsupported sync-level shapes before handlers are
      exposed.
1104. CardDAV now has a WebDAV `multistatus` response builder for future
      PROPFIND, REPORT, and sync handlers, rendering principal discovery,
      address-book collection metadata, contact-object metadata, requested
      `address-data`, supported reports, supported vCard data types, sync
      tokens, and per-property 404 propstats.
1105. CardDAV now has an internal RFC 6764/WebDAV-style discovery handler for
      `/.well-known/carddav`, `OPTIONS`, and bounded `PROPFIND` across root,
      principal, address-book home, address-book collection, and contact-object
      resources. The handler rejects cross-user paths, `Depth: infinity`,
      malformed WebDAV XML, and contact-object discovery above `Depth: 0`;
      the PostgreSQL repository satisfies the discovery store by resolving
      active user principals through the shared Directory layer.
1106. CardDAV now executes internal REPORT requests for `addressbook-multiget`,
      `addressbook-query`, and WebDAV `sync-collection`. Multiget scopes hrefs
      to the requested home or collection and returns per-href 404 propstats,
      query responses can include `address-data` and apply the current bounded
      first text-match filter, and sync responses emit the current collection
      sync token while returning full snapshots or bounded change rows,
      including 404 responses for deleted contact objects.
1106a. CardDAV address-book collection discovery now includes the
       `sync-collection` supported-report only when the runtime store
       implements the sync-change interface, matching OPTIONS DAV token
       advertising and keeping native-client feature discovery honest.
1107. CardDAV now handles internal contact-object `GET`, `HEAD`, `PUT`, and
      `DELETE` paths with `text/vcard` content negotiation, bounded body reads,
      strong ETag and Last-Modified response headers, HTTP cache/precondition
      handling, vCard validation through the repository write path, and
      standard 201/204/304/412 outcomes for create, update, delete, and
      conditional reads.
1108. CardDAV now has runtime wiring: `gogomail --mode=carddav` starts a
      dedicated HTTP listener on `GOGOMAIL_CARDDAV_ADDR`, uses CardDAV Basic
      auth backed by the existing Submission password verifier, rejects
      production insecure Basic auth through
      `GOGOMAIL_CARDDAV_ALLOW_INSECURE_AUTH`, and shares the existing HTTP
      server timeout/header guardrails.
1109. CardDAV `addressbook-query` now preserves the first `prop-filter` name
      and applies bounded `text-match` evaluation to parsed unfolded vCard
      property values, while rejecting missing or invalid `prop-filter` names
      before handler execution.
1110. CardDAV `text-match` handling now honors RFC 6352 defaults and
      attributes for the first parsed query predicate: default
      `i;unicode-casemap` collation, `equals`, `contains`, `starts-with`,
      `ends-with`, and `negate-condition`, while rejecting unsupported
      collations or malformed text-match attributes instead of silently
      changing query semantics.
1110a. CardDAV filter parsing now rejects duplicate `text-match` elements
       inside a single `prop-filter` or `param-filter`, preserving RFC 6352's
       singular text-match grammar instead of silently widening or ignoring
       native-client address-book query predicates.
1111. CardDAV `addressbook-query` now evaluates the first nested
      `param-filter` under a `prop-filter`, parsing unfolded vCard content-line
      parameters and supporting parameter existence, `is-not-defined`, and
      text-match checks before returning matching contact objects.
1112. CardDAV `addressbook-query` now evaluates multiple `prop-filter`
      predicates and multiple per-property `text-match`/`param-filter`
      predicates with RFC 6352 `test=anyof|allof` composition at both the
      top-level `filter` and individual `prop-filter` levels.
1113. CardDAV REPORT `address-data` now honors requested vCard property
      selectors such as `<C:prop name="FN"/>`, projecting returned contact data
      while preserving structural BEGIN/VERSION/END lines and omitting
      unrequested contact properties.
1114. CardDAV REPORT `address-data` parsing now validates requested
      `content-type` and `version` attributes against the advertised supported
      `text/vcard` data types, rejecting unsupported formats before handler
      execution.
1115. CardDAV REPORT `address-data` responses now emit explicit
      `content-type="text/vcard"` and `version` attributes, keeping
      returned contact data aligned with the advertised supported address-data
      type.
1116. CardDAV `addressbook-query` execution now honors bounded
      `limit/nresults` values by capping returned matching responses before
      building the multistatus body.
1117. CardDAV repository-backed query execution now has a streaming object
      walker path, letting handlers evaluate filters and stop once
      `limit/nresults` is satisfied without materializing the whole address
      book collection.
1118. CardDAV `address-data` projection failures now return explicit handler
      errors instead of silently falling back to full vCard bodies when a
      projected response cannot be built.
1119. CardDAV PROPFIND responses now expose conservative RFC 3744-shaped
      `current-user-privilege-set` values. Principals, homes, and address-book
      collections advertise `DAV:read`; address-book collections also
      advertise `DAV:write-properties` now that collection `PROPPATCH`
      semantics exist; contact objects additionally advertise
      `DAV:write-content` because object `PUT`/`DELETE` semantics exist today.
      Address-book homes advertise `DAV:bind` because extended `MKCOL` can
      create child address-book collections there and `DAV:unbind` because
      collection `DELETE` can remove them. ACL and broader collection write
      privileges remain unadvertised until their exact WebDAV semantics exist.
1120. CardDAV address-book collection PROPFIND now exposes the
      CalendarServer-compatible `getctag` extension from the same durable
      collection sync token used for WebDAV `sync-token`, improving native
      client change detection without introducing a second version source.
1121. CardDAV address-book collection PROPFIND now returns RFC 6352
      `addressbook-description` from stored address-book metadata, keeping
      client-visible collection discovery aligned with the repository model.
1121a. CardDAV address-book collection `PROPFIND Depth: 1` child-object
       discovery now uses the bounded object limiter with the default
       one-extra-row truncation probe, returning an explicit request error
       rather than a partial WebDAV multistatus for oversized address books.
1122. CardDAV now handles WebDAV `PROPPATCH` for authenticated address-book
      collections, parsing bounded namespace-aware `propertyupdate` bodies for
      `DAV:displayname` and RFC 6352 `addressbook-description`, updating the
      repository through a small collection-metadata boundary, refreshing the
      durable sync token, and appending an `addressbook-updated` change row.
      Unknown structural children inside `DAV:set` and `DAV:remove` are now
      rejected as parse errors, while unsupported/protected property failure
      semantics remain scoped to property elements inside `DAV:prop`.
      The `MaxWebDAVProperties` parser bound is enforced across the complete
      `propertyupdate` body, counting supported, unsupported, and protected
      properties even when clients split them across multiple `DAV:prop`
      blocks. Repeated mutable properties keep document-order final-value
      semantics, while success and failed-dependency `propstat` responses
      report each repeated property name once. `DAV:remove` property elements
      are required to be empty name markers; text values or nested XML children
      are rejected as malformed requests before mutation or property-failure
      handling. Empty `DAV:set` and `DAV:remove` instructions are rejected even
      when sibling instructions contain valid properties, and each instruction
      now accepts exactly one `DAV:prop` child per the RFC 4918 `set (prop)` /
      `remove (prop)` grammar. `DAV:prop xml:lang` is persisted for
      `DAV:displayname` and `CARDDAV:addressbook-description`, returned on
      `PROPFIND` and successful `PROPPATCH`, and cleared with removed
      description values. Extended `MKCOL` creation now stores the same
      `xml:lang` metadata and exposes it on the newly created address book's
      `PROPFIND` responses. Migration guardrails assert 0097 keeps the CardDAV
      language columns, constraints, and Down cleanup aligned with repository
      SQL, and optional PostgreSQL integration coverage now verifies migrated
      create/get/update round-trips plus raw column persistence. Negative
      PostgreSQL coverage bypasses application validation and asserts migration
      0097 rejects invalid language values with the expected check constraints.
      PROPPATCH parsing now distinguishes absent `xml:lang` from explicit empty
      `xml:lang=""`, preserving existing CardDAV language tags on text-only
      updates unless clients explicitly clear them. Explicit empty language
      clearing is covered through handler and PostgreSQL repository tests, and
      empty stored language values are emitted without `xml:lang` attributes.
      Unsupported/protected-property rollback tests now assert attempted
      language changes are not persisted and repository updates are not called
      on RFC 4918 atomic failure responses. Conditional failure coverage now
      asserts `If-Match`, `If-None-Match`, and `If-Unmodified-Since`
      preconditions reject before body reads, leaving attempted language
      mutations unapplied. Conditional success coverage now asserts matching
      collection `If-Match` preserves omitted language tags and `If-Match: *`
      carries observed collection ETags alongside explicit language updates.
      WebDAV `If` header coverage now asserts matching collection conditions
      preserve omitted language tags and failing conditions reject before body
      reads with language mutations unapplied. Optional PostgreSQL integration
      tests now verify observed collection ETag success preserves omitted
      language tags and stale observed ETags reject attempted text/language
      mutations before persistence. Tagged WebDAV `If` header coverage now
      asserts exact collection path tags preserve omitted language tags and
      non-matching or stale-ETag tagged lists reject before body reads. WebDAV
      `If` `Not` condition-list coverage now asserts negated stale ETags allow
      text-only updates with omitted language preservation while negated current
      ETags reject before body reads. Multi-list WebDAV `If` coverage now
      asserts a later matching condition list can satisfy the precondition
      after an earlier stale list, while all-failed lists reject before body
      reads. Malformed WebDAV `If` coverage now asserts parser errors return
      HTTP 400 before body reads and before attempted language mutations,
      including line breaks, unterminated lists/tokens, empty lists, and
      unsupported conditions. Absolute URI tagged `If` coverage now asserts
      HTTP(S) resource tags match by collection path, preserving omitted
      language tags on matching paths and rejecting non-matching paths before
      body reads. State-token `If` coverage now documents the no-lock-store
      semantics: bare state tokens reject before body reads, while negated
      state tokens allow text-only updates and preserve omitted language tags.
      Compound WebDAV `If` coverage now asserts multiple conditions inside one
      list are conjunctive for both omitted-language success and pre-body-read
      rejection. Repeated HTTP `If` header coverage now asserts joined header
      values preserve WebDAV condition-list semantics for omitted-language
      success and pre-body-read failure. Repeated malformed WebDAV `If`
      coverage now asserts later malformed relevant lists still return HTTP
      400 before body reads even when an earlier list already matched.
      Irrelevant tagged malformed `If` coverage now asserts condition-list
      syntax is validated before resource-tag relevance, returning HTTP 400
      before body reads for non-matching tagged malformed lists. Trailing
      WebDAV `If` coverage now rejects tokens left after a valid condition
      list with HTTP 400 before body reads. Malformed prefix `If` coverage now
      rejects non-empty condition-list prefixes unless they are valid
      `<resource-tag>` forms. Empty resource-tag `If` coverage now asserts
      `<>` prefixes return HTTP 400 before body reads. Resource-tag suffix
      coverage now rejects raw `<` or `>` inside tag content with HTTP 400.
1123. CardDAV address-book collections now derive a strong collection ETag
      from the durable sync token, expose it through WebDAV `getetag`, and use
      it with `If-Match` plus `If-Unmodified-Since` to reject stale collection
      `PROPPATCH` requests before reading XML request bodies.
1124. CardDAV current-user privilege discovery now tracks implemented
      collection metadata writes: address-book collections advertise
      `DAV:write-properties` only after `PROPPATCH` support exists, while
      broader collection, ACL, bind, and unbind privileges remain unadvertised.
1125. CardDAV now handles RFC 6352-style extended `MKCOL` for authenticated
      address-book collection creation at UUID request-URI paths. The handler
      validates the requested path, home/principal scope, non-existence, and
      bounded WebDAV creation XML for `DAV:resourcetype`,
      `DAV:displayname`, and `CARDDAV:addressbook-description`, then creates
      the collection through the repository with durable sync/change state and
      returns `201 Created` with `Location`. Unsupported creation properties
      and unsupported resource type values now fail atomically before creation
      with an RFC 5689 `DAV:mkcol-response`; failing properties return
      `403 Forbidden`, dependent properties return `424 Failed Dependency`,
      and OPTIONS discovery advertises `extended-mkcol`. Address-book creation
      now requires an extended MKCOL body with `DAV:resourcetype` including both
      `DAV:collection` and `CARDDAV:addressbook`, so empty or generic MKCOL
      bodies cannot accidentally create address books. Whitespace-only
      non-empty bodies are rejected as malformed XML before the resource-type
      path, and unknown structural children outside `DAV:prop` now return
      parse errors instead of being skipped into semantic failure handling.
      Creation success and
      property-failure responses now include `Cache-Control: no-store, no-cache`.
      Explicit non-empty request-body `Content-Type` headers are validated
      before XML parsing: malformed or duplicate values return `400`, non-XML
      media types return `415`, and absent headers remain accepted for client
      compatibility. Unsupported properties from arbitrary XML namespaces are
      preserved in `PROPPATCH` and extended `MKCOL` failure responses with a
      scoped fallback namespace declaration instead of surfacing as `500`.
1126. CardDAV current-user privilege discovery now advertises `DAV:bind` and
      `DAV:unbind` on address-book homes after child collection `MKCOL` and
      `DELETE` support.
1127. CardDAV now handles `DELETE` on authenticated address-book collection
      paths, soft-deleting the collection and active child contact objects in
      one repository transaction, honoring collection ETag/time preconditions,
      recording an `addressbook-deleted` change row, and rejecting home,
      contact-object, and cross-user collection-delete targets.
1128. CardDAV `sync-collection` can answer stale-token requests after an
      address-book collection has been deleted by reading durable change rows
      and returning the latest deletion sync token without requiring the
      collection to remain active.
1128a. CardDAV sync-change retention now has a bounded repository prune
       boundary and prune-order migration index. `PruneAddressBookChanges` can
       dry-run or delete old RFC 6578 address-book change rows without removing
       the newest marker per address book, so future Contacts retention workers
       can expire history while preserving current-token continuity; unknown or
       expired client tokens continue to fail with DAV `valid-sync-token`.
1129. CardDAV contact-object writes now preflight duplicate active vCard UIDs
      within the same address book before SQL upsert, returning a predictable
      repository/handler error while the PostgreSQL partial unique index stays
      as the final concurrency guard.
1130. CardDAV contact-object upsert now maps PostgreSQL unique-index races for
      active object names or vCard UIDs into stable repository errors, so
      concurrent writes keep developer-readable CardDAV failure semantics
      instead of exposing raw driver details.
1131. CardDAV contact-object `DELETE` now carries observed strong ETags into
      the repository transaction and rechecks `If-Match` state under the
      address-book lock before deleting the active object row, closing the
      precondition race between handler lookup and storage mutation.
1132. CardDAV `addressbook-query` now follows the RFC 6352 Depth contract more
      closely: the handler requires an explicit `Depth` header, uses `Depth: 1`
      for address-object child scans, and treats `Depth: 0` as
      collection-scoped rather than returning every child object.
1133. CardDAV `addressbook-multiget` now requires an explicit `Depth` header
      before resolving requested hrefs, preserving the REPORT's depth-scoped
      request model while accepting common Depth 0/1 native-client shapes.
1134. CardDAV `sync-collection` now enforces RFC 6578 Depth behavior by
      accepting the default or explicit `Depth: 0` request scope and rejecting
      `Depth: 1` before sync lookup or change-log work.
1135. CardDAV `sync-collection` parsing now distinguishes an empty initial
      `DAV:sync-token` element from a missing element and rejects requests that
      omit the required token element before sync lookup or snapshot work.
1136. CardDAV `addressbook-query` now rejects unsupported vCard property or
      parameter filters with the RFC 6352 `CARDDAV:supported-filter`
      precondition instead of returning misleading empty success responses.
1137. CardDAV `addressbook-query` now validates supported filter names before
      applying Depth scope, so unsupported filters still return the RFC 6352
      precondition on `Depth: 0` requests instead of being hidden by an empty
      collection-scoped response.
1138. CardDAV address-book collection privilege discovery now advertises
      `DAV:bind`/`DAV:unbind` after contact-object `PUT`/`DELETE` support,
      aligning `current-user-privilege-set` with implemented child `.vcf`
      member creation and removal semantics.
1139. CardDAV `addressbook-query` initially accepted RFC 6352
      `Depth: infinity` requests with the same flat address-book scan
      semantics as `Depth: 1`; later release gating tightened this so
      infinity traversal probes fail before XML body parsing.
1140. CardDAV REPORT parsing now maps unsupported requested `address-data`
      content types or versions to the RFC 6352
      `CARDDAV:supported-address-data` precondition instead of a generic
      bad-request text error.
1141. CardDAV `addressbook-query` now maps unsupported text-match collations to
      the RFC 6352 `CARDDAV:supported-collation` precondition instead of a
      generic bad-request parse failure.
1142. CardDAV `addressbook-query` now maps unsupported CardDAV filter child
      elements to the RFC 6352 `CARDDAV:supported-filter` precondition instead
      of a generic bad-request parse failure.
1143. CardDAV address-book collections now advertise RFC 6352
      `CARDDAV:supported-collation-set` with working `i;ascii-casemap` and
      `i;unicode-casemap` text-match evaluation.
1144. CardDAV PROPFIND selection now keeps capability properties available
      through explicit `prop`, `include`, and `propname` discovery while
      omitting allprop-unfriendly properties from bare `allprop` responses.
1145. IMAP command and IDLE line reads now enforce the command-line byte cap
      while reading from the socket instead of after an unbounded line
      allocation.
1146. IMAP `LOGIN` and SASL PLAIN decoded credentials now reject blank,
      CR/LF-bearing, or oversized authentication identities plus oversized or
      CR/LF-bearing passwords before backend auth work.
1147. IMAP server coverage now verifies literalized `LOGIN` commands with
      separate synchronizing user-name and password literals, including the
      reconstructed credentials delivered to backend authentication.
1148. CalDAV calendar-home discovery now returns WebDAV
      `current-user-principal` and `owner` hrefs anchored to the canonical
      principal URL, keeping principal semantics ready for future
      Directory/Identity-backed delegated and shared calendars.
1149. CalDAV discovery now exposes RFC 3744-shaped
      `current-user-privilege-set` values for implemented local-user behavior
      only: principal reads, calendar-home calendar bind/unbind, collection
      object bind/unbind plus metadata property writes, and object content
      writes.
1150. CalDAV now resolves the advertised `/caldav/principals/` principal
      collection for `PROPFIND`, returning collection metadata at `Depth: 0`
      and the authenticated principal as a `Depth: 1` child without exposing
      unrelated users.
1151. CardDAV now resolves the advertised `/carddav/principals/` principal
      collection for `PROPFIND`, returning collection metadata at `Depth: 0`
      and the authenticated principal as a `Depth: 1` child without exposing
      unrelated users.
1152. IMAP bare `UID` commands now return `BAD UID requires subcommand`, keeping
      missing-subcommand diagnostics distinct from unknown but well-formed UID
      subcommands.
1153. CalDAV `REPORT sync-collection` can now return a final top-level sync
      token for stale-token clients after a calendar collection has been
      deleted, using the durable `collection-deleted` change row instead of
      requiring the live calendar row to still exist.
1154. CalDAV service-root `PROPFIND` now exposes `/caldav/` as a read-only
      collection discovery anchor instead of reusing authenticated principal
      properties, keeping `calendar-home-set` and other principal-only
      semantics on the principal resource while still advertising
      `current-user-principal` and `principal-collection-set`.
1155. CalDAV Directory discovery now explicitly converts only Directory user
      principals into CalDAV principals, rejecting organization, group, and
      resource principals until delegated/shared calendar and resource-booking
      semantics have real policy and storage boundaries.
1156. CalDAV `Allow` headers now come from an explicit implemented-method list
      shared by `OPTIONS` and 405 responses, so future constants such as
      `COPY` and `MOVE` do not leak into native-client capability discovery
      before handler semantics exist.
1157. CardDAV `Allow` headers now also come from an explicit implemented-method
      list shared by `OPTIONS` and 405 responses, matching the CalDAV capability
      pattern and keeping native contact clients aligned with real handlers.
1158. IMAP `ENABLE` now has explicit regression coverage for RFC 5161-compatible
      unknown capability handling: malformed capability atoms are rejected, but
      syntactically valid unsupported names are ignored and can return an empty
      `ENABLED` response.
1159. Storage portability now has a reusable backend-neutral contract test and
      documented migration smoke matrix covering canonical special-character
      keys plus `Put`, `Get`, `GetRange`, `Stat`, `List`, `Copy`, `Move`,
      idempotent `Delete`, and bounded `DeletePrefix` across local/NFS and
      optional S3-compatible integration backends.
1160. Directory/Identity now has a company-scoped delegation foundation:
      `directory_delegations` records owner principal, delegate principal,
      product scope (`calendar`, `contacts`, `drive`, `mailbox`), hierarchical
      role (`read`, `write`, `manage`), active uniqueness, and lookup indexes;
      `internal/directory` exposes normalized delegation checks so future
      CalDAV/CardDAV/Drive/shared-inbox access can share one auditable
      relationship model.
1161. IMAP successful `LOGIN` and `AUTHENTICATE PLAIN` responses now include an
      authenticated `[CAPABILITY ...]` response code, exposing the post-auth
      capability set immediately while keeping pre-auth `SASL-IR` and
      `AUTH=PLAIN` out of authenticated capability advertisements.
1162. IMAP connection greetings now include a state-aware `[CAPABILITY ...]`
      response code: plaintext TLS-required sessions advertise
      `STARTTLS`/`LOGINDISABLED`, and implicit TLS sessions advertise immediate
      `SASL-IR`/`AUTH=PLAIN` login capability.
1163. Drive public share-link resolution/download now has a backend API
      boundary: token paths resolve only by SHA-256 hash, enforce active,
      unexpired, unrevoked links plus active owner/domain/file state, keep
      storage backend/path details out of metadata responses, and reuse the
      no-store, checksum, HEAD, and single-range download contract for
      `download`-permission links.
1164. Drive public share endpoints now have a configurable Redis-backed
      fixed-window abuse-control boundary for anonymous metadata/download
      traffic, bucketed by normalized remote address plus share token, returning
      HTTP 429 with `Retry-After` on quota exhaustion while failing open on
      transient limiter errors after startup.
1165. Drive public share metadata and download successes now emit best-effort
      immutable audit-log rows under `category=drive`, capturing sanitized
      link/node/request metadata and byte-range intent without raw tokens or
      storage backend/path values, so Admin audit-log filters can inspect
      public-link activity before a dedicated activity dashboard exists.
1165a. Drive public share denied token/permission checks and rate-limited
       requests now use the same best-effort audit boundary, capturing action,
       result, status, normalized remote address, token suffix, and available
       link/node metadata without recording raw share tokens or storage
       backend/path values.
1166. CalDAV principal discovery now exposes the Directory primary email as an
      RFC 4791 `calendar-user-address-set` `mailto:` href when available,
      giving future organizer/attendee and scheduling work a standards-shaped
      principal-address boundary without prematurely enabling public scheduling
      or delegated/shared calendar semantics.
1167. Directory/Identity now exposes `CheckEffectiveDelegation`, a bounded
      group-expansion access check that preserves direct delegation behavior,
      applies the same `manage >= write >= read` role hierarchy, respects
      active-only owner/delegate principal checks plus group filters, and lets
      group-granted delegations satisfy effective user, organization, group, or
      resource members without adding product-local sharing models.
1168. A new `internal/accesspolicy` boundary wraps Directory effective
      delegation into normalized allow/deny decisions, forcing active
      principal checks and giving CalDAV/CardDAV/Drive/mailbox/admin modules a
      product-neutral policy adapter surface before public sharing semantics
      are wired to protocol privileges or audit logs.
1169. `internal/accesspolicy` now maps allowed delegation decisions to RFC
      4918 WebDAV privilege names, giving CalDAV/CardDAV a single
      read/write/manage-to-privilege translation point before shared calendar
      or delegated address-book privileges are advertised.
1170. `internal/accesspolicy` now builds bounded delegated-access audit detail
      JSON from the same normalized request and decision boundary, including
      company, owner, actor, scope, role, allow/deny state, fixed reason enums,
      and WebDAV privileges so future CalDAV/CardDAV/Drive/mailbox audit
      emitters do not invent divergent log shapes or accept free-form reason
      cardinality.
1171. `internal/accesspolicy` now also builds the delegated-access `audit.Log`
      envelope with stable `access` category, `delegation.access_checked`
      action, owner principal target, actor principal, and fixed
      `allowed`/`denied` results so product adapters can insert consistent
      audit records without duplicating envelope conventions.
1172. Admin audit-log listing now supports bounded `actor_id` and `target_id`
      filters in the repository, HTTP API, OpenAPI contract, and backend API
      docs. New partial actor/time and target/time indexes make
      delegated-access forensic queries operable by acting principal or
      owner/resource target.
1173. `internal/accesspolicy` now exposes `DelegationAuditRecorder`, a small
      repository-backed recorder that validates and inserts the standard
      delegated-access audit log envelope. Future CalDAV/CardDAV/Drive/mailbox
      adapters can record policy decisions without duplicating audit row
      construction or bypassing the shared audit repository boundary.
1174. IMAP `FETCH` and `UID FETCH` now preserve requested
      `HEADER.FIELDS (...)` and `HEADER.FIELDS.NOT (...)` section names in
      `BODY[...]` literal response items, including partial-window suffixes,
      so RFC 3501-shaped header subset clients can correlate responses without
      treating every subset read as a generic `BODY[HEADER]` literal.
1175. `internal/accesspolicy` now exposes `DelegatedAccessAuthorizer`, a
      composed effective-delegation check plus audit insertion boundary. It
      normalizes each request once, records both allowed and denied decisions
      with the standard delegated-access audit envelope, skips fabricated audit
      rows on checker errors, and fails closed on audit insertion errors so
      future CalDAV/CardDAV/Drive/mailbox adapters do not permit unaudited
      delegated access.
1175a. Directory/Identity now exposes a bounded `ListDelegations` repository
       boundary for owner/delegate/scope/role-filtered delegation inspection.
       Requests validate company scope, optional principal filters, active-only
       state, and result limits before SQL execution, giving future admin
       consoles, shared-calendar management, Drive shares, shared inboxes, and
       Contacts/CardDAV delegation one observable relationship read model
       instead of product-local delegation queries.
1175b. The admin backend API now exposes Directory delegation inspection at
       `GET /admin/v1/directory/delegations`, returning a
       `directory_delegations` envelope with bounded company, owner, delegate,
       scope, role, active-only, and limit filters documented in OpenAPI and
       the backend API contract. This gives the future admin console a
       contract-first diagnostics surface without making delegated CalDAV,
       Drive, Contacts/CardDAV, or shared-inbox mutation workflows public.
1175b1. Directory/Identity now exposes audited delegation creation through
        `CreateDelegationWithAudit` and
        `POST /admin/v1/directory/delegations`, returning a
        `directory_delegation` envelope. The boundary normalizes principal
        kinds, scope, and role, verifies active same-company owner/delegate
        principals, rejects self-delegation, maps duplicate active grants to a
        stable error, and commits `directory_delegation.create` with the grant.
1175b2. Directory/Identity now exposes audited delegation deletion through
        `DeleteDelegationWithAudit` and
        `DELETE /admin/v1/directory/delegations/{id}`, soft-deleting active
        grants and committing `directory_delegation.delete` in the same
        transaction so shared-calendar, Drive, Contacts/CardDAV, and shared
        inbox access can be revoked through one platform boundary.
1175b2a. Directory/Identity now exposes audited delegation role updates through
         `UpdateDelegationRoleWithAudit` and
         `PATCH /admin/v1/directory/delegations/{id}/role`, changing active
         grants under active companies in-place and committing
         `directory_delegation.role_update` with previous/new role detail in
         the same transaction. This keeps shared-calendar, Drive,
         Contacts/CardDAV, and shared inbox role semantics in one platform
         boundary.
1175b2b. Directory/Identity now enforces a single active delegation grant per
         company/owner/delegate/scope at the database boundary, independent of
         role. Role changes are therefore auditable mutations of one
         relationship instead of parallel active grants with conflicting
         privilege semantics; create and update uniqueness races map to the
         same stable duplicate-delegation error.
1175b2c. Directory/Identity now exposes audited delegation reassignment through
         `ReassignDelegationWithAudit` and
         `PATCH /admin/v1/directory/delegations/{id}/assignment`, moving active
         grants to a new owner/delegate/scope while preserving the role,
         validating active same-company new principals, mapping duplicate
         active grants to the stable duplicate-delegation error, and committing
         `directory_delegation.reassign` in the same transaction.
1175b3. Directory/Identity now exposes audited group membership creation
        through `CreateGroupMembershipWithAudit` and
        `POST /admin/v1/directory/group-memberships`, returning a
        `directory_group_membership` envelope. The boundary validates active
        same-company group/member principals, membership role, self-membership,
        and nested group cycles, and commits
        `directory_group_membership.create` with the membership insert.
1175b4. Directory/Identity now exposes audited group membership deletion
        through `DeleteGroupMembershipWithAudit` and
        `DELETE /admin/v1/directory/group-memberships/{id}`, soft-deleting
        active memberships and committing
        `directory_group_membership.delete` in the same transaction so
        group-backed delegation, resource access, and shared inbox membership
        can be revoked through one platform boundary.
1175b5. Directory/Identity now exposes group membership listing through
        `ListGroupMemberships` and
        `GET /admin/v1/directory/group-memberships`, returning a
        `directory_group_memberships` envelope with bounded company, group,
        member, role, active-only, and limit filters so operators can inspect
        group-backed access without product modules querying Directory tables.
1175b6. Directory/Identity now exposes audited group membership role updates
        through `UpdateGroupMembershipRoleWithAudit` and
        `PATCH /admin/v1/directory/group-memberships/{id}/role`, changing
        active membership roles in-place and committing
        `directory_group_membership.role_update` with the role change.
1175b7. Directory/Identity now exposes audited group membership reassignment
        through `ReassignGroupMembershipWithAudit` and
        `PATCH /admin/v1/directory/group-memberships/{id}/assignment`,
        preserving the role while moving an active membership to a different
        group/member assignment and committing
        `directory_group_membership.reassign` with cycle and duplicate guards.
1175c. The admin backend API now exposes Directory principal search at
       `GET /admin/v1/directory/principals`, returning a
       `directory_principals` envelope over the existing bounded
       `SearchPrincipals` repository. Company, domain, organization,
       comma-separated kind, query, active-only, and limit filters are
       documented in OpenAPI and the backend API contract, giving admin
       consoles, CalDAV attendee/resource lookup, Contacts/CardDAV
       autocomplete, Drive sharing, and shared inbox targeting one
       contract-first principal discovery surface.
1175d. The admin backend API now exposes Directory alias resolution at
       `GET /admin/v1/directory/aliases/resolve`, returning a
       `directory_alias` envelope with the resolved target principal. The
       endpoint normalizes the requested email address through the shared mail
       address parser and keeps active-only lookup explicit, giving mail
       routing diagnostics, attendee resolution, shared inbox targeting, and
       admin consoles one address-to-principal contract.
1175e. Directory/Identity now exposes a bounded `ListAliases` repository
       boundary for alias inspection. Requests validate company/domain scope,
       optional target principal filters, text query length, active-only state,
       and result limits before SQL execution, then resolve each returned alias
       through the shared principal resolver so shared inbox management,
       mail-routing diagnostics, and admin alias screens do not query
       `directory_aliases` directly.
1175f. The admin backend API now exposes Directory alias listing at
       `GET /admin/v1/directory/aliases`, returning a `directory_aliases`
       envelope with bounded company, domain, target principal, query,
       active-only, and limit filters documented in OpenAPI and the backend API
       contract. This gives future admin alias screens and shared inbox
       management a contract-first read surface while alias mutation policy and
       audit semantics remain future work.
1175g. Directory/Identity now exposes a guarded `CreateAlias` repository
       mutation boundary. It normalizes alias addresses, requires active
       company/domain scope, verifies the alias address domain matches the
       Directory domain, resolves an active same-company target principal, and
       maps active-address unique-index races to a predictable duplicate-alias
       error.
1175h. The admin backend API now exposes audited Directory alias creation at
       `POST /admin/v1/directory/aliases`, returning a `directory_alias`
       envelope. The API uses the transaction-audited Directory mutation
       boundary so the alias insert and `directory_alias.create` audit row
       commit together; shared-inbox UX and non-admin product flows remain
       future work.
1175i. The admin backend API now exposes audited Directory alias deletion at
       `DELETE /admin/v1/directory/aliases/{id}`, soft-deleting active aliases
       and committing the `directory_alias.delete` audit row in the same
       transaction so operators can safely reclaim alias addresses without
       product-local mutation semantics.
1176. S3-compatible storage `Copy` now reads and validates bounded successful
      `CopyObject` response bodies, accepting normal `CopyObjectResult`
      responses while rejecting embedded `<Error>` XML inside `200 OK`
      responses. This keeps AWS S3/compatible copy failures from being treated
      as successful Drive, attachment lifecycle, or reconciliation object
      duplication.
1177. Drive runtime storage wiring now treats configured `s3` and `minio`
      backends as bidirectional labels for the same S3-compatible store. Rows
      persisted with `storage_backend='minio'` can still be served after an
      AWS S3-style config flip, and rows persisted with `storage_backend='s3'`
      can still be served by a MinIO-style deployment when object keys and
      bucket contents are compatible.
1178. IMAP `SEARCH` and `UID SEARCH` now preserve RFC 3501 zero-length search
      string semantics for quoted empty strings across envelope, body/text, and
      header substring criteria. Empty search strings remain syntactically
      valid substring searches instead of being normalized into guaranteed
      no-match results, and escaped quote characters inside search strings
      remain literal query text instead of being trimmed from the query
      boundary.
1179. CalDAV `REPORT sync-collection` now enforces HTTP `Depth: 0` before
      repository lookup or change-log work, aligning the gateway with RFC
      6578 request-scope semantics and the existing CardDAV behavior while
      keeping child traversal controlled by the required body `sync-level`.
1180. CalDAV `REPORT calendar-query` now honors HTTP `Depth: 0` by returning
      no child calendar-object matches unless the client explicitly requests
      `Depth: 1`, preventing collection-scoped searches from silently widening
      their WebDAV request scope.
1181. S3-compatible missing-object reads now wrap `os.ErrNotExist` for `GET`,
      ranged `GET`, and `HEAD`/`Stat` `404 Not Found` responses, keeping
      backend-neutral missing-object handling aligned with local/NFS storage
      while retaining sanitized S3 status diagnostics.
1182. IMAP `SELECT` and `EXAMINE` now require optional `CONDSTORE` select
      parameters to arrive as a single RFC-shaped parenthesized select-param
      list, rejecting bare `CONDSTORE` and over-parenthesized
      `((CONDSTORE))` before authentication or backend mailbox lookup.
1183. CalDAV `REPORT sync-collection` now requires an explicit
      `DAV:sync-token` element in the request body, accepting an empty element
      for initial sync while rejecting omitted sync-token anchors before
      repository work. This keeps RFC 6578 sync state transitions aligned with
      the existing CardDAV behavior.
1184. CalDAV stale-token `sync-collection` delta reads now request one extra
      change-log row behind bounded `limit/nresults`, allowing exact-limit
      change sets to complete while still rejecting genuinely truncating
      responses until continuation support exists.
1185. CardDAV stale-token `sync-collection` delta reads now use the same
      bounded one-extra-row probe as CalDAV, allowing exact-limit address-book
      change sets to complete while still rejecting genuinely truncating
      responses until continuation support exists.
1186. CalDAV and CardDAV initial `sync-collection` snapshots now use
      sync-specific one-extra-object repository list paths, so omitted-limit
      snapshots cannot be silently clipped by generic list defaults while still
      returning current collection sync tokens.
1186a. CalDAV and CardDAV `sync-collection` truncation now returns RFC 6578
      compliant HTTP 403 with `<D:number-of-matches>0</D:number-of-matches>`
      instead of HTTP 400 with a generic error message, aligning bounded
      sync responses with the RFC requirement for explicit truncation signaling.
1187. IMAP command framing now turns oversized command literals into a tagged
      `BAD` response when the command tag is available, followed by `BYE` and a
      clean connection close. This keeps literal-size enforcement observable to
      clients without trying to resynchronize an unrecoverable input stream.
1188. Shared storage `Get` and `GetRange` readers now observe context
      cancellation after local/NFS files or S3-compatible response bodies have
      opened, and local/NFS `GetRange` now reports `io.ErrUnexpectedEOF` for
      short requested byte windows. This keeps canceled downloads/previews and
      partial-read failure semantics consistent across backend flips.
1189. IMAP `LOGIN` and SASL PLAIN credential validation now rejects empty
      passwords while preserving intentional leading/trailing spaces in quoted
      or decoded credential values, keeping the auth boundary strict without
      rewriting RFC string operands before backend verification.
1190. Local/NFS storage now rejects filesystem symbolic links for object reads,
      range reads, metadata probes, deletes, and source moves, hides symlinks
      from list pages, and rejects direct directory deletes so mounted
      filesystems preserve backend-neutral object-store semantics instead of
      following host-specific links or treating folders as objects.
1191. S3-compatible full-object `GET` readers now bounded-drain a small
      response remainder on close, matching the existing range-reader cleanup
      shape so preview/cancel download paths can reuse HTTP connections without
      unbounded cleanup reads.
1192. CalDAV `REPORT calendar-query` now lists calendar objects through the
      bounded object limiter with a one-extra-row truncation probe, honoring
      explicit or default `limit/nresults` caps and rejecting partial result
      sets until continuation semantics are designed. This keeps large
      collection scans aligned with the existing sync-collection bounded
      snapshot contract.
1193. CalDAV `REPORT free-busy-query` now uses the same bounded object limiter
      and one-extra-row truncation probe for `Depth: 1` child-object scans,
      honoring explicit or default `limit/nresults` caps before deriving
      VFREEBUSY periods. This prevents large-calendar availability queries from
      silently producing partial busy windows.
1194. CalDAV calendar collection `PROPFIND Depth: 1` child-object discovery now
      uses the bounded object limiter with the default one-extra-row truncation
      probe, returning an explicit request error instead of a partial WebDAV
      multistatus when a collection is too large to enumerate safely.
1195. CalDAV `calendar-query` and `free-busy-query` now expand bounded VEVENT
      recurrence sets through the RFC 5545 parser, including library-backed
      `RRULE`, `EXDATE`, and `RDATE` handling. Dense or unbounded recurrence
      rules are capped per object so native-client time-range scans remain
      predictable while recurring events become visible in query and VFREEBUSY
      responses.
1195a. CalDAV `calendar-query` now evaluates VTODO time-range filters with RFC
       4791 overlap semantics for `DTSTART`, `DUE`, `DURATION`, `COMPLETED`,
       and `CREATED`, including effective `DUE = DTSTART + DURATION`, so
       supported VTODO objects participate in native-client range syncs.
1196. CalDAV iCalendar object validation now accepts the common recurring-event
      storage shape of one VEVENT master plus same-UID `RECURRENCE-ID`
      detached override VEVENTs. Calendar-query and free-busy evaluation now
      scan all VEVENTs in a stored object and suppress the replaced master
      occurrence when an override exists, improving RFC 5545 native-client
      compatibility without introducing a product-specific event model.
1196a. CalDAV `calendar-query` filter parsing now rejects RFC 4791-invalid
       `time-range` placement directly under `filter` and duplicate
       `time-range` elements within one component filter, keeping native-client
       range matching fail-closed instead of silently accepting ambiguous XML.
1197. Admin audit-log listing now supports bounded `action_prefix` filters,
      giving operators a contract-level way to inspect action families such as
      `share_link.` across successful, denied, and rate-limited public Drive
      share activity before a dedicated aggregate activity dashboard exists.
1198. IMAP SASL PLAIN decoding now rejects oversized encoded and decoded
      responses before credential splitting or backend authentication, keeping
      continuation and `SASL-IR` literal initial-response paths allocation-aware
      under the same username/password credential caps.
1199. CardDAV contact-object validation now accepts bounded vCard 3.0 and 4.0
      bodies, address-book discovery advertises both `text/vcard` versions, and
      returned `address-data` carries the stored vCard body version instead of
      assuming 4.0. This keeps the experimental CardDAV surface moving toward
      native-client compatibility without turning contacts into a generic CRUD
      API or introducing a separate principal model ahead of the Directory
      boundary.
1200. CardDAV contact-object `PUT` now validates explicit `text/vcard`
      media-type `version` parameters, accepting only 3.0/4.0 and requiring the
      parameter to match the vCard body `VERSION` before repository mutation.
      This prevents native clients and integrations from storing data under an
      ambiguous or over-advertised format contract.
1201. CardDAV vCard content-line parsing now finds the value separator at the
      first unquoted colon, so quoted parameters such as address labels can
      contain colons without causing valid contact objects to fail validation.
      This improves native-client compatibility while preserving bounded line
      and body validation.
1202. IMAP malformed-command parsing now returns tagged `BAD` responses when a
      syntactically valid command tag can still be recovered from the line,
      while preserving untagged `BAD` for malformed or missing tags. This keeps
      RFC-shaped clients able to correlate tokenizer failures with the command
      they issued.
1203. S3-compatible full-object `GET`, `HEAD`/`Stat`, and `ListObjectsV2` now
      require exact `200 OK` responses, rejecting unexpected partial-content or
      other non-OK 2xx statuses so partial provider responses cannot be treated
      as complete backend-neutral object reads, metadata, or list pages.
1204. S3-compatible `Copy` now requires exact `200 OK` responses with bounded
      `CopyObjectResult` XML bodies, rejecting empty bodies, unexpected XML, and
      non-OK 2xx statuses so Drive and lifecycle object duplication cannot
      accept ambiguous provider copy acknowledgements.
1205. S3-compatible `PutObject` now requires exact `200 OK` responses,
      rejecting accepted/deferred or otherwise non-OK 2xx statuses so mail,
      Drive, and lifecycle writes cannot treat ambiguous provider
      acknowledgements as durable object commits.
1206. S3-compatible `DeleteObject` now accepts only completed `200 OK` or
      `204 No Content` success responses plus idempotent `404 Not Found`,
      rejecting accepted/deferred or otherwise ambiguous non-OK 2xx statuses so
      cleanup workers cannot mark uncertain object deletes as complete.
1207. CalDAV iCalendar object validation now rejects RFC-invalid
      `VEVENT`/`VTODO` duration/end combinations, including `VEVENT`
      `DTEND`+`DURATION`, `VTODO` `DUE`+`DURATION`, and `VTODO` `DURATION`
      without `DTSTART`, before malformed calendar objects can reach storage.
1208. CalDAV iCalendar object validation now rejects duplicated singleton
      time/status properties such as `DTSTAMP`, `DTSTART`, `DTEND`, `DUE`,
      `DURATION`, `STATUS`, `TRANSP`, and `RECURRENCE-ID` on supported
      calendar components, keeping stored `.ics` objects deterministic for
      future native-client sync and scheduling semantics.
1209. CalDAV iCalendar object validation now enforces RFC 5545 `VCALENDAR`
      root shape by requiring exactly one `VERSION:2.0` and exactly one
      non-empty `PRODID`, rejecting legacy, missing, or duplicated root
      identity properties before calendar objects can be persisted.
1210. CalDAV iCalendar object validation now rejects stored `VCALENDAR`
      `METHOD` properties for calendar object resources, matching RFC 4791
      storage rules while preserving server-generated `METHOD:REPLY` free-busy
      responses outside the object-write path.
1211. CalDAV object `PUT` now rejects explicit non-`2.0` `text/calendar`
      media-version parameters before body parsing, aligning the HTTP media
      contract with the advertised `supported-calendar-data` version.
1212. CalDAV REPORT `calendar-data` parsing now rejects unsupported
      `content-type` and non-`2.0` `version` attributes, keeping multiget,
      query, and sync projection requests aligned with advertised calendar-data
      support instead of silently serving an unadvertised media variant.
1213. CalDAV and CardDAV object `PUT` handlers now reject repeated
      `Content-Type` headers before media parsing, preventing ambiguous
      calendar/contact body interpretation at the DAV object write boundary.
1214. S3-compatible `ListObjectsV2` decoding now requires bounded successful
      XML bodies to use the `ListBucketResult` root, preventing unexpected
      provider success XML from being accepted as an empty canonical object
      page.
1215. CalDAV and CardDAV `REPORT`/`PROPFIND` handlers now reject repeated HTTP
      `Depth` headers before XML request-body parsing, keeping WebDAV traversal
      scope deterministic for query, sync, and discovery paths.
1216. CalDAV and CardDAV conditional request handling now combines repeated
      `If-Match` and `If-None-Match` headers into a single ETag list before
      evaluating object and collection preconditions, preserving HTTP
      field-combination semantics for cache validation and write guards.
1217. IMAP `APPEND` internaldate parsing now enforces RFC 3501 fixed-width
      `date-day-fixed` syntax, accepting zero-padded or space-padded days while
      rejecting bare one-digit dates before backend append dispatch. Date-month
      atoms are canonicalized ASCII-case-insensitively before parsing.
1218. S3-compatible `ListObjectsV2` result processing now validates object size
      only after a returned key maps to the requested canonical gogomail prefix,
      preserving the storage contract that out-of-scope bucket keys are hidden
      from callers.
1219. IMAP `SEARCH` and `UID SEARCH` now reject `CHARSET` prefixes that omit
      the required following search-key before authentication or selected-state
      checks, preserving RFC 3501 grammar semantics at the command boundary.
1220. Shared storage list cursors now reject leading/trailing whitespace
      instead of trimming opaque provider tokens, preserving exact pagination
      identity across local/NFS and S3-compatible Drive, lifecycle, and
      reconciliation scans.
1221. IMAP `FETCH` and `UID FETCH` now validate malformed fetch data-item
      syntax such as nested `((FLAGS))` before authentication or selected-state
      checks, keeping RFC 3501 fetch grammar failures distinct from mailbox
      state failures.
1222. IMAP `STORE` and `UID STORE` now validate malformed `UNCHANGEDSINCE`,
      store mode, and flag-list syntax before authentication or selected-state
      checks, keeping RFC 3501 and CONDSTORE mutation grammar failures
      distinct from mailbox state failures.
1223. IMAP selected-state commands now validate malformed message sequence-set
      and UID set syntax, including signed values such as `+1` and `+7`,
      before authentication or selected-state checks while preserving
      selected-mailbox bounds validation for execution time.
1224. IMAP `SEARCH` and `UID SEARCH` now validate malformed search
      sequence-set and `UID` search-key set syntax before authentication or
      selected-state checks, so signed values such as `SEARCH +1` and
      `UID SEARCH UID +7` fail as RFC 3501 grammar errors instead of state
      errors.
1225. IMAP `SORT`, `UID SORT`, `THREAD`, and `UID THREAD` now reuse the same
      syntax-only search-key validation before authentication or selected-state
      checks, keeping malformed embedded criteria consistent across the
      search/sort/thread command family.
1226. Shared storage list cursors now reject embedded control characters as
      well as leading/trailing whitespace, keeping opaque provider pagination
      tokens safe for S3 query forwarding, local/NFS cleanup cursors, and
      operational logs.
1227. CalDAV `calendar-query` parsing now rejects component filters that omit
      the RFC 4791 `name` attribute or use a non-`VCALENDAR` top-level
      component filter, preventing malformed native-client search requests from
      silently widening into whole-calendar scans.
1228. CalDAV `calendar-query` filters now require exactly one top-level
      `VCALENDAR` component filter, rejecting missing, direct `time-range`, or
      repeated top-level filter shapes before calendar-object scan planning.
1229. Drive public share-link rate limiting now passes a share-token SHA-256
      digest, not the raw token, across the limiter interface while preserving
      normalized-remote plus token-scoped abuse buckets and existing 429
      semantics.
1230. IMAP `FETCH` and `UID FETCH` now reject unsupported data items at the
      RFC 3501 syntax boundary before authentication or selected-mailbox state
      checks, while preserving supported macro, `BODY`/`BODY.PEEK`,
      `RFC822.*`, header-field-list, partial body, MIME section, and
      `CHANGEDSINCE` request shapes.
1231. Drive permanent-delete object cleanup now records retry rows for every
      object not proven deleted after a partial cleanup failure, including the
      failed object and trailing unattempted objects, preventing post-commit
      metadata deletion from leaving S3/local/NFS storage drift untracked.
1232. IMAP `SELECT` and `EXAMINE` now emit RFC 4551-shaped `[NOMODSEQ]` when
      a CONDSTORE-aware selection has no persistent mailbox mod-sequence
      baseline, keeping advertised CONDSTORE behavior explicit for clients
      that use `SELECT ... (CONDSTORE)` or prior `ENABLE CONDSTORE`.
1233. CardDAV delegated contacts access now consumes the shared
      Directory/accesspolicy/audit boundary instead of inventing a
      contacts-local sharing model. Cross-user address-book and contact-object
      paths require the matching `contacts` read/write/manage delegation role,
      execute against the owner store when allowed, and derive delegated
      `DAV:current-user-privilege-set` discovery and REPORT responses from the
      same policy decision.
1234. CalDAV delegated calendar access now applies the same access-policy
      decision to REPORT and sync `DAV:current-user-privilege-set` calendar
      object responses, not only PROPFIND, and treats missing Directory
      principals as fail-closed authorization denial instead of a distinct
      server-error path.
1235. S3-compatible storage `Move` now returns a structured cleanup error when
      server-side copy succeeds but source deletion fails, carrying source and
      destination paths so callers can distinguish recoverable duplicate-object
      cleanup from pre-copy move failures across AWS S3 and compatible stores.
1236. Shared `DeletePrefix` now returns a structured unsafe-listed-object
      partial-progress error when a backend listing yields a non-canonical
      object path, separating corrupt listing data from ordinary provider
      delete failures for lifecycle and reconciliation workers.
1237. IMAP `STATUS` now rejects empty parenthesized status data-item lists with
      an explicit tagged `BAD STATUS requires status data items`, keeping
      malformed `STATUS inbox ()` requests separate from unsupported or
      duplicate item validation.
1238. IMAP `THREAD` and `UID THREAD` now reject unsupported algorithms before
      authentication or selected-mailbox checks, keeping the advertised
      `THREAD=ORDEREDSUBJECT` capability boundary explicit when clients probe
      unsupported algorithms such as `REFERENCES`.
1239. IMAP LIST-STATUS parsing now preserves RFC-shaped status return
      diagnostics for malformed advertised `LIST RETURN (STATUS ...)` requests:
      empty status lists, unparenthesized status item lists, and unsupported or
      duplicate status items return specific tagged `BAD` responses instead of
      being collapsed into generic LIST arity failures.
1239. IMAP literal parsing regression coverage now locks malformed literal
      marker placement, trailing atom data after literal payloads, and unused
      literal payloads to parser-level `BAD` responses before command handlers
      can consume corrupted arguments.
1240. Production `s3` storage configuration now requires an explicit
      `GOGOMAIL_STORAGE_S3_ENDPOINT`, even for AWS regional endpoints, keeping
      release configs auditable about the object-store target while preserving
      development/test endpoint derivation from region.
1240a. Production `s3` storage configuration now also requires that endpoint
       to use HTTPS, preserving transport integrity for streaming SigV4
       `UNSIGNED-PAYLOAD` requests while keeping local HTTP MinIO on the
       explicit non-production `minio` backend.
1241. IMAP `STATUS` empty-list validation now also treats spaced empty item
      lists such as `STATUS inbox ( )` as `BAD STATUS requires status data
      items`, keeping equivalent malformed RFC 3501 status requests on the same
      client-visible diagnostic path.
1242. IMAP `IDLE` continuation reads now route oversized line framing errors
      through the same tagged `BAD command line is too long` plus `BYE`
      response path as ordinary commands, so long-lived clients receive a
      deterministic protocol close reason.
1243. CalDAV roadmap/status documentation now aligns collection `DELETE`
      wording with the implemented durable sync-change-log behavior: stale-token
      clients can receive object tombstones and final collection-deleted sync
      tokens, while long-history retention remains the explicit future gate.
1244. Directory direct delegation checks now require active owner and delegate
      principals when `ActiveOnly` is set, matching effective delegation
      fail-closed semantics so policy callers do not honor active delegation
      rows after either endpoint principal is suspended or deleted.
1245. CalDAV sync-change retention now has a bounded repository prune boundary
      and prune-order migration index. `PruneCalendarSyncChanges` can dry-run
      or delete old RFC 6578 change-log rows without removing the newest marker
      per calendar, so future retention workers can expire history while
      preserving current-token continuity; unknown or expired client tokens
      continue to fail with DAV `valid-sync-token`.
1246. `dav-sync-retention-worker` now runs CalDAV and CardDAV sync-change
      pruning on an interval or once-and-exit. It is dry-run by default,
      validates interval/cutoff/batch settings, and requires explicit
      `confirm_ready` before destructive runs, turning the DAV retention
      repository boundaries into an operationally safe worker without making
      token-expiry policy public/client-ready yet.
1247. S3-compatible `ListObjectsV2` request queries now use SigV4 canonical
      URI encoding instead of form-style query escaping, so prefixes and opaque
      continuation tokens containing spaces, literal `+`, `/`, `=`, or `@`
      characters are signed and transmitted consistently across AWS S3, MinIO,
      and strict compatible providers.
1248. DAV sync retention worker executions now persist
      `dav_sync_retention_runs` audit/read-model rows with cutoff, limit,
      dry-run/confirmation flags, completed/failed status, bounded error text,
      and CalDAV/CardDAV candidate/deleted counts. Partial failures after one
      DAV side prunes successfully are now traceable before Admin API
      readiness/history endpoints make retention publicly operable.
1249. The DAV sync retention repository now exposes bounded run-history reads:
      list by status and created-at window with a capped limit, and fetch one
      run by bounded ID. This gives Admin API readiness/history endpoints a
      stable operational read boundary over `dav_sync_retention_runs` without
      coupling controllers to worker execution internals.
1250. Admin API now exposes DAV sync retention run history through
      `GET /admin/v1/dav-sync/retention-runs` and
      `GET /admin/v1/dav-sync/retention-runs/{id}` with explicit JSON
      envelopes, bounded status/created-at filters, unknown-query rejection,
      OpenAPI coverage, and console capability advertising.
1251. Optional PostgreSQL integration coverage now applies the release
      migrations and round-trips DAV sync retention completed/failed run rows,
      sanitized failure text, bounded detail reads, and status/time-window list
      filters.
1252. Admin API now exposes a DAV sync retention readiness preview at
      `GET /admin/v1/dav-sync/retention-readiness`. The endpoint performs
      dry-run CalDAV/CardDAV retention probes only, rejects future cutoffs and
      unknown query controls, caps the per-backend probe limit at 10000, and
      returns aggregate plus backend-specific candidate counts with
      truncation/readiness flags before any destructive policy is made public.
1253. Admin API can now execute DAV sync retention runs at
      `POST /admin/v1/dav-sync/retention-runs`. Dry-run calls persist bounded
      candidate-count audit rows, while destructive calls require explicit
      `confirm_ready`, reuse the readiness preview, fail closed when the probe
      is truncated, and record completed or failed CalDAV/CardDAV prune counts
      in the same retention run read model exposed to operators.
1254. Admin Console capabilities now include a redacted storage backend profile
      for local/NFS, MinIO, and AWS S3-compatible deployments. Operators can
      inspect normalized active labels, supported object primitives,
      local-vs-S3-compatible class, path-style addressing, and sanitized S3
      endpoint/bucket/prefix/region fields without exposing credentials or
      local filesystem roots.
1255. CalDAV `DELETE` now shares the default authenticated user resolver
      fallback used by object reads/writes and WebDAV discovery/report methods,
      preventing manually assembled handlers from panicking on a nil resolver
      while preserving fail-closed unauthorized responses.
1256. Storage configuration now accepts `GOGOMAIL_STORAGE_BACKEND=nfs` as an
      explicit alias for the local filesystem adapter. Runtime Drive storage
      wiring registers `local` and `nfs` as bidirectional compatibility labels,
      and Admin capabilities/OpenAPI/docs expose the alias without changing
      object-key semantics or leaking local root paths.
1257. CalDAV and CardDAV delegated access policies now verify that resolved
      owner and actor principals are `user` principals before role checks or
      audit insertion. Non-user Directory principals fail closed at the DAV
      policy boundary, preserving future organization/group/resource semantics
      instead of leaking them into personal calendar/address-book storage.
1258. Delegated CalDAV and CardDAV `PROPFIND` discovery now keeps
      `DAV:current-user-principal` anchored to the authenticated actor while
      resource hrefs, `DAV:owner`, and repository lookups remain owner-scoped.
      This preserves WebDAV identity semantics for native clients and avoids
      confusing delegated access with account impersonation.
1259. CalDAV and CardDAV mutation paths now enqueue transactional `dav.event`
      outbox rows whenever they append durable sync-change rows. CalDAV emits
      `calendar.changed` and CardDAV emits `contacts.changed` v1 payloads with
      DAV kind, action, user, collection, object, ETag, sync token, and changed
      timestamp fields, giving Notification & Sync, search indexing, reminders,
      and mobile delta fan-out a clean asynchronous boundary.
1260. `event-worker` now registers DAV change audit handlers for
      `calendar.changed` and `contacts.changed`. Deployments can point a worker
      instance at `GOGOMAIL_EVENT_STREAM=dav.event` to validate DAV event
      payloads and persist audit rows before public reminder, push, indexing,
      or mobile sync consumers are enabled.
1261. IMAP `FETCH` and `UID FETCH` now treat empty `HEADER.FIELDS ()` and
      `HEADER.FIELDS.NOT ()` field lists as RFC-valid header-section requests.
      Empty include lists return only the header terminator, and empty exclude
      lists return the full header block instead of falling through without the
      requested literal.
1262. IMAP `FETCH` and `UID FETCH` now apply the same empty
      `HEADER.FIELDS ()` and `HEADER.FIELDS.NOT ()` semantics to
      `message/rfc822` MIME-part sections. Nested forwarded-message requests
      such as `BODY[1.HEADER.FIELDS ()]` and
      `BODY[2.HEADER.FIELDS.NOT ()]` now return RFC-shaped literals instead of
      being skipped by the MIME-part header-field parser.
1263. IMAP IDLE unexpected-command recovery is now regression-covered. A
      non-`DONE` line during IDLE continuation mode returns the pending IDLE
      tag as `BAD`, exits idle state, and keeps the authenticated session
      usable for the next legal command.
1264. IMAP `FETCH` and `UID FETCH` now have regression coverage for
      partial-window empty top-level header-field-list requests. Preview-style
      requests such as `BODY.PEEK[HEADER.FIELDS ()]<0.1>` and
      `BODY.PEEK[HEADER.FIELDS.NOT ()]<0.10>` continue to return bounded
      RFC-shaped literals.
1265. S3-compatible runtime storage wiring now supports private MinIO/S3 TLS
      trust through `GOGOMAIL_STORAGE_S3_CA_CERT_FILE`, validates PEM CA input,
      injects a dedicated TLS 1.2+ HTTP client into the existing S3 adapter, and
      rejects `GOGOMAIL_STORAGE_S3_INSECURE_SKIP_VERIFY=true` in production.
1266. Optional S3-compatible integration coverage now accepts
      `GOGOMAIL_TEST_S3_CA_CERT_FILE` and
      `GOGOMAIL_TEST_S3_INSECURE_SKIP_VERIFY`, letting release smoke tests
      verify private CA or local self-signed MinIO/S3 endpoints through the same
      injected HTTP-client path.
1267. Runtime startup now accepts `--config=<path>` for a validated flat YAML
      overlay on top of defaults/env values. The example config carries
      local/NFS/S3-compatible storage knobs, making backend flips reviewable as
      config-file changes without bypassing startup validation.
1268. The `gogomail` CLI startup path now has focused regression coverage for
      `--config` handoff and fail-fast invalid config/mode behavior, so config
      file support is guarded at the binary boundary instead of only the parser
      boundary.
1269. IMAP nested `message/rfc822` header-field partial fetches now have
      regression coverage for forwarded-message previews, including non-empty
      `HEADER.FIELDS`, empty `HEADER.FIELDS`, and empty `HEADER.FIELDS.NOT`
      windows on attached messages.
1270. IMAP `SEARCH HEADER` now validates RFC-shaped header field names before
      authentication or selected-mailbox state, rejecting empty, space-bearing,
      colon-suffixed, or IMAP atom-special-bearing field-name arguments as
      malformed criteria. `HEADER.FIELDS` filtering also matches raw RFC 5322
      field names without trimming malformed whitespace before the colon.
1271. CalDAV object reads, object writes, object deletes, and collection
      precondition checks now reject repeated `If-Modified-Since` or
      `If-Unmodified-Since` headers before storage work, keeping date-based
      conditional requests deterministic across native clients and
      intermediaries.
1272. CalDAV mutating repository requests now carry optional actor user IDs
      alongside owner user IDs, and `calendar.changed` v1 `dav.event` payloads
      plus DAV audit logs preserve `owner_user_id`, `actor_user_id`, and
      `delegated` context so delegated writes/deletes remain auditable and
      usable by future Notification & Sync, reminder, search, and mobile delta
      consumers.
1273. CalDAV object `DELETE` now carries matched strong `If-Match` ETags into
      `DeleteObjectRequest` and revalidates them inside the repository
      transaction before soft deletion, aligning delete concurrency semantics
      with the observed-ETag guard already used by `PUT`.
1274. CalDAV principal discovery now enriches RFC 4791
      `calendar-user-address-set` with active user-targeted Directory aliases
      in addition to the Directory primary email, normalizing and deduplicating
      `mailto:` hrefs while filtering non-user/resource/group aliases until
      scheduling and resource-booking policy is explicitly implemented.
1275. IMAP search-key syntax validation now rejects unknown or unsupported
      search-key atoms such as vendor-specific `X-GM-RAW` probes before
      authentication or selected-mailbox state across `SEARCH`, `UID SEARCH`,
      `SORT`, and `THREAD` embedded criteria.
1276. CardDAV contact-object reads, writes, deletes, and address-book
      collection precondition checks now reject repeated `If-Modified-Since`
      or `If-Unmodified-Since` headers before storage/body work, matching the
      CalDAV date-conditional fail-closed boundary and avoiding ambiguous
      timestamp guards through clients or intermediaries.
1277. CardDAV principal discovery now uses an explicit Directory-to-CardDAV
      conversion guard that accepts only Directory user principals, keeping
      organization, group, and resource principals out of user-owned
      address-book discovery until their CardDAV semantics are deliberately
      designed.
1278. IMAP `SEARCH`, `SORT`, and `THREAD` now return RFC-shaped
      `[BADCHARSET (US-ASCII UTF-8)]` diagnostics for unsupported charset
      probes before authentication or selected-mailbox state checks, keeping
      charset fallback behavior consistent during client capability probing.
1279. IMAP `STATUS` and advertised `LIST-STATUS` now report duplicated status
      data items with explicit duplicate diagnostics instead of folding them
      into unsupported-item failures, including before authentication checks.
1280. Validated storage config overlays now cover local filesystem, explicit
      NFS, local MinIO, and AWS S3-style profiles under `configs/storage.*.yaml`,
      with config-loader and CLI handoff tests proving each profile parses and
      passes startup validation as a reviewed `--config` starting point.
1281. IMAP `LSUB` now rejects LIST-EXTENDED-style option probes such as
      `(SPECIAL-USE)` prefixes or `RETURN (...)` tails before authentication
      with a dedicated `LSUB` tagged `BAD`, keeping RFC 3501 subscribed-mailbox
      discovery separate from advertised extended `LIST` behavior.
1282. IMAP `LIST` and `LSUB` now normalize mailbox patterns beginning with the
      hierarchy delimiter as root-absolute selectors before matching the
      server's root-relative mailbox names, so absolute probes are not
      accidentally matched against impossible leading-slash mailbox names.
1283. CardDAV mutating repository requests now carry optional actor user IDs
      alongside owner user IDs, and `contacts.changed` v1 `dav.event` payloads
      preserve `owner_user_id`, `actor_user_id`, and `delegated` context for
      address-book and contact-object changes, matching the CalDAV event
      boundary for future Contacts, Notification & Sync, search, audit, and
      mobile delta consumers.
1284. IMAP `LIST` reference names that begin with the hierarchy delimiter now
      normalize to the same root-relative namespace as absolute mailbox
      patterns before joining relative patterns, so root-style namespace
      probes such as `LIST "/Projects" "2026"` discover `Projects/2026`
      instead of an impossible leading-slash path.
1285. IMAP `LIST`/`LSUB` mailbox-pattern matching now prepares the decoded
      wildcard matcher once per command and reuses it across mailbox rows and
      subscribed-parent inference, reducing repeated regular-expression
      construction during large mailbox hierarchy discovery.
1286. YAML config overlays now accept `storage_root` as the file-level alias
      for the local/NFS object root, matching the existing
      `GOGOMAIL_STORAGE_ROOT` environment alias while keeping `mailstore_root`
      backward-compatible. The validated storage profile overlays use the
      storage-focused key so config-only local/NFS flips are clearer.
1287. IMAP UIDPLUS `COPYUID` generation now uses explicit `CopyMessageResult`
      source-UID to destination-summary mappings through the gateway, service,
      and PostgreSQL repository boundary. Sparse UID copy/move probes are
      regression-covered so nonexistent UID members are ignored without
      contaminating response codes, and alternate backends no longer have to
      rely on implicit result ordering to preserve COPYUID semantics.
1288. IMAP `MOVE`/`UID MOVE` UIDPLUS response generation now builds source UID
      sets from `MoveMessageResult.Source` rather than the originally requested
      UID slice, keeping MOVE's already explicit source/destination contract
      aligned with COPY and preserving correct `COPYUID` semantics when missing
      UID members are ignored.
1289. IMAP UID sequence-set response rendering now compacts contiguous
      ascending runs into RFC sequence-set ranges, reducing bulk UIDPLUS,
      ESEARCH, and SEARCHRES response size while preserving non-contiguous
      response order.
1290. IMAP destination mailbox metadata now exposes `UIDNotSticky`, and
      `COPY`/`UID COPY`/`MOVE`/`UID MOVE` suppress UIDPLUS `COPYUID` response
      codes for non-sticky destination UID stores, matching RFC 4315 semantics
      instead of emitting meaningless UID mappings.
1291. IMAP `UID EXPUNGE` sparse and mixed UID-set behavior is now
      regression-covered at protocol and PostgreSQL boundaries, ensuring
      missing UIDs and existing unmarked messages are ignored while only
      existing `\Deleted` messages are expunged.
1292. IMAP saved SEARCHRES state now applies adjusted multi-`EXPUNGE`
      sequence-number semantics when removing expunged messages, preserving
      `$` search-result correctness after batch `EXPUNGE`, `UID EXPUNGE`, or
      MOVE-driven expunge responses.
1293. IMAP `APPEND` results now carry `UIDNotSticky`, allowing alternate
      backends to suppress UIDPLUS `APPENDUID` response codes for non-sticky
      UID stores even when append storage returns UID metadata. This aligns
      append UIDPLUS behavior with the existing COPY/MOVE `UIDNOTSTICKY`
      response-code boundary.
1294. IMAP SEARCHRES `$` reuse now passes the search-criterion validator as a
      bare sequence-set atom, so RFC 5182-style `SEARCH $` and
      `UID SEARCH $ ...` probes work after `SEARCH RETURN (SAVE)` instead of
      being rejected before execution.
1295. IMAP `MOVE` and `UID MOVE` now emit UIDPLUS `COPYUID` as an untagged
      `OK` before source `EXPUNGE` responses and leave the tagged completion
      as a plain success response. This follows RFC 6851's UIDPLUS ordering
      guidance so clients can process source-to-destination UID mappings before
      source sequence numbers are removed.
1296. IMAP SEARCHRES `$` reuse is now regression-covered through `SORT`,
      `UID SORT`, `THREAD`, and `UID THREAD`, ensuring the saved-result
      sequence-set extension flows through the full search-oriented command
      family, not just `SEARCH` and `FETCH`.
1297. Storage profile smoke tests now assert that the NFS YAML overlay carries
      its `storage_root` and explicit `local` compatibility label through both
      config-loader parsing and CLI `--config` handoff, strengthening the
      config-only local/NFS flip contract.
1298. IMAP CONDSTORE awareness from `STATUS HIGHESTMODSEQ` is now
      regression-covered through a following `SELECT` and `UID STORE`, proving
      MODSEQ-aware STORE echo responses survive mailbox selection after the
      initial status probe.
1299. IMAP `ENABLE CONDSTORE` issued after mailbox selection now emits the
      selected mailbox's `HIGHESTMODSEQ` or `NOMODSEQ` untagged OK response
      before tagged completion, matching RFC 7162 first-enabling-command
      semantics. Selected-session highest-mod-sequence state is retained from
      SELECT and refreshed by known APPEND/COPY/MOVE/STORE/event mutations.
1300. IMAP `CAPABILITY` now advertises RFC 5258 `LIST-EXTENDED` alongside
      RFC 5819 `LIST-STATUS`, matching the gateway's existing extended `LIST`
      selection/return option implementation and preventing standards-aware
      clients from treating `RETURN (STATUS ...)`, `RETURN (CHILDREN)`, or
      `SPECIAL-USE` list options as unadvertised behavior.
1301. IMAP repeated `ENABLE CONDSTORE` now honors RFC 7162's first-enabling
      boundary: if the session is already CONDSTORE-aware through
      `SELECT ... (CONDSTORE)` or `STATUS HIGHESTMODSEQ` followed by `SELECT`,
      the server returns `ENABLED CONDSTORE` and tagged completion without
      re-emitting the selected mailbox's `HIGHESTMODSEQ`/`NOMODSEQ` baseline.
1302. Mail API `limit` parsing now applies the documented default of 50 when
      the query parameter is omitted or empty, and regression tests verify both
      message listing response metadata and search-service dispatch. This
      closes an OpenAPI/runtime drift that previously passed `0` to list/search
      handlers despite the shared pagination contract.
1303. IMAP mailboxes selected with `NOMODSEQ` now reject mod-sequence-dependent
      operations before backend scan or mutation work: `FETCH`/`UID FETCH`
      `MODSEQ`/`CHANGEDSINCE`, `SEARCH`/`SORT`/`THREAD` `MODSEQ`, and
      `STORE`/`UID STORE` `UNCHANGEDSINCE`. This aligns non-persistent
      mod-sequence stores with RFC 7162 instead of returning synthetic
      `MODSEQ (0)` data or dispatching conditional flag mutations.
1304. Webmail capability discovery now advertises only runtime-backed
      `GET /api/v1/search` filters (`q`, `folder_id`, `from`, `subject`, and
      `has_attachment`) and locks the OpenAPI enum plus regression coverage to
      that list, preventing generated clients from sending unsupported
      `since`, `before`, `read`, or `starred` search parameters.
1305. Admin console capability discovery now pins `GET /console/capabilities`
      to the `/admin/v1` OpenAPI server at the operation level and has runtime
      coverage that `/api/v1/console/capabilities` is not registered, avoiding
      generated-client base-path ambiguity before admin frontend integration.
1306. Health probes now pin `GET /health/live` and `GET /health/ready` to the
      service-root OpenAPI server, while service info pins `GET /info` to the
      `/api/v1` server; runtime regressions reject wrong-base variants so
      operators and generated clients do not rely on undocumented probe URLs.
1307. CalDAV and CardDAV OPTIONS discovery now advertise `DAV:
      sync-collection` only when the runtime store implements the relevant
      sync change-log interface, preventing native clients from discovering a
      sync-token capability that a limited backend cannot actually serve.
1308. Admin storage capability support flags are now derived from normalized
      active backend labels instead of hard-coded `true` values, so local/NFS,
      MinIO, and AWS/S3-compatible deployments advertise only the storage-label
      families they can serve.
1309. Admin console capability discovery now documents both `X-Admin-Token`
      and bearer-token OpenAPI security alternatives and has runtime coverage
      that the bootstrap endpoint accepts each form while rejecting ambiguous
      mixed credentials.
1310. API usage export capability discovery now pins the Admin API server and
      `X-Admin-Token`/bearer-token OpenAPI security alternatives, with runtime
      coverage that the readiness bootstrap accepts each documented credential
      form and rejects ambiguous mixed credentials.
1311. Drive public share-link downloads now have full OpenAPI binary-response
      coverage for `HEAD`, full-body `200`, and byte-range `206` responses,
      including no-store/nosniff, range, content-disposition, content-length,
      and optional SHA-256 headers plus runtime coverage for portable `HEAD`
      metadata.
1312. IMAP `APPEND` now resolves destination mailbox names to canonical
      mailbox IDs before mutation dispatch and rejects appends to the currently
      `EXAMINE`-selected read-only mailbox without calling backend append,
      extending read-only selected-state protection to the append path while
      preserving syntax-before-state validation.
1313. Drive public share-link metadata and download operations now explicitly
      opt out of global bearer auth in OpenAPI, with drift coverage for
      resolve, `HEAD` download, and `GET` download so generated public-share
      clients match the unauthenticated runtime boundary.
1314. Admin readiness bootstrap operations now pin the Admin API server and
      `X-Admin-Token`/bearer-token OpenAPI security alternatives for API usage
      ledger retention readiness, DAV sync retention readiness, and API usage
      export handoff readiness, keeping generated operator clients off the
      public Mail API base for admin-only checks.
1315. IMAP `CLOSE` now clears the selected-session saved SEARCHRES `$` state
      while tearing down selected mailbox metadata, keeping RFC 5182 saved
      results scoped to the same mailbox-selection lifecycle as `SELECT`,
      `EXAMINE`, and `UNSELECT`.
1316. IMAP `DELETE` of the currently selected mailbox now clears saved
      SEARCHRES `$` state and closes the mailbox event subscription together
      with selected mailbox metadata, keeping mailbox-removal lifecycle
      behavior aligned with the rest of the selected-state teardown paths.
1317. IMAP `RENAME` now resolves the source mailbox wire name to the backend's
      canonical mailbox ID before mutation dispatch, aligning mailbox
      management with the canonical-ID boundaries already used by `DELETE`,
      `APPEND`, `COPY`, and `MOVE`.
1318. IMAP `ENABLE CONDSTORE` after a mailbox selection with no persistent
      mod-sequences now records selected `NOMODSEQ` state as well as emitting
      the untagged `[NOMODSEQ]` response, keeping later MODSEQ-dependent
      `FETCH`, search, sort/thread, and store commands behind the RFC 7162
      persistent-mod-sequence guard.
1319. IMAP `SELECT`/`EXAMINE` subscription setup now cancels a newly opened
      mailbox event subscription if response writing fails before the
      subscription is installed into connection state, avoiding leaked event
      listeners on broken-client or network-failure paths.
1320. IMAP selected-mailbox `RENAME` now tracks a backend-returned canonical
      mailbox ID and refreshes the mailbox event subscription to that ID while
      preserving same-selection SEARCHRES sequence results.
1320a. IMAP selected-mailbox `RENAME` now also refreshes selected
       `HIGHESTMODSEQ`/`NOMODSEQ` metadata from the backend-returned mailbox,
       preventing stale CONDSTORE state when a renamed mailbox reports no
       persistent mod-sequences.
1321. S3-compatible request construction now has regression coverage for
      automatic path-style addressing on HTTPS dotted buckets and
      localhost/IPv4/IPv6 endpoints, preserving AWS certificate compatibility
      and MinIO-style local behavior even when operators use the generic `s3`
      backend.
1322. Storage profile smoke coverage now verifies MinIO and AWS S3 profile
      region, bucket, prefix, and credential fields in addition to endpoint and
      path-style settings, preventing config-only backend flips from silently
      losing required object-storage settings.
1323. CLI `--config` storage profile handoff coverage now asserts the same
      MinIO and AWS S3 profile fields as direct config loading, keeping
      file-driven storage flips auditable across endpoint, region, bucket,
      prefix, credentials, and path-style behavior.
1323a. Storage backend compatibility labels are now validated as bounded safe
       extensible tokens and exposed through the Admin API as sorted,
       de-duplicated active labels rather than a closed OpenAPI enum; unknown
       labels remain non-activating until the support matrix recognizes them.
1324. IMAP mailbox event publishing now performs non-blocking delivery while
      holding the broker lock, eliminating the race where concurrent
      subscription cancellation could close a snapshotted channel before
      publish sent an `EXISTS`/`EXPUNGE` update.
1325. Mail API default `limit=50` regression coverage now spans message lists,
      thread lists, thread-message lists, active search, and draft search,
      keeping webmail pagination behavior contract-stable across flat,
      conversation, and compose-focused read models.
1326. CalDAV and CardDAV `REPORT` parsing now rejects duplicate `DAV:limit`
      elements and duplicate nested `DAV:nresults` elements, preventing
      ambiguous bounded query or sync pagination from reaching object and
      change-list repository scans.
1327. CalDAV and CardDAV object `GET`/`HEAD` now ignore `If-Modified-Since`
      whenever `If-None-Match` is present, preserving HTTP conditional
      precedence so stale timestamp validators cannot mask changed `.ics` or
      `.vcf` bodies behind a false `304 Not Modified`.
1328. IMAP authenticated `SELECT`/`EXAMINE` attempts now deselect the current
      mailbox before attempting the new selection, matching RFC 3501 selection
      lifecycle semantics so failed selections leave no stale selected mailbox
      for later selected-state commands.
1329. Admin storage capability OpenAPI now models `active_labels` as a
      non-empty unique safe-token list and `operations` as a unique primitive
      list, with runtime coverage pinning the default advertised storage
      operation set for generated admin consoles.
1330. Drive JSON mutation routes now have regression coverage for required
      `application/json` content type, unknown-field rejection, and
      trailing-token rejection before service dispatch, keeping Drive API
      payload semantics aligned with Mail/Admin JSON contracts.
1331. CalDAV and CardDAV `sync-collection` REPORT parsing now rejects duplicate
      `DAV:sync-token` and `DAV:sync-level` controls, preventing ambiguous sync
      anchors from silently changing snapshot or change-list semantics.
1332. Public Drive share-link path tokens now preserve exact bearer-credential
      semantics by rejecting URL-decoded surrounding whitespace, embedded
      whitespace, and non-printable ASCII before rate limiting, audit, or
      service dispatch.
1333. IMAP selected-state sequence-set commands now drain queued mailbox
      events before dispatch, ensuring live `EXISTS`/`EXPUNGE` updates shape
      `*`, range, search, and mutation resolution before backend work begins.
1334. CalDAV and CardDAV request paths and absolute REPORT hrefs now reject
      encoded path separators before URL decoding, preventing `%2F` or `%5C`
      from remapping principal, collection, or object boundaries.
1335. Public Drive shared-file downloads now reject malformed or unsatisfiable
      byte ranges with HTTP 416 and `Content-Range: bytes */<size>` before
      object opens, while OpenAPI pins the shared-download range-error header.
1336. Shared storage `DeletePrefix` now rejects truncated list pages that omit
      a continuation cursor before deleting listed objects, and S3-compatible
      tests verify cursor handoff across bounded cleanup pages.
1337. API usage ledger list, NDJSON export, and stats OpenAPI operations now
      pin `/admin/v1` plus admin-token/bearer auth alternatives, matching the
      runtime `adminAuth` boundary for generated operator clients.
1338. API usage daily and monthly aggregate OpenAPI operations now also pin
      `/admin/v1` and admin-token/bearer auth alternatives, matching their
      runtime admin-authenticated analytics routes.
1339. API usage export batch list/create/detail/export OpenAPI operations now
      pin `/admin/v1` and admin-token/bearer auth alternatives, matching their
      runtime admin-authenticated export routes.
1340. API usage export artifact list/create/detail/write/download/verification
      OpenAPI operations now pin `/admin/v1` and admin-token/bearer auth
      alternatives, matching their sensitive runtime admin-authenticated
      artifact routes.
1341. API usage export manifest digest/signature OpenAPI operations now pin
      `/admin/v1` and admin-token/bearer auth alternatives, matching their
      runtime admin-authenticated audit/export proof routes.
1342. Core queue stats, delivery route counters, and IMAP UID backfill OpenAPI
      operations now pin `/admin/v1` and admin-token/bearer auth alternatives,
      matching their runtime admin-authenticated diagnostics/repair routes.
1343. Tenant, domain, and user administration OpenAPI operations now pin
      `/admin/v1` and admin-token/bearer auth alternatives, matching their
      runtime admin-authenticated organization identity, domain policy, DNS,
      quota, and user lifecycle routes.
1344. Outbox event, audit log, Directory principal/alias/delegation/group
      membership, and SMTP backpressure OpenAPI operations now pin `/admin/v1`
      and admin-token/bearer auth alternatives, matching their runtime
      admin-authenticated forensics, identity, delegated-access, and
      flow-control routes.
1345. Quota pressure, attachment upload cleanup, Drive upload session, Drive
      node, Drive usage, and Drive cleanup failure OpenAPI operations now pin
      `/admin/v1` and admin-token/bearer auth alternatives, matching their
      runtime admin-authenticated storage/Drive routes across local, NFS,
      MinIO, and S3-compatible deployments.
1346. API usage ledger retention run and DAV sync retention run OpenAPI
      operations now pin `/admin/v1` and admin-token/bearer auth alternatives,
      matching their runtime admin-authenticated destructive/audited retention
      routes.
1347. Quota reconciliation, delivery attempt, exhausted delivery attempt, and
      push notification attempt/statistics OpenAPI operations now pin
      `/admin/v1` and admin-token/bearer auth alternatives, matching their
      runtime admin-authenticated observability and provider outcome routes.
1348. Suppression list, trusted relay, delivery route, DKIM key/DNS
      verification, and outbox retry OpenAPI operations now pin `/admin/v1`
      and admin-token/bearer auth alternatives, matching their runtime
      admin-authenticated outbound mail operations, relay trust, domain
      signing, and retry-control routes.
1349. OpenAPI contract tests now derive registered `/admin/v1` routes from
      `admin.go` and require every matching operation to pin `/admin/v1` plus
      admin-token/bearer auth alternatives, preventing future admin route
      additions from silently drifting to ambiguous generated-client contracts.
1350. IMAP `LIST-EXTENDED` now supports RFC 5258 `SUBSCRIBED` selection and
      `RETURN (SUBSCRIBED)`, routing `LIST (SUBSCRIBED) ...` through the
      subscribed mailbox store, emitting `\Subscribed` only when requested, and
      preserving `CHILDREN`, `SPECIAL-USE`, and `LIST-STATUS` combinations for
      standards-aware clients.
1351. IMAP `UID ESEARCH` now returns the same explicit tagged `BAD` diagnostic
      as direct `ESEARCH` when RFC 7377 `MULTISEARCH` is not advertised,
      keeping RFC 4731 `ESEARCH` capability support scoped to `SEARCH RETURN`
      and `UID SEARCH RETURN` semantics instead of falling through as an
      unknown UID command.
1352. S3-compatible storage `List` now filters provider-returned keys against
      the caller's logical gogomail prefix after stripping the configured
      bucket/storage prefix, and `DeletePrefix` coverage proves sibling prefix
      keys are not deleted if an S3-compatible provider returns an overly broad
      list page.
1353. IMAP LIST-STATUS now rejects duplicated `STATUS` return options before
      mailbox metadata lookup, so forms such as `RETURN (STATUS (...) CHILDREN
      STATUS (...))` cannot silently replace earlier requested status data with
      a later status list.
1354. Local/NFS storage now rejects symlinked intermediate path components
      across writes, reads, range reads, metadata probes, deletes, copies,
      moves, and prefix listings. The local filesystem adapter creates
      destination directories one segment at a time and verifies existing
      components with `Lstat`, so mounted deployments cannot follow
      host-specific symlink parents outside the configured object root.
1355. CalDAV and CardDAV object `PUT`/`DELETE` now pass observed strong ETags
      into repository mutation guards when `If-Match: *` succeeds, preserving
      existing-resource WebDAV semantics while rechecking the exact object
      looked up by the handler before durable write or delete mutation.
1356. S3-compatible storage now rejects percent-encoded path separators in
      configured prefixes, object-key requests, list prefixes, copy/move
      endpoints, and provider-returned list keys, keeping object identity and
      prefix isolation portable across AWS S3, MinIO, and compatible gateways
      with different URL-decoding behavior.
1357. Shared storage object path and prefix validation now rejects
      percent-encoded path separators before local/NFS or S3-compatible adapter
      use, preventing deployments from creating keys on one backend that depend
      on provider-specific decoding behavior after a configuration-only
      storage backend flip.
1358. IMAP RFC 5258 `LIST-EXTENDED` now rejects unparenthesized `RETURN`
      option lists before mailbox lookup, so forms such as
      `LIST "" * RETURN CHILDREN` cannot bypass the parenthesized return-option
      shape required for `CHILDREN`, `SPECIAL-USE`, `SUBSCRIBED`, and
      LIST-STATUS controls.
1359. IMAP `FETCH` and `UID FETCH` now support RFC 3501 `RFC822<offset.count>`
      partial full-message literals, preserve the `RFC822<offset>` response
      atom instead of normalizing it to `BODY[]`, and apply the same `\Seen`
      mutation semantics as full `RFC822` body fetches.
1360. Shared storage object path and prefix validation now rejects
      double-encoded path separators such as `%252F` and `%255C`, and
      S3-compatible storage now delegates separator checks to the shared
      validator so local/NFS, MinIO, AWS S3, and compatible gateways keep the
      same key-boundary semantics under proxy or provider double-decoding.
1361. CalDAV and CardDAV request paths and absolute REPORT hrefs now reject
      double-encoded path separators such as `%252F` and `%255C` before
      resource parsing, extending the direct `%2F`/`%5C` guard so principal,
      collection, and object identities cannot change segment shape after
      proxy or client double-decoding.
1362. IMAP `APPEND` now drains queued selected-mailbox events before append
      mutation responses, preserving command-order visibility for pending
      FLAGS/EXISTS/EXPUNGE updates and preventing them from being delayed until
      a later `NOOP` or selected mailbox command.
1363. CalDAV and CardDAV object `PUT` now reject `If-Unmodified-Since` for
      missing objects with HTTP 412 before reading request bodies, keeping
      WebDAV timestamp preconditions fail-closed when native clients intended
      to update an existing `.ics` or `.vcf` representation.
1364. S3-compatible `GetRange` now accepts safe `200 OK` full-range
      compatibility responses only when `Content-Range` matches the requested
      byte window or offset-zero `Content-Length` exactly equals the requested
      length, while draining and rejecting ambiguous range responses before
      exposing provider bodies.
1365. IMAP `FETCH` body-section parsing now enforces RFC-shaped `nz-number`
      semantics for MIME part paths and partial counts, rejecting leading-zero
      forms such as `BODY[01]`, `BODY[1.02.TEXT]`, and
      `BODY.PEEK[]<12.034>` before command execution.
1366. Local/NFS storage `Move` now falls back to the shared copy-delete path
      only when filesystem rename reports cross-device `EXDEV`, preserving the
      normal rename fast path while keeping object relocation portable across
      NFS or bind-mount style storage roots.
1367. IMAP `SEARCH HEADER` and `FETCH`
      `HEADER.FIELDS`/`HEADER.FIELDS.NOT` validation now accepts RFC
      5322-style visible custom field names containing `_`, `+`, or `.`, while
      continuing to reject empty, whitespace/control-bearing, colon-suffixed,
      or non-ASCII field names at the parser boundary.
1368. Shared `storage.DeletePrefix` now verifies every listed object remains
      under the requested canonical prefix before deletion, preserving completed
      progress and returning a structured out-of-scope listing error if a
      backend returns sibling keys during Drive or lifecycle cleanup scans.
1369. S3-compatible `List` now validates continuation tokens only on truncated
      pages and clears final-page cursors, so ignored or malformed
      `NextContinuationToken` values on `IsTruncated=false` provider responses
      do not break final-page listings.
1370. IMAP listener concurrency can now be bounded with
      `GOGOMAIL_IMAP_MAX_CONNECTIONS` or `imap_max_connections` in YAML. The
      protocol server holds one slot per active `ServeConn` session and sends
      an initial `BYE [ALERT]` to excess clients, keeping IMAP deployment
      goroutine growth operator-controlled without changing the default
      unlimited development behavior.
1371. Local/NFS storage `List` now returns a single-object final page when the
      requested prefix exactly names an object, aligning local filesystem
      semantics with S3 `Prefix` responses and extending the shared storage
      portability contract for config-only flips between local, NFS, MinIO,
      and AWS/S3-compatible backends.
1372. CalDAV and CardDAV object `DELETE` now honors `If-None-Match`
      preconditions before mutation, returning HTTP 412 for `*` or matching
      object ETags and keeping existing calendar/contact objects intact under
      WebDAV conditional delete probes.
1373. CalDAV and CardDAV collection `DELETE` now honors `If-None-Match`
      preconditions against calendar/address-book collection ETags before
      recursive deletion, preserving child `.ics` and `.vcf` members when
      native DAV clients send `*` or a matching collection validator.
1374. CalDAV and CardDAV collection `PROPPATCH` now has explicit
      `If-None-Match` regression coverage for matching ETags and `*`, proving
      metadata mutations fail with HTTP 412 before WebDAV XML bodies are read
      when native clients send a matching collection validator.
1375. CalDAV and CardDAV collection `DELETE`/`PROPPATCH` now pass observed
      collection ETags into repository mutation guards after conditional
      preflight, including successful `If-Match: *`, so stale
      calendar/address-book collection races are rechecked inside the storage
      transaction before recursive delete or metadata update state is
      committed.
1376. CalDAV `MKCALENDAR` and CardDAV extended `MKCOL` now evaluate collection
      creation preconditions before reading WebDAV XML request bodies:
      existing targets reject matching `If-None-Match` validators with HTTP
      412, missing targets reject `If-Match`/`If-Unmodified-Since`, and
      `If-None-Match: *` remains the create-only success path for absent
      collections.
1377. CalDAV/CardDAV collection creation now validates missing-target
      UUID-shaped collection path IDs before conditional create evaluation,
      preserving HTTP 400 syntax errors ahead of 412 state preconditions and
      XML body reads while keeping existing legacy collection IDs on their
      normal already-exists and conditional-response paths.
1378. IMAP partial fetch offsets now enforce RFC 3501 `number` syntax by
      rejecting leading-zero forms such as `BODY.PEEK[]<00.34>` and
      `<012.34>` before command execution, while preserving the valid
      zero-offset `<0.count>` window used by clients and capping offset/count
      values to the unsigned 32-bit IMAP `number` range.
1379. IMAP UID and message sequence-set number parsing now enforces RFC
      `nz-number` spelling, rejecting leading-zero values such as
      `FETCH 01 FLAGS` and `UID FETCH 1:02 FLAGS` before set expansion instead
      of normalizing them to `1` or `2`.
1380. IMAP `SEARCH` and `UID SEARCH` size criteria now enforce RFC 3501
      `number` spelling for `LARGER` and `SMALLER`, rejecting leading-zero
      values such as `SEARCH LARGER 020` before command execution while
      preserving valid zero-size searches and rejecting values above the
      unsigned 32-bit IMAP `number` range.
1381. IMAP CONDSTORE parsing now separates positive RFC `mod-sequence-value`
      inputs from zero-allowed `mod-sequence-valzer` inputs: `SEARCH MODSEQ 0`
      and `FETCH (CHANGEDSINCE 0)` are rejected, while
      `STORE (UNCHANGEDSINCE 0)` is carried through the gateway, service, and
      repository as a real conditional guard that returns `MODIFIED` instead
      of mutating flags unconditionally.
1382. S3-compatible `206 Partial Content` range responses now validate present
      `Content-Length` headers against the requested byte-window length,
      rejecting invalid or mismatched values after draining the body so
      provider metadata contradictions cannot masquerade as valid bounded
      range readers.
1383. S3-compatible `Content-Range` validation now rejects internal whitespace
      inside the `start-end/size` byte-range grammar, preventing malformed
      range metadata from being normalized before AWS S3, MinIO, or compatible
      gateway range reads are exposed to callers.
1384. S3-compatible `200 OK` range compatibility responses with a matching
      `Content-Range` now validate any present `Content-Length` against the
      requested byte window, keeping that compatibility path aligned with the
      ordinary `206 Partial Content` metadata checks before a bounded reader is
      exposed.
1385. S3-compatible `Content-Length` parsing now requires unsigned decimal
      digits for `HEAD` metadata and range-response validation, rejecting
      signed values such as `+5` instead of normalizing them into valid object
      sizes.
1386. S3-compatible `Content-Range` start, end, and total-size numbers now
      reuse the unsigned decimal parser, rejecting signed values such as
      `bytes +1-3/5` or `bytes 1-3/+5` before provider range metadata can be
      normalized.
1387. S3-compatible `ListObjectsV2` object-size parsing now also requires
      unsigned decimal digits, rejecting signed or whitespace-padded `<Size>`
      values such as `+5` or ` 5 ` before list metadata reaches cleanup,
      Drive, or reconciliation callers.
1388. S3-compatible `ListObjectsV2` object entries now reject missing or blank
      `<Key>` elements instead of silently skipping malformed provider entries
      before prefix mapping and cleanup scans.
1389. S3-compatible `ListObjectsV2` pagination control now requires an explicit
      canonical `<IsTruncated>true</IsTruncated>` or
      `<IsTruncated>false</IsTruncated>` value, rejecting missing or
      non-canonical forms before deciding whether a page is final.
1390. S3-compatible `CopyObject` success XML now accepts namespace-free or AWS
      S3 namespace `CopyObjectResult` roots only, rejecting same-local-name XML
      from unexpected namespaces before copy/move is reported successful.
1391. S3-compatible `ListObjectsV2` response XML now accepts namespace-free or
      AWS S3 namespace `ListBucketResult` roots only, rejecting
      same-local-name XML from unexpected namespaces before pagination, prefix
      filtering, cleanup, or Drive callers see listed object metadata.
1392. S3-compatible `ListObjectsV2` object `LastModified` metadata now rejects
      non-empty malformed or whitespace-padded timestamp values instead of
      silently exposing zero timestamps to cleanup, Drive, or reconciliation
      callers, while still allowing missing values from compatible providers.
1392a. S3-compatible `ListObjectsV2` object `LastModified` metadata now
       distinguishes omission from a present-but-blank element: missing remains
       compatible, while blank, malformed, or whitespace-padded values fail
       closed before list pages reach cleanup, Drive, or reconciliation
       callers.
1393. S3-compatible `ListObjectsV2` pagination controls now reject duplicate
      top-level `<IsTruncated>` or `<NextContinuationToken>` elements before
      XML unmarshalling can collapse ambiguous final/truncated state or cursor
      identity.
1394. S3-compatible `ListObjectsV2` object metadata now rejects duplicate
      per-object `<Key>`, `<Size>`, `<ETag>`, or `<LastModified>` elements
      before XML unmarshalling can collapse conflicting provider values into
      one listed object.
1395. S3-compatible `CopyObjectResult` success XML now rejects duplicate
      top-level `ETag` or `LastModified` metadata and nested `Error` elements
      before provider-side copy metadata can be collapsed into a successful
      copy/move result.
1396. IMAP command literal size framing now enforces RFC 3501 `number`
      spelling, preserving valid `{0}` literals while rejecting leading-zero
      forms such as `{00}`, `{001}`, and `{001+}`, plus signed or malformed
      forms such as `{+1}`, `{-1}`, and `{1++}`, with a tagged `BAD` framing
      response before reading literal bytes.
1397. S3-compatible `HEAD`/`Stat` metadata now rejects non-empty malformed
      `Last-Modified` headers instead of silently returning zero timestamps,
      while preserving HTTP optional-whitespace compatibility around otherwise
      valid timestamp values.
1398. S3-compatible `ListObjectsV2` and `CopyObjectResult` XML validation now
      applies the namespace boundary to core child elements as well as roots,
      so foreign-namespace pagination controls, object metadata, copy
      metadata, or embedded copy errors cannot be collapsed into canonical
      provider metadata by XML unmarshalling.
1399. S3-compatible `CopyObjectResult` `LastModified` metadata now rejects
      non-empty malformed or whitespace-padded timestamp values instead of
      accepting ambiguous successful copy metadata.
1400. S3-compatible `CopyObjectResult` `ETag` metadata now uses the same
      bounded safe single-line validation as `Stat` and `List`, rejecting
      malformed copy success metadata before copy/move callers treat the
      provider response as durable.
1400a. S3-compatible `CopyObjectResult` `LastModified` metadata now
       distinguishes omission from a present-but-blank element: missing remains
       compatible, while blank, malformed, or whitespace-padded values fail
       closed before copy/move callers treat the provider response as durable.
1401. IMAP `IDLE` now requires an exact case-insensitive `DONE` continuation
      token, rejecting leading/trailing whitespace variants as malformed
      termination instead of silently ending the idle state.
1402. IMAP `AUTHENTICATE PLAIN` SASL-IR initial responses now validate
      malformed PLAIN payloads before plaintext privacy policy checks,
      preserving syntax-before-policy diagnostics without authenticating before
      TLS.
1403. IMAP `AUTHENTICATE PLAIN` continuation cancellation now requires an
      exact `*` token, rejecting whitespace-padded cancellation attempts as
      malformed SASL responses while keeping the session usable.
1404. IMAP `SEARCH`/`UID SEARCH` size and MODSEQ numeric criteria now reject
      whitespace-padded numeric strings such as `LARGER " 20 "` or
      `MODSEQ " 20 "` instead of trimming them into valid number atoms.
1405. IMAP `SEARCH`/`UID SEARCH` date criteria now reject whitespace-padded
      date strings such as `SINCE " 05-May-2026 "` instead of trimming them
      into valid date atoms.
1406. IMAP UID and message sequence-set syntax now rejects whitespace-padded
      quoted or literal set strings such as `SEARCH " 1 "` or
      `UID SEARCH UID " 7 "` instead of trimming them into valid set atoms.
1407. IMAP `FETCH`/`UID FETCH` `HEADER.FIELDS` and `HEADER.FIELDS.NOT`
      parsing now rejects whitespace-only, padded, or collapsed field-list
      forms such as `HEADER.FIELDS ( )` while preserving exact empty-list
      `()` compatibility for clients that intentionally request an empty
      header projection.
1408. IMAP `FETCH`/`UID FETCH` `CHANGEDSINCE` and `STORE`/`UID STORE`
      `UNCHANGEDSINCE` modifier parsing now rejects whitespace-padded numeric
      atoms instead of trimming them into valid CONDSTORE thresholds.
1409. IMAP `SEARCH`/`UID SEARCH` `MODSEQ` entry-type parsing now rejects
      whitespace-padded `ALL`, `PRIV`, or `SHARED` atoms instead of trimming
      them into valid RFC 7162 entry-type controls.
1410. IMAP SEARCHRES `$` reuse now requires an exact `$` atom for sequence-set
      and UID-set helpers, rejecting whitespace-padded quoted or literal
      values instead of normalizing them into saved-result references.
1411. IMAP `STORE`/`UID STORE` mode atoms and `UNCHANGEDSINCE` markers now
      reject whitespace-padded quoted or literal values instead of trimming
      them into valid mutation controls.
1412. IMAP `STORE`/`UID STORE` flag-list parsing now rejects whitespace-padded
      quoted or literal list values such as ` (\\Seen) ` while preserving
      exact `()` and parenthesized flag-list semantics.
1413. IMAP APPEND/STORE flag-list parsing now rejects malformed inner list
      whitespace such as `( \\Seen)`, `(\\Seen )`, `(\\Seen  \\Flagged)`, or
      tab-separated flag names instead of collapsing them into valid flags.
1414. IMAP `STATUS` item-list parsing now rejects malformed inner whitespace
      such as `( UIDNEXT)` or `(UIDNEXT  RECENT)` instead of collapsing
      quoted/literal list values into valid status data items, while
      LIST-STATUS keeps its normalized return-option path regression-covered.
1415. IMAP `LIST RETURN` option-list parsing now rejects whitespace-padded
      quoted or literal list values such as `RETURN " (CHILDREN) "` instead
      of trimming them into valid parenthesized return controls.
1416. IMAP `SEARCH`, `SORT`, and `THREAD` charset parsing now rejects
      whitespace-padded quoted or literal atoms such as `CHARSET " UTF-8 "`,
      and `THREAD` algorithm parsing rejects padded `ORDEREDSUBJECT` values
      instead of trimming them into advertised control atoms.
1417. IMAP `SORT`/`UID SORT` criterion-list parsing now rejects leading,
      trailing, or nested parenthesized atom-list shapes such as `( DATE)`,
      `(DATE )`, and `((DATE))` before authentication or selected-mailbox
      state checks.
1418. IMAP `SELECT`/`EXAMINE` `CONDSTORE` select-param parsing now rejects
      whitespace-padded quoted or literal list values such as `" (CONDSTORE) "`
      instead of trimming them into valid RFC 4551 select parameters.
1419. IMAP `SEARCH RETURN (...)` and `SORT`/`THREAD` `RETURN (SAVE)`
      option-list parsing now rejects whitespace-padded quoted or literal list
      values such as `RETURN " (COUNT) "` or `RETURN " (SAVE) "` instead of
      trimming them into valid ESEARCH/SEARCHRES controls.
1420. IMAP RFC 5258 `LIST-EXTENDED` selection option-list parsing now consumes
      the full parenthesized option list and rejects whitespace-padded quoted
      or literal list values such as `" (SPECIAL-USE) "` instead of trimming
      them into valid selection controls.
1421. IMAP `FETCH`/`UID FETCH` data-item parsing now rejects whitespace-padded
      quoted or literal values such as `" (FLAGS) "` or `" FLAGS "` instead
      of trimming them into valid fetch attributes.
1422. S3-compatible `ListObjectsV2` object `ETag` metadata now fails closed
      when a non-empty provider value is malformed, line-bearing, empty after
      quote cleanup, or larger than the bounded metadata limit, instead of
      silently dropping suspect listed-object metadata.
1422a. S3-compatible `ListObjectsV2` object `ETag` metadata now distinguishes
       omission from a present-but-blank element: missing remains compatible,
       while blank or malformed values fail closed before list metadata reaches
       cleanup, Drive, or reconciliation callers.
1423. S3-compatible `Content-Length` parsing now rejects whitespace-padded
      values for `HEAD` metadata, `206 Partial Content` range validation, and
      safe `200 OK` full-range compatibility checks, keeping provider numeric
      metadata on the same exact unsigned decimal grammar as object sizes.
1424. IMAP UID and message sequence-set range parsing now rejects
      whitespace-bearing quoted or literal components such as `"1: 2"`,
      `"1 :2"`, or `"1, 2"` before authentication, selected-mailbox state,
      or set expansion, keeping RFC 3501 sequence-set grammar exact across
      command validation and execution helpers.
1425. IMAP `AUTHENTICATE PLAIN` now rejects whitespace-padded SASL response
      tokens, including quoted SASL-IR values such as `" <base64> "`, before
      privacy-policy or backend authentication checks while still preserving
      intentional spaces inside decoded SASL PLAIN credentials.
1426. S3-compatible `HEAD`/`Stat` now validates raw `Content-Length` metadata
      even when the HTTP response has already populated `ContentLength`,
      rejecting malformed or contradictory provider length metadata instead
      of trusting normalized transport state alone.
1427. S3-compatible full-object `GET` now validates present `Content-Length`
      headers with exact unsigned decimal grammar and wraps known-length
      successful bodies in a bounded reader, surfacing truncated provider
      bodies as `io.ErrUnexpectedEOF` instead of silent short reads.
1428. S3-compatible status-error diagnostics now parse standard S3 `<Error>`
      XML bodies into bounded one-line `Code: Message` previews with
      request-id context, keeping AWS/MinIO-compatible failures
      operator-friendly while preserving the existing sanitized plain-text
      fallback for non-XML provider errors.
1429. S3-compatible `ListObjectsV2` `200 OK` bodies now reject top-level
      standard S3 `<Error>` documents as embedded provider errors with the
      same bounded `Code: Message` and request-id diagnostics, preventing
      throttling or auth failures from degrading into generic invalid list
      control errors.
1430. S3-compatible `CopyObjectResult` bodies now format nested standard S3
      `<Error>` details with the same bounded `Code: Message` and request-id
      diagnostics used for top-level provider errors, so copy throttling or
      auth failures inside `200 OK` responses remain operator-readable while
      still failing closed before copy/move success is reported.
1431. S3-compatible standard `<Error>` diagnostics now also preserve bounded
      `HostId` context as `host-id=...` alongside `request-id=...` for
      top-level status errors, `ListObjectsV2` embedded errors, and nested
      `CopyObjectResult` errors, improving AWS supportability without exposing
      raw XML bodies.
1432. S3-compatible standard `<Error>` diagnostics now parse XML error fields
      through a streaming best-effort decoder, so bounded or truncated
      provider error bodies still surface parsed `Code`/`Message` context
      instead of falling back to raw XML snippets.
1433. S3-compatible standard `<Error>` diagnostics now cap each parsed XML
      error field before formatting, preventing oversized provider messages
      from creating large diagnostic strings while preserving useful
      `Code`/`Message`/request/host context.
1434. S3-compatible status-error diagnostics now suppress previews for
      standard `<Error>` XML bodies that contain no safe S3 error fields,
      preventing empty or extension-only provider errors from falling back to
      raw XML fragments.
1435. S3-compatible `CopyObject` embedded-error diagnostics now reuse the same
      capped streaming standard `<Error>` field parser for both top-level and
      nested copy errors, preventing oversized provider copy-failure messages
      from bypassing the status-error diagnostic bounds. The same standard
      `<Error>` preview path now also suppresses previews when
      safe same-local-name fields such as `Message` arrive from foreign XML
      namespaces, keeping status and embedded copy errors from collapsing
      ambiguous provider diagnostics into canonical S3 text.
1436. IMAP `FETCH` header-field section detection now requires either an exact
      top-level body section or a valid numeric MIME part path before
      `HEADER.FIELDS`/`HEADER.FIELDS.NOT`, preventing malformed section
      prefixes from being accepted merely because they contain header-subset
      marker text.
1437. CalDAV `OPTIONS` discovery now returns `Cache-Control: no-store` and
      `X-Content-Type-Options: nosniff`, matching CardDAV discovery behavior so
      native clients, proxies, and browser-like tooling do not retain stale DAV
      capability headers or infer content types from empty discovery responses.
1438. CalDAV and CardDAV unsupported-method discovery responses now also return
      `Cache-Control: no-store` and `X-Content-Type-Options: nosniff` alongside
      their implemented-method `Allow` headers, keeping native-client method
      probes aligned with the same cache and content-sniffing safety contract
      as `OPTIONS`.
1439. S3-compatible `Content-Length` handling now rejects duplicate provider
      headers for `HEAD`/`Stat`, full-object `GET`, `206 Partial Content`, and
      safe `200 OK` range-compatibility validation, preventing ambiguous object
      length metadata from being collapsed into a bounded reader or stored
      object metadata.
1440. S3-compatible `HEAD`/`Stat` metadata now rejects duplicate
      `Last-Modified` headers before timestamp parsing, preventing compatible
      provider timestamp ambiguity from being collapsed into a single object
      modification time.
1441. S3-compatible `HEAD`/`Stat` metadata now rejects duplicate `ETag`
      headers before object metadata is returned, preventing compatible
      provider identity ambiguity from being collapsed into a single ETag by
      HTTP header lookup.
1442. S3-compatible `HEAD`/`Stat` metadata now rejects duplicate
      `Content-Type` headers before exposing MIME metadata, preventing preview
      or download handling from depending on first-header collapse.
1443. IMAP `ENABLE` now has regression coverage for duplicate `CONDSTORE`
      capability probes, keeping the untagged `ENABLED` response singular even
      when clients retry the same capability atom with different casing.
1444. IMAP RFC 2971 `ID` now accepts the bare no-argument command form as an
      empty client parameter set, returning the normal server identity response
      while preserving strict `NIL` and parenthesized field/value-list
      validation.
1445. IMAP `LOGIN` now treats an empty quoted password as syntactically valid
      and routes it to backend authentication, returning
      `[AUTHENTICATIONFAILED]` for rejected credentials instead of classifying
      the command as malformed protocol input.
1446. S3-compatible `HEAD`/`Stat` metadata now rejects non-empty malformed
      `ETag` and `Content-Type` headers instead of silently dropping them,
      keeping object identity and MIME metadata fail-closed at the storage
      adapter boundary.
1447. S3-compatible range `GET` now rejects duplicate `Content-Range` headers
      for both `206 Partial Content` and safe `200 OK` compatibility paths, so
      byte-window identity is never selected by HTTP header collapse.
1448. IMAP RFC 5258 `LIST-EXTENDED` now accepts parenthesized mailbox pattern
      lists, applies `RETURN` options such as `STATUS` and `SUBSCRIBED` to the
      union of matching folders, and de-duplicates overlapping pattern results.
1449. S3-compatible `CopyObjectResult` success bodies now require a non-blank
      bounded `ETag`, preventing copy/move success from being reported when
      provider success metadata omits object identity.
1450. SMTP receive and submission listeners now accept optional non-negative
      connection caps via env/YAML config. Positive caps bound active session
      goroutines and reject overflow clients with a transient RFC-shaped
      `421 4.3.2` banner before close, while the default remains unlimited for
      small or externally rate-limited deployments.
1451. IMAP RFC 5258 `LIST-EXTENDED` parenthesized mailbox pattern lists now
      preserve quoted pattern strings containing spaces, letting clients match
      folders such as `"Archive 2026"` and request `RETURN (STATUS ...)`
      without the command parser splitting the mailbox pattern early.
1452. IMAP RFC 5258 `LIST-EXTENDED` pattern lists now also preserve mailbox
      patterns supplied as command literals immediately after `(`, so literal
      folder names containing spaces flow through the same matcher and
      `RETURN (STATUS ...)` response path as quoted pattern strings.
1453. IMAP parenthesized literal parsing now requires literal markers to be
      token-delimited after `(`, space, or tab, keeping embedded atom fragments
      such as `Archive{12}` rejected while preserving legal RFC 5258
      `LIST-EXTENDED` literal pattern lists.
1454. S3-compatible `PutObject` and `DeleteObject` successful status responses
      now reject top-level standard S3 `<Error>` bodies with bounded
      one-line diagnostics before reporting completed writes or cleanup,
      preventing compatible-provider throttling/auth/policy failures from
      crossing the shared storage contract as false success.
1455. IMAP `SEARCH`/`UID SEARCH` `KEYWORD` and `UNKEYWORD` criteria now reuse
      the common IMAP atom validator for RFC `flag-keyword` arguments,
      rejecting system flags such as `\Seen` and response-special atoms such
      as `bad]flag` before search evaluation.
1456. IMAP `SORT`/`UID SORT` criterion parsing now treats standard sort keys
      and `REVERSE` case-insensitively, so clients can send forms such as
      `SORT (reverse subject) UTF-8 ALL` while the strict parenthesized
      atom-list shape remains enforced.
1457. IMAP `UID` selected-state subcommands now drain queued mailbox events
      before execution, aligning UID-addressed workflows such as
      `UID FETCH *` with the existing pre-command `EXISTS`/`EXPUNGE`/`FLAGS`
      drain used by non-UID selected-state commands.
1458. S3-compatible `PutObject` success responses now validate optional
      `ETag` headers when providers send them, rejecting blank, duplicate, or
      malformed write identity metadata while preserving compatibility with
      providers that omit the header.
1459. S3-compatible `CopyObjectResult` success XML now rejects unknown
      top-level success children, keeping copy/move success metadata limited to
      canonical S3 fields instead of accepting provider-specific payload shape
      drift as durable object duplication.
1460. S3-compatible `PutObject` and `DeleteObject` success bodies now must be
      empty apart from whitespace unless they are rejected as standard S3
      embedded errors, preventing arbitrary provider success text or XML from
      crossing the shared storage contract as durable write/delete success.
1461. CardDAV future WebDAV `COPY` and `MOVE` method constants are now
      regression-covered as unadvertised in `OPTIONS` and 405 `Allow` headers
      until the handler implements full address-book object relocation and
      duplication semantics.
1462. CalDAV future WebDAV `COPY` and `MOVE` method constants are now
      regression-covered as unadvertised in `OPTIONS`, 405 `Allow`, and the
      shared implemented-method list until calendar object relocation and
      duplication semantics exist.
1463. S3-compatible `ListObjectsV2` object metadata and `CopyObjectResult`
      success metadata now reject nested child elements inside simple
      `Key`/`Size`/`ETag`/`LastModified` fields before XML unmarshalling can
      treat structured provider metadata as canonical strings.
1464. OpenAPI contract tests now derive registered Mail and Drive API routes
      from `mail.go` and `drive.go` and require every `/api/v1` operation to
      pin the Mail API server at the operation level, preventing generated
      clients from inheriting `/admin/v1` for user-facing routes.
1465. CardDAV `REPORT` now rejects `Depth: infinity` before parsing XML
      bodies, keeping `addressbook-query` traversal bounded to explicit
      `Depth: 0` or `Depth: 1` client semantics until native-client
      compatibility for broader traversal is proven.
1466. IMAP `COPY`/`UID COPY` and `UID MOVE` now have regression coverage for
      quoted and literal destination mailbox names with spaces, plus escaped
      quotes, preserving RFC 3501 string tokenization through canonical
      mailbox lookup and service-backed mutation requests.
1467. IMAP selected-state sequence-set and UID-set arguments now reject quoted
      or command-literal set values for FETCH/STORE/COPY/MOVE/UID mutation
      commands before state checks, keeping RFC set atoms distinct from IMAP
      string values while still allowing strings for mailbox names.
1468. IMAP command names and UID subcommand names now reject quoted-string or
      command-literal command words before dispatch, preserving RFC atom
      boundaries for executable verbs while keeping quoted/literal strings
      available where the grammar actually allows strings.
1469. IMAP command tags now reject quoted-string or command-literal values
      before tag recovery, returning untagged malformed-command responses for
      non-atom tag probes.
1470. IMAP SEARCH sequence-set criteria and UID SEARCH UID set operands now
      reject quoted-string or command-literal set values, preserving atom-only
      set semantics without weakening string operands for text/header search.
1471. IMAP SORT/UID SORT and THREAD/UID THREAD embedded search criteria now
      apply the same atom-only sequence-set boundary to quoted and
      literal-framed set operands before state checks.
1472. IMAP search-family numeric operands now reject quoted or literal-framed
      values for LARGER/SMALLER and MODSEQ thresholds/entry types, while
      preserving RFC string handling for MODSEQ entry names.
1473. IMAP search-family charset and date controls now reject quoted values,
      keeping CHARSET and date keys atom-only while preserving string-capable
      text/header search operands.
1474. IMAP KEYWORD and UNKEYWORD search operands now reject quoted
      flag-keyword values, preserving RFC atom semantics for flag names before
      authentication or selected-mailbox state.
1475. IMAP STORE and UID STORE mutation controls now reject quoted or
      command-literal flag update mode tokens and quoted UNCHANGEDSINCE
      markers before authentication or selected-mailbox mutation state.
1476. IMAP FETCH and UID FETCH data-item controls now reject exact quoted or
      command-literal fetch attribute atoms such as FLAGS before
      authentication or selected-mailbox state.
1477. IMAP ENABLE capability operands now reject quoted or command-literal
      CONDSTORE probes before authentication, preserving atom-only capability
      negotiation.
1478. IMAP AUTHENTICATE mechanism names and SASL-IR initial responses now
      reject quoted values before unsupported-mechanism, privacy, or backend
      authentication policy checks.
1479. IMAP parenthesized control lists now reject quoted or command-literal
      STORE/UID STORE flag-lists, APPEND flag-lists, and STATUS item-lists
      before authentication or mutation/status state.
1480. IMAP SELECT and EXAMINE optional CONDSTORE select parameters now reject
      quoted or command-literal parenthesized lists before authentication or
      mailbox selection state.
1481. IMAP LIST selection option-lists, RETURN introducers, and RETURN
      option-lists now reject quoted or command-literal controls while
      preserving RFC mailbox pattern-list operands.
1482. IMAP SEARCH, UID SEARCH, SORT, UID SORT, THREAD, and UID THREAD RETURN
      introducers and return option-lists now reject quoted or command-literal
      controls before state checks, preserving RFC atom/list boundaries for
      ESEARCH and SEARCHRES.
1483. IMAP SORT and UID SORT criterion lists now reject exact quoted or
      command-literal parenthesized lists before authentication or selected
      state, keeping RFC 5256 sort criteria as raw list controls.
1484. IMAP THREAD and UID THREAD algorithm controls now reject exact quoted or
      command-literal ORDEREDSUBJECT values before state checks, keeping
      advertised RFC 5256 thread algorithms atom-only.
1485. S3-compatible ListObjectsV2 standard metadata now shares the same
      namespace boundary as core list controls, preserving AWS metadata fields
      while rejecting foreign-namespace Prefix/Name/KeyCount/MaxKeys/
      StorageClass/Owner elements before unmarshalling.
1486. S3-compatible ListObjectsV2 simple standard metadata now rejects nested
      XML inside Prefix and StorageClass-style fields before unmarshalling,
      while preserving structured AWS metadata such as Owner.
1487. S3-compatible ListObjectsV2 mapped object keys now fail closed on
      leading/trailing whitespace or encoded separators after matching the
      configured storage prefix, instead of silently skipping corrupt provider
      keys.
1488. S3-compatible standard `<Error>` diagnostics now suppress previews when
      safe fields such as `Code`, `Message`, `RequestId`, or `HostId` are
      duplicated, preventing ambiguous provider XML from becoming concatenated
      status strings while still recognizing the response as structured S3
      error XML.
1489. S3-compatible standard `<Error>` diagnostics now also suppress previews
      when those safe fields contain nested XML, preserving structured-error
      recognition without flattening ambiguous provider-specific elements into
      operator-facing status strings.
1490. IMAP APPEND/STORE flag-list parsing now rejects duplicate canonical
      system flags such as `(\\Seen \\Seen)`, keeping mutation and append
      option lists set-shaped before backend writes or APPEND body handling.
1491. IMAP `SELECT` now canonicalizes backend-provided permanent flags before
      rendering `FLAGS`/`PERMANENTFLAGS` and before selected-state STORE
      permission checks. Duplicate, aliased, lower-case, or unknown backend
      metadata is collapsed into the supported RFC-shaped flag set, keeping
      client-visible mailbox metadata and mutation permissions aligned.
1492. S3-compatible `ListObjectsV2` root metadata now rejects duplicate simple
      standard elements such as `<KeyCount>` or `<Prefix>`, and validates
      `KeyCount` as an unsigned decimal that exactly matches the returned
      `<Contents>` count before pagination, cleanup, Drive, or reconciliation
      callers trust the provider page.
1493. IMAP `SEARCH` and `UID SEARCH` now evaluate `RECENT`, `NEW`, and `OLD`
      against per-message recentness instead of fixed empty/all results.
      `NEW` matches recent unseen messages, `OLD` matches non-recent messages,
      and legacy zero-value summaries remain old until a backend exposes
      session-specific recent state.
1494. IMAP custom keyword flags are now supported at the protocol-core
      boundary. Backend-provided permanent keyword atoms can be advertised in
      `FLAGS`/`PERMANENTFLAGS`, `FETCH FLAGS` renders canonical duplicate-free
      keywords, `SEARCH KEYWORD`/`UNKEYWORD` evaluates them, and `STORE`
      accepts custom keywords only when the selected mailbox permits them.
1495. PostgreSQL `maildb` now persists IMAP custom keyword flags in the
      protocol-specific `imap_keywords` JSONB flag array. `APPEND`, `STORE`,
      `COPY`, `MOVE`, `FETCH`, and `SEARCH` paths share the same canonical
      keyword boundary, preserving RFC-shaped IMAP state without coupling it
      to future product labels or webmail categories.
1496. S3-compatible `ListObjectsV2` root `MaxKeys` metadata now uses the same
      exact unsigned decimal boundary as object sizes and `KeyCount`, and is
      rejected when it undercounts raw `<Contents>`. This keeps Drive,
      lifecycle cleanup, and reconciliation callers from trusting impossible
      provider page shapes while preserving AWS-compatible default `MaxKeys`
      echoes.
1496a. S3-compatible `ListObjectsV2` root `KeyCount` and `MaxKeys` metadata
       now distinguish omission from present-but-blank elements: missing
       remains compatible, while blank values fail the same numeric boundary as
       malformed values before list pages reach Drive, lifecycle cleanup, or
       reconciliation callers.
1497. S3-compatible `ListObjectsV2` root `Prefix` metadata is now validated
      when providers return it: the echo must be nonblank and exactly match the
      signed provider prefix requested by gogomail, including configured
      storage prefixes, while omitted `Prefix` echoes remain compatible.
      Object-key prefix filtering remains the authoritative safety boundary.
1498. S3-compatible `ListObjectsV2` root `Name` metadata is now validated when
      providers return it: the echoed bucket name must be nonblank and match
      the configured bucket, so wrong-bucket or blank-bucket compatible-provider
      responses fail closed instead of looking like ordinary empty or
      successful list pages.
1499. S3-compatible `ListObjectsV2` root `EncodingType` metadata is now
      rejected when present, including blank elements, because gogomail does
      not request encoded-key list mode. This keeps URL-encoded provider keys
      from being treated as ordinary storage object paths before an explicit
      decoding contract exists.
1500. S3-compatible `ListObjectsV2` root `ContinuationToken` metadata is now
      validated when present: the echoed token must match an explicitly
      requested cursor exactly, and returned tokens are rejected when no
      request cursor was sent. This keeps page diagnostics and retry semantics
      aligned with the signed list request.
1501. S3-compatible `ListObjectsV2` delimiter grouping controls are now
      rejected. gogomail never requests delimiter-based grouped listing, so
      returned `Delimiter` elements, including blank elements, or
      `CommonPrefixes` responses fail closed instead of being treated as
      ordinary object pages by Drive, lifecycle cleanup, or reconciliation
      callers.
1502. S3-compatible `ListObjectsV2` root `StartAfter` metadata is now rejected
      when present, including blank elements. gogomail uses continuation-token
      pagination rather than start-after list mode, so provider responses
      cannot silently shift cursor semantics for Drive, lifecycle cleanup, or
      reconciliation callers.
1503. S3-compatible requester-pays success response headers are now rejected
      across the adapter. The current storage adapter does not request
      requester-pays mode, so billing-mode response metadata cannot cross the
      portable local/NFS, MinIO, AWS S3, and compatible-gateway boundary as if
      it were an ordinary object or list response.
1504. S3-compatible `ListObjectsV2` object `ChecksumType` metadata now shares
      the same namespace and simple-field nested-XML boundary as
      `ChecksumAlgorithm`, preserving compatibility with newer AWS S3 checksum
      list metadata without letting foreign-namespace or structured checksum
      fields cross the storage boundary.
1505. IMAP command-line framing now requires RFC CRLF endings for ordinary
      commands, literal suffix lines, `AUTHENTICATE PLAIN` SASL continuations,
      and `IDLE` continuations. LF-only input receives a tagged
      `BAD command line must end with CRLF` plus the existing framing-error
      `BYE` close before command handlers can normalize malformed line endings.
1506. S3-compatible `HEAD`/`Stat` metadata now distinguishes omitted optional
      headers from present-but-blank metadata for `Last-Modified`, `ETag`, and
      `Content-Type`: omitted remains compatible, while blank or malformed
      present values fail closed before timestamp, identity, or MIME metadata
      crosses the shared storage contract.
1507. IMAP atom and quoted-string parsing now rejects 8-bit non-ASCII bytes
      for unquoted atoms, quoted strings, parenthesized quoted controls,
      RFC 2971 ID quoted tokens, command tags, command names, and `UID`
      subcommand names, preserving RFC 3501's 7-bit atom/string boundary
      before malformed input can route as an unknown command or reach
      selected-state UID handling.
1508. IMAP quoted-string response rendering now replaces invalid UTF-8 and
      non-ASCII runes with `?`, keeping ENVELOPE, BODYSTRUCTURE, STATUS, LIST,
      and related quoted response strings 7-bit safe while gogomail does not
      advertise an IMAP UTF-8 response extension.
1508a. IMAP ENVELOPE subject, message-id, in-reply-to, and address display/
       mailbox/host nstrings now use the bounded UTF-8-safe metadata text path
       before response quoting, preventing oversized backend metadata from
       inflating FETCH responses. ENVELOPE address lists are also capped after
       placeholder filtering so abnormal recipient fan-out metadata cannot
       amplify FETCH responses, and malformed empty or incomplete address
       entries are dropped before they can render as stray `(NIL NIL NIL NIL)`
       or display-name-only tuples, or hide later valid addresses.
1509. S3-compatible `ListObjectsV2` object metadata now rejects duplicate
      single-value `StorageClass` and `ChecksumType` elements before XML
      unmarshalling can collapse provider ambiguity, while preserving repeated
      `ChecksumAlgorithm` compatibility for providers that report multiple
      checksum algorithms.
1510. S3-compatible `ListObjectsV2` object metadata now also rejects duplicate
      single-value structured `Owner` and `RestoreStatus` elements while still
      accepting one structured value from namespace-free or AWS S3 namespaced
      responses, keeping compatible-provider metadata deterministic without
      breaking repeated `ChecksumAlgorithm` responses. Nested children inside
      structured `Owner` and `RestoreStatus` metadata now also reject foreign
      XML namespaces, unknown child names, and deeper nested elements before
      unmarshalling can hide provider-specific shapes. Duplicate known child
      names inside those structured fields now fail closed as well.
1511. S3-compatible XML success metadata now rejects whitespace-padded
      `ListObjectsV2` object `ETag` and `CopyObjectResult` `ETag` values
      instead of trimming them into valid identity metadata, while preserving
      HTTP header optional-whitespace compatibility for `HEAD`/`Stat`.
1512. S3-compatible ETag parsing now rejects malformed quote nesting such as
      `""etag""` across optional `PutObject` success headers, `HEAD`/`Stat`
      metadata, `ListObjectsV2` object metadata, and `CopyObjectResult`
      success XML, while still accepting ordinary quoted and unquoted
      compatible-provider ETags.
1513. S3-compatible ETag parsing now also requires the unwrapped opaque value
      to be printable ASCII across optional `PutObject` success headers,
      `HEAD`/`Stat`, `ListObjectsV2`, and `CopyObjectResult`, rejecting
      non-ASCII, control, whitespace, or otherwise non-printable provider
      identity metadata before it crosses the shared storage boundary.
1514. S3-compatible `HEAD`/`Stat` content-type parsing now validates present
      provider metadata as an ASCII MIME media type with optional parameters,
      rejecting slashless, non-ASCII, control-bearing, or otherwise malformed
      values before they can become trusted shared storage object metadata.
1515. IMAP BODY/BODYSTRUCTURE rendering now validates MIME media type, subtype,
      parameter-list names, and transfer-encoding tokens against RFC
      2045-style token boundaries, falling back to conservative defaults for
      malformed tspecial/control-bearing source metadata, using pair-shaped
      type/subtype fallback, and suppressing empty parameter values before
      writing client-visible structure responses.
      Canonical duplicate parameter names are collapsed so malformed MIME
      source metadata cannot emit repeated `BODYSTRUCTURE` keys, and MIME
      parameter values are bounded at UTF-8 boundaries before rendering so
      oversized filenames or boundaries cannot inflate fetch responses.
1516. IMAP BODYSTRUCTURE disposition rendering now treats malformed disposition
      tokens as `NIL` instead of falling back to `ATTACHMENT`, preventing
      invalid MIME source metadata from inventing attachment semantics.
1517. IMAP BODYSTRUCTURE content ID and description nstrings are now trimmed
      and bounded at UTF-8 boundaries before response quoting, preventing
      oversized source MIME metadata from inflating fetch responses.
1518. S3-compatible `ListObjectsV2` structured object metadata now rejects
      direct text on `Owner` and `RestoreStatus` wrapper elements before XML
      unmarshalling can ignore ambiguous provider content, while preserving
      known namespace-free/AWS child elements for compatible-provider
      metadata.
1519. IMAP parenthesized literal parsing now requires literal values embedded
      in grouped control operands, including LIST pattern-lists, to remain
      printable ASCII before they are wrapped as quoted strings for later
      parsing. This prevents control-bearing or raw non-ASCII literal bytes
      from being normalized into a different mailbox pattern while preserving
      literal-framed modified UTF-7 mailbox names.
1520. S3-compatible requester-pays response metadata now treats blank or
      whitespace-only `x-amz-request-charged` values as invalid provider
      metadata before rejecting nonblank requester-pays mode as unsupported
      across adapter success paths.
1521. S3-compatible full-object `GET` now rejects `Content-Length` mismatches
      even when Go's normalized response length is known to be zero, keeping
      exact body-length identity aligned between raw provider headers and the
      shared bounded-reader contract.
1522. S3-compatible offset-zero `200 OK` range compatibility now applies the
      same raw-header versus normalized-length agreement when both metadata
      surfaces are known, so downgraded full-window range responses cannot
      contradict known zero-length metadata before a bounded reader is exposed.
1523. IMAP required mailbox targets now reject decoded empty names for
      selection, status, mailbox mutation/subscription, append, copy, and move
      commands before backend lookup or selected-state handling, while keeping
      empty `LIST`/`LSUB` reference and pattern strings available for RFC
      mailbox discovery semantics.
1524. S3-compatible `PutObject` success `ETag` headers and `HEAD`/`Stat`
      `ETag` metadata now reject leading or trailing whitespace padding before
      quote cleanup, aligning header metadata with the existing strict
      `ListObjectsV2` and `CopyObjectResult` ETag boundaries.
1525. S3-compatible `HEAD`/`Stat` `Content-Type` metadata now rejects leading
      or trailing whitespace padding before MIME parsing, so preview/download
      MIME decisions cannot depend on adapter-side normalization of provider
      metadata.
1526. IMAP mailbox mutation special handling now treats only exact
      case-insensitive `INBOX` as the RFC special namespace, preserving quoted
      mailbox names with real leading or trailing spaces such as `" INBOX "`
      as ordinary backend-bound folder names.
1527. IMAP subscription canonicalization and service/repository
      `SUBSCRIBE`/`UNSUBSCRIBE` delegation now preserve decoded mailbox-name
      leading and trailing spaces, so missing subscriptions such as
      `" INBOX "` remain distinct `LSUB` rows instead of collapsing into
      `INBOX`.
1528. IMAP live mailbox-event subscription now preserves decoded mailbox ID
      leading and trailing spaces after validation, keeping IDLE/NOOP event
      fan-out keyed to the exact selected mailbox identity instead of a
      trimmed variant.
1529. IMAP service-backed mailbox lookup for `SELECT`/`EXAMINE` now preserves
      decoded mailbox ID leading and trailing spaces after validation before
      repository delegation, preventing the protocol adapter from collapsing
      legitimate mailbox-name characters at the service boundary.
1530. PostgreSQL IMAP mailbox and APPEND-target lookup now separates exact
      decoded mailbox-name matching from compatibility aliases, keeping
      unpadded `INBOX` and slash-trimmed path compatibility while preventing
      names with real leading/trailing spaces from falling through to trimmed
      aliases before storage lookup.
1531. IMAP service-backed `APPEND` now preserves decoded destination mailbox
      ID leading and trailing spaces after validation before append-target
      resolution, aligning literal storage and UID assignment with the same
      mailbox identity boundary used by `SELECT`/`EXAMINE`.
1532. IMAP service-backed `FETCH` and message listing now preserve decoded
      mailbox ID leading and trailing spaces after validation before
      repository delegation, keeping read-side storage access aligned with the
      exact selected-mailbox identity.
1533. IMAP service-backed `STORE`, `COPY`, `MOVE`, and `EXPUNGE` now preserve
      decoded mailbox ID leading and trailing spaces after validation before
      repository delegation, while mutation event fan-out remains keyed to the
      repository-returned summary mailbox IDs.
1534. PostgreSQL IMAP UID/message operations now validate mailbox IDs for
      emptiness without trimming them before UUID-bound queries, so padded
      mailbox UUIDs fail closed instead of being silently promoted to canonical
      folder IDs across list, fetch, store, copy, move, expunge, append-store,
      backfill, mailbox state, and message UID assignment boundaries.
1535. Service-level IMAP UID backfill now preserves mailbox IDs after
      validation before repository delegation, aligning operator/bootstrap UID
      assignment with the same exact mailbox identity and audit semantics as
      client-visible IMAP paths.
1536. S3-compatible success responses with `x-amz-request-charged` metadata
       now classify whitespace-padded values as invalid provider metadata,
       keeping requester-pays detection exact before rejecting exact nonblank
       requester-pays mode as unsupported across adapter success paths.
1537. CalDAV webmail REST API now exposes JSON endpoints for calendar operations
       through `CalendarHandler` with `CalendarRepo` interface, implementing CRUD
       endpoints for calendars and calendar objects for webmail frontend integration.
       Supports `text/calendar` and `application/ics` content types, ETag-based
       conditional requests, and `user_id` query parameter authentication when
       `tokenManager` is nil. Comprehensive unit tests with `fakeCalendarRepo`
       provide coverage.
1538. CardDAV webmail REST API now exposes JSON endpoints for address book
       operations through `ContactHandler` with `ContactRepo` interface, implementing
       CRUD endpoints for address books and contacts for webmail frontend integration.
       Supports `text/vcard`, `application/vcard+xml`, and `text/x-vcard` content types,
       ETag-based conditional requests, and `user_id` query parameter authentication
        when `tokenManager` is nil. Comprehensive unit tests with `fakeContactRepo`
        provide coverage.
 1539. Quota alert system: Admin API now exposes quota alert threshold CRUD
         (`GET/POST /quota-alert-thresholds`, `GET/PATCH/DELETE
         /quota-alert-thresholds/{id}`) and quota alert history
         (`GET /quota-alerts`, `GET /quota-alerts/{id}`) endpoints.
         Migration 0068 creates `quota_alert_thresholds` and `quota_alerts`
         tables with scope (user/domain/company), configurable warning/critical
         ratios, and event tracking. `mail.quota_warning` event emission
         remains a future enhancement for async threshold-based alerting.
 1540. CalDAV now interprets calendar-query and free-busy-query time ranges in
         the calendar's configured timezone per RFC 7809 Section 5.3. The
         `calendar-timezone` property (RFC 7809 Section 5.2) is stored in the
         `caldav_calendars.timezone` column and returned in PROPFIND responses.
         `CalendarObjectMatchesTimeRange`, `eventOverlapsRange`,
         `todoOverlapsRange`, and `CalendarObjectBusyPeriods` now accept an
         optional `*time.Location` parameter, defaulting to UTC when nil.
         `calendarQueryResponses` and `freeBusyCalendar` in handler.go now look
         up the calendar's timezone and pass it to these functions, so DTSTART,
         DTEND, DUE, and recurrence expansion use the calendar's configured
         timezone instead of always UTC.
 1541. CalDAV recurrence expansion now covers broader RFC 5545 edge cases.
         Unit tests verify WEEKLY with BYDAY (e.g., `MO,WE,FR`), MONTHLY with
         BYMONTHDAY (e.g., day 15), YEARLY with BYMONTH (e.g., June/December),
         ordinal BYDAY patterns (`1SA` = first Saturday), INTERVAL modifiers
         (`INTERVAL=3`), and timezone-aware matching all expand and match
         correctly through `CalendarObjectMatchesTimeRange`.
 1542. CalDAV/CardDAV collection `PROPPATCH` now has WebDAV `If` header
         regression coverage for duplicate resource-tag closing delimiters.
         Inputs such as `</caldav/calendars/user-1/work/>> (...)` and
         `</carddav/addressbooks/user-1/personal/>> (...)` are rejected as
         malformed resource tags with HTTP 400 before reading the XML body, so
         collection `xml:lang` metadata cannot mutate through malformed
         preconditions.
 1543. POP3 UPDATE-phase delete commits now normalize pending message IDs
         before calling the shared bulk delete service boundary. Whitespace-only
         IDs are skipped, duplicates are collapsed in first-seen order, and
         successful commits still clear pending state, keeping QUIT-triggered
         deletion work idempotent and audit/storage side effects stable even if
         adapter-local pending state is retried or internally duplicated.
 1544. PostgreSQL-backed IMAP APPEND now backfills existing active mailbox
         messages that lack `imap_message_uid` rows inside the same locked
         mailbox-state transaction before assigning the appended message UID.
         This keeps STATUS `UIDNEXT`/`HIGHESTMODSEQ` prediction, APPENDUID,
         sequence numbers, and later LIST/FETCH ordering on one monotonic UID
         timeline when legacy or API-created messages are first observed by
         IMAP.
 1545. PostgreSQL-backed IMAP COPY now applies the same lazy UID ordering guard
         to the destination mailbox: existing active destination messages
         without `imap_message_uid` rows are backfilled under the locked
         destination state before copied-message UIDs are allocated. COPYUID,
         destination sequence numbers, STATUS prediction, and later LIST/FETCH
         results therefore remain monotonic even when the destination mailbox
         has legacy or API-created unassigned messages.
 1546. PostgreSQL-backed cross-mailbox IMAP MOVE now backfills existing active
         destination messages without `imap_message_uid` rows before assigning
         moved-message destination UIDs. MOVE destination UID/sequence results,
         source removal, STATUS prediction, and later LIST/FETCH ordering now
         remain on the same monotonic destination UID timeline under legacy or
         API-created unassigned destination messages.
 1547. PostgreSQL-backed same-mailbox IMAP MOVE now applies the lazy UID
         ordering guard inside its single-mailbox CTE: existing active
         unassigned messages are backfilled before the replacement moved
         message receives a fresh UID. After the original source UID is
         expunged, destination sequence numbers, STATUS prediction, LIST/FETCH
         order, and final UIDNEXT/HIGHESTMODSEQ remain monotonic.
 1548. IMAP COPY and same-mailbox MOVE lazy UID backfill is now gated on
         actual source rows, so all-missing UID no-op commands do not assign
         legacy unassigned message UIDs or advance stored UIDNEXT/HIGHESTMODSEQ
         merely as a side effect of checking destination mailbox state.
 1549. IMAP lazy UID allocation now preflights UID capacity for APPEND, COPY,
         same-mailbox MOVE, and cross-mailbox MOVE destination backfill by
         combining existing unassigned-message backlog with requested new UID
         allocations before inserts. Near the 32-bit IMAP UID limit, these
         paths now fail with a stable exhaustion error before leaving partial
         UID rows or relying on database check-constraint failures.
 1550. IMAP lazy UID capacity checks now lock the target folder row together
         with the mailbox UID state row before counting unassigned messages.
         Because message inserts reference the folder row, API/receive/recovery
         writes are serialized with capacity preflight and destination backfill
         target selection, reducing race windows between backlog counting and
         UID allocation.
 1551. Operational IMAP UID backfill now follows the same mailbox UID mutation
         lock order as live APPEND/COPY/MOVE lazy allocation: mailbox UID
         state first, folder row second, and message rows last. This keeps
         manual repair/backfill jobs aligned with protocol write paths and
         avoids introducing a reversed lock order around folder/message scans.
 1552. Single-message lazy UID assignment through `EnsureIMAPMessageUID` now
         uses the same mailbox UID state then folder-row lock order before
         inspecting the target message, and the assignment CTE locks that
         message row before inserting a UID. It preserves existing UID lookups
         while preflighting capacity for new UID rows, returning stable
         exhaustion errors near the 32-bit IMAP UID limit instead of relying on
         database constraint failures.
 1553. Batch lazy UID assurance through `EnsureIMAPMessageUIDsForMessages`
         now assigns missing UIDs in mailbox order rather than caller request
         order: active targets are sorted by mailbox, internal date, and
         message ID before invoking the single-message assignment path. This
         keeps restored/exists event preparation aligned with operational
         backfill and LIST lazy assignment timelines.
 1554. Restored-message IMAP EXISTS events now carry exact mailbox message
         counts from the ensured UID sequence number. UID-based restore events
         therefore match APPEND/COPY summary events by publishing
         `Messages=SequenceNumber`, allowing selected sessions to jump to the
         correct EXISTS count instead of blindly incrementing local state.
 1555. Restored-message IMAP EXISTS events are coalesced by mailbox before
         publishing. Bulk restore operations keep only the highest ensured
         sequence count for each mailbox, reducing redundant EXISTS chatter
         while preserving one final count update per affected mailbox.
 1556. UID-based IMAP event publishing now skips entries whose mailbox ID is
         empty, matching the summary-event path. Restore/delete UID event
         helpers therefore avoid publishing malformed mailbox events if a
         repository result is incomplete or a legacy row cannot identify its
         mailbox.
 1557. The `mail.stored` IMAP notification path now publishes EXISTS events
         with `Messages=SequenceNumber` after UID assurance. New inbound
         delivery notifications therefore use the same exact mailbox-count
         update semantics as APPEND/COPY and restored-message events.
 1558. The `mail.stored` IMAP notification path now drops UID assurance
         results with an empty mailbox ID before publishing EXISTS or delta
         sync mailbox-change notifications, preventing malformed mailbox
         fanout from incomplete UID rows.
 1559. IMAP mailbox event broker subscriptions and published events now trim
         user and mailbox IDs before storing or fanout matching. Inputs with
         surrounding whitespace no longer pass validation only to miss
         subscribers because comparisons used the unnormalized values.
 1560. IMAP mailbox event broker publish now trims event types and rejects
         anything outside EXISTS, EXPUNGE, and FLAGS. Producer mistakes such
         as whitespace-wrapped or unknown event types are normalized or
         surfaced at the broker boundary instead of being silently ignored by
         selected sessions.
 1561. IMAP mailbox event broker slow-subscriber drops are now counted inside
         the broker and exposed through a safe aggregate accessor. Non-blocking
         fanout remains intact, but event-loss pressure is visible to tests
         and future operational diagnostics.
 1562. IMAP mailbox event broker slow-subscriber drops are also tracked per
         normalized user/mailbox identity. Diagnostics can now distinguish
         which selected mailbox is losing events instead of relying solely on
         an aggregate broker-wide counter.
 1563. IMAP mailbox event broker cancellation coverage now verifies a canceled
         publish context exits before fanout and before aggregate or
         per-mailbox slow-subscriber drop counters can change.
 1564. IMAP mailbox event broker subscription diagnostics now expose a safe
         subscriber count, and canceled-subscribe coverage verifies an already
         canceled context returns no channel/cancel function and leaves no
         subscriber behind.
 1565. IMAP mailbox event broker cancel idempotency is now covered explicitly:
         repeated subscription cancel calls leave subscriber accounting at zero
         and keep the subscription channel closed without panic.
 1566. IMAP mailbox event broker context-cancel idempotency is now covered:
         repeated context cancellation closes the subscription channel, drops
         subscriber accounting to zero, and remains stable when the explicit
         cancel function is called afterward.
 1567. IMAP mailbox event broker validation failures are now covered for
         side-effect safety: invalid publish attempts do not fan out events or
         update drop counters, and invalid subscribe attempts do not change
         subscriber accounting.
 1568. IMAP mailbox event broker diagnostics are now covered under concurrent
         publish, subscribe, and cancel activity. Aggregate/per-mailbox drop
         counters and subscriber-count reads are exercised while state is
         changing, with final subscriber accounting verified to converge to
         zero.
 1569. IMAP mailbox event broker diagnostics now have an explicit race-detector
         verification point: `go test -race -count=1 ./internal/imapgw` passes
         across the concurrent publish, subscribe, cancel, and diagnostic
         coverage.
 1570. IMAP server mailbox event filtering is now covered on both NOOP drain
         and IDLE live paths: other-user and other-mailbox events are consumed
         without wire responses, while the selected mailbox event still updates
         the client-visible EXISTS count.
 1571. IMAP server mailbox event unknown-type handling is now covered on both
         NOOP drain and IDLE live paths. A selected-mailbox event with an
         unsupported type is ignored without a wire response, and the following
         valid EXISTS event still drives the visible message count.
 1572. IMAP server stale EXISTS handling is now covered directly:
         `writeMailboxEvent` ignores EXISTS counts that are below or equal to
         the selected message count, producing no wire response and leaving
         `selectedMessages` unchanged.
 1573. IMAP server fresh EXISTS handling is now covered directly:
         `writeMailboxEvent` treats `Messages` above the selected count as an
         absolute mailbox count, emits `* N EXISTS`, and updates
         `selectedMessages` to that count rather than incrementing blindly.
 1574. IMAP server legacy EXISTS handling is now covered directly:
         `Messages=0` remains a compatibility signal to increment the selected
         count by one, with separate tests from the absolute-count EXISTS path.
 1575. IMAP server initial legacy EXISTS handling is now covered directly:
         when a selected mailbox starts at zero messages, a `Messages=0`
         legacy EXISTS event still emits `* 1 EXISTS` and updates
         `selectedMessages` to one.
 1576. IMAP server zero-sequence EXPUNGE handling is now covered directly:
         EXPUNGE events without a sequence number produce no wire response and
         leave selected message counts plus saved SEARCH sequence state
         unchanged.
 1577. IMAP server out-of-range EXPUNGE handling is now covered directly:
         sequence numbers above the selected message count are clamped to the
         selected count before emitting EXPUNGE, and saved SEARCH state is
         updated using the clamped sequence.
 1578. IMAP server empty-selected EXPUNGE handling now drops EXPUNGE events
         when `selectedMessages=0`, preventing invalid `* 1 EXPUNGE` responses
         for an empty selected mailbox while preserving the existing clamp path
         for non-empty mailboxes.
 1579. IMAP IDLE live-event coverage now verifies the empty-selected EXPUNGE
         guard end to end: an empty selected mailbox receives no EXPUNGE wire
         response for an injected EXPUNGE event, and DONE still completes
         normally afterward.
 1580. IMAP NOOP drain coverage now verifies the same empty-selected EXPUNGE
         guard end to end: an empty selected mailbox drains a queued EXPUNGE
         event without emitting EXPUNGE, and NOOP still completes normally.
 1581. IMAP empty-selected EXPUNGE handling now has a race-detector verification
         point: `go test -race -count=1 ./internal/imapgw` passes across the
         updated writeMailboxEvent logic and the IDLE/NOOP integration paths.
 1582. POP3 authentication policy freshness is now covered at the mailservice
         adapter boundary: each login re-invokes the authenticator, and a
         `must_change_password` policy change between logins is enforced on the
         next POP3 authentication attempt.
 1583. POP3 authentication identity freshness is now covered at the mailservice
         adapter boundary: a second login that resolves to a different user ID
         uses that fresh identity for folder loads, inbox page loads, and the
         maildrop lock key.
 1584. POP3 authentication identity normalization is now covered at the
         mailservice adapter boundary: whitespace around the authenticated user
         ID is trimmed before folder loads, inbox page loads, and maildrop lock
         key generation.
 1585. POP3 authentication now rejects empty authenticated identities at the
         adapter boundary: a user ID that becomes empty after trimming fails
         before folder or inbox page lookups can run.
 1586. POP3 authentication now rejects control-character authenticated
         identities at the adapter boundary: CR/LF-bearing user IDs fail before
         normalization, folder lookup, inbox page lookup, or maildrop lock key
         generation.
 1587. POP3 authenticated user ID validation is now centralized in a
         mailservice adapter helper, with direct coverage for trimming, empty
         identity rejection, and CR/LF rejection before mailbox construction.
 1588. POP3 credential validation is now centralized in mailservice adapter
         helpers, with direct coverage for username trimming, username
         empty/CR/LF rejection, and password CR/LF rejection.
 1589. POP3 username normalization is now covered at the authenticator boundary:
         whitespace-wrapped usernames are trimmed before `AuthenticatePlain`
         receives them, and the test authenticator records the exact value used.
 1590. POP3 password preservation is now covered at the authenticator boundary:
         surrounding spaces remain part of the password and are passed to
         `AuthenticatePlain` unchanged while CR/LF remains rejected separately.
 1591. POP3 invalid credential short-circuiting is now covered at the adapter
         boundary: empty usernames, CR/LF-bearing usernames, and CR/LF-bearing
         passwords fail before authenticator calls or mail service lookups.
 1592. POP3 must-change-password policy enforcement is now covered as a service
         short-circuit: blocked users authenticate once but fail before folder
         or inbox page lookup.
 1593. POP3 credential failure handling is now covered as a service
         short-circuit: wrong-password authenticator failures call the
         authenticator once and perform no folder or inbox page lookup.
 1594. POP3 INBOX folder detection is now covered as case-insensitive at the
         mailservice adapter boundary: `SystemType=INBOX` is accepted and
         message listing continues with the normalized user ID.
 1595. POP3 INBOX selection is now covered as first-match routing: when folder
         lists contain non-INBOX entries and multiple INBOX candidates, message
         page lookup uses the first matching INBOX folder ID.
 1596. POP3 missing-INBOX handling is now covered as a message-listing
         short-circuit: folder lookup can run, but no message page lookup occurs
         when no INBOX system folder is present.
 1597. POP3 folder listing failure handling is now covered as a message-listing
         short-circuit: folder errors propagate as `list folders` failures and
         prevent message page lookup.
 1598. POP3 message page failure handling is now covered at authentication
         time: INBOX page errors propagate as `list inbox messages` failures
         after a single lookup against the selected INBOX folder.
 1599. POP3 multi-page cursor decode failures are now covered at authentication
         time: malformed message IDs in a paged INBOX surface as
         `decode inbox cursor` errors and stop after the first page lookup.
 1600. POP3 multi-page missing-cursor handling is now hardened: `HasMore=true`
         pages without a `NextCursor` fail as `missing inbox cursor` after the
         first lookup, preventing repeated first-page reads.
 1601. POP3 multi-page cursor progression is now covered for large INBOX loads:
         a 450-message fixture performs exactly three page calls, with page 2
         and page 3 cursors based on the previous page's last message ID.
 1602. POP3 empty-INBOX pagination is now covered: authentication succeeds with
         a zero-message mailbox and performs exactly one zero-cursor page
         lookup.
 1603. POP3 message size conversion is now hardened at the adapter boundary:
         maildb `int64` sizes are normalized before POP3 `int` exposure, with
         negative sizes clamped to zero and oversized values clamped to max int.
 1604. POP3 message size normalization is now covered through the mailbox
         creation path: summary sizes of negative, zero, and positive values
         produce stable `MessageSize` outputs after Authenticate.
 1605. POP3 invalid message-size index handling is now covered: negative and
         out-of-range `MessageSize` lookups return zero instead of panicking or
         exposing invalid sizes.
 1606. POP3 invalid UIDL index handling is now covered: negative and
         out-of-range `MessageUIDL` lookups return an empty string instead of
         exposing arbitrary or stale message identifiers.
 1607. POP3 invalid content index handling is now covered: invalid
         `MessageContent` lookups return an empty string, while
         `MessageContentWithError` returns explicit errors for the same indexes.
 1608. POP3 deleted-message content access is now covered: after `MarkDeleted`,
         `MessageContent` returns an empty body and `MessageContentWithError`
         returns an explicit error.
 1609. POP3 reset behavior is now covered for content access: after
         `ResetDeleted`, a previously deleted message can be read again through
         `MessageContentWithError`.
 1610. POP3 delete commit cleanup is now covered: successful `CommitDeletes`
         clears pending delete state, and repeated commits do not issue
         duplicate bulk delete requests.
 1611. POP3 failed delete commits now have explicit preservation coverage:
         bulk delete errors leave pending delete IDs and deleted flags intact
         for retry/error handling.
 1612. POP3 reset-after-failed-commit behavior is now covered: `ResetDeleted`
         clears preserved pending delete state, and the next commit is a no-op
         instead of retrying an intentionally reset delete.
 1613. POP3 duplicate delete marking is now covered: repeated `MarkDeleted`
         calls for the same message create one pending delete ID and one bulk
         delete entry at commit time.
 1614. POP3 deleted-message UIDL visibility is now covered at the wire layer:
         after `DELE`, single-message UIDL fails and multi-line UIDL omits the
         deleted message while retaining non-deleted messages.
 1615. POP3 deleted-message LIST visibility is now covered at the wire layer:
         after `DELE`, single-message LIST fails and multi-line LIST omits the
         deleted message while retaining non-deleted messages.
 1616. POP3 deleted-message body visibility is now covered at the wire layer:
         after `DELE`, both `RETR` and `TOP` fail for the deleted message before
         content is exposed.
 1617. POP3 RSET restoration is now covered beyond STAT: after `DELE` then
         `RSET`, LIST, UIDL, and RETR all expose the message again at the wire
         layer.
 1618. POP3 failed QUIT commit rollback is now covered at the wire layer:
         after a delete commit failure, LIST, UIDL, and RETR expose the message
         again on the same connection.
 1619. POP3 successful QUIT close behavior is now covered: after a successful
         delete commit and `+OK`, the TCP connection closes instead of staying
         readable.
 1620. POP3 authorization-state QUIT close behavior is now covered: pre-login
         `QUIT` returns `+OK` and then closes the TCP connection.
 1621. POP3 connection-close tests now share a deadline-enabled connection
         helper so EOF-sensitive regressions avoid duplicated greeting and
         timeout setup.
 1622. POP3 STLS handshake failure behavior is now covered: invalid plaintext
         after `STLS` triggers the failure path and the server closes the TCP
         connection.
 1623. POP3 transaction-state STLS denial is now covered: after authentication,
         `STLS` returns a clear `-ERR` and the existing transaction session
         remains usable for `NOOP` and `STAT`.
 1624. POP3 unavailable-STLS auth-state denial is now covered: servers without
         TLS config reject `STLS` while keeping normal `USER/PASS` login and
         `STAT` usable afterward.
 1625. POP3 AUTH PLAIN cancellation now verifies authorization-state capability
         preservation: after `*`, CAPA still advertises USER and SASL before
         normal login continues.
 1626. POP3 AUTH LOGIN username cancellation now verifies authorization-state
         capability preservation: after `*`, CAPA still advertises USER and
         SASL before normal login continues.
 1627. POP3 AUTH LOGIN password cancellation now verifies authorization-state
         capability preservation: after password-step `*`, CAPA still advertises
         USER and SASL before normal login continues.
 1628. POP3 AUTH PLAIN invalid-base64 handling now verifies authorization-state
         capability preservation: parse errors leave CAPA and normal login
         usable afterward.
 1629. POP3 AUTH PLAIN invalid-format handling now verifies authorization-state
         capability preservation: malformed decoded credentials leave CAPA and
         normal login usable afterward.
 1630. POP3 AUTH LOGIN username invalid-base64 handling now verifies
         authorization-state capability preservation: username parse errors
         leave CAPA and normal login usable afterward.
 1631. POP3 AUTH LOGIN password invalid-base64 handling now verifies
         authorization-state capability preservation: password parse errors
         leave CAPA and normal login usable afterward.
 1632. POP3 auth capability regression tests now share an assertion helper for
         USER and SASL preservation, reducing duplicated CAPA checks across
         AUTH PLAIN and AUTH LOGIN error paths.
 1633. POP3 AUTH PLAIN challenge invalid-base64 handling now verifies
         authorization-state capability preservation: continuation parse errors
         leave CAPA and normal login usable afterward.
 1634. POP3 AUTH PLAIN challenge invalid-format handling now verifies
         authorization-state capability preservation: malformed continuation
         credentials leave CAPA and normal login usable afterward.
 1635. POP3 AUTH PLAIN challenge success is now covered: valid continuation
         credentials authenticate, auth-only CAPA entries disappear, and STAT
         works in transaction state.
 1636. POP3 AUTH PLAIN initial-response success is now covered: valid inline
         credentials authenticate, auth-only CAPA entries disappear, and STAT
         works in transaction state.
 1637. POP3 AUTH LOGIN success is now covered: valid username/password
         challenge credentials authenticate, auth-only CAPA entries disappear,
         and STAT works in transaction state.
 1638. POP3 AUTH LOGIN wrong-password handling now verifies authorization-state
         capability preservation: failed authentication leaves CAPA and normal
         login usable afterward.
 1639. POP3 AUTH PLAIN initial-response wrong-password handling now verifies
         authorization-state capability preservation: failed authentication
         leaves CAPA and normal login usable afterward.
 1640. POP3 AUTH PLAIN challenge wrong-password handling now verifies
         authorization-state capability preservation: failed continuation
         authentication leaves CAPA and normal login usable afterward.
 1641. POP3 auth success regressions now share an authenticated-state assertion
         helper for transaction CAPA and STAT checks across AUTH PLAIN and AUTH
         LOGIN success paths.
 1642. POP3 USER/PASS authentication success is now covered for capability
         transition: auth-only CAPA entries disappear and STAT works in
         transaction state.
 1643. POP3 USER/PASS wrong-password handling now verifies authorization-state
         capability preservation: failed authentication leaves CAPA and normal
         login usable afterward.
 1644. POP3 PASS-without-USER handling now verifies authorization-state
         capability preservation: command-order errors leave CAPA and normal
         login usable afterward.
 1645. POP3 USER replacement before PASS is now covered: a later USER command
         replaces the pending username, and PASS authenticates against the
         replacement before entering transaction state.
 1646. POP3 USER syntax-error handling now verifies authorization-state
         capability preservation: malformed USER commands leave CAPA and normal
         login usable afterward.
 1647. POP3 PASS syntax-error handling now verifies authorization-state
         capability preservation: malformed PASS commands leave CAPA intact and
         a later valid PASS still enters transaction state.
 1648. POP3 authorization-state unknown-command handling now verifies session
         recovery: unknown commands leave CAPA and normal login usable
         afterward.
 1649. POP3 transaction-state unknown-command handling now verifies session
         recovery: unknown commands leave NOOP and STAT usable afterward.
 1650. POP3 transaction-state empty-command handling now verifies session
         recovery: blank command lines return syntax error while NOOP and STAT
         remain usable afterward.
 1651. POP3 authorization-state empty-command handling now verifies session
         recovery: blank command lines return syntax error while CAPA and normal
         login remain usable afterward.
 1652. POP3 transaction-state USER/PASS denial is now covered: reauthentication
         attempts are rejected while NOOP and STAT remain usable afterward.
 1653. POP3 transaction-state AUTH denial is now covered: SASL reauthentication
         attempts are rejected while NOOP and STAT remain usable afterward.
 1654. POP3 transaction-state STLS denial coverage has been revalidated without
         duplicate tests: the existing regression checks the clear `-ERR`,
         `NOOP`, and `STAT` recovery behavior.
 1655. POP3 transaction-state CAPA stability is now covered: repeated CAPA
         calls keep core capabilities stable, keep auth-only entries hidden, and
         leave STAT usable afterward.
 1656. POP3 authorization-state CAPA stability is now covered: repeated CAPA
         calls keep auth capabilities stable and leave normal authentication
         usable afterward.
 1657. POP3 authorization-state NOOP stability is now covered: repeated NOOP
         calls keep auth capabilities and normal authentication usable
         afterward.
 1658. POP3 transaction-state NOOP stability is now covered: repeated NOOP
         calls keep LIST and STAT maildrop state usable afterward.
 1659. POP3 NOOP after DELE is now covered: NOOP preserves pending delete state,
         so LIST fails for the deleted message and STAT excludes it afterward.
 1660. POP3 CAPA after DELE is now covered: capability lookup preserves pending
         delete state, so LIST fails for the deleted message and STAT excludes
         it afterward.
 1661. POP3 unknown-command handling after DELE is now covered: command errors
         preserve pending delete state, so LIST fails for the deleted message
         and STAT excludes it afterward.
 1662. POP3 empty-command handling after DELE is now covered: parser errors
         preserve pending delete state, so LIST fails for the deleted message
         and STAT excludes it afterward.
 1663. POP3 transaction AUTH denial after DELE is now covered: reauthentication
         denials preserve pending delete state, so LIST fails for the deleted
         message and STAT excludes it afterward.
 1664. POP3 transaction USER/PASS denial after DELE is now covered:
         reauthentication denials preserve pending delete state, so LIST fails
         for the deleted message and STAT excludes it afterward.
 1665. POP3 transaction STLS denial after DELE is now covered: STLS denial
         preserves pending delete state, so LIST fails for the deleted message
         and STAT excludes it afterward.
 1666. POP3 DELE invalid-command sequence coverage has been audited across
         NOOP, CAPA, empty command, unknown command, AUTH, USER/PASS, and STLS
         paths without adding duplicate tests.
 1667. POP3 DELE invalid-command sequence tests now share a pending-delete
         visibility helper for LIST and STAT assertions across no-op, parser
         error, unknown command, reauthentication, and STLS denial paths.
 1668. POP3 RSET pending-delete clearing is now tied to a shared assertion:
         after DELE then RSET, LIST and STAT both prove the message is restored.
 1669. POP3 successful QUIT delete commit is now covered: after DELE, QUIT
         invokes CommitDeletes exactly once and preserves the committed delete
         mark.
 1670. POP3 no-delete QUIT now skips the delete commit hook: CommitDeletes is
         only called when the transaction has at least one pending delete.
 1671. POP3 no-delete QUIT close behavior is now covered: the commit-skip path
         still returns `+OK`, skips CommitDeletes, and closes the TCP
         connection.
 1672. POP3 QUIT after RSET now skips the delete commit hook: RSET clears the
         delete mark, QUIT returns `+OK`, and CommitDeletes is not called.
 1673. POP3 QUIT retry after failed delete commit is now covered: failed commit
         rollback clears the delete mark, and a later QUIT without another
         DELE skips CommitDeletes instead of retrying stale work.
 1674. POP3 QUIT re-delete retry after failed delete commit is now covered:
         after rollback, a fresh DELE in the same session causes the next QUIT
         to invoke CommitDeletes again and preserve the committed delete mark.
 1675. POP3 QUIT no-delete close after failed delete commit is now covered:
         after rollback, a later QUIT without another DELE returns `+OK`,
         skips CommitDeletes, and closes the TCP connection.
 1676. POP3 CAPA after failed delete commit is now covered: failed QUIT keeps
         the session in transaction state, keeps auth-only capabilities hidden,
         and exposes the restored maildrop count through STAT.
 1677. POP3 NOOP after failed delete commit is now covered: failed QUIT keeps
         the connection usable for NOOP, and STAT still reports the restored
         maildrop state afterward.
 1678. POP3 RSET after failed delete commit is now covered: failed QUIT
         rollback followed by RSET keeps the message visible and leaves the
         delete mark clear.
 1679. POP3 invalid DELE after failed delete commit is now covered: an
         out-of-range DELE after rollback returns `-ERR` without hiding the
         restored message or setting a delete mark.
 1680. POP3 RETR after failed delete commit is now covered: failed QUIT
         rollback allows retrieving the restored message body, keeps the
         delete mark clear, and preserves the later no-delete QUIT fast path.
 1681. POP3 TOP after failed delete commit is now covered: failed QUIT
         rollback allows retrieving the restored message header and requested
         body lines, keeps the delete mark clear, and preserves the later
         no-delete QUIT fast path.
 1682. POP3 UIDL after failed delete commit is now covered: failed QUIT
         rollback exposes the restored message UIDL, keeps the delete mark
         clear, and preserves the later no-delete QUIT fast path.
 1683. POP3 LIST after failed delete commit is now covered: failed QUIT
         rollback exposes the restored message size, keeps the delete mark
         clear, and preserves the later no-delete QUIT fast path.
 1684. POP3 STAT after failed delete commit is now covered: failed QUIT
         rollback reports the restored message count and size, keeps the
         delete mark clear, and preserves the later no-delete QUIT fast path.
 1685. POP3 multiline LIST after failed delete commit is now covered: failed
         QUIT rollback exposes all restored message sizes, keeps the delete
         mark clear, and preserves the later no-delete QUIT fast path.
 1686. SMTP inbound mixed-domain message size policy is now covered: receiver
         sessions aggregate enforcing recipient-domain policies, and DATA
         returns `552 5.3.4` when a later recipient domain has a stricter
         message size limit.
 1687. SMTP inbound domain policy lookup failure isolation is now covered:
         a later recipient lookup failure returns `451 4.7.1` without
         poisoning earlier accepted recipients, and DATA records only accepted
         recipients afterward.
 1688. SMTP inbound mixed-domain policy reset is now covered: a failed DATA
         caused by a later recipient domain's stricter size limit clears the
         accumulated policy state before the next MAIL transaction.
 1689. SMTP inbound RSET policy reset is now covered: explicit reset after
         mixed-domain RCPT state clears accumulated domain limits before the
         next MAIL transaction.
 1690. SMTP inbound MAIL policy reset is now covered: a new MAIL command after
         mixed-domain RCPT state clears accumulated domain limits before
         accepting the next recipient set.
 1691. SMTP inbound EHLO policy reset is now covered at the TCP protocol layer:
         repeated EHLO after mixed-domain RCPT state clears accumulated domain
         limits before accepting the next recipient set.
 1692. SMTP inbound HELO policy reset is now covered at the TCP protocol layer:
         HELO after mixed-domain RCPT state clears accumulated domain limits
         before accepting the next recipient set.
 1693. SMTP inbound QUIT policy isolation is now covered at the TCP protocol
         layer: mixed-domain RCPT state from a closed connection does not leak
         into a new connection's recipient-domain limits.
 1694. SMTP inbound DATA failure reset is now covered for domain policy and DSN
         state: a mixed-domain size failure clears accumulated limits and DSN
         metadata before the next accepted transaction.
 1695. SMTP inbound RSET reset is now covered for domain policy and DSN state:
         explicit reset clears accumulated limits and DSN metadata before the
         next accepted transaction.
 1696. SMTP inbound MAIL reset is now covered for domain policy and DSN state:
         a new MAIL command clears accumulated limits and DSN metadata before
         the next accepted transaction.
 1697. SMTP inbound EHLO reset is now covered for domain policy and DSN state
         at the TCP protocol layer: repeated EHLO clears accumulated limits and
         DSN metadata before the next accepted transaction.
 1698. SMTP inbound HELO reset is now covered for domain policy and DSN state
         at the TCP protocol layer: HELO clears accumulated limits and DSN
         metadata before the next accepted transaction.
 1699. SMTP inbound QUIT isolation is now covered for domain policy and DSN
         state at the TCP protocol layer: closed connections do not leak
         accumulated limits or DSN metadata into later connections.
 1700. SMTP inbound auth reset is now covered for domain policy and DSN state:
         Logout clears accumulated limits and DSN metadata, requires
         re-authentication, and prevents pre-Logout state from leaking.
 1701. IMAP single-message UID row locking is now audited: EnsureIMAPMessageUID
         locks the target messages row with `FOR UPDATE OF m`, and Postgres
         coverage verifies no stale UID row is inserted while that row is
         locked.
 1702. IMAP regular message move/delete stale UID rows are now covered:
         MoveMessage removes old mailbox UID rows before destination
         reassignment, and DeleteMessage removes assigned UID rows before
         rejecting later reallocation.

## Deferred until backend contracts stabilize

- Web Frontend modules: Next.js TypeScript apps for webmail, Drive UI,
  calendar UI, contacts UI, admin console, and shared inbox UI, using
  shadcn/ui and `DESIGN.md` with a Notion Mail-like product feel after the
  frontend start gate is explicitly opened.
- Mobile apps for mail, Drive, calendar, contacts, push, and offline sync.
- Desktop/power-user experience for keyboard-driven workflows, multi-pane
  productivity, bulk triage, advanced search, and drag/drop actions.
- Kafka
- OpenSearch as the default/mandatory search backend
- etcd
- Vault
- IMAP
- CalDAV public/client-ready compatibility, including broader recurrence edge
  cases (complete), native-client scheduling (iMIP/RFC 6047) (complete), attendee
  resolution via Directory + CardDAV (complete), production
  sync-token retention-age policy, and broader Apple/Android/Windows/macOS
  compatibility tests. ADR 0015 timezone support (RFC 7809) is now partially
  implemented: calendar-timezone property storage, PROPFIND/PROPPATCH/MKCALENDAR
  support, timezone service endpoint, and time-range interpretation are complete.
  `calendar-query` time-range response limits are applied after time-range
  matching so non-matching recent objects do not hide older matching events.
  `MKCALENDAR`/`PROPPATCH` now pass parsed calendar timezone/slug properties
  through to storage and return them in WebDAV multistatus responses.
- Directory/Identity expansion for delegated relationships, effective
  resource booking policy beyond the initial principal tables, resolver, alias
  lookup, bounded membership expansion, company-scoped delegation relationship
  checks, and bounded group-backed effective delegation reads
- Contacts/CardDAV broader vCard
  compatibility, and native-client compatibility beyond the experimental
  runtime, internal discovery/REPORT/object I/O, path/href, storage metadata,
  repository, bounded vCard 3.0/4.0 validation, REPORT parsing, and multistatus
  response boundaries. CardDAV `sync-collection` now uses a joined
  change+object read path for incremental responses and returns RFC-shaped
  truncation preconditions for over-limit deltas. `addressbook-multiget` now
  batches href object lookup while preserving WebDAV per-href response
  ordering and 404 semantics. CardDAV contact/address-book writes now reduce
  unnecessary `FOR UPDATE` contention, rely on active object unique indexes for
  UID/name conflicts, and retry bounded serialization/deadlock/lock-contention
  failures.
- Notification & Sync boundary for domain events, reminders, devices, quiet
  hours, per-device policy, and delta fan-out
- Vendor push notification delivery adapters

---

## Phase 2: Runtime Config Store & Settings Hierarchy

Target outcome:

> 설정 변경이 재배포나 스키마 마이그레이션 없이 모든 프로세스에 즉시 반영된다.
> 회사(Company) → 도메인(Domain) → 사용자(User) 3단 계층으로 설정이 상속·독립 운영된다.

### 2-A. Runtime Config Store (PostgreSQL JSONB + LISTEN/NOTIFY)

**스키마 설계**:

```sql
runtime_config (
  scope_type  TEXT,          -- 'global' | 'company' | 'domain' | 'user'
  scope_id    UUID,          -- NULL(global), company_id, domain_id, user_id
  key         TEXT,
  value       JSONB,
  locked      BOOLEAN DEFAULT false,  -- true이면 하위 스코프가 재정의 불가
  version     BIGINT  DEFAULT 0,
  updated_at  TIMESTAMPTZ DEFAULT now(),
  UNIQUE (scope_type, scope_id, key)
)
```

**회사 트리 구조 (companies 자기참조)**:

현재 `companies` 테이블은 flat. 자회사 지원을 위해 `parent_id` 컬럼 추가:

```sql
-- Migration: companies 자기참조 트리
ALTER TABLE companies
  ADD COLUMN parent_id uuid REFERENCES companies(id) ON DELETE SET NULL;

-- 기존 rows는 parent_id = NULL → 루트 회사로 유지
```

```
Root Company (그룹사, parent_id=NULL)
  ├── Sub-Company A (자회사, parent_id=root)
  │     ├── Domain A-1  ← Sub-Company A 소속
  │     └── Domain A-2
  ├── Sub-Company B (자회사, parent_id=root)
  │     └── Domain B-1
  └── Domain C          ← Root Company 직속 도메인 (자회사 없이)
```

- 트리 깊이 제한 없음. 실제 배포는 2-3 레벨이 일반적.
- `domains.company_id`는 그대로 — 도메인은 항상 특정 company에 직속.
- **도메인 중첩 금지**: `Domain → Sub-domain` 계층은 허용하지 않는다. 도메인은 항상 리프 노드.
  - 허용: `Company → Domain` (직속 또는 자회사 소속)
  - 금지: `Domain → Domain` (도메인 하위에 도메인)
- `organizations` (부서 트리)는 domain 내부 계층이므로 별도 유지.

**설정 상속 모델 — Copy-on-create + 독립 운영**:

```
Root Company  [설정 A=X, locked=false] [설정 B=Y, locked=true]
     │
     ├── Sub-Company A  → 생성 시 Root 설정 복사
     │     설정 A=X (재정의 가능)  설정 B=Y (locked, 재정의 불가)
     │     │
     │     ├── Domain A-1 → 생성 시 Sub-Company A 설정 복사, 독립 운영
     │     │      └── User → 개인 재정의 (domain locked 아닌 키만)
     │     └── Domain A-2 → 독립 운영
     │
     └── Domain C → 생성 시 Root Company 설정 복사, 독립 운영
```

- **생성 시 복사**: 자회사/도메인 생성 시 **직속 부모**의 현재 설정이 자동 복사. 이후 독립.
- **locked 상속**: 상위 어느 레벨에서든 `locked=true`면 해당 키는 모든 하위 스코프에서 재정의 불가.
- **자동 전파 없음**: 부모가 나중에 설정을 바꿔도 자식에게 자동 전파되지 않음. 명시적 propagate API로만 전파.

**유효 값 해결 순서** (읽기 시, 트리 루트까지 순회):

```
User
  → Domain (domain이 locked 아닌 경우)
    → Company (직속 부모)
      → Parent Company (grandparent, ...)
        → Root Company
          → Global (시스템 기본값)

※ 어느 레벨에서든 locked=true를 만나면 그 이하 스코프는 즉시 차단
```

- 트리 순회는 `companies.parent_id` 체인을 루트까지 따라감.
- 실제 배포에서 depth ≤ 3이므로 조회 비용 무시 가능.
- 메모리 캐시에 company 트리 + config를 함께 유지하여 런타임 DB 조회 없음.

**Propagate API (명시적 전파)**:

```
POST /admin/v1/companies/{id}/config/propagate
  ?scope=subtree   ← 이 회사 + 모든 하위 자회사 + 그들의 도메인에 강제 적용
  ?scope=children  ← 즉각 자식 자회사만
  ?scope=domains   ← 이 회사 직속 도메인만
Body: { "key": "...", "value": ..., "locked": true/false }
```

**구현 순서**:

1. Migration: `companies.parent_id UUID REFERENCES companies(id) ON DELETE SET NULL`.
2. Migration `runtime_config` 테이블: `(scope_type, scope_id, key, value JSONB, locked BOOLEAN, version BIGINT, updated_at)`, unique `(scope_type, scope_id, key)`.
3. `internal/configstore` package:
   - `ConfigStore` interface: `Resolve(ctx, userID, domainID, companyID, key) (json.RawMessage, error)`.
   - `PostgresConfigStore`: 시작 시 full load (company 트리 포함) → in-memory 계층 맵 → `LISTEN config_changed` goroutine.
   - reconnect/failover handler: 재연결 후 full reload.
   - `EtcdConfigStore` stub: 인터페이스만, 구현 deferred.
4. Admin API:
   - `GET/POST/PUT/DELETE /admin/v1/companies/{id}/config/{key}`
   - `GET/POST/PUT/DELETE /admin/v1/domains/{id}/config/{key}`
   - `GET/POST/PUT/DELETE /admin/v1/users/{id}/config/{key}`
   - `POST /admin/v1/companies/{id}/config/propagate?scope=subtree|children|domains`
5. 생성 훅: 자회사/도메인 생성 시 직속 부모 설정 자동 복사 (unlocked 키만).
6. Optimistic locking: `UPDATE ... WHERE version = $expected` → 충돌 시 `409 Conflict`.
7. 감사 로그: `config_change_log (scope_type, scope_id, key, old_value, new_value, changed_by, changed_at)`.
8. 기존 env-var config와 공존: 점진적으로 동적 설정으로 이관.
9. 테스트: 트리 해결 순서 (루트→자회사→도메인→사용자), locked 차단, propagate 전파 범위, 생성 복사.

### 2-B. 2FA / TOTP (RFC 6238)

2FA 정책은 설정 계층의 일부로 동작한다. 도메인 정책에 따라 강제/선택/비활성화.

**설정 키**:
```json
"auth.mfa.mode":     "optional" | "required" | "disabled"
"auth.mfa.methods":  ["totp"]              // 향후 SMS, WebAuthn 확장 가능
"auth.mfa.grace_period_hours": 24          // 신규 사용자 유예 기간
```

- 회사 기본값 설정 → 도메인이 독립 재정의 가능 (회사가 locked하지 않는 한).
- 예: 회사 기본 `optional`, 특정 보안 도메인에서 `required`로 재정의.

**TOTP 구현 (RFC 6238)**:

1. `internal/authmfa` package:
   - TOTP 시크릿 생성 (32바이트 랜덤, Base32 인코딩).
   - QR코드 URI: `otpauth://totp/{issuer}:{email}?secret={secret}&issuer={issuer}&digits=6&period=30` (RFC 6238).
   - 코드 검증: 현재 시간 ±2 window (±60초 허용, 클럭 스큐 대응).
   - 리플레이 방지: 검증된 코드 `totp_used_codes` 테이블에 기록, 같은 코드 재사용 차단.
2. Migration: `user_mfa_secrets (user_id, secret_encrypted, verified, created_at, last_used_at)`.
3. Recovery codes: 8개 단일 사용 복구 코드 생성/검증/재생성.
4. Auth flow 연동:
   - `auth.mfa.mode=required`: 1차 인증(비밀번호/SSO) 성공 후 TOTP 입력 단계 강제.
   - `auth.mfa.mode=optional`: 사용자가 설정에서 활성화 선택.
   - `auth.mfa.mode=disabled`: TOTP 등록/검증 경로 비활성화.
5. Admin API: 특정 사용자 MFA 강제 초기화 (관리자 지원 시).
6. JWT에 `mfa_verified: true` 클레임 포함 — API는 required 도메인에서 이 클레임 없는 토큰 거부.

### 2-C. Batch Worker & Distributed Job Lock

**`--mode=batch-worker`** 는 주기적 반복 작업을 담당하는 단독 실행 가능 컴포넌트다.
다중화(수평 확장) 시 동일 잡의 **중복 실행을 원천 차단**하는 설계를 내장한다.

**작업 유형별 중복 방지 전략**:

| 작업 유형 | 메커니즘 |
|---|---|
| 큐 기반 (outbox, 예약 메일) | `SELECT FOR UPDATE SKIP LOCKED` — 행 단위 원자적 분배 |
| 주기적 반복 잡 (cleanup, sync 등) | PostgreSQL Advisory Lock — 전역 잠금, 크래시 시 자동 해제 |

**Advisory Lock 선택 이유**:
- PostgreSQL이 이미 필수 의존성 → 추가 인프라 없음
- 프로세스 크래시 시 연결 종료와 함께 잠금 자동 해제 (Redis TTL 만료 대기 없음)
- 강한 일관성 보장

**`internal/batchlock` 패키지**:

```go
type JobLock interface {
    TryAcquire(ctx context.Context, jobName string) (acquired bool, release func(), err error)
}

// PostgresJobLock: pg_try_advisory_lock(hashtext('batch:' + jobName))
// acquired=false → 다른 인스턴스 실행 중, 정상 skip
// 크래시 → PG 연결 종료 시 pg_advisory_unlock 자동 호출
```

**모든 주기적 잡의 표준 패턴**:

```go
func (j *SomePeriodicJob) Run(ctx context.Context) error {
    acquired, release, err := j.lock.TryAcquire(ctx, "job-name")
    if err != nil { return err }
    if !acquired { return nil }  // 다른 인스턴스 실행 중 — skip
    defer release()
    return j.doWork(ctx)
}
```

**초기 등록 잡 목록**:

1. `ScheduledMailFlusherJob` — `available_at <= now()` 예약 메일 상태 점검 및 enqueue 확인
2. `OrgChartSyncJob` — 외부 HR 시스템 연동 조직도 주기적 동기화 (인터페이스만, 어댑터는 플러그인)
3. `QuotaAlertCheckJob` — 사용자/도메인/회사 할당량 임계치 초과 시 알림 이벤트 emit
4. `MFAGracePeriodJob` — 2FA 유예기간 만료 사용자 처리
5. `TokenCleanupJob` — 만료된 TOTP used-codes, 세션 토큰 정리

**구현 순서**:

1. `internal/batchlock` 패키지: `PostgresJobLock` 구현 + `TryAcquire` 단위 테스트.
2. `batch-worker` 모드 wiring: job registry + ticker loop + graceful shutdown.
3. 각 잡 구현 (인터페이스 기반, 잡별 interval 환경변수 설정).
4. 테스트: 동시 2개 인스턴스 → 하나만 실행되고 나머지는 skip 검증.

### 2-D. 실시간 설정 전파 (SSE) + 스코프 보안

**목표**: 관리 콘솔에서 설정 변경 시 로그인된 모든 프론트엔드 클라이언트에 즉시 반영.

**아키텍처 (백엔드 LISTEN/NOTIFY → SSE)**:

```
DB 트리거 → NOTIFY 'config_changed' payload
     │
     └── ConfigStore LISTEN goroutine
               │
               ├── 인메모리 캐시 갱신 (즉시)
               └── SSE fan-out → 연결된 클라이언트에게 push
```

- 백엔드 프로세스 간 전파: `LISTEN/NOTIFY`로 이미 커버됨 (2-A 설계 포함).
- 프론트엔드 클라이언트 전파: SSE 스트림 추가.
- 클라이언트는 변경된 `scope_type + key` 정보를 받아 UI를 즉시 갱신.

**SSE 엔드포인트**:

```
GET /api/v1/config/stream
  Authorization: Bearer <user JWT>
  Accept: text/event-stream

GET /admin/v1/config/stream
  Authorization: Bearer <admin token>
  Accept: text/event-stream
```

이벤트 포맷:
```
event: config_changed
data: {"scope_type":"domain","scope_id":"...","key":"auth.mfa.mode","version":42}
```

**구현 순서**:

1. `internal/configstore.Notifier` 인터페이스: `Subscribe() <-chan ConfigChangeEvent`, `Unsubscribe(ch)`.
2. `PostgresConfigStore`에 subscriber fan-out 추가: NOTIFY 수신 시 모든 채널로 broadcast.
3. `GET /api/v1/config/stream` — 사용자 JWT 필요, 해당 사용자 스코프(domain + user) 이벤트만 전달.
4. `GET /admin/v1/config/stream` — 관리자 토큰 필요, 전체 이벤트 스트림.
5. SSE 연결 유지: 30초 heartbeat (`event: ping`), 클라이언트 연결 종료 시 정리.
6. 테스트: DB 설정 변경 → SSE 이벤트 수신 확인 (통합 테스트).

**어드민/사용자 스코프 보안 경계**:

| 스코프 | 읽기 | 쓰기 |
|---|---|---|
| `global` | 시스템 관리자 전용 | 시스템 관리자 전용 |
| `company` | 회사 관리자 이상 | 회사 관리자 이상 |
| `domain` | 도메인 관리자 이상 | 도메인 관리자 이상 |
| `user` | 해당 사용자 본인만 | 해당 사용자 본인만 |

- 관리자(어드민)는 **`user` 스코프에 직접 쓰기 불가** → `403 Forbidden`.
- 예외적 관리 작업(MFA 초기화, 비밀번호 리셋)은 별도 감사 로그 기록 전용 엔드포인트를 통해서만.
- 도메인 관리자는 자신이 관리하는 도메인 범위 내의 `company`/`domain` 스코프만 접근.
- JWT 클레임에 `admin_scope: domain_id` 포함 → API는 요청 scope_id와 대조 검증.

### 2-E. Open API + API 키 관리 (도메인 관리자용)

**목표**: 써드파티 시스템이 표준 인증된 REST API로 메일/캘린더/주소록 데이터에 접근하고 메일을 발송할 수 있다. 도메인 관리자가 직접 API 키를 발급·관리하며, IP 대역(CIDR) 제한을 설정할 수 있다.

**현황**: `POST /api/v1/messages/send`, `GET /api/v1/messages` 등 Mail REST API는 이미 구현됨. 인증은 사용자 JWT 전용. 써드파티용 API 키 발급/검증 레이어 없음.

**API 키 스키마**:

```sql
domain_api_keys (
  id            uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  domain_id     uuid        NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
  name          TEXT        NOT NULL,                  -- 식별용 레이블 (예: "CRM Integration")
  key_prefix    TEXT        NOT NULL,                  -- 노출용 접두사 (예: "gm_abc12345")
  key_hash      TEXT        NOT NULL,                  -- bcrypt hash, 원본은 발급 시 1회만 표시
  scopes        TEXT[]      NOT NULL DEFAULT '{}',     -- ['mail:read','mail:send','calendar:read', ...]
  cidr_allowlist CIDR[]     NOT NULL DEFAULT '{}',     -- 빈 배열 = ANY (모든 IP 허용)
  created_by    uuid        REFERENCES users(id),
  created_at    TIMESTAMPTZ DEFAULT now(),
  last_used_at  TIMESTAMPTZ,
  expires_at    TIMESTAMPTZ,                           -- NULL = 만료 없음
  revoked       BOOLEAN     DEFAULT false,
  revoked_at    TIMESTAMPTZ,
  UNIQUE (key_prefix)
)
```

**API 키 포맷**: `gm_{prefix8}_{secret32}` (prefix는 저장, secret은 bcrypt 해시만 저장).

**CIDR 제한 동작**:
- `cidr_allowlist = []` → IP 제한 없음 (ANY).
- `cidr_allowlist = ['192.168.1.0/24', '203.0.113.5/32']` → 해당 대역만 허용.
- PostgreSQL `CIDR` 타입 + `<<=` 연산자로 O(n) 검증, 키당 최대 20개 CIDR 허용.
- 불일치 시 `403 Forbidden` + 로그 기록.

**지원 스코프**:

| 스코프 | 권한 |
|---|---|
| `mail:read` | 메일함 읽기, 메시지 목록/상세 조회 |
| `mail:send` | 메일 발송 (`POST /api/v1/messages/send`) |
| `mail:manage` | 이동/삭제/폴더 관리 |
| `calendar:read` | 캘린더/이벤트 읽기 |
| `calendar:write` | 이벤트 생성/수정/삭제 |
| `contacts:read` | 주소록 읽기 |
| `contacts:write` | 주소록 쓰기 |
| `admin:users` | 도메인 내 사용자 관리 (도메인 관리자용) |

**인증 흐름**:

```
Authorization: Bearer gm_{prefix}_{secret}

1. prefix로 domain_api_keys 행 조회 (캐시 가능)
2. bcrypt.Compare(secret, key_hash) 검증
3. revoked / expires_at 확인
4. cidr_allowlist ≠ [] → remote_ip << 각 CIDR 검사
5. 요청 경로의 필요 scope ∈ key.scopes 확인
6. 검증 통과 → domain_id + scopes를 request context에 주입
7. last_used_at 비동기 갱신
```

**도메인 관리자 API (Admin 콘솔)**:

```
GET    /admin/v1/domains/{id}/api-keys           # 목록 (secret 미포함)
POST   /admin/v1/domains/{id}/api-keys           # 발급 → 응답에 full key 1회 포함
GET    /admin/v1/domains/{id}/api-keys/{keyId}   # 상세 (secret 미포함)
PATCH  /admin/v1/domains/{id}/api-keys/{keyId}   # cidr_allowlist / scopes / name / expires_at 수정
DELETE /admin/v1/domains/{id}/api-keys/{keyId}   # 즉시 폐기 (revoked=true)
POST   /admin/v1/domains/{id}/api-keys/{keyId}/rotate  # 시크릿 재발급
```

**구현 순서**:

1. Migration: `domain_api_keys` 테이블 (CIDR 배열 포함).
2. `internal/apikeys` 패키지:
   - `Generate() (fullKey, prefix, hash string)` — crypto/rand 기반.
   - `Verify(fullKey, hash string) bool` — bcrypt 검증.
   - `CheckCIDR(ip net.IP, allowlist []net.IPNet) bool` — 빈 배열 = 허용.
3. `ApiKeyMiddleware`: `Authorization: Bearer gm_...` 패턴 감지 → JWT 검증과 분기.
4. Admin API CRUD + rotate 엔드포인트.
5. 기존 Mail/Calendar/Contacts API에 scope 검증 레이어 추가 (JWT 경로는 현행 유지).
6. 감사 로그: 발급/폐기/CIDR 변경 이벤트 `config_change_log` 기록.
7. 테스트: CIDR 허용/차단, 스코프 부족 거부, 만료/폐기 키 거부, rotate 후 구 키 무효화.

---

## Phase 3: Enterprise Identity & Directory

Standards first — every component maps to a public RFC so operators can swap implementations without vendor lock-in.

Target outcome:

> 기업 디렉토리(LDAP)와 프로비저닝(SCIM 2.0), SSO(SAML/OIDC)가 표준 프로토콜로 동작한다.

### 3-A. LDAP Gateway (RFC 4511)

Read-only LDAP server so mail clients, CalDAV/CardDAV resolvers, and internal autocomplete can query the directory over the standard protocol without a proprietary API.

Implementation order:

1. `internal/ldapgw` package: LDAP v3 protocol listener (RFC 4511).
2. `BindRequest`: simple bind with domain user credentials, delegating to existing auth boundary.
3. `SearchRequest`: maps LDAP filter to SQL/repository queries for `cn`, `mail`, `uid`, `displayName`, `givenName`, `sn` attributes (RFC 4519 schema).
4. `SearchResultEntry`: returns attributes from `users`/`user_addresses`.
5. Read-only enforced: `AddRequest`, `ModifyRequest`, `DeleteRequest`, `ModifyDNRequest` return `unwillingToPerform` using their RFC 4511 response protocolOp tags.
6. TLS: LDAPS (implicit, port 636) and StartTLS (RFC 4511 §4.14) support.
7. LDAP referral for multi-domain environments.
8. Metrics boundary for bind/search/error observations.

Current implementation notes:

- LDAP gateway mode (`gogomail --mode=ldap-gateway`) can expose plaintext LDAP on `GOGOMAIL_LDAP_ADDR` and implicit TLS LDAP on `GOGOMAIL_LDAPS_ADDR`.
- StartTLS is advertised through Root DSE `supportedExtension` and handled when `GOGOMAIL_LDAP_TLS_CERT_FILE` / `GOGOMAIL_LDAP_TLS_KEY_FILE` are configured.
- Root DSE exposes Active Directory-style discovery metadata (`defaultNamingContext`, `rootDomainNamingContext`, `configurationNamingContext`, `schemaNamingContext`, `supportedCapabilities`, `dnsHostName`, `serverName`, `dsServiceName`, `currentTime`, `highestCommittedUSN`, domain/forest functionality levels, and readiness flags) derived from the configured naming context for AD-oriented clients.
- Root DSE advertises `supportedFeatures=1.3.6.1.4.1.4203.1.5.1` so OpenLDAP-compatible clients can discover all-operational-attributes support.
- Root DSE advertises `subschemaSubentry`, and `cn=Subschema` base-object search returns minimal RFC 4512/RFC 4519 schema metadata for person/inetOrgPerson-style directory clients.
- Root DSE, subschema, and synthetic kind-container base-object searches validate and apply the requested LDAP filter before returning entries.
- SearchRequest parsing now covers and validates scope, deref aliases, client size/time limits, typesOnly, filter, and requested attribute selection before mapping supported directory filters into the repository boundary.
- SearchRequest `timeLimit` is enforced with `timeLimitExceeded` results when repository lookup or post-filtering runs past the client-requested duration.
- Negative SearchRequest `sizeLimit` and `timeLimit` BER INTEGER values are rejected during decoding instead of being widened into large positive limits.
- AbandonRequest cancels outstanding same-connection searches by message ID while preserving the RFC no-response behavior for both the abandon operation and the abandoned search.
- AbandonRequest target IDs are range-checked before cancellation, so malformed, negative, zero, overlong, or above-`maxInt` abandon targets are ignored without changing the required no-response behavior.
- OCTET STRING encoding uses BER long-form lengths for values beyond 127 bytes, preserving long DNs, long attribute values, controls, and binary AD compatibility attributes.
- BindRequest encoding uses BER long-form lengths for large generated bind requests, covering long bind DNs and long simple-auth credentials in internal/client-side helpers.
- Common client search filters using OR/AND/NOT wrappers, substring matches, and RFC 4511 extensibleMatch type/value assertions are accepted for RFC 4519 directory attributes (`cn`, `mail`, `uid`, `displayName`, `givenName`, `sn`) plus Active Directory-style compatibility aliases (`name`, `sAMAccountName`, `userPrincipalName`).
- Repository narrowing uses only safe conjunctive LDAP filter hints; OR/NOT branches are validated and left to the full entry filter evaluator so matching entries are not under-returned.
- LDAP filter validation now recursively rejects malformed child filters, trailing bytes, and empty or option-only AttributeDescription values before repository narrowing or post-filter evaluation.
- LDAP ordering filters (`greaterOrEqual`, `lessOrEqual`) are evaluated only by the full entry filter evaluator, avoiding unsafe text-search narrowing.
- Negated LDAP filter assertions are validated but not used as positive repository search terms or principal-kind narrowing hints.
- SearchResultEntry candidates are post-filtered against the full LDAP filter tree, including AD bitwise `userAccountControl` matching rules, so repository search hints cannot over-return entries that fail the requested filter.
- LDAP search materializes candidate LDAP attributes once and reuses them across full-filter evaluation, server-side sort, matched-values processing, and response projection to avoid repeated per-entry map construction.
- Paged-results searches fetch at least the default directory candidate window before post-filter pagination, avoiding sparse-match under-return when clients request very small pages.
- LDAP search scans repository candidates in bounded 100-entry batches up to a protective cap when full-filter post-processing needs more entries, covering sparse matches beyond the first repository window.
- LDAP response encoding preserves BER INTEGER message IDs above 255 for long-lived clients that continue incrementing request IDs.
- LDAPMessage decoding rejects out-of-range `messageID` values, including zero, negative BER INTEGER encodings, values above RFC 4511 `maxInt`, and overlong integer encodings.
- BER length decoding rejects indefinite-length encodings instead of treating them as zero-length values, preserving the definite-length envelope expected by LDAP clients and servers.
- BER length decoding rejects overlong length-of-length fields and element lengths above the 16 MB safety cap before allocating or slicing payloads.
- LDAPMessage control parsing rejects trailing bytes after the optional controls wrapper instead of silently ignoring data beyond the RFC 4511 envelope.
- LDAP control decoding validates `controlType` as a numeric LDAPOID, rejecting empty, descriptor-style, or malformed dotted strings before control dispatch.
- LDAP user entries expose Exchange/AD-style mail aliases (`mailNickname`, `proxyAddresses`) and directory filter extraction accepts those attributes for broader mail-client lookup compatibility.
- LDAP entries expose AD-style identity attributes (`distinguishedName`, binary deterministic `objectGUID`, binary deterministic `objectSid`, `objectCategory`) alongside the RFC-oriented attributes for clients that cache or render from those fields.
- LDAP entries expose AD-style directory metadata (`canonicalName`, `instanceType`, `whenCreated`, `whenChanged`, `uSNCreated`, `uSNChanged`) plus conservative user account hints (`userAccountControl`, `accountExpires`, `primaryGroupID`) for Active Directory-oriented address book and tree-browsing clients, with canonical-name filters narrowed to the final path segment for repository lookup.
- Repository query extraction preserves organization/resource attributes (`ou`, `description`) in addition to user attributes.
- Principal-kind-aware LDAP entries now map users to `inetOrgPerson` plus AD-compatible `user`, organizations to `organizationalUnit`, groups to `groupOfNames`, and resources to `device` under kind-specific OU subtrees.
- LDAP `groupOfNames` entries populate requested/default `member` attributes with active Directory group-membership DNs for user, organization, group, and resource principals, with a schema-safe fallback for empty groups and no membership expansion for narrow non-member projections.
- LDAP principal entries populate requested/default `memberOf` reverse membership attributes from Directory group memberships, with no reverse expansion for narrow non-memberOf projections.
- LDAP `objectClass`/`objectCategory` filters and base DNs narrow repository searches to the matching principal kinds, including organization-chart searches under `ou=organizations` and AD-style category searches.
- LDAP search scope filtering now applies base-object, one-level, and subtree semantics against RFC 4514-normalized generated entry DNs, including equivalent escaped separators such as `\,` and `\2c`.
- CompareRequest base-object lookup uses the same RFC 4514-normalized generated-DN matching as SearchRequest, so equivalent escaped DN forms remain interoperable for `ldapcompare`-style clients.
- LDAP client size limits return `sizeLimitExceeded` only when matching entries exceed the requested limit, while exact-limit result sets complete with success.
- Simple bind accepts common client identity formats, including raw username/email and generated entry DN forms such as `uid=<user-id>,ou=users,...`, while preserving escaped commas and hex-escaped first-RDN values during identity extraction.
- Failed re-bind clears the connection authorization state, and unsupported bind authentication choices return `authMethodNotSupported` instead of falling through as empty simple passwords.
- Generated user/organization/group/resource DNs escape RDN values, and base-object lookup unescapes generated DN values before resolving principals.
- Non-discovery directory searches require successful bind while Root DSE and `cn=Subschema` discovery remain pre-bind accessible.
- SearchResultEntry responses are encoded as RFC 4511 application payloads and covered by OpenLDAP `ldapsearch` plaintext, Assertion control, Matched Values control, Domain Scope/Don't Use Copy/Subentries controls, LDAP Sync refreshOnly, Proxied Authorization, Dereference, Relax/Password Policy/Session Tracking/Pre/Post Read controls, extensibleMatch, server-side sort, Virtual List View, StartTLS, LDAPS, and paged-results smoke tests when the client is available.
- SearchRequest requested-attribute lists are decoded from real client requests, preserving narrow attribute projection for compatibility and payload efficiency; AttributeDescription options such as `mail;lang-en` are resolved against their base attribute for filter, CompareRequest, sort, and projection matching.
- SearchRequest decoding rejects malformed, empty, option-only requested-attribute lists and trailing data after the attribute list instead of silently truncating or ignoring the request tail.
- Attribute selection honors LDAP special selectors: `1.1` for no attributes, `*` for user attributes, and `+` for operational attributes.
- Principal and container entries expose stable operational attributes (`entryDN`, deterministic `entryUUID`, timestamps, creator/modifier names, subordinate hints) for clients that request `+`.
- LDAP entries provide conservative fallbacks for declared objectClass MUST attributes, including user `sn` and empty-group `member`.
- Read-only write operations reject Modify/Add/Delete/ModifyDN with `ModifyResponse`, `AddResponse`, `DelResponse`, and `ModifyDNResponse` tags plus `unwillingToPerform`.
- Read-only CompareRequest is implemented with RFC 4511 CompareResponse result codes and OpenLDAP `ldapcompare` coverage.
- CompareRequest decoding rejects trailing data after the attribute value assertion sequence instead of ignoring bytes beyond the RFC 4511 request shape.
- Root DSE advertises the Who Am I? extended operation (`1.3.6.1.4.1.4203.1.11.3`), with OpenLDAP `ldapwhoami` coverage.
- ExtendedRequest decoding validates request names as numeric LDAPOIDs and rejects trailing data unless it is the RFC 4511 optional requestValue wrapper.
- AbandonRequest follows RFC 4511 no-response semantics.
- Cross-naming-context requests can return SearchResultReference values from `GOGOMAIL_LDAP_REFERRAL_URLS` for multi-domain deployments.
- LDAPMessage controls are parsed separately from protocolOp bytes; supported critical controls (`ManageDsaIT`, Simple Paged Results, Server Side Sort, Virtual List View, Assertion, Matched Values, Domain Scope, Don't Use Copy, Subentries, LDAP Sync refreshOnly, Proxied Authorization, Dereference, Relax, No-Op, Pre/Post Read, Password Policy, Session Tracking) are accepted, Simple Paged Results returns continuation cookies, Server Side Sort orders entries and returns the RFC response control, Virtual List View returns sorted position/count windows, Assertion validates RFC 4528 filters before search dispatch, Matched Values applies RFC 3876 filters to response attribute values, Domain Scope and Don't Use Copy are accepted as read-only directory no-ops, Subentries returns an empty success result for `subentries=true`, LDAP Sync refresh returns entry sync-state controls and sync-done, Proxied Authorization allows only the already-bound authorization identity, Dereference decodes OpenLDAP dereference specs and accepts them as read-only no-ops until DN-valued relationship expansion exists, common OpenLDAP general controls are accepted as read-only no-ops, and unsupported critical controls return `unavailableCriticalExtension`.
- Bind/search/extended/read-only outcomes now expose a metrics boundary and can be logged through `GOGOMAIL_METRICS_BACKEND=slog`.
- LDAP connection reads accumulate valid BER PDUs beyond the initial 8 KiB read buffer up to the configured safety cap, while oversized declared messages are rejected.
- Remaining hardening for full enterprise interoperability: broader optional client controls and a wider real-client compatibility matrix.

### 3-B. SCIM 2.0 Provisioning API (RFC 7642, 7643, 7644)

Machine-to-machine user/group provisioning for identity providers (Okta, Azure AD, Google Workspace).

Implementation order:

1. `internal/scimsvc` package: SCIM 2.0 REST server (RFC 7644).
2. `/scim/v2/Users` CRUD: `GET`/`POST`/`PUT`/`PATCH`/`DELETE` with RFC 7643 User schema.
3. `/scim/v2/Groups` CRUD: group-to-domain mapping with member management.
4. `/scim/v2/ServiceProviderConfig` and `/scim/v2/ResourceTypes` discovery endpoints.
5. Filter operators: `eq`, `ne`, `co`, `sw`, `ew`, `pr`, `gt`, `ge`, `lt`, `le`, `and`, `or`, `not` (RFC 7644 §3.4.2).
6. Pagination: `startIndex`/`count`/`totalResults` list envelope.
7. ETag-based optimistic locking for concurrent provisioning conflicts.
8. SCIM bearer token auth independent of the admin token.
9. Audit log entries for all provisioning operations.

### 3-C. SAML 2.0 / OIDC SSO

Federated login for enterprise tenants without passwords stored in gogomail.

Implementation order:

1. SAML 2.0 SP mode: consume IdP assertions and issue gogomail session tokens.
2. OIDC Relying Party mode: PKCE + authorization code flow (RFC 7636); map `sub`/`email` claims to gogomail users.
3. Per-domain IdP configuration in `sso_configurations` table.
4. Just-in-time provisioning: create user on first SSO login when domain allows it.
5. Session token lifetime and refresh policy configurable per domain.
6. Admin API CRUD at `/admin/v1/sso-configurations`.

---

## Phase 4: Storage & Collaboration

Target outcome:

> Drive 파일 스토리지가 WebDAV 표준 프로토콜로 접근 가능하고, CalDAV/CardDAV가 프로덕션 수준의 클라이언트 호환성을 갖춘다.

### 4-A. Drive WebDAV Gateway (RFC 4918, RFC 3744, RFC 4331)

WebDAV exposes gogomail Drive over the standard protocol so macOS Finder, Windows Explorer, Cyberduck, and rclone can mount it without proprietary plugins.

Implementation order:

1. `internal/webdavgw` package: WebDAV server (RFC 4918).
2. Methods: `OPTIONS`, `PROPFIND`, `PROPPATCH`, `MKCOL`, `GET`, `PUT`, `DELETE`, `COPY`, `MOVE`, `LOCK`, `UNLOCK`.
3. Live properties: `DAV:` namespace (`getcontentlength`, `getcontenttype`, `getetag`, `getlastmodified`, `resourcetype`, `creationdate`, `displayname`).
4. ACL (RFC 3744): `current-user-privilege-set`, `acl`, `supported-privilege-set` — read/write/read-acl for owner and shared principals.
5. Quota (RFC 4331): `quota-used-bytes`/`quota-available-bytes` delegating to quota ledger.
6. `Depth:` headers: `0`, `1`, `infinity` with configurable infinity guard.
7. Locking: shared/exclusive write locks with `Lock-Token` header and `If:` conditional requests.
8. `PUT` streams body directly to object storage; no full-file buffering.
9. Auth: bearer token in `Authorization` header; Basic auth over HTTPS only.
10. Drive quota enforced before `PUT` completes.
11. Metrics boundary for method/outcome observations.

### 4-B. CalDAV / CardDAV Production Hardening

Promote from experimental to production-ready client compatibility.

1. Compatibility test matrix: Apple iCal, Thunderbird Lightning, DAVx⁵ (Android), Outlook + CalDav Synchronizer.
2. CardDAV: vCard 3.0/4.0 validation, REPORT parsing, multistatus boundary hardening.
3. CalDAV: production sync-token retention-age policy, full recurrence edge case coverage.
4. Well-Known URIs per RFC 6764 (`/.well-known/caldav`, `/.well-known/carddav`).
5. iOS/macOS autodiscovery via DNS SRV records (`_caldavs._tcp`, `_carddavs._tcp`).

---

## Phase 5: Mail Security & Filter Module

Target outcome:

> milter 프로토콜로 외부 스팸 필터(Rspamd, SpamAssassin, ClamAV)가 gogomail 수신 파이프라인에 연결된다.

### 5-A. Milter Protocol Adapter

Milter is the de-facto MTA filter protocol. gogomail exposes a milter client hook so external milter servers attach at standard pipeline stages without coupling spam logic to SMTP core.

Implementation order:

1. `internal/milter` package: milter client (gogomail connects to external milter server over TCP).
2. Milter protocol v2/v6 framing: negotiate capabilities, send macro/command frames, receive action responses.
3. Stages: `CONNECT`, `HELO`, `MAIL FROM`, `RCPT TO`, `HEADER`, `EOH`, `BODY`, `EOM`.
4. Actions: `ACCEPT`, `REJECT`, `TEMPFAIL`, `DISCARD`, `QUARANTINE`, `SKIP`, header add/modify/delete, body replace.
5. Hook adapter wires into existing `authentication_checked` and `parsed` SMTP pipeline stages.
6. Shadow mode: verdict is logged but not enforced (A/B rollout).
7. Disabled by default (`GOGOMAIL_MILTER_ENABLED=false`); external pre-filtering via smarthost relay is an independent deployment topology.
8. Multiple milter targets configurable with per-milter policy (accept/reject/timeout).
9. Milter connection pool with health checks and circuit breaker.

### 5-B. RBL / DNSBL Integration (RFC 5782)

DNS-based block list checks at `RCPT TO` stage before content scanning.

1. Configurable DNSBL zone list (Spamhaus ZEN, Barracuda, etc.).
2. A-record lookup for `<reversed-ip>.dnsbl.zone`.
3. Return code interpretation per RFC 5782 §2.2.
4. Results propagated as `authentication_checked` hook results alongside SPF/DKIM/DMARC.
5. Per-zone policy: `monitor`, `reject`, or `tag`.

### 5-C. External Spam Pre-processing via Smarthost (already implemented)

When an upstream spam MTA pre-filters before gogomail, the relay/smarthost gateway deployment pattern already works:

- `RelayAuthorizer` trusted-relay CIDR enforcement gates which IPs can submit without auth.
- Disable internal milter (`GOGOMAIL_MILTER_ENABLED=false`) for this topology.
- Document as an approved ADR deployment pattern.

---

## Phase 6: POP3

Standards: RFC 1939 (POP3), RFC 2449 (CAPA/extensions), RFC 2595 (STLS), RFC 1734 (AUTH).

Target outcome:

> POP3/POP3S로 모든 메일 클라이언트가 메일을 다운로드할 수 있다.

Implementation order:

1. `internal/pop3d` package: POP3 server.
2. Commands: `USER`, `PASS`, `STAT`, `LIST`, `RETR`, `DELE`, `NOOP`, `RSET`, `QUIT`.
3. Extensions: `UIDL`, `TOP`, `STLS` (RFC 2595), `CAPA` (RFC 2449), `AUTH PLAIN/LOGIN` (RFC 1734).
4. POP3S implicit TLS on port 995 alongside STARTTLS.
5. Per-user exclusive maildrop lock: no concurrent POP3 access to the same mailbox.
6. `DELE` marks messages; `QUIT` commits as soft-deletes in `messages` table.
7. `RETR` streams `.eml` from object storage without full in-memory load.
8. Metrics boundary for AUTH/RETR/DELE observations.
9. Pipeline stages parallel to IMAP so quota enforcement and audit logging attach without duplicating logic.

Progress:

1. POP3 `RETR` and `TOP` multi-line responses now apply RFC 1939
   dot-stuffing to every content line beginning with `.`, canonicalize content
   line endings to CRLF, and preserve the final `.\r\n` terminator. Regression
   tests cover body, header, and TOP body dot-starting lines.
2. POP3 `CAPA` now advertises `STLS` only before authentication when TLS is
   configured and has not already been negotiated. TRANSACTION-state `CAPA`
   omits `STLS`, and TRANSACTION-state `STLS` returns `-ERR` without closing the
   mailbox session.
3. POP3 raw-message fetch failures now propagate from the mailservice adapter
   to the protocol server. `RETR` and `TOP` return `-ERR` before starting a
   multi-line success response when message content cannot be loaded.
4. The mailservice POP3 adapter now walks INBOX with cursor pagination using
   the service message-list maximum page size, preserving message order and
   exposing mailboxes larger than a single normalized page to POP3 clients.
5. POP3 authentication now enforces an exclusive maildrop lock per normalized
   mailbox key before entering TRANSACTION state. Concurrent logins for the
   same user receive `-ERR`, and locks are released on QUIT or connection
   close. The mailservice adapter provides the canonical DB user ID as the lock
   key.
6. POP3 `MaxConnections` is now wired from `GOGOMAIL_POP3_MAX_CONNECTIONS` and
   YAML `pop3_max_connections` into the server accept loop. Connections over
   the configured limit receive `-ERR too many connections`, and slots are
   released when sessions close.
7. POP3 `CAPA` responses are now state-aware. AUTHORIZATION state advertises
   `USER` and `SASL PLAIN LOGIN`; TRANSACTION state omits auth-only
   capabilities while preserving common capabilities and stable
   `IMPLEMENTATION gogomail` / `LOGIN-DELAY 0` metadata.
8. POP3 `AUTH PLAIN` and `AUTH LOGIN` now reject mechanism-specific extra
   arguments with `-ERR syntax error` before SASL decoding or continuation
   prompts, keeping the connection in AUTHORIZATION state for a clean retry.
9. POP3 SASL continuation cancellation now handles `*` for `AUTH PLAIN` and
   both `AUTH LOGIN` prompts, returning `-ERR authentication cancelled` while
   preserving the connection for a subsequent authentication attempt.
10. POP3 `STLS` now clears any pre-TLS `USER` state after a successful TLS
    handshake, so clients must repeat `USER`/`PASS` on the protected channel.
11. POP3S implicit TLS runtime wiring is now available via
    `GOGOMAIL_POP3S_ADDR` / YAML `pop3s_addr`. The implicit TLS listener uses
    the configured POP3 TLS certificate/key and shares the same POP3 server
    instance, maildrop locks, and connection limit as the STLS listener.

---

## Phase 7: Push Notifications & Mobile Sync

Target outcome:

> 새 메일/캘린더 이벤트가 iOS/Android/Web 디바이스에 즉시 푸시된다.

### 7-A. FCM / APNs / Web Push Adapters (`internal/pushnotify`)

1. `PushSink` interface: `Send(ctx, DeviceToken, Payload) error`.
2. Concrete adapters: `FCMSink` (Firebase Cloud Messaging), `APNsSink` (Apple Push Notification service), `WebPushSink` (RFC 8030 Web Push).
3. Per-device registration in `device_tokens` table: platform, token, user binding, expiry.
4. Event worker consumes `mail.received`, `calendar.event_reminder`, `calendar.invite_received` and fans out to registered devices.
5. Quiet-hours policy per device: suppress push during configured local-time windows.
6. Retry with exponential backoff; expired/invalid tokens auto-unregistered.
7. Admin API `/admin/v1/device-tokens` for listing and revoking device registrations.

### 7-B. Delta Sync Boundary

1. Per-device delta-sync cursor: `last_seen_sequence` for mailbox, calendar, and contacts.
2. IMAP IDLE fans out real-time mailbox events to connected clients.
3. CalDAV sync-token retention lets offline clients catch up without full re-sync.
4. CardDAV ctag/sync-token support for contacts delta sync.
5. CalDAV time-range `calendar-query` uses component candidate walking directly,
   avoiding broad or component-list prefetch on large calendars.
6. CalDAV `free-busy-query` limits optimized object loading to VEVENT and
   VFREEBUSY candidates so VTODO/VJOURNAL rows do not consume busy limits.
7. CalDAV `sync-collection` coalesces change rows before loading requested
   `calendar-data`, avoiding duplicate ICS reads for superseded object changes.
8. CardDAV `sync-collection` coalesces contact changes before loading requested
   `address-data`, avoiding duplicate vCard reads and limit pressure.
9. IMAP mailbox status aggregation includes active messages awaiting lazy UID
   assignment when predicting `UIDNEXT` and `HIGHESTMODSEQ`.
10. IMAP message listings now emit explicit mailbox-relative sequence numbers
    after lazy UID assignment, including `AfterUID` partial listings.
11. POP3 `RETR` now reports the same mailbox size used by `LIST`/`STAT`,
    keeping message octet counts consistent within a maildrop snapshot.

---

## Module × RFC Compliance Map

| Module | Key Standards |
|---|---|
| SMTP receive (edge MTA) | RFC 5321, RFC 5322, RFC 2045–2049, RFC 6531/6532 |
| SMTP submission (outbound MTA) | RFC 5321, RFC 6409, RFC 4954 (AUTH) |
| SMTP delivery (outbound transport) | RFC 5321, RFC 7505 (null MX), RFC 3461/3464 (DSN) |
| SMTP relay / smarthost gateway | RFC 5321 (**implemented**) |
| DKIM signing | RFC 6376 |
| SPF | RFC 7208 |
| DMARC | RFC 7489 |
| IMAP | RFC 9051 (IMAP4rev2), RFC 3501 (IMAP4rev1) |
| POP3 | RFC 1939, RFC 2449 (CAPA), RFC 2595 (STLS), RFC 1734 (AUTH) |
| CalDAV | RFC 4791, RFC 5545 (iCalendar), RFC 6638, RFC 7809 (timezone) |
| iMIP scheduling | RFC 6047 |
| CardDAV | RFC 6352, RFC 6350 (vCard 4.0), RFC 2426 (vCard 3.0) |
| Drive WebDAV | RFC 4918, RFC 3744 (ACL), RFC 4331 (quota) |
| LDAP Gateway | RFC 4511, RFC 4512, RFC 4519 |
| SCIM 2.0 | RFC 7642, RFC 7643, RFC 7644 |
| SAML 2.0 | OASIS SAML 2.0 Core |
| OIDC | OpenID Connect Core 1.0, RFC 7636 (PKCE) |
| Milter (spam filter hook) | sendmail milter v2/v6 protocol |
| DNSBL | RFC 5782 |
| DNS autodiscovery | RFC 6764 (Well-Known URIs, DNS SRV) |
| DSN / bounce | RFC 3461, RFC 3464, RFC 5321 §4.5.5 (VERP) |
| Push notifications (Web) | RFC 8030 |
| TLS (all protocols) | RFC 8446 (TLS 1.3), RFC 5246 (TLS 1.2 minimum) |
| 2FA / TOTP | RFC 6238 (TOTP), RFC 4226 (HOTP) |
| JWT auth | RFC 7519 |
| Open API / API key auth | Bearer token + CIDR allowlist (domain_api_keys) |
| Real-time config SSE | Server-Sent Events (HTML5 EventSource) |
- Built-in spam filtering and pattern filtering; SMTP core should keep only pluggable boundaries and optional external relay adapters.

---

## Phase 8: Admin Console & Enterprise Features

Target outcome:

> 엔터프라이즈급 Admin Console으로 SaaS/On-Premises 모두 지원하고, AWS급의 강력한 관리, 모니터링, 감사 기능을 제공한다.

### 8-A. Core Admin Console (다중테넌시, RBAC, Identity Provider 추상화)

1. Admin Console은 SaaS 모드(multitenancy)와 On-Premises 모드 동시 지원.
2. System Admin (전역) vs Domain Admin (회사/도메인별) 계층.
3. 7개 내장 역할: System Admin, Domain Admin, Security Officer, HR Officer, Monitoring Officer, Auditor, Support Specialist.
4. 커스텀 역할: Domain Admin이 추가 역할 정의 및 세밀한 권한 설정 (resource × action × scope).
5. Identity Provider 추상화: Database Only, LDAP/Active Directory, Azure AD, External RDBMS 플러그인식 지원.
6. Database Mode (기본): gogomail 자체 DB로 사용자/조직도 관리.
7. LDAP Mode: 외부 LDAP/AD와 동기화, 주기적 또는 수동 sync, 속성 매핑.
8. Azure AD Mode: Microsoft Azure AD 연동, 사용자 자동 프로비저닝.
9. External RDBMS Mode: 외부 HR DB의 독자 스키마에서 SQL 쿼리로 사용자/조직도 읽기, 필드 매핑.
10. Identity Mode 전환: Database ↔ LDAP ↔ Azure ↔ RDBMS 변경 가능, 전환 전 미리보기.
11. Organization Management: 계층적 부서/팀 구조, 드래그앤드롭 이동, 부서별 관리자 위임.
12. User Management: 생성/수정/삭제, 비밀번호 초기화, 2FA 관리, 일괄 import (CSV).
13. Role Management: 내장 역할은 읽기 전용, 커스텀 역할은 CRUD.
14. Permission Delegation: Domain Admin이 다른 사용자에게 역할 부여, 임시 위임 지원.
15. Admin Session: JWT 기반 인증, secure token storage, session timeout.

### 8-B. Monitoring & Analytics

16. Dashboard: System Admin은 전체 통계, Domain Admin은 자신 회사만, 실시간 메일 트래픽, 트렌드. Console UI now uses compact shared metric cards for mail volume, user activity, and storage.
17. Mail Log: 발신/수신 로그, 검색 (sender, recipient, date range), 상세 조회 (headers, Authentication-Results).
18. Spam Monitoring (활성화시): 일일 스팸 차단율, 상위 규칙, false positive 신고.
19. Login Audit: 로그인 이력 (시간, IP, 디바이스), 성공/실패, 의심 활동 감지.
20. User Activity Stats: DAU/WAU/MAU, 활동도별 분포, 상위 활동 사용자.
21. Storage Stats: 전체 사용률, 사용자별 분포, 예상 소진 날짜, 1년 이상 된 메일 크기.
22. API Metering (Domain Admin 보기): 월별 호출량, 일별 추이, 엔드포인트별 상위, 오류율, 응답시간.
23. Statistics Export: CSV (모든 통계), PDF (월간 리포트), NDJSON (대용량 분석).
24. Stats Cache: 대시보드 통계를 사전 계산해 성능 최적화, 배치 롤업 (daily, monthly).

### 8-C. Audit & Compliance

25. Audit Levels (Domain Admin 선택):
    - Level 1 (기본): Admin 행위만 (user CRUD, policy change).
    - Level 2 (권장): Level 1 + 로그인/보안 이벤트, API 오류.
    - Level 3 (규제 필수): Level 2 + 모든 사용자 행위 (mail read/delete, attachment download).
26. Audit Log: timestamp, actor, action, resource, changes (before/after), ip, user_agent, status.
27. Log Retention: 최근 30일 온라인, 30-90일 압축 아카이브, 90일+ 자동 삭제 또는 cold storage.
28. Data Masking (Level 3): 메일 내용 저장 안함, 수신자 이메일 마스킹 (선택), API request body 민감 정보 제외.
29. Audit Query UI: 필터 (기간, 사용자, 액션), 정렬, 상세 조회, CSV/JSON 내보내기.
30. Permission-based Log Access: System Admin 전체, Domain Admin 자신 회사만, Security Officer 보안 이벤트만, Auditor 감시 로그만.

### 8-D. UI/UX & Settings

31. Admin Console UI: AWS Console 스타일 (정보 밀도, 다크 테마, 고대비), 테이블 컴팩트, 여백 최소화, 하나의 페이지에 많은 데이터.
32. Navigation: 좌측 접이식 사이드바 (220px), 상단 최소 네비게이션, 커맨드 팔레트 (Cmd+K), 빵부스러기.
33. Table Interactions: 정렬, 필터, 검색, 다중 선택, 우클릭 컨텍스트 메뉴, 인라인 편집 (더블클릭), 열 재정렬 (드래그).
34. Forms: 인라인 폼 선호, Modal 최소화 (side panel), 저장/취소 always visible.
35. Charts: Recharts 기반, 호버 시 상세 정보, 범위 선택, 축소/확대.
36. Accessibility: 키보드 네비게이션, 스크린 리더 지원, 높은 대비.
37. Domain Settings: TLS 정책, Quota (사용자당 스토리지), IP 화이트리스트, 2FA 요구, 세션 타임아웃, 비밀번호 정책.
38. API Settings: API Key 관리 (생성, 회전, 삭제), Rate Limit 설정, CIDR Allowlist, OpenAPI 문서 링크.
39. Alerts & Notifications: 임계값 기반 자동 알림 (스토리지 > 80%, 로그인 실패 > 10회/시간, API 오류율 > 5%).
40. Alert Channels: 이메일, 웹훅, 대시보드 팝업.

### 8-E. Backend APIs

41. Auth API: login, logout, me, refresh-token.
42. User API: CRUD, bulk-import, reset-password, assign-role.
43. Organization API: unit CRUD, hierarchy, member assign/remove.
44. Identity Config API: identity-mode select, LDAP/Azure/RDBMS config CRUD, test-connection, validate-query, sync.
45. Log APIs: mail, login, audit, spam logs with filters.
46. Stats APIs: dashboard, mail-volume, users, storage, api-usage.
47. Settings APIs: domain-settings, api-settings, audit-policy.
48. Role APIs: list (builtin + custom), CRUD (custom only), assign/revoke.

### 8-F. Data Model

49. Admin roles: `admin_role_definitions` (builtin + custom), `admin_role_permissions` (matrix), `admin_user_roles` (assignment).
50. Audit: `audit_logs` (admin actions), `login_audit_logs` (login history), `audit_policy_configs` (level, retention, masking).
51. Identity: `domain_identity_config` (mode, sync settings), `ldap_sync_configs`, `external_rdbms_configs`.
52. Stats: `api_usage_daily` (metering), `admin_stats_cache` (dashboard cache).

---

### Phase 8 구현 로드맵

| TASK | 제목 | 설명 |
|------|------|------|
| TASK-063 | Admin Console Architecture | Schema + RBAC + Custom Roles |
| TASK-064 | Admin Auth & Session | JWT, login, refresh-token |
| TASK-065 | User Management CRUD | Create/Read/Update/Delete users |
| TASK-066 | Organization Management | Unit CRUD, hierarchy, members |
| TASK-067 | Audit Logs (Level 1+2) | Admin actions + security events |
| TASK-068 | Identity Provider Abstraction | Database/LDAP/Azure/RDBMS plugin |
| TASK-069 | Database Identity Mode | Default implementation |
| TASK-070 | LDAP Identity Config & Sync | LDAP server config, test, sync |
| TASK-071 | LDAP Sync UI & Logs | Admin console LDAP management |
| TASK-072 | External RDBMS Config & Sync | HR DB connection, query, mapping |
| TASK-073 | External RDBMS Sync UI & Logs | Admin console RDBMS management |
| TASK-074 | Mail Log Queries & UI | Send/receive logs, search, detail |
| TASK-075 | Login/Security Audit Logs | Login history, suspicious activity |
| TASK-076 | Statistics & Dashboard | Mail volume, user activity, storage |
| TASK-077 | API Metering | Daily rollup, per-domain visibility |
| TASK-078 | Dashboard UI | System/Domain admin views |
| TASK-079 | Audit Policy Config UI | Company audit-policy settings, retention, masking |
| TASK-080 | Export & Reports | CSV, PDF, NDJSON |
| TASK-081 | Role Management UI | Builtin roles view, company-scoped custom role create |
| TASK-082 | Domain Settings UI | TLS, quota, IP whitelist, 2FA, hook-backed save flow |
| TASK-083 | API Settings UI | API key management, rate limit, CIDR allowlist |
| TASK-084 | Alerts & Notifications | Threshold-based alerts, channels |
| TASK-085 | Admin Console Frontend (Phase 1) | Login, dashboard, user list |
| TASK-086 | Admin Console Frontend (Phase 2) | Organization, settings pages |
| TASK-087 | Admin Console Frontend (Phase 3) | Logs, analytics, exports |
| TASK-088 | Mail Infrastructure Hardening | Connection pooling, pipelining, retry policy, performance metrics |
| TASK-089 | Protocol Gateway Hardening | IMAP/CalDAV/CardDAV buffer pooling, metrics, health checks, graceful degradation |
| TASK-090 | Message Storage & Delivery Optimization | Query/index optimization, bulk delivery batching, message metadata caching |

1703. IMAP bulk message move/delete stale UID rows are now covered:
     `BulkMoveMessages` removes source mailbox UID rows before destination
     reassignment, and `BulkDeleteMessages` removes assigned UID rows before
     rejecting later reallocation.
1704. IMAP bulk thread move/delete stale UID rows are now covered:
     `BulkMoveThreads` removes source mailbox UID rows for every moved thread
     message before destination reassignment, and `BulkDeleteThreads` removes
     assigned UID rows for every deleted thread message before rejecting later
     reallocation.
1705. IMAP bulk restore fresh UID behavior is now covered:
     `BulkRestoreMessages` and `BulkRestoreThreads` restore deleted messages
     without reusing expunged UID values; subsequent IMAP UID assignment must
     allocate values above the previous mailbox max UID.
1706. IMAP single-message restore fresh UID behavior is now covered:
     `RestoreMessage` restores a deleted message without reusing the expunged
     UID value; subsequent IMAP UID assignment must allocate above the deleted
     message's previous UID and keep one message-specific UID row.
1707. Mailbox delete with deleted messages now returns a clean not-empty error:
     `DeleteFolder` checks for any remaining message row, not only active
     messages, before deleting a folder so soft-deleted messages cannot leak a
     database foreign-key error after IMAP UID row cleanup.
1708. IMAP mailbox state cascade cleanup is now covered:
     deleting an empty user folder removes its `imap_mailbox_state` row and
     `GetIMAPMailbox` no longer exposes the deleted mailbox after cleanup.
1709. IMAP deleted-folder subscription behavior is now covered:
     a subscribed user folder deleted through `DeleteFolder` remains visible as
     a retained non-existing subscription name, and the retained name can still
     be used to unsubscribe.
1710. IMAP subscription name normalization now trims mailbox name/id inputs:
      spaced names like `" INBOX "` resolve to the existing Inbox rather than a
      missing retained subscription, and the same trim rule applies to
      unsubscribe and canonical subscription keys.
1711. IMAP retained subscription names are now covered for case-insensitive
      unsubscribe: a missing mailbox retained as `Retired` can be removed with
      `retired`, and the retained LSUB entry disappears from subscription
      listings afterward.
1712. IMAP existing mailbox subscriptions are now covered for case-insensitive
      unsubscribe: an `INBOX` subscription can be removed with `inbox`, and the
      existing mailbox LSUB entry disappears from subscription listings
      afterward.
1713. IMAP duplicate-casing retained subscriptions are now covered:
      repeated SUBSCRIBE for a retained missing mailbox with different casing
      updates the same canonical subscription row and returns one retained LSUB
      entry using the latest display name.
1714. CalDAV/CardDAV collection creation now has active principal policy
      coverage: `CreateCalendar` and `CreateAddressBook` reject disabled user,
      domain, and company state instead of creating DAV collections for inactive
      principals.
1715. CalDAV Basic auth now has parity coverage with CardDAV:
      unauthorized failures expose `WWWAuthenticate()` with `Basic realm="CalDAV"`,
      and trusted-proxy `X-Forwarded-Proto` handling accepts uppercase/whitespace
      HTTPS tokens.
1716. DAV Basic auth repository active policy is now covered:
      `AuthenticatePlain`, used by CalDAV/CardDAV Basic auth, rejects suspended
      users and domains after proving the same credentials work while active.
1717. DAV Basic auth must-change-password propagation is now covered:
      `AuthenticatePlain` returns the DB `must_change_password` flag alongside
      user/domain IDs so CalDAV/CardDAV resolvers can reject password-change-
      required users.
1718. SMTP submission now enforces must-change-password policy:
      `AUTH PLAIN` rejects authenticated users flagged `MustChangePassword`,
      leaves the session unauthenticated, records a rejected auth metric, and
      continues to require AUTH before `MAIL FROM`.
1719. SMTP submission must-change-password event isolation is now covered:
      rejected password-change-required auth attempts do not emit hook events,
      preventing downstream audit/logging extensions from seeing a false
      `StageAuthenticated` success.
1720. SMTP submission auth hook failure isolation is now fixed and covered:
      the session user is stored only after `StageAuthenticated` hooks succeed,
      and hook failure leaves the session unauthenticated with `MAIL FROM`
      still requiring AUTH.
1721. SMTP submission auth hook failure metrics are now covered:
      failed `StageAuthenticated` hooks record rejected auth metrics and do not
      produce accepted auth metrics.
1722. SMTP submission invalid credential event isolation is now covered:
      invalid AUTH credentials emit no authenticated hook events while still
      recording a rejected auth metric.
1723. SMTP submission malformed AUTH PLAIN payload isolation is now covered:
      parser-level failures leave the session unauthenticated, emit no
      authenticated hook events, and keep `MAIL FROM` behind AUTH.
1724. SMTP submission unsupported auth mechanism handling is now covered:
      mechanisms such as `LOGIN` return `ErrAuthUnsupported` with no SASL
      server and without hooks, metrics, session authentication, or `MAIL FROM`
      access.
1725. SMTP submission repeated AUTH side-effect isolation is now covered:
      repeated AUTH after successful authentication does not replace or clear
      the existing user, does not add hooks or metrics, and leaves `MAIL FROM`
      available for the original authenticated user.
1726. SMTP submission repeated AUTH transaction state is now covered:
      repeated AUTH after `MAIL FROM` is rejected without clearing the active
      envelope sender, and the transaction can continue through `RCPT` and
      `DATA`.
1727. SMTP submission unsupported AUTH transaction state is now covered:
      unsupported AUTH after `MAIL FROM` is rejected without clearing the active
      envelope sender, and the transaction can continue through `RCPT` and
      `DATA`.
1728. SMTP submission Logout domain policy reset is now covered: successful
      `AUTH` and `Logout` clear authenticated-user domain policy cache, so a
      same-session reauthentication as another domain user cannot inherit the
      previous domain's message-size policy or DSN options.
1729. SMTP submission explicit Reset DSN isolation is now covered: `Reset`
      clears the active envelope plus RFC 3461 `RET`, `ENVID`, `NOTIFY`, and
      `ORCPT` state while preserving authentication for the next submitted
      transaction.
1730. TASK-083 API Settings UI is complete: the console uses admin-proxy hooks
      backed by generated OpenAPI types for domain API settings and API key
      management, including the backend-required `created_by` field on key
      creation.
1731. Admin console capability discovery now exposes integration truth states
      (`available`, `placeholder`, `planned`) for LDAP read, LDAP sync, and
      organization sync, and the console organization surface renders
      placeholder status instead of presenting external sync as available.
1732. Admin maintainability hardening split the oversized HTTP admin service
      contract into domain facets, delegated admin route registration through
      section functions, moved company repository methods into
      `internal/maildb/admin_company.go`, and extracted IMAP `STATUS` handling
      into a dedicated server helper without changing wire responses.
1733. Drive upload-session chunk storage now carries `Content-Range` from HTTP
      into the service layer, assembles sequential chunks into one backend
      object, enforces the previous received size under the locked metadata
      row, and records whole-object checksum metadata for finalized Drive files.
1734. OWASP-oriented security hardening now covers backend outbound URL guards,
      production bootstrap-admin blocking, webhook secret redaction, webmail
      HTML/image proxy XSS/SSRF controls, patched Go/PostCSS dependency pins,
      and console/webmail proxy header plus same-origin protections.
1735. Enterprise security posture tightened cookie-backed mutation provenance,
      production `__Host-` auth cookies, server-only backend URL handling, and
      production CSP/security headers for console and webmail.
1736. Company/domain security governance now exposes explicit policy endpoints
      for posture presets and controlled webhook private-network exceptions,
      keeping enterprise defaults strict while allowing audited tenant-specific
      relaxation where operationally required.
1737. TASK-090 attachment upload session cleanup now batches stale-session expiry
      and quota release: expired sessions are marked with one typed UUID-array
      update, and user/domain/company quota ledgers are decremented through
      aggregated CTEs instead of one write sequence per expired session.
1738. TASK-090 message storage GC lookup now removes correlated per-candidate
      reference counts from bulk delete and IMAP EXPUNGE paths.  Both queries
      build target storage paths once, group `messages` references by path, and
      return only paths whose grouped reference count is one.
1739. TASK-090 legacy stale attachment upload cleanup now uses typed-array batch
      updates and the shared aggregated quota-decrement CTE, matching upload
      session cleanup and avoiding one attachment update plus three quota
      writes per stale attachment.
1740. TASK-090 thread list index coverage now includes folder-scoped and
      unscoped partial expression indexes for active messages keyed by
      `COALESCE(thread_id, id)` and the thread-list message timestamp
      expression, supporting the existing aggregate query without changing
      pagination semantics.
1741. TASK-090 delivery attempt preparation now pre-indexes DSN recipient options
      by normalized address for attempt records and exhausted-event payloads,
      removing repeated linear scans across DSN metadata for large recipient
      batches while preserving first-match duplicate handling.
1742. TASK-090 retry scheduling dedupe-key generation now avoids `fmt.Sprint`
      and `strings.Join` intermediate strings, writes into a pre-sized builder,
      and tracks 1k/10k-recipient benchmark cases for large batch retries.
1743. Storage repository projection tightening now removes remaining
      `SELECT * FROM updated/inserted` CTE reads from Drive rename, move, and
      upload-session creation queries, with a regression test guarding explicit
      result shapes.
1744. IMAP unsupported command responses now use client-facing unsupported
      wording for unknown commands and UID subcommands, avoiding
      implementation-status language while preserving pre-auth validation
      ordering.
1745. CardDAV/CalDAV REPORT and directory principal-kind fallback errors now
      use client-facing unsupported wording instead of implementation-status
      language, with regression tests preventing `not implemented` leakage on
      DAV unsupported REPORT paths.
1746. Organization sync no longer reports false success when no external HR/LDAP
      sync adapter is configured: the no-op adapter returns a typed
      not-configured error, manual sync returns HTTP 501, failed sync logs retain
      the reason, and the batch worker skips the hourly sync job until a real
      adapter is wired.
1747. LDAP sync requests now expose external LDAP import/sync placeholder status
      as an explicit unavailable operation: admin sync runs are marked failed
      with `ErrSyncNotConfigured`, and `POST /admin/v1/domains/{id}/ldap/sync`
      returns HTTP 501 instead of a pending response that could be mistaken for
      live synchronization. Direct LDAP provider read/sync methods also return
      typed unavailable errors instead of implementation placeholder wording,
      and the console organization page now presents localized configuration
      status labels instead of internal placeholder/not-available phrasing.
1748. RDBMS sync admin routes are now registered instead of falling through to
      404, and requests use the same explicit unavailable contract when no
      external provider is configured: admin sync runs are marked failed with
      typed `ErrSyncNotConfigured`, and
      `POST /admin/v1/domains/{id}/rdbms/sync` returns HTTP 501 instead of
      presenting a false pending run. Membership sync also returns typed
      `ErrMembershipSyncUnsupported` instead of a successful no-op while the
      provider schema has no membership query.
1749. TASK-090 message detail reads now skip attachment-list hydration when the
      message row reports `HasAttachment=false`, removing a repository round
      trip from the common attachment-free detail path.  `BenchmarkGetMessageBodyCache`
      now tracks parsed-body miss/hit costs, with the current sample at miss
      `~7.83 us/op` and hit `~933.6 ns/op`.
1750. Partial delivery attempt recording now reuses one DSN recipient option map
      across delivered and failed recipient attempts, removing an O(n²) map
      rebuild pattern from high-volume partial failure recording.
1751. Launch documentation is refreshed across README, README.ko, Docker,
      webmail, console, and backend release readiness docs so recent
      performance, storage, backup/restore, push/webhook, system email, API
      usage, and frontend public-origin environment variables are visible in
      operator-facing setup surfaces.
1752. BIMI validation no longer reports VMC verification from URL presence
      alone.  Until full VMC certificate-chain validation exists, `ValidateAndFetch`
      returns `vmcVerified=false`, and the logo cache now records the actual
      SHA-256 body digest for fetched logo bytes.
1753. Webmail preferences saves now merge with the existing server-side
      preference document before replacing `settings->webmail`, preventing
      signature, filter-rule, and general-settings saves from erasing sibling
      keys.  Both settings surfaces hydrate server filter rules, signatures,
      and general settings back into local caches used by the message-list and
      compose paths, and the filter UI copy now reflects that rules are active
      and server-synced.
1754. Webmail calendar recurring-event edits now expose only the currently
      supported whole-series edit behavior.  The modal no longer offers a
      single-occurrence option that failed after submit, so users see the
      actual update scope before saving.
1755. Webmail Web Push registration now decodes the VAPID public key into the
      standard `Uint8Array` `applicationServerKey` before subscribing, and the
      mail view only registers the service worker when notifications were
      already allowed.  Permission prompts remain tied to the explicit Settings
      opt-in action.
1756. Webmail quick reply templates now live in server preferences rather than
      only in browser `localStorage`.  Settings, compose, and Spotlight search
      share a normalized template cache with stable ids, preserving offline
      lookup while making templates portable across browsers and devices.
1757. Admin outbox event listing now builds dynamic sargable WHERE clauses for
      topic, partition key, status, and created-at filters.  The query no
      longer uses `NULLIF(...) OR ...` predicates, keeping large outbox
      inspection paths aligned with the queue indexes.
1758. Admin delivery-attempt listing now uses dynamic sargable predicates for
      status, attempted-at, recipient domain, message id, farm, and sender
      filters.  This keeps production delivery-history inspection from routing
      selective filters through optional `OR` predicates on the hot table.
1759. Delivery-attempt stats and exhausted-attempt views now reuse the same
      sargable predicate builder as the delivery-attempt list, so every
      high-volume delivery-history admin read path avoids `NULLIF(...) OR ...`
      filters and keeps selective queries index-friendly.
1760. Push-notification attempt list and stats queries now build only the
      predicates requested by the operator, including message, user, platform,
      device, provider, and since filters.  This removes optional `OR`
      predicates from Web Push operations reads before production attempt
      history grows.
1761. Domain DNS check history listing now uses dynamic sargable predicates for
      status and checked-at filters, so domain operations pages can inspect DNS
      verification history without routing selective filters through optional
      `OR` predicates.
1762. API usage daily and monthly aggregate listings now share a dynamic
      sargable query builder for tenant, company, domain, user, API key,
      principal, auth source, method, route, status, and time-window filters.
      This keeps SaaS usage and billing analytics queries index-friendly as
      aggregate tables grow.
1763. API usage export batch listing now uses dynamic sargable predicates for
      tenant, principal, status, and export-window filters, keeping saved-batch
      discovery for billing handoff and retention checks index-friendly as
      export history grows.
1764. Directory alias listing now uses dynamic optional predicates for domain,
      target-kind, target-id, query, and active-status filters, avoiding broad
      optional `OR` guards on address-book alias operations as tenant directory
      data grows.
1765. Directory organization-tree listing now emits the domain predicate only
      when requested, avoiding broad optional `OR` guards on organization
      navigation reads while preserving the same active-unit projection.
1766. Directory alias and user-email exact lookup queries now add active-status
      predicates only when `ActiveOnly` is requested, avoiding boolean optional
      `OR` guards in hot address resolution paths while preserving inactive
      lookup behavior.
1767. Directory principal ID lookup queries for users, organizations, groups,
      and resources now add active-status predicates only when `ActiveOnly` is
      requested, avoiding boolean optional `OR` guards in delegation and
      membership validation paths.
1768. Directory group-membership listing now uses dynamic optional predicates
      for group, member-kind, member-id, role, and active-status filters,
      avoiding broad optional `OR` guards on large membership operations.
1769. Directory direct and effective group-membership checks now add
      active-status predicates only when `ActiveOnly` is requested, removing
      boolean optional `OR` guards from validation hot paths.
1770. Directory direct delegation checks now add active-status predicates only
      when `ActiveOnly` is requested, removing boolean optional `OR` guards from
      delegation validation paths.
1771. Directory delegation listing now uses dynamic optional predicates for
      owner, delegate, scope, role, and active-status filters, avoiding broad
      optional `OR` guards on large delegation operations.
1772. Directory principal search now emits only requested principal-kind UNION
      branches and dynamic optional predicates for domain, organization, query,
      and active-status filters, avoiding broad optional `OR` guards and
      unnecessary branch scans on large address books.
1773. Drive node listing now uses dynamic optional predicates for name search,
      node type, and parent scope, avoiding broad optional `OR` and `NULLIF`
      guards on large folder and whole-drive listing operations.
