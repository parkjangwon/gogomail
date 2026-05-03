# gogomail

Go-first backend implementation for the gogomail webmail/mail-server platform.

## Current scope

This repository starts with the backend foundation only. The Next.js web apps will be added after the SMTP/API backend contracts stabilize.

## Backend modes

```bash
go run ./cmd/gogomail --mode=all-in-one
go run ./cmd/gogomail --mode=edge-mta
go run ./cmd/gogomail --mode=inbound-mta
go run ./cmd/gogomail --mode=outbound-mta
go run ./cmd/gogomail --mode=delivery-worker
go run ./cmd/gogomail --mode=auth-server
go run ./cmd/gogomail --mode=mail-api
go run ./cmd/gogomail --mode=admin-api
```

## Verify

```bash
go test ./...
```

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
