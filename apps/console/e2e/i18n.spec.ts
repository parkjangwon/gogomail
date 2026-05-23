import { test, expect } from '@playwright/test';
import { setupAuthedAdminPage } from './helpers';

test.describe('i18n locales', () => {
  test('Korean dashboard renders Korean heading', async ({ page }) => {
    await setupAuthedAdminPage(page, { locale: 'ko' });
    await expect(page.getByRole('heading', { name: '대시보드' })).toBeVisible({ timeout: 15_000 });
  });

  test('English dashboard renders English heading', async ({ page }) => {
    await setupAuthedAdminPage(page, { locale: 'en' });
    await expect(page.getByRole('heading', { name: /Dashboard/i })).toBeVisible({ timeout: 15_000 });
  });

  test('Japanese locale loads without crash', async ({ page }) => {
    await setupAuthedAdminPage(page, { locale: 'ja' });
    await expect(page.locator('body')).toBeVisible();
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });

  test('Chinese locale loads without crash', async ({ page }) => {
    await setupAuthedAdminPage(page, { locale: 'zh-CN' });
    await expect(page.locator('body')).toBeVisible();
    await expect(page.getByRole('heading').first()).toBeVisible({ timeout: 10_000 });
  });
});
