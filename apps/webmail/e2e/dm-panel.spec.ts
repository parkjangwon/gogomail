import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';

test.describe('DM panel', () => {
  test('opens from the lower rail as a modal and sends a message', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();

    await expect(page.getByRole('heading', { name: 'DM' })).toBeVisible();
    await expect(page.getByText('Launch room').first()).toBeVisible();
    await expect(page.getByText('DM smoke hello').first()).toBeVisible();

    await page.getByPlaceholder('Message').fill('Browser smoke reply');
    await page.getByRole('button', { name: 'Send message' }).click();

    await expect(page.getByText('Browser smoke reply').first()).toBeVisible();
  });

  test('creates a direct room from directory users', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();
    await page.getByRole('button', { name: 'New DM' }).click();
    await page.getByPlaceholder('Search people').fill('Kim');
    await page.getByText('kim.chulsoo@parkjw.org').first().click();
    await page.getByRole('button', { name: 'Create room' }).click();

    await expect(page.getByText('Kim Chulsoo').first()).toBeVisible();
  });
});
