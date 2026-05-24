import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient } from "../clients/gogomail.js";
import type { SuppoClient } from "../clients/suppo.js";

export type OptionalSuppo = SuppoClient | null;

// ── Tool definitions ────────────────────────────────────────────

export const toolDefinitions: Tool[] = [
  // ── User & directory lookup ──────────────────────────────────
  {
    name: "gogomail_search_principals",
    description:
      "Search for users, groups, or aliases by email address or name. Use this as the first step to find a user's ID before calling other user tools.",
    inputSchema: {
      type: "object",
      properties: {
        q: { type: "string", description: "Email address or name to search for", maxLength: 200 },
        companyId: { type: "string", description: "Filter by company ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        domainId: { type: "string", description: "Filter by domain ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        limit: { type: "number", description: "Max results (default 20)", minimum: 1, maximum: 200 },
      },
      required: ["q"],
    },
  },
  {
    name: "gogomail_list_users",
    description:
      "List users, optionally filtered by domain and/or status. Returns users with has_more flag.",
    inputSchema: {
      type: "object",
      properties: {
        domainId: { type: "string", description: "Filter by domain ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        status: { type: "string", description: "Filter by user status", enum: ["active", "suspended", "disabled"] },
        limit: { type: "number", minimum: 1, maximum: 200 },
      },
    },
  },
  {
    name: "gogomail_get_user",
    description: "Get full user details including status, role, quota, and domain.",
    inputSchema: {
      type: "object",
      properties: { userId: { type: "string", description: "User entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_get_user_quota",
    description: "Get storage quota allocation and current usage bytes for a user.",
    inputSchema: {
      type: "object",
      properties: { userId: { type: "string", description: "User entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
      required: ["userId"],
    },
  },
  // ── Company & domain lookup ──────────────────────────────────
  {
    name: "gogomail_list_companies",
    description: "List all companies. Filter by status if needed.",
    inputSchema: {
      type: "object",
      properties: {
        status: { type: "string", description: "Filter by company status", enum: ["active", "suspended"] },
        limit: { type: "number", minimum: 1, maximum: 200 },
      },
    },
  },
  {
    name: "gogomail_get_company",
    description: "Get company details by company ID.",
    inputSchema: {
      type: "object",
      properties: { companyId: { type: "string", description: "Company entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_list_domains",
    description: "List domains. Filter by company, status, or DNS verification status.",
    inputSchema: {
      type: "object",
      properties: {
        companyId: { type: "string", description: "Filter by company ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        status: { type: "string", description: "Filter by domain status", enum: ["active", "suspended"] },
        dnsStatus: { type: "string", description: "Filter by DNS verification status", enum: ["verified", "unverified", "partial"] },
        limit: { type: "number", minimum: 1, maximum: 200 },
      },
    },
  },
  {
    name: "gogomail_get_domain_settings",
    description:
      "Get domain configuration: SPF, DKIM, DMARC, catch-all, and max message size.",
    inputSchema: {
      type: "object",
      properties: { domainId: { type: "string", description: "Domain entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
      required: ["domainId"],
    },
  },
  {
    name: "gogomail_check_domain_dns",
    description:
      "Check DNS record verification status for a domain (SPF, DKIM, DMARC, MX). Use to diagnose mail delivery failures caused by DNS misconfiguration.",
    inputSchema: {
      type: "object",
      properties: { domainId: { type: "string", description: "Domain entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
      required: ["domainId"],
    },
  },
  // ── Mail flow diagnostics ────────────────────────────────────
  {
    name: "gogomail_list_mail_flow_logs",
    description:
      "Search mail flow logs across all users. Filter by direction, status, sender, recipient, or time range. Essential for diagnosing delivery issues.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string", description: "Filter by user ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        companyId: { type: "string", description: "Filter by company ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        domainId: { type: "string", description: "Filter by domain ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        messageId: { type: "string", description: "Filter by message ID" },
        fromAddr: { type: "string", description: "Sender email address", format: "email", maxLength: 254 },
        toAddr: { type: "string", description: "Recipient email address", format: "email", maxLength: 254 },
        direction: { type: "string", description: "Mail flow direction", enum: ["inbound", "outbound"] },
        flowStatus: {
          type: "string",
          description: "Mail flow delivery status",
          enum: ["delivered", "bounced", "deferred", "rejected", "quarantined", "expired"],
        },
        since: { type: "string", description: "ISO 8601 start time" },
        until: { type: "string", description: "ISO 8601 end time" },
        limit: { type: "number", description: "Max results (default 20, max 100)", minimum: 1, maximum: 200 },
      },
    },
  },
  {
    name: "gogomail_get_mail_flow_stats",
    description:
      "Get aggregated mail flow statistics (counts by status and direction) for a time range. Good for spotting delivery anomalies.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string", description: "Filter by user ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        companyId: { type: "string", description: "Filter by company ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        domainId: { type: "string", description: "Filter by domain ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        direction: { type: "string", description: "Mail flow direction", enum: ["inbound", "outbound"] },
        since: { type: "string", description: "ISO 8601 start time" },
        until: { type: "string", description: "ISO 8601 end time" },
      },
    },
  },
  // ── Delivery attempts ────────────────────────────────────────
  {
    name: "gogomail_list_delivery_attempts",
    description:
      "List delivery attempts with error details. Filter by message ID, status, recipient domain, or sender. Use to diagnose why a specific message failed.",
    inputSchema: {
      type: "object",
      properties: {
        messageId: { type: "string", description: "Filter by message ID" },
        status: { type: "string", description: "Delivery attempt status", enum: ["pending", "success", "failed", "exhausted"] },
        recipientDomain: { type: "string", description: "Filter by recipient domain" },
        sender: { type: "string", description: "Filter by sender email address", format: "email", maxLength: 254 },
        since: { type: "string", description: "ISO 8601 start time" },
        limit: { type: "number", minimum: 1, maximum: 200 },
      },
    },
  },
  {
    name: "gogomail_list_exhausted_deliveries",
    description:
      "List messages that have exhausted all delivery retries. These require manual intervention — either retry (gogomail_retry_outbox) or remove from DLQ.",
    inputSchema: {
      type: "object",
      properties: {
        messageId: { type: "string", description: "Filter by message ID" },
        recipientDomain: { type: "string", description: "Filter by recipient domain" },
        sender: { type: "string", description: "Filter by sender email address", format: "email", maxLength: 254 },
        since: { type: "string", description: "ISO 8601 start time" },
        limit: { type: "number", minimum: 1, maximum: 200 },
      },
    },
  },
  // ── DLQ management ──────────────────────────────────────────
  {
    name: "gogomail_list_dlq",
    description:
      "List messages stuck in the Dead Letter Queue for a stream. DLQ entries represent messages that failed processing after all retries.",
    inputSchema: {
      type: "object",
      properties: {
        stream: { type: "string", description: "DLQ stream name (required)", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        count: { type: "number", description: "Max entries to return", minimum: 1, maximum: 500 },
      },
      required: ["stream"],
    },
  },
  {
    name: "gogomail_delete_dlq_entry",
    description:
      "Delete a message from the Dead Letter Queue. Use when a DLQ entry cannot be retried and should be discarded. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        stream: { type: "string", description: "DLQ stream name", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        id: { type: "string", description: "DLQ entry ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["stream", "id"],
    },
  },
  // ── Outbox retry ─────────────────────────────────────────────
  {
    name: "gogomail_retry_outbox",
    description:
      "Manually retry a stuck outbox message by its outbox ID. PREREQUISITE: use gogomail_list_exhausted_deliveries or gogomail_list_delivery_attempts to find the outbox ID first. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        id: { type: "string", description: "Outbox message ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["id"],
    },
  },
  // ── Suppression list ─────────────────────────────────────────
  {
    name: "gogomail_list_suppression_list",
    description:
      "List email addresses on the suppression list. Suppressed emails bounce silently — use this when a customer reports they're not receiving mail.",
    inputSchema: {
      type: "object",
      properties: {
        email: { type: "string", description: "Filter by specific email address", format: "email", maxLength: 254 },
        domainId: { type: "string", description: "Filter by domain ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        reason: { type: "string", description: "Suppression reason", enum: ["bounce", "complaint", "manual"] },
        limit: { type: "number", minimum: 1, maximum: 200 },
      },
    },
  },
  {
    name: "gogomail_remove_suppression_entry",
    description:
      "Remove an email from the suppression list so it can receive mail again. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        id: { type: "string", description: "Suppression entry ID from list_suppression_list", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["id"],
    },
  },
  // ── Quota management ─────────────────────────────────────────
  {
    name: "gogomail_list_quota_usage",
    description:
      "List quota usage across users/domains. Use over_limit=true to find users who have exceeded their quota.",
    inputSchema: {
      type: "object",
      properties: {
        domainId: { type: "string", description: "Filter by domain ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        overLimit: { type: "boolean", description: "Only show entities over quota limit" },
        limit: { type: "number", minimum: 1, maximum: 200 },
      },
    },
  },
  {
    name: "gogomail_list_quota_alerts",
    description: "List quota threshold alerts that have been triggered.",
    inputSchema: {
      type: "object",
      properties: {
        limit: { type: "number", minimum: 1, maximum: 200 },
      },
    },
  },
  // ── User actions ─────────────────────────────────────────────
  {
    name: "gogomail_send_invite_email",
    description:
      "Send a password setup invitation email to the user. Use for password reset requests. PREREQUISITE: call gogomail_get_user first to verify the account exists and is active. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string", description: "User entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_update_user_status",
    description:
      "Change account status to active, suspended, or disabled. PREREQUISITE: call gogomail_get_user first to confirm current status. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string", description: "User entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        status: { type: "string", description: "New user account status", enum: ["active", "suspended", "disabled"] },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId", "status"],
    },
  },
  {
    name: "gogomail_update_user_quota",
    description:
      "Adjust user storage quota in bytes. PREREQUISITE: call gogomail_get_user_quota first. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string", description: "User entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        quotaBytes: { type: "number", description: "New quota in bytes", minimum: 0, maximum: 10995116277760 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId", "quotaBytes"],
    },
  },
  {
    name: "gogomail_update_user_role",
    description:
      "Change a user's role. PREREQUISITE: call gogomail_get_user first to see current role. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string", description: "User entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        role: { type: "string", description: "New role to assign to the user" },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId", "role"],
    },
  },
  {
    name: "gogomail_update_user_recovery_email",
    description:
      "Update the recovery email address for a user account. Used when a customer loses access to their recovery email. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string", description: "User entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        recoveryEmail: { type: "string", description: "New recovery email address", format: "email", maxLength: 254 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId", "recoveryEmail"],
    },
  },
  {
    name: "gogomail_create_user",
    description:
      "Create a new user account in a domain. If password is provided, the user will be prompted to change it on first login. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        domainId: { type: "string", description: "Domain entity ID to create the user in", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        username: { type: "string", description: "Local part of the email address (before @)", maxLength: 64 },
        displayName: { type: "string", description: "User's full display name", maxLength: 256 },
        recoveryEmail: { type: "string", description: "Recovery email (optional)", format: "email", maxLength: 254 },
        password: { type: "string", description: "Initial password — user will be required to change it on first login", maxLength: 256 },
        quotaLimit: { type: "number", description: "Storage quota in bytes (optional, uses domain default)", minimum: 0, maximum: 10995116277760 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["domainId", "username", "displayName"],
    },
  },
  {
    name: "gogomail_delete_user",
    description:
      "Permanently delete a user account. THIS IS IRREVERSIBLE. PREREQUISITE: call gogomail_get_user first to confirm identity. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string", description: "User entity ID to permanently delete", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_update_domain_settings",
    description:
      "Update domain configuration (catch-all, SPF, DKIM, DMARC, max message size). PREREQUISITE: call gogomail_get_domain_settings first. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        domainId: { type: "string", description: "Domain entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        settings: {
          type: "object",
          description: "Partial domain settings to update",
        },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["domainId", "settings"],
    },
  },
  // ── Session management ───────────────────────────────────────
  {
    name: "gogomail_list_company_sessions",
    description:
      "List all active login sessions for all users in a company. Use for security investigations.",
    inputSchema: {
      type: "object",
      properties: { companyId: { type: "string", description: "Company entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_revoke_company_session",
    description:
      "Force-logout a specific user from a company. PREREQUISITE: call gogomail_list_company_sessions first to confirm the session exists. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        companyId: { type: "string", description: "Company entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        userId: { type: "string", description: "User entity ID whose session to revoke", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["companyId", "userId"],
    },
  },
  // ── Security & monitoring ────────────────────────────────────
  {
    name: "gogomail_get_spam_filter",
    description: "Get spam filter policy configuration for a company.",
    inputSchema: {
      type: "object",
      properties: { companyId: { type: "string", description: "Company entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_get_spam_filter_events",
    description:
      "Get recent spam filter events for a company. Use to investigate why legitimate mail is being quarantined.",
    inputSchema: {
      type: "object",
      properties: { companyId: { type: "string", description: "Company entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_list_dkim_keys",
    description:
      "List DKIM signing keys. Filter by domain to check if a domain has valid DKIM configured.",
    inputSchema: {
      type: "object",
      properties: { domainId: { type: "string", description: "Filter by domain ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
    },
  },
  {
    name: "gogomail_get_alert_events",
    description: "Get recent system alert events for a company.",
    inputSchema: {
      type: "object",
      properties: {
        companyId: { type: "string", description: "Company entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        limit: { type: "number", minimum: 1, maximum: 200 },
      },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_get_audit_logs",
    description:
      "Get admin audit logs for a user or company. Filter by time range to see what actions were taken.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string", description: "Filter by user ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        companyId: { type: "string", description: "Filter by company ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        from: { type: "string", description: "ISO 8601 start time" },
        to: { type: "string", description: "ISO 8601 end time" },
        limit: { type: "number", minimum: 1, maximum: 200 },
      },
    },
  },
  // ── System health ────────────────────────────────────────────
  {
    name: "gogomail_check_health",
    description: "Check GoGoMail system health status and component availability.",
    inputSchema: { type: "object", properties: {} },
  },
  {
    name: "gogomail_get_queue_stats",
    description:
      "Get mail queue depth and processing stats. High queue depth indicates a backlog or delivery bottleneck.",
    inputSchema: { type: "object", properties: {} },
  },
];

// ── Audit helper ────────────────────────────────────────────────

// Strip newlines so audit fields can't inject extra lines into the comment body
const sanitizeAuditField = (s: string) => s.replace(/[\r\n]/g, " ").slice(0, 500);

async function writeAuditComment(
  suppo: OptionalSuppo,
  ticketId: string | undefined,
  toolName: string,
  targetInfo: string,
  change: string,
): Promise<void> {
  const body = [
    `[자동 실행] ${toolName}`,
    `- 대상: ${sanitizeAuditField(targetInfo)}`,
    `- 변경: ${sanitizeAuditField(change)}`,
    `- 실행 시각: ${new Date().toISOString()}`,
  ].join("\n");

  if (!suppo) {
    console.error(`[audit] ${body}`);
    return;
  }

  if (ticketId) {
    await suppo.addComment(ticketId, { body, internal: true });
  } else {
    await suppo.createTicket({
      customerName: "System",
      customerEmail: "system@gogomail.io",
      subject: `[감사 기록] ${toolName}`,
      description: body,
      priority: "low",
    });
  }
}

// ── Zod schemas ─────────────────────────────────────────────────

// Allowlist ensures IDs used as URL path segments cannot contain traversal sequences.
// Real GoGoMail IDs are UUIDs or short slugs: alphanumeric + hyphen + underscore only.
const id = () =>
  z.string().max(128).regex(/^[A-Za-z0-9_-]+$/, "ID must be alphanumeric with hyphens/underscores only");
// Stream names (DLQ, outbox) — same character set; only used in query params but
// applying the same allowlist for defense in depth.
const stream = () =>
  z.string().max(128).regex(/^[A-Za-z0-9_-]+$/, "stream name must be alphanumeric with hyphens/underscores only");
const email = () => z.string().email().max(254);
const status32 = () => z.string().max(32);
const ts = () => z.string().max(64);
// Bounded page limit — prevents agents from issuing unbounded backend queries
const pageLimit = () => z.number().int().min(1).max(200).optional();

const SearchPrincipalsSchema = z.object({
  q: z.string().max(200),
  companyId: id().optional(),
  domainId: id().optional(),
  limit: pageLimit(),
});
const ListUsersSchema = z.object({
  domainId: id().optional(),
  status: status32().optional(),
  limit: pageLimit(),
});
const UserIdSchema = z.object({ userId: id() });
const CompanyIdSchema = z.object({ companyId: id() });
const ListCompaniesSchema = z.object({
  status: status32().optional(),
  limit: pageLimit(),
});
const ListDomainsSchema = z.object({
  companyId: id().optional(),
  status: status32().optional(),
  dnsStatus: status32().optional(),
  limit: pageLimit(),
});
const DomainIdSchema = z.object({ domainId: id() });
const ListMailFlowLogsSchema = z.object({
  userId: id().optional(),
  companyId: id().optional(),
  domainId: id().optional(),
  messageId: z.string().max(256).optional(),
  fromAddr: email().optional(),
  toAddr: email().optional(),
  direction: z.string().max(16).optional(),
  flowStatus: status32().optional(),
  since: ts().optional(),
  until: ts().optional(),
  limit: pageLimit(),
});
const MailFlowStatsSchema = z.object({
  userId: id().optional(),
  companyId: id().optional(),
  domainId: id().optional(),
  direction: z.string().max(16).optional(),
  since: ts().optional(),
  until: ts().optional(),
});
const ListDeliveryAttemptsSchema = z.object({
  messageId: z.string().max(256).optional(),
  status: status32().optional(),
  recipientDomain: z.string().max(253).optional(),
  sender: email().optional(),
  since: ts().optional(),
  limit: pageLimit(),
});
const ListExhaustedSchema = z.object({
  messageId: z.string().max(256).optional(),
  recipientDomain: z.string().max(253).optional(),
  sender: email().optional(),
  since: ts().optional(),
  limit: pageLimit(),
});
const DlqListSchema = z.object({
  stream: stream(),
  count: z.number().int().min(1).max(500).optional(),
});
const DlqDeleteSchema = z.object({
  stream: stream(),
  id: id(),
  ticketId: id().optional(),
});
const RetryOutboxSchema = z.object({
  id: id(),
  ticketId: id().optional(),
});
const ListSuppressionSchema = z.object({
  email: email().optional(),
  domainId: id().optional(),
  reason: status32().optional(),
  limit: pageLimit(),
});
const RemoveSuppressionSchema = z.object({
  id: id(),
  ticketId: id().optional(),
});
const ListQuotaUsageSchema = z.object({
  domainId: id().optional(),
  overLimit: z.boolean().optional(),
  limit: pageLimit(),
});
const ListQuotaAlertsSchema = z.object({
  limit: pageLimit(),
});
const SendInviteSchema = z.object({
  userId: id(),
  ticketId: id().optional(),
});
const UpdateStatusSchema = z.object({
  userId: id(),
  status: z.enum(["active", "suspended", "disabled"]),
  ticketId: id().optional(),
});
const UpdateQuotaSchema = z.object({
  userId: id(),
  // 0 bytes (remove quota) up to 10 TiB per user
  quotaBytes: z.number().int().min(0).max(10_995_116_277_760),
  ticketId: id().optional(),
});
const UpdateRoleSchema = z.object({
  userId: id(),
  role: z.string().max(64),
  ticketId: id().optional(),
});
const UpdateRecoveryEmailSchema = z.object({
  userId: id(),
  recoveryEmail: email(),
  ticketId: id().optional(),
});
const CreateUserSchema = z.object({
  domainId: id(),
  username: z.string().max(64),
  displayName: z.string().max(256),
  recoveryEmail: email().optional(),
  password: z.string().max(256).optional(),
  // 0 = domain default; max 10 TiB
  quotaLimit: z.number().int().min(0).max(10_995_116_277_760).optional(),
  ticketId: id().optional(),
});
const DeleteUserSchema = z.object({
  userId: id(),
  ticketId: id().optional(),
});
// Restrict to known GogomailDomainSettings fields — prevents prototype pollution
// via arbitrary key injection through z.record()
const UpdateDomainSchema = z.object({
  domainId: id(),
  settings: z.object({
    catchAll: z.boolean().optional(),
    spfEnabled: z.boolean().optional(),
    dkimEnabled: z.boolean().optional(),
    dmarcEnabled: z.boolean().optional(),
    maxMessageSize: z.number().int().min(0).max(157_286_400).optional(), // max 150 MiB
  }),
  ticketId: id().optional(),
});
const ListCompanySessionsSchema = z.object({ companyId: id() });
const RevokeSessionSchema = z.object({
  companyId: id(),
  userId: id(),
  ticketId: id().optional(),
});
const GetSpamFilterSchema = z.object({ companyId: id() });
const ListDkimSchema = z.object({ domainId: id().optional() });
const AlertEventsSchema = z.object({
  companyId: id(),
  limit: pageLimit(),
});
const AuditLogsSchema = z.object({
  userId: id().optional(),
  companyId: id().optional(),
  from: ts().optional(),
  to: ts().optional(),
  limit: pageLimit(),
});

// ── callTool dispatcher ─────────────────────────────────────────

export async function callTool(
  gogomail: GogomailClient,
  suppo: OptionalSuppo,
  name: string,
  args: Record<string, unknown>,
): Promise<unknown> {
  switch (name) {
    // ── Directory / user search ──────────────────────────────
    case "gogomail_search_principals": {
      const p = SearchPrincipalsSchema.parse(args);
      return gogomail.searchPrincipals(p);
    }
    case "gogomail_list_users": {
      const p = ListUsersSchema.parse(args);
      return gogomail.listUsers(p);
    }
    case "gogomail_get_user": {
      const { userId } = UserIdSchema.parse(args);
      return gogomail.getUser(userId);
    }
    case "gogomail_get_user_quota": {
      const { userId } = UserIdSchema.parse(args);
      return gogomail.getUserQuota(userId);
    }
    // ── Company & domain lookup ──────────────────────────────
    case "gogomail_list_companies": {
      const p = ListCompaniesSchema.parse(args);
      return gogomail.listCompanies(p);
    }
    case "gogomail_get_company": {
      const { companyId } = CompanyIdSchema.parse(args);
      return gogomail.getCompany(companyId);
    }
    case "gogomail_list_domains": {
      const p = ListDomainsSchema.parse(args);
      return gogomail.listDomains(p);
    }
    case "gogomail_get_domain_settings": {
      const { domainId } = DomainIdSchema.parse(args);
      return gogomail.getDomainSettings(domainId);
    }
    case "gogomail_check_domain_dns": {
      const { domainId } = DomainIdSchema.parse(args);
      return gogomail.checkDomainDns(domainId);
    }
    // ── Mail flow diagnostics ────────────────────────────────
    case "gogomail_list_mail_flow_logs": {
      const p = ListMailFlowLogsSchema.parse(args);
      return gogomail.listMailFlowLogs(p);
    }
    case "gogomail_get_mail_flow_stats": {
      const p = MailFlowStatsSchema.parse(args);
      return gogomail.getMailFlowStats(p);
    }
    // ── Delivery attempts ────────────────────────────────────
    case "gogomail_list_delivery_attempts": {
      const p = ListDeliveryAttemptsSchema.parse(args);
      return gogomail.listDeliveryAttempts(p);
    }
    case "gogomail_list_exhausted_deliveries": {
      const p = ListExhaustedSchema.parse(args);
      return gogomail.listExhaustedDeliveries(p);
    }
    // ── DLQ management ──────────────────────────────────────
    case "gogomail_list_dlq": {
      const { stream, count } = DlqListSchema.parse(args);
      return gogomail.listDlq(stream, count);
    }
    case "gogomail_delete_dlq_entry": {
      const { stream, id, ticketId } = DlqDeleteSchema.parse(args);
      await gogomail.deleteDlqEntry(stream, id);
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_delete_dlq_entry",
        `stream: ${stream}, id: ${id}`,
        "DLQ 항목 삭제",
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return { status: "ok", stream, id };
    }
    // ── Outbox retry ─────────────────────────────────────────
    case "gogomail_retry_outbox": {
      const { id, ticketId } = RetryOutboxSchema.parse(args);
      const result = await gogomail.retryOutbox(id);
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_retry_outbox",
        `outbox id: ${id}`,
        "발송 재시도",
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return result;
    }
    // ── Suppression list ─────────────────────────────────────
    case "gogomail_list_suppression_list": {
      const p = ListSuppressionSchema.parse(args);
      return gogomail.listSuppressionList(p);
    }
    case "gogomail_remove_suppression_entry": {
      const { id, ticketId } = RemoveSuppressionSchema.parse(args);
      await gogomail.removeSuppressionEntry(id);
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_remove_suppression_entry",
        `suppression id: ${id}`,
        "수신 거부 해제",
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return { status: "ok", id };
    }
    // ── Quota management ─────────────────────────────────────
    case "gogomail_list_quota_usage": {
      const p = ListQuotaUsageSchema.parse(args);
      return gogomail.listQuotaUsage(p);
    }
    case "gogomail_list_quota_alerts": {
      const p = ListQuotaAlertsSchema.parse(args);
      return gogomail.listQuotaAlerts(p);
    }
    // ── User actions ─────────────────────────────────────────
    case "gogomail_send_invite_email": {
      const { userId, ticketId } = SendInviteSchema.parse(args);
      const user = await gogomail.getUser(userId);
      const result = await gogomail.sendInviteEmail(userId);
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_send_invite_email",
        `${user.email} (userId: ${userId})`,
        "초대 이메일 발송 (비밀번호 설정 링크)",
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return result;
    }
    case "gogomail_update_user_status": {
      const { userId, status, ticketId } = UpdateStatusSchema.parse(args);
      const before = await gogomail.getUser(userId);
      const result = await gogomail.updateUserStatus(userId, status);
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_status",
        `${before.email} (userId: ${userId})`,
        `${before.status} → ${status}`,
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return result;
    }
    case "gogomail_update_user_quota": {
      const { userId, quotaBytes, ticketId } = UpdateQuotaSchema.parse(args);
      const before = await gogomail.getUserQuota(userId);
      const result = await gogomail.updateUserQuota(userId, quotaBytes);
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_quota",
        `userId: ${userId}`,
        `${before.allocatedBytes} → ${quotaBytes} bytes`,
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return result;
    }
    case "gogomail_update_user_role": {
      const { userId, role, ticketId } = UpdateRoleSchema.parse(args);
      const before = await gogomail.getUser(userId);
      const result = await gogomail.updateUserRole(userId, role);
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_role",
        `${before.email} (userId: ${userId})`,
        `${before.role} → ${role}`,
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return result;
    }
    case "gogomail_update_user_recovery_email": {
      const { userId, recoveryEmail, ticketId } = UpdateRecoveryEmailSchema.parse(args);
      const user = await gogomail.getUser(userId);
      await gogomail.updateUserRecoveryEmail(userId, recoveryEmail);
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_recovery_email",
        `${user.email} (userId: ${userId})`,
        `복구 이메일 변경 → ${recoveryEmail}`,
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return { status: "ok", userId, recoveryEmail };
    }
    case "gogomail_create_user": {
      const { domainId, username, displayName, recoveryEmail, password, quotaLimit, ticketId } =
        CreateUserSchema.parse(args);
      const user = await gogomail.createUser({ domainId, username, displayName, recoveryEmail, password, quotaLimit });
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_create_user",
        `${user.email} (userId: ${user.id})`,
        `신규 사용자 생성 — domainId: ${domainId}`,
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return user;
    }
    case "gogomail_delete_user": {
      const { userId, ticketId } = DeleteUserSchema.parse(args);
      const user = await gogomail.getUser(userId);
      await gogomail.deleteUser(userId);
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_delete_user",
        `${user.email} (userId: ${userId})`,
        "사용자 계정 영구 삭제",
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return { status: "ok", deletedUserId: userId, email: user.email };
    }
    case "gogomail_update_domain_settings": {
      const { domainId, settings, ticketId } = UpdateDomainSchema.parse(args);
      const before = await gogomail.getDomainSettings(domainId);
      const result = await gogomail.updateDomainSettings(domainId, settings);
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_domain_settings",
        `${before.domain} (domainId: ${domainId})`,
        `변경 전: ${JSON.stringify(before)}\n- 변경 후: ${JSON.stringify(settings)}`,
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return result;
    }
    // ── Session management ───────────────────────────────────
    case "gogomail_list_company_sessions": {
      const { companyId } = ListCompanySessionsSchema.parse(args);
      return gogomail.listCompanySessions(companyId);
    }
    case "gogomail_revoke_company_session": {
      const { companyId, userId, ticketId } = RevokeSessionSchema.parse(args);
      await gogomail.revokeCompanySession(companyId, userId);
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_revoke_company_session",
        `companyId: ${companyId}, userId: ${userId}`,
        "세션 강제 종료",
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e instanceof Error ? e.message : String(e)),
      );
      return { status: "ok" };
    }
    // ── Security & monitoring ────────────────────────────────
    case "gogomail_get_spam_filter": {
      const { companyId } = GetSpamFilterSchema.parse(args);
      return gogomail.getSpamFilter(companyId);
    }
    case "gogomail_get_spam_filter_events": {
      const { companyId } = GetSpamFilterSchema.parse(args);
      return gogomail.getSpamFilterEvents(companyId);
    }
    case "gogomail_list_dkim_keys": {
      const { domainId } = ListDkimSchema.parse(args);
      return gogomail.listDkimKeys(domainId);
    }
    case "gogomail_get_alert_events": {
      const p = AlertEventsSchema.parse(args);
      return gogomail.getAlertEvents(p);
    }
    case "gogomail_get_audit_logs": {
      const p = AuditLogsSchema.parse(args);
      return gogomail.getAuditLogs(p);
    }
    // ── System health ────────────────────────────────────────
    case "gogomail_check_health": {
      return gogomail.checkHealth();
    }
    case "gogomail_get_queue_stats": {
      return gogomail.getQueueStats();
    }
    default:
      throw new Error(`Unknown GoGoMail tool: ${name}`);
  }
}
