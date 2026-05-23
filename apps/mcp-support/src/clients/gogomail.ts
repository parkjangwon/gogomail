export interface GogomailUser {
  id: string;
  email: string;
  name: string;
  status: "active" | "suspended" | "disabled";
  role: string;
  companyId: string;
  quotaBytes: number;
  createdAt: string;
}

export interface GogomailQuota {
  userId: string;
  allocatedBytes: number;
  usedBytes: number;
  updatedAt: string;
}

export interface GogomailMailLog {
  id: string;
  userId: string;
  messageId: string;
  direction: "inbound" | "outbound";
  status: string;
  from: string;
  to: string;
  subject: string;
  timestamp: string;
}

export interface GogomailMessageTrace {
  messageId: string;
  hops: Array<{
    server: string;
    timestamp: string;
    action: string;
    status: string;
  }>;
}

export interface GogomailDeliveryAttempt {
  id: string;
  messageId: string;
  attemptedAt: string;
  status: string;
  errorCode: string | null;
  errorMessage: string | null;
  nextRetryAt: string | null;
}

export interface GogomailAuditLog {
  id: string;
  actorId: string | null;
  targetId: string | null;
  action: string;
  meta: Record<string, unknown>;
  createdAt: string;
}

export interface GogomailSession {
  id: string;
  userId: string;
  userAgent: string;
  ip: string;
  createdAt: string;
  lastSeenAt: string;
}

export interface GogomailHealth {
  status: "ok" | "degraded" | "down";
  queueDepth: number;
  components: Record<string, string>;
}

export interface GogomailCompany {
  id: string;
  name: string;
  domains: string[];
  plan: string;
  createdAt: string;
}

export interface GogomailDomainSettings {
  domainId: string;
  domain: string;
  catchAll: boolean;
  spfEnabled: boolean;
  dkimEnabled: boolean;
  dmarcEnabled: boolean;
  maxMessageSize: number;
}

export interface GogomailAlertEvent {
  id: string;
  companyId: string;
  type: string;
  severity: "info" | "warning" | "critical";
  message: string;
  createdAt: string;
}

export class GogomailClient {
  private readonly baseUrl: string;
  private readonly apiKey: string;

  constructor(baseUrl: string, apiKey: string) {
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.apiKey = apiKey;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown,
  ): Promise<T> {
    const url = `${this.baseUrl}/api/admin${path}`;
    const res = await fetch(url, {
      method,
      headers: {
        Authorization: `Bearer ${this.apiKey}`,
        "Content-Type": "application/json",
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    });
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      throw new Error(
        `GoGoMail Admin API ${method} ${path} → ${res.status}: ${text}`,
      );
    }
    return res.json() as Promise<T>;
  }

  // ── Read operations ────────────────────────────────────────────

  async findUser(email: string): Promise<GogomailUser[]> {
    const q = new URLSearchParams({ email });
    return this.request<GogomailUser[]>("GET", `/users?${q}`);
  }

  async getUser(userId: string): Promise<GogomailUser> {
    return this.request<GogomailUser>("GET", `/users/${userId}`);
  }

  async getUserQuota(userId: string): Promise<GogomailQuota> {
    return this.request<GogomailQuota>("GET", `/users/${userId}/quota`);
  }

  async getMailLogs(params: {
    userId: string;
    direction?: string;
    status?: string;
    from?: string;
    to?: string;
  }): Promise<GogomailMailLog[]> {
    const { userId, ...filters } = params;
    const q = new URLSearchParams();
    if (filters.direction) q.set("direction", filters.direction);
    if (filters.status) q.set("status", filters.status);
    if (filters.from) q.set("from", filters.from);
    if (filters.to) q.set("to", filters.to);
    return this.request<GogomailMailLog[]>(
      "GET",
      `/users/${userId}/maillogs?${q}`,
    );
  }

  async traceMessage(messageId: string): Promise<GogomailMessageTrace> {
    return this.request<GogomailMessageTrace>(
      "GET",
      `/messages/${messageId}/trace`,
    );
  }

  async getDeliveryAttempts(
    messageId: string,
  ): Promise<GogomailDeliveryAttempt[]> {
    return this.request<GogomailDeliveryAttempt[]>(
      "GET",
      `/messages/${messageId}/delivery-attempts`,
    );
  }

  async getAuditLogs(params: {
    userId?: string;
    companyId?: string;
    from?: string;
    to?: string;
  }): Promise<GogomailAuditLog[]> {
    const q = new URLSearchParams();
    if (params.userId) q.set("userId", params.userId);
    if (params.companyId) q.set("companyId", params.companyId);
    if (params.from) q.set("from", params.from);
    if (params.to) q.set("to", params.to);
    return this.request<GogomailAuditLog[]>("GET", `/audit-logs?${q}`);
  }

  async listUserSessions(userId: string): Promise<GogomailSession[]> {
    return this.request<GogomailSession[]>("GET", `/users/${userId}/sessions`);
  }

  async checkHealth(): Promise<GogomailHealth> {
    return this.request<GogomailHealth>("GET", "/health");
  }

  // ── Action operations ──────────────────────────────────────────

  async resetPassword(userId: string): Promise<{ sent: boolean }> {
    return this.request<{ sent: boolean }>(
      "POST",
      `/users/${userId}/reset-password`,
    );
  }

  async updateUserStatus(
    userId: string,
    status: "active" | "suspended" | "disabled",
  ): Promise<GogomailUser> {
    return this.request<GogomailUser>("PATCH", `/users/${userId}/status`, {
      status,
    });
  }

  async updateUserQuota(
    userId: string,
    quotaBytes: number,
  ): Promise<GogomailQuota> {
    return this.request<GogomailQuota>("PATCH", `/users/${userId}/quota`, {
      quotaBytes,
    });
  }

  async revokeSessions(userId: string): Promise<{ revoked: number }> {
    return this.request<{ revoked: number }>(
      "DELETE",
      `/users/${userId}/sessions`,
    );
  }

  async updateUserRole(userId: string, role: string): Promise<GogomailUser> {
    return this.request<GogomailUser>("PATCH", `/users/${userId}/role`, {
      role,
    });
  }

  async getCompany(companyId: string): Promise<GogomailCompany> {
    return this.request<GogomailCompany>("GET", `/companies/${companyId}`);
  }

  async getDomainSettings(domainId: string): Promise<GogomailDomainSettings> {
    return this.request<GogomailDomainSettings>(
      "GET",
      `/domains/${domainId}/settings`,
    );
  }

  async updateDomainSettings(
    domainId: string,
    settings: Partial<GogomailDomainSettings>,
  ): Promise<GogomailDomainSettings> {
    return this.request<GogomailDomainSettings>(
      "PATCH",
      `/domains/${domainId}/settings`,
      settings,
    );
  }

  async getAlertEvents(params: {
    companyId: string;
    limit?: number;
  }): Promise<GogomailAlertEvent[]> {
    const q = new URLSearchParams();
    if (params.limit) q.set("limit", String(params.limit));
    return this.request<GogomailAlertEvent[]>(
      "GET",
      `/companies/${params.companyId}/alerts?${q}`,
    );
  }
}
