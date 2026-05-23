import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('Alerts', () => {
  test('alerts page renders', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default/alerts' });
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });

  test('security/alerts renders', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default/security/alerts' });
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });
});
