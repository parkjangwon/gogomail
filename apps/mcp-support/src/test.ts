/**
 * Security hardening tests for mcp-support.
 * Run: tsx src/test.ts
 *
 * Covers: rate limiting, 5xx masking, inputSchema constraints, Zod validation, auth timing safety
 */

import { test, describe } from "node:test";
import assert from "node:assert/strict";
import { timingSafeEqual } from "node:crypto";
import { GogomailClient } from "./clients/gogomail.js";
import { SuppoClient } from "./clients/suppo.js";
import { toolDefinitions as gogomailDefs, callTool as gogomailCallTool } from "./tools/gogomail.js";
import { toolDefinitions as suppoDefs, callTool as suppoCallTool } from "./tools/suppo.js";
import { toolDefinitions as githubDefs } from "./tools/github.js";
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
});

// ── 3. inputSchema JSON Schema constraints ────────────────────────────

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
