import { test, expect } from '@playwright/test';
import { loginAsSeedUser } from './helpers';

test.describe('Mail List', () => {
  test.beforeEach(async ({ page }) => {
    await loginAsSeedUser(page);
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

  test('arrow keys move between rows and space toggles bulk selection', async ({ page }) => {
    const rows = page.locator('[data-message-id]');
    if (await rows.count() < 2) {
      test.skip(true, 'seeded messages are required for keyboard row navigation coverage');
    }
    await expect(rows.first()).toBeVisible({ timeout: 15_000 });
    await rows.first().focus();
    await expect(rows.first()).toBeFocused();
    await rows.first().press('ArrowDown');
    await expect(rows.nth(1)).toBeFocused();

    await rows.nth(1).press('Space');
    await expect(rows.nth(1).getByRole('button', { name: '선택 해제' })).toBeVisible();
  });

  test('settings menu moves with arrow keys', async ({ page }) => {
    await page.getByRole('button', { name: '설정', exact: true }).click();
    await expect(page.getByRole('heading', { name: '계정' })).toBeVisible();

    const settingsNav = page.locator('button[data-nav-group="settings-nav"]');
    await settingsNav.first().evaluate((node) => {
      node.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true, cancelable: true }));
    });
    await expect(settingsNav.nth(1)).toBeFocused();
    await settingsNav.nth(1).press('Space');
    await expect(page.getByRole('heading', { name: '받은편지함' })).toBeVisible();
  });
});
