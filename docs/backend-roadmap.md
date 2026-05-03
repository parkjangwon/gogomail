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

## Deferred until backend contracts stabilize

- Next.js shell/webmail/admin apps
- Kafka
- OpenSearch
- etcd
- Vault
- IMAP
- Push notifications
