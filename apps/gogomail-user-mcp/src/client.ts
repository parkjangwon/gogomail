export interface MCPSettings {
  enabled?: boolean;
  permission_mode?: "basic" | "bypass";
  generated_mail_notice_enabled?: boolean;
  generated_mail_notice_text?: string;
}

export class GogomailUserClient {
  private readonly baseUrl: string;
  private readonly accessKey: string;
  private settingsCache?: MCPSettings;

  constructor(baseUrl: string, accessKey: string) {
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.accessKey = accessKey;
  }

  async request<T>(method: string, path: string, body?: unknown, headers: Record<string, string> = {}): Promise<T> {
    const requestHeaders: Record<string, string> = {
      Authorization: `Bearer ${this.accessKey}`,
      ...headers,
    };
    let requestBody: BodyInit | undefined;
    if (body !== undefined) {
      if (body instanceof Uint8Array) {
        const bytes = new ArrayBuffer(body.byteLength);
        new Uint8Array(bytes).set(body);
        requestBody = bytes;
      } else if (typeof body === "string") {
        requestBody = body;
      } else {
        requestHeaders["Content-Type"] = "application/json";
        requestBody = JSON.stringify(body);
      }
    }
    const res = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers: requestHeaders,
      body: requestBody,
      signal: AbortSignal.timeout(30_000),
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      if (res.status >= 500) {
        throw new Error(`GoGoMail API ${method} ${path} -> ${res.status} (internal server error)`);
      }
      throw new Error(`GoGoMail API ${method} ${path} -> ${res.status}: ${text.slice(0, 500)}`);
    }
    if (res.status === 204) return {} as T;
    const contentType = res.headers.get("Content-Type") ?? "";
    if (!contentType.includes("application/json")) {
      const bytes = Buffer.from(await res.arrayBuffer());
      const body = bytes.toString("utf8");
      return {
        body,
        body_text: body,
        body_base64: bytes.toString("base64"),
        content_type: contentType,
      } as T;
    }
    return (await res.json()) as T;
  }

  async settings(): Promise<MCPSettings> {
    if (this.settingsCache) return this.settingsCache;
    const res = await this.request<{ mcp?: MCPSettings }>("GET", "/api/v1/me/mcp/settings");
    this.settingsCache = res.mcp ?? {};
    return this.settingsCache;
  }
}

export function appendQuery(path: string, params: Record<string, unknown>): string {
  const url = new URL(path, "http://local");
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") continue;
    url.searchParams.set(key, String(value));
  }
  return url.pathname + url.search;
}

export function withMCPNotice(input: { text_body?: string; html_body?: string }, settings: MCPSettings): typeof input {
  if (settings.generated_mail_notice_enabled === false) return input;
  const text = settings.generated_mail_notice_text?.trim() || "MCP를 통해 작성된 메일입니다.";
  const out = { ...input };
  if (out.html_body && out.html_body.trim()) {
    out.html_body = `<p style="color:#8a8a8a;font-size:12px;margin:0 0 12px">${escapeHTML(text)}</p>${out.html_body}`;
  }
  if (out.text_body && out.text_body.trim()) {
    out.text_body = `${text}\n\n${out.text_body}`;
  }
  return out;
}

function escapeHTML(value: string): string {
  return value.replace(/[&<>"']/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[ch]!));
}
