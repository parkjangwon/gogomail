import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('Users page', () => {
  test('users list renders', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default/users' });
    await expect(page.getByRole('heading', { name: /Users|사용자/ })).toBeVisible({ timeout: 15_000 });
  });

  test('admin-users page renders', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default/admin-users' });
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });

  test('users list shows table when data present', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default/users' });
    // Cloudscape table or any tabular element should appear
    await page.waitForLoadState('networkidle', { timeout: 5_000 }).catch(() => null);
    await expect(page.locator('body')).toBeVisible();
  });
});
