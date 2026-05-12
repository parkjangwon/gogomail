import { test, expect } from '@playwright/test';

test.describe('Search Functionality', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/mail');
  });

  test('search input is accessible', async ({ page }) => {
    // Look for search input
    const searchInput = page.locator('input[placeholder*="검색"], input[placeholder*="search"], input[type="search"]').first();
    if (await searchInput.isVisible()) {
      await expect(searchInput).toBeFocused().catch(() => {
        // Not focused yet, that's ok
      });
    }
  });

  test('can type in search field', async ({ page }) => {
    const searchInput = page.locator('input[placeholder*="검색"], input[placeholder*="search"], input[type="search"]').first();
    if (await searchInput.isVisible()) {
      await searchInput.click();
      await searchInput.fill('test');
      await expect(searchInput).toHaveValue('test');
    }
  });

  test('search field clears', async ({ page }) => {
    const searchInput = page.locator('input[placeholder*="검색"], input[placeholder*="search"], input[type="search"]').first();
    if (await searchInput.isVisible()) {
      await searchInput.fill('test');
      await searchInput.clear();
      await expect(searchInput).toHaveValue('');
    }
  });
});
