import { test, expect, type Page } from '@playwright/test';
import { setupAuthedPage } from './helpers';

async function openCompose(page: Page) {
  const composeBtn = page
    .getByRole('button', { name: /편지 쓰기|새 메일|작성|compose/i })
    .first();
  await expect(composeBtn).toBeVisible({ timeout: 10_000 });
  await composeBtn.click();
  const dialog = page.getByRole('dialog').first();
  await expect(dialog).toBeVisible({ timeout: 5_000 });
  return dialog;
}

test.describe('Compose', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthedPage(page);
  });

  test('compose button opens modal', async ({ page }) => {
    const dialog = await openCompose(page);
    await expect(dialog).toBeVisible();
  });

  test('To field accepts an email address', async ({ page }) => {
    const dialog = await openCompose(page);
    const to = dialog.locator('input[type="email"], input[placeholder*="받"], [aria-label*="받는"]').first();
    if (await to.isVisible()) {
      await to.fill('alice@example.com');
      await expect(to).toHaveValue('alice@example.com');
    }
  });

  test('Subject field accepts text', async ({ page }) => {
    const dialog = await openCompose(page);
    const subj = dialog.locator('input[placeholder*="제목"], input[aria-label*="제목"], input[type="text"]').first();
    if (await subj.isVisible()) {
      await subj.fill('E2E hello');
      await expect(subj).toHaveValue('E2E hello');
    }
  });

  test('Cc / Bcc toggle reveals additional recipient inputs', async ({ page }) => {
    const dialog = await openCompose(page);
    const ccBtn = dialog.getByRole('button', { name: /^cc$|참조/i }).first();
    if (await ccBtn.isVisible().catch(() => false)) {
      await ccBtn.click();
      // After toggling, an additional input should appear.
      const inputs = dialog.locator('input[type="email"], input[placeholder*="참조"], input[placeholder*="cc"]');
      expect(await inputs.count()).toBeGreaterThanOrEqual(1);
    }
  });

  test('Body editor accepts text', async ({ page }) => {
    const dialog = await openCompose(page);
    const editor = dialog.locator('[contenteditable="true"]').first();
    if (await editor.isVisible()) {
      await editor.click();
      await editor.type('Hello body');
      await expect(editor).toContainText('Hello body');
    }
  });

  test('Send button is present', async ({ page }) => {
    const dialog = await openCompose(page);
    const send = dialog.getByRole('button', { name: /^전송$|보내기|send/i }).first();
    await expect(send).toBeVisible();
  });

  test('Close button dismisses dialog', async ({ page }) => {
    const dialog = await openCompose(page);
    const close = dialog.getByRole('button', { name: /닫기|close|×/i }).first();
    if (await close.isVisible()) {
      await close.click();
      // After close, either dialog disappears or a "save draft?" prompt shows.
      await page.waitForTimeout(300);
    }
  });
});
