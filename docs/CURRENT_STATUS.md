# gogomail current status

Last updated: 2026-05-26

## Platform summary

GoGoMail is a production-grade self-hosted email platform written in Go
(single binary, 24 runtime modes). Key capabilities:

- **Protocols**: SMTP (inbound/outbound), IMAP, POP3, CalDAV, CardDAV, WebDAV, LDAP
- **Mail security**: SPF, DKIM, DMARC, ARC, MTA-STS, TLS-RPT, DANE
- **Storage**: PostgreSQL (multi-tenant), Redis Streams (outbox), S3-compatible object storage
- **Auth**: JWT + TOTP MFA + refresh token rotation
- **Frontend**: Next.js 16 webmail SPA + admin console (TypeScript/TSX)
- **AI interface**: User MCP (123 tools) + Manage MCP (50 admin tools)
- **Monitoring**: Prometheus + Loki + Promtail + Grafana (provisioned dashboards)

## Completed milestones (2026-05)

| Date | Feature |
|------|---------|
| 2026-05-26 | Codebase improvements (10 tasks): doc cleanup, security hardening, TypeScript domain splits (MCP tools, webmail API, UI components), Go package refactoring (httpapi/admin.go 8901→420 lines + 12 files, app/admin_service.go 1759→93 lines + 5 files) |
| 2026-05-25 | DM search candidate limit 10000→1000; system messages injectable |
| 2026-05-25 | Grafana default password removed; metrics interface{} replaced with typed interfaces |
| 2026-05-24 | User MCP DM tools (18 tools): rooms, messages, attachments, reactions, search |
| 2026-05-23 | Web push notifications + push device management |
| 2026-05-23 | Monitoring stack: Prometheus, Loki, Promtail, Grafana with provisioned dashboards |
| 2026-05-22 | Admin console MFA enforcement |
| 2026-05-21 | Multiple query sargability improvements, LDAP batching, i18n cleanup |
| 2026-05-14 | DM instant messaging: rooms, encrypted messages, attachments, reactions, invites, Drive messages |
| 2026-05-10 | Web Push service worker, calendar edit/delete, password reset UI, server-synced signatures |
| 2026-05-07 | User MCP server: 123 tools across mail, DM, contacts, Drive, calendar, preferences |
| 2026-05-05 | Manage MCP server: 50 admin tools for Admin API, queue/health, org/security/spam policies |

## Architecture

See `docs/ARCHITECTURE.md` for the architecture overview.
See `docs/backend-roadmap.md` for the full feature roadmap.
See `docs/openapi.yaml` for the REST API spec.
See `PROJECT_HARNESS.md` for development workflow.
