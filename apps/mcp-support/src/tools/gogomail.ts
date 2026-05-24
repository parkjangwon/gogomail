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
      "Get domain configuration: TLS policy, per-user quota, IP allowlist, 2FA, session timeout, password policy, and invite/reset policy.",
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
        confirm: { type: "string", description: "Must exactly equal: delete <stream>/<id>", maxLength: 160 },
        reason: { type: "string", description: "Operational reason for this destructive change", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["stream", "id", "confirm", "reason"],
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
        reason: { type: "string", description: "Operational reason for this retry", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["id", "reason"],
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
        reason: { type: "string", description: "Operational reason for this removal", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["id", "reason"],
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
        reason: { type: "string", description: "Operational reason for sending the invite", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId", "reason"],
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
        reason: { type: "string", description: "Operational reason for the status change", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId", "status", "reason"],
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
        reason: { type: "string", description: "Operational reason for the quota change", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId", "quotaBytes", "reason"],
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
        role: { type: "string", description: "New role to assign to the user", enum: ["user", "company_admin", "system_admin"] },
        reason: { type: "string", description: "Operational reason for the role change", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId", "role", "reason"],
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
        reason: { type: "string", description: "Operational reason for the recovery email change", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId", "recoveryEmail", "reason"],
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
        password: { type: "string", description: "Initial password — user will be required to change it on first login", minLength: 8, maxLength: 256 },
        quotaLimit: { type: "number", description: "Storage quota in bytes (optional, uses domain default)", minimum: 0, maximum: 10995116277760 },
        reason: { type: "string", description: "Operational reason for creating the user", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["domainId", "username", "displayName", "reason"],
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
        confirm: { type: "string", description: "Must exactly equal: delete <userId>", maxLength: 160 },
        reason: { type: "string", description: "Operational reason for this irreversible deletion", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["userId", "confirm", "reason"],
    },
  },
  {
    name: "gogomail_update_domain_settings",
    description:
      "Update domain configuration. Settings are merged with the current server-side domain settings before PUT so omitted fields are preserved. PREREQUISITE: call gogomail_get_domain_settings first. Provide ticketId to attach audit memo to an existing Suppo ticket; omit to auto-create a standalone audit ticket.",
    inputSchema: {
      type: "object",
      properties: {
        domainId: { type: "string", description: "Domain entity ID", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        settings: {
          type: "object",
          description: "Partial domain settings to update. Supported keys: tls_policy, quota_per_user, ip_whitelist_enabled, ip_whitelist, require_2fa, session_timeout_minutes, password_min_length, password_require_uppercase, password_require_numbers, password_require_special_chars, password_expiry_days, user_registration_mode, password_reset_token_ttl_minutes.",
        },
        reason: { type: "string", description: "Operational reason for the domain settings change", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["domainId", "settings", "reason"],
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
        reason: { type: "string", description: "Operational reason for revoking the session", maxLength: 500 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["companyId", "userId", "reason"],
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

type AuditResult =
  | { status: "written"; destination: "suppo_ticket"; ticketId: string }
  | { status: "written"; destination: "suppo_audit_ticket" }
  | { status: "written"; destination: "stderr" }
  | { status: "failed"; error: string };

// Strip newlines so audit fields can't inject extra lines into the comment body
const sanitizeAuditField = (s: string) => s.replace(/[\r\n]/g, " ").slice(0, 500);
const auditErrorMessage = (e: unknown) =>
  (e instanceof Error ? e.message : String(e)).replace(/[\r\n]/g, " ").slice(0, 500);

function withAudit<T>(result: T, audit: AuditResult): unknown {
  if (result && typeof result === "object" && !Array.isArray(result)) {
    return { ...(result as Record<string, unknown>), audit };
  }
  return { result, audit };
}

function describeUser(user: { username?: string; display_name?: string; id: string }): string {
  const label = user.display_name ? `${user.username ?? "(unknown)"} / ${user.display_name}` : (user.username ?? "(unknown)");
  return `${label} (userId: ${user.id})`;
}

function requireConfirm(actual: string, expected: string): void {
  if (actual !== expected) {
    throw new Error(`confirm must exactly equal "${expected}"`);
  }
}

async function writeAuditComment(
  suppo: OptionalSuppo,
  ticketId: string | undefined,
  toolName: string,
  targetInfo: string,
  change: string,
): Promise<AuditResult> {
  const body = [
    `[자동 실행] ${toolName}`,
    `- 대상: ${sanitizeAuditField(targetInfo)}`,
    `- 변경: ${sanitizeAuditField(change)}`,
    `- 실행 시각: ${new Date().toISOString()}`,
  ].join("\n");

  if (!suppo) {
    console.error(`[audit] ${body}`);
    return { status: "written", destination: "stderr" };
  }

  try {
    if (ticketId) {
      await suppo.addComment(ticketId, { body, internal: true });
      return { status: "written", destination: "suppo_ticket", ticketId };
    }
    await suppo.createTicket({
      customerName: "System",
      customerEmail: "system@gogomail.io",
      subject: `[감사 기록] ${toolName}`,
      description: body,
      priority: "low",
    });
    return { status: "written", destination: "suppo_audit_ticket" };
  } catch (e) {
    const error = auditErrorMessage(e);
    console.error("[audit] failed to write comment:", error);
    return { status: "failed", error };
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
const singleLine = (name: string, max: number) =>
  z.string().trim().min(1).max(max).regex(/^[^\r\n]+$/, `${name} must be a single line`);
const reason = () => singleLine("reason", 500);
const confirm = () => singleLine("confirm", 160);
const userStatus = z.enum(["active", "suspended", "disabled"]);
const companyStatus = z.enum(["active", "suspended"]);
const domainStatus = z.enum(["active", "suspended"]);
const dnsStatus = z.enum(["verified", "unverified", "partial"]);
const direction = z.enum(["inbound", "outbound"]);
const mailFlowStatus = z.enum(["delivered", "bounced", "deferred", "rejected", "quarantined", "expired"]);
const deliveryStatus = z.enum(["pending", "success", "failed", "exhausted"]);
const suppressionReason = z.enum(["bounce", "complaint", "manual"]);
const role = z.enum(["user", "company_admin", "system_admin"]);
const ts = () =>
  z.string().max(64).refine((value) => !Number.isNaN(Date.parse(value)), "must be a valid ISO 8601 timestamp");
// Bounded page limit — prevents agents from issuing unbounded backend queries
const pageLimit = () => z.number().int().min(1).max(200).optional();

const SearchPrincipalsSchema = z.object({
  q: singleLine("query", 200),
  companyId: id().optional(),
  domainId: id().optional(),
  limit: pageLimit(),
});
const ListUsersSchema = z.object({
  domainId: id().optional(),
  status: userStatus.optional(),
  limit: pageLimit(),
});
const UserIdSchema = z.object({ userId: id() });
const CompanyIdSchema = z.object({ companyId: id() });
const ListCompaniesSchema = z.object({
  status: companyStatus.optional(),
  limit: pageLimit(),
});
const ListDomainsSchema = z.object({
  companyId: id().optional(),
  status: domainStatus.optional(),
  dnsStatus: dnsStatus.optional(),
  limit: pageLimit(),
});
const DomainIdSchema = z.object({ domainId: id() });
const ListMailFlowLogsSchema = z.object({
  userId: id().optional(),
  companyId: id().optional(),
  domainId: id().optional(),
  messageId: singleLine("messageId", 256).optional(),
  fromAddr: email().optional(),
  toAddr: email().optional(),
  direction: direction.optional(),
  flowStatus: mailFlowStatus.optional(),
  since: ts().optional(),
  until: ts().optional(),
  limit: pageLimit(),
});
const MailFlowStatsSchema = z.object({
  userId: id().optional(),
  companyId: id().optional(),
  domainId: id().optional(),
  direction: direction.optional(),
  since: ts().optional(),
  until: ts().optional(),
});
const ListDeliveryAttemptsSchema = z.object({
  messageId: singleLine("messageId", 256).optional(),
  status: deliveryStatus.optional(),
  recipientDomain: singleLine("recipientDomain", 253).optional(),
  sender: email().optional(),
  since: ts().optional(),
  limit: pageLimit(),
});
const ListExhaustedSchema = z.object({
  messageId: singleLine("messageId", 256).optional(),
  recipientDomain: singleLine("recipientDomain", 253).optional(),
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
  confirm: confirm(),
  reason: reason(),
  ticketId: id().optional(),
});
const RetryOutboxSchema = z.object({
  id: id(),
  reason: reason(),
  ticketId: id().optional(),
});
const ListSuppressionSchema = z.object({
  email: email().optional(),
  domainId: id().optional(),
  reason: suppressionReason.optional(),
  limit: pageLimit(),
});
const RemoveSuppressionSchema = z.object({
  id: id(),
  reason: reason(),
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
  reason: reason(),
  ticketId: id().optional(),
});
const UpdateStatusSchema = z.object({
  userId: id(),
  status: userStatus,
  reason: reason(),
  ticketId: id().optional(),
});
const UpdateQuotaSchema = z.object({
  userId: id(),
  // 0 bytes (remove quota) up to 10 TiB per user
  quotaBytes: z.number().int().min(0).max(10_995_116_277_760),
  reason: reason(),
  ticketId: id().optional(),
});
const UpdateRoleSchema = z.object({
  userId: id(),
  role,
  reason: reason(),
  ticketId: id().optional(),
});
const UpdateRecoveryEmailSchema = z.object({
  userId: id(),
  recoveryEmail: email(),
  reason: reason(),
  ticketId: id().optional(),
});
const CreateUserSchema = z.object({
  domainId: id(),
  username: singleLine("username", 64).regex(/^[A-Za-z0-9._+-]+$/, "username contains unsupported characters"),
  displayName: singleLine("displayName", 256),
  recoveryEmail: email().optional(),
  password: z.string().min(8).max(256).optional(),
  // 0 = domain default; max 10 TiB
  quotaLimit: z.number().int().min(0).max(10_995_116_277_760).optional(),
  reason: reason(),
  ticketId: id().optional(),
});
const DeleteUserSchema = z.object({
  userId: id(),
  confirm: confirm(),
  reason: reason(),
  ticketId: id().optional(),
});
// Restrict to known Admin DomainSettings fields — prevents prototype pollution
// via arbitrary key injection through z.record().
const UpdateDomainSchema = z.object({
  domainId: id(),
  settings: z.object({
    tls_policy: z.enum(["opportunistic", "require", "disable"]).optional(),
    quota_per_user: z.number().int().min(1).max(10_995_116_277_760).optional(),
    ip_whitelist_enabled: z.boolean().optional(),
    ip_whitelist: z.array(z.string().max(128)).max(200).optional(),
    require_2fa: z.boolean().optional(),
    session_timeout_minutes: z.number().int().min(1).max(10_080).optional(),
    password_min_length: z.number().int().min(1).max(256).optional(),
    password_require_uppercase: z.boolean().optional(),
    password_require_numbers: z.boolean().optional(),
    password_require_special_chars: z.boolean().optional(),
    password_expiry_days: z.number().int().min(0).max(3650).optional(),
    user_registration_mode: z.enum(["temp_password", "email_invite"]).optional(),
    password_reset_token_ttl_minutes: z.number().int().min(1).max(10_080).optional(),
  }).refine((settings) => Object.keys(settings).length > 0, {
    message: "settings must include at least one supported field",
  }),
  reason: reason(),
  ticketId: id().optional(),
});
const ListCompanySessionsSchema = z.object({ companyId: id() });
const RevokeSessionSchema = z.object({
  companyId: id(),
  userId: id(),
  reason: reason(),
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
      const { stream, id, confirm, reason, ticketId } = DlqDeleteSchema.parse(args);
      requireConfirm(confirm, `delete ${stream}/${id}`);
      await gogomail.deleteDlqEntry(stream, id);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_delete_dlq_entry",
        `stream: ${stream}, id: ${id}`,
        `DLQ 항목 삭제 / 사유: ${reason}`,
      );
      return { status: "ok", stream, id, audit };
    }
    // ── Outbox retry ─────────────────────────────────────────
    case "gogomail_retry_outbox": {
      const { id, reason, ticketId } = RetryOutboxSchema.parse(args);
      const result = await gogomail.retryOutbox(id);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_retry_outbox",
        `outbox id: ${id}`,
        `발송 재시도 / 사유: ${reason}`,
      );
      return withAudit(result, audit);
    }
    // ── Suppression list ─────────────────────────────────────
    case "gogomail_list_suppression_list": {
      const p = ListSuppressionSchema.parse(args);
      return gogomail.listSuppressionList(p);
    }
    case "gogomail_remove_suppression_entry": {
      const { id, reason, ticketId } = RemoveSuppressionSchema.parse(args);
      await gogomail.removeSuppressionEntry(id);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_remove_suppression_entry",
        `suppression id: ${id}`,
        `수신 거부 해제 / 사유: ${reason}`,
      );
      return { status: "ok", id, audit };
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
      const { userId, reason, ticketId } = SendInviteSchema.parse(args);
      const user = await gogomail.getUser(userId);
      const result = await gogomail.sendInviteEmail(userId);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_send_invite_email",
        describeUser(user),
        `초대 이메일 발송 (비밀번호 설정 링크) / 사유: ${reason}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_update_user_status": {
      const { userId, status, reason, ticketId } = UpdateStatusSchema.parse(args);
      const before = await gogomail.getUser(userId);
      const result = await gogomail.updateUserStatus(userId, status);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_status",
        describeUser(before),
        `${before.status} → ${status} / 사유: ${reason}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_update_user_quota": {
      const { userId, quotaBytes, reason, ticketId } = UpdateQuotaSchema.parse(args);
      const before = await gogomail.getUserQuota(userId);
      const result = await gogomail.updateUserQuota(userId, quotaBytes);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_quota",
        `userId: ${userId}`,
        `${before.allocatedBytes} → ${quotaBytes} bytes / 사유: ${reason}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_update_user_role": {
      const { userId, role, reason, ticketId } = UpdateRoleSchema.parse(args);
      const before = await gogomail.getUser(userId);
      const result = await gogomail.updateUserRole(userId, role);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_role",
        describeUser(before),
        `${before.role} → ${role} / 사유: ${reason}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_update_user_recovery_email": {
      const { userId, recoveryEmail, reason, ticketId } = UpdateRecoveryEmailSchema.parse(args);
      const user = await gogomail.getUser(userId);
      await gogomail.updateUserRecoveryEmail(userId, recoveryEmail);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_recovery_email",
        describeUser(user),
        `복구 이메일 변경 → ${recoveryEmail} / 사유: ${reason}`,
      );
      return { status: "ok", userId, recoveryEmail, audit };
    }
    case "gogomail_create_user": {
      const { domainId, username, displayName, recoveryEmail, password, quotaLimit, reason, ticketId } =
        CreateUserSchema.parse(args);
      const user = await gogomail.createUser({ domainId, username, displayName, recoveryEmail, password, quotaLimit });
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_create_user",
        describeUser(user),
        `신규 사용자 생성 / domainId: ${domainId} / 사유: ${reason}`,
      );
      return withAudit(user, audit);
    }
    case "gogomail_delete_user": {
      const { userId, confirm, reason, ticketId } = DeleteUserSchema.parse(args);
      requireConfirm(confirm, `delete ${userId}`);
      const user = await gogomail.getUser(userId);
      await gogomail.deleteUser(userId);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_delete_user",
        describeUser(user),
        `사용자 계정 영구 삭제 / 사유: ${reason}`,
      );
      return { status: "ok", deletedUserId: userId, username: user.username, audit };
    }
    case "gogomail_update_domain_settings": {
      const { domainId, settings, reason, ticketId } = UpdateDomainSchema.parse(args);
      const before = await gogomail.getDomainSettings(domainId);
      const mergedSettings = { ...before, ...settings };
      const result = await gogomail.updateDomainSettings(domainId, mergedSettings);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_domain_settings",
        `domainId: ${domainId}`,
        `변경 전: ${JSON.stringify(before)}\n- 변경 요청: ${JSON.stringify(settings)}\n- 사유: ${reason}`,
      );
      return withAudit(result, audit);
    }
    // ── Session management ───────────────────────────────────
    case "gogomail_list_company_sessions": {
      const { companyId } = ListCompanySessionsSchema.parse(args);
      return gogomail.listCompanySessions(companyId);
    }
    case "gogomail_revoke_company_session": {
      const { companyId, userId, reason, ticketId } = RevokeSessionSchema.parse(args);
      await gogomail.revokeCompanySession(companyId, userId);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_revoke_company_session",
        `companyId: ${companyId}, userId: ${userId}`,
        `세션 강제 종료 / 사유: ${reason}`,
      );
      return { status: "ok", audit };
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
