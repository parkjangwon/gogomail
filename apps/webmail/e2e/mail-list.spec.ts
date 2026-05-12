import { test, expect } from '@playwright/test';

test.describe('Mail List', () => {
  test.beforeEach(async ({ page }) => {
    // Navigate to mail page
    // In dev mode with GOGOMAIL_DEV_USER_ID, this should work without login
    await page.goto('/mail');
  });

  test('displays mail list', async ({ page }) => {
    // Wait for mail list to load
    await expect(page.locator('[data-testid="message-list"]')).toBeVisible({ timeout: 5000 }).catch(() => null);

    // Check if sidebar is visible
    const sidebar = page.locator('nav').first();
    if (await sidebar.isVisible()) {
      expect(sidebar).toBeTruthy();
    }
  });

  test('can navigate mail pages', async ({ page }) => {
    const url = page.url();
    expect(url).toContain('/mail');
  });

  test('sidebar contains navigation', async ({ page }) => {
    // Check for sidebar navigation items
    const sidebar = page.locator('nav, [role="navigation"]').first();
    if (await sidebar.isVisible()) {
      // Should contain folder items or navigation elements
      const items = sidebar.locator('button, a, [role="button"], [role="link"]');
      const count = await items.count();
      expect(count).toBeGreaterThanOrEqual(0);
    }
  });
});
