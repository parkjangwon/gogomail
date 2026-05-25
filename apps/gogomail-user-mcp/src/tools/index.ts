import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import { GogomailUserClient, type MCPSettings } from "../client.js";
import * as accountTools from "./account.js";
import * as calendarTools from "./calendar.js";
import * as contactsTools from "./contacts.js";
import * as dmTools from "./dm.js";
import * as driveTools from "./drive.js";
import * as mailTools from "./mail.js";
import * as notifTools from "./notifications.js";

// Domains that need settings passed through (for withMCPNotice, gogomail_mcp_get_settings)
type DomainWithSettings = {
  toolDefinitions: Tool[];
  schemas: Record<string, z.ZodTypeAny>;
  callTool(
    client: GogomailUserClient,
    name: string,
    args: Record<string, unknown>,
    mode: "basic" | "bypass",
    requireConfirm: (expected: string) => Record<string, string>,
    settings: MCPSettings,
  ): Promise<unknown>;
};

type DomainWithoutSettings = {
  toolDefinitions: Tool[];
  schemas: Record<string, z.ZodTypeAny>;
  callTool(
    client: GogomailUserClient,
    name: string,
    args: Record<string, unknown>,
    mode: "basic" | "bypass",
    requireConfirm: (expected: string) => Record<string, string>,
  ): Promise<unknown>;
};

const settingsDomains: DomainWithSettings[] = [accountTools, mailTools];
const plainDomains: DomainWithoutSettings[] = [notifTools, dmTools, driveTools, calendarTools, contactsTools];

export const toolDefinitions: Tool[] = [
  ...settingsDomains.flatMap((m) => m.toolDefinitions),
  ...plainDomains.flatMap((m) => m.toolDefinitions),
];

const allSchemas: Record<string, z.ZodTypeAny> = Object.assign(
  {},
  ...settingsDomains.map((m) => m.schemas),
  ...plainDomains.map((m) => m.schemas),
);

export async function callTool(
  client: GogomailUserClient,
  name: string,
  rawArgs: Record<string, unknown>,
  envMode: "basic" | "bypass",
): Promise<unknown> {
  const schema = allSchemas[name];
  if (!schema) throw new Error(`Unknown tool: ${name}`);
  const args = schema.parse(rawArgs) as Record<string, unknown>;
  const settings: MCPSettings = await client.settings().catch(() => ({}));
  const mode = settings.permission_mode ?? envMode;
  const requireConfirm = (expected: string): Record<string, string> => {
    if (mode === "bypass") return {};
    if (args.confirm !== expected) throw new Error(`confirmation required: confirm must equal "${expected}"`);
    return { "X-Gogomail-MCP-Confirm": expected };
  };

  for (const domain of settingsDomains) {
    if (name in domain.schemas) {
      return domain.callTool(client, name, args, mode, requireConfirm, settings);
    }
  }
  for (const domain of plainDomains) {
    if (name in domain.schemas) {
      return domain.callTool(client, name, args, mode, requireConfirm);
    }
  }
  throw new Error(`Unhandled tool: ${name}`);
}
