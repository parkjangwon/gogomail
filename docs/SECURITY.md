# Security Model

gogomail is designed to be deployed on the public internet as an email and
collaboration platform. This document describes the threat model, security
controls, and accepted residual risks.

See also: `docs/SECURITY_REVIEW.md` (audit findings),
`docs/backend-release-readiness.md` (release gates).

---

## Threat model

| Actor | Capability | Primary mitigations |
|---|---|---|
| Anonymous internet attacker | Probe SMTP/IMAP/HTTP, send spam, mount DDoS | Rate limits, DNSBL, DMARC, brute-force tracker, nginx layer, backpressure |
| Spammer/spoofer | Send forged inbound mail | SPF + DKIM + DMARC enforcement (`reject` by default) |
| Authenticated end user | Read other users' mail, escalate to admin | Per-user UUID scoping, RLS-style query filters, JWT scopes, MFA |
| Malicious tenant admin | Read other tenants' data, exfiltrate | `company_id` boundary on every query, separate JWT scopes, admin audit log |
| Compromised admin credential | Wholesale tenant takeover | MFA required for admin, login rate limit 5/min, audit log, IP allowlist (optional) |
| Compromised mail server account | Send phishing as legit user | Submission DKIM signing, bulk-sender rate limit, abuse heuristics |
| Compromised database | Read everything | Encryption-at-rest (storage), PBKDF2 password hashes, JWT secret not in DB |
| Compromised backup | Same as DB | Encrypted snapshots, off-site key management |
| Insider with host access | Process memory, secrets in env | Secrets never logged, JWT secret rotatable, scoped IAM for S3 |

Out of scope: physical attacks on the host, supply-chain compromise of Go
toolchain.

---

## Authentication architecture

| Subject | Mechanism | Notes |
|---|---|---|
| End-user web/IMAP/POP3/SMTP submission | Password + optional TOTP MFA | PBKDF2-SHA256, configurable cost; refresh tokens; per-account `authFailureTracker` (lockout after N) |
| End-user API | JWT access token (≤15 min) + refresh token | `GOGOMAIL_AUTH_JWT_SECRET` ≥ 32 bytes in production |
| Admin API | JWT + MFA when `GOGOMAIL_ADMIN_MFA_REQUIRED=true` | Login limiter 5/min/IP; MFA grace deadline tracked per user |
| Admin static token | `GOGOMAIL_ADMIN_TOKEN` (≥ 32 bytes prod) | For automation only; rotate quarterly |
| SCIM provisioning | Bearer token `GOGOMAIL_SCIM_TOKEN` | Scoped to default domain id |
| LDAP gateway | Simple bind (LDAPS recommended) | Same credential store as IMAP/SMTP |
| Inbound MTA | None (public) | Policy enforced by SPF/DKIM/DMARC + DNSBL |
| Internal inbound MTA | Source IP allowlist (`GOGOMAIL_INBOUND_TRUSTED_RELAYS`) | Defaults to loopback only |
| API keys (per-tenant) | DB-stored, hashed | Used by `api-metering-worker` for billing/export |

### Token lifecycle

```
Login -> access (~15m) + refresh (~30d)
Refresh -> rotate (issue new refresh, revoke old, store family for replay detection)
Logout -> revoke refresh family
Password reset -> revoke all tokens for user
```

Refresh-token replay (using an already-rotated refresh) revokes the **whole
family** — defense against stolen refresh tokens.

---

## Authorization model

Three nested boundaries:

```
company (tenant)
  └── domain (mail domain owned by the company)
        └── user (individual mailbox)
```

Every persistent row is keyed by at least `company_id`. Every query goes
through repository methods that take `company_id` as a parameter.

### Admin scopes

| Scope | Purpose |
|---|---|
| `super-admin` | Cross-tenant: create companies, view billing |
| `company-admin` | Single tenant: domains, users, policies |
| `domain-admin` | Single domain: users in domain |
| `user` | Self only |
| `audit-viewer` | Read-only audit log access |

JWT carries scope + `company_id` + `domain_id` (when applicable). Middleware
rejects mismatched scopes at the route level.

---

## Rate limiting

All limits are per source IP and use Redis when available (single-process
fallback for dev).

| Surface | Limit | Source |
|---|---|---|
| Auth login (mail-api) | 10/min | `mail.go:439` |
| Auth login (admin-api) | 5/min | `admin.go:4969` |
| Password reset request | 5 / 15 min | `password_reset.go:98` |
| Password reset confirm | 10/min | `password_reset.go:152` |
| Search | 30/min | `mail.go:847` |
| Attachment download | 60/min | `mail.go:1939` |
| Mail mutations | 300/min (configurable) | `GOGOMAIL_MAIL_MUTATION_RATELIMIT_PER_MINUTE` |
| Drive share creation | 120/min (configurable) | `GOGOMAIL_DRIVE_SHARE_RATELIMIT_PER_MINUTE` |
| Inbound SMTP per IP | configurable | `GOGOMAIL_RATELIMIT_BACKEND=redis` |
| Submission bulk-sender | configurable | `GOGOMAIL_SUBMISSION_BULK_SENDER_RATE` |

### Brute-force protection

Each of IMAP, POP3, SMTP submission, LDAP, CalDAV, CardDAV uses an in-process
`authFailureTracker` keyed by `(remote_ip, username)`. After threshold
failures, subsequent attempts are rejected for an exponentially-growing window
even if credentials are correct (until the window expires).

---

## Trusted-proxy handling

`X-Real-IP` and `X-Forwarded-For` headers are honored **only** when the
immediate peer is in the configured trusted-proxy list:

- HTTP: `GOGOMAIL_CALDAV_TRUSTED_PROXIES`, `GOGOMAIL_CARDDAV_TRUSTED_PROXIES`
- Otherwise: TCP peer address used directly.

Setting `GOGOMAIL_*_TRUST_FORWARDED_PROTO=true` additionally honors
`X-Forwarded-Proto` for HTTPS detection — required when behind a TLS-terminating
LB.

Misconfiguration risk: trusting a proxy list that does not match reality lets
attackers forge their source IP. The validator rejects empty/invalid CIDRs.

---

## Transport security

| Endpoint | TLS | Notes |
|---|---|---|
| HTTP (`mail-api`, `admin-api`, `auth-server`) | Required in prod | Terminate at LB; backend can be plain HTTP inside trusted network |
| Inbound SMTP (`edge-mta`) | STARTTLS (RFC 3207); MTA-STS recommended | TLS cert/key required if exposed |
| Submission (`outbound-mta`) | STARTTLS (587) + implicit TLS (465) | `GOGOMAIL_SUBMISSION_ALLOW_INSECURE_AUTH=false` enforced in prod |
| IMAP / POP3 | STARTTLS or implicit TLS | `*_ALLOW_INSECURE_AUTH=false` in prod |
| LDAP | STARTTLS + LDAPS | Optional but recommended |
| Delivery (outbound) | Opportunistic, required, or DANE | `GOGOMAIL_DELIVERY_TLS_MODE` |
| Storage (S3) | HTTPS required in prod | Validator rejects HTTP S3 endpoint in prod |

HSTS header is sent on all HTTP responses; the LB can extend `max-age` and add
`preload`.

---

## Data at rest

| Data | Storage | Protection |
|---|---|---|
| User passwords | `users.password_hash` | PBKDF2-SHA256, 32-byte salt, configurable iterations |
| Refresh tokens | DB | Hashed before storage |
| TOTP secrets | DB | Encrypted with `GOGOMAIL_AUTH_JWT_SECRET`-derived key |
| Mail bodies | S3 / MinIO / local FS | S3 SSE (server-side encryption) — enable on bucket; transit HTTPS |
| Attachments | S3 / MinIO / local FS | Same as mail bodies |
| Drive objects | S3 / MinIO / local FS | Same |
| Audit log | DB | Indexed by `(actor_id, action, ts)` |
| DKIM private keys | DB | Per-domain; rotated via admin API |
| JWT secret | env var only | Never persisted to DB, never logged |

The application does **not** itself encrypt blobs in storage. Enable SSE on
S3 / MinIO and disk-level encryption (LUKS, EBS) on Postgres volumes.

---

## Email security (RFC compliance)

| Standard | Role |
|---|---|
| SPF (RFC 7208) | Inbound verification by `edge-mta` when `SMTP_AUTH_VERIFICATION_ENABLED=true` |
| DKIM (RFC 6376) | Inbound verification + outbound signing (`GOGOMAIL_DKIM_ENABLED=true`) |
| DMARC (RFC 7489) | Inbound enforcement, `GOGOMAIL_SMTP_DMARC_ENFORCEMENT` ∈ `{reject, quarantine, none}` |
| ARC (RFC 8617) | Sealed by `edge-mta` on forwarded mail |
| MTA-STS (RFC 8461) | Recommend publishing `_mta-sts.<domain>` policy |
| TLS-RPT (RFC 8460) | Recommend publishing `_smtp._tls.<domain>` |
| DANE (RFC 7672) | Optional via `GOGOMAIL_DELIVERY_TLS_MODE=dane` |
| BIMI | Out of scope (DNS-only, no code) |

DKIM keys are 2048-bit RSA by default; per-domain rotation supported via
admin API.

---

## DDoS mitigation

Layered defenses:

1. **Network** — Use an upstream WAF / scrubbing service (Cloudflare,
   AWS Shield) on HTTP. SMTP must be unprotected — rely on application-layer.
2. **L4** — nginx/HAProxy `limit_conn` per source IP for SMTP/IMAP/POP3.
3. **Application** —
   - Connection caps: `GOGOMAIL_SMTP_MAX_CONNECTIONS=10000`,
     `IMAP_MAX_CONNECTIONS=5000`, `POP3_MAX_CONNECTIONS=2000`.
   - Backpressure: auto-backpressure monitors memory + queue depth and
     refuses new SMTP sessions when thresholds breached.
   - Header / body size caps: `GOGOMAIL_HTTP_MAX_HEADER_BYTES=65536`,
     `GOGOMAIL_SMTP_MAX_MESSAGE_BYTES`.
   - Timeouts: aggressive `READ`/`WRITE`/`IDLE` defaults.
4. **Dependency** — DB connection pool caps (`DB_MAX_OPEN_CONNS=20` per
   replica); Redis Sentinel for failover; circuit breakers on outbound
   delivery.

---

## Audit logging

Logged structured (slog JSON in prod):

- Admin actions: tenant create/delete, user create/delete/disable, password
  reset, MFA reset, domain add/remove, policy change.
- Auth events: login success/failure, MFA challenge, refresh, logout, lockout.
- Tenant-impacting events: quota threshold reached, auto-purge run, retention
  delete.
- Outbound delivery: each attempt outcome (success/defer/bounce).

Retention: defaults indefinite; prune via `batch-worker` token-cleanup +
explicit ops tooling. Audit log rows are append-only at the application layer
(no UPDATE endpoint).

---

## Secrets management

- All secrets sourced from env vars (12-factor). Never read from VCS-tracked
  files except `docker/.env` for local dev.
- Production validator (`validate.go`) requires:
  - `GOGOMAIL_AUTH_JWT_SECRET` ≥ 32 bytes
  - `GOGOMAIL_ADMIN_TOKEN` non-empty
  - `GOGOMAIL_REDIS_PASSWORD` non-empty (when farm coordinator uses Redis)
- Secrets never enter log output: slog handlers redact known-secret keys
  (`password`, `token`, `secret`, `key`, `private_key`).
- Rotate `AUTH_JWT_SECRET` by deploying two values briefly (`OLD` + `NEW`
  acceptance window) — see runbook in `OPERATIONS.md`.
- S3 access via short-lived STS tokens preferred over static keys when
  available.

---

## Known limitations / accepted risks

- **No end-to-end mail encryption** (PGP/S/MIME). Operators may add it client
  side; gogomail treats encrypted bodies as opaque.
- **No homomorphic search**. Server-side index (OpenSearch) reads plaintext
  bodies. Operators handling regulated content should disable
  `search-index-worker` and rely on client-side search.
- **MFA TOTP only** at present. FIDO2/WebAuthn is on the roadmap.
- **Outbox-relay singleton** is a single point of contention. The relay does
  not block ingest; if it lags, delivery lags, but no data is lost.
- **Self-signed DKIM keys** are stored in the DB. Operators must protect DB
  backups accordingly.
- **TLS termination at LB** means in-cluster traffic between LB and app is
  plaintext by default. Use mTLS or a service mesh for zero-trust
  environments.
