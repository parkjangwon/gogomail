import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { randomUUID } from "node:crypto";
import { z } from "zod";
import { appendQuery, GogomailUserClient, type MCPSettings } from "../client.js";
import {
  apiMethod,
  apiPayloadLimitBytes,
  apiQueryValue,
  avatarPayloadLimitBytes,
  confirm,
} from "./schemas.js";

export const toolDefinitions: Tool[] = [
  { name: "gogomail_mcp_get_settings", description: "Read the current user's MCP automation settings.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_api_request", description: "Agent-native escape hatch for existing GoGoMail user APIs. Allows /api/v1 mail, Drive, calendar, and /api/mail contacts/directory routes; blocks auth/admin/MCP key-management routes. In basic mode, mutations require confirm. Sensitive routes forward X-Gogomail-MCP-Confirm to the backend.", inputSchema: { type: "object", properties: { method: { type: "string", enum: ["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"] }, path: { type: "string", maxLength: 1024 }, query: { type: "object", additionalProperties: { type: "string" } }, body_json: {}, body_text: { type: "string" }, body_base64: { type: "string" }, content_type: { type: "string", maxLength: 128 }, confirm: { type: "string", maxLength: 300 } }, required: ["method", "path"] } },
  { name: "gogomail_webmail_get_capabilities", description: "Read webmail feature limits/capabilities using GET /api/v1/webmail/capabilities.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_mailbox_get_overview", description: "Read mailbox summary counts using GET /api/v1/mailbox/overview.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_account_get_profile", description: "Read the current user's profile/quota using GET /api/v1/me.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_account_update_profile", description: "Update the current user's display name and/or backup recovery email using PATCH /api/v1/me.", inputSchema: { type: "object", properties: { display_name: { type: "string", maxLength: 200 }, recovery_email: { type: "string", format: "email" } } } },
  { name: "gogomail_account_list_addresses", description: "List the current user's sender addresses using GET /api/v1/me/addresses.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_account_upload_avatar", description: "Upload the current user's profile photo using PUT /api/v1/me/avatar. Provide PNG, JPEG, GIF, or WebP bytes as base64, max 256 KiB decoded. In basic mode confirm must equal `upload avatar`.", inputSchema: { type: "object", properties: { avatar_base64: { type: "string", maxLength: 350000 }, mime_type: { type: "string", enum: ["image/png", "image/jpeg", "image/gif", "image/webp"] }, filename: { type: "string", maxLength: 255 }, confirm: { type: "string", maxLength: 300 } }, required: ["avatar_base64", "mime_type"] } },
  { name: "gogomail_account_delete_avatar", description: "Remove the current user's profile photo using DELETE /api/v1/me/avatar. In basic mode confirm must equal `delete avatar`.", inputSchema: { type: "object", properties: { confirm: { type: "string", maxLength: 300 } } } },
  { name: "gogomail_preferences_get", description: "Read current webmail preferences using GET /api/v1/preferences. Writes stay available through gogomail_api_request to avoid accidental full-object clobbering.", inputSchema: { type: "object", properties: {} } },
];

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_mcp_get_settings: z.object({}),
  gogomail_api_request: z
    .object({
      method: apiMethod,
      path: z.string().trim().min(1).max(1024),
      query: z.record(apiQueryValue).optional(),
      body_json: z.unknown().optional(),
      body_text: z.string().optional(),
      body_base64: z.string().max(apiPayloadLimitBytes).optional(),
      content_type: z
        .string()
        .trim()
        .min(1)
        .max(128)
        .regex(/^[^\r\n]+$/)
        .optional(),
      confirm_external_recipients: z.literal("external recipients").optional(),
      confirm_attachments: z.literal("send attachments").optional(),
      confirm,
    })
    .refine(
      (value) =>
        [value.body_json !== undefined, value.body_text !== undefined, value.body_base64 !== undefined].filter(Boolean).length <= 1,
      { message: "provide at most one of body_json, body_text, or body_base64" },
    ),
  gogomail_webmail_get_capabilities: z.object({}),
  gogomail_mailbox_get_overview: z.object({}),
  gogomail_account_get_profile: z.object({}),
  gogomail_account_update_profile: z
    .object({
      display_name: z.string().trim().min(1).max(200).optional(),
      recovery_email: z.string().trim().email().max(320).optional(),
    })
    .refine((value) => value.display_name !== undefined || value.recovery_email !== undefined, {
      message: "display_name or recovery_email is required",
    }),
  gogomail_account_list_addresses: z.object({}),
  gogomail_account_upload_avatar: z.object({
    avatar_base64: z.string().min(1).max(350000),
    mime_type: z.enum(["image/png", "image/jpeg", "image/gif", "image/webp"]),
    filename: z
      .string()
      .trim()
      .min(1)
      .max(255)
      .regex(/^[^\r\n/\\]+$/)
      .default("avatar"),
    confirm,
  }),
  gogomail_account_delete_avatar: z.object({ confirm }),
  gogomail_preferences_get: z.object({}),
};

export async function callTool(
  client: GogomailUserClient,
  name: string,
  args: Record<string, unknown>,
  mode: "basic" | "bypass",
  requireConfirm: (expected: string) => Record<string, string>,
  settings: MCPSettings,
): Promise<unknown> {
  switch (name) {
    case "gogomail_mcp_get_settings":
      return settings;
    case "gogomail_api_request":
      return callGenericAPI(client, args, mode);
    case "gogomail_webmail_get_capabilities":
      return client.request("GET", "/api/v1/webmail/capabilities");
    case "gogomail_mailbox_get_overview":
      return client.request("GET", "/api/v1/mailbox/overview");
    case "gogomail_account_get_profile":
      return client.request("GET", "/api/v1/me");
    case "gogomail_account_update_profile":
      return client.request("PATCH", "/api/v1/me", { display_name: args.display_name, recovery_email: args.recovery_email });
    case "gogomail_account_list_addresses":
      return client.request("GET", "/api/v1/me/addresses");
    case "gogomail_account_upload_avatar":
      return uploadAvatar(client, args, mode);
    case "gogomail_account_delete_avatar":
      return client.request("DELETE", "/api/v1/me/avatar", undefined, requireConfirm("delete avatar"));
    case "gogomail_preferences_get":
      return client.request("GET", "/api/v1/preferences");
    default:
      throw new Error(`account: unhandled tool: ${name}`);
  }
}

function callGenericAPI(client: GogomailUserClient, args: Record<string, unknown>, mode: "basic" | "bypass"): Promise<unknown> {
  const method = String(args.method).toUpperCase();
  const path = normalizeUserAPIPath(String(args.path));
  if (!isAllowedUserAPIPath(method, path)) {
    throw new Error(`path is not allowed for user MCP API bridge: ${path}`);
  }
  const query = (args.query ?? {}) as Record<string, unknown>;
  const finalPath = appendQuery(path, query);
  const headers: Record<string, string> = {};
  const sensitiveConfirm = confirmationForUserAPI(method, path);
  if (mode !== "bypass") {
    const expected = sensitiveConfirm ?? `${method} ${path}`;
    if (method !== "GET" && method !== "HEAD" && args.confirm !== expected) {
      throw new Error(`confirmation required: confirm must equal "${expected}"`);
    }
  }
  if (mode !== "bypass" && sensitiveConfirm) {
    headers["X-Gogomail-MCP-Confirm"] = sensitiveConfirm;
  }
  if (args.confirm_external_recipients) headers["X-Gogomail-MCP-External-Confirm"] = String(args.confirm_external_recipients);
  if (args.confirm_attachments) headers["X-Gogomail-MCP-Attachment-Confirm"] = String(args.confirm_attachments);
  let body: unknown;
  if (args.body_base64 !== undefined) {
    body = Buffer.from(String(args.body_base64), "base64");
    headers["Content-Type"] = String(args.content_type ?? "application/octet-stream");
  } else if (args.body_text !== undefined) {
    body = String(args.body_text);
    if (args.content_type) headers["Content-Type"] = String(args.content_type);
  } else if (args.body_json !== undefined) {
    body = args.body_json;
  }
  return client.request(method, finalPath, body, headers);
}

function normalizeUserAPIPath(raw: string): string {
  if (raw.includes("://") || /[\r\n]/.test(raw)) throw new Error("path must be a relative GoGoMail API path");
  const url = new URL(raw.startsWith("/") ? raw : `/${raw}`, "http://gogomail.local");
  return url.pathname + url.search;
}

function isAllowedUserAPIPath(method: string, path: string): boolean {
  const pathname = path.split("?")[0]!;
  if (pathname.startsWith("/api/v1/auth/") || pathname.startsWith("/admin/")) return false;
  if (pathname.startsWith("/api/v1/me/mcp/access-keys")) return false;
  if (pathname === "/api/v1/me/mcp/settings" && method !== "GET") return false;
  if (pathname === "/api/v1/me/password" || pathname === "/api/v1/auth/sessions/revoke-all") return false;
  if (
    pathname === "/api/v1/webmail/capabilities" ||
    pathname === "/api/v1/mailbox/overview" ||
    pathname === "/api/v1/me/addresses" ||
    pathname === "/api/v1/me/mcp/settings"
  )
    return method === "GET" || method === "HEAD";
  return userAPIRouteManifest.some((route) => (route.method === "*" || route.method === method) && route.pattern.test(pathname));
}

type UserAPIRoute = { method: string; pattern: RegExp };

const safeID = "[^/]+";
const userAPIRouteManifest: UserAPIRoute[] = [
  { method: "GET", pattern: /^\/api\/v1\/search$/ },
  { method: "GET", pattern: /^\/api\/v1\/folders$/ },
  { method: "POST", pattern: /^\/api\/v1\/folders$/ },
  { method: "PATCH", pattern: new RegExp(`^/api/v1/folders/${safeID}$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/folders/${safeID}$`) },
  { method: "GET", pattern: /^\/api\/v1\/messages$/ },
  { method: "POST", pattern: /^\/api\/v1\/messages\/send$/ },
  { method: "POST", pattern: /^\/api\/v1\/messages\/bulk\/delete$/ },
  { method: "POST", pattern: /^\/api\/v1\/messages\/bulk\/restore$/ },
  { method: "PATCH", pattern: /^\/api\/v1\/messages\/bulk\/flags$/ },
  { method: "PATCH", pattern: /^\/api\/v1\/messages\/bulk\/folder$/ },
  { method: "GET", pattern: new RegExp(`^/api/v1/messages/${safeID}$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/messages/${safeID}$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/messages/${safeID}/restore$`) },
  { method: "PATCH", pattern: new RegExp(`^/api/v1/messages/${safeID}/flags$`) },
  { method: "PATCH", pattern: new RegExp(`^/api/v1/messages/${safeID}/folder$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/messages/${safeID}/delivery-status$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/messages/${safeID}/tracking$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/messages/${safeID}/attachments$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/messages/${safeID}/attachments/${safeID}/download$`) },
  { method: "GET", pattern: /^\/api\/v1\/drafts$/ },
  { method: "POST", pattern: /^\/api\/v1\/drafts$/ },
  { method: "GET", pattern: /^\/api\/v1\/drafts\/search$/ },
  { method: "PATCH", pattern: new RegExp(`^/api/v1/drafts/${safeID}$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/drafts/${safeID}$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/drafts/${safeID}/send$`) },
  { method: "GET", pattern: /^\/api\/v1\/threads$/ },
  { method: "GET", pattern: new RegExp(`^/api/v1/threads/${safeID}/messages$`) },
  { method: "POST", pattern: /^\/api\/v1\/threads\/bulk\/delete$/ },
  { method: "POST", pattern: /^\/api\/v1\/threads\/bulk\/restore$/ },
  { method: "PATCH", pattern: /^\/api\/v1\/threads\/bulk\/flags$/ },
  { method: "PATCH", pattern: /^\/api\/v1\/threads\/bulk\/folder$/ },
  { method: "GET", pattern: /^\/api\/v1\/attachments\/capabilities$/ },
  { method: "POST", pattern: /^\/api\/v1\/attachments\/upload-sessions$/ },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/attachments/${safeID}$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/attachments/upload-sessions/${safeID}$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/attachments/upload-sessions/${safeID}$`) },
  { method: "PUT", pattern: new RegExp(`^/api/v1/attachments/upload-sessions/${safeID}/body$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/attachments/upload-sessions/${safeID}/finalize$`) },
  { method: "GET", pattern: /^\/api\/v1\/drive\/nodes$/ },
  { method: "POST", pattern: /^\/api\/v1\/drive\/folders$/ },
  { method: "GET", pattern: /^\/api\/v1\/drive\/usage$/ },
  { method: "GET", pattern: /^\/api\/v1\/drive\/share-links$/ },
  { method: "GET", pattern: new RegExp(`^/api/v1/drive/nodes/${safeID}$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/drive/nodes/${safeID}/download$`) },
  { method: "HEAD", pattern: new RegExp(`^/api/v1/drive/nodes/${safeID}/download$`) },
  { method: "PATCH", pattern: new RegExp(`^/api/v1/drive/nodes/${safeID}/name$`) },
  { method: "PATCH", pattern: new RegExp(`^/api/v1/drive/nodes/${safeID}/parent$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/drive/nodes/${safeID}/copy$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/drive/nodes/${safeID}/trash$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/drive/nodes/${safeID}/restore$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/drive/nodes/${safeID}$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/drive/nodes/${safeID}/share-links$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/drive/share-links/${safeID}$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/drive/share-links/${safeID}$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/drive/share-links/${safeID}/download$`) },
  { method: "HEAD", pattern: new RegExp(`^/api/v1/drive/share-links/${safeID}/download$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/drive/share-links/${safeID}/download$`) },
  { method: "POST", pattern: /^\/api\/v1\/drive\/upload-sessions$/ },
  { method: "GET", pattern: /^\/api\/v1\/drive\/upload-sessions$/ },
  { method: "GET", pattern: new RegExp(`^/api/v1/drive/upload-sessions/${safeID}$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/drive/upload-sessions/${safeID}$`) },
  { method: "PUT", pattern: new RegExp(`^/api/v1/drive/upload-sessions/${safeID}/body$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/drive/upload-sessions/${safeID}/finalize$`) },
  { method: "GET", pattern: /^\/api\/v1\/calendars$/ },
  { method: "POST", pattern: /^\/api\/v1\/calendars$/ },
  { method: "GET", pattern: new RegExp(`^/api/v1/calendars/${safeID}$`) },
  { method: "PATCH", pattern: new RegExp(`^/api/v1/calendars/${safeID}$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/calendars/${safeID}$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/calendars/${safeID}/objects$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/calendars/${safeID}/objects/${safeID}$`) },
  { method: "PUT", pattern: new RegExp(`^/api/v1/calendars/${safeID}/objects/${safeID}$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/calendars/${safeID}/objects/${safeID}$`) },
  { method: "GET", pattern: /^\/api\/v1\/calendar-subscriptions$/ },
  { method: "POST", pattern: /^\/api\/v1\/calendar-subscriptions$/ },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/calendar-subscriptions/${safeID}$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/calendar-subscriptions/${safeID}/events$`) },
  { method: "GET", pattern: /^\/api\/v1\/preferences$/ },
  { method: "PUT", pattern: /^\/api\/v1\/preferences$/ },
  { method: "GET", pattern: /^\/api\/v1\/me$/ },
  { method: "PATCH", pattern: /^\/api\/v1\/me$/ },
  { method: "PUT", pattern: /^\/api\/v1\/me\/avatar$/ },
  { method: "DELETE", pattern: /^\/api\/v1\/me\/avatar$/ },
  { method: "GET", pattern: /^\/api\/v1\/me\/notification-preferences$/ },
  { method: "PUT", pattern: /^\/api\/v1\/me\/notification-preferences$/ },
  { method: "GET", pattern: /^\/api\/v1\/config\/web-push$/ },
  { method: "GET", pattern: /^\/api\/v1\/me\/push-subscriptions$/ },
  { method: "POST", pattern: /^\/api\/v1\/me\/push-subscriptions$/ },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/me/push-subscriptions/${safeID}$`) },
  { method: "GET", pattern: /^\/api\/v1\/push-devices$/ },
  { method: "POST", pattern: /^\/api\/v1\/push-devices$/ },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/push-devices/${safeID}$`) },
  { method: "GET", pattern: /^\/api\/v1\/dm\/rooms$/ },
  { method: "POST", pattern: /^\/api\/v1\/dm\/rooms$/ },
  { method: "GET", pattern: /^\/api\/v1\/dm\/rooms\/public$/ },
  { method: "POST", pattern: new RegExp(`^/api/v1/dm/rooms/${safeID}/members$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/dm/rooms/${safeID}/members/${safeID}$`) },
  { method: "PATCH", pattern: new RegExp(`^/api/v1/dm/rooms/${safeID}/owner$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/dm/rooms/${safeID}/invites$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/dm/join/${safeID}$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/dm/rooms/${safeID}/messages$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/dm/rooms/${safeID}/messages$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/dm/rooms/${safeID}/attachments$`) },
  { method: "POST", pattern: new RegExp(`^/api/v1/dm/rooms/${safeID}/read$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/dm/rooms/${safeID}/search$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/dm/rooms/${safeID}/media$`) },
  { method: "GET", pattern: new RegExp(`^/api/v1/dm/messages/${safeID}/attachment$`) },
  { method: "PATCH", pattern: new RegExp(`^/api/v1/dm/messages/${safeID}$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/v1/dm/messages/${safeID}$`) },
  { method: "PUT", pattern: new RegExp(`^/api/v1/dm/messages/${safeID}/reactions$`) },
  { method: "PUT", pattern: new RegExp(`^/api/v1/dm/messages/${safeID}/reactions/${safeID}$`) },
  { method: "GET", pattern: /^\/api\/mail\/addressbooks$/ },
  { method: "POST", pattern: /^\/api\/mail\/addressbooks$/ },
  { method: "GET", pattern: new RegExp(`^/api/mail/addressbooks/${safeID}$`) },
  { method: "PATCH", pattern: new RegExp(`^/api/mail/addressbooks/${safeID}$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/mail/addressbooks/${safeID}$`) },
  { method: "GET", pattern: new RegExp(`^/api/mail/addressbooks/${safeID}/contacts$`) },
  { method: "GET", pattern: new RegExp(`^/api/mail/addressbooks/${safeID}/contacts/${safeID}$`) },
  { method: "PUT", pattern: new RegExp(`^/api/mail/addressbooks/${safeID}/contacts/${safeID}$`) },
  { method: "DELETE", pattern: new RegExp(`^/api/mail/addressbooks/${safeID}/contacts/${safeID}$`) },
  { method: "GET", pattern: /^\/api\/mail\/contacts\/autocomplete$/ },
  { method: "GET", pattern: /^\/api\/mail\/directory\/users$/ },
  { method: "GET", pattern: /^\/api\/mail\/directory\/org-tree$/ },
  { method: "GET", pattern: /^\/api\/mail\/directory\/profile$/ },
];

function confirmationForUserAPI(method: string, path: string): string | undefined {
  const pathname = path.split("?")[0]!;
  const segment = (index: number) => decodeURIComponent(pathname.split("/")[index] ?? "");
  if (method === "POST" && pathname === "/api/v1/messages/send") return "send message";
  if (method === "POST" && /^\/api\/v1\/drafts\/[^/]+\/send$/.test(pathname)) return `send draft ${segment(4)}`;
  if (method === "DELETE" && /^\/api\/v1\/drafts\/[^/]+$/.test(pathname)) return `delete draft ${segment(4)}`;
  if (method === "DELETE" && /^\/api\/v1\/messages\/[^/]+$/.test(pathname)) return `delete message ${segment(4)}`;
  if (method === "POST" && pathname === "/api/v1/messages/bulk/delete") return "POST /api/v1/messages/bulk/delete";
  if (method === "POST" && pathname === "/api/v1/threads/bulk/delete") return "POST /api/v1/threads/bulk/delete";
  if (method === "DELETE" && /^\/api\/v1\/attachments\/[^/]+$/.test(pathname)) return `DELETE ${pathname}`;
  if (method === "DELETE" && /^\/api\/mail\/addressbooks\/[^/]+\/contacts\/[^/]+$/.test(pathname)) return `delete contact ${segment(6)}`;
  if (method === "DELETE" && /^\/api\/mail\/addressbooks\/[^/]+$/.test(pathname)) return `delete addressbook ${segment(4)}`;
  if (method === "POST" && /^\/api\/v1\/drive\/nodes\/[^/]+\/trash$/.test(pathname)) return `trash drive ${segment(5)}`;
  if (method === "DELETE" && /^\/api\/v1\/drive\/nodes\/[^/]+$/.test(pathname)) return `delete drive ${segment(5)}`;
  if (method === "POST" && /^\/api\/v1\/drive\/nodes\/[^/]+\/share-links$/.test(pathname)) return `share drive ${segment(5)}`;
  if (method === "DELETE" && /^\/api\/v1\/drive\/share-links\/[^/]+$/.test(pathname)) return `DELETE ${pathname}`;
  if (method === "DELETE" && /^\/api\/v1\/calendars\/[^/]+\/objects\/[^/]+$/.test(pathname)) return `delete calendar ${segment(6)}`;
  if (method === "DELETE" && /^\/api\/v1\/calendars\/[^/]+$/.test(pathname)) return `delete calendar ${segment(4)}`;
  if (method === "DELETE" && /^\/api\/v1\/calendar-subscriptions\/[^/]+$/.test(pathname)) return `DELETE ${pathname}`;
  if (method === "DELETE" && pathname === "/api/v1/me/avatar") return "delete avatar";
  if (method === "DELETE" && /^\/api\/v1\/me\/push-subscriptions\/[^/]+$/.test(pathname))
    return `delete web push subscription ${segment(5)}`;
  if (method === "DELETE" && /^\/api\/v1\/push-devices\/[^/]+$/.test(pathname)) return `delete push device ${segment(4)}`;
  if (method === "POST" && pathname === "/api/v1/dm/rooms") return "create dm room";
  if (method === "POST" && /^\/api\/v1\/dm\/rooms\/[^/]+\/members$/.test(pathname)) return `add dm members ${segment(5)}`;
  if (method === "DELETE" && /^\/api\/v1\/dm\/rooms\/[^/]+\/members\/[^/]+$/.test(pathname))
    return `remove dm member ${segment(5)} ${segment(7)}`;
  if (method === "PATCH" && /^\/api\/v1\/dm\/rooms\/[^/]+\/owner$/.test(pathname)) return `transfer dm owner ${segment(5)}`;
  if (method === "POST" && /^\/api\/v1\/dm\/rooms\/[^/]+\/invites$/.test(pathname)) return `create dm invite ${segment(5)}`;
  if (method === "POST" && /^\/api\/v1\/dm\/join\/[^/]+$/.test(pathname)) return `join dm invite ${segment(5)}`;
  if (method === "POST" && /^\/api\/v1\/dm\/rooms\/[^/]+\/messages$/.test(pathname)) return `send dm message ${segment(5)}`;
  if (method === "POST" && /^\/api\/v1\/dm\/rooms\/[^/]+\/attachments$/.test(pathname)) return `send dm attachment ${segment(5)}`;
  if (method === "PATCH" && /^\/api\/v1\/dm\/messages\/[^/]+$/.test(pathname)) return `edit dm message ${segment(5)}`;
  if (method === "DELETE" && /^\/api\/v1\/dm\/messages\/[^/]+$/.test(pathname)) return `delete dm message ${segment(5)}`;
  return undefined;
}

async function uploadAvatar(client: GogomailUserClient, args: Record<string, unknown>, mode: "basic" | "bypass"): Promise<unknown> {
  if (mode !== "bypass" && args.confirm !== "upload avatar") {
    throw new Error('confirmation required: confirm must equal "upload avatar"');
  }
  const bytes = Buffer.from(String(args.avatar_base64), "base64");
  if (bytes.length === 0 || bytes.length > avatarPayloadLimitBytes) {
    throw new Error("avatar must decode to 1..262144 bytes");
  }
  const boundary = `gogomail-mcp-${randomUUID()}`;
  const filename = String(args.filename ?? "avatar").replace(/"/g, "");
  const mimeType = String(args.mime_type);
  const prefix = Buffer.from(
    `--${boundary}\r\nContent-Disposition: form-data; name="avatar"; filename="${filename}"\r\nContent-Type: ${mimeType}\r\n\r\n`,
    "utf8",
  );
  const suffix = Buffer.from(`\r\n--${boundary}--\r\n`, "utf8");
  return client.request("PUT", "/api/v1/me/avatar", Buffer.concat([prefix, bytes, suffix]), {
    "Content-Type": `multipart/form-data; boundary=${boundary}`,
  });
}
