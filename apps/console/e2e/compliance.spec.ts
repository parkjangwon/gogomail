import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('Compliance pages', () => {
  test('compliance dashboard renders', async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default/compliance' });
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });

  test('legal holds renders', async ({ page }) => {
    await setupAuthedAdminPage(page, {
      gotoPath: '/companies/default/compliance/legal-holds',
    });
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });
});
