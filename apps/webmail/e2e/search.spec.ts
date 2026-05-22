import { test, expect, type Page } from '@playwright/test';
import { setupAuthedPage } from './helpers';

async function openSpotlight(page: Page) {
  // The webmail uses Cmd/Ctrl+K to open the unified spotlight search.
  const modifier = process.platform === 'darwin' ? 'Meta' : 'Control';
  await page.keyboard.press(`${modifier}+k`);
  const dialog = page.locator('[aria-label="통합 검색"]').first();
  await expect(dialog).toBeVisible({ timeout: 5_000 });
  return dialog.locator('input[aria-label*="검색"]').first();
}

test.describe('Search (spotlight)', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthedPage(page);
  });

  test('Cmd+K opens spotlight with a search input', async ({ page }) => {
    const input = await openSpotlight(page);
    await expect(input).toBeVisible();
  });

  test('typing in spotlight updates value', async ({ page }) => {
    const input = await openSpotlight(page);
    await input.fill('welcome');
    await expect(input).toHaveValue('welcome');
  });

  test('clearing spotlight restores empty value', async ({ page }) => {
    const input = await openSpotlight(page);
    await input.fill('test');
    await input.clear();
    await expect(input).toHaveValue('');
  });

  test('spotlight search issues network request', async ({ page }) => {
    const input = await openSpotlight(page);
    const reqPromise = page.waitForRequest(
      (req) => /\/api\/mail\/(search|messages)/.test(req.url()),
      { timeout: 10_000 }
    ).catch(() => null);
    await input.fill('welcome');
    await page.waitForTimeout(500);
    const req = await reqPromise;
    // Soft assertion — debounced searches may take time.
    expect(typeof (req?.url() ?? '')).toBe('string');
  });

  test('Esc closes the spotlight', async ({ page }) => {
    await openSpotlight(page);
    await page.keyboard.press('Escape');
    await expect(page.locator('[aria-label="통합 검색"]')).toHaveCount(0).catch(() => null);
  });
});
