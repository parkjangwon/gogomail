# gogomail next steps

Last updated: 2026-05-31

## Current focus

All core backend phases (0–8), the security remediation sprint, protocol
hardening, cleanup/rollback observability, SCIM sync logging, remote-signer
hardening, and frontend promise-policy cleanup are complete as of 2026-05-31.
The platform is ready for production use. Focus shifts to product expansion,
mobile/desktop clients, and deployment-specific operator polish.

## Backlog (priority order)

1. **Mobile app** — React Native or PWA wrapper; mail, Drive, calendar, contacts, push, offline sync
2. **Desktop/power-user app** — keyboard workflows, multi-pane, bulk triage, advanced search
3. **AI email assistant** — smart compose, auto-categorization, thread summary via MCP agents
4. **Admin audit log retention** — configurable auto-expiry with policy UI
5. **Multi-region failover** — PostgreSQL replication + S3 cross-region
6. **DM message pinning** — pin important messages in a room for quick access
7. **CalDAV Apple/Android/Windows compatibility** — broader client compatibility tests
8. **CardDAV vCard 3.0/4.0 client compatibility** — additional native-client tests
9. **Kafka / OpenSearch as optional backends** — for operators who want dedicated search/event infra

## Recently completed

- **Cleanup/rollback observability** — best-effort delete failures in
  attachment, Drive, SMTP, IMAP APPEND, outbound-send, DSN enqueue, API usage
  export, and storage readiness paths now warn or persist retryable failure
  records (2026-05-31)
- **SCIM + fail-open tracking** — SCIM IdP status sync failures and API metering
  sink errors now emit structured warnings while preserving requested runtime
  behavior (2026-05-31)
- **Remote signer hardening** — structured JSON logs, config validation, HTTP
  timeouts, max-header limits, graceful shutdown, and lifecycle tests
  (2026-05-31)
- **Protocol/RFC boundary tests** — POP3 command caps, LDAP malformed/oversized
  PDU handling, and SMTP spool/header line caps covered by regression tests
  (2026-05-31)
- **Frontend promise/API helper cleanup** — `ignoreNonCritical()` standardizes
  intentional fire-and-forget promise handling; server proxy fallback helpers
  are explicit and tested (2026-05-31)
- **Security remediation** — IDOR sweep across all admin handlers, StripInternalHeadersMiddleware, Helm CHANGEME guard, CSP nonce, PBKDF2 auto-upgrade, RDBMS IdP SQL allowlist (2026-05-28)
- **Admin console page decomposition** — spam-filter (1273→153 lines), domain detail (945→116 lines) (2026-05-28)
- **Settings UI deduplication** — removed dead SettingsModal from webmail Sidebar (2026-05-28)
- **APNS private key file option** — `GOGOMAIL_APNS_PRIVATE_KEY_FILE` config support for K8s secret mounts (2026-05-28)
- **ClamAV attachment scanning** — INSTREAM protocol, concurrent scan cap, circuit breaker (2026-05-26)
- **SMTP rate limiting per recipient domain** — `InMemoryDomainRateLimiter` (fixed-window, per-minute) (2026-05-26)
- **DM search scalability** — paginated full-history scan, removed hard cap (2026-05-26)
- **DM key rotation** — `POST /api/v1/dm/rooms/{roomID}/rotate-key`, atomic re-encryption (2026-05-26)
- **K8s deployment** — 8 manifests (namespace, configmap, secret, deployment, service, HPA, PDB, ingress) (2026-05-26)
- **OpenSearch integration** — configurable FTS backend (`SEARCH_INDEX_BACKEND=opensearch`) (2026-05-26)

## Out of scope

See `docs/backend-roadmap.md` for deferred items and the Module × RFC Compliance Map.
