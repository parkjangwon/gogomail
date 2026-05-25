import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient } from "../clients/gogomail.js";
import {
  type OptionalSuppo,
  id, email, singleLine, reason, pageLimit, ts,
  securityScope, securityPolicy,
  validSinceUntil,
  pathWithQuery, securityPolicyPath,
  withAudit, writeAuditComment,
} from "./shared.js";

// ── Tool definitions ─────────────────────────────────────────────────

export const toolDefinitions: Tool[] = [
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
    name: "gogomail_get_security_policy",
    description:
      "Get a company or domain security policy: ip-policy, auth-policy, retention-policy, session-policy, rate-limit, dmarc-spf, mcp-policy, smtp-policy, or spam-filter.",
    inputSchema: {
      type: "object",
      properties: {
        scope: { type: "string", enum: ["company", "domain"] },
        id: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        policy: { type: "string", enum: ["ip-policy", "auth-policy", "retention-policy", "session-policy", "rate-limit", "dmarc-spf", "mcp-policy", "smtp-policy", "spam-filter"] },
      },
      required: ["scope", "id", "policy"],
    },
  },
  {
    name: "gogomail_update_security_policy",
    description:
      "Update a company or domain security policy. Requires reason and writes an audit memo.",
    inputSchema: {
      type: "object",
      properties: {
        scope: { type: "string", enum: ["company", "domain"] },
        id: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        policy: { type: "string", enum: ["ip-policy", "auth-policy", "retention-policy", "session-policy", "rate-limit", "dmarc-spf", "mcp-policy", "smtp-policy", "spam-filter"] },
        bodyJson: { description: "Policy JSON body to PUT" },
        reason: { type: "string", maxLength: 500 },
        ticketId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["scope", "id", "policy", "bodyJson", "reason"],
    },
  },
  {
    name: "gogomail_get_spam_filter_policy",
    description: "Get company- or domain-scoped spam filter policy.",
    inputSchema: {
      type: "object",
      properties: {
        scope: { type: "string", enum: ["company", "domain"] },
        id: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["scope", "id"],
    },
  },
  {
    name: "gogomail_update_spam_filter_policy",
    description: "Update company- or domain-scoped spam filter policy. Requires reason and writes an audit memo.",
    inputSchema: {
      type: "object",
      properties: {
        scope: { type: "string", enum: ["company", "domain"] },
        id: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        bodyJson: { description: "Spam filter policy JSON body to PUT" },
        reason: { type: "string", maxLength: 500 },
        ticketId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["scope", "id", "bodyJson", "reason"],
    },
  },
  {
    name: "gogomail_get_spam_filter_stats",
    description: "Get spam filter statistics for a company, optionally scoped to a domain.",
    inputSchema: {
      type: "object",
      properties: {
        companyId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        domainId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        userId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        since: { type: "string", description: "ISO 8601 start time" },
        until: { type: "string", description: "ISO 8601 end time" },
      },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_list_spam_filter_events",
    description:
      "List spam filter decision logs with console-equivalent filters. Use this to inspect blocked, rejected, quarantined, or suspicious accepted messages.",
    inputSchema: {
      type: "object",
      properties: {
        companyId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        domainId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        userId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        fromAddr: { type: "string", format: "email", maxLength: 254 },
        toAddr: { type: "string", format: "email", maxLength: 254 },
        subject: { type: "string", maxLength: 200 },
        flowStatus: { type: "string", enum: ["filtered", "rejected", "received"] },
        since: { type: "string", description: "ISO 8601 start time" },
        until: { type: "string", description: "ISO 8601 end time" },
        limit: { type: "number", minimum: 1, maximum: 200 },
      },
      required: ["companyId"],
    },
  },
];

// ── Zod schemas ──────────────────────────────────────────────────────

const GetSpamFilterSchema = z.object({ companyId: id() });
const ListDkimSchema = z.object({ domainId: id().optional() });
const SecurityPolicySchema = z.object({
  scope: securityScope,
  id: id(),
  policy: securityPolicy,
});
const UpdateSecurityPolicySchema = SecurityPolicySchema.extend({
  bodyJson: z.unknown(),
  reason: reason(),
  ticketId: id().optional(),
});
const SpamFilterPolicySchema = z.object({
  scope: securityScope,
  id: id(),
});
const UpdateSpamFilterPolicySchema = SpamFilterPolicySchema.extend({
  bodyJson: z.unknown(),
  reason: reason(),
  ticketId: id().optional(),
});
const SpamFilterStatsSchema = z.object({
  companyId: id(),
  domainId: id().optional(),
  userId: id().optional(),
  since: ts().optional(),
  until: ts().optional(),
}).refine(validSinceUntil, {
  message: "since must be earlier than or equal to until",
});
const ListSpamFilterEventsSchema = z.object({
  companyId: id(),
  domainId: id().optional(),
  userId: id().optional(),
  fromAddr: email().optional(),
  toAddr: email().optional(),
  subject: singleLine("subject", 200).optional(),
  flowStatus: z.enum(["filtered", "rejected", "received"]).optional(),
  since: ts().optional(),
  until: ts().optional(),
  limit: pageLimit(),
}).refine(validSinceUntil, {
  message: "since must be earlier than or equal to until",
});

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_get_spam_filter: GetSpamFilterSchema,
  gogomail_get_spam_filter_events: GetSpamFilterSchema,
  gogomail_list_dkim_keys: ListDkimSchema,
  gogomail_get_security_policy: SecurityPolicySchema,
  gogomail_update_security_policy: UpdateSecurityPolicySchema,
  gogomail_get_spam_filter_policy: SpamFilterPolicySchema,
  gogomail_update_spam_filter_policy: UpdateSpamFilterPolicySchema,
  gogomail_get_spam_filter_stats: SpamFilterStatsSchema,
  gogomail_list_spam_filter_events: ListSpamFilterEventsSchema,
};

// ── callTool dispatcher ──────────────────────────────────────────────

export async function callTool(
  gogomail: GogomailClient,
  suppo: OptionalSuppo,
  name: string,
  args: unknown,
): Promise<unknown> {
  switch (name) {
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
    case "gogomail_get_security_policy": {
      const { scope, id: entityId, policy } = SecurityPolicySchema.parse(args);
      return gogomail.adminRequest("GET", securityPolicyPath(scope, entityId, policy));
    }
    case "gogomail_update_security_policy": {
      const { scope, id: entityId, policy, bodyJson, reason: rsn, ticketId } = UpdateSecurityPolicySchema.parse(args);
      const path = securityPolicyPath(scope, entityId, policy);
      const result = await gogomail.adminRequest("PUT", path, bodyJson);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_security_policy",
        `${scope}: ${entityId}, policy: ${policy}`,
        `보안 정책 변경 / 사유: ${rsn}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_get_spam_filter_policy": {
      const { scope, id: entityId } = SpamFilterPolicySchema.parse(args);
      return gogomail.adminRequest("GET", securityPolicyPath(scope, entityId, "spam-filter"));
    }
    case "gogomail_update_spam_filter_policy": {
      const { scope, id: entityId, bodyJson, reason: rsn, ticketId } = UpdateSpamFilterPolicySchema.parse(args);
      const path = securityPolicyPath(scope, entityId, "spam-filter");
      const result = await gogomail.adminRequest("PUT", path, bodyJson);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_spam_filter_policy",
        `${scope}: ${entityId}`,
        `스팸 필터 정책 변경 / 사유: ${rsn}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_get_spam_filter_stats": {
      const { companyId, domainId, userId, since, until } = SpamFilterStatsSchema.parse(args);
      return gogomail.adminRequest("GET", pathWithQuery(`/companies/${companyId}/security/spam-filter/stats`, {
        domain_id: domainId ?? null,
        user_id: userId ?? null,
        since: since ?? null,
        until: until ?? null,
      }));
    }
    case "gogomail_list_spam_filter_events": {
      const p = ListSpamFilterEventsSchema.parse(args);
      return gogomail.adminRequest("GET", pathWithQuery(`/companies/${p.companyId}/security/spam-filter/events`, {
        domain_id: p.domainId ?? null,
        user_id: p.userId ?? null,
        from_addr: p.fromAddr ?? null,
        to_addr: p.toAddr ?? null,
        subject: p.subject ?? null,
        flow_status: p.flowStatus ?? null,
        since: p.since ?? null,
        until: p.until ?? null,
        limit: p.limit ?? null,
      }));
    }
    default:
      throw new Error(`Unknown tool: ${name}`);
  }
}
