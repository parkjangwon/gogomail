# SMTP backend release runbook

This runbook covers the backend-only SMTP checks that should pass before a
release cut. It intentionally avoids frontend startup and built-in spam
filtering work.

## PostgreSQL-backed verification

Use a disposable PostgreSQL database or schema. The integration tests create a
temporary schema, run all migrations, and clean the schema up afterward.

```sh
export GOGOMAIL_TEST_DATABASE_URL='postgres://gogomail:gogomail@localhost:5432/gogomail?sslmode=disable'
go test ./internal/maildb ./internal/outbox
```

What this verifies:

- all migrations apply in a real PostgreSQL schema;
- draft-to-send can create a sent message, move draft attachments, refresh
  `has_attachment`, and enqueue `mail.outbound.general`;
- DSN envelope metadata survives the PostgreSQL outbox payload path;
- outbox relay claiming respects `available_at`;
- failed outbox rows return to `pending` until retry exhaustion and then become
  `failed` with bounded UTF-8-safe diagnostics.

## Same-connection SMTP soak

The unit/integration suite includes a repeated-DATA transaction guard, but an
operator soak should also exercise the deployed listener.

Recommended manual shape:

1. open one SMTP TCP connection to the intended edge/inbound listener;
2. issue `EHLO`;
3. send at least 100 sequential `MAIL FROM` / `RCPT TO` / `DATA` transactions
   on the same connection;
4. confirm every accepted transaction stores one `.eml` and records one
   `mail.stored` outbox row;
5. confirm envelope state does not leak between transactions by varying sender,
   recipients, DSN options, and message IDs.

Abort the release if accepted message counts, stored object counts, or outbox
counts diverge.

## STARTTLS submission

Production submission should reject AUTH before TLS unless explicitly configured
otherwise for development.

Checklist:

- set `GOGOMAIL_SUBMISSION_ADDR`;
- set `GOGOMAIL_SMTP_TLS_CERT_FILE` and `GOGOMAIL_SMTP_TLS_KEY_FILE`;
- keep insecure AUTH disabled in production;
- verify `EHLO` advertises `STARTTLS`;
- verify `AUTH PLAIN` fails before STARTTLS;
- complete STARTTLS, issue a fresh `EHLO`, then verify `AUTH PLAIN` succeeds
  with valid credentials;
- send one authenticated message and confirm it queues through
  `mail.outbound.general`.

## SMTPS submission

Implicit TLS requires the same certificate/key pair as STARTTLS submission.

Checklist:

- set `GOGOMAIL_SUBMISSION_SMTPS_ADDR`;
- set `GOGOMAIL_SMTP_TLS_CERT_FILE` and `GOGOMAIL_SMTP_TLS_KEY_FILE`;
- verify startup fails clearly if either TLS file is missing;
- connect with TLS from byte zero;
- verify AUTH and message submission work without a STARTTLS command;
- verify STARTTLS submission and SMTPS can run side by side when both listener
  addresses are configured.

## Trusted relay / inbound policy

Use `GOGOMAIL_INBOUND_TRUSTED_RELAYS` for post-filter inbound deployments where
only known relays may inject mail.

Checklist:

- configure explicit CIDRs, not broad catch-alls, for production;
- verify a trusted relay can reach `MAIL FROM`;
- verify an untrusted remote is rejected at the `MAIL FROM` boundary before
  RCPT or DATA state is accepted;
- include IPv4, IPv6, IPv4-mapped IPv6, and remote address strings with TCP
  ports in policy tests for listener adapter coverage.

## Outbound DSN / bounce smoke

Use a controlled SMTP sink before release.

Automated coverage already verifies:

- outbound DSN parameters are emitted only to peers advertising `DSN`;
- non-DSN peers receive no `RET`, `ENVID`, `NOTIFY`, or `ORCPT` parameters;
- SMTP wire receive preserves `NOTIFY=NEVER` for downstream bounce suppression;
- generated bounce DSNs use a null reverse path;
- null reverse-path delivery suppresses outbound DSN request parameters;
- RCPT-level permanent and temporary sink failures remain distinct after DATA so
  the delivery handler can bounce permanent recipients and retry temporary ones;
- malformed DSN xtext metadata is rejected at queue and bounce-event trust
  boundaries.

Checklist:

- verify outbound DSN options are sent only when the remote advertises `DSN`;
- verify the same delivery suppresses `RET`, `ENVID`, `NOTIFY`, and `ORCPT`
  when the remote does not advertise `DSN`;
- verify `NOTIFY=NEVER` suppresses generated failure DSNs;
- verify null reverse-path DSN delivery suppresses outbound DSN request
  parameters to avoid bounce loops;
- verify temporary recipient failures schedule retries while permanent failures
  produce terminal bounce events.
