# GoGoMail Support MCP Server

Autonomous AI support agent MCP server for GoGoMail. Exposes 34 tools across Suppo (helpdesk), GoGoMail Admin API, and GitHub Issues.

## Quick Start

### Build

```bash
cd apps/mcp-support
npm install
npm run build
```

### Claude Desktop (stdio)

Add to `~/Library/Application Support/Claude/claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "gogomail-support": {
      "command": "node",
      "args": ["/absolute/path/to/apps/mcp-support/dist/index.js"],
      "env": {
        "GOGOMAIL_ADMIN_URL": "https://admin.gogomail.io",
        "GOGOMAIL_ADMIN_KEY": "...",
        "SUPPO_API_URL": "https://support.gogomail.io",
        "SUPPO_API_KEY": "crn_live_...",
        "GITHUB_TOKEN": "ghp_...",
        "GITHUB_REPO": "parkjangwon/gogomail"
      }
    }
  }
}
```

### Remote Autonomous Agent (HTTP+SSE)

```bash
GOGOMAIL_ADMIN_URL=... \
GOGOMAIL_ADMIN_KEY=... \
SUPPO_API_URL=... \
SUPPO_API_KEY=... \
GITHUB_TOKEN=... \
MCP_TRANSPORT=sse \
MCP_PORT=3100 \
node apps/mcp-support/dist/index.js
```

## Tools (34 total)

### Suppo (10)
- `suppo_list_tickets` — list tickets with optional status/priority filter
- `suppo_get_ticket` — get ticket detail + comment history
- `suppo_search_tickets` — search by customer email or keyword
- `suppo_create_ticket` — create new ticket
- `suppo_update_ticket` — change status/priority
- `suppo_add_comment` — add customer reply or internal memo
- `suppo_assign_ticket` — assign to an agent
- `suppo_list_agents` — list assignable agents
- `suppo_search_kb` — search knowledge base
- `suppo_create_kb_article` — create KB article from resolved case

### GoGoMail Admin (18)

**Read (9):** `gogomail_find_user` · `gogomail_get_user` · `gogomail_get_user_quota` · `gogomail_get_mail_logs` · `gogomail_trace_message` · `gogomail_get_delivery_attempts` · `gogomail_get_audit_logs` · `gogomail_list_user_sessions` · `gogomail_check_health`

**Action (9):** `gogomail_reset_password` · `gogomail_update_user_status` · `gogomail_update_user_quota` · `gogomail_revoke_sessions` · `gogomail_update_user_role` · `gogomail_get_company` · `gogomail_get_domain_settings` · `gogomail_update_domain_settings` · `gogomail_get_alert_events`

### GitHub (6)
- `github_search_issues` · `github_get_issue` · `github_list_issues` · `github_create_issue` · `github_add_comment` · `github_update_issue`

## Audit Trail

All GoGoMail action tools auto-write an internal Suppo comment after execution. Pass `ticketId` to attach the audit memo to the active ticket. Without `ticketId`, a standalone audit ticket is created automatically.

## Suppo New APIs Required

These endpoints must exist in the Suppo project before end-to-end use:

| Method | Path | Used by |
|---|---|---|
| `POST` | `/api/public/tickets/{id}/comments` | `suppo_add_comment` |
| `GET` | `/api/public/agents` | `suppo_list_agents` |
| `GET` | `/api/public/kb/articles/search?q=` | `suppo_search_kb` |
| `POST` | `/api/public/kb/articles` | `suppo_create_kb_article` |
