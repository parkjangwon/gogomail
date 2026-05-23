import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('Domains', () => {
  test('tenancy/domains renders', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default/tenancy/domains' });
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });

  test('tenancy/domain-settings renders', async ({ page }) => {
    await setupAuthedAdminPage(page, {
      gotoPath: '/companies/default/tenancy/domain-settings',
    });
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });
});
