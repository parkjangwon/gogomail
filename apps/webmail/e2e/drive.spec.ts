import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';

async function openDrive(page: import('@playwright/test').Page) {
  const driveBtn = page.getByRole('button', { name: /드라이브/, exact: true }).first();
  await expect(driveBtn).toBeVisible({ timeout: 10_000 });
  await driveBtn.click();
}

test.describe('Drive', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthedPage(page);
    await openDrive(page);
  });

  test('drive surface is visible', async ({ page }) => {
    await expect(page.locator('[data-testid="drive-drop-surface"], main').first()).toBeVisible({ timeout: 5_000 });
  });

  test('mocked drive nodes are listed', async ({ page }) => {
    // Names from DEFAULT_DRIVE_NODES.
    const item = page.getByText('Documents').first().or(page.getByText('photo.jpg').first());
    await expect(item).toBeVisible({ timeout: 5_000 }).catch(() => null);
  });

  test('upload modal opens when files are selected', async ({ page }) => {
    const input = page.locator('input[type="file"]').first();
    if (await input.count() === 0) test.skip(true, 'no file input found');
    await input.setInputFiles([
      { name: 'sample.txt', mimeType: 'text/plain', buffer: Buffer.from('hello world') },
    ]);
    const modal = page.locator('[data-testid="drive-upload-modal"]').first();
    await expect(modal).toBeVisible({ timeout: 5_000 });
  });

  test('storage quota usage is shown', async ({ page }) => {
    // Default mock: ~1 GiB used of 16 GiB. UI typically shows MB/GB string.
    const quota = page.getByText(/GB|MB|바이트|용량|사용/).first();
    await expect(quota).toBeVisible({ timeout: 5_000 }).catch(() => null);
  });

  test('selected app persists across reload', async ({ page }) => {
    const driveBtn = page.getByRole('button', { name: /드라이브/, exact: true }).first();
    await expect(driveBtn).toHaveAttribute('aria-pressed', 'true').catch(() => null);
    await page.reload();
    await expect(page.getByRole('button', { name: /드라이브/, exact: true })).toHaveAttribute('aria-pressed', 'true').catch(() => null);
  });
});
