export interface GogomailUser {
  id: string;
  domain_id: string;
  username: string;
  display_name: string;
  recovery_email?: string;
  status: "active" | "suspended" | "disabled";
  role: string;
  password_configured: boolean;
  must_change_password: boolean;
  quota_used: number;
  quota_limit?: number;
  quota_remaining: number;
  quota_source: "default" | "custom";
  created_at: string;
}

export interface GogomailQuota {
  userId: string;
  allocatedBytes: number;
  usedBytes: number;
  quotaSource: string;
}

export interface GogomailMailFlowLog {
  id: string;
  userId: string;
  messageId: string;
  rfcMessageId: string;
  direction: "inbound" | "outbound";
  flowStatus: string;
  fromAddr: string;
  toAddr: string;
  subject: string;
  timestamp: string;
}

export interface GogomailDeliveryAttempt {
  id: string;
  messageId: string;
  attemptedAt: string;
  status: string;
  errorCode: string | null;
  errorMessage: string | null;
  nextRetryAt: string | null;
  recipientDomain: string;
  sender: string;
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
  components: Record<string, string>;
}

export interface GogomailCompany {
  id: string;
  name: string;
  status: string;
  plan: string;
  createdAt: string;
}

export interface GogomailDomain {
  id: string;
  name: string;
  companyId: string;
  status: string;
  dnsStatus: string;
  createdAt: string;
}

export interface GogomailDomainSettings {
  domain_id: string;
  tls_policy: "opportunistic" | "require" | "disable";
  quota_per_user: number;
  ip_whitelist_enabled: boolean;
  ip_whitelist: string[];
  require_2fa: boolean;
  session_timeout_minutes: number;
  password_min_length: number;
  password_require_uppercase: boolean;
  password_require_numbers: boolean;
  password_require_special_chars: boolean;
  password_expiry_days: number;
  user_registration_mode?: "temp_password" | "email_invite";
  password_reset_token_ttl_minutes: number;
  updated_at?: string;
  updated_by?: string;
}

export interface GogomailIDStatus {
  status: string;
  id: string;
}

export interface GogomailAlertEvent {
  id: string;
  companyId: string;
  type: string;
  severity: "info" | "warning" | "critical";
  message: string;
  createdAt: string;
}

export interface GogomailDlqEntry {
  id: string;
  stream: string;
  payload: unknown;
  errorMessage: string;
  createdAt: string;
}

export interface GogomailSuppressionEntry {
  id: string;
  email: string;
  domainId: string;
  reason: string;
  createdAt: string;
}

export interface GogomailQuotaUsage {
  entityId: string;
  entityType: string;
  allocatedBytes: number;
  usedBytes: number;
  usedPercent: number;
}

export interface GogomailQuotaAlert {
  id: string;
  userId: string;
  thresholdPercent: number;
  triggeredAt: string;
  resolvedAt: string | null;
}

export interface GogomailDirectoryPrincipal {
  id: string;
  kind: string;
  email: string;
  name: string;
  companyId: string;
  domainId: string;
  status: string;
}

export interface GogomailDkimKey {
  id: string;
  domainId: string;
  selector: string;
  status: string;
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
    const url = `${this.baseUrl}/admin/v1${path}`;
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 30_000);
    let res: Response;
    try {
      res = await fetch(url, {
        method,
        headers: {
          Authorization: `Bearer ${this.apiKey}`,
          "Content-Type": "application/json",
        },
        body: body !== undefined ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      });
    } finally {
      clearTimeout(timer);
    }
    if (!res.ok) {
      const text = await res.text().catch(() => "");
      if (res.status >= 500) {
        // Log full body internally but don't expose server internals to the agent
        console.error(
          `[gogomail] ${method} /admin/v1${path} → ${res.status}: ${text.slice(0, 300)}`,
        );
        throw new Error(
          `GoGoMail Admin API ${method} /admin/v1${path} → ${res.status} (internal server error)`,
        );
      }
      throw new Error(
        `GoGoMail Admin API ${method} /admin/v1${path} → ${res.status}: ${text}`,
      );
    }
    if (method === "DELETE" && res.status === 204) {
      return undefined as T;
    }
    return res.json() as Promise<T>;
  }

  // ── Directory / user search ─────────────────────────────────────

  async searchPrincipals(params: {
    q: string;
    companyId?: string;
    domainId?: string;
    limit?: number;
  }): Promise<GogomailDirectoryPrincipal[]> {
    const q = new URLSearchParams({ q: params.q });
    if (params.companyId) q.set("company_id", params.companyId);
    if (params.domainId) q.set("domain_id", params.domainId);
    if (params.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ directory_principals: GogomailDirectoryPrincipal[] }>(
      "GET",
      `/directory/principals?${q}`,
    );
    return res.directory_principals;
  }

  async listUsers(params: {
    domainId?: string;
    status?: string;
    limit?: number;
  }): Promise<{ users: GogomailUser[]; hasMore: boolean }> {
    const q = new URLSearchParams();
    if (params.domainId) q.set("domain_id", params.domainId);
    if (params.status) q.set("status", params.status);
    if (params.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ users: GogomailUser[]; has_more: boolean }>(
      "GET",
      `/users?${q}`,
    );
    return { users: res.users, hasMore: res.has_more };
  }

  async getUser(userId: string): Promise<GogomailUser> {
    const res = await this.request<{ user: GogomailUser }>("GET", `/users/${userId}`);
    return res.user;
  }

  async getUserQuota(userId: string): Promise<GogomailQuota> {
    const user = await this.getUser(userId);
    return {
      userId: user.id,
      allocatedBytes: user.quota_limit ?? 0,
      usedBytes: user.quota_used,
      quotaSource: user.quota_source,
    };
  }

  // ── Companies ───────────────────────────────────────────────────

  async listCompanies(params?: {
    status?: string;
    limit?: number;
  }): Promise<{ companies: GogomailCompany[]; hasMore: boolean }> {
    const q = new URLSearchParams();
    if (params?.status) q.set("status", params.status);
    if (params?.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ companies: GogomailCompany[]; has_more: boolean }>(
      "GET",
      `/companies?${q}`,
    );
    return { companies: res.companies, hasMore: res.has_more };
  }

  async getCompany(companyId: string): Promise<GogomailCompany> {
    const res = await this.request<{ company: GogomailCompany }>(
      "GET",
      `/companies/${companyId}`,
    );
    return res.company;
  }

  // ── Domains ─────────────────────────────────────────────────────

  async listDomains(params?: {
    companyId?: string;
    status?: string;
    dnsStatus?: string;
    limit?: number;
  }): Promise<{ domains: GogomailDomain[]; hasMore: boolean }> {
    const q = new URLSearchParams();
    if (params?.companyId) q.set("company_id", params.companyId);
    if (params?.status) q.set("status", params.status);
    if (params?.dnsStatus) q.set("dns_status", params.dnsStatus);
    if (params?.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ domains: GogomailDomain[]; has_more: boolean }>(
      "GET",
      `/domains?${q}`,
    );
    return { domains: res.domains, hasMore: res.has_more };
  }

  async getDomainSettings(domainId: string): Promise<GogomailDomainSettings> {
    const res = await this.request<{ settings: GogomailDomainSettings }>(
      "GET",
      `/domains/${domainId}/settings`,
    );
    return res.settings;
  }

  async updateDomainSettings(
    domainId: string,
    settings: Partial<GogomailDomainSettings>,
  ): Promise<GogomailDomainSettings> {
    await this.request<GogomailIDStatus>(
      "PUT",
      `/domains/${domainId}/settings`,
      { ...settings, domain_id: domainId },
    );
    return this.getDomainSettings(domainId);
  }

  async checkDomainDns(domainId: string): Promise<unknown> {
    return this.request<unknown>("GET", `/domains/${domainId}/dns-check`);
  }

  // ── Mail flow logs ──────────────────────────────────────────────

  async listMailFlowLogs(params: {
    userId?: string;
    companyId?: string;
    domainId?: string;
    messageId?: string;
    fromAddr?: string;
    toAddr?: string;
    direction?: string;
    flowStatus?: string;
    since?: string;
    until?: string;
    limit?: number;
  }): Promise<GogomailMailFlowLog[]> {
    const q = new URLSearchParams();
    if (params.userId) q.set("user_id", params.userId);
    if (params.companyId) q.set("company_id", params.companyId);
    if (params.domainId) q.set("domain_id", params.domainId);
    if (params.messageId) q.set("message_id", params.messageId);
    if (params.fromAddr) q.set("from_addr", params.fromAddr);
    if (params.toAddr) q.set("to_addr", params.toAddr);
    if (params.direction) q.set("direction", params.direction);
    if (params.flowStatus) q.set("flow_status", params.flowStatus);
    if (params.since) q.set("since", params.since);
    if (params.until) q.set("until", params.until);
    if (params.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ mail_flow_logs: GogomailMailFlowLog[] }>(
      "GET",
      `/mail-flow-logs?${q}`,
    );
    return res.mail_flow_logs;
  }

  async getMailFlowStats(params?: {
    userId?: string;
    companyId?: string;
    domainId?: string;
    direction?: string;
    since?: string;
    until?: string;
  }): Promise<unknown> {
    const q = new URLSearchParams();
    if (params?.userId) q.set("user_id", params.userId);
    if (params?.companyId) q.set("company_id", params.companyId);
    if (params?.domainId) q.set("domain_id", params.domainId);
    if (params?.direction) q.set("direction", params.direction);
    if (params?.since) q.set("since", params.since);
    if (params?.until) q.set("until", params.until);
    const res = await this.request<{ mail_flow_stats: unknown }>(
      "GET",
      `/mail-flow-logs/stats?${q}`,
    );
    return res.mail_flow_stats;
  }

  // ── Delivery attempts ───────────────────────────────────────────

  async listDeliveryAttempts(params?: {
    messageId?: string;
    status?: string;
    recipientDomain?: string;
    sender?: string;
    since?: string;
    limit?: number;
  }): Promise<{ deliveryAttempts: GogomailDeliveryAttempt[]; hasMore: boolean }> {
    const q = new URLSearchParams();
    if (params?.messageId) q.set("message_id", params.messageId);
    if (params?.status) q.set("status", params.status);
    if (params?.recipientDomain) q.set("recipient_domain", params.recipientDomain);
    if (params?.sender) q.set("sender", params.sender);
    if (params?.since) q.set("since", params.since);
    if (params?.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ delivery_attempts: GogomailDeliveryAttempt[]; has_more: boolean }>(
      "GET",
      `/delivery-attempts?${q}`,
    );
    return { deliveryAttempts: res.delivery_attempts, hasMore: res.has_more };
  }

  async listExhaustedDeliveries(params?: {
    messageId?: string;
    recipientDomain?: string;
    sender?: string;
    since?: string;
    limit?: number;
  }): Promise<GogomailDeliveryAttempt[]> {
    const q = new URLSearchParams();
    if (params?.messageId) q.set("message_id", params.messageId);
    if (params?.recipientDomain) q.set("recipient_domain", params.recipientDomain);
    if (params?.sender) q.set("sender", params.sender);
    if (params?.since) q.set("since", params.since);
    if (params?.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ delivery_attempts: GogomailDeliveryAttempt[] }>(
      "GET",
      `/delivery-attempts/exhausted?${q}`,
    );
    return res.delivery_attempts;
  }

  async retryOutbox(id: string): Promise<{ status: string; id: string }> {
    return this.request<{ status: string; id: string }>(
      "POST",
      `/outbox/${id}/retry`,
    );
  }

  // ── Dead Letter Queue ───────────────────────────────────────────

  async listDlq(stream: string, count?: number): Promise<GogomailDlqEntry[]> {
    const q = new URLSearchParams({ stream });
    if (count) q.set("count", String(count));
    const res = await this.request<{ dlq_entries: GogomailDlqEntry[] }>(
      "GET",
      `/dlq?${q}`,
    );
    return res.dlq_entries;
  }

  async deleteDlqEntry(stream: string, id: string): Promise<void> {
    const q = new URLSearchParams({ stream });
    await this.request<void>("DELETE", `/dlq/${id}?${q}`);
  }

  // ── Suppression list ────────────────────────────────────────────

  async listSuppressionList(params?: {
    email?: string;
    domainId?: string;
    reason?: string;
    limit?: number;
  }): Promise<GogomailSuppressionEntry[]> {
    const q = new URLSearchParams();
    if (params?.email) q.set("email", params.email);
    if (params?.domainId) q.set("domain_id", params.domainId);
    if (params?.reason) q.set("reason", params.reason);
    if (params?.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ suppression_list: GogomailSuppressionEntry[] }>(
      "GET",
      `/suppression-list?${q}`,
    );
    return res.suppression_list;
  }

  async removeSuppressionEntry(id: string): Promise<void> {
    await this.request<void>("DELETE", `/suppression-list/${id}`);
  }

  // ── Quota ───────────────────────────────────────────────────────

  async listQuotaUsage(params?: {
    domainId?: string;
    overLimit?: boolean;
    limit?: number;
  }): Promise<GogomailQuotaUsage[]> {
    const q = new URLSearchParams();
    if (params?.domainId) q.set("domain_id", params.domainId);
    if (params?.overLimit !== undefined) q.set("over_limit", String(params.overLimit));
    if (params?.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ quota_usage: GogomailQuotaUsage[] }>(
      "GET",
      `/quota-usage?${q}`,
    );
    return res.quota_usage;
  }

  async listQuotaAlerts(params?: {
    limit?: number;
  }): Promise<GogomailQuotaAlert[]> {
    const q = new URLSearchParams();
    if (params?.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ quota_alerts: GogomailQuotaAlert[] }>(
      "GET",
      `/quota-alerts?${q}`,
    );
    return res.quota_alerts;
  }

  async updateUserQuota(userId: string, quotaBytes: number): Promise<GogomailQuota> {
    await this.request<GogomailIDStatus>("PATCH", `/users/${userId}/quota`, {
      quota_limit: quotaBytes,
      quota_source: quotaBytes > 0 ? "custom" : "default",
    });
    return this.getUserQuota(userId);
  }

  // ── Sessions ────────────────────────────────────────────────────

  async listCompanySessions(companyId: string): Promise<GogomailSession[]> {
    const res = await this.request<{ sessions: GogomailSession[] }>(
      "GET",
      `/companies/${companyId}/sessions`,
    );
    return res.sessions;
  }

  async revokeCompanySession(companyId: string, userId: string): Promise<void> {
    await this.request<void>(
      "DELETE",
      `/companies/${companyId}/sessions/${userId}`,
    );
  }

  // ── System ──────────────────────────────────────────────────────

  async checkHealth(): Promise<GogomailHealth> {
    return this.request<GogomailHealth>("GET", "/health");
  }

  async getQueueStats(): Promise<unknown> {
    const res = await this.request<{ queues: unknown }>("GET", "/queue");
    return res.queues;
  }

  async getAuditLogs(params: {
    userId?: string;
    companyId?: string;
    from?: string;
    to?: string;
    limit?: number;
  }): Promise<GogomailAuditLog[]> {
    const q = new URLSearchParams();
    if (params.userId) q.set("userId", params.userId);
    if (params.companyId) q.set("companyId", params.companyId);
    if (params.from) q.set("from", params.from);
    if (params.to) q.set("to", params.to);
    if (params.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ audit_logs: GogomailAuditLog[] }>(
      "GET",
      `/audit-logs?${q}`,
    );
    return res.audit_logs ?? [];
  }

  async getAlertEvents(params: {
    companyId: string;
    limit?: number;
  }): Promise<GogomailAlertEvent[]> {
    const q = new URLSearchParams();
    if (params.limit) q.set("limit", String(params.limit));
    const res = await this.request<{ alert_events: GogomailAlertEvent[] }>(
      "GET",
      `/companies/${params.companyId}/alert-events?${q}`,
    );
    return res.alert_events;
  }

  // ── DKIM ────────────────────────────────────────────────────────

  async listDkimKeys(domainId?: string): Promise<GogomailDkimKey[]> {
    const q = new URLSearchParams();
    if (domainId) q.set("domain_id", domainId);
    const res = await this.request<{ dkim_keys: GogomailDkimKey[] }>(
      "GET",
      `/dkim-keys?${q}`,
    );
    return res.dkim_keys;
  }

  // ── Spam filter ─────────────────────────────────────────────────

  async getSpamFilter(companyId: string): Promise<unknown> {
    return this.request<unknown>(
      "GET",
      `/companies/${companyId}/security/spam-filter`,
    );
  }

  async getSpamFilterEvents(companyId: string): Promise<unknown> {
    return this.request<unknown>(
      "GET",
      `/companies/${companyId}/security/spam-filter/events`,
    );
  }

  // ── User actions ─────────────────────────────────────────────────

  async sendInviteEmail(userId: string): Promise<unknown> {
    return this.request<unknown>("POST", `/users/${userId}/invite`);
  }

  async updateUserStatus(
    userId: string,
    status: "active" | "suspended" | "disabled",
  ): Promise<GogomailUser> {
    await this.request<GogomailIDStatus>("PATCH", `/users/${userId}/status`, {
      status,
    });
    return this.getUser(userId);
  }

  async updateUserRole(userId: string, role: string): Promise<GogomailUser> {
    await this.request<GogomailIDStatus>("PATCH", `/users/${userId}/role`, { role });
    return this.getUser(userId);
  }

  async updateUserRecoveryEmail(userId: string, recoveryEmail: string): Promise<void> {
    await this.request<void>("PATCH", `/users/${userId}/recovery-email`, {
      recovery_email: recoveryEmail,
    });
  }

  async createUser(params: {
    domainId: string;
    username: string;
    displayName: string;
    recoveryEmail?: string;
    password?: string;
    quotaLimit?: number;
  }): Promise<GogomailUser> {
    const res = await this.request<{ user: GogomailUser }>("POST", "/users", {
      domain_id: params.domainId,
      username: params.username,
      display_name: params.displayName,
      recovery_email: params.recoveryEmail,
      password: params.password,
      must_change_password: !!params.password,
      quota_limit: params.quotaLimit,
    });
    return res.user;
  }

  async deleteUser(userId: string): Promise<void> {
    await this.request<void>("DELETE", `/users/${userId}`);
  }
}
