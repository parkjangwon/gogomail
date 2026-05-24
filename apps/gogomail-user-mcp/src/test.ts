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
