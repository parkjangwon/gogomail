/**
 * Combiner: imports all domain modules and re-exports a unified toolDefinitions + callTool.
 */

import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import type { GogomailClient } from "../clients/gogomail.js";
import { type OptionalSuppo } from "./shared.js";

import * as usersTools from "./users.js";
import * as companyTools from "./company.js";
import * as mailOpsTools from "./mail-ops.js";
import * as securityTools from "./security.js";
import * as orgTools from "./org.js";
import * as systemTools from "./system.js";

export type { OptionalSuppo };

export const toolDefinitions: Tool[] = [
  ...usersTools.toolDefinitions,
  ...companyTools.toolDefinitions,
  ...mailOpsTools.toolDefinitions,
  ...securityTools.toolDefinitions,
  ...orgTools.toolDefinitions,
  ...systemTools.toolDefinitions,
];

export async function callTool(
  gogomail: GogomailClient,
  suppo: OptionalSuppo,
  name: string,
  args: Record<string, unknown>,
): Promise<unknown> {
  if (name in usersTools.schemas) return usersTools.callTool(gogomail, suppo, name, args);
  if (name in companyTools.schemas) return companyTools.callTool(gogomail, suppo, name, args);
  if (name in mailOpsTools.schemas) return mailOpsTools.callTool(gogomail, suppo, name, args);
  if (name in securityTools.schemas) return securityTools.callTool(gogomail, suppo, name, args);
  if (name in orgTools.schemas) return orgTools.callTool(gogomail, suppo, name, args);
  if (name in systemTools.schemas) return systemTools.callTool(gogomail, suppo, name, args);
  throw new Error(`Unknown GoGoMail tool: ${name}`);
}
