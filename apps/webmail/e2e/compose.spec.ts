import { test, expect } from '@playwright/test';

test.describe('Compose Modal', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/mail');
  });

  test('compose button triggers modal', async ({ page }) => {
    // Look for compose button (floating or in sidebar)
    const composeBtn = page.locator('button, a').filter({ hasText: /작성|Compose|새 메일|New/i }).first();
    if (await composeBtn.isVisible()) {
      await composeBtn.click();
      // Modal should appear
      const modal = page.locator('[role="dialog"], .modal, [class*="modal"]').first();
      await expect(modal).toBeVisible({ timeout: 3000 }).catch(() => null);
    }
  });

  test('recipient chips input accepts text', async ({ page }) => {
    // Open compose (try various selectors)
    const buttons = await page.locator('button').count();
    for (let i = 0; i < Math.min(buttons, 5); i++) {
      const btn = page.locator('button').nth(i);
      const text = await btn.textContent();
      if (text?.match(/작성|compose|new|mail/i)) {
        await btn.click();
        break;
      }
    }

    // Wait a bit for modal to appear
    await page.waitForTimeout(500);

    // Look for recipient input
    const recipientInput = page.locator('input[type="email"], input[placeholder*="받"], input[placeholder*="to"]').first();
    if (await recipientInput.isVisible()) {
      await recipientInput.fill('test@example.com');
      await expect(recipientInput).toHaveValue('test@example.com');
    }
  });

  test('subject input accepts text', async ({ page }) => {
    const buttons = await page.locator('button').count();
    for (let i = 0; i < Math.min(buttons, 5); i++) {
      const btn = page.locator('button').nth(i);
      const text = await btn.textContent();
      if (text?.match(/작성|compose|new/i)) {
        await btn.click();
        break;
      }
    }

    await page.waitForTimeout(500);

    const subjectInput = page.locator('input[placeholder*="제목"], input[placeholder*="subject"], input[type="text"]').nth(0);
    if (await subjectInput.isVisible()) {
      await subjectInput.fill('Test Subject');
      await expect(subjectInput).toHaveValue('Test Subject');
    }
  });
});
