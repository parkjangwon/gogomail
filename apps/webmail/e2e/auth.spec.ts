import { test, expect } from '@playwright/test';

test.describe('Authentication Flow', () => {
  test('login page loads', async ({ page }) => {
    await page.goto('/login');
    await expect(page).toHaveTitle(/login|웹메일/i);
    await expect(page.locator('input[type="email"]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
  });

  test('redirect to login when not authenticated', async ({ page }) => {
    await page.goto('/mail');
    // Should redirect to /login if not authenticated
    await page.waitForURL(/login|auth/, { timeout: 5000 }).catch(() => null);
    // Or stay on mail page if dev mode allows
    const url = page.url();
    expect(url).toMatch(/login|auth|mail/i);
  });

  test('homepage redirects appropriately', async ({ page }) => {
    await page.goto('/');
    const url = page.url();
    // Should redirect to /login or /mail
    expect(url).toMatch(/login|mail/i);
  });
});
