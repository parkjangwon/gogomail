import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient } from "../clients/gogomail.js";
import {
  type OptionalSuppo,
  id, reason, confirm, orgRole, orgTitle,
  withAudit, requireConfirm, writeAuditComment,
} from "./shared.js";

// ── Tool definitions ─────────────────────────────────────────────────

export const toolDefinitions: Tool[] = [
  {
    name: "gogomail_list_org_units",
    description: "List organization units for a company. Mirrors the admin console organization chart API.",
    inputSchema: {
      type: "object",
      properties: { companyId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_get_org_hierarchy",
    description: "Get the organization hierarchy tree for a company.",
    inputSchema: {
      type: "object",
      properties: { companyId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
      required: ["companyId"],
    },
  },
  {
    name: "gogomail_list_user_org_memberships",
    description: "List organization unit memberships for a user, including role/title metadata used by the admin console.",
    inputSchema: {
      type: "object",
      properties: { userId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 } },
      required: ["userId"],
    },
  },
  {
    name: "gogomail_assign_user_org_membership",
    description:
      "Assign a user to an organization unit with optional role/title. Requires reason and writes an audit memo.",
    inputSchema: {
      type: "object",
      properties: {
        unitId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        userId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        role: { type: "string", maxLength: 64 },
        title: { type: "string", maxLength: 128 },
        reason: { type: "string", maxLength: 500 },
        ticketId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["unitId", "userId", "reason"],
    },
  },
  {
    name: "gogomail_update_org_membership",
    description:
      "Update an organization membership role/title. Requires reason and writes an audit memo.",
    inputSchema: {
      type: "object",
      properties: {
        membershipId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        role: { type: "string", maxLength: 64 },
        title: { type: "string", maxLength: 128 },
        reason: { type: "string", maxLength: 500 },
        ticketId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["membershipId", "reason"],
    },
  },
  {
    name: "gogomail_remove_org_membership",
    description:
      "Remove an organization membership. Requires reason and confirm exactly equal to remove org membership <membershipId>.",
    inputSchema: {
      type: "object",
      properties: {
        membershipId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
        confirm: { type: "string", maxLength: 160 },
        reason: { type: "string", maxLength: 500 },
        ticketId: { type: "string", pattern: "^[A-Za-z0-9_-]+$", maxLength: 128 },
      },
      required: ["membershipId", "confirm", "reason"],
    },
  },
];

// ── Zod schemas ──────────────────────────────────────────────────────

const OrgCompanySchema = z.object({ companyId: id() });
const UserOrgMembershipsSchema = z.object({ userId: id() });
const AssignOrgMembershipSchema = z.object({
  unitId: id(),
  userId: id(),
  role: orgRole(),
  title: orgTitle(),
  reason: reason(),
  ticketId: id().optional(),
});
const UpdateOrgMembershipSchema = z.object({
  membershipId: id(),
  role: orgRole(),
  title: orgTitle(),
  reason: reason(),
  ticketId: id().optional(),
}).refine((p) => p.role !== undefined || p.title !== undefined, {
  message: "role or title must be provided",
});
const RemoveOrgMembershipSchema = z.object({
  membershipId: id(),
  confirm: confirm(),
  reason: reason(),
  ticketId: id().optional(),
});

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_list_org_units: OrgCompanySchema,
  gogomail_get_org_hierarchy: OrgCompanySchema,
  gogomail_list_user_org_memberships: UserOrgMembershipsSchema,
  gogomail_assign_user_org_membership: AssignOrgMembershipSchema,
  gogomail_update_org_membership: UpdateOrgMembershipSchema,
  gogomail_remove_org_membership: RemoveOrgMembershipSchema,
};

// ── callTool dispatcher ──────────────────────────────────────────────

export async function callTool(
  gogomail: GogomailClient,
  suppo: OptionalSuppo,
  name: string,
  args: unknown,
): Promise<unknown> {
  switch (name) {
    case "gogomail_list_org_units": {
      const { companyId } = OrgCompanySchema.parse(args);
      return gogomail.adminRequest("GET", `/organization/units?company_id=${encodeURIComponent(companyId)}`);
    }
    case "gogomail_get_org_hierarchy": {
      const { companyId } = OrgCompanySchema.parse(args);
      return gogomail.adminRequest("GET", `/organization/hierarchy?company_id=${encodeURIComponent(companyId)}`);
    }
    case "gogomail_list_user_org_memberships": {
      const { userId } = UserOrgMembershipsSchema.parse(args);
      return gogomail.adminRequest("GET", `/organization/members?user_id=${encodeURIComponent(userId)}`);
    }
    case "gogomail_assign_user_org_membership": {
      const { unitId, userId, role, title, reason: rsn, ticketId } = AssignOrgMembershipSchema.parse(args);
      const result = await gogomail.adminRequest("POST", "/organization/members", {
        unit_id: unitId,
        user_id: userId,
        role,
        title,
      });
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_assign_user_org_membership",
        `unitId: ${unitId}, userId: ${userId}`,
        `조직 구성원 배정 / role: ${role ?? "(none)"}, title: ${title ?? "(none)"} / 사유: ${rsn}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_update_org_membership": {
      const { membershipId, role, title, reason: rsn, ticketId } = UpdateOrgMembershipSchema.parse(args);
      const result = await gogomail.adminRequest("PATCH", `/organization/members/${membershipId}`, {
        role,
        title,
      });
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_org_membership",
        `membershipId: ${membershipId}`,
        `조직 구성원 메타데이터 변경 / role: ${role ?? "(unchanged)"}, title: ${title ?? "(unchanged)"} / 사유: ${rsn}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_remove_org_membership": {
      const { membershipId, confirm: conf, reason: rsn, ticketId } = RemoveOrgMembershipSchema.parse(args);
      requireConfirm(conf, `remove org membership ${membershipId}`);
      await gogomail.adminRequest("DELETE", `/organization/members/${membershipId}`);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_remove_org_membership",
        `membershipId: ${membershipId}`,
        `조직 구성원 배정 제거 / 사유: ${rsn}`,
      );
      return { status: "ok", membershipId, audit };
    }
    default:
      throw new Error(`Unknown tool: ${name}`);
  }
}
