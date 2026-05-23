import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient, GogomailDomainSettings } from "../clients/gogomail.js";
import type { SuppoClient } from "../clients/suppo.js";

// ── Tool definitions ────────────────────────────────────────────

export const toolDefinitions: Tool[] = [
  // Read tools
  {
    name: "gogomail_find_user",
    description: "Find a GoGoMail user by email address.",
    inputSchema: {
      type: "object",
      properties: { email: { type: "string" } },
      required: ["email"],
    },
  },
  {
    name: "gogomail_get_user",
    description: "Get full user details including status, role, and quota.",
    inputSchema: {
      type: "object",
      properties: { userId: { type: "string" } },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_get_user_quota",
    description: "Get storage quota allocation and usage for a user.",
    inputSchema: {
      type: "object",
      properties: { userId: { type: "string" } },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_get_mail_logs",
    description: "Get mail flow logs for a user. Filter by direction, status, and time range.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string" },
        direction: { type: "string", description: "inbound | outbound" },
        status: { type: "string" },
        from: { type: "string", description: "ISO 8601 start time" },
        to: { type: "string", description: "ISO 8601 end time" },
      },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_trace_message",
    description: "Trace the delivery path of a specific message by its ID.",
    inputSchema: {
      type: "object",
      properties: { messageId: { type: "string" } },
      required: ["messageId"],
    },
  },
  {
    name: "gogomail_get_delivery_attempts",
    description: "Get all delivery attempts and error details for a message.",
    inputSchema: {
      type: "object",
      properties: { messageId: { type: "string" } },
      required: ["messageId"],
    },
  },
  {
    name: "gogomail_get_audit_logs",
    description: "Get system audit logs for a user or company, optionally filtered by time range.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string" },
        companyId: { type: "string" },
        from: { type: "string" },
        to: { type: "string" },
      },
    },
  },
  {
    name: "gogomail_list_user_sessions",
    description: "List all active sessions for a user.",
    inputSchema: {
      type: "object",
      properties: { userId: { type: "string" } },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_check_health",
    description: "Check GoGoMail system health and mail queue status.",
    inputSchema: { type: "object", properties: {} },
  },
  // Action tools
  {
    name: "gogomail_reset_password",
    description: "Send a password reset invitation email to the user. PREREQUISITE: call gogomail_get_user first to verify the current account state. Requires ticketId for audit trail.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string" },
        ticketId: { type: "string", description: "Suppo ticket ID for audit memo" },
      },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_update_user_status",
    description: "Change account status. PREREQUISITE: call gogomail_get_user to confirm current status first. status must be 'active' | 'suspended' | 'disabled'. Requires ticketId for audit trail.",
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
    description: "Adjust user storage quota in bytes. PREREQUISITE: call gogomail_get_user_quota first. Requires ticketId for audit trail.",
    inputSchema: {
      type: "object",
      properties: {
        userId: { type: "string" },
        quotaBytes: { type: "number" },
        ticketId: { type: "string" },
      },
      required: ["userId", "quotaBytes"],
    },
  },
  {
    name: "gogomail_revoke_sessions",
    description: "Force-logout all active sessions for a user. PREREQUISITE: call gogomail_list_user_sessions first. Requires ticketId for audit trail.",
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
    name: "gogomail_update_user_role",
    description: "Change a user's role. PREREQUISITE: call gogomail_get_user first. Requires ticketId for audit trail.",
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
    name: "gogomail_get_company",
    description: "Get company and domain information by company ID.",
    inputSchema: {
      type: "object",
      properties: { companyId: { type: "string" } },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_get_domain_settings",
    description: "Get domain configuration (SPF, DKIM, DMARC, catch-all, message size).",
    inputSchema: {
      type: "object",
      properties: { domainId: { type: "string" } },
      required: ["domainId"],
    },
  },
  {
    name: "gogomail_update_domain_settings",
    description: "Update domain configuration. PREREQUISITE: call gogomail_get_domain_settings first. Requires ticketId for audit trail.",
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
  {
    name: "gogomail_get_alert_events",
    description: "Get recent alert events for a company.",
    inputSchema: {
      type: "object",
      properties: {
        companyId: { type: "string" },
        limit: { type: "number" },
      },
      required: ["companyId"],
    },
  },
];

// ── Audit helper ────────────────────────────────────────────────

async function writeAuditComment(
  suppo: SuppoClient,
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

// ── Zod schemas ────────────────────────────────────────────────

const EmailSchema = z.object({ email: z.string() });
const UserIdSchema = z.object({ userId: z.string() });
const MailLogsSchema = z.object({
  userId: z.string(),
  direction: z.string().optional(),
  status: z.string().optional(),
  from: z.string().optional(),
  to: z.string().optional(),
});
const MessageIdSchema = z.object({ messageId: z.string() });
const AuditLogsSchema = z.object({
  userId: z.string().optional(),
  companyId: z.string().optional(),
  from: z.string().optional(),
  to: z.string().optional(),
});
const ResetPasswordSchema = z.object({
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
const RevokeSessionsSchema = z.object({
  userId: z.string(),
  ticketId: z.string().optional(),
});
const UpdateRoleSchema = z.object({
  userId: z.string(),
  role: z.string(),
  ticketId: z.string().optional(),
});
const CompanyIdSchema = z.object({ companyId: z.string() });
const DomainIdSchema = z.object({ domainId: z.string() });
const UpdateDomainSchema = z.object({
  domainId: z.string(),
  settings: z.record(z.unknown()),
  ticketId: z.string().optional(),
});
const AlertEventsSchema = z.object({
  companyId: z.string(),
  limit: z.number().optional(),
});

// ── callTool dispatcher ─────────────────────────────────────────

export async function callTool(
  gogomail: GogomailClient,
  suppo: SuppoClient,
  name: string,
  args: Record<string, unknown>,
): Promise<unknown> {
  switch (name) {
    // Read tools
    case "gogomail_find_user": {
      const { email } = EmailSchema.parse(args);
      return gogomail.findUser(email);
    }
    case "gogomail_get_user": {
      const { userId } = UserIdSchema.parse(args);
      return gogomail.getUser(userId);
    }
    case "gogomail_get_user_quota": {
      const { userId } = UserIdSchema.parse(args);
      return gogomail.getUserQuota(userId);
    }
    case "gogomail_get_mail_logs": {
      const p = MailLogsSchema.parse(args);
      return gogomail.getMailLogs(p);
    }
    case "gogomail_trace_message": {
      const { messageId } = MessageIdSchema.parse(args);
      return gogomail.traceMessage(messageId);
    }
    case "gogomail_get_delivery_attempts": {
      const { messageId } = MessageIdSchema.parse(args);
      return gogomail.getDeliveryAttempts(messageId);
    }
    case "gogomail_get_audit_logs": {
      const p = AuditLogsSchema.parse(args);
      return gogomail.getAuditLogs(p);
    }
    case "gogomail_list_user_sessions": {
      const { userId } = UserIdSchema.parse(args);
      return gogomail.listUserSessions(userId);
    }
    case "gogomail_check_health": {
      return gogomail.checkHealth();
    }
    // Action tools
    case "gogomail_reset_password": {
      const { userId, ticketId } = ResetPasswordSchema.parse(args);
      const result = await gogomail.resetPassword(userId);
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_reset_password",
        `userId: ${userId}`,
        "비밀번호 재설정 메일 발송",
      );
      return result;
    }
    case "gogomail_update_user_status": {
      const { userId, status, ticketId } = UpdateStatusSchema.parse(args);
      const before = await gogomail.getUser(userId);
      const result = await gogomail.updateUserStatus(userId, status);
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_status",
        `${before.email} (userId: ${userId})`,
        `${before.status} → ${status}`,
      );
      return result;
    }
    case "gogomail_update_user_quota": {
      const { userId, quotaBytes, ticketId } = UpdateQuotaSchema.parse(args);
      const before = await gogomail.getUserQuota(userId);
      const result = await gogomail.updateUserQuota(userId, quotaBytes);
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_quota",
        `userId: ${userId}`,
        `${before.allocatedBytes} → ${quotaBytes} bytes`,
      );
      return result;
    }
    case "gogomail_revoke_sessions": {
      const { userId, ticketId } = RevokeSessionsSchema.parse(args);
      const result = await gogomail.revokeSessions(userId);
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_revoke_sessions",
        `userId: ${userId}`,
        `${result.revoked}개 세션 강제 종료`,
      );
      return result;
    }
    case "gogomail_update_user_role": {
      const { userId, role, ticketId } = UpdateRoleSchema.parse(args);
      const before = await gogomail.getUser(userId);
      const result = await gogomail.updateUserRole(userId, role);
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_role",
        `${before.email} (userId: ${userId})`,
        `${before.role} → ${role}`,
      );
      return result;
    }
    case "gogomail_get_company": {
      const { companyId } = CompanyIdSchema.parse(args);
      return gogomail.getCompany(companyId);
    }
    case "gogomail_get_domain_settings": {
      const { domainId } = DomainIdSchema.parse(args);
      return gogomail.getDomainSettings(domainId);
    }
    case "gogomail_update_domain_settings": {
      const { domainId, settings, ticketId } = UpdateDomainSchema.parse(args);
      const result = await gogomail.updateDomainSettings(
        domainId,
        settings as Partial<GogomailDomainSettings>,
      );
      await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_domain_settings",
        `domainId: ${domainId}`,
        JSON.stringify(settings),
      );
      return result;
    }
    case "gogomail_get_alert_events": {
      const p = AlertEventsSchema.parse(args);
      return gogomail.getAlertEvents(p);
    }
    default:
      throw new Error(`Unknown GoGoMail tool: ${name}`);
  }
}
