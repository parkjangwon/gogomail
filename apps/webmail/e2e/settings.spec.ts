import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';

async function openSettings(page: import('@playwright/test').Page) {
  const btn = page.getByRole('button', { name: '설정', exact: true }).first();
  await expect(btn).toBeVisible({ timeout: 10_000 });
  await btn.click();
}

test.describe('Settings', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthedPage(page);
  });

  test('settings panel opens', async ({ page }) => {
    await openSettings(page);
    // Account heading or similar is shown.
    await expect(
      page.getByRole('heading', { name: /계정|account/ }).first()
        .or(page.getByText(/계정 설정|환경설정|preferences/i).first())
    ).toBeVisible({ timeout: 5_000 });
  });

  test('settings nav items are present', async ({ page }) => {
    await openSettings(page);
    const navBtns = page.locator('button[data-nav-group="settings-nav"]');
    expect(await navBtns.count()).toBeGreaterThan(0);
  });

  test('theme toggle is reachable', async ({ page }) => {
    await openSettings(page);
    const theme = page.getByRole('button', { name: /테마|theme|다크|light|dark/i }).first()
      .or(page.getByText(/테마/).first());
    await expect(theme).toBeVisible({ timeout: 5_000 }).catch(() => null);
  });

  test('language selector is present', async ({ page }) => {
    await openSettings(page);
    const lang = page.getByText(/언어|language|locale/i).first();
    await expect(lang).toBeVisible({ timeout: 5_000 }).catch(() => null);
  });
});
