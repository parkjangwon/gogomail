import type { Page } from "@playwright/test";

const AUTH_OK = { ok: true, user: { id: "admin", role: "owner", company_id: "default" } };
const EMPTY_ADMIN_RESPONSE = {
  ok: true,
  alerts: [],
  api_keys: [],
  audit_logs: [],
  companies: [],
  data: [],
  domains: [],
  events: [],
  items: [],
  logs: [],
  mail_flow_logs: [],
  relays: [],
  routes: [],
  capabilities: {},
  policy: {
    enabled: false,
    allowlist: [],
    denylist: [],
    protocols: ["smtp", "imap", "api"],
    action: "deny",
  },
  roles: [],
  stats: {},
  total: 0,
  users: [],
};
const AUDIT_POLICY = {
  company_id: "default",
  audit_level: "level_2",
  audit_admin_actions: true,
  audit_security_events: true,
  retention_days: 90,
  mask_mail_content: true,
  mask_recipient_emails: false,
};
const POSTURE = {
  score: 0,
  mfa: { total: 0, enabled: 0, rate: 0 },
  ip_policy_configured: false,
  users_without_password: 0,
  domain_count: 0,
  active_domains: 0,
};
const CAPABILITIES = {
  integrations: { organization_sync: "planned" },
};

export async function installLocalAdminSession(page: Page) {
  await page.route("**/api/admin/**", async route => {
    const path = new URL(route.request().url()).pathname;
    let body: unknown = EMPTY_ADMIN_RESPONSE;
    if (path.endsWith("/auth/verify")) body = AUTH_OK;
    if (path.endsWith("/auth/login")) body = { ok: true };
    if (path.endsWith("/companies")) body = { ...EMPTY_ADMIN_RESPONSE, companies: [] };
    if (path.endsWith("/security/ip-policy")) body = EMPTY_ADMIN_RESPONSE;
    if (path.endsWith("/security/audit-policy")) body = { policy: AUDIT_POLICY };
    if (path.endsWith("/security/posture")) body = POSTURE;
    if (path.endsWith("/console/capabilities")) body = { admin_console_capabilities: CAPABILITIES };
    if (path.endsWith("/roles")) body = { roles: [] };
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(body),
    });
  });
}
