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
207. Mail API exposes a small-deployment search endpoint backed by Postgres FTS over message metadata and draft text, while full received-body indexing remains reserved for the future index worker/OpenSearch boundary.
208. Quota roadmap is hierarchical and SaaS-oriented: company owns the contracted storage pool, domains receive allocations within the company pool, and users receive unified personal storage usable across mailbox, attachments, future Drive, and other user-owned storage. Domain default user quota changes should update default-following users while preserving custom user overrides.

204. Mailbox quota is enforced atomically at SMTP receive, Submission MTA, and Mail API delete flows using a PostgreSQL row-level lock on the user row; the SMTP layer returns RFC-correct 452 4.2.2 when the mailbox is full.
205. Per-domain inbound SMTP policy (max recipients per message, max message bytes, inbound mode) is enforced at the SMTP receive and Submission boundaries without leaking policy logic into protocol core; the `DomainPolicyLookup` interface keeps the SMTP engine decoupled from `maildb`.
206. DKIM key DNS verification workflow: operators can trigger `POST /admin/v1/dkim-keys/{id}/verify-dns`, which runs a targeted DNS lookup, persists the result to `domain_dns_checks`, and sets `dns_verified_at` on the key when the record matches.
207. Delivery route runtime counters (`RouteCounters`) track per-pool delivered/failed/retried/exhausted since process start and are exposed via `GET /admin/v1/delivery-routes/counters` when configured.
208. Retry exhaustion hook: when all delivery retries for a message are exhausted, an `exhausted` status row is written to `delivery_attempts` and a `mail.delivery_exhausted` outbox event is emitted; `GET /admin/v1/delivery-attempts/exhausted` lists these for operator triage.
209. SMTPUTF8 declared correctly on outbound MAIL FROM whenever the sender or any recipient contains non-ASCII bytes and the remote server advertises SMTPUTF8, complying with RFC 6531 Section 3.3; also fixes a typo where RCPT TO responses were checked against status 25 instead of 250.
210. DMARC reject policy enforcement is opt-in via `ReceiverOptions.DMARCEnforce`; when enabled, messages failing DMARC with `p=reject` are refused with SMTP 550 5.7.1, while quarantine policy messages are delivered with the policy visible in the Authentication-Results header.
211. Admin API now exposes per-domain aggregate statistics (`GET /admin/v1/domains/{id}/stats`): active/total user counts, inbound/outbound/active message counts, storage used/limit bytes, 24-hour delivery outcomes, and suppression list size.
212. OpenAPI schemas expanded: DKIMKey now includes `dns_verified_at` and `status` enum; DeliveryAttempt includes `status` enum with `exhausted`; DKIM DNS verify response references typed `DKIMKeyDNSVerification` and `DNSRecordCheck` schemas; `ExhaustedAttemptsEnvelope` added.
213. Hierarchical quota ledger first implementation: Admin API exposes company quota read/update, user records track `quota_source=default|custom`, domain quota updates can propagate `default_user_quota` to default-following users, and mail write/delete transactions atomically adjust company, domain, and user ledgers.
214. API metering is a planned platform capability: collect company/domain/user/api-key, route, method, status, latency, response size, and timestamp asynchronously for SaaS billing, rate-limit policy, abuse detection, and operations dashboards without adding synchronous hot-path writes.
215. Attachment uploads now participate in the hierarchical quota ledger: upload metadata creation reserves bytes from company/domain/user counters, stale upload cleanup releases those bytes, stored objects are deleted after DB cleanup, and Mail API quota exhaustion is surfaced as HTTP 507 `insufficient_storage`.

## Deferred until backend contracts stabilize

- Next.js shell/webmail/admin apps
- Kafka
- OpenSearch
- etcd
- Vault
- IMAP
- Push notifications
- Built-in spam filtering and pattern filtering; SMTP core should keep only pluggable boundaries and optional external relay adapters.
