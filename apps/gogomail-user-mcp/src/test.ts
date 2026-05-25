import { describe, test } from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { withMCPNotice } from "./client.js";
import { callTool, toolDefinitions } from "./tools.js";

type CapturedCall = { method: string; path: string; body?: unknown; headers?: Record<string, string> };

describe("MCP tool schema compatibility", () => {
  test("all advertised tools use Codex-compatible top-level object schemas", () => {
    for (const tool of toolDefinitions) {
      const schema = tool.inputSchema as Record<string, unknown>;
      assert.equal(schema.type, "object", `${tool.name} must expose an object input schema`);
      for (const keyword of ["oneOf", "anyOf", "allOf", "enum", "not"]) {
        assert.equal(Object.hasOwn(schema, keyword), false, `${tool.name} must not expose top-level ${keyword}`);
      }
      assertNoSchemaCombinators(schema, tool.name);
    }
  });
});

function assertNoSchemaCombinators(value: unknown, path: string): void {
  if (!value || typeof value !== "object") return;
  const record = value as Record<string, unknown>;
  for (const keyword of ["oneOf", "anyOf", "allOf", "not"]) {
    assert.equal(Object.hasOwn(record, keyword), false, `${path} must not expose ${keyword}`);
  }
  for (const [key, child] of Object.entries(record)) {
    assertNoSchemaCombinators(child, `${path}.${key}`);
  }
}

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
  test("creates Drive text files through upload sessions with declared_size and backend-default storage", async () => {
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

    await callTool(fake as never, "gogomail_drive_create_text_file", { name: "note.txt", content: "hello" }, "basic");

    assert.equal(calls[0]?.method, "POST");
    assert.equal(calls[0]?.path, "/api/v1/drive/upload-sessions");
    assert.deepEqual(calls[0]?.body, {
      parent_id: undefined,
      name: "note.txt",
      mime_type: "text/plain; charset=utf-8",
      declared_size: 5,
      storage_backend: undefined,
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

  test("typed context and bulk-mail tools map to documented user APIs", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_webmail_get_capabilities", {}, "basic");
    await callTool(fake as never, "gogomail_mailbox_get_overview", {}, "basic");
    await callTool(fake as never, "gogomail_account_get_profile", {}, "basic");
    await callTool(fake as never, "gogomail_account_update_profile", { display_name: "Park JW", recovery_email: "backup@example.com" }, "basic");
    await callTool(fake as never, "gogomail_account_list_addresses", {}, "basic");
    await callTool(fake as never, "gogomail_preferences_get", {}, "basic");
    await callTool(fake as never, "gogomail_mail_bulk_update_flags", { message_ids: ["m1", "m2"], flag: "answered", value: true }, "basic");
    await callTool(fake as never, "gogomail_mail_bulk_move_messages", { message_ids: ["m1"], folder_id: "archive" }, "basic");
    await callTool(fake as never, "gogomail_mail_bulk_delete_messages", { message_ids: ["m1"], confirm: "POST /api/v1/messages/bulk/delete" }, "basic");
    await callTool(fake as never, "gogomail_mail_bulk_update_thread_flags", { thread_ids: ["t1"], flag: "forwarded", value: false }, "basic");
    await callTool(fake as never, "gogomail_mail_bulk_move_threads", { thread_ids: ["t1"], folder_id: "archive" }, "basic");
    await callTool(fake as never, "gogomail_mail_bulk_delete_threads", { thread_ids: ["t1"], confirm: "POST /api/v1/threads/bulk/delete" }, "basic");

    assert.equal(calls[0]?.path, "/api/v1/webmail/capabilities");
    assert.equal(calls[1]?.path, "/api/v1/mailbox/overview");
    assert.equal(calls[2]?.path, "/api/v1/me");
    assert.deepEqual(calls[3], { method: "PATCH", path: "/api/v1/me", body: { display_name: "Park JW", recovery_email: "backup@example.com" }, headers: undefined });
    assert.equal(calls[4]?.path, "/api/v1/me/addresses");
    assert.equal(calls[5]?.path, "/api/v1/preferences");
    assert.deepEqual(calls[6], { method: "PATCH", path: "/api/v1/messages/bulk/flags", body: { message_ids: ["m1", "m2"], flag: "answered", value: true }, headers: undefined });
    assert.deepEqual(calls[7]?.body, { message_ids: ["m1"], folder_id: "archive" });
    assert.equal(calls[8]?.headers?.["X-Gogomail-MCP-Confirm"], "POST /api/v1/messages/bulk/delete");
    assert.deepEqual(calls[9]?.body, { thread_ids: ["t1"], flag: "forwarded", value: false });
    assert.equal(calls[10]?.method, "PATCH");
    assert.equal(calls[11]?.headers?.["X-Gogomail-MCP-Confirm"], "POST /api/v1/threads/bulk/delete");
  });

  test("notification and push tools map to documented webmail APIs", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        return { ok: true };
      },
    };
    const dnd = { weekdays: [1, 2], time_ranges: [{ start: "09:00", end: "17:00" }], timezone: "Asia/Seoul" };

    await callTool(fake as never, "gogomail_notifications_get_preferences", {}, "basic");
    await callTool(fake as never, "gogomail_notifications_update_preferences", {
      global_dnd_enabled: true,
      global_dnd_schedule: dnd,
      folder_overrides: {
        "11111111-1111-1111-1111-111111111111": { enabled: false, dnd_inherit: true, dnd_schedule: { weekdays: [], time_ranges: [], timezone: "UTC" } },
      },
      thread_overrides: {
        "22222222-2222-2222-2222-222222222222": { enabled: true },
      },
    }, "basic");
    await callTool(fake as never, "gogomail_notifications_get_web_push_config", {}, "basic");
    await callTool(fake as never, "gogomail_notifications_list_push_subscriptions", {}, "basic");
    await callTool(fake as never, "gogomail_notifications_upsert_push_subscription", { endpoint: "https://push.example/sub", p256dh: "key", auth: "auth", user_agent: "agent" }, "basic");
    await callTool(fake as never, "gogomail_notifications_delete_push_subscription", { id: "sub-1", confirm: "delete web push subscription sub-1" }, "basic");
    await callTool(fake as never, "gogomail_notifications_list_push_devices", { limit: 5 }, "basic");
    await callTool(fake as never, "gogomail_notifications_upsert_push_device", { platform: "webpush", token: "token-1", label: "browser" }, "basic");
    await callTool(fake as never, "gogomail_notifications_delete_push_device", { id: "device-1", confirm: "delete push device device-1" }, "basic");

    assert.deepEqual(calls[0], { method: "GET", path: "/api/v1/me/notification-preferences", body: undefined, headers: undefined });
    assert.equal(calls[1]?.method, "PUT");
    assert.equal(calls[1]?.path, "/api/v1/me/notification-preferences");
    assert.deepEqual((calls[1]?.body as { global_dnd_schedule?: unknown }).global_dnd_schedule, dnd);
    assert.equal(calls[2]?.path, "/api/v1/config/web-push");
    assert.equal(calls[3]?.path, "/api/v1/me/push-subscriptions");
    assert.deepEqual(calls[4]?.body, { endpoint: "https://push.example/sub", p256dh: "key", auth: "auth", userAgent: "agent" });
    assert.equal(calls[5]?.headers?.["X-Gogomail-MCP-Confirm"], "delete web push subscription sub-1");
    assert.equal(calls[6]?.path, "/api/v1/push-devices?limit=5");
    assert.deepEqual(calls[7]?.body, { platform: "webpush", token: "token-1", label: "browser" });
    assert.equal(calls[8]?.headers?.["X-Gogomail-MCP-Confirm"], "delete push device device-1");
  });

  test("profile avatar tools map to profile APIs with confirmation", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        return { avatar_url: "data:image/png;base64,AA==" };
      },
    };

    await callTool(fake as never, "gogomail_account_upload_avatar", {
      avatar_base64: Buffer.from("png").toString("base64"),
      mime_type: "image/png",
      filename: "avatar.png",
      confirm: "upload avatar",
    }, "basic");
    await callTool(fake as never, "gogomail_account_delete_avatar", { confirm: "delete avatar" }, "basic");

    assert.equal(calls[0]?.method, "PUT");
    assert.equal(calls[0]?.path, "/api/v1/me/avatar");
    assert.ok(Buffer.isBuffer(calls[0]?.body));
    assert.match(calls[0]?.headers?.["Content-Type"] ?? "", /^multipart\/form-data; boundary=gogomail-mcp-/);
    assert.equal(calls[1]?.method, "DELETE");
    assert.equal(calls[1]?.headers?.["X-Gogomail-MCP-Confirm"], "delete avatar");
  });

  test("directory profile and spam sender tools cover recent webmail spam UX", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        if (method === "GET" && path === "/api/v1/preferences") {
          return { preferences: { settings: { keep: true }, blocked_senders: ["old@example.com"], allowed_senders: ["friend@example.com"] } };
        }
        if (method === "GET" && path === "/api/v1/folders") {
          return { folders: [{ id: "inbox-id", system_type: "inbox" }, { id: "spam-id", system_type: "spam" }] };
        }
        if (method === "GET" && path === "/api/v1/messages/msg-1") {
          return { message: { from_addr: "Spammer@Example.com" } };
        }
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_directory_get_profile", { email: "person@example.com" }, "basic");
    await callTool(fake as never, "gogomail_spam_list_senders", { kind: "blocked", q: "old" }, "basic");
    await callTool(fake as never, "gogomail_spam_add_sender", { kind: "allowed", sender: "@Example.com", confirm: "add allowed sender @example.com" }, "basic");
    await callTool(fake as never, "gogomail_spam_remove_sender", { kind: "blocked", sender: "old@example.com", confirm: "remove blocked sender old@example.com" }, "basic");
    await callTool(fake as never, "gogomail_spam_report_message", { id: "msg-1", block_sender: true, block_domain: true, confirm: "report spam msg-1" }, "basic");
    await callTool(fake as never, "gogomail_spam_mark_not_spam", { id: "msg-1" }, "basic");

    assert.equal(calls[0]?.path, "/api/mail/directory/profile?email=person%40example.com");
    const allowedPut = calls.find((call) => call.method === "PUT" && call.path === "/api/v1/preferences" && Array.isArray((call.body as { allowed_senders?: unknown }).allowed_senders));
    assert.deepEqual((allowedPut?.body as { allowed_senders?: string[] }).allowed_senders, ["friend@example.com", "@example.com"]);
    const blockedRemovePut = calls.find((call) => call.method === "PUT" && call.path === "/api/v1/preferences" && Array.isArray((call.body as { blocked_senders?: unknown }).blocked_senders) && !(call.body as { blocked_senders: string[] }).blocked_senders.includes("old@example.com"));
    assert.ok(blockedRemovePut);
    assert.ok(calls.some((call) => call.method === "PATCH" && call.path === "/api/v1/messages/msg-1/folder" && (call.body as { folder_id?: string }).folder_id === "spam-id"));
    const spamPut = calls.filter((call) => call.method === "PUT" && call.path === "/api/v1/preferences").at(-1);
    assert.deepEqual((spamPut?.body as { blocked_senders?: string[] }).blocked_senders, ["old@example.com", "spammer@example.com", "@example.com"]);
    assert.ok(calls.some((call) => call.method === "PATCH" && call.path === "/api/v1/messages/msg-1/folder" && (call.body as { folder_id?: string }).folder_id === "inbox-id"));
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

  test("generic API bridge allows documented notification and push routes", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_api_request", { method: "GET", path: "/api/v1/config/web-push" }, "basic");
    await callTool(fake as never, "gogomail_api_request", { method: "POST", path: "/api/v1/me/push-subscriptions", body_json: { endpoint: "https://push.example/sub", p256dh: "key", auth: "auth" }, confirm: "POST /api/v1/me/push-subscriptions" }, "basic");
    await callTool(fake as never, "gogomail_api_request", { method: "DELETE", path: "/api/v1/push-devices/device-1", confirm: "delete push device device-1" }, "basic");

    assert.equal(calls[0]?.path, "/api/v1/config/web-push");
    assert.deepEqual(calls[1]?.body, { endpoint: "https://push.example/sub", p256dh: "key", auth: "auth" });
    assert.equal(calls[2]?.headers?.["X-Gogomail-MCP-Confirm"], "delete push device device-1");
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

  test("generic API bridge accepts documented PATCH bulk routes", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_api_request", {
      method: "PATCH",
      path: "/api/v1/messages/bulk/flags",
      body_json: { message_ids: ["m1"], flag: "read", value: true },
      confirm: "PATCH /api/v1/messages/bulk/flags",
    }, "basic");

    assert.equal(calls[0]?.method, "PATCH");
    assert.deepEqual(calls[0]?.body, { message_ids: ["m1"], flag: "read", value: true });
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

  test("DM tools map to the documented DM API surface", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        if (method === "GET" && path.includes("/attachment?")) {
          return { body_base64: Buffer.from("dm file").toString("base64"), content_type: "text/plain" };
        }
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_dm_list_rooms", {}, "basic");
    await callTool(fake as never, "gogomail_dm_list_public_rooms", {}, "basic");
    await callTool(fake as never, "gogomail_dm_create_room", { room_type: "group", user_ids: ["u1", "u2"], name: "Team", visibility: "private", confirm: "create dm room" }, "basic");
    await callTool(fake as never, "gogomail_dm_add_members", { room_id: "room-1", user_ids: ["u3"], confirm: "add dm members room-1" }, "basic");
    await callTool(fake as never, "gogomail_dm_remove_member", { room_id: "room-1", user_id: "u3", confirm: "remove dm member room-1 u3" }, "basic");
    await callTool(fake as never, "gogomail_dm_transfer_owner", { room_id: "room-1", user_id: "u2", confirm: "transfer dm owner room-1 u2" }, "basic");
    await callTool(fake as never, "gogomail_dm_create_invite", { room_id: "room-1", confirm: "create dm invite room-1" }, "basic");
    await callTool(fake as never, "gogomail_dm_join_invite", { token: "tok-1", confirm: "join dm invite tok-1" }, "basic");
    await callTool(fake as never, "gogomail_dm_list_messages", { room_id: "room-1", before: "m2", limit: 25 }, "basic");
    await callTool(fake as never, "gogomail_dm_send_message", { room_id: "room-1", body: "hello", confirm: "send dm message room-1" }, "basic");
    await callTool(fake as never, "gogomail_dm_send_attachment", { room_id: "room-1", filename: "note.txt", mime_type: "text/plain", content_base64: Buffer.from("hello").toString("base64"), confirm: "send dm attachment room-1" }, "basic");
    await callTool(fake as never, "gogomail_dm_mark_read", { room_id: "room-1", last_message_id: "m3" }, "basic");
    await callTool(fake as never, "gogomail_dm_search", { room_id: "room-1", q: "hello", limit: 5 }, "basic");
    await callTool(fake as never, "gogomail_dm_list_media", { room_id: "room-1", type: "file" }, "basic");
    await callTool(fake as never, "gogomail_dm_download_attachment", { attachment_download_url: "/api/v1/dm/messages/m-file/attachment?token=t1" }, "basic");
    await callTool(fake as never, "gogomail_dm_edit_message", { message_id: "m1", body: "edited", confirm: "edit dm message m1" }, "basic");
    await callTool(fake as never, "gogomail_dm_delete_message", { message_id: "m1", confirm: "delete dm message m1" }, "basic");
    await callTool(fake as never, "gogomail_dm_toggle_reaction", { message_id: "m2", emoji: "👍" }, "basic");

    assert.deepEqual(calls[0], { method: "GET", path: "/api/v1/dm/rooms", body: undefined, headers: undefined });
    assert.equal(calls[1]?.path, "/api/v1/dm/rooms/public");
    assert.equal(calls[2]?.headers?.["X-Gogomail-MCP-Confirm"], "create dm room");
    assert.deepEqual(calls[2]?.body, { room_type: "group", user_ids: ["u1", "u2"], name: "Team", visibility: "private" });
    assert.equal(calls[3]?.path, "/api/v1/dm/rooms/room-1/members");
    assert.equal(calls[4]?.method, "DELETE");
    assert.equal(calls[5]?.headers?.["X-Gogomail-MCP-Confirm"], "transfer dm owner room-1 u2");
    assert.equal(calls[6]?.path, "/api/v1/dm/rooms/room-1/invites");
    assert.equal(calls[7]?.path, "/api/v1/dm/join/tok-1");
    assert.equal(calls[8]?.path, "/api/v1/dm/rooms/room-1/messages?before=m2&limit=25");
    assert.deepEqual(calls[9]?.body, { body: "hello", drive_file_id: undefined });
    assert.ok(Buffer.isBuffer(calls[10]?.body));
    assert.match(calls[10]?.headers?.["Content-Type"] ?? "", /^multipart\/form-data; boundary=gogomail-mcp-/);
    assert.deepEqual(calls[11]?.body, { last_message_id: "m3" });
    assert.equal(calls[12]?.path, "/api/v1/dm/rooms/room-1/search?q=hello&limit=5");
    assert.equal(calls[13]?.path, "/api/v1/dm/rooms/room-1/media?type=file");
    assert.equal(calls[14]?.path, "/api/v1/dm/messages/m-file/attachment?token=t1");
    assert.equal(calls[15]?.headers?.["X-Gogomail-MCP-Confirm"], "edit dm message m1");
    assert.equal(calls[16]?.headers?.["X-Gogomail-MCP-Confirm"], "delete dm message m1");
    assert.deepEqual(calls[17]?.body, { emoji: "👍" });
  });

  test("generic API bridge admits DM routes and forwards DM confirmations", async () => {
    const calls: CapturedCall[] = [];
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        return { ok: true };
      },
    };

    await callTool(fake as never, "gogomail_api_request", { method: "POST", path: "/api/v1/dm/rooms/room-1/messages", body_json: { body: "hello" }, confirm: "send dm message room-1" }, "basic");
    await callTool(fake as never, "gogomail_api_request", { method: "DELETE", path: "/api/v1/dm/messages/msg-1", confirm: "delete dm message msg-1" }, "basic");

    assert.equal(calls[0]?.path, "/api/v1/dm/rooms/room-1/messages");
    assert.equal(calls[0]?.headers?.["X-Gogomail-MCP-Confirm"], "send dm message room-1");
    assert.equal(calls[1]?.headers?.["X-Gogomail-MCP-Confirm"], "delete dm message msg-1");
  });

  test("agent-native contact, calendar, Drive session, share, and download helpers work", async () => {
    const calls: CapturedCall[] = [];
    const tmp = await mkdtemp(join(tmpdir(), "gogomail-user-mcp-"));
    const downloadPath = join(tmp, "download.txt");
    const fake = {
      settings: async () => ({ permission_mode: "basic" as const }),
      request: async (method: string, path: string, body?: unknown, headers?: Record<string, string>) => {
        calls.push({ method, path, body, headers });
        if (path.includes("/download")) {
          return { body_text: "hello", body_base64: Buffer.from("hello", "utf8").toString("base64"), content_type: "text/plain" };
        }
        return { ok: true };
      },
    };

    try {
      await callTool(fake as never, "gogomail_contacts_upsert_simple", { addressbook_id: "book-1", full_name: "Park JW", email: "pjw@example.com", organization: "GoGoMail" }, "basic");
      await callTool(fake as never, "gogomail_calendar_upsert_event_simple", { calendar_id: "cal-1", summary: "Planning", starts_at: "2026-05-24T01:00:00Z", ends_at: "2026-05-24T02:00:00Z", location: "Seoul" }, "basic");
      await callTool(fake as never, "gogomail_drive_list_upload_sessions", { status: "open", limit: 5 }, "basic");
      await callTool(fake as never, "gogomail_drive_get_upload_session", { id: "up-1" }, "basic");
      await callTool(fake as never, "gogomail_drive_cancel_upload_session", { id: "up-1", confirm: "DELETE /api/v1/drive/upload-sessions/up-1" }, "basic");
      await callTool(fake as never, "gogomail_drive_get_share_link", { id: "share-1" }, "basic");
      await callTool(fake as never, "gogomail_drive_download_share_link", { id: "share-1", password: "pw" }, "basic");
      const saved = await callTool(fake as never, "gogomail_drive_download", { id: "node-1", save_to_path: downloadPath, confirm: `save download ${downloadPath}` }, "basic");

      assert.equal(calls[0]?.method, "PUT");
      assert.match(String(calls[0]?.body), /BEGIN:VCARD/);
      assert.match(String(calls[0]?.body), /FN:Park JW/);
      assert.equal(calls[0]?.headers?.["Content-Type"], "text/vcard");
      assert.equal(calls[1]?.method, "PUT");
      assert.match(String(calls[1]?.body), /BEGIN:VCALENDAR/);
      assert.match(String(calls[1]?.body), /SUMMARY:Planning/);
      assert.equal(calls[2]?.path, "/api/v1/drive/upload-sessions?status=open&limit=5");
      assert.equal(calls[3]?.path, "/api/v1/drive/upload-sessions/up-1");
      assert.equal(calls[4]?.headers?.["X-Gogomail-MCP-Confirm"], "DELETE /api/v1/drive/upload-sessions/up-1");
      assert.equal(calls[5]?.path, "/api/v1/drive/share-links/share-1");
      assert.deepEqual(calls[6]?.body, { password: "pw" });
      assert.equal(await readFile(downloadPath, "utf8"), "hello");
      assert.equal((saved as { saved_bytes?: number }).saved_bytes, 5);
    } finally {
      await rm(tmp, { recursive: true, force: true });
    }
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
