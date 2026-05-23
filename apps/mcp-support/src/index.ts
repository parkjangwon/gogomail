import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import {
  CallToolRequestSchema,
  ListToolsRequestSchema,
} from "@modelcontextprotocol/sdk/types.js";
import { config } from "./config.js";
import { GogomailClient } from "./clients/gogomail.js";
import { SuppoClient } from "./clients/suppo.js";
import { GithubClient } from "./clients/github.js";
import * as suppoTools from "./tools/suppo.js";
import * as gogomailTools from "./tools/gogomail.js";
import * as githubTools from "./tools/github.js";

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
    const message = err instanceof Error ? err.message : String(err);
    return {
      content: [{ type: "text", text: `Error: ${message}` }],
      isError: true,
    };
  }
});

async function main(): Promise<void> {
  if (config.transport === "sse") {
    const { SSEServerTransport } = await import(
      "@modelcontextprotocol/sdk/server/sse.js"
    );
    const { createServer } = await import("http");

    const sessions = new Map<string, InstanceType<typeof SSEServerTransport>>();

    const httpServer = createServer((req, res) => {
      if (req.method === "GET" && req.url === "/sse") {
        const transport = new SSEServerTransport("/messages", res);
        sessions.set(transport.sessionId, transport);
        server.connect(transport).catch(console.error);
        req.on("close", () => sessions.delete(transport.sessionId));
        return;
      }

      if (req.method === "POST" && req.url?.startsWith("/messages")) {
        const url = new URL(req.url, `http://localhost`);
        const sessionId = url.searchParams.get("sessionId") ?? "";
        const transport = sessions.get(sessionId);
        if (!transport) {
          res.writeHead(404).end("Session not found");
          return;
        }
        let body = "";
        req.on("data", (chunk: Buffer) => (body += chunk));
        req.on("end", () => {
          transport.handlePostMessage(req, res, body).catch(console.error);
        });
        return;
      }

      res.writeHead(404).end();
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
  console.error("[mcp-support] Fatal error:", err);
  process.exit(1);
});
