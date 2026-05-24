# GoGoMail Manage MCP Server

Korean / 한국어: [README.ko.md](README.ko.md)

`gogomail-manage-mcp` is an [MCP (Model Context Protocol)](https://modelcontextprotocol.io/) server that gives an AI agent direct, structured management access to GoGoMail's Admin API, an optional Suppo helpdesk, and GitHub Issues. Designed for **unmanned 24/7 mail service operation** — an agent can diagnose and fix delivery failures, manage user accounts, inspect mail queues, and work through support tickets without human intervention.

Current GoGoMail Admin coverage is **49 tools**. The catalog includes typed tools for user/domain operations, delivery and queue diagnostics, organization membership/title metadata, security and spam-filter policies, and a guarded `gogomail_admin_api_request` bridge for documented admin-console routes that do not yet need dedicated wrappers.

---

## Table of Contents

- [Architecture](#architecture)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Configuration](#configuration)
- [Running the Server](#running-the-server)
  - [Claude Desktop (stdio)](#claude-desktop-stdio)
  - [Autonomous Agent (HTTP + SSE)](#autonomous-agent-http--sse)
- [Tool Reference](#tool-reference)
  - [GoGoMail Admin (49)](#gogomail-admin-49)
  - [Suppo Helpdesk (10)](#suppo-helpdesk-10)
  - [GitHub Issues (6)](#github-issues-6)
- [Workflow Examples](#workflow-examples)
- [Audit Trail](#audit-trail)
- [Security Considerations](#security-considerations)

---

## Architecture

```
Natural-language request (human or ticket system)
          │
          ▼
    AI agent (Claude, GPT-4, …)
          │  MCP protocol (JSON-RPC 2.0)
          ▼
  ┌─────────────────────────────┐
  │   apps/gogomail-manage-mcp  │
  │   (Node.js / TypeScript)    │
  │                             │
  │  ┌──────────────────────┐   │
  │  │  GoGoMail Admin      │   │──► GET/PATCH/POST/DELETE /admin/v1/…
  │  │  49 tools  [required]│   │    Bearer: GOGOMAIL_ADMIN_KEY
  │  └──────────────────────┘   │
  │  ┌──────────────────────┐   │
  │  │  Suppo helpdesk      │   │──► /api/public/…
  │  │  10 tools  [optional]│   │    Bearer: SUPPO_API_KEY
  │  └──────────────────────┘   │
  │  ┌──────────────────────┐   │
  │  │  GitHub Issues       │   │──► api.github.com
  │  │   6 tools  [optional]│   │    Token: GITHUB_TOKEN
  │  └──────────────────────┘   │
  └─────────────────────────────┘
```

**Transport modes:**
- **stdio** — for Claude Desktop and local CLI use. The MCP host (Claude Desktop) spawns the server as a subprocess and communicates over stdin/stdout.
- **HTTP + SSE** — for remote autonomous agents. The server runs as an HTTP service; the agent connects over Server-Sent Events and sends commands via POST.

---

## Prerequisites

- Node.js 20 or later
- A running GoGoMail instance with the Admin API accessible

---

## Installation

```bash
cd apps/gogomail-manage-mcp
npm install
npm run build        # compiles TypeScript → dist/
```

The compiled entry point is `dist/index.js`.

---

## Configuration

All configuration is via environment variables.

| Variable | Required | Description |
|---|---|---|
| `GOGOMAIL_ADMIN_URL` | **Yes** | Base URL of the GoGoMail server, e.g. `https://mail.example.com` |
| `GOGOMAIL_ADMIN_KEY` | **Yes** | Admin API Bearer token |
| `SUPPO_API_URL` | No | Base URL of the Suppo helpdesk, e.g. `https://support.example.com` |
| `SUPPO_API_KEY` | No | Suppo public API key (`crn_live_…`). Requires `kb:write tickets:read tickets:create tickets:update` scopes |
| `GITHUB_TOKEN` | No | GitHub personal access token (PAT) with `repo` scope |
| `GITHUB_REPO` | No | `owner/repo` to use for Issues (default: `parkjangwon/gogomail`) |
| `MCP_TRANSPORT` | No | `stdio` (default) or `sse` |
| `MCP_HOST` | No | Bind address for SSE transport (default: `127.0.0.1`). Set explicitly if a reverse proxy or private network listener is required. |
| `MCP_PORT` | No | HTTP port when using SSE transport (default: `3100`) |
| `MCP_SECRET` | Required for `MCP_TRANSPORT=sse` | All SSE connections must include `Authorization: Bearer <value>`. Use a strong random secret (≥ 32 chars). |
| `MCP_ALLOWED_ORIGINS` | No | Comma-separated browser origins allowed to call SSE endpoints. Requests with an `Origin` header are rejected unless listed. |
| `MCP_ALLOW_INSECURE_UPSTREAMS` | No | Defaults to `false`. Allows non-loopback `http://` upstream URLs only when explicitly set to `true`. |

### Minimal setup (GoGoMail only)

Only two variables are required. The server starts without Suppo or GitHub — calling those tools returns a descriptive "not configured" error rather than crashing.

```bash
export GOGOMAIL_ADMIN_URL=https://mail.example.com
export GOGOMAIL_ADMIN_KEY=your-admin-token
node dist/index.js
```

### Full stack

```bash
export GOGOMAIL_ADMIN_URL=https://mail.example.com
export GOGOMAIL_ADMIN_KEY=your-admin-token
export SUPPO_API_URL=https://support.example.com
export SUPPO_API_KEY=crn_live_...
export GITHUB_TOKEN=ghp_...
export GITHUB_REPO=parkjangwon/gogomail
node dist/index.js
```

---

## Running the Server

### Claude Desktop (stdio)

Claude Desktop spawns the MCP server as a child process. Edit `~/Library/Application Support/Claude/claude_desktop_config.json`:

**GoGoMail only:**

```json
{
  "mcpServers": {
    "gogomail-manage-mcp": {
      "command": "node",
      "args": ["/absolute/path/to/apps/gogomail-manage-mcp/dist/index.js"],
      "env": {
        "GOGOMAIL_ADMIN_URL": "https://mail.example.com",
        "GOGOMAIL_ADMIN_KEY": "your-admin-token"
      }
    }
  }
}
```

**Full stack (GoGoMail + Suppo + GitHub):**

```json
{
  "mcpServers": {
    "gogomail-manage-mcp": {
      "command": "node",
      "args": ["/absolute/path/to/apps/gogomail-manage-mcp/dist/index.js"],
      "env": {
        "GOGOMAIL_ADMIN_URL": "https://mail.example.com",
        "GOGOMAIL_ADMIN_KEY": "your-admin-token",
        "SUPPO_API_URL": "https://support.example.com",
        "SUPPO_API_KEY": "crn_live_...",
        "GITHUB_TOKEN": "ghp_...",
        "GITHUB_REPO": "parkjangwon/gogomail"
      }
    }
  }
}
```

After saving, restart Claude Desktop. You should see the hammer icon (🔨) in the chat UI indicating MCP tools are loaded.

### Autonomous Agent (HTTP + SSE)

Start the server in SSE mode so a remote agent can connect:

```bash
MCP_SECRET=your-strong-random-secret \
MCP_TRANSPORT=sse \
MCP_PORT=3100 \
GOGOMAIL_ADMIN_URL=https://mail.example.com \
GOGOMAIL_ADMIN_KEY=your-admin-token \
node dist/index.js
```

The server exposes:
- `GET  http://localhost:3100/sse` — SSE stream; the agent connects here first
- `POST http://localhost:3100/messages?sessionId=<id>` — the agent sends tool calls here

> **Security:** `MCP_SECRET` is required in SSE mode; the server refuses to start without it.
> Every request must include `Authorization: Bearer <MCP_SECRET>`.
> By default the SSE server binds to `127.0.0.1`; set `MCP_HOST` deliberately when exposing it through a private interface or reverse proxy.

---

## Tool Reference

All tool names follow the pattern `{provider}_{action}_{object}`. Every GoGoMail action (write operation) requires a human-readable `reason` and is audit-logged — either as an internal comment on the Suppo ticket referenced by `ticketId`, or as a standalone audit ticket created automatically when `ticketId` is omitted. Irreversible deletes also require an exact `confirm` phrase. When Suppo is not configured, the audit record is written to stderr.

### GoGoMail Admin (49)

#### User & Directory

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_search_principals` | `GET /admin/v1/directory/principals?q=` | Search users, groups, or aliases by email or name. **Start here** when you only know an email address. |
| `gogomail_list_users` | `GET /admin/v1/users` | List users, filtered by `domainId` and/or `status`. |
| `gogomail_get_user` | `GET /admin/v1/users/{id}` | Get full user record: status, role, quota, domain. |
| `gogomail_get_user_quota` | `GET /admin/v1/users/{id}/quota` | Get quota allocation and current usage in bytes. |

#### Companies & Domains

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_list_companies` | `GET /admin/v1/companies` | List all tenant companies. |
| `gogomail_get_company` | `GET /admin/v1/companies/{id}` | Get company details by ID. |
| `gogomail_list_domains` | `GET /admin/v1/domains` | List domains; filter by `companyId`, `status`, `dnsStatus`. |
| `gogomail_get_domain_settings` | `GET /admin/v1/domains/{id}/settings` | Get domain config: TLS policy, per-user quota, IP allowlist, 2FA, session timeout, password policy, and invite/reset policy. |
| `gogomail_check_domain_dns` | `GET /admin/v1/domains/{id}/dns-check` | Check DNS record verification (SPF, DKIM, DMARC, MX). Use to diagnose mail failures caused by DNS misconfiguration. |

#### Mail Flow Diagnostics

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_list_mail_flow_logs` | `GET /admin/v1/mail-flow-logs` | Search mail flow logs. Filter by `userId`, `companyId`, `domainId`, `messageId`, `fromAddr`, `toAddr`, `direction` (`inbound`/`outbound`), `flowStatus` (`delivered`/`bounced`/`deferred`/`rejected`/`quarantined`/`expired`), `since`, `until`, `limit`. |
| `gogomail_get_mail_flow_stats` | `GET /admin/v1/mail-flow-logs/stats` | Aggregated delivery counts by status for a time window. Good for spotting bulk failure spikes. |

#### Delivery Failures

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_list_delivery_attempts` | `GET /admin/v1/delivery-attempts` | List delivery attempts with per-hop error details. Filter by `messageId`, `status`, `recipientDomain`, `sender`, `since`. |
| `gogomail_list_exhausted_deliveries` | `GET /admin/v1/delivery-attempts/exhausted` | Messages that have exhausted all automatic retries and need manual action. |

#### Dead Letter Queue (DLQ)

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_list_dlq` | `GET /admin/v1/dlq?stream=` | List entries in a DLQ stream. `stream` is required. |
| `gogomail_delete_dlq_entry` | `DELETE /admin/v1/dlq/{id}?stream=` | Discard a stuck DLQ entry. Requires `reason` and `confirm="delete <stream>/<id>"`. *(audit logged)* |

#### Outbox Recovery

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_retry_outbox` | `POST /admin/v1/outbox/{id}/retry` | Manually retry a stuck outbox message by its ID. *(audit logged)* |

#### Suppression List

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_list_suppression_list` | `GET /admin/v1/suppression-list` | Find suppressed addresses. Filter by `email`, `domainId`, `reason` (`bounce`/`complaint`/`manual`). |
| `gogomail_remove_suppression_entry` | `DELETE /admin/v1/suppression-list/{id}` | Remove an address from the suppression list so it can receive mail again. *(audit logged)* |

#### Quota Management

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_list_quota_usage` | `GET /admin/v1/quota-usage` | List quota usage; pass `overLimit=true` to find users who have exceeded their allocation. |
| `gogomail_list_quota_alerts` | `GET /admin/v1/quota-alerts` | List triggered quota threshold alerts. |

#### User Actions (all audit-logged)

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_send_invite_email` | `POST /admin/v1/users/{id}/invite` | Send a password setup invitation email. Use for password reset requests. |
| `gogomail_update_user_status` | `PATCH /admin/v1/users/{id}/status` | Set account status: `active`, `suspended`, or `disabled`. |
| `gogomail_update_user_quota` | `PATCH /admin/v1/users/{id}/quota` | Set storage quota in bytes. |
| `gogomail_update_user_role` | `PATCH /admin/v1/users/{id}/role` | Change role: `user`, `company_admin`, or `system_admin`. |
| `gogomail_update_user_recovery_email` | `PATCH /admin/v1/users/{id}/recovery-email` | Update the recovery email address. |
| `gogomail_create_user` | `POST /admin/v1/users` | Create a new user account in a domain. |
| `gogomail_delete_user` | `DELETE /admin/v1/users/{id}` | Permanently delete a user. Requires `reason` and `confirm="delete <userId>"`. **Irreversible.** |
| `gogomail_update_domain_settings` | `PUT /admin/v1/domains/{id}/settings` | Update domain config: TLS policy, per-user quota, IP allowlist, 2FA, session timeout, password policy, and invite/reset policy. Omitted fields are preserved by merging with current settings first. |

#### Session Management

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_list_company_sessions` | `GET /admin/v1/companies/{id}/sessions` | List all active login sessions for a company. |
| `gogomail_revoke_company_session` | `DELETE /admin/v1/companies/{id}/sessions/{userId}` | Force-logout a specific user. *(audit logged)* |

#### Security & Monitoring

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_get_spam_filter` | `GET /admin/v1/companies/{id}/security/spam-filter` | Get the spam filter policy for a company. |
| `gogomail_get_spam_filter_events` | `GET /admin/v1/companies/{id}/security/spam-filter/events` | Get recent spam filter events. Use to investigate false positives. |
| `gogomail_list_dkim_keys` | `GET /admin/v1/dkim-keys` | List DKIM signing keys. Pass `domainId` to check a specific domain. |
| `gogomail_get_alert_events` | `GET /admin/v1/companies/{id}/alert-events` | Get system alert events for a company. |
| `gogomail_get_audit_logs` | `GET /admin/v1/audit-logs` | Get admin audit logs. Filter by `userId`, `companyId`, `from`, `to`. |

#### System Health

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_check_health` | `GET /admin/v1/health` | Check system health and component availability. |
| `gogomail_get_queue_stats` | `GET /admin/v1/queue` | Get mail queue depth and processing stats. High depth indicates a backlog. |

#### Admin Console Parity

| Tool | Method + Path | Description |
|---|---|---|
| `gogomail_admin_api_request` | Allowlisted `/admin/v1/...` | Guarded bridge for documented admin-console routes not yet represented by a dedicated typed tool. Read calls are direct; writes require `reason` and audit logging; deletes require `confirm="DELETE <path>"`. |
| `gogomail_list_org_units` | `GET /admin/v1/organization/units` | List organization units for a company. |
| `gogomail_get_org_hierarchy` | `GET /admin/v1/organization/hierarchy` | Get the organization hierarchy tree for a company. |
| `gogomail_list_user_org_memberships` | `GET /admin/v1/organization/members` | List a user's organization memberships, including role/title metadata. |
| `gogomail_assign_user_org_membership` | `POST /admin/v1/organization/members` | Assign a user to an organization unit with optional role/title. *(audit logged)* |
| `gogomail_update_org_membership` | `PATCH /admin/v1/organization/members/{id}` | Update organization membership role/title metadata. *(audit logged)* |
| `gogomail_remove_org_membership` | `DELETE /admin/v1/organization/members/{id}` | Remove an organization membership. Requires `confirm="remove org membership <membershipId>"`. *(audit logged)* |
| `gogomail_get_security_policy` | `GET /admin/v1/{companies|domains}/{id}/security/...` | Get company/domain security policy documents including IP, auth, retention, session, rate-limit, DMARC/SPF, SMTP, MCP, and spam-filter policy. |
| `gogomail_update_security_policy` | `PUT /admin/v1/{companies|domains}/{id}/security/...` | Update a company/domain security policy JSON document. *(audit logged)* |
| `gogomail_get_spam_filter_policy` | `GET /admin/v1/{companies|domains}/{id}/security/spam-filter` | Get company- or domain-scoped spam filter policy. |
| `gogomail_update_spam_filter_policy` | `PUT /admin/v1/{companies|domains}/{id}/security/spam-filter` | Update company- or domain-scoped spam filter policy. *(audit logged)* |
| `gogomail_get_spam_filter_stats` | `GET /admin/v1/companies/{id}/security/spam-filter/stats` | Get spam filter statistics, optionally filtered by domain. |

---

### Suppo Helpdesk (10)

Requires `SUPPO_API_URL` and `SUPPO_API_KEY`. Returns a "not configured" error otherwise.

| Tool | Description |
|---|---|
| `suppo_list_tickets` | List tickets. Filter by `status` (`open`/`pending`/`closed`/`resolved`) and/or `priority` (`low`/`normal`/`high`/`urgent`). |
| `suppo_get_ticket` | Get full ticket detail including the complete comment history. |
| `suppo_search_tickets` | Search tickets by `customerEmail` or keyword (`query`). |
| `suppo_create_ticket` | Create a new support ticket. Fields: `customerName`, `customerEmail`, `subject`, `description`, `priority`. |
| `suppo_update_ticket` | Update ticket `status` and/or `priority`. |
| `suppo_add_comment` | Add a customer-visible reply or an internal memo (`internal: true`). |
| `suppo_assign_ticket` | Assign a ticket to an agent by `assigneeId`. |
| `suppo_list_agents` | List all available support agents (id, name, email). Use to find an `assigneeId`. |
| `suppo_search_kb` | Full-text search over published KB articles. |
| `suppo_create_kb_article` | Create a new KB article. Useful after resolving a recurring issue. |

---

### GitHub Issues (6)

Requires `GITHUB_TOKEN`. Returns a "not configured" error otherwise.

| Tool | Description |
|---|---|
| `github_search_issues` | Full-text search across issues in the configured repo. User-provided `repo:`, `org:`, and `user:` qualifiers are ignored so searches cannot escape `GITHUB_REPO`. |
| `github_get_issue` | Get issue detail and comment thread by issue number. |
| `github_list_issues` | List issues. Filter by `state` (`open`/`closed`/`all`) and/or `labels`. |
| `github_create_issue` | Create a bug report or feature request with `title`, `body`, and optional `labels`. |
| `github_add_comment` | Add a comment to an existing issue. |
| `github_update_issue` | Update issue `state` (`open`/`closed`) and/or `labels`. |

---

## Workflow Examples

These are representative patterns an AI agent would follow. The agent calls tools in sequence, reading results to decide the next step.

### Scenario 1 — Customer cannot send email

```
User: "alice@example.com says she can't send email since yesterday."

Agent:
  1. gogomail_search_principals(q: "alice@example.com")
     → finds userId: "usr_abc123", domainId: "dom_xyz"

  2. gogomail_get_user(userId: "usr_abc123")
     → status: "active", role: "user" — account looks fine

  3. gogomail_list_mail_flow_logs(
       userId: "usr_abc123",
       direction: "outbound",
       flowStatus: "rejected",
       since: "2026-05-23T00:00:00Z"
     )
     → 12 entries: "550 5.7.1 Message rejected due to SPF failure"

  4. gogomail_check_domain_dns(domainId: "dom_xyz")
     → SPF: FAIL — TXT record missing "include:mail.example.com"

  Agent response: "Alice's outbound mail is being rejected because example.com's
  SPF record doesn't include the GoGoMail sending server. The DNS record at
  example.com needs a TXT entry: v=spf1 include:mail.example.com ~all.
  I can't edit DNS directly, but here are the exact instructions for your
  DNS provider."
```

### Scenario 2 — User locked out, requests password reset

```
User: "bob@example.com forgot his password."

Agent:
  1. gogomail_search_principals(q: "bob@example.com")
     → userId: "usr_def456"

  2. gogomail_get_user(userId: "usr_def456")
     → status: "active" — safe to send invite

  3. gogomail_send_invite_email(
       userId: "usr_def456",
       reason: "Verified active account and customer requested password reset",
       ticketId: "tkt-789"
     )
     → invite_token created; email dispatched
     → audit memo written to ticket tkt-789 automatically

  Agent response: "Password reset email sent to bob@example.com.
  The link expires in 24 hours."
```

### Scenario 3 — Mail delivery failures piling up

```
User: "We're seeing a lot of bounces. What's going on?"

Agent:
  1. gogomail_get_queue_stats()
     → queues: { outbound: { depth: 4821, processing: 12 } }
     — queue depth is elevated

  2. gogomail_get_mail_flow_stats(
       direction: "outbound",
       since: "2026-05-24T00:00:00Z"
     )
     → delivered: 1203, deferred: 892, bounced: 441

  3. gogomail_list_exhausted_deliveries(since: "2026-05-24T00:00:00Z")
     → 38 messages exhausted, all to @hotmail.com

  4. gogomail_list_delivery_attempts(
       recipientDomain: "hotmail.com",
       status: "failed",
       since: "2026-05-24T00:00:00Z"
     )
     → error: "421 4.7.0 Too many connections from your IP"

  Agent response: "GoGoMail is being rate-limited by Hotmail (421 Too many
  connections). 38 messages are exhausted. Recommend: check the outbound IP
  reputation and consider retrying after a cooldown. I can retry the outbox
  messages individually — want me to proceed?"
```

### Scenario 4 — Suspected account compromise

```
User: "We think charlie@example.com's account was accessed without authorization."

Agent:
  1. gogomail_search_principals(q: "charlie@example.com")
     → userId: "usr_ghi789", companyId: "co_123"

  2. gogomail_list_company_sessions(companyId: "co_123")
     → 3 sessions for charlie: IP 203.0.113.5 (unknown country), last seen 2 min ago

  3. gogomail_revoke_company_session(
       companyId: "co_123",
       userId: "usr_ghi789",
       reason: "Suspected account compromise; unknown active session observed",
       ticketId: "tkt-900"
     )
     → session revoked, audit logged to ticket tkt-900

  4. gogomail_get_audit_logs(userId: "usr_ghi789", from: "2026-05-20T00:00:00Z")
     → 14 login events, 9 from the foreign IP

  5. gogomail_send_invite_email(
       userId: "usr_ghi789",
       reason: "Credential reset after suspected account compromise",
       ticketId: "tkt-900"
     )
     → password reset email sent

  Agent response: "Charlie's sessions have been revoked and a password reset
  email has been sent. Audit logs show 9 logins from 203.0.113.5 over the
  past 4 days. I've logged everything to ticket tkt-900. You may want to
  review the email content for data exfiltration."
```

---

## Audit Trail

Every GoGoMail **action** tool (write operation) requires a `reason` input and automatically records an audit memo after execution. The memo contains:

- Tool name
- Target entity (email, userId, domainId, etc.)
- Operator/agent-provided reason
- Before → after state where applicable
- UTC timestamp

**Where the memo is written:**

| Condition | Behavior |
|---|---|
| `ticketId` provided + Suppo configured | Internal comment added to that Suppo ticket |
| `ticketId` omitted + Suppo configured | Standalone audit ticket created automatically |
| Suppo not configured (any case) | Memo written to `stderr` as structured log |

Audit writes are awaited but best-effort: a failure to write the memo does **not** roll back or fail the already-completed action. The tool result includes an `audit` object (`written` or `failed`) so the agent can escalate missing evidence without retrying a duplicate mutation.

Irreversible delete tools require a second confirmation field:
- `gogomail_delete_user`: `confirm` must exactly equal `delete <userId>`.
- `gogomail_delete_dlq_entry`: `confirm` must exactly equal `delete <stream>/<id>`.

---

## Security Considerations

**Admin key protection**
The `GOGOMAIL_ADMIN_KEY` grants full Admin API access. Treat it like a root password:
- Store it in a secrets manager (AWS Secrets Manager, Vault, 1Password), not in plain env files committed to source control.
- Rotate it if it is ever exposed.

**SSE endpoint authentication**
`MCP_SECRET` enables Bearer token authentication and is mandatory when `MCP_TRANSPORT=sse`. All SSE connections (`GET /sse`) and tool call requests (`POST /messages`) must include:
```
Authorization: Bearer <MCP_SECRET>
```
Token comparison uses `crypto.timingSafeEqual` to prevent timing attacks. Without `MCP_SECRET`, SSE mode exits during startup.

Recommended for production:
- Set a strong `MCP_SECRET` (≥ 32 random bytes, base64-encoded)
- Keep the default `MCP_HOST=127.0.0.1`, or bind only to a private interface behind a reverse proxy
- Set `MCP_ALLOWED_ORIGINS` if browser-based agents must call the SSE endpoint
- Use HTTPS upstream URLs; non-loopback `http://` upstreams are rejected unless `MCP_ALLOW_INSECURE_UPSTREAMS=true`
- Never expose the port to the public internet

**Principle of least privilege**
If you only need a subset of tools (e.g., read-only diagnostics), you can safely omit `SUPPO_API_KEY` / `GITHUB_TOKEN` and use a GoGoMail admin key with reduced permissions.

**Audit log integrity**
The audit trail written to Suppo tickets is append-only from the agent's perspective — agents cannot delete Suppo comments via these tools. For forensic purposes, cross-reference with `gogomail_get_audit_logs` which reads from GoGoMail's own immutable audit log.
