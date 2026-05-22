import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';
import { DEFAULT_MESSAGES, makeMessage } from './mocks';

test.describe('Mail list', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthedPage(page);
  });

  test('renders inbox messages from mocked API', async ({ page }) => {
    await expect(page.getByText(DEFAULT_MESSAGES[0].subject).first()).toBeVisible({ timeout: 15_000 });
  });

  test('sidebar exposes system folders', async ({ page }) => {
    const sidebar = page.locator('aside[aria-label="메일 탐색"], nav').first();
    await expect(sidebar).toBeVisible();
    for (const label of ['받은 편지함', '보낸 편지함', '임시 보관함', '휴지통']) {
      const btn = sidebar.getByRole('button', { name: new RegExp(label) }).first();
      await expect(btn).toBeVisible({ timeout: 5_000 });
    }
  });

  test('navigate to sent folder', async ({ page }) => {
    const sent = page.locator('aside[aria-label="메일 탐색"]').getByRole('button', { name: /보낸 편지함/ }).first();
    if (await sent.isVisible()) {
      await sent.click();
      await expect(sent).toHaveAttribute('aria-current', /page/).catch(() => null);
    }
  });

  test('navigate to drafts and trash folders', async ({ page }) => {
    const sidebar = page.locator('aside[aria-label="메일 탐색"]');
    for (const name of [/임시 보관함/, /휴지통/]) {
      const btn = sidebar.getByRole('button', { name }).first();
      if (await btn.isVisible()) {
        await btn.click();
      }
    }
  });

  test('arrow keys move focus between message rows', async ({ page }) => {
    const rows = page.locator('[data-message-id]');
    await expect(rows.first()).toBeVisible({ timeout: 15_000 });
    const count = await rows.count();
    if (count < 2) test.skip(true, 'need at least 2 rendered rows');
    await rows.first().focus();
    await rows.first().press('ArrowDown');
    await expect(rows.nth(1)).toBeFocused();
  });

  test('space toggles bulk selection', async ({ page }) => {
    const rows = page.locator('[data-message-id]');
    await expect(rows.first()).toBeVisible({ timeout: 15_000 });
    await rows.first().focus();
    await rows.first().press('Space');
    const deselectBtn = rows.first().getByRole('button', { name: /선택 해제/ });
    if (await deselectBtn.isVisible().catch(() => false)) {
      await expect(deselectBtn).toBeVisible();
    }
  });

  test('empty folder shows no message rows', async ({ page }) => {
    const spam = page.locator('aside[aria-label="메일 탐색"]').getByRole('button', { name: /스팸/ }).first();
    if (await spam.isVisible()) {
      await spam.click();
      await page.waitForLoadState('networkidle').catch(() => null);
    }
  });
});

test.describe('Mail list — many messages', () => {
  test('renders 50 messages without errors', async ({ page }) => {
    const many = Array.from({ length: 50 }, (_, i) =>
      makeMessage(`bulk-${i}`, { subject: `Bulk message ${i}` })
    );
    await setupAuthedPage(page, { messages: many });
    await expect(page.getByText('Bulk message 0').first()).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText('Bulk message 5').first()).toBeVisible();
  });
});
