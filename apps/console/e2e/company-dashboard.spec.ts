import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('Company dashboard', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthedAdminPage(page);
  });

  test('renders dashboard heading', async ({ page }) => {
    await expect(page.getByRole('heading', { name: /Dashboard|대시보드/ })).toBeVisible({ timeout: 15_000 });
  });

  test('shows quick action / metric content', async ({ page }) => {
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 15_000 });
    const text = (await page.locator('body').innerText()).toLowerCase();
    expect(text.length).toBeGreaterThan(20);
    expect(text).not.toContain('this page could not be found');
  });
});
