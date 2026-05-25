import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import type { GogomailClient } from "../clients/gogomail.js";
import {
  type OptionalSuppo,
  id, email, singleLine, reason, confirm, pageLimit,
  userStatus, role,
  withAudit, describeUser, requireConfirm, writeAuditComment,
} from "./shared.js";

// ── Tool definitions ─────────────────────────────────────────────────

export const toolDefinitions: Tool[] = [
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
];

// ── Zod schemas ──────────────────────────────────────────────────────

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
const UpdateStatusSchema = z.object({
  userId: id(),
  status: userStatus,
  reason: reason(),
  ticketId: id().optional(),
});
const UpdateQuotaSchema = z.object({
  userId: id(),
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
const SendInviteSchema = z.object({
  userId: id(),
  reason: reason(),
  ticketId: id().optional(),
});

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_search_principals: SearchPrincipalsSchema,
  gogomail_list_users: ListUsersSchema,
  gogomail_get_user: UserIdSchema,
  gogomail_get_user_quota: UserIdSchema,
  gogomail_update_user_status: UpdateStatusSchema,
  gogomail_update_user_quota: UpdateQuotaSchema,
  gogomail_update_user_role: UpdateRoleSchema,
  gogomail_update_user_recovery_email: UpdateRecoveryEmailSchema,
  gogomail_create_user: CreateUserSchema,
  gogomail_delete_user: DeleteUserSchema,
  gogomail_send_invite_email: SendInviteSchema,
};

// ── callTool dispatcher ──────────────────────────────────────────────

export async function callTool(
  gogomail: GogomailClient,
  suppo: OptionalSuppo,
  name: string,
  args: unknown,
): Promise<unknown> {
  switch (name) {
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
    case "gogomail_update_user_status": {
      const { userId, status, reason: rsn, ticketId } = UpdateStatusSchema.parse(args);
      const before = await gogomail.getUser(userId);
      const result = await gogomail.updateUserStatus(userId, status);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_status",
        describeUser(before),
        `${before.status} → ${status} / 사유: ${rsn}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_update_user_quota": {
      const { userId, quotaBytes, reason: rsn, ticketId } = UpdateQuotaSchema.parse(args);
      const before = await gogomail.getUserQuota(userId);
      const result = await gogomail.updateUserQuota(userId, quotaBytes);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_quota",
        `userId: ${userId}`,
        `${before.allocatedBytes} → ${quotaBytes} bytes / 사유: ${rsn}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_update_user_role": {
      const { userId, role: newRole, reason: rsn, ticketId } = UpdateRoleSchema.parse(args);
      const before = await gogomail.getUser(userId);
      const result = await gogomail.updateUserRole(userId, newRole);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_role",
        describeUser(before),
        `${before.role} → ${newRole} / 사유: ${rsn}`,
      );
      return withAudit(result, audit);
    }
    case "gogomail_update_user_recovery_email": {
      const { userId, recoveryEmail, reason: rsn, ticketId } = UpdateRecoveryEmailSchema.parse(args);
      const user = await gogomail.getUser(userId);
      await gogomail.updateUserRecoveryEmail(userId, recoveryEmail);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_update_user_recovery_email",
        describeUser(user),
        `복구 이메일 변경 → ${recoveryEmail} / 사유: ${rsn}`,
      );
      return { status: "ok", userId, recoveryEmail, audit };
    }
    case "gogomail_create_user": {
      const { domainId, username, displayName, recoveryEmail, password, quotaLimit, reason: rsn, ticketId } =
        CreateUserSchema.parse(args);
      const user = await gogomail.createUser({ domainId, username, displayName, recoveryEmail, password, quotaLimit });
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_create_user",
        describeUser(user),
        `신규 사용자 생성 / domainId: ${domainId} / 사유: ${rsn}`,
      );
      return withAudit(user, audit);
    }
    case "gogomail_delete_user": {
      const { userId, confirm: conf, reason: rsn, ticketId } = DeleteUserSchema.parse(args);
      requireConfirm(conf, `delete ${userId}`);
      const user = await gogomail.getUser(userId);
      await gogomail.deleteUser(userId);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_delete_user",
        describeUser(user),
        `사용자 계정 영구 삭제 / 사유: ${rsn}`,
      );
      return { status: "ok", deletedUserId: userId, username: user.username, audit };
    }
    case "gogomail_send_invite_email": {
      const { userId, reason: rsn, ticketId } = SendInviteSchema.parse(args);
      const user = await gogomail.getUser(userId);
      const result = await gogomail.sendInviteEmail(userId);
      const audit = await writeAuditComment(
        suppo,
        ticketId,
        "gogomail_send_invite_email",
        describeUser(user),
        `초대 이메일 발송 (비밀번호 설정 링크) / 사유: ${rsn}`,
      );
      return withAudit(result, audit);
    }
    default:
      throw new Error(`Unknown tool: ${name}`);
  }
}
