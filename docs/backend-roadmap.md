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
182. PostgreSQL outbox integration tests now verify `available_at` claiming, retry-to-pending behavior, retry exhaustion, and UTF-8-safe failure diagnostics against the migrated schema.
183. Trusted relay tests now explicitly cover empty relay policy defaults and malformed remote address rejection, tightening final inbound relay boundary verification.
184. Same-connection SMTP soak coverage now verifies DSN `RET`/`ENVID`/`NOTIFY`/`ORCPT` state does not leak into a later transaction on the same TCP session.
185. SMTP backend release operations now have a dedicated runbook for PostgreSQL verification, same-connection soak, STARTTLS, SMTPS, trusted relay policy, and outbound DSN/bounce smoke checks.
186. DSN queue and bounce-event trust boundaries now reject malformed RFC 3461 xtext metadata before outbound SMTP command generation or RFC 3464 report composition.
187. Attachment storage-path contracts now reject unsafe caller-provided paths and sanitize generated attachment object path segments before writing to storage.
188. SMTP release verification now covers `NOTIFY=NEVER` over a real TCP SMTP session and controlled outbound SMTP sink recipient-classification behavior.
189. Backend API contract metadata is centralized in code and guarded against OpenAPI drift, keeping service info and generated client contracts aligned.
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
221. API metering can emit durable `api.usage` events through the generic outbox on topic `api.event`, keeping request handling fail-open while giving future aggregation workers a persistent event source.
222. Quota reconciliation corrections can be explicitly applied by operators through `POST /admin/v1/quota-reconciliation/corrections`; corrections lock the affected quota hierarchy and set counters from message/attachment source rows.
223. Domain outbound policy includes `max_attachment_bytes`, and Mail API attachment reservation/direct upload enforce it before quota reservation or object storage writes.
224. Attachment scanning has a disabled-by-default hook adapter outside SMTP core, allowing metadata-first attachment scanners to attach at the parsed stage without adding spam or vendor logic to protocol paths.
225. API metering aggregation has a first worker/read-model boundary: `api-metering-worker` consumes `api.usage` events from `api.event`, upserts `api_usage_daily`, and Admin API exposes `GET /admin/v1/api-usage/daily`.
226. ADR 0004 captures the API metering aggregation boundary: HTTP remains fail-open, aggregation is disabled by default, daily aggregates are operational read models, and billing-grade idempotency is deferred.
227. Search results now have opt-in relevance ordering, rank scores, and bounded headline snippets through `sort=relevance`, `include_rank=true`, and `include_highlights=true`, while the default response remains date sorted.
228. `internal/imapgw` establishes a dependency-light IMAP gateway boundary with native DTOs/interfaces, UID-oriented mailbox state, RFC 3501 system flag mapping, mailbox helpers, and explicit deferral of `\Deleted`/EXPUNGE semantics.
229. ADR 0005 records that IMAP will be a separate gateway over stable mailbox/message interfaces rather than protocol code embedded into Mail API, SMTP, or `maildb` internals.
230. Push notification enqueue has a first async worker boundary: `push-notification-worker` consumes committed `mail.stored` events, routes them through `internal/pushnotify`, and supports a disabled-by-default `slog` sink while FCM/APNs delivery remains adapter work.
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
767. IMAP `SEARCH` and `UID SEARCH` now support `RECENT`, `OLD`, and `NEW`,
     returning no recent/new matches while durable recent-state semantics remain
     deferred and treating active messages as old.
768. IMAP `SEARCH` and `UID SEARCH` now support `KEYWORD` and `UNKEYWORD`
     criteria with validated keyword atoms, returning no custom-keyword matches
     until durable user keyword storage exists and treating active messages as
     unkeyworded.
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
     `docs/storage-backends.md`, and `deploy/docker-compose.dev.yml` includes a
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
     `BODY[]<+12.34>` before fetch processing.
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
     validate RFC-shaped header field names instead of trimming stray brackets,
     rejecting malformed requests such as `HEADER.FIELDS ([Subject])`.
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
     compatibility without weakening syntax guardrails.
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
      `MKCALENDAR` again only after those semantics exist; human-readable slug
      aliases remain future compatibility work.
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
      returns `201 Created` with `Location`.
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
1139. CardDAV `addressbook-query` now accepts RFC 6352 `Depth: infinity`
      requests, treating them with the same flat address-book scan semantics as
      `Depth: 1` while preserving `Depth: 0` as collection-scoped.
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
      shared by `OPTIONS` and 405 responses, so future constants such as `MOVE`
      do not leak into native-client capability discovery before handler
      semantics exist.
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
      no-match results.
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
1196. CalDAV iCalendar object validation now accepts the common recurring-event
      storage shape of one VEVENT master plus same-UID `RECURRENCE-ID`
      detached override VEVENTs. Calendar-query and free-busy evaluation now
      scan all VEVENTs in a stored object and suppress the replaced master
      occurrence when an override exists, improving RFC 5545 native-client
      compatibility without introducing a product-specific event model.
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
      rejecting bare one-digit dates before backend append dispatch.
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
  cases and native-client scheduling behavior
- Directory/Identity expansion for delegated relationships, effective
  resource booking policy beyond the initial principal tables, resolver, alias
  lookup, bounded membership expansion, company-scoped delegation relationship
  checks, and bounded group-backed effective delegation reads
- Contacts/CardDAV broader vCard
  compatibility, and native-client compatibility beyond the experimental
  runtime, internal discovery/REPORT/object I/O, path/href, storage metadata,
  repository, bounded vCard 3.0/4.0 validation, REPORT parsing, and multistatus
  response boundaries
- Notification & Sync boundary for domain events, reminders, devices, quiet
  hours, per-device policy, and delta fan-out
- Vendor push notification delivery adapters
- Built-in spam filtering and pattern filtering; SMTP core should keep only pluggable boundaries and optional external relay adapters.
