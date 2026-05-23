import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

const SECURITY_PAGES = [
  '/companies/default/security/api-keys',
  '/companies/default/security/dkim-keys',
  '/companies/default/security/api-settings',
  '/companies/default/security/mfa',
  '/companies/default/security/ip-access',
  '/companies/default/security/auth-policy',
  '/companies/default/security/audit-policy',
  '/companies/default/security/retention',
  '/companies/default/security/sessions',
  '/companies/default/security/rate-limits',
  '/companies/default/security/dmarc',
  '/companies/default/security/spam-filter',
  '/companies/default/security/smtp-policy',
  '/companies/default/security/posture',
  '/companies/default/security/suppression',
];

test.describe('Security pages', () => {
  for (const path of SECURITY_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      await setupAuthedAdminPage(page, { gotoPath: path });
      await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
    });
  }

  test('global /settings/security renders', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/settings/security', noNavigate: false });
    await expect(page.locator('body')).toBeVisible();
  });
});
