function requireEnv(name: string): string {
  const value = process.env[name]?.trim();
  if (!value) {
    console.error(`[gogomail-user-mcp] Missing required env var: ${name}`);
    process.exit(1);
  }
  return value;
}

function optionalEnv(name: string): string | undefined {
  const value = process.env[name]?.trim();
  return value || undefined;
}

function reject(name: string, message: string): never {
  console.error(`[gogomail-user-mcp] ${name} ${message}`);
  process.exit(1);
}

function validateSingleLine(value: string, name: string): string {
  if (/[\r\n]/.test(value)) reject(name, "must not contain newline characters");
  return value;
}

function validateBaseUrl(value: string): string {
  validateSingleLine(value, "GOGOMAIL_API_URL");
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    reject("GOGOMAIL_API_URL", "must be a valid URL");
  }
  if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
    reject("GOGOMAIL_API_URL", "must use http or https");
  }
  if (parsed.username || parsed.password) {
    reject("GOGOMAIL_API_URL", "must not contain embedded credentials");
  }
  return value.replace(/\/$/, "");
}

function validatePermissionMode(value: string | undefined): "basic" | "bypass" {
  const normalized = (value ?? "basic").toLowerCase().trim();
  if (normalized === "basic" || normalized === "bypass") return normalized;
  reject("GOGOMAIL_MCP_PERMISSION_MODE", 'must be "basic" or "bypass"');
}

export const config = {
  apiUrl: validateBaseUrl(requireEnv("GOGOMAIL_API_URL")),
  accessKey: validateSingleLine(requireEnv("GOGOMAIL_USER_MCP_KEY"), "GOGOMAIL_USER_MCP_KEY"),
  permissionMode: validatePermissionMode(optionalEnv("GOGOMAIL_MCP_PERMISSION_MODE")),
};
