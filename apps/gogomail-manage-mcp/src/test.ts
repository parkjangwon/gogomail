/**
 * Security hardening tests for gogomail-manage-mcp.
 * Run: tsx src/test.ts
 *
 * Covers: rate limiting, 5xx masking, inputSchema constraints, Zod validation, auth timing safety
 */

import { test, describe } from "node:test";
import assert from "node:assert/strict";
import { timingSafeEqual } from "node:crypto";
import { GogomailClient } from "./clients/gogomail.js";
import { SuppoClient } from "./clients/suppo.js";
import { GithubClient } from "./clients/github.js";
import { toolDefinitions as gogomailDefs, callTool as gogomailCallTool } from "./tools/gogomail.js";
import { toolDefinitions as suppoDefs, callTool as suppoCallTool } from "./tools/suppo.js";
import { toolDefinitions as githubDefs, callTool as githubCallTool } from "./tools/github.js";
import type { Tool } from "@modelcontextprotocol/sdk/types.js";

// ── Helper ───────────────────────────────────────────────────────────

function findTool(defs: Tool[], name: string): Tool {
  const t = defs.find((d) => d.name === name);
  assert.ok(t, `Tool "${name}" not found in toolDefinitions`);
  return t!;
}

function prop(tool: Tool, field: string): Record<string, unknown> {
  const props = (tool.inputSchema as { properties?: Record<string, unknown> }).properties ?? {};
  assert.ok(props[field] !== undefined, `Field "${field}" not in ${tool.name}.inputSchema`);
  return props[field] as Record<string, unknown>;
}

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

// ── 1. Rate limiting algorithm ────────────────────────────────────────

describe("Rate limiting (sliding window)", () => {
  const WINDOW_MS = 60_000;
  const MAX = 100;

  // Exact replica of checkRateLimit from src/index.ts (module-internal)
  function makeRateLimiter() {
    const state = new Map<string, { count: number; windowStart: number }>();
    return (sessionId: string, now: number = Date.now()): boolean => {
      const s = state.get(sessionId);
      if (!s || now - s.windowStart >= WINDOW_MS) {
        state.set(sessionId, { count: 1, windowStart: now });
        return true;
      }
      if (s.count >= MAX) return false;
      s.count++;
      return true;
    };
  }

  test("allows exactly 100 requests in one window", () => {
    const rl = makeRateLimiter();
    const now = 1_000_000;
    for (let i = 0; i < 100; i++) {
      assert.equal(rl("s1", now), true, `request ${i + 1} should pass`);
    }
  });

  test("rejects the 101st request within the same window", () => {
    const rl = makeRateLimiter();
    const now = 1_000_000;
    for (let i = 0; i < 100; i++) rl("s1", now);
    assert.equal(rl("s1", now), false, "101st request must be rejected");
  });

  test("sessions are isolated — one session's limit does not affect another", () => {
    const rl = makeRateLimiter();
    const now = 1_000_000;
    for (let i = 0; i < 100; i++) rl("s1", now);
    assert.equal(rl("s2", now), true, "s2 is a fresh session, must be allowed");
  });

  test("counter resets after window expires", () => {
    const rl = makeRateLimiter();
    const t0 = 1_000_000;
    for (let i = 0; i < 100; i++) rl("s1", t0);
    assert.equal(rl("s1", t0), false, "must be blocked before window expires");
    assert.equal(rl("s1", t0 + WINDOW_MS + 1), true, "must be allowed after window expires");
  });

  test("partial count does not reset mid-window", () => {
    const rl = makeRateLimiter();
    const t0 = 1_000_000;
    for (let i = 0; i < 50; i++) rl("s1", t0);
    // Still within window but past some time
    for (let i = 0; i < 50; i++) rl("s1", t0 + 1000);
    assert.equal(rl("s1", t0 + 1000), false, "50+50=100 → 101st must be blocked");
  });
});

// ── 2. 5xx response body masking ─────────────────────────────────────

describe("GogomailClient: 5xx response body masking", () => {
  const client = new GogomailClient("https://internal.example.com", "test-key");

  test("masks 5xx response body — does not expose server internals to caller", async () => {
    const secretBody = "Traceback: DB_PASSWORD=supersecret, line 42 in /app/internal.py";
    const origFetch = globalThis.fetch;
    globalThis.fetch = async () =>
      new Response(secretBody, { status: 500 });

    try {
      await assert.rejects(
        () => (client as unknown as { request: (...a: unknown[]) => Promise<unknown> })
          .request("GET", "/test"),
        (err: Error) => {
          assert.ok(!err.message.includes(secretBody), "Error must not contain server body");
          assert.ok(err.message.includes("internal server error"), "Error must say 'internal server error'");
          return true;
        },
      );
    } finally {
      globalThis.fetch = origFetch;
    }
  });

  test("passes 4xx response body through — client errors are informative", async () => {
    const origFetch = globalThis.fetch;
    globalThis.fetch = async () =>
      new Response("Not found: mailbox does not exist", { status: 404 });

    try {
      await assert.rejects(
        () => (client as unknown as { request: (...a: unknown[]) => Promise<unknown> })
          .request("GET", "/test"),
        (err: Error) => {
          assert.ok(err.message.includes("Not found: mailbox does not exist"), "4xx body should be visible");
          return true;
        },
      );
    } finally {
      globalThis.fetch = origFetch;
    }
  });
});

describe("SuppoClient: 5xx response body masking", () => {
  const client = new SuppoClient("https://internal.example.com", "test-key");

  test("masks 5xx response body", async () => {
    const secretBody = "NullPointerException at com.suppo.internal.Handler.process:83";
    const origFetch = globalThis.fetch;
    globalThis.fetch = async () =>
      new Response(secretBody, { status: 503 });

    try {
      await assert.rejects(
        () => (client as unknown as { request: (...a: unknown[]) => Promise<unknown> })
          .request("GET", "/test"),
        (err: Error) => {
          assert.ok(!err.message.includes(secretBody), "5xx body must not leak");
          assert.ok(err.message.includes("internal server error"), "Must say 'internal server error'");
          return true;
        },
      );
    } finally {
      globalThis.fetch = origFetch;
    }
  });

  test("GET requests do not send Content-Type when there is no request body", async () => {
    const origFetch = globalThis.fetch;
    const contentTypes: (string | null)[] = [];
    globalThis.fetch = async (_input, init) => {
      contentTypes.push(new Headers(init?.headers).get("Content-Type"));
      return new Response(JSON.stringify([]), { status: 200, headers: { "Content-Type": "application/json" } });
    };

    try {
      await client.listAgents();
      assert.equal(contentTypes[0], null);
    } finally {
      globalThis.fetch = origFetch;
    }
  });
});

describe("GogomailClient: Admin API contract mapping", () => {
  test("GET requests do not send Content-Type when there is no request body", async () => {
    const client = new GogomailClient("https://admin.example.com", "test-key");
    const origFetch = globalThis.fetch;
    const contentTypes: (string | null)[] = [];
    globalThis.fetch = async (_input, init) => {
      contentTypes.push(new Headers(init?.headers).get("Content-Type"));
      return new Response(JSON.stringify({
        checks: [],
      }), { status: 200, headers: { "Content-Type": "application/json" } });
    };

    try {
      await client.checkHealth();
      assert.equal(contentTypes[0], null);
    } finally {
      globalThis.fetch = origFetch;
    }
  });

  test("getUser unwraps the { user } response envelope", async () => {
    const client = new GogomailClient("https://admin.example.com", "test-key");
    const origFetch = globalThis.fetch;
    globalThis.fetch = async () =>
      new Response(JSON.stringify({
        user: {
          id: "usr_1",
          domain_id: "dom_1",
          username: "alice",
          display_name: "Alice",
          role: "user",
          status: "active",
          password_configured: true,
          must_change_password: false,
          quota_used: 10,
          quota_limit: 100,
          quota_remaining: 90,
          quota_source: "custom",
          created_at: "2026-05-24T00:00:00Z",
        },
      }), { status: 200, headers: { "Content-Type": "application/json" } });

    try {
      const user = await client.getUser("usr_1");
      assert.equal(user.id, "usr_1");
      assert.equal(user.username, "alice");
      assert.equal(user.quota_limit, 100);
    } finally {
      globalThis.fetch = origFetch;
    }
  });

  test("updateUserQuota sends quota_limit and quota_source, then derives quota from refreshed user", async () => {
    const client = new GogomailClient("https://admin.example.com", "test-key");
    const origFetch = globalThis.fetch;
    const bodies: unknown[] = [];
    globalThis.fetch = async (_input, init) => {
      if (init?.method === "PATCH") {
        bodies.push(JSON.parse(String(init.body)));
        return new Response(JSON.stringify({ status: "ok", id: "usr_1" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        });
      }
      return new Response(JSON.stringify({
        user: {
          id: "usr_1",
          domain_id: "dom_1",
          username: "alice",
          display_name: "Alice",
          role: "user",
          status: "active",
          password_configured: true,
          must_change_password: false,
          quota_used: 25,
          quota_limit: 200,
          quota_remaining: 175,
          quota_source: "custom",
          created_at: "2026-05-24T00:00:00Z",
        },
      }), { status: 200, headers: { "Content-Type": "application/json" } });
    };

    try {
      const quota = await client.updateUserQuota("usr_1", 200);
      assert.deepEqual(bodies[0], { quota_limit: 200, quota_source: "custom" });
      assert.equal(quota.allocatedBytes, 200);
      assert.equal(quota.usedBytes, 25);
    } finally {
      globalThis.fetch = origFetch;
    }
  });

  test("adminRequest preserves non-JSON Admin API responses for export endpoints", async () => {
    const client = new GogomailClient("https://admin.example.com", "test-key");
    const origFetch = globalThis.fetch;
    globalThis.fetch = async () =>
      new Response("email,status\nalice@example.com,active\n", {
        status: 200,
        headers: { "Content-Type": "text/csv" },
      });

    try {
      const res = await client.adminRequest("GET", "/companies/co_1/users/bulk-export");
      assert.deepEqual(res, {
        content_type: "text/csv",
        body_text: "email,status\nalice@example.com,active\n",
      });
    } finally {
      globalThis.fetch = origFetch;
    }
  });
});

// ── 3. inputSchema JSON Schema constraints ────────────────────────────

describe("all toolDefinitions: MCP client schema compatibility", () => {
  test("all advertised tools expose conservative object schemas", () => {
    for (const tool of [...gogomailDefs, ...suppoDefs, ...githubDefs]) {
      const schema = tool.inputSchema as Record<string, unknown>;
      assert.equal(schema.type, "object", `${tool.name} must expose an object input schema`);
      for (const keyword of ["oneOf", "anyOf", "allOf", "enum", "not"]) {
        assert.equal(Object.hasOwn(schema, keyword), false, `${tool.name} must not expose top-level ${keyword}`);
      }
      assertNoSchemaCombinators(schema, tool.name);
    }
  });
});

describe("gogomail toolDefinitions: ID field constraints", () => {
  test("gogomail_get_user: userId has pattern and maxLength", () => {
    const p = prop(findTool(gogomailDefs, "gogomail_get_user"), "userId");
    assert.equal(p["pattern"], "^[A-Za-z0-9_-]+$");
    assert.equal(p["maxLength"], 128);
  });

  test("gogomail_get_domain_settings: domainId has pattern and maxLength", () => {
    const p = prop(findTool(gogomailDefs, "gogomail_get_domain_settings"), "domainId");
    assert.equal(p["pattern"], "^[A-Za-z0-9_-]+$");
    assert.equal(p["maxLength"], 128);
  });
});

describe("gogomail toolDefinitions: email field constraints", () => {
  test("gogomail_create_user: fromAddr has format=email and maxLength", () => {
    const tool = findTool(gogomailDefs, "gogomail_create_user");
    // email field in create_user
    const props = (tool.inputSchema as { properties?: Record<string, Record<string, unknown>> }).properties ?? {};
    const emailField = Object.entries(props).find(([, v]) => (v as Record<string, unknown>)["format"] === "email");
    assert.ok(emailField, "At least one field must have format=email");
    assert.equal((emailField[1] as Record<string, unknown>)["maxLength"], 254);
  });
});

describe("gogomail toolDefinitions: limit field constraints", () => {
  test("gogomail_list_users: limit has minimum=1, maximum=200", () => {
    const p = prop(findTool(gogomailDefs, "gogomail_list_users"), "limit");
    assert.equal(p["minimum"], 1);
    assert.equal(p["maximum"], 200);
  });

  test("gogomail_list_domains: limit has minimum=1, maximum=200", () => {
    const p = prop(findTool(gogomailDefs, "gogomail_list_domains"), "limit");
    assert.equal(p["minimum"], 1);
    assert.equal(p["maximum"], 200);
  });

  test("gogomail_list_dlq: count has minimum=1, maximum=500", () => {
    const p = prop(findTool(gogomailDefs, "gogomail_list_dlq"), "count");
    assert.equal(p["minimum"], 1);
    assert.equal(p["maximum"], 500);
  });
});

describe("gogomail toolDefinitions: enum constraints", () => {
  test("gogomail_list_users: status is an enum", () => {
    const p = prop(findTool(gogomailDefs, "gogomail_list_users"), "status");
    assert.ok(Array.isArray(p["enum"]), "status must have enum");
    assert.ok((p["enum"] as string[]).includes("active"));
    assert.ok((p["enum"] as string[]).includes("suspended"));
  });

  test("gogomail_list_mail_flow_logs: direction is an enum", () => {
    const p = prop(findTool(gogomailDefs, "gogomail_list_mail_flow_logs"), "direction");
    assert.ok(Array.isArray(p["enum"]), "direction must have enum");
    assert.ok((p["enum"] as string[]).includes("inbound"));
    assert.ok((p["enum"] as string[]).includes("outbound"));
  });
});

describe("suppo toolDefinitions: constraints", () => {
  test("suppo_get_ticket: ticketId has pattern and maxLength", () => {
    const p = prop(findTool(suppoDefs, "suppo_get_ticket"), "ticketId");
    assert.equal(p["pattern"], "^[A-Za-z0-9_-]+$");
    assert.equal(p["maxLength"], 128);
  });

  test("suppo_list_tickets: status is an enum", () => {
    const p = prop(findTool(suppoDefs, "suppo_list_tickets"), "status");
    assert.ok(Array.isArray(p["enum"]));
    assert.deepEqual(p["enum"], ["open", "pending", "resolved", "closed"]);
  });

  test("suppo_list_tickets: limit has minimum and maximum", () => {
    const p = prop(findTool(suppoDefs, "suppo_list_tickets"), "limit");
    assert.equal(p["minimum"], 1);
    assert.equal(p["maximum"], 200);
  });

  test("suppo_create_ticket: customerEmail has format=email", () => {
    const p = prop(findTool(suppoDefs, "suppo_create_ticket"), "customerEmail");
    assert.equal(p["format"], "email");
    assert.equal(p["maxLength"], 254);
  });
});

describe("github toolDefinitions: constraints", () => {
  test("github_get_issue: issueNumber has minimum=1", () => {
    const p = prop(findTool(githubDefs, "github_get_issue"), "issueNumber");
    assert.equal(p["minimum"], 1);
  });

  test("github_search_issues: state is an enum with open/closed/all", () => {
    const p = prop(findTool(githubDefs, "github_search_issues"), "state");
    assert.deepEqual(p["enum"], ["open", "closed", "all"]);
  });

  test("github_create_issue: labels has maxItems=20", () => {
    const p = prop(findTool(githubDefs, "github_create_issue"), "labels");
    assert.equal(p["maxItems"], 20);
  });

  test("github_update_issue: state enum excludes 'all' (only open/closed)", () => {
    const p = prop(findTool(githubDefs, "github_update_issue"), "state");
    assert.deepEqual(p["enum"], ["open", "closed"]);
    assert.ok(!(p["enum"] as string[]).includes("all"), "'all' must not be valid for update");
  });

  test("gogomail_delete_user: requires confirm and reason", () => {
    const tool = findTool(gogomailDefs, "gogomail_delete_user");
    const required = (tool.inputSchema as { required?: string[] }).required ?? [];
    assert.ok(required.includes("confirm"));
    assert.ok(required.includes("reason"));
  });

  test("gogomail_update_user_role: role enum is constrained", () => {
    const p = prop(findTool(gogomailDefs, "gogomail_update_user_role"), "role");
    assert.deepEqual(p["enum"], ["user", "company_admin", "system_admin"]);
  });
});

describe("SSE HTTP request gates", () => {
  function isMessagesPath(rawUrl: string): boolean {
    return new URL(rawUrl, "http://localhost").pathname === "/messages";
  }

  function isJsonContentType(value: string): boolean {
    const mediaType = value.split(";", 1)[0]?.trim().toLowerCase();
    return mediaType === "application/json";
  }

  test("messages route matching is exact, not prefix-based", () => {
    assert.equal(isMessagesPath("/messages?sessionId=s1"), true);
    assert.equal(isMessagesPath("/messages-extra?sessionId=s1"), false);
    assert.equal(isMessagesPath("/messages/extra?sessionId=s1"), false);
  });

  test("Content-Type validation accepts only application/json media type", () => {
    assert.equal(isJsonContentType("application/json"), true);
    assert.equal(isJsonContentType("application/json; charset=utf-8"), true);
    assert.equal(isJsonContentType("text/plain application/json"), false);
    assert.equal(isJsonContentType("application/jsonp"), false);
  });
});

describe("GithubClient: repository scope hardening", () => {
  test("searchIssues ignores user-provided repo/org/user qualifiers and searches issues only", async () => {
    const client = new GithubClient("token", "owner/repo");
    let capturedQuery = "";
    (client as unknown as {
      octokit: {
        search: {
          issuesAndPullRequests: (params: { q: string }) => Promise<{ data: { items: [] } }>;
        };
      };
    }).octokit = {
      search: {
        issuesAndPullRequests: async ({ q }) => {
          capturedQuery = q;
          return { data: { items: [] } };
        },
      },
    };

    await client.searchIssues({
      query: "repo:evil/other org:evil user:attacker delivery failure",
      state: "open",
    });

    assert.ok(capturedQuery.includes("repo:owner/repo"));
    assert.ok(capturedQuery.includes("is:issue"));
    assert.ok(capturedQuery.includes("delivery failure"));
    assert.ok(!capturedQuery.includes("repo:evil/other"));
    assert.ok(!capturedQuery.includes("org:evil"));
    assert.ok(!capturedQuery.includes("user:attacker"));
  });
});

// ── 4. Zod runtime validation via callTool ────────────────────────────

describe("Zod validation: rejects out-of-range input before hitting API", () => {
  const mockGogomail = {} as InstanceType<typeof GogomailClient>;
  const mockSuppo = {} as InstanceType<typeof SuppoClient>;

  test("gogomail_list_users: limit > 200 throws ZodError", async () => {
    await assert.rejects(
      () => gogomailCallTool(mockGogomail, mockSuppo, "gogomail_list_users", { limit: 201 }),
      (err: Error) => {
        assert.ok(err.constructor.name === "ZodError" || err.message.toLowerCase().includes("number must be less than or equal to 200"),
          `Expected ZodError, got: ${err.message}`);
        return true;
      },
    );
  });

  test("gogomail_list_users: limit=0 throws ZodError", async () => {
    await assert.rejects(
      () => gogomailCallTool(mockGogomail, mockSuppo, "gogomail_list_users", { limit: 0 }),
      (err: Error) => {
        assert.ok(err.constructor.name === "ZodError" || err.message.includes("1"),
          "Should reject limit < 1");
        return true;
      },
    );
  });

  test("suppo_get_ticket: ticketId with path separators throws ZodError (path traversal blocked)", async () => {
    await assert.rejects(
      () => suppoCallTool(mockSuppo, "suppo_get_ticket", { ticketId: "../../../etc/passwd" }),
      (err: Error) => {
        // ZodError from regex validation — must NOT reach the client
        assert.ok(err.constructor.name === "ZodError",
          `Expected ZodError for path traversal attempt, got: ${err.constructor.name}: ${err.message}`);
        return true;
      },
    );
  });

  test("suppo_list_tickets: limit > 200 throws ZodError", async () => {
    await assert.rejects(
      () => suppoCallTool(mockSuppo, "suppo_list_tickets", { limit: 999 }),
      (err: Error) => {
        assert.ok(err.constructor.name === "ZodError",
          `Expected ZodError, got: ${err.message}`);
        return true;
      },
    );
  });

  test("gogomail_update_user_role: rejects unknown roles before hitting API", async () => {
    await assert.rejects(
      () => gogomailCallTool(mockGogomail, mockSuppo, "gogomail_update_user_role", {
        userId: "usr_1",
        role: "root",
        reason: "test",
      }),
      (err: Error) => {
        assert.equal(err.constructor.name, "ZodError");
        return true;
      },
    );
  });

  test("gogomail_delete_user: rejects missing confirmation before hitting API", async () => {
    await assert.rejects(
      () => gogomailCallTool(mockGogomail, mockSuppo, "gogomail_delete_user", {
        userId: "usr_1",
        reason: "test",
      }),
      (err: Error) => {
        assert.equal(err.constructor.name, "ZodError");
        return true;
      },
    );
  });

  test("gogomail_delete_user: rejects mismatched confirmation before hitting API", async () => {
    await assert.rejects(
      () => gogomailCallTool(mockGogomail, mockSuppo, "gogomail_delete_user", {
        userId: "usr_1",
        confirm: "delete usr_2",
        reason: "test",
      }),
      /confirm must exactly equal "delete usr_1"/,
    );
  });

  test("gogomail_list_mail_flow_logs: rejects incomplete date strings", async () => {
    await assert.rejects(
      () => gogomailCallTool(mockGogomail, mockSuppo, "gogomail_list_mail_flow_logs", {
        since: "2026-05-24",
      }),
      (err: Error) => {
        assert.equal(err.constructor.name, "ZodError");
        return true;
      },
    );
  });

  test("gogomail_list_mail_flow_logs: rejects inverted time ranges", async () => {
    await assert.rejects(
      () => gogomailCallTool(mockGogomail, mockSuppo, "gogomail_list_mail_flow_logs", {
        since: "2026-05-25T00:00:00Z",
        until: "2026-05-24T00:00:00Z",
      }),
      (err: Error) => {
        assert.equal(err.constructor.name, "ZodError");
        return true;
      },
    );
  });

  test("gogomail_update_domain_settings: rejects invalid IP allowlist entries", async () => {
    await assert.rejects(
      () => gogomailCallTool(mockGogomail, mockSuppo, "gogomail_update_domain_settings", {
        domainId: "dom_1",
        settings: { ip_whitelist: ["999.1.1.1"] },
        reason: "test",
      }),
      (err: Error) => {
        assert.equal(err.constructor.name, "ZodError");
        return true;
      },
    );
  });

  test("gogomail_admin_api_request rejects routes outside the admin console allowlist", async () => {
    await assert.rejects(
      () => gogomailCallTool(mockGogomail, mockSuppo, "gogomail_admin_api_request", {
        method: "GET",
        path: "/auth/login",
      }),
      /not in the documented MCP allowlist/,
    );
  });

  test("gogomail_admin_api_request requires reason for writes", async () => {
    await assert.rejects(
      () => gogomailCallTool(mockGogomail, mockSuppo, "gogomail_admin_api_request", {
        method: "PATCH",
        path: "/users/usr_1/status",
        bodyJson: { status: "suspended" },
      }),
      /reason is required/,
    );
  });

  test("gogomail_admin_api_request requires exact DELETE confirmation", async () => {
    await assert.rejects(
      () => gogomailCallTool(mockGogomail, mockSuppo, "gogomail_admin_api_request", {
        method: "DELETE",
        path: "/organization/members/mem_1",
        reason: "test",
        confirm: "delete mem_1",
      }),
      /confirm must exactly equal "DELETE \/organization\/members\/mem_1"/,
    );
  });

  test("gogomail_assign_user_org_membership maps to organization members API and audits", async () => {
    const calls: unknown[] = [];
    const mockAdmin = {
      adminRequest: async (method: string, path: string, body?: unknown) => {
        calls.push({ method, path, body });
        return { membership: { id: "mem_1" } };
      },
    } as InstanceType<typeof GogomailClient>;

    const result = await gogomailCallTool(mockAdmin, null, "gogomail_assign_user_org_membership", {
      unitId: "ou_1",
      userId: "usr_1",
      role: "lead",
      title: "Director",
      reason: "admin console parity test",
    });

    assert.deepEqual(calls[0], {
      method: "POST",
      path: "/organization/members",
      body: {
        unit_id: "ou_1",
        user_id: "usr_1",
        role: "lead",
        title: "Director",
      },
    });
    assert.equal((result as { audit?: { destination?: string } }).audit?.destination, "stderr");
  });

  test("suppo_update_ticket: rejects empty updates before hitting API", async () => {
    await assert.rejects(
      () => suppoCallTool(mockSuppo, "suppo_update_ticket", { ticketId: "t_1" }),
      (err: Error) => {
        assert.equal(err.constructor.name, "ZodError");
        return true;
      },
    );
  });

  test("suppo_add_comment: defaults to internal=true", async () => {
    let captured: unknown;
    const suppo = {
      addComment: async (_ticketId: string, data: unknown) => {
        captured = data;
        return { id: "c_1", ticketId: "t_1", body: "memo", internal: true, authorId: null, createdAt: "now" };
      },
    } as InstanceType<typeof SuppoClient>;

    await suppoCallTool(suppo, "suppo_add_comment", { ticketId: "t_1", body: "memo" });
    assert.deepEqual(captured, { body: "memo", internal: true });
  });

  test("github_update_issue: rejects empty updates before hitting API", async () => {
    const mockGithub = {} as Parameters<typeof githubCallTool>[0];
    await assert.rejects(
      () => githubCallTool(mockGithub, "github_update_issue", { issueNumber: 1 }),
      (err: Error) => {
        assert.equal(err.constructor.name, "ZodError");
        return true;
      },
    );
  });
});

// ── 5. Auth: timingSafeEqual constant-time comparison ─────────────────

describe("Bearer token auth: timing-safe comparison", () => {
  // Replica of checkAuth logic from src/index.ts
  function checkAuth(auth: string, secret: string): boolean {
    const expected = `Bearer ${secret}`;
    try {
      const a = Buffer.from(auth);
      const b = Buffer.from(expected);
      return a.length === b.length && timingSafeEqual(a, b);
    } catch {
      return false;
    }
  }

  test("correct token is accepted", () => {
    assert.equal(checkAuth("Bearer my-secret-token", "my-secret-token"), true);
  });

  test("wrong token is rejected", () => {
    assert.equal(checkAuth("Bearer wrong-token-value", "my-secret-token"), false);
  });

  test("token with same length but different content is rejected", () => {
    const secret = "aaaa";
    const attack = "aaab"; // same length
    assert.equal(checkAuth(`Bearer ${attack}`, secret), false);
  });

  test("empty auth header is rejected", () => {
    assert.equal(checkAuth("", "my-secret-token"), false);
  });

  test("different-length token is rejected without calling timingSafeEqual", () => {
    // timingSafeEqual would throw on different-length buffers, so length check must come first
    assert.equal(checkAuth("Bearer short", "my-very-long-secret-token"), false);
  });
});

// ── 6. Error sanitization ─────────────────────────────────────────────

describe("Error sanitization: truncates at 500 chars", () => {
  // Replica of sanitizeError from src/index.ts
  function sanitizeError(err: unknown): string {
    const raw = err instanceof Error ? err.message : String(err);
    return raw.length > 500 ? raw.slice(0, 500) + " [truncated]" : raw;
  }

  test("short error passes through unchanged", () => {
    const result = sanitizeError(new Error("short error"));
    assert.equal(result, "short error");
  });

  test("long error is truncated at 500 chars with [truncated] suffix", () => {
    const long = "x".repeat(600);
    const result = sanitizeError(new Error(long));
    assert.equal(result.length, 500 + " [truncated]".length);
    assert.ok(result.endsWith("[truncated]"));
  });

  test("non-Error values are stringified", () => {
    assert.equal(sanitizeError("plain string error"), "plain string error");
    assert.equal(sanitizeError(42), "42");
  });
});

// ── 7. Session TTL cleanup logic ──────────────────────────────────────

describe("Session TTL: idle session eviction", () => {
  const SESSION_TTL_MS = 30 * 60_000;

  function makeEvictIdleSessions() {
    const sessionActivity = new Map<string, number>();
    const evicted: string[] = [];

    function tick(now: number) {
      for (const [id, lastActive] of sessionActivity) {
        if (now - lastActive > SESSION_TTL_MS) {
          sessionActivity.delete(id);
          evicted.push(id);
        }
      }
    }

    return { sessionActivity, evicted, tick };
  }

  test("active session is not evicted before TTL", () => {
    const { sessionActivity, evicted, tick } = makeEvictIdleSessions();
    sessionActivity.set("active-session", 0);
    tick(SESSION_TTL_MS - 1);
    assert.equal(evicted.length, 0, "Should not evict before TTL");
  });

  test("idle session is evicted after TTL expires", () => {
    const { sessionActivity, evicted, tick } = makeEvictIdleSessions();
    sessionActivity.set("idle-session", 0);
    tick(SESSION_TTL_MS + 1);
    assert.ok(evicted.includes("idle-session"), "Idle session must be evicted");
    assert.equal(sessionActivity.size, 0, "sessionActivity map must be cleaned up");
  });

  test("recently-active session survives eviction run", () => {
    const { sessionActivity, evicted, tick } = makeEvictIdleSessions();
    const now = 10_000_000;
    sessionActivity.set("old-session", 0);
    sessionActivity.set("new-session", now - 1000); // active 1 second ago
    tick(now);
    assert.ok(evicted.includes("old-session"), "Old session must be evicted");
    assert.ok(!evicted.includes("new-session"), "Recent session must survive");
    assert.equal(sessionActivity.size, 1);
  });
});
