import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient } from "../clients/gogomail.js";
import {
  type OptionalSuppo,
  id, reason, confirm, pageLimit, ts,
  adminMethod, adminQuery, adminJsonBody,
  validFromTo,
  normalizeAdminPath, pathWithQuery, requireConfirm,
  withAudit, writeAuditComment,
} from "./shared.js";

// ── Tool definitions ─────────────────────────────────────────────────

export const toolDefinitions: Tool[] = [
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
  {
    name: "gogomail_admin_api_request",
    description:
      "Guarded bridge for documented GoGoMail Admin API routes used by the admin console. Path is relative to /admin/v1, for example /organization/units. Non-read methods require reason and are audit-logged; DELETE also requires confirm exactly equal to DELETE <path>.",
    inputSchema: {
      type: "object",
      properties: {
        method: { type: "string", enum: ["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"] },
        path: { type: "string", description: "Admin API path relative to /admin/v1; no query string", maxLength: 1024 },
        query: { type: "object", description: "Optional query parameters as string, number, boolean, or null values" },
        bodyJson: { description: "JSON request body for POST, PUT, PATCH, or DELETE requests" },
        reason: { type: "string", description: "Operational reason required for non-read requests", maxLength: 500 },
        confirm: { type: "string", description: "For DELETE only, must exactly equal: DELETE <path>", maxLength: 160 },
        ticketId: { type: "string", description: "Suppo ticket ID to attach audit memo to", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["method", "path"],
    },
  },
];

// ── Zod schemas ──────────────────────────────────────────────────────

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
}).refine(validFromTo, {
  message: "from must be earlier than or equal to to",
});
const AdminApiRequestSchema = z.object({
  method: adminMethod,
  path: z.string().trim().min(1).max(1024).regex(/^[^\r\n]+$/, "path must be a single line"),
  query: adminQuery,
  bodyJson: adminJsonBody,
  reason: reason().optional(),
  confirm: confirm().optional(),
  ticketId: id().optional(),
});

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_get_alert_events: AlertEventsSchema,
  gogomail_get_audit_logs: AuditLogsSchema,
  gogomail_check_health: z.object({}),
  gogomail_get_queue_stats: z.object({}),
  gogomail_admin_api_request: AdminApiRequestSchema,
};

// ── callTool dispatcher ──────────────────────────────────────────────

export async function callTool(
  gogomail: GogomailClient,
  suppo: OptionalSuppo,
  name: string,
  args: unknown,
): Promise<unknown> {
  switch (name) {
    case "gogomail_get_alert_events": {
      const p = AlertEventsSchema.parse(args);
      return gogomail.getAlertEvents(p);
    }
    case "gogomail_get_audit_logs": {
      const p = AuditLogsSchema.parse(args);
      return gogomail.getAuditLogs(p);
    }
    case "gogomail_check_health": {
      return gogomail.checkHealth();
    }
    case "gogomail_get_queue_stats": {
      return gogomail.getQueueStats();
    }
    case "gogomail_admin_api_request": {
      const { method, path, query, bodyJson, reason: rsn, confirm: conf, ticketId } = AdminApiRequestSchema.parse(args);
      const normalizedPath = normalizeAdminPath(path);
      const requestPath = pathWithQuery(normalizedPath, query);
      if (method !== "GET" && method !== "HEAD" && !rsn) {
        throw new Error("reason is required for non-read Admin API requests");
      }
      if (method === "DELETE") {
        requireConfirm(conf ?? "", `DELETE ${normalizedPath}`);
      }
      const result = await gogomail.adminRequest(method, requestPath, bodyJson);
      if (method === "GET" || method === "HEAD") return result;
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_admin_api_request",
        `${method} ${requestPath}`,
        `관리자 API 요청 / 사유: ${rsn}`,
      );
      return withAudit(result ?? { status: "ok" }, audit);
    }
    default:
      throw new Error(`Unknown tool: ${name}`);
  }
}
