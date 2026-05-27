# gogomail backend roadmap

Last updated: 2026-05-28

## Completed phases

All core phases through Phase 8 are complete as of 2026-05-28.

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 0 | Backend foundation: single binary, env config, liveness/readiness, mail-address normalization | ✅ Done |
| Phase 1 | Receive and read mail: SMTP inbound, IMAP4rev2, local-domain delivery | ✅ Done |
| Phase 2 | Runtime config + settings hierarchy: company/domain/user 3-tier, LISTEN/NOTIFY | ✅ Done |
| Phase 3 | Submission and outbound delivery: SMTP submission (587/465), delivery workers, DKIM/SPF/DMARC, DSN/bounce, Outbox Pattern | ✅ Done |
| Phase 4 | Auth and multi-tenancy: JWT, TOTP MFA, refresh-token rotation, replay detection, PBKDF2, SCIM 2.0, SAML/OIDC SSO | ✅ Done |
| Phase 5 | Collaboration protocols: CalDAV (RFC 4791+7809), CardDAV (RFC 6352), Drive WebDAV (RFC 4918), LDAP gateway (RFC 4511) | ✅ Done |
| Phase 6 | Anti-abuse and security: brute-force tracker, DNSBL, milter, rate limiting, MTA-STS, ARC, DANE, ClamAV, IDOR sweep, header stripping | ✅ Done |
| Phase 7 | Observability and reliability: Prometheus metrics, Loki+Promtail, Grafana dashboards, X-Request-ID tracing, POP3, Web Push | ✅ Done |
| Phase 8 | Admin console enterprise hardening: RBAC, audit logs, IdP abstraction (database/LDAP/RDBMS), mail logs, statistics, export, alerts | ✅ Done |

**Webmail SPA** (Next.js 16): mail, compose, drafts, folders, contacts, Drive, calendar, encrypted DM, notification center, Web Push, MFA, i18n (en/ko/ja/zh-CN) — all shipped.

**AI Agent automation**: User MCP (123 tools) and Management MCP (50 tools) — both shipped and documented.

**Security hardening** (2026-05-28): IDOR sweep complete across all admin handlers, StripInternalHeadersMiddleware, Helm CHANGEME guard, CSP nonce, PBKDF2 auto-upgrade, RDBMS IdP SQL allowlist.

---

## Active backlog

These are items not yet implemented or deliberately deferred:

### Deferred until backend contracts stabilize

- Mobile apps (iOS/Android): mail, Drive, calendar, contacts, push, offline sync
- Desktop/power-user app: keyboard workflows, multi-pane, bulk triage, advanced search
- Kafka / OpenSearch as mandatory backend (optional integrations only for now)
- etcd, Vault
- CalDAV production sync-token retention-age policy; broader Apple/Android/Windows/macOS compatibility tests
- Directory/Identity expansion: effective resource booking policy, company-scoped delegation reads, bounded group membership expansion
- CardDAV broader vCard 3.0/4.0 compatibility with additional native-client compatibility tests
- Notification & Sync boundary: domain events, reminders, device policy, delta fan-out
- Vendor push notification delivery adapters (APNs file-key done; FCM native adapter pending)

---

## Module × RFC Compliance Map

| Module | Key Standards |
|---|---|
| SMTP receive (edge MTA) | RFC 5321, RFC 5322, RFC 2045–2049, RFC 6531/6532 |
| SMTP submission (outbound MTA) | RFC 5321, RFC 6409, RFC 4954 (AUTH) |
| SMTP delivery (outbound transport) | RFC 5321, RFC 7505 (null MX), RFC 3461/3464 (DSN) |
| SMTP relay / smarthost gateway | RFC 5321 |
| DKIM signing | RFC 6376 |
| SPF | RFC 7208 |
| DMARC | RFC 7489 |
| IMAP | RFC 9051 (IMAP4rev2), RFC 3501 (IMAP4rev1) |
| POP3 | RFC 1939, RFC 2449 (CAPA), RFC 2595 (STLS), RFC 1734 (AUTH) |
| CalDAV | RFC 4791, RFC 5545 (iCalendar), RFC 6638, RFC 7809 (timezone) |
| iMIP scheduling | RFC 6047 |
| CardDAV | RFC 6352, RFC 6350 (vCard 4.0), RFC 2426 (vCard 3.0) |
| Drive WebDAV | RFC 4918, RFC 3744 (ACL), RFC 4331 (quota) |
| LDAP Gateway | RFC 4511, RFC 4512, RFC 4519 |
| SCIM 2.0 | RFC 7642, RFC 7643, RFC 7644 |
| SAML 2.0 | OASIS SAML 2.0 Core |
| OIDC | OpenID Connect Core 1.0, RFC 7636 (PKCE) |
| Milter (spam filter hook) | sendmail milter v2/v6 protocol |
| DNSBL | RFC 5782 |
| DNS autodiscovery | RFC 6764 (Well-Known URIs, DNS SRV) |
| DSN / bounce | RFC 3461, RFC 3464, RFC 5321 §4.5.5 (VERP) |
| Push notifications (Web) | RFC 8030 |
| TLS (all protocols) | RFC 8446 (TLS 1.3), RFC 5246 (TLS 1.2 minimum) |
| 2FA / TOTP | RFC 6238 (TOTP), RFC 4226 (HOTP) |
| JWT auth | RFC 7519 |
| Open API / API key auth | Bearer token + CIDR allowlist (domain_api_keys) |
| Real-time config SSE | Server-Sent Events (HTML5 EventSource) |
