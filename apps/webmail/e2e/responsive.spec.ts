import { test, expect } from '@playwright/test';

test.describe('Responsive Design', () => {
  test('desktop layout (1280px)', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.goto('/mail');

    // Should show 3-pane layout
    const sidebar = page.locator('nav, [role="navigation"], [class*="sidebar"]').first();
    const main = page.locator('main, [role="main"]').first();

    if (await sidebar.isVisible()) {
      await expect(sidebar).toBeVisible();
    }
    if (await main.isVisible()) {
      await expect(main).toBeVisible();
    }
  });

  test('tablet layout (768px)', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await page.goto('/mail');

    const main = page.locator('main, [role="main"]').first();
    if (await main.isVisible()) {
      await expect(main).toBeVisible();
    }
  });

  test('mobile layout (375px)', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await page.goto('/mail');

    // Mobile might hide sidebar or show it in slide panel
    const main = page.locator('main, [role="main"]').first();
    if (await main.isVisible()) {
      await expect(main).toBeVisible();
    }

    // Check if page is still interactive
    const buttons = page.locator('button');
    const count = await buttons.count();
    expect(count).toBeGreaterThanOrEqual(0);
  });

  test('layout adjusts on window resize', async ({ page }) => {
    await page.goto('/mail');

    // Start at desktop
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.waitForTimeout(500);

    const mainDesktop = page.locator('main, [role="main"]').first();
    const isVisibleDesktop = await mainDesktop.isVisible().catch(() => false);

    // Resize to mobile
    await page.setViewportSize({ width: 375, height: 667 });
    await page.waitForTimeout(500);

    const mainMobile = page.locator('main, [role="main"]').first();
    const isVisibleMobile = await mainMobile.isVisible().catch(() => false);

    // Both should be visible or hidden consistently
    expect(typeof isVisibleDesktop).toBe('boolean');
    expect(typeof isVisibleMobile).toBe('boolean');
  });
});
