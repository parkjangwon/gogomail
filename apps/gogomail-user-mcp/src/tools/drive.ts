import type { Tool } from "@modelcontextprotocol/sdk/types.js";
import { createHash } from "node:crypto";
import { mkdir, writeFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { z } from "zod";
import { appendQuery, GogomailUserClient } from "../client.js";
import { confirm, id, optionalID, outputPath, storageBackend } from "./schemas.js";

export const toolDefinitions: Tool[] = [
  { name: "gogomail_drive_list", description: "List Drive nodes using GET /api/v1/drive/nodes.", inputSchema: { type: "object", properties: { parent_id: { type: "string", maxLength: 200 }, q: { type: "string", maxLength: 255 }, all_parents: { type: "boolean" }, status: { type: "string", enum: ["active", "trashed", "deleted"] }, node_type: { type: "string", enum: ["folder", "file"] }, sort: { type: "string", enum: ["name", "updated", "created", "size"] }, limit: { type: "number", minimum: 1, maximum: 200 } } } },
  { name: "gogomail_drive_get", description: "Get a Drive node using GET /api/v1/drive/nodes/{id}.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, status: { type: "string", enum: ["active", "trashed", "deleted"] } }, required: ["id"] } },
  { name: "gogomail_drive_download", description: "Download a Drive file using GET /api/v1/drive/nodes/{id}/download. Returns base64 and can optionally save to a local path; in basic mode local saves require confirm=`save download <path>`.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, save_to_path: { type: "string", maxLength: 4096 }, overwrite: { type: "boolean" }, confirm: { type: "string" } }, required: ["id"] } },
  { name: "gogomail_drive_create_folder", description: "Create a Drive folder using POST /api/v1/drive/folders.", inputSchema: { type: "object", properties: { parent_id: { type: "string", maxLength: 200 }, name: { type: "string" } }, required: ["name"] } },
  { name: "gogomail_drive_create_text_file", description: "Upload a UTF-8 text file through the real Drive upload-session API. Omitting storage_backend uses the backend's configured default store.", inputSchema: { type: "object", properties: { parent_id: { type: "string", maxLength: 200 }, name: { type: "string" }, mime_type: { type: "string" }, storage_backend: { type: "string" }, content: { type: "string" } }, required: ["name", "content"] } },
  { name: "gogomail_drive_list_upload_sessions", description: "List Drive upload sessions using GET /api/v1/drive/upload-sessions.", inputSchema: { type: "object", properties: { status: { type: "string" }, limit: { type: "number", minimum: 1, maximum: 200 } } } },
  { name: "gogomail_drive_get_upload_session", description: "Get one Drive upload session using GET /api/v1/drive/upload-sessions/{id}.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 } }, required: ["id"] } },
  { name: "gogomail_drive_cancel_upload_session", description: "Cancel a Drive upload session using DELETE /api/v1/drive/upload-sessions/{id}. In basic mode confirm must equal `DELETE /api/v1/drive/upload-sessions/<id>`.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, confirm: { type: "string" } }, required: ["id"] } },
  { name: "gogomail_drive_rename", description: "Rename a Drive node using PATCH /api/v1/drive/nodes/{id}/name.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, name: { type: "string" } }, required: ["id", "name"] } },
  { name: "gogomail_drive_move", description: "Move a Drive node using PATCH /api/v1/drive/nodes/{id}/parent.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, parent_id: { type: "string", maxLength: 200 } }, required: ["id"] } },
  { name: "gogomail_drive_copy", description: "Copy a Drive node using POST /api/v1/drive/nodes/{id}/copy.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, parent_id: { type: "string", maxLength: 200 }, name: { type: "string" } }, required: ["id", "name"] } },
  { name: "gogomail_drive_trash", description: "Move a Drive node to trash. In basic mode confirm must equal `trash drive <id>`.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, confirm: { type: "string" } }, required: ["id"] } },
  { name: "gogomail_drive_restore", description: "Restore a trashed Drive node using POST /api/v1/drive/nodes/{id}/restore.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 } }, required: ["id"] } },
  { name: "gogomail_drive_delete", description: "Permanently delete an already-trashed Drive node using DELETE /api/v1/drive/nodes/{id}. In basic mode confirm must equal `delete drive <id>`.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, confirm: { type: "string" } }, required: ["id"] } },
  { name: "gogomail_drive_share_link", description: "Create a public Drive share link. In basic mode confirm must equal `share drive <id>`.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, permission: { type: "string", enum: ["view", "download"] }, expires_at: { type: "string" }, password: { type: "string" }, confirm: { type: "string" } }, required: ["id"] } },
  { name: "gogomail_drive_get_share_link", description: "Resolve public Drive share-link metadata using GET /api/v1/drive/share-links/{id}.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 } }, required: ["id"] } },
  { name: "gogomail_drive_download_share_link", description: "Download a public Drive share link. Supports password-protected links and optional local save; in basic mode local saves require confirm=`save download <path>`.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, password: { type: "string" }, save_to_path: { type: "string", maxLength: 4096 }, overwrite: { type: "boolean" }, confirm: { type: "string" } }, required: ["id"] } },
  { name: "gogomail_drive_usage", description: "Read Drive quota/usage using GET /api/v1/drive/usage.", inputSchema: { type: "object", properties: {} } },
  { name: "gogomail_drive_list_share_links", description: "List Drive share links using GET /api/v1/drive/share-links.", inputSchema: { type: "object", properties: { node_id: { type: "string", maxLength: 200 }, status: { type: "string" }, limit: { type: "number", minimum: 1, maximum: 200 } } } },
  { name: "gogomail_drive_delete_share_link", description: "Delete a Drive share link using DELETE /api/v1/drive/share-links/{id}. In basic mode confirm must equal `DELETE /api/v1/drive/share-links/<id>`.", inputSchema: { type: "object", properties: { id: { type: "string", maxLength: 200 }, confirm: { type: "string" } }, required: ["id"] } },
];

export const schemas: Record<string, z.ZodTypeAny> = {
  gogomail_drive_list: z.object({
    parent_id: optionalID,
    q: z.string().max(255).optional(),
    all_parents: z.boolean().optional(),
    status: z.enum(["active", "trashed", "deleted"]).optional(),
    node_type: z.enum(["folder", "file"]).optional(),
    sort: z.enum(["name", "updated", "created", "size"]).optional(),
    limit: z.number().int().min(1).max(200).optional(),
  }),
  gogomail_drive_get: z.object({ id, status: z.enum(["active", "trashed", "deleted"]).optional() }),
  gogomail_drive_download: z.object({ id, save_to_path: outputPath.optional(), overwrite: z.boolean().default(false), confirm }),
  gogomail_drive_create_folder: z.object({ parent_id: optionalID, name: z.string().min(1).max(255) }),
  gogomail_drive_create_text_file: z.object({
    parent_id: optionalID,
    name: z.string().min(1).max(255),
    mime_type: z.string().default("text/plain; charset=utf-8"),
    storage_backend: storageBackend.optional(),
    content: z.string(),
  }),
  gogomail_drive_list_upload_sessions: z.object({ status: z.string().max(64).optional(), limit: z.number().int().min(1).max(200).optional() }),
  gogomail_drive_get_upload_session: z.object({ id }),
  gogomail_drive_cancel_upload_session: z.object({ id, confirm }),
  gogomail_drive_rename: z.object({ id, name: z.string().min(1).max(255) }),
  gogomail_drive_move: z.object({ id, parent_id: optionalID }),
  gogomail_drive_copy: z.object({ id, parent_id: optionalID, name: z.string().min(1).max(255) }),
  gogomail_drive_trash: z.object({ id, confirm }),
  gogomail_drive_restore: z.object({ id }),
  gogomail_drive_delete: z.object({ id, confirm }),
  gogomail_drive_share_link: z.object({
    id,
    permission: z.enum(["view", "download"]).default("view"),
    expires_at: z.string().optional(),
    password: z.string().optional(),
    confirm,
  }),
  gogomail_drive_get_share_link: z.object({ id }),
  gogomail_drive_download_share_link: z.object({
    id,
    password: z.string().optional(),
    save_to_path: outputPath.optional(),
    overwrite: z.boolean().default(false),
    confirm,
  }),
  gogomail_drive_usage: z.object({}),
  gogomail_drive_list_share_links: z.object({ node_id: optionalID, status: z.string().max(64).optional(), limit: z.number().int().min(1).max(200).optional() }),
  gogomail_drive_delete_share_link: z.object({ id, confirm }),
};

export async function callTool(
  client: GogomailUserClient,
  name: string,
  args: Record<string, unknown>,
  mode: "basic" | "bypass",
  requireConfirm: (expected: string) => Record<string, string>,
): Promise<unknown> {
  switch (name) {
    case "gogomail_drive_list":
      return client.request("GET", appendQuery("/api/v1/drive/nodes", args));
    case "gogomail_drive_get":
      return client.request("GET", appendQuery(`/api/v1/drive/nodes/${encodeURIComponent(String(args.id))}`, { status: args.status }));
    case "gogomail_drive_download": {
      const downloaded = await client.request("GET", `/api/v1/drive/nodes/${encodeURIComponent(String(args.id))}/download`);
      return saveDownloadIfRequested(downloaded, args, mode);
    }
    case "gogomail_drive_create_folder":
      return client.request("POST", "/api/v1/drive/folders", { parent_id: args.parent_id, name: args.name });
    case "gogomail_drive_create_text_file": {
      const content = Buffer.from(String(args.content), "utf8");
      const session = await client.request<{ drive_upload_session: { id: string } }>("POST", "/api/v1/drive/upload-sessions", {
        parent_id: args.parent_id,
        name: args.name,
        mime_type: args.mime_type,
        declared_size: content.length,
        storage_backend: args.storage_backend,
      });
      await client.request("PUT", `/api/v1/drive/upload-sessions/${encodeURIComponent(session.drive_upload_session.id)}/body`, content, {
        "Content-Type": "application/octet-stream",
        "X-Content-SHA256": createHash("sha256").update(content).digest("hex"),
      });
      return client.request("POST", `/api/v1/drive/upload-sessions/${encodeURIComponent(session.drive_upload_session.id)}/finalize`);
    }
    case "gogomail_drive_list_upload_sessions":
      return client.request("GET", appendQuery("/api/v1/drive/upload-sessions", args));
    case "gogomail_drive_get_upload_session":
      return client.request("GET", `/api/v1/drive/upload-sessions/${encodeURIComponent(String(args.id))}`);
    case "gogomail_drive_cancel_upload_session":
      return client.request(
        "DELETE",
        `/api/v1/drive/upload-sessions/${encodeURIComponent(String(args.id))}`,
        undefined,
        requireConfirm(`DELETE /api/v1/drive/upload-sessions/${args.id}`),
      );
    case "gogomail_drive_rename":
      return client.request("PATCH", `/api/v1/drive/nodes/${encodeURIComponent(String(args.id))}/name`, { name: args.name });
    case "gogomail_drive_move":
      return client.request("PATCH", `/api/v1/drive/nodes/${encodeURIComponent(String(args.id))}/parent`, { parent_id: args.parent_id });
    case "gogomail_drive_copy":
      return client.request("POST", `/api/v1/drive/nodes/${encodeURIComponent(String(args.id))}/copy`, { parent_id: args.parent_id, name: args.name });
    case "gogomail_drive_trash":
      return client.request(
        "POST",
        `/api/v1/drive/nodes/${encodeURIComponent(String(args.id))}/trash`,
        undefined,
        requireConfirm(`trash drive ${args.id}`),
      );
    case "gogomail_drive_restore":
      return client.request("POST", `/api/v1/drive/nodes/${encodeURIComponent(String(args.id))}/restore`);
    case "gogomail_drive_delete":
      return client.request(
        "DELETE",
        `/api/v1/drive/nodes/${encodeURIComponent(String(args.id))}`,
        undefined,
        requireConfirm(`delete drive ${args.id}`),
      );
    case "gogomail_drive_share_link":
      return client.request(
        "POST",
        `/api/v1/drive/nodes/${encodeURIComponent(String(args.id))}/share-links`,
        { permission: args.permission, expires_at: args.expires_at, password: args.password },
        requireConfirm(`share drive ${args.id}`),
      );
    case "gogomail_drive_get_share_link":
      return client.request("GET", `/api/v1/drive/share-links/${encodeURIComponent(String(args.id))}`);
    case "gogomail_drive_download_share_link": {
      const path = `/api/v1/drive/share-links/${encodeURIComponent(String(args.id))}/download`;
      const downloaded = args.password ? await client.request("POST", path, { password: args.password }) : await client.request("GET", path);
      return saveDownloadIfRequested(downloaded, args, mode);
    }
    case "gogomail_drive_usage":
      return client.request("GET", "/api/v1/drive/usage");
    case "gogomail_drive_list_share_links":
      return client.request("GET", appendQuery("/api/v1/drive/share-links", args));
    case "gogomail_drive_delete_share_link":
      return client.request(
        "DELETE",
        `/api/v1/drive/share-links/${encodeURIComponent(String(args.id))}`,
        undefined,
        requireConfirm(`DELETE /api/v1/drive/share-links/${args.id}`),
      );
    default:
      throw new Error(`drive: unhandled tool: ${name}`);
  }
}

type DownloadEnvelope = {
  body?: string;
  body_text?: string;
  body_base64?: string;
  content_type?: string;
  [key: string]: unknown;
};

async function saveDownloadIfRequested(result: unknown, args: Record<string, unknown>, mode: "basic" | "bypass"): Promise<unknown> {
  const saveToPath = args.save_to_path ? String(args.save_to_path) : "";
  if (!saveToPath) return result;
  const expected = `save download ${saveToPath}`;
  if (mode !== "bypass" && args.confirm !== expected) {
    throw new Error(`confirmation required: confirm must equal "${expected}"`);
  }
  const record = (result ?? {}) as DownloadEnvelope;
  const bytes = downloadBytes(record);
  const finalPath = resolve(saveToPath);
  await mkdir(dirname(finalPath), { recursive: true });
  await writeFile(finalPath, bytes, { flag: args.overwrite ? "w" : "wx" });
  return { ...record, saved_to_path: finalPath, saved_bytes: bytes.length };
}

function downloadBytes(record: DownloadEnvelope): Buffer {
  if (typeof record.body_base64 === "string") return Buffer.from(record.body_base64, "base64");
  if (typeof record.body_text === "string") return Buffer.from(record.body_text, "utf8");
  if (typeof record.body === "string") return Buffer.from(record.body, "utf8");
  throw new Error("download response did not include body_base64 or body_text");
}
