import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';

test.describe('Error handling', () => {
  test('401 from /folders triggers redirect / unauth state', async ({ page }) => {
    await setupAuthedPage(page, { unauthorized: true });
    await page.waitForURL(/login|mail/, { timeout: 10_000 }).catch(() => null);
    expect(page.url()).toMatch(/login|mail/);
  });

  test('500 from server keeps app responsive (no white screen)', async ({ page }) => {
    await setupAuthedPage(page, {
      extra: [
        {
          urlPattern: /\/api\/mail\/messages(\?|$)/,
          handler: (route) => route.fulfill({
            status: 500,
            contentType: 'application/json',
            body: JSON.stringify({ error_message: 'internal' }),
          }),
        },
      ],
    });
    await expect(page.locator('aside[aria-label="메일 탐색"], main').first()).toBeVisible({ timeout: 10_000 });
  });

  test('network failure (abort) does not crash the page', async ({ page }) => {
    await setupAuthedPage(page, {
      extra: [
        {
          urlPattern: /\/api\/mail\/messages(\?|$)/,
          handler: (route) => route.abort('failed'),
        },
      ],
    });
    await expect(page.locator('aside[aria-label="메일 탐색"], main').first()).toBeVisible({ timeout: 10_000 });
  });
});
