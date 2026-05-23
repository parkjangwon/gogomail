import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('Companies list', () => {
  test('tenancy/companies renders heading', async ({ page }) => {
    await setupAuthedAdminPage(page, {
      gotoPath: '/companies/default/tenancy/companies',
    });
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });

  test('company overview page loads', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default' });
    await expect(page.locator('body')).toBeVisible();
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });
});
