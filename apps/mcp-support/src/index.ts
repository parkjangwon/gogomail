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
    "[mcp-support] Suppo not configured — helpdesk tools disabled; audit trail will be logged to stderr only",
  );
}
if (!githubClient) {
  console.error(
    "[mcp-support] GitHub not configured — GitHub Issues tools disabled",
  );
}
if (config.mcpSecret) {
  console.error("[mcp-support] MCP_SECRET is set — SSE endpoints require Bearer auth");
} else if (config.transport === "sse") {
  console.error("[mcp-support] WARNING: MCP_SECRET is not set — SSE endpoint is unauthenticated");
}

// ── Auth middleware ───────────────────────────────────────────────

function checkAuth(req: IncomingMessage, res: ServerResponse): boolean {
  if (!config.mcpSecret) return true;
  const auth = req.headers["authorization"];
  if (!auth || auth !== `Bearer ${config.mcpSecret}`) {
    res.writeHead(401, { "Content-Type": "application/json", "X-Content-Type-Options": "nosniff" })
      .end(JSON.stringify({ error: "Unauthorized" }));
    return false;
  }
  return true;
}

// ── Security headers ─────────────────────────────────────────────

function securityHeaders(): Record<string, string> {
  return {
    "X-Content-Type-Options": "nosniff",
    "X-Frame-Options": "DENY",
    "Cache-Control": "no-store",
  };
}

// ── Error sanitization ───────────────────────────────────────────

function sanitizeError(err: unknown): string {
  const raw = err instanceof Error ? err.message : String(err);
  // Truncate to 500 chars to prevent info leakage from verbose upstream errors
  return raw.length > 500 ? raw.slice(0, 500) + " [truncated]" : raw;
}

// ── MCP server ───────────────────────────────────────────────────

const allTools = [
  ...suppoTools.toolDefinitions,
  ...gogomailTools.toolDefinitions,
  ...githubTools.toolDefinitions,
];

const server = new Server(
  { name: "gogomail-support", version: "1.0.0" },
  { capabilities: { tools: {} } },
);

server.setRequestHandler(ListToolsRequestSchema, async () => ({
  tools: allTools,
}));

server.setRequestHandler(CallToolRequestSchema, async (request) => {
  const { name, arguments: args = {} } = request.params;
  const typedArgs = args as Record<string, unknown>;

  try {
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
      // GET /sse — establish SSE stream
      if (req.method === "GET" && req.url === "/sse") {
        if (!checkAuth(req, res)) return;

        if (sessions.size >= MAX_SESSIONS) {
          res.writeHead(503, securityHeaders()).end("Too many sessions");
          return;
        }

        const transport = new SSEServerTransport("/messages", res);
        sessions.set(transport.sessionId, transport);
        server.connect(transport).catch(console.error);
        req.on("close", () => sessions.delete(transport.sessionId));
        return;
      }

      // POST /messages — tool call from agent
      if (req.method === "POST" && req.url?.startsWith("/messages")) {
        if (!checkAuth(req, res)) return;

        // Validate Content-Type
        const ct = req.headers["content-type"] ?? "";
        if (!ct.includes("application/json")) {
          res.writeHead(415, securityHeaders()).end("Unsupported Media Type — use application/json");
          return;
        }

        const url = new URL(req.url, `http://localhost`);
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

        // Enforce body size limit to prevent OOM
        let body = "";
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
          body += chunk;
        });

        req.on("end", () => {
          if (limitExceeded) return;
          transport.handlePostMessage(req, res, body).catch((e: unknown) => {
            console.error("[mcp-support] handlePostMessage error:", e instanceof Error ? e.message : String(e));
          });
        });

        return;
      }

      res.writeHead(404, securityHeaders()).end();
    });

    httpServer.listen(config.port, () => {
      console.error(
        `[mcp-support] SSE transport listening on port ${config.port}`,
      );
    });
  } else {
    const transport = new StdioServerTransport();
    await server.connect(transport);
    console.error("[mcp-support] stdio transport ready");
  }
}

main().catch((err) => {
  console.error("[mcp-support] Fatal error:", err instanceof Error ? err.message : String(err));
  process.exit(1);
});
