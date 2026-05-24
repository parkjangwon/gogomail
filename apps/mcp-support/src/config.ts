import { isIP } from "node:net";

function requireEnv(name: string): string {
  const val = process.env[name]?.trim();
  if (!val) {
    console.error(`[mcp-support] Missing required env var: ${name}`);
    process.exit(1);
  }
  return val;
}

function optionalEnv(name: string): string | undefined {
  const val = process.env[name]?.trim();
  return val || undefined;
}

function boolEnv(name: string, defaultValue = false): boolean {
  const val = optionalEnv(name);
  if (!val) return defaultValue;
  if (["1", "true", "yes"].includes(val.toLowerCase())) return true;
  if (["0", "false", "no"].includes(val.toLowerCase())) return false;
  console.error(`[mcp-support] ${name} must be true/false, got: ${val}`);
  process.exit(1);
}

function isLoopbackHost(hostname: string): boolean {
  return hostname === "localhost" || hostname === "127.0.0.1" || hostname === "::1";
}

function validateUrl(val: string, name: string, allowInsecureHttp: boolean): string {
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
  if (parsed.protocol === "http:" && !isLoopbackHost(parsed.hostname) && !allowInsecureHttp) {
    console.error(
      `[mcp-support] ${name} must use https for non-loopback hosts. Set MCP_ALLOW_INSECURE_UPSTREAMS=true only for an explicitly trusted private network.`,
    );
    process.exit(1);
  }
  // Embedded credentials in URLs (https://user:pass@host) would be stored in
  // baseUrl and could appear in network-level error messages.
  if (parsed.username || parsed.password) {
    console.error(`[mcp-support] ${name} must not contain embedded credentials`);
    process.exit(1);
  }
  return val;
}

function reject(name: string, message: string): never {
  console.error(`[mcp-support] ${name} ${message}`);
  process.exit(1);
}

// Guard against header/log injection in tokens and other one-line config values.
function validateSingleLine(val: string | undefined, name: string): string | undefined {
  if (val && /[\r\n]/.test(val)) {
    reject(name, "must not contain newline characters");
  }
  return val;
}

function requireSecretEnv(name: string): string {
  return validateSingleLine(requireEnv(name), name)!;
}

function optionalSecretEnv(name: string): string | undefined {
  return validateSingleLine(optionalEnv(name), name);
}

function validateGithubRepo(val: string): string {
  const repo = validateSingleLine(val.trim(), "GITHUB_REPO")!;
  if (!/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repo)) {
    reject("GITHUB_REPO", 'must be in "owner/repo" format');
  }
  return repo;
}

function validateHost(val: string): string {
  const host = validateSingleLine(val.trim(), "MCP_HOST")!;
  if (!host || host.length > 255 || /[/;?#\s]/.test(host)) {
    reject("MCP_HOST", "must be a hostname or IP address without scheme, port, path, or whitespace");
  }
  if (isIP(host)) return host;
  if (host.includes(":")) {
    reject("MCP_HOST", "must be a hostname or IP address without scheme, port, path, or whitespace");
  }
  return host;
}

function parseAllowedOrigins(raw: string | undefined): string[] {
  if (!raw) return [];
  return raw.split(",").map((origin) => origin.trim()).filter(Boolean).map((origin) => {
    const oneLine = validateSingleLine(origin, "MCP_ALLOWED_ORIGINS")!;
    let parsed: URL;
    try {
      parsed = new URL(oneLine);
    } catch {
      reject("MCP_ALLOWED_ORIGINS", `contains an invalid origin: ${oneLine}`);
    }
    if (parsed.origin !== oneLine || parsed.pathname !== "/" || parsed.search || parsed.hash) {
      reject("MCP_ALLOWED_ORIGINS", `must contain origins only, got: ${oneLine}`);
    }
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      reject("MCP_ALLOWED_ORIGINS", `origin must use http or https: ${oneLine}`);
    }
    return parsed.origin;
  });
}

const allowInsecureUpstreams = boolEnv("MCP_ALLOW_INSECURE_UPSTREAMS", false);

const gogomailAdminUrl = validateUrl(
  requireEnv("GOGOMAIL_ADMIN_URL"),
  "GOGOMAIL_ADMIN_URL",
  allowInsecureUpstreams,
);
const gogomailAdminKey = requireSecretEnv("GOGOMAIL_ADMIN_KEY");

const suppoApiUrl = optionalEnv("SUPPO_API_URL");
const suppoApiKey = optionalSecretEnv("SUPPO_API_KEY");

if (suppoApiUrl) {
  validateUrl(suppoApiUrl, "SUPPO_API_URL", allowInsecureUpstreams);
}

if ((suppoApiUrl && !suppoApiKey) || (!suppoApiUrl && suppoApiKey)) {
  reject("SUPPO_API_URL/SUPPO_API_KEY", "must be configured together");
}

const githubToken = optionalSecretEnv("GITHUB_TOKEN");

const portRaw = parseInt(process.env["MCP_PORT"] ?? "3100", 10);
if (isNaN(portRaw) || portRaw < 1 || portRaw > 65535) {
  console.error(
    `[mcp-support] MCP_PORT must be a valid port number (1-65535), got: ${process.env["MCP_PORT"] ?? "(unset)"}`,
  );
  process.exit(1);
}

const transport = optionalEnv("MCP_TRANSPORT") ?? "stdio";
if (transport !== "stdio" && transport !== "sse") {
  console.error(`[mcp-support] MCP_TRANSPORT must be "stdio" or "sse", got: ${transport}`);
  process.exit(1);
}

const host = validateHost(optionalEnv("MCP_HOST") ?? "127.0.0.1");
const mcpAllowedOrigins = parseAllowedOrigins(optionalEnv("MCP_ALLOWED_ORIGINS"));
const mcpSecret = optionalSecretEnv("MCP_SECRET");
if (transport === "sse") {
  if (!mcpSecret) {
    console.error("[mcp-support] MCP_SECRET is required when MCP_TRANSPORT=sse");
    process.exit(1);
  }
  if (Buffer.byteLength(mcpSecret, "utf8") < 32) {
    console.error("[mcp-support] MCP_SECRET must be at least 32 bytes when MCP_TRANSPORT=sse");
    process.exit(1);
  }
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
    repo: validateGithubRepo(process.env["GITHUB_REPO"] ?? "parkjangwon/gogomail"),
  },
  // When set, all SSE connections must send: Authorization: Bearer <mcpSecret>
  mcpSecret,
  mcpAllowedOrigins,
  transport: transport as "stdio" | "sse",
  host,
  port: portRaw,
};
