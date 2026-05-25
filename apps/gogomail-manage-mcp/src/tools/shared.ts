/**
 * Shared helpers, Zod primitives, and audit utilities used across domain modules.
 */

import { isIP } from "node:net";
import { z } from "zod";
import type { SuppoClient } from "../clients/suppo.js";

export type OptionalSuppo = SuppoClient | null;

// ── ID / field validators ────────────────────────────────────────────

/** GoGoMail entity IDs — alphanumeric, hyphens, underscores only */
export const id = () =>
  z.string().max(128).regex(/^[A-Za-z0-9_-]+$/, "ID must be alphanumeric with hyphens/underscores only");

/** DLQ / outbox stream names — same character set as IDs */
export const stream = () =>
  z.string().max(128).regex(/^[A-Za-z0-9_-]+$/, "stream name must be alphanumeric with hyphens/underscores only");

export const email = () => z.string().email().max(254);

export const singleLine = (name: string, max: number) =>
  z.string().trim().min(1).max(max).regex(/^[^\r\n]+$/, `${name} must be a single line`);

export const reason = () => singleLine("reason", 500);
export const confirm = () => singleLine("confirm", 160);
export const orgRole = () => singleLine("role", 64).optional();
export const orgTitle = () => singleLine("title", 128).optional();

/** Bounded page limit — prevents agents from issuing unbounded backend queries */
export const pageLimit = () => z.number().int().min(1).max(200).optional();

const isoTimestampPattern =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/;

export const ts = () =>
  z.string()
    .max(64)
    .regex(isoTimestampPattern, "must be a complete ISO 8601 timestamp with timezone")
    .refine((value) => !Number.isNaN(Date.parse(value)), "must be a valid ISO 8601 timestamp");

export const ipOrCidr = () =>
  singleLine("ip whitelist entry", 128).refine((value) => {
    const parts = value.split("/");
    if (parts.length > 2) return false;
    const version = isIP(parts[0] ?? "");
    if (!version) return false;
    if (parts.length === 1) return true;
    const prefix = parts[1] ?? "";
    if (!/^\d+$/.test(prefix)) return false;
    const prefixNumber = Number(prefix);
    return version === 4
      ? prefixNumber >= 0 && prefixNumber <= 32
      : prefixNumber >= 0 && prefixNumber <= 128;
  }, "must be an IPv4/IPv6 address or CIDR range");

// ── Date range validators ────────────────────────────────────────────

export const validSinceUntil = (p: { since?: string; until?: string }) =>
  !p.since || !p.until || Date.parse(p.since) <= Date.parse(p.until);

export const validFromTo = (p: { from?: string; to?: string }) =>
  !p.from || !p.to || Date.parse(p.from) <= Date.parse(p.to);

// ── Enum definitions ─────────────────────────────────────────────────

export const userStatus = z.enum(["active", "suspended", "disabled"]);
export const companyStatus = z.enum(["active", "suspended"]);
export const domainStatus = z.enum(["active", "suspended"]);
export const dnsStatus = z.enum(["verified", "unverified", "partial"]);
export const direction = z.enum(["inbound", "outbound"]);
export const mailFlowStatus = z.enum(["delivered", "bounced", "deferred", "rejected", "quarantined", "expired"]);
export const deliveryStatus = z.enum(["pending", "success", "failed", "exhausted"]);
export const suppressionReason = z.enum(["bounce", "complaint", "manual"]);
export const role = z.enum(["user", "company_admin", "system_admin"]);
export const adminMethod = z.enum(["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"]);
export const adminQueryValue = z.union([z.string().max(1000), z.number(), z.boolean(), z.null()]);
export const adminQuery = z.record(adminQueryValue).optional();
export const adminJsonBody = z.unknown().optional();
export const securityScope = z.enum(["company", "domain"]);
export const securityPolicy = z.enum([
  "ip-policy",
  "auth-policy",
  "retention-policy",
  "session-policy",
  "rate-limit",
  "dmarc-spf",
  "mcp-policy",
  "smtp-policy",
  "spam-filter",
]);

// ── Admin API allowlist & path helpers ───────────────────────────────

export const ADMIN_API_ALLOWLIST: RegExp[] = [
  /^\/admin-users(?:\/[A-Za-z0-9_-]+)?$/,
  /^\/users(?:\/[A-Za-z0-9_-]+(?:\/(?:quota|role|recovery-email|status|invite|mfa))?)?$/,
  /^\/companies(?:\/[A-Za-z0-9_-]+(?:\/(?:users\/bulk-import|users\/bulk-export|quota-summary|routing-rules|legal-holds(?:\/[A-Za-z0-9_-]+)?|sessions(?:\/[A-Za-z0-9_-]+)?|config(?:\/[A-Za-z0-9_-]+)?|mfa\/stats|alert-events|security\/(?:ip-policy|auth-policy|retention-policy|session-policy|rate-limit|spam-filter(?:\/(?:events|stats))?)))?)?$/,
  /^\/domains(?:\/[A-Za-z0-9_-]+(?:\/(?:settings|dns-check|config(?:\/[A-Za-z0-9_-]+)?|routing-rules|mcp-policy|smtp-policy|security\/(?:ip-policy|auth-policy|retention-policy|rate-limit|dmarc-spf|spam-filter)))?)?$/,
  /^\/organization\/(?:units(?:\/[A-Za-z0-9_-]+)?|hierarchy|members(?:\/[A-Za-z0-9_-]+)?|sync|settings)$/,
  /^\/(?:directory\/principals|mail-flow-logs(?:\/stats)?|delivery-attempts(?:\/exhausted)?|outbox\/[A-Za-z0-9_-]+\/retry|dlq(?:\/[A-Za-z0-9_-]+)?|suppression-list(?:\/[A-Za-z0-9_-]+)?|quota-usage|quota-alerts|audit-logs|dkim-keys|health|queue|delivery-routes(?:\/[A-Za-z0-9_-]+)?|trusted-relays(?:\/[A-Za-z0-9_-]+)?|attachments|attachment-cleanup\/runs|drive-nodes|quota-reconciliation)$/,
];

export function normalizeAdminPath(path: string): string {
  let normalized = path.trim();
  if (normalized.startsWith("/admin/v1/")) {
    normalized = normalized.slice("/admin/v1".length);
  }
  if (!normalized.startsWith("/")) normalized = `/${normalized}`;
  if (
    normalized.length > 1024 ||
    normalized.includes("?") ||
    normalized.includes("#") ||
    normalized.includes("://") ||
    normalized.includes("..") ||
    /[\r\n]/.test(normalized)
  ) {
    throw new Error("path must be a clean /admin/v1-relative path without query, fragment, traversal, or URL scheme");
  }
  if (!ADMIN_API_ALLOWLIST.some((pattern) => pattern.test(normalized))) {
    throw new Error(`Admin API path is not in the documented MCP allowlist: ${normalized}`);
  }
  return normalized;
}

export function pathWithQuery(path: string, query?: Record<string, string | number | boolean | null>): string {
  const qs = new URLSearchParams();
  for (const [key, value] of Object.entries(query ?? {})) {
    if (!/^[A-Za-z0-9_.-]+$/.test(key)) {
      throw new Error(`query key contains unsupported characters: ${key}`);
    }
    if (value !== null) qs.set(key, String(value));
  }
  const encoded = qs.toString();
  return encoded ? `${path}?${encoded}` : path;
}

export function securityPolicyPath(scope: "company" | "domain", entityId: string, policy: z.infer<typeof securityPolicy>): string {
  if (policy === "mcp-policy") {
    if (scope !== "domain") throw new Error("mcp-policy is domain-scoped");
    return `/domains/${entityId}/mcp-policy`;
  }
  if (policy === "smtp-policy") {
    if (scope !== "domain") throw new Error("smtp-policy is domain-scoped");
    return `/domains/${entityId}/smtp-policy`;
  }
  if (policy === "dmarc-spf") {
    if (scope !== "domain") throw new Error("dmarc-spf is domain-scoped");
    return `/domains/${entityId}/security/dmarc-spf`;
  }
  return scope === "company"
    ? `/companies/${entityId}/security/${policy}`
    : `/domains/${entityId}/security/${policy}`;
}

// ── Audit helper ─────────────────────────────────────────────────────

export type AuditResult =
  | { status: "written"; destination: "suppo_ticket"; ticketId: string }
  | { status: "written"; destination: "suppo_audit_ticket" }
  | { status: "written"; destination: "stderr" }
  | { status: "failed"; error: string };

const sanitizeAuditField = (s: string) => s.replace(/[\r\n]/g, " ").slice(0, 500);
const auditErrorMessage = (e: unknown) =>
  (e instanceof Error ? e.message : String(e)).replace(/[\r\n]/g, " ").slice(0, 500);

export function withAudit<T>(result: T, audit: AuditResult): unknown {
  if (result && typeof result === "object" && !Array.isArray(result)) {
    return { ...(result as Record<string, unknown>), audit };
  }
  return { result, audit };
}

export function describeUser(user: { username?: string; display_name?: string; id: string }): string {
  const label = user.display_name ? `${user.username ?? "(unknown)"} / ${user.display_name}` : (user.username ?? "(unknown)");
  return `${label} (userId: ${user.id})`;
}

export function requireConfirm(actual: string, expected: string): void {
  if (actual !== expected) {
    throw new Error(`confirm must exactly equal "${expected}"`);
  }
}

export async function writeAuditComment(
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
