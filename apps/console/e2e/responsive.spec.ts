import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('Responsive layout', () => {
  test('renders on desktop viewport', async ({ page }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await setupAuthedAdminPage(page);
    await expect(page.locator('body')).toBeVisible();
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });

  test('renders on tablet viewport', async ({ page }) => {
    await page.setViewportSize({ width: 820, height: 1180 });
    await setupAuthedAdminPage(page);
    await expect(page.locator('body')).toBeVisible();
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });

  test('renders on mobile viewport', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await setupAuthedAdminPage(page);
    await expect(page.locator('body')).toBeVisible();
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });
});
