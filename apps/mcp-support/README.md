# GoGoMail Support MCP Server

Autonomous AI support agent MCP server for GoGoMail. Exposes **50 tools** across Suppo (helpdesk), GoGoMail Admin API, and GitHub Issues — designed for unmanned 24/7 support operation.

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

> **Security note:** The SSE endpoint has no built-in authentication. Bind to a private interface or firewall the port. Do not expose port 3100 to the public internet.

## Tools (50 total)

### Suppo Helpdesk (10)
| Tool | Description |
|---|---|
| `suppo_list_tickets` | List tickets with optional status/priority filter |
| `suppo_get_ticket` | Get ticket detail + full comment history |
| `suppo_search_tickets` | Search by customer email or keyword |
| `suppo_create_ticket` | Create new support ticket |
| `suppo_update_ticket` | Change status, priority, or subject |
| `suppo_add_comment` | Add customer reply or internal memo |
| `suppo_assign_ticket` | Assign ticket to an agent |
| `suppo_list_agents` | List assignable support agents |
| `suppo_search_kb` | Search published knowledge base articles |
| `suppo_create_kb_article` | Create KB article from a resolved case |

### GoGoMail Admin (34)

#### User & Directory
| Tool | Description |
|---|---|
| `gogomail_search_principals` | Search users/groups/aliases by email or name |
| `gogomail_list_users` | List users filtered by domain or status |
| `gogomail_get_user` | Get full user details (status, role, quota, domain) |
| `gogomail_get_user_quota` | Get quota allocation and current usage |

#### Companies & Domains
| Tool | Description |
|---|---|
| `gogomail_list_companies` | List all companies |
| `gogomail_get_company` | Get company details by ID |
| `gogomail_list_domains` | List domains with optional filters |
| `gogomail_get_domain_settings` | Get domain config (SPF/DKIM/DMARC/catch-all) |
| `gogomail_check_domain_dns` | Check DNS record verification status |

#### Mail Flow Diagnostics
| Tool | Description |
|---|---|
| `gogomail_list_mail_flow_logs` | Search mail logs by user/addr/status/time range |
| `gogomail_get_mail_flow_stats` | Aggregated delivery stats (counts by status) |

#### Delivery Failures
| Tool | Description |
|---|---|
| `gogomail_list_delivery_attempts` | List delivery attempts with error details |
| `gogomail_list_exhausted_deliveries` | Messages that exhausted all retries |

#### Dead Letter Queue
| Tool | Description |
|---|---|
| `gogomail_list_dlq` | List messages stuck in a DLQ stream |
| `gogomail_delete_dlq_entry` | Discard a stuck DLQ message *(action, audit logged)* |

#### Outbox Recovery
| Tool | Description |
|---|---|
| `gogomail_retry_outbox` | Manually retry a stuck outbox message *(action, audit logged)* |

#### Suppression List
| Tool | Description |
|---|---|
| `gogomail_list_suppression_list` | Find suppressed email addresses |
| `gogomail_remove_suppression_entry` | Remove email from suppression list *(action, audit logged)* |

#### Quota Management
| Tool | Description |
|---|---|
| `gogomail_list_quota_usage` | List quota usage; filter `overLimit=true` for at-risk users |
| `gogomail_list_quota_alerts` | List triggered quota threshold alerts |

#### User Actions
| Tool | Description |
|---|---|
| `gogomail_send_invite_email` | Send password setup link to user *(action, audit logged)* |
| `gogomail_update_user_status` | Change account status *(action, audit logged)* |
| `gogomail_update_user_quota` | Adjust user storage quota *(action, audit logged)* |
| `gogomail_update_user_role` | Change user role *(action, audit logged)* |
| `gogomail_update_domain_settings` | Update domain config *(action, audit logged)* |

#### Session Management
| Tool | Description |
|---|---|
| `gogomail_list_company_sessions` | List all active sessions for a company |
| `gogomail_revoke_company_session` | Force-logout a specific user *(action, audit logged)* |

#### Security & Monitoring
| Tool | Description |
|---|---|
| `gogomail_get_spam_filter` | Get company spam filter policy |
| `gogomail_get_spam_filter_events` | Get recent spam filter events |
| `gogomail_list_dkim_keys` | List DKIM signing keys by domain |
| `gogomail_get_alert_events` | Get system alert events for a company |
| `gogomail_get_audit_logs` | Get admin audit logs with time filter |

#### System Health
| Tool | Description |
|---|---|
| `gogomail_check_health` | Check system health and component status |
| `gogomail_get_queue_stats` | Get mail queue depth and processing stats |

### GitHub Issues (6)
| Tool | Description |
|---|---|
| `github_search_issues` | Full-text search across issues |
| `github_get_issue` | Get issue detail and comments |
| `github_list_issues` | List issues with state/label filters |
| `github_create_issue` | Create bug report or feature request |
| `github_add_comment` | Add comment to an issue |
| `github_update_issue` | Update issue state or labels |

## Audit Trail

All GoGoMail **action** tools (marked *action, audit logged* above) automatically write an internal Suppo memo after execution, recording: tool name, target entity, before→after state, and UTC timestamp.

Pass `ticketId` to attach the memo to an active ticket. Without `ticketId`, a standalone audit ticket is auto-created.

## Suppo APIs Required

These endpoints must exist in the Suppo project:

| Method | Path | Used by |
|---|---|---|
| `POST` | `/api/public/tickets/{id}/comments` | `suppo_add_comment` |
| `GET` | `/api/public/agents` | `suppo_list_agents` |
| `GET` | `/api/public/kb/articles/search?q=` | `suppo_search_kb` |
| `POST` | `/api/public/kb/articles` | `suppo_create_kb_article` |

All four endpoints are implemented in `parkjangwon/suppo` as of 2026-05-23.
