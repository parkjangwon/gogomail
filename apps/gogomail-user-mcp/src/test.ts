import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { withMCPNotice } from "./client.js";
import { callTool } from "./tools.js";

type CapturedCall = { method: string; path: string; body?: unknown; headers?: Record<string, string> };

describe("MCP generated mail notice", () => {
  test("prepends Korean notice to text and HTML bodies by default", () => {
    const out = withMCPNotice({ text_body: "hello", html_body: "<p>hello</p>" }, {});
    assert.match(out.text_body ?? "", /^MCP를 통해 작성된 메일입니다\./);
    assert.match(out.html_body ?? "", /color:#8a8a8a/);
  });

  test("does not prepend notice when disabled", () => {
    const out = withMCPNotice({ text_body: "hello" }, { generated_mail_notice_enabled: false });
    assert.equal(out.text_body, "hello");
  });
});

describe("GoGoMail API contract alignment", () => {
  test("creates Drive text files through upload sessions with declared_size and storage_backend", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "bypass" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        if (method === "POST" && path === "/api/v1/drive/upload-sessions") {
          return { drive_upload_session: { id: "session-1" } };
        }
        if (method === "POST" && path.endsWith("/finalize")) return { drive_node: { id: "node-1" } };
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_drive_create_text_file", { name: "note.txt", content: "hello", storage_backend: "local" }, "basic");

    assert.equal(calls[0]?.method, "POST");
    assert.equal(calls[0]?.path, "/api/v1/drive/upload-sessions");
    assert.deepEqual(calls[0]?.body, {
      parent_id: undefined,
      name: "note.txt",
      mime_type: "text/plain; charset=utf-8",
      declared_size: 5,
      storage_backend: "local",
    });
    assert.equal(calls[1]?.method, "PUT");
    assert.equal(calls[1]?.path, "/api/v1/drive/upload-sessions/session-1/body");
    assert.equal(calls[1]?.headers?.["Content-Type"], "application/octet-stream");
    assert.match(calls[1]?.headers?.["X-Content-SHA256"] ?? "", /^[0-9a-f]{64}$/);
    assert.equal(calls[2]?.method, "POST");
    assert.equal(calls[2]?.path, "/api/v1/drive/upload-sessions/session-1/finalize");
  });

  test("uses CardDAV and CalDAV name fields accepted by the Go handlers", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "bypass" as const }),
      request: async (method: string, path: string, body?: unknown) => {
        calls.push({ method, path, body });
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_contacts_create_addressbook", { name: "Team", description: "People" }, "basic");
    await callTool(fake as never, "gogomail_contacts_update_addressbook", { addressbook_id: "book-1", name: "Team 2" }, "basic");
    await callTool(fake as never, "gogomail_calendar_update", { calendar_id: "cal-1", name: "Work", color: "#2563eb" }, "basic");

    assert.deepEqual(calls[0], { method: "POST", path: "/api/mail/addressbooks", body: { name: "Team", description: "People" } });
    assert.deepEqual(calls[1], { method: "PATCH", path: "/api/mail/addressbooks/book-1", body: { name: "Team 2", description: undefined } });
    assert.deepEqual(calls[2], { method: "PATCH", path: "/api/v1/calendars/cal-1", body: { name: "Work", description: undefined, color: "#2563eb" } });
  });

  test("generic API bridge reaches user APIs and forwards sensitive confirmation", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_api_request", { method: "DELETE", path: "/api/v1/messages/msg-1", confirm: "delete message msg-1" }, "basic");

    assert.equal(calls[0]?.method, "DELETE");
    assert.equal(calls[0]?.path, "/api/v1/messages/msg-1");
    assert.equal(calls[0]?.headers?.["X-Gogomail-MCP-Confirm"], "delete message msg-1");
  });

  test("generic API bridge blocks account and key-management routes", async () => {
    const fake = {
      settings: async () => ({ permission_mode: "bypass" as const }),
      request: async () => ({ ok: true }),
    };

    await assert.rejects(
      () => callTool(fake as never, "gogomail_api_request", { method: "GET", path: "/api/v1/me/mcp/access-keys" }, "basic"),
      /not allowed/,
    );
    await assert.rejects(
      () => callTool(fake as never, "gogomail_api_request", { method: "POST", path: "/api/v1/auth/token", body_json: { email: "a@example.com" } }, "basic"),
      /not allowed/,
    );
  });

  test("generic API bridge supports text bodies for CalDAV and CardDAV routes", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "bypass" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_api_request", {
      method: "PUT",
      path: "/api/v1/calendars/cal-1/objects/event.ics",
      body_text: "BEGIN:VCALENDAR\nEND:VCALENDAR",
      content_type: "text/calendar",
      confirm: "PUT /api/v1/calendars/cal-1/objects/event.ics",
    }, "basic");

    assert.equal(calls[0]?.headers?.["Content-Type"], "text/calendar");
    assert.equal(calls[0]?.body, "BEGIN:VCALENDAR\nEND:VCALENDAR");
  });

  test("mail send forwards granular backend confirmation headers", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const, generated_mail_notice_enabled: false }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_mail_send", {
      to: [{ email: "external@example.net" }],
      attachment_ids: ["att-1"],
      text_body: "body",
      confirm: "send message",
      confirm_external_recipients: "external recipients",
      confirm_attachments: "send attachments",
    }, "basic");

    assert.equal(calls[0]?.headers?.["X-Gogomail-MCP-Confirm"], "send message");
    assert.equal(calls[0]?.headers?.["X-Gogomail-MCP-External-Confirm"], "external recipients");
    assert.equal(calls[0]?.headers?.["X-Gogomail-MCP-Attachment-Confirm"], "send attachments");
    assert.equal((calls[0]?.body as { confirm_external_recipients?: unknown }).confirm_external_recipients, undefined);
  });

  test("typed coverage reaches folders, threads, directory, subscriptions, and attachment sessions", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "bypass" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        if (method === "POST" && path === "/api/v1/attachments/upload-sessions") {
          return { attachment_upload_session: { id: "session-1" } };
        }
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_mail_create_folder", { name: "Projects" }, "basic");
    await callTool(fake as never, "gogomail_mail_get_thread_messages", { id: "thread-1", limit: 10 }, "basic");
    await callTool(fake as never, "gogomail_directory_search_users", { q: "kim", limit: 5 }, "basic");
    await callTool(fake as never, "gogomail_calendar_get_subscription_events", { id: "sub-1", since: "2026-01-01T00:00:00Z" }, "basic");
    await callTool(fake as never, "gogomail_mail_create_text_attachment", { draft_id: "draft-1", filename: "note.txt", content: "hello" }, "basic");

    assert.deepEqual(calls[0], { method: "POST", path: "/api/v1/folders", body: { name: "Projects" }, headers: undefined });
    assert.equal(calls[1]?.path, "/api/v1/threads/thread-1/messages?limit=10");
    assert.equal(calls[2]?.path, "/api/mail/directory/users?q=kim&limit=5");
    assert.equal(calls[3]?.path, "/api/v1/calendar-subscriptions/sub-1/events?since=2026-01-01T00%3A00%3A00Z");
    assert.equal(calls[4]?.path, "/api/v1/attachments/upload-sessions");
    assert.deepEqual(calls[4]?.body, { draft_id: "draft-1", filename: "note.txt", declared_size: 5, mime_type: "text/plain; charset=utf-8" });
    assert.equal(calls[5]?.method, "PUT");
    assert.equal(calls[5]?.path, "/api/v1/attachments/upload-sessions/session-1/body");
    assert.equal(calls[5]?.headers?.["Content-Type"], "text/plain; charset=utf-8");
    assert.match(calls[5]?.headers?.["X-Content-SHA256"] ?? "", /^[0-9a-f]{64}$/);
    assert.equal(calls[6]?.path, "/api/v1/attachments/upload-sessions/session-1/finalize");
  });
});

describe("sensitive action confirmation", () => {
  test("requires send confirmation in basic mode", async () => {
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async () => ({}),
    };
    await assert.rejects(
      () => callTool(fake as never, "gogomail_mail_send", { to: [{ email: "a@example.com" }], subject: "hi", text_body: "body" }, "basic"),
      /confirmation required/,
    );
  });

  test("allows send confirmation bypass mode", async () => {
    const calls: unknown[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "bypass" as const }),
      request: async (_method: string, _path: string, body?: unknown) => {
        calls.push(body);
        return { ok: true };
      },
    };
    const result = await callTool(fake as never, "gogomail_mail_send", { to: [{ email: "a@example.com" }], subject: "hi", text_body: "body" }, "basic");
    assert.deepEqual(result, { ok: true });
    assert.equal(calls.length, 1);
  });

  test("allows CC-only send and forwards backend confirmation header in basic mode", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_mail_send", { cc: [{ email: "cc@example.com" }], text_body: "body", confirm: "send message" }, "basic");

    assert.equal(calls[0]?.path, "/api/v1/messages/send");
    assert.equal(calls[0]?.headers?.["X-Gogomail-MCP-Confirm"], "send message");
    assert.deepEqual((calls[0]?.body as { cc?: unknown }).cc, [{ email: "cc@example.com" }]);
  });

  test("requires Drive copy name to match backend contract", async () => {
    const fake = {
      settings: async () => ({ permission_mode: "bypass" as const }),
      request: async () => ({ ok: true }),
    };

    await assert.rejects(
      () => callTool(fake as never, "gogomail_drive_copy", { id: "node-1" }, "basic"),
      /Required|name/,
    );
  });
});
