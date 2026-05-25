import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient } from "../clients/gogomail.js";
import {
  type OptionalSuppo,
  id, reason, pageLimit, ipOrCidr,
  companyStatus, domainStatus, dnsStatus,
  withAudit, writeAuditComment,
} from "./shared.js";

// ── Tool definitions ─────────────────────────────────────────────────

export const toolDefinitions: Tool[] = [
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
];

// ── Zod schemas ──────────────────────────────────────────────────────

const ListCompaniesSchema = z.object({
  status: companyStatus.optional(),
  limit: pageLimit(),
});
const CompanyIdSchema = z.object({ companyId: id() });
const ListDomainsSchema = z.object({
  companyId: id().optional(),
  status: domainStatus.optional(),
  dnsStatus: dnsStatus.optional(),
  limit: pageLimit(),
});
const DomainIdSchema = z.object({ domainId: id() });
const UpdateDomainSchema = z.object({
  domainId: id(),
  settings: z.object({
    tls_policy: z.enum(["opportunistic", "require", "disable"]).optional(),
    quota_per_user: z.number().int().min(1).max(10_995_116_277_760).optional(),
    ip_whitelist_enabled: z.boolean().optional(),
    ip_whitelist: z.array(ipOrCidr()).max(200).optional(),
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

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_list_companies: ListCompaniesSchema,
  gogomail_get_company: CompanyIdSchema,
  gogomail_list_domains: ListDomainsSchema,
  gogomail_get_domain_settings: DomainIdSchema,
  gogomail_check_domain_dns: DomainIdSchema,
  gogomail_update_domain_settings: UpdateDomainSchema,
  gogomail_list_company_sessions: ListCompanySessionsSchema,
  gogomail_revoke_company_session: RevokeSessionSchema,
};

// ── callTool dispatcher ──────────────────────────────────────────────

export async function callTool(
  gogomail: GogomailClient,
  suppo: OptionalSuppo,
  name: string,
  args: unknown,
): Promise<unknown> {
  switch (name) {
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
    case "gogomail_update_domain_settings": {
      const { domainId, settings, reason: rsn, ticketId } = UpdateDomainSchema.parse(args);
      const before = await gogomail.getDomainSettings(domainId);
      const mergedSettings = { ...before, ...settings };
      const result = await gogomail.updateDomainSettings(domainId, mergedSettings);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_domain_settings",
        `domainId: ${domainId}`,
        `변경 요청: ${JSON.stringify(settings)} / 사유: ${rsn}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_list_company_sessions": {
      const { companyId } = ListCompanySessionsSchema.parse(args);
      return gogomail.listCompanySessions(companyId);
    }
    case "gogomail_revoke_company_session": {
      const { companyId, userId, reason: rsn, ticketId } = RevokeSessionSchema.parse(args);
      await gogomail.revokeCompanySession(companyId, userId);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_revoke_company_session",
        `companyId: ${companyId}, userId: ${userId}`,
        `세션 강제 종료 / 사유: ${rsn}`,
      );
      return { status: "ok", audit };
    }
    default:
      throw new Error(`Unknown tool: ${name}`);
  }
}
