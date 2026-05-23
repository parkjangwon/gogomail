import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

const TENANCY_PAGES = [
  '/companies/default/tenancy/companies',
  '/companies/default/tenancy/domains',
  '/companies/default/tenancy/domain-settings',
  '/companies/default/tenancy/health',
  '/companies/default/tenancy/change-history',
  '/companies/default/tenancy/onboarding',
];

test.describe('Tenancy pages', () => {
  for (const path of TENANCY_PAGES) {
    test(`renders ${path}`, async ({ page }) => {
      await setupAuthedAdminPage(page, { gotoPath: path });
      await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
    });
  }
});
