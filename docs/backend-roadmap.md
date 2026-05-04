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

## Deferred until backend contracts stabilize

- Next.js shell/webmail/admin apps
- Kafka
- OpenSearch as the default/mandatory search backend
- etcd
- Vault
- IMAP
- Vendor push notification delivery adapters
- Built-in spam filtering and pattern filtering; SMTP core should keep only pluggable boundaries and optional external relay adapters.
