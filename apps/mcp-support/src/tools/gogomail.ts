import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient, GogomailDomainSettings } from "../clients/gogomail.js";
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
        q: { type: "string", description: "Email address or name to search for" },
        companyId: { type: "string", description: "Filter by company ID" },
        domainId: { type: "string", description: "Filter by domain ID" },
        limit: { type: "number", description: "Max results (default 20)" },
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
        domainId: { type: "string", description: "Filter by domain ID" },
        status: { type: "string", description: "active | suspended | disabled" },
        limit: { type: "number" },
      },
    },
  },
  {
    name: "gogomail_get_user",
    description: "Get full user details including status, role, quota, and domain.",
    inputSchema: {
      type: "object",
      properties: { userId: { type: "string" } },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_get_user_quota",
    description: "Get storage quota allocation and current usage bytes for a user.",
    inputSchema: {
      type: "object",
      properties: { userId: { type: "string" } },
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
        status: { type: "string", description: "active | suspended" },
        limit: { type: "number" },
      },
    },
  },
  {
    name: "gogomail_get_company",
    description: "Get company details by company ID.",
    inputSchema: {
      type: "object",
      properties: { companyId: { type: "string" } },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_list_domains",
    description: "List domains. Filter by company, status, or DNS verification status.",
    inputSchema: {
      type: "object",
      properties: {
        companyId: { type: "string" },
        status: { type: "string", description: "active | suspended" },
        dnsStatus: { type: "string", description: "verified | unverified | partial" },
        limit: { type: "number" },
      },
    },
  },
  {
    name: "gogomail_get_domain_settings",
    description:
      "Get domain configuration: SPF, DKIM, DMARC, catch-all, and max message size.",
    inputSchema: {
      type: "object",
      properties: { domainId: { type: "string" } },
      required: ["domainId"],
    },
  },
  {
    name: "gogomail_check_domain_dns",
    description:
      "Check DNS record verification status for a domain (SPF, DKIM, DMARC, MX). Use to diagnose mail delivery failures caused by DNS misconfiguration.",
    inputSchema: {
      type: "object",
      properties: { domainId: { type: "string" } },
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
        userId: { type: "string" },
        companyId: { type: "string" },
        domainId: { type: "string" },
        messageId: { type: "string" },
        fromAddr: { type: "string", description: "Sender email address" },
        toAddr: { type: "string", description: "Recipient email address" },
        direction: { type: "string", description: "inbound | outbound" },
        flowStatus: {
          type: "string",
          description:
            "delivered | bounced | deferred | rejected | quarantined | expired",
        },
        since: { type: "string", description: "ISO 8601 start time" },
        until: { type: "string", description: "ISO 8601 end time" },
        limit: { type: "number", description: "Max results (default 20, max 100)" },
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
        userId: { type: "string" },
        companyId: { type: "string" },
        domainId: { type: "string" },
        direction: { type: "string" },
        since: { type: "string" },
        until: { type: "string" },
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
        messageId: { type: "string" },
        status: { type: "string", description: "pending | success | failed | exhausted" },
        recipientDomain: { type: "string" },
        sender: { type: "string" },
        since: { type: "string", description: "ISO 8601 start time" },
        limit: { type: "number" },
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
        messageId: { type: "string" },
        recipientDomain: { type: "string" },
        sender: { type: "string" },
        since: { type: "string" },
        limit: { type: "number" },
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
        stream: { type: "string", description: "DLQ stream name (required)" },
        count: { type: "number", description: "Max entries to return" },
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
        stream: { type: "string" },
        id: { type: "string", description: "DLQ entry ID" },
        ticketId: { type: "string" },
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
        id: { type: "string", description: "Outbox message ID" },
        ticketId: { type: "string" },
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
        email: { type: "string", description: "Filter by specific email address" },
        domainId: { type: "string" },
        reason: { type: "string", description: "bounce | complaint | manual" },
        limit: { type: "number" },
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
        id: { type: "string", description: "Suppression entry ID from list_suppression_list" },
        ticketId: { type: "string" },
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
        domainId: { type: "string" },
        overLimit: { type: "boolean", description: "Only show entities over quota limit" },
        limit: { type: "number" },
      },
    },
  },
  {
    name: "gogomail_list_quota_alerts",
    description: "List quota threshold alerts that have been triggered.",
    inputSchema: {
      type: "object",
      properties: {
        limit: { type: "number" },
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
        userId: { type: "string" },
        ticketId: { type: "string" },
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
        userId: { type: "string" },
        status: { type: "string", description: "active | suspended | disabled" },
        ticketId: { type: "string" },
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
        userId: { type: "string" },
        quotaBytes: { type: "number", description: "New quota in bytes" },
        ticketId: { type: "string" },
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
        userId: { type: "string" },
        role: { type: "string" },
        ticketId: { type: "string" },
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
        userId: { type: "string" },
        recoveryEmail: { type: "string", description: "New recovery email address" },
        ticketId: { type: "string" },
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
        domainId: { type: "string" },
        username: { type: "string", description: "Local part of the email address (before @)" },
        displayName: { type: "string", description: "User's full display name" },
        recoveryEmail: { type: "string", description: "Recovery email (optional)" },
        password: { type: "string", description: "Initial password — user will be required to change it on first login" },
        quotaLimit: { type: "number", description: "Storage quota in bytes (optional, uses domain default)" },
        ticketId: { type: "string" },
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
        userId: { type: "string" },
        ticketId: { type: "string" },
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
        domainId: { type: "string" },
        settings: {
          type: "object",
          description: "Partial domain settings to update",
        },
        ticketId: { type: "string" },
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
      properties: { companyId: { type: "string" } },
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
        companyId: { type: "string" },
        userId: { type: "string" },
        ticketId: { type: "string" },
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
      properties: { companyId: { type: "string" } },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_get_spam_filter_events",
    description:
      "Get recent spam filter events for a company. Use to investigate why legitimate mail is being quarantined.",
    inputSchema: {
      type: "object",
      properties: { companyId: { type: "string" } },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_list_dkim_keys",
    description:
      "List DKIM signing keys. Filter by domain to check if a domain has valid DKIM configured.",
    inputSchema: {
      type: "object",
      properties: { domainId: { type: "string" } },
    },
  },
  {
    name: "gogomail_get_alert_events",
    description: "Get recent system alert events for a company.",
    inputSchema: {
      type: "object",
      properties: {
        companyId: { type: "string" },
        limit: { type: "number" },
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
        userId: { type: "string" },
        companyId: { type: "string" },
        from: { type: "string", description: "ISO 8601 start time" },
        to: { type: "string", description: "ISO 8601 end time" },
        limit: { type: "number" },
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

async function writeAuditComment(
  suppo: OptionalSuppo,
  ticketId: string | undefined,
  toolName: string,
  targetInfo: string,
  change: string,
): Promise<void> {
  const body = [
    `[자동 실행] ${toolName}`,
    `- 대상: ${targetInfo}`,
    `- 변경: ${change}`,
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

const SearchPrincipalsSchema = z.object({
  q: z.string(),
  companyId: z.string().optional(),
  domainId: z.string().optional(),
  limit: z.number().optional(),
});
const ListUsersSchema = z.object({
  domainId: z.string().optional(),
  status: z.string().optional(),
  limit: z.number().optional(),
});
const UserIdSchema = z.object({ userId: z.string() });
const CompanyIdSchema = z.object({ companyId: z.string() });
const ListCompaniesSchema = z.object({
  status: z.string().optional(),
  limit: z.number().optional(),
});
const ListDomainsSchema = z.object({
  companyId: z.string().optional(),
  status: z.string().optional(),
  dnsStatus: z.string().optional(),
  limit: z.number().optional(),
});
const DomainIdSchema = z.object({ domainId: z.string() });
const ListMailFlowLogsSchema = z.object({
  userId: z.string().optional(),
  companyId: z.string().optional(),
  domainId: z.string().optional(),
  messageId: z.string().optional(),
  fromAddr: z.string().optional(),
  toAddr: z.string().optional(),
  direction: z.string().optional(),
  flowStatus: z.string().optional(),
  since: z.string().optional(),
  until: z.string().optional(),
  limit: z.number().optional(),
});
const MailFlowStatsSchema = z.object({
  userId: z.string().optional(),
  companyId: z.string().optional(),
  domainId: z.string().optional(),
  direction: z.string().optional(),
  since: z.string().optional(),
  until: z.string().optional(),
});
const ListDeliveryAttemptsSchema = z.object({
  messageId: z.string().optional(),
  status: z.string().optional(),
  recipientDomain: z.string().optional(),
  sender: z.string().optional(),
  since: z.string().optional(),
  limit: z.number().optional(),
});
const ListExhaustedSchema = z.object({
  messageId: z.string().optional(),
  recipientDomain: z.string().optional(),
  sender: z.string().optional(),
  since: z.string().optional(),
  limit: z.number().optional(),
});
const DlqListSchema = z.object({
  stream: z.string(),
  count: z.number().optional(),
});
const DlqDeleteSchema = z.object({
  stream: z.string(),
  id: z.string(),
  ticketId: z.string().optional(),
});
const RetryOutboxSchema = z.object({
  id: z.string(),
  ticketId: z.string().optional(),
});
const ListSuppressionSchema = z.object({
  email: z.string().optional(),
  domainId: z.string().optional(),
  reason: z.string().optional(),
  limit: z.number().optional(),
});
const RemoveSuppressionSchema = z.object({
  id: z.string(),
  ticketId: z.string().optional(),
});
const ListQuotaUsageSchema = z.object({
  domainId: z.string().optional(),
  overLimit: z.boolean().optional(),
  limit: z.number().optional(),
});
const ListQuotaAlertsSchema = z.object({
  limit: z.number().optional(),
});
const SendInviteSchema = z.object({
  userId: z.string(),
  ticketId: z.string().optional(),
});
const UpdateStatusSchema = z.object({
  userId: z.string(),
  status: z.enum(["active", "suspended", "disabled"]),
  ticketId: z.string().optional(),
});
const UpdateQuotaSchema = z.object({
  userId: z.string(),
  quotaBytes: z.number(),
  ticketId: z.string().optional(),
});
const UpdateRoleSchema = z.object({
  userId: z.string(),
  role: z.string(),
  ticketId: z.string().optional(),
});
const UpdateRecoveryEmailSchema = z.object({
  userId: z.string(),
  recoveryEmail: z.string().email(),
  ticketId: z.string().optional(),
});
const CreateUserSchema = z.object({
  domainId: z.string(),
  username: z.string(),
  displayName: z.string(),
  recoveryEmail: z.string().optional(),
  password: z.string().optional(),
  quotaLimit: z.number().optional(),
  ticketId: z.string().optional(),
});
const DeleteUserSchema = z.object({
  userId: z.string(),
  ticketId: z.string().optional(),
});
const UpdateDomainSchema = z.object({
  domainId: z.string(),
  settings: z.record(z.unknown()),
  ticketId: z.string().optional(),
});
const ListCompanySessionsSchema = z.object({ companyId: z.string() });
const RevokeSessionSchema = z.object({
  companyId: z.string(),
  userId: z.string(),
  ticketId: z.string().optional(),
});
const GetSpamFilterSchema = z.object({ companyId: z.string() });
const ListDkimSchema = z.object({ domainId: z.string().optional() });
const AlertEventsSchema = z.object({
  companyId: z.string(),
  limit: z.number().optional(),
});
const AuditLogsSchema = z.object({
  userId: z.string().optional(),
  companyId: z.string().optional(),
  from: z.string().optional(),
  to: z.string().optional(),
  limit: z.number().optional(),
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
        console.error("[audit] failed to write comment:", e),
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
        console.error("[audit] failed to write comment:", e),
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
        console.error("[audit] failed to write comment:", e),
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
        console.error("[audit] failed to write comment:", e),
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
        console.error("[audit] failed to write comment:", e),
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
        console.error("[audit] failed to write comment:", e),
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
        console.error("[audit] failed to write comment:", e),
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
        console.error("[audit] failed to write comment:", e),
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
        console.error("[audit] failed to write comment:", e),
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
        console.error("[audit] failed to write comment:", e),
      );
      return { status: "ok", deletedUserId: userId, email: user.email };
    }
    case "gogomail_update_domain_settings": {
      const { domainId, settings, ticketId } = UpdateDomainSchema.parse(args);
      const before = await gogomail.getDomainSettings(domainId);
      const result = await gogomail.updateDomainSettings(
        domainId,
        settings as Partial<GogomailDomainSettings>,
      );
      writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_domain_settings",
        `${before.domain} (domainId: ${domainId})`,
        `변경 전: ${JSON.stringify(before)}\n- 변경 후: ${JSON.stringify(settings)}`,
      ).catch((e: unknown) =>
        console.error("[audit] failed to write comment:", e),
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
        console.error("[audit] failed to write comment:", e),
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
