import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

const BASE_URL = 'http://localhost:3001';

// All 38 admin console pages
const PAGES = [
  { name: 'Login', path: '/login', requiresAuth: false },
  { name: 'Dashboard', path: '/companies/default/dashboard', requiresAuth: true },

  // System (3)
  { name: 'System > Queue Stats', path: '/companies/default/system/queue', requiresAuth: true },
  { name: 'System > Backpressure', path: '/companies/default/system/backpressure', requiresAuth: true },
  { name: 'System > API Health', path: '/companies/default/system/health', requiresAuth: true },

  // Tenancy (3)
  { name: 'Tenancy > Companies', path: '/companies/default/tenancy/companies', requiresAuth: true },
  { name: 'Tenancy > Domains', path: '/companies/default/tenancy/domains', requiresAuth: true },
  { name: 'Tenancy > Domain Settings', path: '/companies/default/tenancy/domain-settings', requiresAuth: true },

  // Users & Access (6)
  { name: 'Users & Access > Users', path: '/companies/default/users', requiresAuth: true },
  { name: 'Users & Access > Admin Users', path: '/companies/default/admin-users', requiresAuth: true },
  { name: 'Users & Access > Directory', path: '/companies/default/access/directory', requiresAuth: true },
  { name: 'Users & Access > Aliases', path: '/companies/default/access/aliases', requiresAuth: true },
  { name: 'Users & Access > Delegations', path: '/companies/default/access/delegations', requiresAuth: true },
  { name: 'Users & Access > Group Memberships', path: '/companies/default/access/groups', requiresAuth: true },

  // Delivery & Mail (5)
  { name: 'Delivery & Mail > Delivery Routes', path: '/companies/default/delivery/routes', requiresAuth: true },
  { name: 'Delivery & Mail > Trusted Relays', path: '/companies/default/delivery/relays', requiresAuth: true },
  { name: 'Delivery & Mail > Mail Flow Logs', path: '/companies/default/mail/flow-logs', requiresAuth: true },
  { name: 'Delivery & Mail > Outbox Events', path: '/companies/default/mail/outbox', requiresAuth: true },
  { name: 'Delivery & Mail > Delivery Attempts', path: '/companies/default/mail/delivery-attempts', requiresAuth: true },

  // Security (5)
  { name: 'Security > API Keys', path: '/companies/default/security/api-keys', requiresAuth: true },
  { name: 'Security > DKIM Keys', path: '/companies/default/security/dkim-keys', requiresAuth: true },
  { name: 'Security > Audit Logs', path: '/companies/default/audit-logs', requiresAuth: true },
  { name: 'Security > Suppression List', path: '/companies/default/security/suppression', requiresAuth: true },
  { name: 'Security > Alert Rules', path: '/companies/default/security/alerts', requiresAuth: true },

  // Storage & Quotas (5)
  { name: 'Storage & Quotas > Quota Usage', path: '/companies/default/storage/quota-usage', requiresAuth: true },
  { name: 'Storage & Quotas > Quota Alerts', path: '/companies/default/storage/quota-alerts', requiresAuth: true },
  { name: 'Storage & Quotas > Attachments', path: '/companies/default/storage/attachments', requiresAuth: true },
  { name: 'Storage & Quotas > Drive', path: '/companies/default/storage/drive', requiresAuth: true },
  { name: 'Storage & Quotas > Quota Reconciliation', path: '/companies/default/storage/reconciliation', requiresAuth: true },

  // Analytics (3)
  { name: 'Analytics > API Usage', path: '/companies/default/analytics/api-usage', requiresAuth: true },
  { name: 'Analytics > Push Notifications', path: '/companies/default/analytics/push', requiresAuth: true },
  { name: 'Analytics > Reports', path: '/companies/default/reports', requiresAuth: true },

  // Configuration (3)
  { name: 'Configuration > Company Config', path: '/companies/default/config/company', requiresAuth: true },
  { name: 'Configuration > Domain Config', path: '/companies/default/config/domain', requiresAuth: true },
  { name: 'Configuration > User Config', path: '/companies/default/config/user', requiresAuth: true },

  // Organization (3)
  { name: 'Organization > Settings', path: '/companies/default/organization', requiresAuth: true },
  { name: 'Organization > Roles', path: '/companies/default/roles', requiresAuth: true },
  { name: 'Organization > Compliance', path: '/companies/default/compliance', requiresAuth: true },
];

test.describe('Admin Console E2E Tests', () => {
  // Allow extra time for Next.js dev-mode on-demand compilation of each route.
  test.setTimeout(60_000);

  PAGES.forEach((pageConfig, index) => {
    test(`[${index + 1}/${PAGES.length}] ${pageConfig.name}`, async ({ page }) => {
      const url = `${BASE_URL}${pageConfig.path}`;
      const consoleErrors: string[] = [];

      page.on('console', msg => {
        if (msg.type() === 'error') {
          consoleErrors.push(msg.text());
        }
      });

      try {
        // Set up auth state (localStorage + cookie) and install API mocks before
        // navigation so CompanyLayout authorises without redirecting to /login.
        if (pageConfig.requiresAuth) {
          await setupAuthedAdminPage(page, { noNavigate: true });
        }

        await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 50_000 });
        await page.waitForTimeout(300);

        const finalUrl = page.url();
        expect(finalUrl).toContain(pageConfig.path);

        const pageContent = await page.content();
        expect(pageContent.length).toBeGreaterThan(100);

        console.log(`✅ ${pageConfig.name}`);
        if (consoleErrors.length > 0) {
          console.log(`  ⚠️ Console errors: ${consoleErrors.join('; ')}`);
        }
      } catch (error) {
        console.log(`❌ ${pageConfig.name}: ${error}`);
        throw error;
      }
    });
  });
});
