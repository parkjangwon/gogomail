import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient } from "../clients/gogomail.js";
import {
  type OptionalSuppo,
  id, email, singleLine, reason, confirm, pageLimit, stream, ts,
  direction, mailFlowStatus, deliveryStatus, suppressionReason,
  validSinceUntil,
  withAudit, requireConfirm, writeAuditComment,
} from "./shared.js";

// ── Tool definitions ─────────────────────────────────────────────────

export const toolDefinitions: Tool[] = [
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
];

// ── Zod schemas ──────────────────────────────────────────────────────

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
}).refine(validSinceUntil, {
  message: "since must be earlier than or equal to until",
});
const MailFlowStatsSchema = z.object({
  userId: id().optional(),
  companyId: id().optional(),
  domainId: id().optional(),
  direction: direction.optional(),
  since: ts().optional(),
  until: ts().optional(),
}).refine(validSinceUntil, {
  message: "since must be earlier than or equal to until",
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

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_list_mail_flow_logs: ListMailFlowLogsSchema,
  gogomail_get_mail_flow_stats: MailFlowStatsSchema,
  gogomail_list_delivery_attempts: ListDeliveryAttemptsSchema,
  gogomail_list_exhausted_deliveries: ListExhaustedSchema,
  gogomail_list_dlq: DlqListSchema,
  gogomail_delete_dlq_entry: DlqDeleteSchema,
  gogomail_retry_outbox: RetryOutboxSchema,
  gogomail_list_suppression_list: ListSuppressionSchema,
  gogomail_remove_suppression_entry: RemoveSuppressionSchema,
  gogomail_list_quota_usage: ListQuotaUsageSchema,
  gogomail_list_quota_alerts: ListQuotaAlertsSchema,
};

// ── callTool dispatcher ──────────────────────────────────────────────

export async function callTool(
  gogomail: GogomailClient,
  suppo: OptionalSuppo,
  name: string,
  args: unknown,
): Promise<unknown> {
  switch (name) {
    case "gogomail_list_mail_flow_logs": {
      const p = ListMailFlowLogsSchema.parse(args);
      return gogomail.listMailFlowLogs(p);
    }
    case "gogomail_get_mail_flow_stats": {
      const p = MailFlowStatsSchema.parse(args);
      return gogomail.getMailFlowStats(p);
    }
    case "gogomail_list_delivery_attempts": {
      const p = ListDeliveryAttemptsSchema.parse(args);
      return gogomail.listDeliveryAttempts(p);
    }
    case "gogomail_list_exhausted_deliveries": {
      const p = ListExhaustedSchema.parse(args);
      return gogomail.listExhaustedDeliveries(p);
    }
    case "gogomail_list_dlq": {
      const { stream: s, count } = DlqListSchema.parse(args);
      return gogomail.listDlq(s, count);
    }
    case "gogomail_delete_dlq_entry": {
      const { stream: s, id: entryId, confirm: conf, reason: rsn, ticketId } = DlqDeleteSchema.parse(args);
      requireConfirm(conf, `delete ${s}/${entryId}`);
      await gogomail.deleteDlqEntry(s, entryId);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_delete_dlq_entry",
        `stream: ${s}, id: ${entryId}`,
        `DLQ 항목 삭제 / 사유: ${rsn}`,
      );
      return { status: "ok", stream: s, id: entryId, audit };
    }
    case "gogomail_retry_outbox": {
      const { id: msgId, reason: rsn, ticketId } = RetryOutboxSchema.parse(args);
      const result = await gogomail.retryOutbox(msgId);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_retry_outbox",
        `outbox id: ${msgId}`,
        `발송 재시도 / 사유: ${rsn}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_list_suppression_list": {
      const p = ListSuppressionSchema.parse(args);
      return gogomail.listSuppressionList(p);
    }
    case "gogomail_remove_suppression_entry": {
      const { id: supId, reason: rsn, ticketId } = RemoveSuppressionSchema.parse(args);
      await gogomail.removeSuppressionEntry(supId);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_remove_suppression_entry",
        `suppression id: ${supId}`,
        `수신 거부 해제 / 사유: ${rsn}`,
      );
      return { status: "ok", id: supId, audit };
    }
    case "gogomail_list_quota_usage": {
      const p = ListQuotaUsageSchema.parse(args);
      return gogomail.listQuotaUsage(p);
    }
    case "gogomail_list_quota_alerts": {
      const p = ListQuotaAlertsSchema.parse(args);
      return gogomail.listQuotaAlerts(p);
    }
    default:
      throw new Error(`Unknown tool: ${name}`);
  }
}
