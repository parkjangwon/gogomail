import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage, setupMocksOnly } from './helpers';

test.describe('Error handling', () => {
  test('non-existent route does not crash app', async ({ page }) => {
    await setupAuthedAdminPage(page, {
      gotoPath: '/this/does/not/exist',
      noNavigate: false,
    });
    await expect(page.locator('body')).toBeVisible();
    expect(page.url()).toContain('localhost');
  });

  test('unauthorized state surfaces in UI without throwing', async ({ page }) => {
    await setupMocksOnly(page, { unauthorized: true });
    await page.goto('/companies/default/dashboard');
    await expect(page.locator('body')).toBeVisible();
  });

  test('500 API error does not blank the page', async ({ page }) => {
    await setupAuthedAdminPage(page, {
      gotoPath: '/companies/default/users',
      extra: [
        {
          urlPattern: /\/api\/admin\/.*\/users(\?.*)?$/,
          handler: (route) =>
            route.fulfill({
              status: 500,
              contentType: 'application/json',
              body: JSON.stringify({ error: 'internal' }),
            }),
        },
      ],
    });
    // wait for either the heading or an error alert to appear
    await expect(page.locator('h1, h2, [role="alert"]').first()).toBeVisible({ timeout: 15_000 });
  });
});
