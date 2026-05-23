function requireEnv(name: string): string {
  const val = process.env[name];
  if (!val) {
    console.error(`[mcp-support] Missing required env var: ${name}`);
    process.exit(1);
  }
  return val;
}

function optionalEnv(name: string): string | undefined {
  return process.env[name] || undefined;
}

function validateUrl(val: string, name: string): string {
  let parsed: URL;
  try {
    parsed = new URL(val);
  } catch {
    console.error(`[mcp-support] ${name} is not a valid URL: ${val}`);
    process.exit(1);
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    console.error(`[mcp-support] ${name} must use http or https scheme: ${val}`);
    process.exit(1);
  }
  return val;
}

// Guard against newline injection in header values (CWE-93)
function validateNoNewlines(val: string | undefined, name: string): string | undefined {
  if (val && /[\r\n]/.test(val)) {
    console.error(`[mcp-support] ${name} must not contain newline characters`);
    process.exit(1);
  }
  return val;
}

const gogomailAdminUrl = validateUrl(requireEnv("GOGOMAIL_ADMIN_URL"), "GOGOMAIL_ADMIN_URL");
const gogomailAdminKey = validateNoNewlines(requireEnv("GOGOMAIL_ADMIN_KEY"), "GOGOMAIL_ADMIN_KEY")!;

const suppoApiUrl = process.env["SUPPO_API_URL"] || undefined;
const suppoApiKey = validateNoNewlines(process.env["SUPPO_API_KEY"] || undefined, "SUPPO_API_KEY");

if (suppoApiUrl) validateUrl(suppoApiUrl, "SUPPO_API_URL");

const githubToken = validateNoNewlines(process.env["GITHUB_TOKEN"] || undefined, "GITHUB_TOKEN");

const portRaw = parseInt(process.env["MCP_PORT"] ?? "3100", 10);
if (isNaN(portRaw) || portRaw < 1 || portRaw > 65535) {
  console.error(
    `[mcp-support] MCP_PORT must be a valid port number (1-65535), got: ${process.env["MCP_PORT"] ?? "(unset)"}`,
  );
  process.exit(1);
}

export const config = {
  gogomail: {
    adminUrl: gogomailAdminUrl,
    adminKey: gogomailAdminKey,
  },
  suppo: {
    apiUrl: suppoApiUrl,
    apiKey: suppoApiKey,
  },
  github: {
    token: githubToken,
    repo: process.env["GITHUB_REPO"] ?? "parkjangwon/gogomail",
  },
  // When set, all SSE connections must send: Authorization: Bearer <mcpSecret>
  mcpSecret: optionalEnv("MCP_SECRET"),
  transport: (process.env["MCP_TRANSPORT"] ?? "stdio") as "stdio" | "sse",
  port: portRaw,
};
