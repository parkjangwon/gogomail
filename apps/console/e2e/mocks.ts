/**
 * Shared API mocks for the admin console E2E suite.
 *
 * The admin console browser code talks to Next.js proxy routes under:
 *   - /api/admin/**   (forwarded to the backend admin API)
 *
 * These mocks intercept those routes via `page.route()` so the test suite
 * does NOT require a real backend.  Tests call `installMocks(page)` for a
 * default canned set, or pass `overrides` to inject test-specific data.
 *
 * Unmocked requests fall through to a generic 200/empty fallback that
 * returns the shape most components expect, keeping tests resilient as
 * new endpoints are introduced.
 */
import type { Page, Route } from '@playwright/test';

// --- Canned fixture data ---------------------------------------------------

export const DEFAULT_COMPANY_ID = 'default';

export const DEFAULT_AUTH = {
  ok: true,
  user: {
    id: 'admin',
    email: 'admin@system',
    name: 'System Admin',
    role: 'owner',
    company_id: DEFAULT_COMPANY_ID,
    mfa_enabled: false,
  },
};

/**
 * NOTE: intentionally empty by default.  The CompanyLayout component
 * resolves the URL `/companies/default/...` by reading the first
 * company from this list and `router.replace`-ing — which causes an
 * infinite spinner when the resolved id matches the URL id ("default").
 * Returning an empty list makes the layout skip resolution and proceed
 * to setAuthorized(true) immediately.  Tests that need company rows can
 * override via `setupAuthedAdminPage(page, { companies: [...] })`.
 */
export const DEFAULT_COMPANIES: Array<{
  id: string;
  name: string;
  domain_count: number;
  user_count: number;
  status: string;
  created_at: string;
}> = [];

export const DEFAULT_COMPANY_DETAIL = {
  id: DEFAULT_COMPANY_ID,
  name: 'Default Company',
  status: 'active',
  created_at: '2025-01-01T00:00:00Z',
  quota_used: 0,
  quota_limit: 1099511627776,
};

export const DEFAULT_DOMAINS = [
  {
    id: 'dom-1',
    name: 'example.com',
    status: 'active',
    last_dns_check_status: 'pass',
    quota_used: 0,
    quota_limit: 1073741824,
    created_at: '2025-01-01T00:00:00Z',
  },
  {
    id: 'dom-2',
    name: 'other.example.com',
    status: 'pending',
    last_dns_check_status: 'fail',
    quota_used: 0,
    quota_limit: 1073741824,
    created_at: '2025-02-01T00:00:00Z',
  },
];

export const DEFAULT_USERS = [
  {
    id: 'user-1',
    email: 'alice@example.com',
    name: 'Alice',
    role: 'user',
    status: 'active',
    storage_used: 0,
    storage_limit: 1073741824,
    last_login: '2026-05-01T10:00:00Z',
  },
  {
    id: 'user-2',
    email: 'bob@example.com',
    name: 'Bob',
    role: 'user',
    status: 'active',
    storage_used: 0,
    storage_limit: 1073741824,
    last_login: '2026-04-21T11:00:00Z',
  },
];

export const DEFAULT_ROLES = [
  { id: 'role-owner', name: 'owner', permissions: ['*'] },
  { id: 'role-admin', name: 'admin', permissions: ['users.read', 'users.write'] },
  { id: 'role-viewer', name: 'viewer', permissions: ['users.read'] },
];

export const DEFAULT_ALERTS = [
  {
    id: 'alert-1',
    name: 'High bounce rate',
    metric: 'bounce_rate',
    threshold: 5,
    enabled: true,
    notify_channel: 'email',
    created_at: '2025-03-10T00:00:00Z',
  },
];

export const DEFAULT_AUDIT_LOGS = [
  {
    id: 'log-1',
    actor: 'admin@system',
    action: 'user.create',
    target: 'alice@example.com',
    timestamp: '2026-05-22T10:00:00Z',
    result: 'success',
  },
  {
    id: 'log-2',
    actor: 'admin@system',
    action: 'domain.update',
    target: 'example.com',
    timestamp: '2026-05-21T10:00:00Z',
    result: 'success',
  },
];

export const DEFAULT_MAIL_FLOW_LOGS = [
  {
    id: 'mail-1',
    from_addr: 'alice@example.com',
    to_addr: 'bob@example.com',
    subject: 'Hello',
    direction: 'outbound',
    status: 'delivered',
    timestamp: '2026-05-22T10:00:00Z',
    size: 1024,
  },
];

export const DEFAULT_DELIVERY_ROUTES = [
  {
    id: 'route-1',
    name: 'Primary',
    domain: 'example.com',
    relay_host: 'relay.example.com',
    relay_port: 25,
    enabled: true,
  },
];

export const DEFAULT_RELAYS = [
  { id: 'relay-1', host: 'trusted.relay.com', enabled: true, note: '' },
];

export const DEFAULT_API_KEYS = [
  {
    id: 'key-1',
    name: 'CI',
    prefix: 'gm_xxx',
    scopes: ['mail.send'],
    last_used: '2026-05-22T00:00:00Z',
    created_at: '2025-01-01T00:00:00Z',
  },
];

export const DEFAULT_STORAGE_USAGE = {
  total_used: 53687091200,
  total_limit: 549755813888,
  per_user: [
    { user_id: 'user-1', email: 'alice@example.com', used: 1073741824, limit: 1073741824 },
    { user_id: 'user-2', email: 'bob@example.com', used: 536870912, limit: 1073741824 },
  ],
};

export const DEFAULT_DASHBOARD = {
  stats: {
    total_users: 5,
    active_users: 4,
    suspended_users: 1,
    active_domains: 2,
    domain_count: 2,
    total_storage_used: 53687091200,
    total_storage_limit: 549755813888,
    storage_pct: 10,
    api_requests_24h: 1234,
    error_rate: 0.5,
    mail_volume_24h: 567,
  },
  health: {
    overall: 'healthy',
    api_server: 'healthy',
    database: 'healthy',
    queue: 'healthy',
  },
  fetchedAt: new Date().toISOString(),
};

export const DEFAULT_POSTURE = {
  score: 80,
  mfa: { total: 2, enabled: 1, rate: 50 },
  ip_policy_configured: false,
  users_without_password: 0,
  domain_count: 2,
  active_domains: 2,
};

export const DEFAULT_AUDIT_POLICY = {
  company_id: DEFAULT_COMPANY_ID,
  audit_level: 'level_2',
  audit_admin_actions: true,
  audit_security_events: true,
  retention_days: 90,
  mask_mail_content: true,
  mask_recipient_emails: false,
};

export const DEFAULT_IP_POLICY = {
  enabled: false,
  allowlist: [],
  denylist: [],
  protocols: ['smtp', 'imap', 'api'],
  action: 'deny',
};

export const DEFAULT_CAPABILITIES = {
  integrations: { organization_sync: 'planned' },
};

// --- Mock installation -----------------------------------------------------

type Json = unknown;
type RouteHandler = (route: Route) => unknown | Promise<unknown>;

export interface MockOverrides {
  companies?: typeof DEFAULT_COMPANIES;
  companyDetail?: typeof DEFAULT_COMPANY_DETAIL;
  domains?: typeof DEFAULT_DOMAINS;
  users?: typeof DEFAULT_USERS;
  roles?: typeof DEFAULT_ROLES;
  alerts?: typeof DEFAULT_ALERTS;
  auditLogs?: typeof DEFAULT_AUDIT_LOGS;
  mailFlowLogs?: typeof DEFAULT_MAIL_FLOW_LOGS;
  deliveryRoutes?: typeof DEFAULT_DELIVERY_ROUTES;
  relays?: typeof DEFAULT_RELAYS;
  apiKeys?: typeof DEFAULT_API_KEYS;
  storageUsage?: typeof DEFAULT_STORAGE_USAGE;
  dashboard?: typeof DEFAULT_DASHBOARD;
  posture?: typeof DEFAULT_POSTURE;
  /** If true, /auth/verify returns 401 (auth failure) */
  unauthorized?: boolean;
  /** If non-null, /auth/login returns this error instead of success */
  loginError?: { status: number; message: string } | null;
  /** Extra raw route handlers — matched before defaults */
  extra?: Array<{ urlPattern: string | RegExp; handler: RouteHandler }>;
}

/**
 * Base envelope merged into every successful JSON response so hooks that
 * pluck `data`/`items`/`users`/etc. never get `undefined`.  Per-endpoint
 * overrides win because they appear later in the spread.
 */
const BASE_ENVELOPE = {
  ok: true,
  data: [],
  items: [],
  events: [],
  logs: [],
  rules: [],
  entries: [],
  series: [],
  routes: [],
  relays: [],
  domains: [],
  users: [],
  admin_users: [],
  groups: [],
  aliases: [],
  delegations: [],
  companies: [],
  roles: [],
  alerts: [],
  audit_logs: [],
  mail_flow_logs: [],
  api_keys: [],
  keys: [],
  dkim_keys: [],
  sessions: [],
  webhooks: [],
  templates: [],
  signatures: [],
  reports: [],
  nodes: [],
  attachments: [],
  seats: [],
  holds: [],
  attempts: [],
  metrics: [],
  activities: [],
  stats: {},
  config: {},
  settings: {},
  policy: {},
  status: 'ok',
  total: 0,
};

function json(route: Route, body: Json, status = 200) {
  const merged =
    body && typeof body === 'object' && !Array.isArray(body)
      ? { ...BASE_ENVELOPE, ...(body as Record<string, unknown>) }
      : body;
  return route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(merged),
  });
}

function empty(route: Route, status = 204) {
  return route.fulfill({ status, body: '' });
}

export async function installMocks(page: Page, overrides: MockOverrides = {}) {
  const companies = overrides.companies ?? DEFAULT_COMPANIES;
  const companyDetail = overrides.companyDetail ?? DEFAULT_COMPANY_DETAIL;
  const domains = overrides.domains ?? DEFAULT_DOMAINS;
  const users = overrides.users ?? DEFAULT_USERS;
  const roles = overrides.roles ?? DEFAULT_ROLES;
  const alerts = overrides.alerts ?? DEFAULT_ALERTS;
  const auditLogs = overrides.auditLogs ?? DEFAULT_AUDIT_LOGS;
  const mailFlowLogs = overrides.mailFlowLogs ?? DEFAULT_MAIL_FLOW_LOGS;
  const deliveryRoutes = overrides.deliveryRoutes ?? DEFAULT_DELIVERY_ROUTES;
  const relays = overrides.relays ?? DEFAULT_RELAYS;
  const apiKeys = overrides.apiKeys ?? DEFAULT_API_KEYS;
  const storageUsage = overrides.storageUsage ?? DEFAULT_STORAGE_USAGE;
  const dashboard = overrides.dashboard ?? DEFAULT_DASHBOARD;
  const posture = overrides.posture ?? DEFAULT_POSTURE;

  for (const { urlPattern, handler } of overrides.extra ?? []) {
    await page.route(urlPattern, handler);
  }

  await page.route('**/api/admin/**', async (route) => {
    const req = route.request();
    const method = req.method();
    const url = new URL(req.url());
    const path = url.pathname.replace(/^.*\/api\/admin\//, '');
    const segs = path.split('/').filter(Boolean);

    // ----- Auth -----
    if (path === 'auth/verify' && method === 'GET') {
      if (overrides.unauthorized) return json(route, { error: 'unauthorized' }, 401);
      return json(route, DEFAULT_AUTH);
    }
    if (path === 'auth/login' && method === 'POST') {
      if (overrides.loginError) {
        return json(route, { error: overrides.loginError.message }, overrides.loginError.status);
      }
      const body = (req.postDataJSON?.() ?? {}) as { email?: string; password?: string };
      if (body.password === 'wrong') {
        return json(route, { error: 'Invalid credentials' }, 401);
      }
      return json(route, { ok: true });
    }
    if (path === 'auth/logout' && method === 'POST') return json(route, { ok: true });
    if (path === 'auth/mfa/verify' && method === 'POST') return json(route, { ok: true });
    if (path === 'auth/mfa/status' && method === 'GET')
      return json(route, { mfa_status: { enrolled: false, enabled: false } });

    // ----- Console capabilities -----
    if (path === 'console/capabilities')
      return json(route, { admin_console_capabilities: DEFAULT_CAPABILITIES });

    // ----- Companies (collection / detail) -----
    if (path === 'companies' && method === 'GET')
      return json(route, { companies, total: companies.length });
    if (path === 'companies' && method === 'POST')
      return json(route, { company: { ...companyDetail, id: `co-${Date.now()}` } }, 201);
    if (segs[0] === 'companies' && segs.length === 2 && method === 'GET')
      return json(route, { company: companyDetail });
    if (segs[0] === 'companies' && segs.length === 2 && method === 'PATCH')
      return json(route, { company: companyDetail });
    if (segs[0] === 'companies' && segs.length === 2 && method === 'DELETE')
      return empty(route, 204);

    // Dashboard endpoints (several possible shapes — return rich object that satisfies hooks)
    if (segs[0] === 'companies' && segs[2] === 'dashboard')
      return json(route, dashboard);
    if (segs[0] === 'companies' && segs[2] === 'stats')
      return json(route, dashboard.stats);

    // ----- Domains -----
    if (segs[0] === 'companies' && segs[2] === 'domains') {
      if (method === 'GET') return json(route, { domains, total: domains.length });
      if (method === 'POST')
        return json(route, { domain: { ...domains[0], id: `dom-${Date.now()}` } }, 201);
    }
    if (segs[0] === 'companies' && segs[2] === 'domains' && segs.length === 4) {
      if (method === 'GET') return json(route, { domain: domains[0] });
      if (method === 'PATCH') return json(route, { domain: domains[0] });
      if (method === 'DELETE') return empty(route, 204);
    }
    if (segs[0] === 'companies' && segs[2] === 'domains' && segs[4] === 'dns-check')
      return json(route, { status: 'pass', records: [] });

    // ----- Users -----
    if (segs[0] === 'companies' && segs[2] === 'users') {
      if (method === 'GET') return json(route, { users, total: users.length });
      if (method === 'POST')
        return json(route, { user: { ...users[0], id: `user-${Date.now()}` } }, 201);
    }
    if (segs[0] === 'companies' && segs[2] === 'users' && segs.length === 4) {
      if (method === 'GET') return json(route, { user: users[0] });
      if (method === 'PATCH') return json(route, { user: users[0] });
      if (method === 'DELETE') return empty(route, 204);
    }
    if (segs[0] === 'companies' && segs[2] === 'admin-users')
      return json(route, { users, admin_users: users, total: users.length });

    // ----- Mail flow logs / outbox / message-trace / delivery attempts / routing rules -----
    if (segs[0] === 'companies' && segs[2] === 'mail-flow-logs')
      return json(route, { logs: mailFlowLogs, mail_flow_logs: mailFlowLogs, total: mailFlowLogs.length });
    if (segs[0] === 'companies' && segs[2] === 'outbox')
      return json(route, { events: [], total: 0 });
    if (segs[0] === 'companies' && segs[2] === 'message-trace')
      return json(route, { events: [], total: 0 });
    if (segs[0] === 'companies' && segs[2] === 'delivery-attempts')
      return json(route, { attempts: [], total: 0 });
    if (segs[0] === 'companies' && segs[2] === 'routing-rules')
      return json(route, { rules: [], total: 0 });

    // ----- Delivery -----
    if (segs[0] === 'companies' && segs[2] === 'delivery-routes') {
      if (method === 'GET') return json(route, { routes: deliveryRoutes, total: deliveryRoutes.length });
      if (method === 'POST') return json(route, { route: deliveryRoutes[0] }, 201);
    }
    if (segs[0] === 'companies' && segs[2] === 'relays') {
      if (method === 'GET') return json(route, { relays, total: relays.length });
      if (method === 'POST') return json(route, { relay: relays[0] }, 201);
    }
    if (segs[0] === 'companies' && segs[2] === 'suppression-list')
      return json(route, { entries: [], total: 0 });

    // ----- Audit -----
    if (segs[0] === 'companies' && segs[2] === 'audit-logs')
      return json(route, { logs: auditLogs, audit_logs: auditLogs, total: auditLogs.length });
    if (segs[0] === 'companies' && segs[2] === 'admin-activity')
      return json(route, { activities: [], total: 0 });

    // ----- Alerts -----
    if (segs[0] === 'companies' && segs[2] === 'alerts') {
      if (method === 'GET') return json(route, { alerts, alert_rules: alerts, rules: alerts, total: alerts.length });
      if (method === 'POST') return json(route, { alert: alerts[0] }, 201);
    }
    if (segs[0] === 'companies' && segs[2] === 'alerts' && segs.length === 4) {
      if (method === 'PATCH') return json(route, { alert: alerts[0] });
      if (method === 'DELETE') return empty(route, 204);
    }

    // ----- Storage / quotas -----
    if (segs[0] === 'companies' && segs[2] === 'storage-usage')
      return json(route, storageUsage);
    if (segs[0] === 'companies' && segs[2] === 'storage')
      return json(route, storageUsage);
    if (segs[0] === 'companies' && segs[2] === 'quota-usage')
      return json(route, { entries: storageUsage.per_user });
    if (segs[0] === 'companies' && segs[2] === 'quota-alerts')
      return json(route, { alerts: [] });
    if (segs[0] === 'companies' && segs[2] === 'quota-reconciliation')
      return json(route, { entries: [] });
    if (segs[0] === 'companies' && segs[2] === 'attachments')
      return json(route, { attachments: [], total: 0 });
    if (segs[0] === 'companies' && segs[2] === 'drive')
      return json(route, { nodes: [] });
    if (segs[0] === 'companies' && segs[2] === 'seat-usage')
      return json(route, { seats: [], total: 0 });

    // ----- Security -----
    if (path === 'security/posture' || path.endsWith('/security/posture'))
      return json(route, posture);
    if (path === 'security/audit-policy' || path.endsWith('/security/audit-policy'))
      return json(route, { policy: DEFAULT_AUDIT_POLICY });
    if (path === 'security/ip-policy' || path.endsWith('/security/ip-policy'))
      return json(route, { policy: DEFAULT_IP_POLICY });
    if (path.endsWith('/security/api-keys') || segs.includes('api-keys')) {
      if (method === 'GET') return json(route, { api_keys: apiKeys, keys: apiKeys, total: apiKeys.length });
      if (method === 'POST') return json(route, { api_key: { ...apiKeys[0], secret: 'gm_secret_xxx' } }, 201);
      if (method === 'DELETE') return empty(route, 204);
    }
    if (path.endsWith('/security/dkim-keys') || segs.includes('dkim-keys'))
      return json(route, { keys: [], dkim_keys: [] });
    if (path.endsWith('/security/suppression') || segs.includes('suppression'))
      return json(route, { entries: [] });
    if (path.endsWith('/security/alerts'))
      return json(route, { alerts, rules: alerts });
    if (path.endsWith('/security/mfa') || segs.includes('mfa'))
      return json(route, { mfa: { enabled: false } });
    if (path.endsWith('/security/sessions'))
      return json(route, { sessions: [] });
    if (path.endsWith('/security/rate-limits') || segs.includes('rate-limits'))
      return json(route, { limits: [] });
    if (path.endsWith('/security/retention'))
      return json(route, { policy: { mail_days: 90, audit_days: 90 } });
    if (path.endsWith('/security/auth-policy'))
      return json(route, { policy: { mfa_required: false, password_min_length: 8 } });
    if (path.endsWith('/security/api-settings'))
      return json(route, { settings: {} });
    if (path.endsWith('/security/ip-access'))
      return json(route, { policy: DEFAULT_IP_POLICY });
    if (path.endsWith('/security/dmarc'))
      return json(route, { policy: { p: 'none', sp: 'none', pct: 100 } });
    if (path.endsWith('/security/spam-filter'))
      return json(route, { settings: {} });
    if (path.endsWith('/security/smtp-policy'))
      return json(route, { policy: {} });

    // ----- Roles -----
    if (segs.includes('roles')) {
      if (method === 'GET') return json(route, { roles, total: roles.length });
      if (method === 'POST') return json(route, { role: roles[0] }, 201);
      if (method === 'PATCH') return json(route, { role: roles[0] });
      if (method === 'DELETE') return empty(route, 204);
    }

    // ----- Organization sub-pages -----
    if (segs.includes('organization') || segs.includes('sso') || segs.includes('webhooks')
        || segs.includes('notification-templates') || segs.includes('signature')
        || segs.includes('idp-config') || segs.includes('scim-status'))
      return json(route, { ok: true, settings: {}, webhooks: [], templates: [], signatures: [], status: 'idle' });

    // ----- Tenancy -----
    if (segs.includes('tenancy') || segs.includes('domain-settings') || segs.includes('health')
        || segs.includes('change-history') || segs.includes('onboarding'))
      return json(route, { items: [], total: 0, status: 'ok' });

    // ----- Compliance / Monitoring / Reports / Analytics / Config -----
    if (segs.includes('compliance') || segs.includes('legal-holds'))
      return json(route, { holds: [], items: [], total: 0 });
    if (segs.includes('monitoring'))
      return json(route, { metrics: [], series: [] });
    if (segs.includes('reports'))
      return json(route, { reports: [], items: [] });
    if (segs.includes('analytics') || segs.includes('api-usage') || segs.includes('push'))
      return json(route, { stats: {}, series: [], items: [] });
    if (segs.includes('config'))
      return json(route, { config: {}, entries: [] });
    if (segs.includes('access') || segs.includes('directory') || segs.includes('aliases')
        || segs.includes('delegations') || segs.includes('groups'))
      return json(route, { items: [], total: 0, users: [], groups: [], aliases: [], delegations: [] });

    // ----- System -----
    if (path === 'system/metrics' || path.endsWith('/system/metrics'))
      return json(route, {
        memory: {
          heap_inuse_bytes: 0,
          heap_sys_bytes: 0,
          sys_bytes: 0,
          alloc_bytes: 0,
          gc_runs: 0,
          usage_pct: 0,
        },
        goroutines: 0,
        timestamp: new Date().toISOString(),
      });
    if (path === 'backpressure' || path.endsWith('/backpressure'))
      return json(route, { backpressure: { enabled: false, threshold: 0 } });
    if (segs.includes('system') || segs.includes('queue') || segs.includes('backpressure'))
      return json(route, { stats: {}, queue: { depth: 0, dead_letter: 0 }, items: [] });

    // ----- Generic fallback (success) -----
    return json(route, {
      ok: true,
      items: [],
      data: [],
      logs: [],
      events: [],
      total: 0,
      stats: {},
    });
  });
}

/**
 * Older helper preserved for compatibility with the existing spec files.
 * Just delegates to installMocks() with defaults.
 */
export async function installLocalAdminSession(page: Page, overrides: MockOverrides = {}) {
  return installMocks(page, overrides);
}
