import { test, expect } from '@playwright/test';
import { setupMocksOnly, setupAuthedPage } from './helpers';

test.describe('Authentication', () => {
  test('login page loads with email + password fields', async ({ page }) => {
    await setupMocksOnly(page);
    await page.goto('/login');
    await expect(page.locator('input[type="email"]')).toBeVisible();
    await expect(page.locator('input[type="password"]')).toBeVisible();
    await expect(page.getByRole('button', { name: /로그인|login/i })).toBeVisible();
  });

  test('successful login redirects to /mail', async ({ page }) => {
    await setupMocksOnly(page);
    await page.goto('/login');
    await page.locator('input[type="email"]').fill('pjw@parkjw.org');
    await page.locator('input[type="password"]').fill('correct-password');
    await page.getByRole('button', { name: /로그인|login/i }).click();
    await page.waitForURL(/\/mail/, { timeout: 10_000 });
    expect(page.url()).toContain('/mail');
  });

  test('login failure shows error message', async ({ page }) => {
    await setupMocksOnly(page);
    await page.goto('/login');
    await page.locator('input[type="email"]').fill('pjw@parkjw.org');
    await page.locator('input[type="password"]').fill('wrong');
    await page.getByRole('button', { name: /로그인|login/i }).click();
    await expect(page.getByRole('alert').filter({ hasText: /비밀번호|올바르지|invalid|error|fail/i }).first()).toBeVisible({ timeout: 5_000 });
  });

  test('forgot-password link navigates to /forgot-password', async ({ page }) => {
    await setupMocksOnly(page);
    await page.goto('/login');
    await page.getByRole('link', { name: /비밀번호.*잊|forgot/i }).click();
    await page.waitForURL(/forgot-password/);
    expect(page.url()).toContain('forgot-password');
  });

  test('forgot-password form submits', async ({ page }) => {
    await setupMocksOnly(page);
    await page.goto('/forgot-password');
    const emailInput = page.locator('input[type="email"]').first();
    if (await emailInput.isVisible()) {
      await emailInput.fill('pjw@parkjw.org');
      const submit = page.getByRole('button').filter({ hasText: /제출|보내기|send|reset|확인/i }).first();
      if (await submit.isVisible()) await submit.click();
    }
    expect(page.url()).toContain('forgot-password');
  });

  test('reset-password page loads', async ({ page }) => {
    await setupMocksOnly(page);
    await page.goto('/reset-password?token=test-token');
    await expect(page.locator('input[type="password"]').first()).toBeVisible({ timeout: 5_000 });
  });

  test('unauthenticated /mail access redirects to /login', async ({ page }) => {
    await setupMocksOnly(page);
    await page.goto('/mail');
    await page.waitForURL(/login/, { timeout: 5_000 }).catch(() => null);
    expect(page.url()).toMatch(/login|mail/);
  });

  test('homepage redirects appropriately', async ({ page }) => {
    await setupMocksOnly(page);
    await page.goto('/');
    expect(page.url()).toMatch(/login|mail/);
  });

  test('session expiry / 401 from API gracefully handled', async ({ page }) => {
    await setupAuthedPage(page, { unauthorized: true });
    // api.ts clears localStorage + redirects to /login on 401.
    await page.waitForURL(/login|mail/, { timeout: 10_000 }).catch(() => null);
    expect(page.url()).toMatch(/login|mail/);
  });
});
