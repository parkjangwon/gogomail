import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';

test.describe('Responsive layout', () => {
  test('desktop (1280×800) shows sidebar + main', async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await setupAuthedPage(page);
    await expect(page.locator('aside[aria-label="메일 탐색"]').first()).toBeVisible({ timeout: 10_000 });
    await expect(page.locator('main, [role="main"]').first()).toBeVisible();
  });

  test('tablet (768×1024) keeps main visible', async ({ page }) => {
    await page.setViewportSize({ width: 768, height: 1024 });
    await setupAuthedPage(page);
    await expect(page.locator('main, [role="main"]').first()).toBeVisible({ timeout: 10_000 });
  });

  test('mobile (375×667) renders without errors', async ({ page }) => {
    await page.setViewportSize({ width: 375, height: 667 });
    await setupAuthedPage(page);
    await expect(page.locator('main, [role="main"]').first()).toBeVisible({ timeout: 10_000 });
    const buttons = page.locator('button');
    expect(await buttons.count()).toBeGreaterThan(0);
  });

  test('layout adjusts on resize', async ({ page }) => {
    await setupAuthedPage(page);
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.waitForTimeout(200);
    await expect(page.locator('main, [role="main"]').first()).toBeVisible();
    await page.setViewportSize({ width: 375, height: 667 });
    await page.waitForTimeout(200);
    await expect(page.locator('main, [role="main"]').first()).toBeVisible();
  });
});
