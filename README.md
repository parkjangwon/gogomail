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
  go run ./cmd/gogomail --mode=edge-mta
```

When `GOGOMAIL_LOCAL_RECIPIENTS` is set, edge-mta uses an in-memory static recipient resolver for local development.

When `GOGOMAIL_LOCAL_RECIPIENTS` is empty, edge-mta uses PostgreSQL:

```txt
user_addresses -> users/domains
folders(system_type=inbox)
messages INSERT
```

Accepted messages are stored as raw `.eml` files under:

```txt
var/mailstore/mailstore/{company_id}/{domain_id}/{user_id}/maildir/{YYYY}/{MM}/{message_id}.eml
```
