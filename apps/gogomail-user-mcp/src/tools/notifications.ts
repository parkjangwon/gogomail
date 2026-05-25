import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import { appendQuery, GogomailUserClient } from "../client.js";
import { confirm, dndScheduleSchema, folderNotificationOverrideSchema, id, limit, threadNotificationOverrideSchema } from "./schemas.js";

export const toolDefinitions: Tool[] = [
  { name: "gogomail_notifications_get_preferences", description: "Read notification preferences using GET /api/v1/me/notification-preferences.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_notifications_update_preferences", description: "Replace notification preferences using PUT /api/v1/me/notification-preferences. Read current preferences first and preserve folder/thread overrides that should remain.", inputSchema: { type: "object", properties: { global_dnd_enabled: { type: "boolean" }, global_dnd_schedule: { type: "object", properties: { weekdays: { type: "array", items: { type: "number", minimum: 0, maximum: 6 }, maxItems: 7 }, time_ranges: { type: "array", items: { type: "object", properties: { start: { type: "string", pattern: "^([01][0-9]|2[0-3]):[0-5][0-9]$" }, end: { type: "string", pattern: "^([01][0-9]|2[0-3]):[0-5][0-9]$" } }, required: ["start", "end"] }, maxItems: 8 }, timezone: { type: "string", maxLength: 128 } }, required: ["weekdays", "time_ranges", "timezone"] }, folder_overrides: { type: "object", additionalProperties: { type: "object", properties: { enabled: { type: "boolean" }, dnd_inherit: { type: "boolean" }, dnd_schedule: { type: "object", properties: { weekdays: { type: "array", items: { type: "number", minimum: 0, maximum: 6 }, maxItems: 7 }, time_ranges: { type: "array", items: { type: "object", properties: { start: { type: "string", pattern: "^([01][0-9]|2[0-3]):[0-5][0-9]$" }, end: { type: "string", pattern: "^([01][0-9]|2[0-3]):[0-5][0-9]$" } }, required: ["start", "end"] }, maxItems: 8 }, timezone: { type: "string", maxLength: 128 } }, required: ["weekdays", "time_ranges", "timezone"] } }, required: ["enabled", "dnd_inherit", "dnd_schedule"] } }, thread_overrides: { type: "object", additionalProperties: { type: "object", properties: { enabled: { type: "boolean" } }, required: ["enabled"] } } }, required: ["global_dnd_enabled", "global_dnd_schedule", "folder_overrides"] } },
  { name: "gogomail_notifications_get_web_push_config", description: "Read Web Push VAPID public-key configuration using GET /api/v1/config/web-push.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_notifications_list_push_subscriptions", description: "List active Web Push browser subscriptions using GET /api/v1/me/push-subscriptions.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_notifications_upsert_push_subscription", description: "Register or refresh a Web Push browser subscription using POST /api/v1/me/push-subscriptions.", inputSchema: { type: "object", properties: { endpoint: { type: "string", format: "uri", maxLength: 2048 }, p256dh: { type: "string", maxLength: 4096 }, auth: { type: "string", maxLength: 4096 }, user_agent: { type: "string", maxLength: 512 } }, required: ["endpoint", "p256dh", "auth"] } },
  { name: "gogomail_notifications_delete_push_subscription", description: "Delete a Web Push browser subscription using DELETE /api/v1/me/push-subscriptions/{id}. In basic mode confirm must equal `delete web push subscription <id>`.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, confirm: { type: "string", maxLength: 300 } }, required: ["id"] } },
  { name: "gogomail_notifications_list_push_devices", description: "List user push notification devices using GET /api/v1/push-devices.", inputSchema: { type: "object", properties: { limit: { type: "number", minimum: 1, maximum: 200 } } } },
  { name: "gogomail_notifications_upsert_push_device", description: "Register or refresh a push notification device using POST /api/v1/push-devices.", inputSchema: { type: "object", properties: { platform: { type: "string", enum: ["apns", "fcm", "webpush"] }, token: { type: "string", minLength: 1, maxLength: 4096 }, label: { type: "string", maxLength: 200 } }, required: ["platform", "token"] } },
  { name: "gogomail_notifications_delete_push_device", description: "Delete a push notification device using DELETE /api/v1/push-devices/{id}. In basic mode confirm must equal `delete push device <id>`.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, confirm: { type: "string", maxLength: 300 } }, required: ["id"] } },
];

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_notifications_get_preferences: z.object({}),
  gogomail_notifications_update_preferences: z.object({
    global_dnd_enabled: z.boolean(),
    global_dnd_schedule: dndScheduleSchema,
    folder_overrides: z.record(folderNotificationOverrideSchema),
    thread_overrides: z.record(threadNotificationOverrideSchema).default({}),
  }),
  gogomail_notifications_get_web_push_config: z.object({}),
  gogomail_notifications_list_push_subscriptions: z.object({}),
  gogomail_notifications_upsert_push_subscription: z.object({
    endpoint: z.string().url().max(2048),
    p256dh: z.string().min(1).max(4096),
    auth: z.string().min(1).max(4096),
    user_agent: z.string().max(512).optional(),
  }),
  gogomail_notifications_delete_push_subscription: z.object({ id, confirm }),
  gogomail_notifications_list_push_devices: z.object({ limit }),
  gogomail_notifications_upsert_push_device: z.object({
    platform: z.enum(["apns", "fcm", "webpush"]),
    token: z.string().min(1).max(4096),
    label: z.string().max(200).optional(),
  }),
  gogomail_notifications_delete_push_device: z.object({ id, confirm }),
};

export async function callTool(
  client: GogomailUserClient,
  name: string,
  args: Record<string, unknown>,
  _mode: "basic" | "bypass",
  requireConfirm: (expected: string) => Record<string, string>,
): Promise<unknown> {
  switch (name) {
    case "gogomail_notifications_get_preferences":
      return client.request("GET", "/api/v1/me/notification-preferences");
    case "gogomail_notifications_update_preferences":
      return client.request("PUT", "/api/v1/me/notification-preferences", {
        global_dnd_enabled: args.global_dnd_enabled,
        global_dnd_schedule: args.global_dnd_schedule,
        folder_overrides: args.folder_overrides,
        thread_overrides: args.thread_overrides,
      });
    case "gogomail_notifications_get_web_push_config":
      return client.request("GET", "/api/v1/config/web-push");
    case "gogomail_notifications_list_push_subscriptions":
      return client.request("GET", "/api/v1/me/push-subscriptions");
    case "gogomail_notifications_upsert_push_subscription":
      return client.request("POST", "/api/v1/me/push-subscriptions", {
        endpoint: args.endpoint,
        p256dh: args.p256dh,
        auth: args.auth,
        userAgent: args.user_agent,
      });
    case "gogomail_notifications_delete_push_subscription":
      return client.request(
        "DELETE",
        `/api/v1/me/push-subscriptions/${encodeURIComponent(String(args.id))}`,
        undefined,
        requireConfirm(`delete web push subscription ${args.id}`),
      );
    case "gogomail_notifications_list_push_devices":
      return client.request("GET", appendQuery("/api/v1/push-devices", args));
    case "gogomail_notifications_upsert_push_device":
      return client.request("POST", "/api/v1/push-devices", { platform: args.platform, token: args.token, label: args.label });
    case "gogomail_notifications_delete_push_device":
      return client.request(
        "DELETE",
        `/api/v1/push-devices/${encodeURIComponent(String(args.id))}`,
        undefined,
        requireConfirm(`delete push device ${args.id}`),
      );
    default:
      throw new Error(`notifications: unhandled tool: ${name}`);
  }
}
