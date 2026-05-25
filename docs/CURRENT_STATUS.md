# gogomail current status

Last updated: 2026-05-26

## Platform summary

GoGoMail is a production-grade self-hosted email platform written in Go
(single binary, 24 runtime modes). Key capabilities:

- **Protocols**: SMTP (inbound/outbound), IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP
- **Mail security**: SPF, DKIM, DMARC, ARC, MTA-STS, TLS-RPT, DANE
- **Storage**: PostgreSQL (multi-tenant), Redis Streams (outbox), S3-compatible object storage
- **Auth**: JWT + TOTP MFA + refresh token rotation
- **Frontend**: Next.js webmail + admin console (TypeScript/TSX)
- **AI interface**: User MCP (123 tools) + Manage MCP (admin tools)
- **Monitoring**: Prometheus + Loki + Promtail + Grafana (provisioned dashboards)

## Recent milestones (2026-05)

| Date | Feature |
|------|---------|
| 2026-05-26 | Refactor: split admin_system.go (3510 lines) into 4 focused files (auth/access-policy/security-config/governance) |
| 2026-05-26 | Codebase improvements: doc cleanup, security hardening, file splits |
| 2026-05-25 | DM search candidate limit lowered 10000 → 1000; system messages injectable |
| 2026-05-25 | Grafana default password removed; metrics interface{} replaced with typed interfaces |
| 2026-05-24 | User MCP DM tools (18 tools): rooms, messages, attachments, reactions, search |
| 2026-05-23 | Web push notifications + push device management |
| 2026-05-23 | Monitoring stack: Prometheus, Loki, Promtail, Grafana with provisioned dashboard |
| 2026-05-22 | Admin console MFA enforcement |
| 2026-05-21 | Multiple query sargability improvements, LDAP batching, i18n cleanup |
| 2026-05-21 | Webmail session settings honesty, integration capability honesty |
| 2026-05-21 | Admin operational list query sargability |
| 2026-05-21 | Mail search text filter sargability |
| 2026-05-21 | LDAP empty group fallback cleanup |
| 2026-05-21 | OpenSearch Korean analyzer diagnostics |
| 2026-05-21 | Webmail settings import feedback |
| 2026-05-21 | Admin company user scope batching |
| 2026-05-21 | LDAP membership expansion batching |
| 2026-05-21 | Scheduling attendee user lookup batching |
| 2026-05-21 | Drive upload session expiry batching |
| 2026-05-21 | CardDAV org-tree member batching |
| 2026-05-21 | Admin console API spec route alignment |
| 2026-05-21 | Console locale placeholder cleanup |

## Architecture

See `docs/backend-roadmap.md` for the full feature roadmap.
See `docs/openapi.yaml` for the REST API spec.
See `PROJECT_HARNESS.md` for development workflow.
