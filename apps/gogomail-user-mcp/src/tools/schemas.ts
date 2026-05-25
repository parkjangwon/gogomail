import { randomUUID } from "node:crypto";
import { z } from "zod";

export const id = z.string().trim().min(1).max(200).regex(/^[^\r\n]+$/);
export const optionalID = id.optional();
export const email = z.string().trim().email().max(320);
export const limit = z.number().int().min(1).max(200).optional();
export const address = z.object({ email, name: z.string().max(200).optional() });
export const confirm = z.string().max(300).optional();
export const storageBackend = z.string().trim().min(1).max(64).regex(/^[^\r\n]+$/);
export const contractName = z.string().min(1).max(200);
export const nameOrLegacyDisplayName = z
  .object({ name: contractName.optional(), display_name: contractName.optional(), description: z.string().max(1000).optional() })
  .refine((value) => value.name || value.display_name, { message: "name is required" });
export const outputPath = z.string().trim().min(1).max(4096).regex(/^[^\r\n]+$/);
export const mailFlag = z.enum(["read", "starred", "answered", "forwarded"]);
export const bulkIDs = z
  .array(id)
  .min(1)
  .max(500)
  .refine((values) => new Set(values).size === values.length, { message: "ids must be unique" });
export const apiMethod = z.enum(["GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"]);
export const apiQueryValue = z.union([z.string(), z.number(), z.boolean()]);
export const apiPayloadLimitBytes = 32 * 1024 * 1024;
export const avatarPayloadLimitBytes = 256 * 1024;
export const dmAttachmentPayloadLimitBytes = 20 * 1024 * 1024;
export const dmRoomType = z.enum(["direct", "group"]);
export const dmVisibility = z.enum(["public", "private"]);
export const dmMediaType = z.enum(["file", "drive_link", "link"]);
export const senderPattern = z
  .string()
  .trim()
  .toLowerCase()
  .min(1)
  .max(320)
  .regex(/^(@[A-Za-z0-9.-]+\.[A-Za-z]{2,}|[^@\s\r\n]+@[^@\s\r\n]+\.[^@\s\r\n]+)$/);
export const senderListKind = z.enum(["blocked", "allowed"]);
export const hhmm = z.string().regex(/^([01][0-9]|2[0-3]):[0-5][0-9]$/);
export const dndScheduleSchema = z.object({
  weekdays: z.array(z.number().int().min(0).max(6)).max(7).default([]),
  time_ranges: z.array(z.object({ start: hhmm, end: hhmm })).max(8).default([]),
  timezone: z.string().trim().min(1).max(128).default("UTC"),
});
export const folderNotificationOverrideSchema = z.object({
  enabled: z.boolean(),
  dnd_inherit: z.boolean(),
  dnd_schedule: dndScheduleSchema,
});
export const threadNotificationOverrideSchema = z.object({ enabled: z.boolean() });

// Shared utility: sanitize a string for use as a filename segment
export function sanitizeFileSegment(value: string): string {
  const cleaned = value
    .trim()
    .replace(/[^A-Za-z0-9._@-]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return cleaned.slice(0, 180) || randomUUID();
}
