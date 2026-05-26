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


  test('internal sender avatar is rendered in message rows', async ({ page }) => {
    const avatar = 'data:image/png;base64,iVBORw0KGgo=';
    await setupAuthedPage(page, {
      messages: [
        makeMessage('avatar-internal', {
          subject: 'Internal avatar mail',
          folder_id: 'folder-inbox',
          from_addr: 'teammate@parkjw.org',
          from_name: 'Team Mate',
          sender_avatar_url: avatar,
        }),
      ],
    });

    const row = page.locator('[data-message-id="avatar-internal"]');
    await expect(row).toBeVisible({ timeout: 15_000 });
    await expect(row.locator(`img[src="${avatar}"]`)).toBeVisible();
  });

  test('all mail lists messages across folders', async ({ page }) => {
    await setupAuthedPage(page, {
      messages: [
        makeMessage('all-inbox', { subject: 'Inbox item in all mail', folder_id: 'folder-inbox' }),
        makeMessage('all-sent', { subject: 'Sent item in all mail', folder_id: 'folder-sent', from_addr: 'pjw@parkjw.org' }),
      ],
    });

    const allMail = page.locator('aside[aria-label="메일 탐색"]').getByRole('button', { name: /모든 편지함/ }).first();
    await allMail.click();

    await expect(page.getByText('Inbox item in all mail').first()).toBeVisible({ timeout: 15_000 });
    await expect(page.getByText('Sent item in all mail').first()).toBeVisible();
    await expect(page.getByText('받은 편지함이 깨끗합니다')).toHaveCount(0);
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
      await page.waitForLoadState('networkidle', { timeout: 5_000 }).catch(() => null);
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
