# gogomail

<img width="1456" height="720" alt="1777874812592" src="https://github.com/user-attachments/assets/3e222678-51be-465f-b37d-58d2390ba40d" />

Go-first backend implementation for the gogomail webmail/mail-server platform.

## Current scope

This repository is currently in the backend platform hardening phase.

The SMTP engine is materially advanced, and current work is broadening into
tenant/domain operations, Admin API, Mail API contracts, delivery routing,
DNS/DKIM onboarding, quota/policy enforcement, and OpenAPI drift prevention.

The Next.js web apps will be added after the backend contracts stabilize and
after the user provides frontend-specific guidance.

## Agent handoff

Future coding agents should read these files before changing code:

- `AGENTS.md`
- `docs/CURRENT_STATUS.md`
- `docs/NEXT_STEPS.md`
- `docs/CHANGE_CHECKLIST.md`
- `docs/backend-roadmap.md`
- `docs/backend-api-contracts.md`
- `docs/backend-release-readiness.md`
- `docs/openapi.yaml`

Key architecture decisions:

- `docs/adr/0001-domain-is-tenant.md`
- `docs/adr/0002-smtp-core-is-not-spam-engine.md`
- `docs/adr/0003-company-domain-user-quota-pool.md`
- `docs/adr/0004-api-metering-aggregation-boundary.md`
- `docs/adr/0005-imap-gateway-boundary.md`

Guardrails:

- Implemented SMTP features must follow the relevant email RFCs.
- Do not advertise SMTP extensions before end-to-end semantics are implemented.
- Do not build a spam engine into SMTP core.
- Do not start frontend implementation without user guidance.
- Commit feature-sized changes and push after successful verification.

Agent work protocol:

1. Rebuild context from the handoff documents and recent git history.
2. Make feature-sized, reviewable changes.
3. Update status/roadmap/API/ADR documents when the change affects them.
4. Run the verification commands.
5. Commit and push to `origin/main`.
6. Report what changed, what passed, and whether the push completed.

## Backend modes

```bash
go run ./cmd/gogomail --mode=all-in-one
go run ./cmd/gogomail --mode=edge-mta
go run ./cmd/gogomail --mode=inbound-mta
go run ./cmd/gogomail --mode=outbound-mta
go run ./cmd/gogomail --mode=delivery-worker
go run ./cmd/gogomail --mode=search-index-worker
go run ./cmd/gogomail --mode=api-metering-worker
go run ./cmd/gogomail --mode=push-notification-worker
go run ./cmd/gogomail --mode=auth-server
go run ./cmd/gogomail --mode=mail-api
go run ./cmd/gogomail --mode=admin-api
```

`push-notification-worker` stays disabled until
`GOGOMAIL_PUSH_NOTIFICATION_BACKEND=slog` or
`GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webhook` is set. The worker consumes
`mail.stored`, resolves active user devices from PostgreSQL, and writes
candidate attempt rows before sink handoff. The `slog` backend logs bounded
notification candidates. The `webhook` backend POSTs raw-token targets and
attempt IDs to an external push gateway. First-party FCM/APNs/Web Push delivery
adapters are intentionally not enabled by default.

Webhook push handoff:

```bash
GOGOMAIL_PUSH_NOTIFICATION_BACKEND=webhook
GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_URL=https://push-gateway.example/send
GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TOKEN='optional-bearer-token'
GOGOMAIL_PUSH_NOTIFICATION_WEBHOOK_TIMEOUT=2s
```

Production webhook URLs must use HTTPS. Local development and private test
harnesses may use HTTP.

## Verify

```bash
go test ./...
```

Release-oriented local verification:

```bash
scripts/verify-backend-release.sh
```

Release-oriented PostgreSQL checks are opt-in because they need a disposable
database. They run migrations in a temporary schema and exercise draft send plus
outbox retry behavior against real SQL:

```bash
GOGOMAIL_TEST_DATABASE_URL='postgres://gogomail:gogomail@localhost:15432/gogomail?sslmode=disable' \
  go test ./internal/maildb ./internal/outbox
```

See `docs/smtp-release-runbook.md` for the SMTP backend release soak,
STARTTLS/SMTPS, trusted relay, and outbound DSN/bounce smoke checklist.
See `docs/api-usage-export-runbook.md` for the API usage export, handoff, and
retention-readiness operator flow.

## Local infrastructure

```bash
docker compose -f deploy/docker-compose.dev.yml up -d
```

Phase 1 uses PostgreSQL, Redis Streams, and object storage. Kafka, OpenSearch, etcd, Vault, and the Next.js web apps are intentionally deferred.

Run database migrations:

```bash
GOGOMAIL_DATABASE_URL='postgres://gogomail:gogomail@localhost:15432/gogomail?sslmode=disable' \
  go run ./cmd/gogomail --migrate --mode=all-in-one
```

## Receive mail locally

Start the current SMTP receive MVP:

```bash
GOGOMAIL_SMTP_ADDR=127.0.0.1:2525 \
GOGOMAIL_SMTP_DOMAIN=example.com \
GOGOMAIL_LOCAL_RECIPIENTS=admin@example.com \
GOGOMAIL_MAILSTORE_ROOT=var/mailstore \
GOGOMAIL_DEDUP_BACKEND=redis \
GOGOMAIL_RATELIMIT_BACKEND=redis \
GOGOMAIL_BACKPRESSURE_BACKEND=redis \
GOGOMAIL_RCPT_RATE_LIMIT_PER_MINUTE=60 \
GOGOMAIL_REDIS_ADDR=127.0.0.1:16379 \
  go run ./cmd/gogomail --mode=edge-mta
```

For the post-filter internal receive boundary, run `inbound-mta` on its own address:

```bash
GOGOMAIL_INBOUND_SMTP_ADDR=127.0.0.1:2526 \
GOGOMAIL_INBOUND_TRUSTED_RELAYS=127.0.0.1/32,::1/128 \
GOGOMAIL_SMTP_DOMAIN=example.com \
GOGOMAIL_LOCAL_RECIPIENTS=admin@example.com \
GOGOMAIL_MAILSTORE_ROOT=var/mailstore \
  go run ./cmd/gogomail --mode=inbound-mta
```

`inbound-mta` is intended for the post-filter/internal receive boundary. Set `GOGOMAIL_INBOUND_TRUSTED_RELAYS` to the Edge MTA or spam-filter relay CIDR list; untrusted remotes are rejected with an SMTP policy response before envelope state is accepted.

When `GOGOMAIL_LOCAL_RECIPIENTS` is set, edge-mta uses an in-memory static recipient resolver for local development.

When `GOGOMAIL_LOCAL_RECIPIENTS` is empty, edge-mta uses PostgreSQL:

```txt
user_addresses -> users/domains
folders(system_type=inbox)
messages INSERT
```

When `GOGOMAIL_DEDUP_BACKEND=redis`, edge-mta uses Redis `SET NX` with a 24-hour TTL to skip duplicate `Message-ID + recipient` deliveries.

When `GOGOMAIL_RATELIMIT_BACKEND=redis`, edge-mta applies a Redis-backed fixed-window RCPT rate limit per remote address.

When `GOGOMAIL_BACKPRESSURE_BACKEND=redis`, edge-mta reads `backpressure:smtp:state`.

```txt
normal/warning  -> accept DATA
danger/critical -> reject DATA before spooling
```

Useful SMTP receive guardrails:

```bash
GOGOMAIL_SMTP_MAX_RECIPIENTS=100
GOGOMAIL_SMTP_MAX_MESSAGE_BYTES=26214400
GOGOMAIL_SMTP_READ_TIMEOUT=30s
GOGOMAIL_SMTP_WRITE_TIMEOUT=30s
GOGOMAIL_SMTP_ADD_RECEIVED_HEADER=true
GOGOMAIL_SMTP_REQUIRE_AUTH=false
GOGOMAIL_SMTP_SUPPORT_SMTPUTF8=false
GOGOMAIL_SMTP_SUPPORT_REQUIRETLS=false
GOGOMAIL_SMTP_SUPPORT_DSN=false
GOGOMAIL_SMTP_SUPPORT_BINARYMIME=false
```

Optional attachment scanner webhook:

```bash
GOGOMAIL_ATTACHMENT_SCAN_BACKEND=webhook
GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_URL=https://scanner.example/scan
GOGOMAIL_ATTACHMENT_SCAN_WEBHOOK_TOKEN='optional-bearer-token'
GOGOMAIL_ATTACHMENT_SCAN_TIMEOUT=2s
```

The scanner hook runs only for parsed messages with attachments and remains
disabled by default. Production webhook URLs must use HTTPS. The same scanner
configuration is available to `edge-mta`, `inbound-mta`, and `outbound-mta`.

Accepted messages are stored as raw `.eml` files under:

```txt
var/mailstore/mailstore/{company_id}/{domain_id}/{user_id}/maildir/{YYYY}/{MM}/{message_id}.eml
```

## Submit outbound mail locally

`outbound-mta` exposes the authenticated SMTP Submission boundary. By default it listens on `:2587` for local development:

```bash
GOGOMAIL_SUBMISSION_ADDR=127.0.0.1:2587 \
GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH=true \
GOGOMAIL_SMTP_DOMAIN=example.com \
GOGOMAIL_DATABASE_URL='postgres://gogomail:gogomail@localhost:15432/gogomail?sslmode=disable' \
GOGOMAIL_MAILSTORE_ROOT=var/mailstore \
  go run ./cmd/gogomail --mode=outbound-mta
```

Submission requires `AUTH PLAIN`, verifies that `MAIL FROM` belongs to the authenticated user, stores the raw RFC 5322 `.eml`, then records the message through the existing `mail.outbound.<farm>` outbox flow.

Useful submission guardrails:

```bash
GOGOMAIL_SUBMISSION_MAX_RECIPIENTS=100
GOGOMAIL_SUBMISSION_MAX_MESSAGE_BYTES=26214400
GOGOMAIL_SMTP_READ_TIMEOUT=30s
GOGOMAIL_SMTP_WRITE_TIMEOUT=30s
GOGOMAIL_SUBMISSION_ADD_RECEIVED_HEADER=true
GOGOMAIL_SUBMISSION_SUPPORT_SMTPUTF8=false
GOGOMAIL_SUBMISSION_SUPPORT_REQUIRETLS=false
GOGOMAIL_SUBMISSION_SUPPORT_DSN=false
GOGOMAIL_SUBMISSION_SUPPORT_BINARYMIME=false
```

For production, configure STARTTLS certificates and keep insecure AUTH disabled:

```bash
GOGOMAIL_ENV=production \
GOGOMAIL_SUBMISSION_ADDR=:587 \
GOGOMAIL_SMTP_TLS_CERT_FILE=/etc/gogomail/tls/fullchain.pem \
GOGOMAIL_SMTP_TLS_KEY_FILE=/etc/gogomail/tls/privkey.pem \
GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH=false \
  gogomail --mode=outbound-mta
```

If you also need implicit TLS submission for legacy clients, set `GOGOMAIL_SUBMISSION_SMTPS_ADDR` (usually `:465`). SMTPS requires the same `GOGOMAIL_SMTP_TLS_CERT_FILE` and `GOGOMAIL_SMTP_TLS_KEY_FILE` pair and runs alongside the STARTTLS listener:

```bash
GOGOMAIL_ENV=production \
GOGOMAIL_SUBMISSION_ADDR=:587 \
GOGOMAIL_SUBMISSION_SMTPS_ADDR=:465 \
GOGOMAIL_SMTP_TLS_CERT_FILE=/etc/gogomail/tls/fullchain.pem \
GOGOMAIL_SMTP_TLS_KEY_FILE=/etc/gogomail/tls/privkey.pem \
GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH=false \
  gogomail --mode=outbound-mta
```

Local users authenticate against `users.password_hash`. Supported formats are:

- `pbkdf2-sha256$<iterations>$<base64-salt>$<base64-key>`
- `sha256:<hex>` for legacy/dev fixtures
- `plain:<password>` only for explicit local development fixtures

### Delivery smart host relay

By default the delivery worker performs direct MX delivery. To relay all outbound mail through a gateway/smart host instead, configure:

```bash
GOGOMAIL_DELIVERY_SMARTHOST=smtp.relay.example.net:587
GOGOMAIL_DELIVERY_TLS_MODE=require
GOGOMAIL_DELIVERY_SMARTHOST_USERNAME=relay-user
GOGOMAIL_DELIVERY_SMARTHOST_PASSWORD=relay-secret
```

For implicit TLS relay gateways, typically port 465, enable:

```bash
GOGOMAIL_DELIVERY_SMARTHOST=smtp.relay.example.net:465
GOGOMAIL_DELIVERY_SMARTHOST_IMPLICIT_TLS=true
GOGOMAIL_DELIVERY_SMARTHOST_TLS_MODE=require
```

`GOGOMAIL_DELIVERY_SMARTHOST_TLS_MODE` is optional; when omitted, smart-host routes inherit `GOGOMAIL_DELIVERY_TLS_MODE`.

### Bounce / DSN handling

Hard-bounce delivery events generate RFC 3464 `multipart/report` Delivery Status Notifications through the event-worker. DSNs are stored as `.eml` files and queued back to the original envelope sender with a null reverse-path, reducing bounce-loop risk.

Useful DSN settings:

```bash
GOGOMAIL_DSN_POSTMASTER='Mail Delivery Subsystem <postmaster@example.com>'
GOGOMAIL_SMTP_DOMAIN=mx.example.com
```

`NOTIFY=NEVER` is honored, DSN queueing uses deterministic storage paths and outbox dedupe keys, and null reverse-path DSN bounces do not create suppression-list entries.
