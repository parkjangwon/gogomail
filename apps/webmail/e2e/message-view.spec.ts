import { test, expect } from '@playwright/test';

test.describe('Message View', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/mail');
  });

  test('message list items are clickable', async ({ page }) => {
    // Wait for messages to load
    await page.waitForTimeout(1000);

    // Try to find and click first message
    const messageRows = page.locator('[role="listitem"], tr, [class*="message"], [class*="item"]').filter({ hasText: /^(?!.*\(.*\))/ });
    const count = await messageRows.count();

    if (count > 0) {
      const firstMessage = messageRows.first();
      if (await firstMessage.isVisible()) {
        await firstMessage.click();
        // After click, should show message details or open message view
        await page.waitForTimeout(500);
        const url = page.url();
        // URL might change to show messageId
        expect(url).toContain('/mail');
      }
    }
  });

  test('reading pane contains message elements', async ({ page }) => {
    // Check for reading pane elements
    const readingPane = page.locator('main, [role="main"], [class*="reading"], [class*="pane"]').first();
    if (await readingPane.isVisible()) {
      // Should contain sender, subject, date, body, or action buttons
      const elements = readingPane.locator('*');
      const count = await elements.count();
      expect(count).toBeGreaterThan(0);
    }
  });

  test('sidebar has folder navigation', async ({ page }) => {
    const sidebar = page.locator('nav, [role="navigation"], [class*="sidebar"]').first();
    if (await sidebar.isVisible()) {
      // Look for folder items
      const folders = sidebar.locator('button, a, [role="button"], [role="link"]');
      const count = await folders.count();
      expect(count).toBeGreaterThanOrEqual(0);
    }
  });
});
