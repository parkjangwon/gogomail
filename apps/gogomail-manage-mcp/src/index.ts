import { timingSafeEqual } from "crypto";
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import type { IncomingMessage, ServerResponse } from "http";
import { config } from "./config.js";
import { GogomailClient } from "./clients/gogomail.js";
import { SuppoClient } from "./clients/suppo.js";
import { GithubClient } from "./clients/github.js";
import * as suppoTools from "./tools/suppo.js";
import * as gogomailTools from "./tools/gogomail.js";
import * as githubTools from "./tools/github.js";

// ── Constants ────────────────────────────────────────────────────

const MAX_BODY_BYTES = 1 * 1024 * 1024; // 1 MB
const MAX_SESSIONS = 100; // prevent session map unbounded growth
const MAX_SESSION_ID_LENGTH = 128;
const SENSITIVE_ARG_KEYS = new Set(["password", "token", "apiKey", "secret", "key"]);
const RATE_LIMIT_WINDOW_MS = 60_000;   // 1-minute sliding window
const RATE_LIMIT_MAX = 100;            // max tool calls per session per window
const SESSION_TTL_MS = 30 * 60_000;   // evict sessions idle for 30 min

const rateLimitState = new Map<string, { count: number; windowStart: number }>();
const sessionActivity = new Map<string, number>();
let isShuttingDown = false;

// ── Clients ──────────────────────────────────────────────────────

const gogomailClient = new GogomailClient(
  config.gogomail.adminUrl,
  config.gogomail.adminKey,
);

const suppoClient =
  config.suppo.apiUrl && config.suppo.apiKey
    ? new SuppoClient(config.suppo.apiUrl, config.suppo.apiKey)
    : null;

const githubClient = config.github.token
  ? new GithubClient(config.github.token, config.github.repo)
  : null;

if (!suppoClient) {
  console.error(
    "[gogomail-manage-mcp] Suppo not configured — helpdesk tools disabled; audit trail will be logged to stderr only",
  );
}
if (!githubClient) {
  console.error(
    "[gogomail-manage-mcp] GitHub not configured — GitHub Issues tools disabled",
  );
}
if (config.mcpSecret) {
  console.error("[gogomail-manage-mcp] MCP_SECRET is set — SSE endpoints require Bearer auth");
}

// ── Auth middleware ───────────────────────────────────────────────

function getSingleHeader(req: IncomingMessage, name: string): string | undefined {
  const val = req.headers[name.toLowerCase()];
  if (Array.isArray(val)) {
    throw new Error(`duplicate ${name} headers are not allowed`);
  }
  return val;
}

function checkAuth(req: IncomingMessage, res: ServerResponse): boolean {
  if (!config.mcpSecret) return true;
  let auth = "";
  try {
    auth = getSingleHeader(req, "authorization") ?? "";
  } catch {
    res.writeHead(400, securityHeaders()).end("Bad Request");
    return false;
  }
  const expected = `Bearer ${config.mcpSecret}`;
  // Constant-time comparison prevents timing-based token enumeration
  let valid = false;
  try {
    const a = Buffer.from(auth);
    const b = Buffer.from(expected);
    // timingSafeEqual requires equal-length buffers; mismatched length → reject
    valid = a.length === b.length && timingSafeEqual(a, b);
  } catch {
    valid = false;
  }
  if (!valid) {
    res.writeHead(401, { "Content-Type": "application/json", "X-Content-Type-Options": "nosniff" })
      .end(JSON.stringify({ error: "Unauthorized" }));
    return false;
  }
  return true;
}

function checkOrigin(req: IncomingMessage, res: ServerResponse): boolean {
  let origin: string | undefined;
  try {
    origin = getSingleHeader(req, "origin");
  } catch {
    res.writeHead(400, securityHeaders()).end("Bad Request");
    return false;
  }
  if (!origin) return true;
  if (!config.mcpAllowedOrigins.includes(origin)) {
    res.writeHead(403, securityHeaders()).end("Forbidden origin");
    return false;
  }
  return true;
}

function parseRequestUrl(req: IncomingMessage): URL {
  return new URL(req.url ?? "/", "http://localhost");
}

function isJsonContentType(value: string): boolean {
  const mediaType = value.split(";", 1)[0]?.trim().toLowerCase();
  return mediaType === "application/json";
}

// ── Security headers ─────────────────────────────────────────────

function securityHeaders(): Record<string, string> {
  return {
    "X-Content-Type-Options": "nosniff",
    "X-Frame-Options": "DENY",
    "Cache-Control": "no-store",
    "Referrer-Policy": "no-referrer",
    "Permissions-Policy": "interest-cohort=()",
  };
}

// ── Error sanitization ───────────────────────────────────────────

function sanitizeError(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err);
  // Truncate to 500 chars to prevent info leakage from verbose upstream errors
  return raw.length > 500 ? raw.slice(0, 500) + " [truncated]" : raw;
}

// ── Rate limiter (SSE only) ──────────────────────────────────────
function checkRateLimit(sessionId: string): boolean {
  const now = Date.now();
  const state = rateLimitState.get(sessionId);
  if (!state || now - state.windowStart >= RATE_LIMIT_WINDOW_MS) {
    rateLimitState.set(sessionId, { count: 1, windowStart: now });
    return true;
  }
  if (state.count >= RATE_LIMIT_MAX) return false;
  state.count++;
  return true;
}

// ── MCP server ───────────────────────────────────────────────────

const allTools = [
  ...suppoTools.toolDefinitions,
  ...gogomailTools.toolDefinitions,
  ...githubTools.toolDefinitions,
];

const server = new Server(
  { name: "gogomail-manage-mcp", version: "1.0.0" },
  { capabilities: { tools: {} } },
);

server.setRequestHandler(ListToolsRequestSchema, async () => ({
  tools: allTools,
}));

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args = {} } = request.params;
  const typedArgs = args as Record<string, unknown>;

  try {
    // Log tool invocations — field names only, values are never logged (may contain credentials)
    const logKeys = Object.keys(typedArgs).filter(k => !SENSITIVE_ARG_KEYS.has(k));
    console.error(`[tool] ${name}${logKeys.length ? ` [${logKeys.join(", ")}]` : ""}`);

    let result: unknown;

    if (name.startsWith("suppo_")) {
      result = await suppoTools.callTool(suppoClient, name, typedArgs);
    } else if (name.startsWith("gogomail_")) {
      result = await gogomailTools.callTool(
        gogomailClient,
        suppoClient,
        name,
        typedArgs,
      );
    } else if (name.startsWith("github_")) {
      result = await githubTools.callTool(githubClient, name, typedArgs);
    } else {
      throw new Error(`Unknown tool: ${name}`);
    }

    return {
      content: [{ type: "text", text: JSON.stringify(result, null, 2) }],
    };
  } catch (err) {
    return {
      content: [{ type: "text", text: `Error: ${sanitizeError(err)}` }],
      isError: true,
    };
  }
});

// ── Transport ────────────────────────────────────────────────────

async function main(): Promise<void> {
  if (config.transport === "sse") {
    const { SSEServerTransport } = await import(
      "@modelcontextprotocol/sdk/server/sse.js"
    );
    const { createServer } = await import("http");

    const sessions = new Map<string, InstanceType<typeof SSEServerTransport>>();

    const httpServer = createServer((req, res) => {
      const url = parseRequestUrl(req);

      // GET /sse — establish SSE stream
      if (req.method === "GET" && url.pathname === "/sse" && url.search === "") {
        if (!checkAuth(req, res)) return;
        if (!checkOrigin(req, res)) return;

        if (isShuttingDown) {
          res.writeHead(503, securityHeaders()).end("Server is shutting down");
          return;
        }

        if (sessions.size >= MAX_SESSIONS) {
          res.writeHead(503, securityHeaders()).end("Too many sessions");
          return;
        }

        const transport = new SSEServerTransport("/messages", res);
        sessions.set(transport.sessionId, transport);
        server.connect(transport).catch(console.error);
        sessionActivity.set(transport.sessionId, Date.now());
        req.on("close", () => {
          sessions.delete(transport.sessionId);
          sessionActivity.delete(transport.sessionId);
          rateLimitState.delete(transport.sessionId);
        });
        return;
      }

      // POST /messages — tool call from agent
      if (req.method === "POST" && url.pathname === "/messages") {
        if (!checkAuth(req, res)) return;
        if (!checkOrigin(req, res)) return;

        // Validate Content-Type
        let ct = "";
        try {
          ct = getSingleHeader(req, "content-type") ?? "";
        } catch {
          res.writeHead(400, securityHeaders()).end("Bad Request");
          return;
        }
        if (!isJsonContentType(ct)) {
          res.writeHead(415, securityHeaders()).end("Unsupported Media Type — use application/json");
          return;
        }

        const sessionId = url.searchParams.get("sessionId") ?? "";

        // Reject obviously invalid session IDs before Map lookup
        if (!sessionId || sessionId.length > MAX_SESSION_ID_LENGTH || !/^[\w-]+$/.test(sessionId)) {
          res.writeHead(400, securityHeaders()).end("Invalid sessionId");
          return;
        }

        const transport = sessions.get(sessionId);
        if (!transport) {
          res.writeHead(404, securityHeaders()).end("Session not found");
          return;
        }

        if (!checkRateLimit(sessionId)) {
          res.writeHead(429, { ...securityHeaders(), "Retry-After": "60" }).end("Too Many Requests");
          return;
        }
        sessionActivity.set(sessionId, Date.now());

        // Enforce body size limit to prevent OOM
        const chunks: Buffer[] = [];
        let bytesRead = 0;
        let limitExceeded = false;

        req.on("data", (chunk: Buffer) => {
          if (limitExceeded) return;
          bytesRead += chunk.length;
          if (bytesRead > MAX_BODY_BYTES) {
            limitExceeded = true;
            res.writeHead(413, securityHeaders()).end("Payload Too Large");
            req.destroy();
            return;
          }
          chunks.push(chunk);
        });

        req.on("end", () => {
          if (limitExceeded) return;
          const body = Buffer.concat(chunks, bytesRead).toString("utf8");
          transport.handlePostMessage(req, res, body).catch((e: unknown) => {
            console.error("[gogomail-manage-mcp] handlePostMessage error:", e instanceof Error ? e.message : String(e));
            if (!res.headersSent) {
              res.writeHead(500, securityHeaders()).end("Internal Server Error");
            }
          });
        });

        req.on("error", () => {
          if (!res.headersSent) {
            res.writeHead(400, securityHeaders()).end("Bad Request");
          }
        });

        return;
      }

      res.writeHead(404, securityHeaders()).end();
    });

    httpServer.listen(config.port, config.host, () => {
      console.error(
        `[gogomail-manage-mcp] SSE transport listening on ${config.host}:${config.port}`,
      );
    });

    setInterval(() => {
      const now = Date.now();
      for (const [id, lastActive] of sessionActivity) {
        if (now - lastActive > SESSION_TTL_MS) {
          sessions.delete(id);
          sessionActivity.delete(id);
          rateLimitState.delete(id);
          console.error(`[gogomail-manage-mcp] Session ${id.slice(0, 8)}… evicted (idle TTL)`);
        }
      }
    }, 60_000).unref();

    const shutdown = () => {
      if (isShuttingDown) return;
      isShuttingDown = true;
      console.error("[gogomail-manage-mcp] Shutdown signal received — draining connections…");
      httpServer.close(() => {
        console.error("[gogomail-manage-mcp] HTTP server closed");
        process.exit(0);
      });
      setTimeout(() => {
        console.error("[gogomail-manage-mcp] Shutdown timeout exceeded, force-exiting");
        process.exit(1);
      }, 30_000).unref();
    };
    process.on("SIGTERM", shutdown);
    process.on("SIGINT", shutdown);
  } else {
    const transport = new StdioServerTransport();
    await server.connect(transport);
    console.error("[gogomail-manage-mcp] stdio transport ready");
  }
}

main().catch((err) => {
  console.error("[gogomail-manage-mcp] Fatal error:", err instanceof Error ? err.message : String(err));
  process.exit(1);
});
