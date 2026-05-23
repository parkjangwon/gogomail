import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('Audit logs', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthedAdminPage(page, { gotoPath: '/companies/default/audit-logs' });
  });

  test('renders audit logs heading', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /Audit Logs|감사 로그/ })).toBeVisible({ timeout: 10_000 });
  });

  test('renders at least one input (filter)', async ({ page }) => {
    await expect(page.locator('input').first()).toBeVisible({ timeout: 10_000 });
  });

  test('filtering keeps the page stable', async ({ page }) => {
    const input = page.locator('input').first();
    if (await input.isVisible()) {
      await input.fill('zzz-nothing-matches');
      await page.waitForTimeout(200);
    }
    await expect(page.getByRole('heading', { name: /Audit Logs|감사 로그/ })).toBeVisible();
  });
});
