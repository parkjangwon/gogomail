import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { randomUUID } from "node:crypto";
import { z } from "zod";
import { appendQuery, GogomailUserClient } from "../client.js";
import { confirm, id } from "./schemas.js";
import { sanitizeFileSegment } from "./schemas.js";

export const toolDefinitions: Tool[] = [
  { name: "gogomail_calendar_list", description: "List calendars using GET /api/v1/calendars.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_calendar_create", description: "Create a calendar using POST /api/v1/calendars.", inputSchema: { type: "object", properties: { name: { type: "string" }, description: { type: "string" }, color: { type: "string" } }, required: ["name"] } },
  { name: "gogomail_calendar_get", description: "Get a calendar using GET /api/v1/calendars/{id}.", inputSchema: { type: "object", properties: { calendar_id: { type: "string", maxLength: 200 } }, required: ["calendar_id"] } },
  { name: "gogomail_calendar_update", description: "Update a calendar using PATCH /api/v1/calendars/{id}.", inputSchema: { type: "object", properties: { calendar_id: { type: "string", maxLength: 200 }, name: { type: "string" }, description: { type: "string" }, color: { type: "string" } }, required: ["calendar_id"] } },
  { name: "gogomail_calendar_delete", description: "Delete a calendar using DELETE /api/v1/calendars/{id}. In basic mode confirm must equal `delete calendar <id>`.", inputSchema: { type: "object", properties: { calendar_id: { type: "string", maxLength: 200 }, confirm: { type: "string" } }, required: ["calendar_id"] } },
  { name: "gogomail_calendar_list_objects", description: "List calendar objects using GET /api/v1/calendars/{id}/objects.", inputSchema: { type: "object", properties: { calendar_id: { type: "string", maxLength: 200 } }, required: ["calendar_id"] } },
  { name: "gogomail_calendar_get_object", description: "Get a calendar object using GET /api/v1/calendars/{id}/objects/{name}.", inputSchema: { type: "object", properties: { calendar_id: { type: "string", maxLength: 200 }, object_name: { type: "string", maxLength: 200 } }, required: ["calendar_id", "object_name"] } },
  { name: "gogomail_calendar_upsert_object", description: "Create or update an iCalendar object using PUT /api/v1/calendars/{id}/objects/{name}.", inputSchema: { type: "object", properties: { calendar_id: { type: "string", maxLength: 200 }, object_name: { type: "string", maxLength: 200 }, ics: { type: "string" } }, required: ["calendar_id", "object_name", "ics"] } },
  { name: "gogomail_calendar_upsert_event_simple", description: "Create or update a calendar event without hand-writing ICS. Times should be ISO-8601 strings; object_name defaults to a generated .ics file.", inputSchema: { type: "object", properties: { calendar_id: { type: "string", maxLength: 200 }, object_name: { type: "string", maxLength: 200 }, uid: { type: "string", maxLength: 200 }, summary: { type: "string" }, starts_at: { type: "string" }, ends_at: { type: "string" }, description: { type: "string" }, location: { type: "string" }, all_day: { type: "boolean" } }, required: ["calendar_id", "summary", "starts_at", "ends_at"] } },
  { name: "gogomail_calendar_delete_object", description: "Delete a calendar object. In basic mode confirm must equal `delete calendar <object_name>`.", inputSchema: { type: "object", properties: { calendar_id: { type: "string", maxLength: 200 }, object_name: { type: "string", maxLength: 200 }, confirm: { type: "string" } }, required: ["calendar_id", "object_name"] } },
  { name: "gogomail_calendar_list_subscriptions", description: "List calendar subscriptions using GET /api/v1/calendar-subscriptions.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_calendar_create_subscription", description: "Create a calendar subscription using POST /api/v1/calendar-subscriptions.", inputSchema: { type: "object", properties: { name: { type: "string" }, url: { type: "string" }, color: { type: "string" } }, required: ["name", "url"] } },
  { name: "gogomail_calendar_delete_subscription", description: "Delete a calendar subscription using DELETE /api/v1/calendar-subscriptions/{id}. In basic mode confirm must equal `DELETE /api/v1/calendar-subscriptions/<id>`.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, confirm: { type: "string" } }, required: ["id"] } },
  { name: "gogomail_calendar_get_subscription_events", description: "Read subscription events using GET /api/v1/calendar-subscriptions/{id}/events.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, since: { type: "string" }, until: { type: "string" } }, required: ["id"] } },
];

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_calendar_list: z.object({}),
  gogomail_calendar_create: z.object({
    name: z.string().min(1).max(200),
    display_name: z.string().max(200).optional(),
    description: z.string().max(1000).optional(),
    color: z.string().max(32).optional(),
  }),
  gogomail_calendar_get: z.object({ calendar_id: id }),
  gogomail_calendar_update: z.object({
    calendar_id: id,
    name: z.string().max(200).optional(),
    display_name: z.string().max(200).optional(),
    description: z.string().max(1000).optional(),
    color: z.string().max(32).optional(),
  }),
  gogomail_calendar_delete: z.object({ calendar_id: id, confirm }),
  gogomail_calendar_list_objects: z.object({ calendar_id: id }),
  gogomail_calendar_get_object: z.object({ calendar_id: id, object_name: id }),
  gogomail_calendar_upsert_object: z.object({ calendar_id: id, object_name: id, ics: z.string().min(1) }),
  gogomail_calendar_upsert_event_simple: z.object({
    calendar_id: id,
    object_name: id.optional(),
    uid: id.optional(),
    summary: z.string().min(1).max(255),
    starts_at: z.string().min(1).max(64),
    ends_at: z.string().min(1).max(64),
    description: z.string().max(2000).optional(),
    location: z.string().max(500).optional(),
    all_day: z.boolean().default(false),
  }),
  gogomail_calendar_delete_object: z.object({ calendar_id: id, object_name: id, confirm }),
  gogomail_calendar_list_subscriptions: z.object({}),
  gogomail_calendar_create_subscription: z.object({
    name: z.string().min(1).max(200),
    url: z.string().url().max(2048),
    color: z.string().max(32).optional(),
  }),
  gogomail_calendar_delete_subscription: z.object({ id, confirm }),
  gogomail_calendar_get_subscription_events: z.object({ id, since: z.string().optional(), until: z.string().optional() }),
};

export async function callTool(
  client: GogomailUserClient,
  name: string,
  args: Record<string, unknown>,
  _mode: "basic" | "bypass",
  requireConfirm: (expected: string) => Record<string, string>,
): Promise<unknown> {
  switch (name) {
    case "gogomail_calendar_list":
      return client.request("GET", "/api/v1/calendars");
    case "gogomail_calendar_create":
      return client.request("POST", "/api/v1/calendars", { name: args.name, description: args.description, color: args.color });
    case "gogomail_calendar_get":
      return client.request("GET", `/api/v1/calendars/${encodeURIComponent(String(args.calendar_id))}`);
    case "gogomail_calendar_update":
      return client.request("PATCH", `/api/v1/calendars/${encodeURIComponent(String(args.calendar_id))}`, {
        name: args.name ?? args.display_name,
        description: args.description,
        color: args.color,
      });
    case "gogomail_calendar_delete":
      return client.request(
        "DELETE",
        `/api/v1/calendars/${encodeURIComponent(String(args.calendar_id))}`,
        undefined,
        requireConfirm(`delete calendar ${args.calendar_id}`),
      );
    case "gogomail_calendar_list_objects":
      return client.request("GET", `/api/v1/calendars/${encodeURIComponent(String(args.calendar_id))}/objects`);
    case "gogomail_calendar_get_object":
      return client.request(
        "GET",
        `/api/v1/calendars/${encodeURIComponent(String(args.calendar_id))}/objects/${encodeURIComponent(String(args.object_name))}`,
      );
    case "gogomail_calendar_upsert_object":
      return client.request(
        "PUT",
        `/api/v1/calendars/${encodeURIComponent(String(args.calendar_id))}/objects/${encodeURIComponent(String(args.object_name))}`,
        String(args.ics),
        { "Content-Type": "text/calendar" },
      );
    case "gogomail_calendar_upsert_event_simple": {
      const event = buildSimpleICS(args);
      return client.request(
        "PUT",
        `/api/v1/calendars/${encodeURIComponent(String(args.calendar_id))}/objects/${encodeURIComponent(event.objectName)}`,
        event.ics,
        { "Content-Type": "text/calendar" },
      );
    }
    case "gogomail_calendar_delete_object":
      return client.request(
        "DELETE",
        `/api/v1/calendars/${encodeURIComponent(String(args.calendar_id))}/objects/${encodeURIComponent(String(args.object_name))}`,
        undefined,
        requireConfirm(`delete calendar ${args.object_name}`),
      );
    case "gogomail_calendar_list_subscriptions":
      return client.request("GET", "/api/v1/calendar-subscriptions");
    case "gogomail_calendar_create_subscription":
      return client.request("POST", "/api/v1/calendar-subscriptions", { name: args.name, url: args.url, color: args.color });
    case "gogomail_calendar_delete_subscription":
      return client.request(
        "DELETE",
        `/api/v1/calendar-subscriptions/${encodeURIComponent(String(args.id))}`,
        undefined,
        requireConfirm(`DELETE /api/v1/calendar-subscriptions/${args.id}`),
      );
    case "gogomail_calendar_get_subscription_events":
      return client.request(
        "GET",
        appendQuery(`/api/v1/calendar-subscriptions/${encodeURIComponent(String(args.id))}/events`, { since: args.since, until: args.until }),
      );
    default:
      throw new Error(`calendar: unhandled tool: ${name}`);
  }
}

function buildSimpleICS(args: Record<string, unknown>): { objectName: string; ics: string } {
  const uid = String(args.uid ?? `${randomUUID()}@gogomail-mcp.local`);
  const objectName = String(args.object_name ?? `${sanitizeFileSegment(uid)}.ics`);
  const allDay = Boolean(args.all_day);
  const dtstamp = formatICSDateTime(new Date().toISOString());
  const startLine = allDay ? `DTSTART;VALUE=DATE:${formatICSDate(String(args.starts_at))}` : `DTSTART:${formatICSDateTime(String(args.starts_at))}`;
  const endLine = allDay ? `DTEND;VALUE=DATE:${formatICSDate(String(args.ends_at))}` : `DTEND:${formatICSDateTime(String(args.ends_at))}`;
  const lines = [
    "BEGIN:VCALENDAR",
    "VERSION:2.0",
    "PRODID:-//GoGoMail//User MCP//EN",
    "BEGIN:VEVENT",
    `UID:${escapeICSText(uid)}`,
    `DTSTAMP:${dtstamp}`,
    startLine,
    endLine,
    `SUMMARY:${escapeICSText(String(args.summary))}`,
  ];
  if (args.description) lines.push(`DESCRIPTION:${escapeICSText(String(args.description))}`);
  if (args.location) lines.push(`LOCATION:${escapeICSText(String(args.location))}`);
  lines.push("END:VEVENT", "END:VCALENDAR");
  return { objectName, ics: `${lines.join("\r\n")}\r\n` };
}

function escapeICSText(value: string): string {
  return value.replace(/\\/g, "\\\\").replace(/\r?\n/g, "\\n").replace(/;/g, "\\;").replace(/,/g, "\\,");
}

function formatICSDateTime(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.valueOf())) throw new Error("invalid ISO-8601 date-time");
  return date.toISOString().replace(/[-:]/g, "").replace(/\.\d{3}Z$/, "Z");
}

function formatICSDate(value: string): string {
  const match = /^(\d{4})-(\d{2})-(\d{2})/.exec(value);
  if (!match) throw new Error("all-day dates must start with YYYY-MM-DD");
  return `${match[1]}${match[2]}${match[3]}`;
}
