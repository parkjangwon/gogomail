import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import { GogomailUserClient } from "../client.js";
import { id } from "./schemas.js";

const ruleField = z.enum(["from", "subject", "body", "to", "any"]);
const ruleMatch = z.enum(["contains", "prefix", "suffix", "exact"]);

export const toolDefinitions: Tool[] = [
  {
    name: "gogomail_assistant_get_settings",
    description:
      "Read the AI email assistant settings (opt-in state and provider) using GET /api/v1/me/assistant/settings.",
    inputSchema: { type: "object", properties: {} },
  },
  {
    name: "gogomail_assistant_update_settings",
    description:
      "Update AI email assistant settings using PUT /api/v1/me/assistant/settings. The assistant is opt-in (enabled defaults to false) and gated by the per-domain policy. Provider `local` keeps all content on the server (offline).",
    inputSchema: {
      type: "object",
      properties: {
        enabled: { type: "boolean" },
        summarization_enabled: { type: "boolean" },
        categorization_enabled: { type: "boolean" },
        compose_assist_enabled: { type: "boolean" },
        provider: { type: "string", maxLength: 64 },
      },
      required: ["enabled"],
    },
  },
  {
    name: "gogomail_assistant_summarize_thread",
    description:
      "Summarize a thread with the AI assistant using POST /api/v1/threads/{id}/summary. Returns participants, key points, action items, and a suggested reply draft. Requires assistant opt-in.",
    inputSchema: {
      type: "object",
      properties: { id: { type: "string", maxLength: 200 } },
      required: ["id"],
    },
  },
  {
    name: "gogomail_assistant_compose_assist",
    description:
      "Get smart compose assistance for a draft using POST /api/v1/compose/assist. Returns subject suggestions, tone/length variants, and pre-send checks such as the missing-attachment warning. Pure local heuristics.",
    inputSchema: {
      type: "object",
      properties: {
        subject: { type: "string", maxLength: 2000 },
        body: { type: "string", maxLength: 1000000 },
        recipients: { type: "array", items: { type: "string", maxLength: 320 }, maxItems: 200 },
        attachment_count: { type: "number", minimum: 0, maximum: 1000 },
      },
    },
  },
  {
    name: "gogomail_assistant_list_categorization_rules",
    description:
      "List auto-categorization rules using GET /api/v1/me/assistant/categorization-rules.",
    inputSchema: { type: "object", properties: {} },
  },
  {
    name: "gogomail_assistant_create_categorization_rule",
    description:
      "Create an auto-categorization rule using POST /api/v1/me/assistant/categorization-rules.",
    inputSchema: {
      type: "object",
      properties: {
        category: { type: "string", minLength: 1, maxLength: 200 },
        field: { type: "string", enum: ["from", "subject", "body", "to", "any"] },
        match_type: { type: "string", enum: ["contains", "prefix", "suffix", "exact"] },
        keyword: { type: "string", minLength: 1, maxLength: 500 },
        priority: { type: "number", minimum: 0, maximum: 1000 },
        weight: { type: "number", minimum: 0, maximum: 1000 },
      },
      required: ["category", "keyword"],
    },
  },
  {
    name: "gogomail_assistant_update_categorization_rule",
    description:
      "Update an auto-categorization rule using PUT /api/v1/me/assistant/categorization-rules/{id}.",
    inputSchema: {
      type: "object",
      properties: {
        id: { type: "string", maxLength: 200 },
        category: { type: "string", minLength: 1, maxLength: 200 },
        field: { type: "string", enum: ["from", "subject", "body", "to", "any"] },
        match_type: { type: "string", enum: ["contains", "prefix", "suffix", "exact"] },
        keyword: { type: "string", minLength: 1, maxLength: 500 },
        priority: { type: "number", minimum: 0, maximum: 1000 },
        weight: { type: "number", minimum: 0, maximum: 1000 },
      },
      required: ["id", "category", "keyword"],
    },
  },
  {
    name: "gogomail_assistant_delete_categorization_rule",
    description:
      "Delete an auto-categorization rule using DELETE /api/v1/me/assistant/categorization-rules/{id}.",
    inputSchema: {
      type: "object",
      properties: { id: { type: "string", maxLength: 200 } },
      required: ["id"],
    },
  },
];

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_assistant_get_settings: z.object({}),
  gogomail_assistant_update_settings: z.object({
    enabled: z.boolean(),
    summarization_enabled: z.boolean().optional(),
    categorization_enabled: z.boolean().optional(),
    compose_assist_enabled: z.boolean().optional(),
    provider: z.string().trim().max(64).optional(),
  }),
  gogomail_assistant_summarize_thread: z.object({ id }),
  gogomail_assistant_compose_assist: z.object({
    subject: z.string().max(2000).optional(),
    body: z.string().max(1000000).optional(),
    recipients: z.array(z.string().max(320)).max(200).optional(),
    attachment_count: z.number().int().min(0).max(1000).optional(),
  }),
  gogomail_assistant_list_categorization_rules: z.object({}),
  gogomail_assistant_create_categorization_rule: z.object({
    category: z.string().trim().min(1).max(200),
    field: ruleField.optional(),
    match_type: ruleMatch.optional(),
    keyword: z.string().trim().min(1).max(500),
    priority: z.number().int().min(0).max(1000).optional(),
    weight: z.number().min(0).max(1000).optional(),
  }),
  gogomail_assistant_update_categorization_rule: z.object({
    id,
    category: z.string().trim().min(1).max(200),
    field: ruleField.optional(),
    match_type: ruleMatch.optional(),
    keyword: z.string().trim().min(1).max(500),
    priority: z.number().int().min(0).max(1000).optional(),
    weight: z.number().min(0).max(1000).optional(),
  }),
  gogomail_assistant_delete_categorization_rule: z.object({ id }),
};

function ruleBody(args: Record<string, unknown>): Record<string, unknown> {
  return {
    category: args.category,
    field: args.field,
    match_type: args.match_type,
    keyword: args.keyword,
    priority: args.priority,
    weight: args.weight,
  };
}

export async function callTool(
  client: GogomailUserClient,
  name: string,
  args: Record<string, unknown>,
  _mode: "basic" | "bypass",
  _requireConfirm: (expected: string) => Record<string, string>,
): Promise<unknown> {
  switch (name) {
    case "gogomail_assistant_get_settings":
      return client.request("GET", "/api/v1/me/assistant/settings");
    case "gogomail_assistant_update_settings":
      return client.request("PUT", "/api/v1/me/assistant/settings", {
        enabled: args.enabled,
        summarization_enabled: args.summarization_enabled ?? true,
        categorization_enabled: args.categorization_enabled ?? true,
        compose_assist_enabled: args.compose_assist_enabled ?? true,
        provider: args.provider ?? "local",
      });
    case "gogomail_assistant_summarize_thread":
      return client.request(
        "POST",
        `/api/v1/threads/${encodeURIComponent(String(args.id))}/summary`,
      );
    case "gogomail_assistant_compose_assist":
      return client.request("POST", "/api/v1/compose/assist", {
        subject: args.subject ?? "",
        body: args.body ?? "",
        recipients: args.recipients ?? [],
        attachment_count: args.attachment_count ?? 0,
      });
    case "gogomail_assistant_list_categorization_rules":
      return client.request("GET", "/api/v1/me/assistant/categorization-rules");
    case "gogomail_assistant_create_categorization_rule":
      return client.request("POST", "/api/v1/me/assistant/categorization-rules", ruleBody(args));
    case "gogomail_assistant_update_categorization_rule":
      return client.request(
        "PUT",
        `/api/v1/me/assistant/categorization-rules/${encodeURIComponent(String(args.id))}`,
        ruleBody(args),
      );
    case "gogomail_assistant_delete_categorization_rule":
      return client.request(
        "DELETE",
        `/api/v1/me/assistant/categorization-rules/${encodeURIComponent(String(args.id))}`,
      );
    default:
      throw new Error(`assistant: unhandled tool: ${name}`);
  }
}
