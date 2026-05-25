import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { z } from "zod";
import { appendQuery, GogomailUserClient } from "../client.js";
import { confirm, contractName, email, id, nameOrLegacyDisplayName } from "./schemas.js";
import { sanitizeFileSegment } from "./schemas.js";

export const toolDefinitions: Tool[] = [
  { name: "gogomail_contacts_list_addressbooks", description: "List address books using GET /api/mail/addressbooks.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_contacts_create_addressbook", description: "Create an address book using POST /api/mail/addressbooks.", inputSchema: { type: "object", properties: { name: { type: "string" }, description: { type: "string" } }, required: ["name"] } },
  { name: "gogomail_contacts_get_addressbook", description: "Get an address book using GET /api/mail/addressbooks/{id}.", inputSchema: { type: "object", properties: { addressbook_id: { type: "string", maxLength: 200 } }, required: ["addressbook_id"] } },
  { name: "gogomail_contacts_update_addressbook", description: "Update an address book using PATCH /api/mail/addressbooks/{id}.", inputSchema: { type: "object", properties: { addressbook_id: { type: "string", maxLength: 200 }, name: { type: "string" }, description: { type: "string" } }, required: ["addressbook_id"] } },
  { name: "gogomail_contacts_upsert_simple", description: "Create or update a contact without hand-writing vCard. Uses PUT /api/mail/addressbooks/{id}/contacts/{name}.", inputSchema: { type: "object", properties: { addressbook_id: { type: "string", maxLength: 200 }, object_name: { type: "string", maxLength: 200 }, full_name: { type: "string" }, email: { type: "string", format: "email" }, phone: { type: "string" }, organization: { type: "string" }, title: { type: "string" }, note: { type: "string" } }, required: ["addressbook_id", "full_name"] } },
  { name: "gogomail_contacts_delete_addressbook", description: "Delete an address book using DELETE /api/mail/addressbooks/{id}. In basic mode confirm must equal `delete addressbook <id>`.", inputSchema: { type: "object", properties: { addressbook_id: { type: "string", maxLength: 200 }, confirm: { type: "string" } }, required: ["addressbook_id"] } },
  { name: "gogomail_contacts_list", description: "List contacts in an address book using GET /api/mail/addressbooks/{id}/contacts.", inputSchema: { type: "object", properties: { addressbook_id: { type: "string", maxLength: 200 } }, required: ["addressbook_id"] } },
  { name: "gogomail_contacts_get", description: "Get a vCard contact using GET /api/mail/addressbooks/{id}/contacts/{name}.", inputSchema: { type: "object", properties: { addressbook_id: { type: "string", maxLength: 200 }, object_name: { type: "string", maxLength: 200 } }, required: ["addressbook_id", "object_name"] } },
  { name: "gogomail_contacts_autocomplete", description: "Search contact suggestions using GET /api/mail/contacts/autocomplete.", inputSchema: { type: "object", properties: { q: { type: "string", maxLength: 255 }, limit: { type: "number", minimum: 1, maximum: 50 } }, required: ["q"] } },
  { name: "gogomail_contacts_upsert", description: "Create or update a vCard contact using PUT /api/mail/addressbooks/{id}/contacts/{name}.", inputSchema: { type: "object", properties: { addressbook_id: { type: "string", maxLength: 200 }, object_name: { type: "string", maxLength: 200 }, vcard: { type: "string" } }, required: ["addressbook_id", "object_name", "vcard"] } },
  { name: "gogomail_contacts_delete", description: "Delete a contact using DELETE /api/mail/addressbooks/{id}/contacts/{name}. In basic mode confirm must equal `delete contact <object_name>`.", inputSchema: { type: "object", properties: { addressbook_id: { type: "string", maxLength: 200 }, object_name: { type: "string", maxLength: 200 }, confirm: { type: "string" } }, required: ["addressbook_id", "object_name"] } },
  { name: "gogomail_directory_search_users", description: "Search company directory users using GET /api/mail/directory/users.", inputSchema: { type: "object", properties: { q: { type: "string", maxLength: 255 }, limit: { type: "number", minimum: 1, maximum: 200 } } } },
  { name: "gogomail_directory_org_tree", description: "Read the organization tree using GET /api/mail/directory/org-tree.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_directory_get_profile", description: "Read a directory profile, including organization unit name and title, using GET /api/mail/directory/profile.", inputSchema: { type: "object", properties: { email: { type: "string", format: "email" } }, required: ["email"] } },
];

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_contacts_list_addressbooks: z.object({}),
  gogomail_contacts_create_addressbook: nameOrLegacyDisplayName,
  gogomail_contacts_get_addressbook: z.object({ addressbook_id: id }),
  gogomail_contacts_update_addressbook: z.object({
    addressbook_id: id,
    name: z.string().max(200).optional(),
    display_name: z.string().max(200).optional(),
    description: z.string().max(1000).optional(),
  }),
  gogomail_contacts_upsert_simple: z.object({
    addressbook_id: id,
    object_name: id.optional(),
    full_name: z.string().min(1).max(255),
    email: email.optional(),
    phone: z.string().max(100).optional(),
    organization: z.string().max(255).optional(),
    title: z.string().max(255).optional(),
    note: z.string().max(1000).optional(),
  }),
  gogomail_contacts_delete_addressbook: z.object({ addressbook_id: id, confirm }),
  gogomail_contacts_list: z.object({ addressbook_id: id }),
  gogomail_contacts_get: z.object({ addressbook_id: id, object_name: id }),
  gogomail_contacts_autocomplete: z.object({ q: z.string().min(1).max(255), limit: z.number().int().min(1).max(50).optional() }),
  gogomail_contacts_upsert: z.object({ addressbook_id: id, object_name: id, vcard: z.string().min(1) }),
  gogomail_contacts_delete: z.object({ addressbook_id: id, object_name: id, confirm }),
  gogomail_directory_search_users: z.object({ q: z.string().max(255).optional(), limit: z.number().int().min(1).max(200).optional() }),
  gogomail_directory_org_tree: z.object({}),
  gogomail_directory_get_profile: z.object({ email }),
};

export async function callTool(
  client: GogomailUserClient,
  name: string,
  args: Record<string, unknown>,
  _mode: "basic" | "bypass",
  requireConfirm: (expected: string) => Record<string, string>,
): Promise<unknown> {
  switch (name) {
    case "gogomail_contacts_list_addressbooks":
      return client.request("GET", "/api/mail/addressbooks");
    case "gogomail_contacts_create_addressbook":
      return client.request("POST", "/api/mail/addressbooks", { name: args.name ?? args.display_name, description: args.description });
    case "gogomail_contacts_get_addressbook":
      return client.request("GET", `/api/mail/addressbooks/${encodeURIComponent(String(args.addressbook_id))}`);
    case "gogomail_contacts_update_addressbook":
      return client.request("PATCH", `/api/mail/addressbooks/${encodeURIComponent(String(args.addressbook_id))}`, {
        name: args.name ?? args.display_name,
        description: args.description,
      });
    case "gogomail_contacts_upsert_simple": {
      const contact = buildSimpleVCard(args);
      return client.request(
        "PUT",
        `/api/mail/addressbooks/${encodeURIComponent(String(args.addressbook_id))}/contacts/${encodeURIComponent(contact.objectName)}`,
        contact.vcard,
        { "Content-Type": "text/vcard" },
      );
    }
    case "gogomail_contacts_delete_addressbook":
      return client.request(
        "DELETE",
        `/api/mail/addressbooks/${encodeURIComponent(String(args.addressbook_id))}`,
        undefined,
        requireConfirm(`delete addressbook ${args.addressbook_id}`),
      );
    case "gogomail_contacts_list":
      return client.request("GET", `/api/mail/addressbooks/${encodeURIComponent(String(args.addressbook_id))}/contacts`);
    case "gogomail_contacts_get":
      return client.request(
        "GET",
        `/api/mail/addressbooks/${encodeURIComponent(String(args.addressbook_id))}/contacts/${encodeURIComponent(String(args.object_name))}`,
      );
    case "gogomail_contacts_autocomplete":
      return client.request("GET", appendQuery("/api/mail/contacts/autocomplete", args));
    case "gogomail_contacts_upsert":
      return client.request(
        "PUT",
        `/api/mail/addressbooks/${encodeURIComponent(String(args.addressbook_id))}/contacts/${encodeURIComponent(String(args.object_name))}`,
        String(args.vcard),
        { "Content-Type": "text/vcard" },
      );
    case "gogomail_contacts_delete":
      return client.request(
        "DELETE",
        `/api/mail/addressbooks/${encodeURIComponent(String(args.addressbook_id))}/contacts/${encodeURIComponent(String(args.object_name))}`,
        undefined,
        requireConfirm(`delete contact ${args.object_name}`),
      );
    case "gogomail_directory_search_users":
      return client.request("GET", appendQuery("/api/mail/directory/users", args));
    case "gogomail_directory_org_tree":
      return client.request("GET", "/api/mail/directory/org-tree");
    case "gogomail_directory_get_profile":
      return client.request("GET", appendQuery("/api/mail/directory/profile", { email: args.email }));
    default:
      throw new Error(`contacts: unhandled tool: ${name}`);
  }
}

function buildSimpleVCard(args: Record<string, unknown>): { objectName: string; vcard: string } {
  const fullName = String(args.full_name);
  const objectName = String(args.object_name ?? `${sanitizeFileSegment(String(args.email ?? fullName))}.vcf`);
  const lines = ["BEGIN:VCARD", "VERSION:3.0", `FN:${escapeVCardText(fullName)}`];
  if (args.email) lines.push(`EMAIL;TYPE=INTERNET:${String(args.email)}`);
  if (args.phone) lines.push(`TEL:${escapeVCardText(String(args.phone))}`);
  if (args.organization) lines.push(`ORG:${escapeVCardText(String(args.organization))}`);
  if (args.title) lines.push(`TITLE:${escapeVCardText(String(args.title))}`);
  if (args.note) lines.push(`NOTE:${escapeVCardText(String(args.note))}`);
  lines.push("END:VCARD");
  return { objectName, vcard: `${lines.join("\r\n")}\r\n` };
}

function escapeVCardText(value: string): string {
  return value.replace(/\\/g, "\\\\").replace(/\r?\n/g, "\\n").replace(/;/g, "\\;").replace(/,/g, "\\,");
}

// contractName is imported but only used via nameOrLegacyDisplayName schema above — keep the import to avoid unused-var warnings
void (contractName as unknown);
