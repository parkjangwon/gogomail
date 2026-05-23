function requireEnv(name: string): string {
  const val = process.env[name];
  if (!val) {
    console.error(`[mcp-support] Missing required env var: ${name}`);
    process.exit(1);
  }
  return val;
}

export const config = {
  gogomail: {
    adminUrl: requireEnv("GOGOMAIL_ADMIN_URL"),
    adminKey: requireEnv("GOGOMAIL_ADMIN_KEY"),
  },
  suppo: {
    apiUrl: requireEnv("SUPPO_API_URL"),
    apiKey: requireEnv("SUPPO_API_KEY"),
  },
  github: {
    token: requireEnv("GITHUB_TOKEN"),
    repo: process.env["GITHUB_REPO"] ?? "parkjangwon/gogomail",
  },
  transport: (process.env["MCP_TRANSPORT"] ?? "stdio") as "stdio" | "sse",
  port: parseInt(process.env["MCP_PORT"] ?? "3100", 10),
};
