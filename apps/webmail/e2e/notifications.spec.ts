import { test, expect, type Page } from '@playwright/test';
import { setupAuthedPage } from './helpers';

/**
 * The notification store exposes `window.__webmailNotifications` for tests
 * and future server-driven events (see `src/lib/notifications/store.ts`).
 */

async function openCenter(page: Page) {
  const bell = page.getByRole('button', { name: /알림 열기|open notifications|通知を開く|打开通知/i }).first();
  await expect(bell).toBeVisible({ timeout: 10_000 });
  await bell.click();
  const dialog = page.getByRole('dialog', { name: /알림|notifications|通知/i }).first();
  await expect(dialog).toBeVisible({ timeout: 5_000 });
  return { bell, dialog };
}

async function pushNotification(
  page: Page,
  data: { title: string; body?: string; category?: string; severity?: string; actionUrl?: string },
) {
  await page.evaluate((d) => {
    const w = window as unknown as {
      __webmailNotifications?: {
        push: (input: Record<string, unknown>) => { id: string };
      };
    };
    if (!w.__webmailNotifications) throw new Error('notifications store not available');
    return w.__webmailNotifications.push({
      category: d.category ?? 'system',
      severity: d.severity ?? 'info',
      title: d.title,
      body: d.body,
      actionUrl: d.actionUrl,
    });
  }, data);
}

async function unreadBadgeText(page: Page): Promise<string | null> {
  const bell = page.getByRole('button', { name: /알림 열기|open notifications|通知を開く|打开通知/i }).first();
  return await bell.evaluate((el) => {
    const span = el.querySelector('span');
    return span ? (span.textContent ?? '') : null;
  });
}

test.describe('Notification center', () => {
  test.beforeEach(async ({ page }) => {
    await setupAuthedPage(page);
    // Clear any persisted notifications from previous runs
    await page.evaluate(() => localStorage.removeItem('webmail_notifications'));
    await page.reload();
    await page.waitForURL(/\/mail/);
  });

  test('bell opens the notification center with empty state', async ({ page }) => {
    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText(/no notifications|아직 알림|通知はまだ|暂无通知/i);
  });

  test('pushed notification appears in list and increments badge', async ({ page }) => {
    await pushNotification(page, { title: 'Hello world', body: 'first body' });

    // Badge should show "1"
    await expect.poll(() => unreadBadgeText(page), { timeout: 5_000 }).toBe('1');

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Hello world');
    await expect(dialog).toContainText('first body');
  });

  test('mark-all-read clears badge but keeps items', async ({ page }) => {
    await pushNotification(page, { title: 'A' });
    await pushNotification(page, { title: 'B' });
    await expect.poll(() => unreadBadgeText(page)).toBe('2');

    const { dialog } = await openCenter(page);
    await dialog.getByRole('button', { name: /mark all read|모두 읽음|すべて既読|全部标记为已读/i }).click();

    await expect.poll(() => unreadBadgeText(page)).toBeNull();
    await expect(dialog).toContainText('A');
    await expect(dialog).toContainText('B');
  });

  test('dismiss removes a single item', async ({ page }) => {
    await pushNotification(page, { title: 'KeepMe' });
    await pushNotification(page, { title: 'RemoveMe' });

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('RemoveMe');

    // The item button has aria-label of its title; dismiss is the inner X
    // with aria-label = t('dismiss').
    const dismissBtns = dialog.locator('[aria-label="Dismiss"], [aria-label="닫기"], [aria-label="閉じる"], [aria-label="关闭"]');
    // The dialog also has a top-level close button (aria-label="close"). Filter to those inside the item list.
    const itemDismiss = dismissBtns.filter({ hasNot: page.locator('svg[role=img]') });
    // Click the first item-level dismiss (note: items are newest-first, so RemoveMe is on top)
    await dialog.locator('button:has-text("RemoveMe")').first().locator('[role="button"]').first().click();

    await expect(dialog).not.toContainText('RemoveMe');
    await expect(dialog).toContainText('KeepMe');
  });

  test('clear-all empties the list', async ({ page }) => {
    await pushNotification(page, { title: 'one' });
    await pushNotification(page, { title: 'two' });

    const { dialog } = await openCenter(page);
    await dialog.getByRole('button', { name: /clear all|모두 지우기|すべて消去|全部清除/i }).click();

    await expect(dialog).toContainText(/no notifications|아직 알림|通知はまだ|暂无通知/i);
  });

  test('shows browser-notification banner when permission is default', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        Object.defineProperty(window.Notification, 'permission', { value: 'default', configurable: true });
      } catch {
        // ignore in browsers that disallow override
      }
      try {
        localStorage.removeItem('webmail_browser_banner_dismissed');
      } catch {
        // ignore
      }
    });
    await setupAuthedPage(page);
    const bell = page.getByRole('button', { name: /알림 열기|open notifications|通知を開く|打开通知/i }).first();
    await bell.click();
    await expect(
      page.getByText(/Enable browser notifications|브라우저 알림 활성화|ブラウザ通知を有効化|启用浏览器通知/),
    ).toBeVisible();
  });

  test('respects browser-notifications toggle when permission is granted', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        Object.defineProperty(window.Notification, 'permission', { value: 'granted', configurable: true });
      } catch {
        // ignore
      }
      localStorage.setItem('webmail_browser_notifications_enabled', 'false');
      // Spy on Notification constructor
      (window as unknown as { __notificationsCreated: number }).__notificationsCreated = 0;
      const Orig = window.Notification;
      window.Notification = new Proxy(Orig, {
        construct(target, args) {
          (window as unknown as { __notificationsCreated: number }).__notificationsCreated++;
          return new (target as unknown as new (...a: unknown[]) => Notification)(...args);
        },
      }) as typeof window.Notification;
    });
    await setupAuthedPage(page);
    await page.evaluate(() => {
      const w = window as unknown as {
        __webmailNotifications?: { push: (input: Record<string, unknown>) => unknown };
      };
      w.__webmailNotifications?.push({
        category: 'mail_received',
        severity: 'info',
        title: 'Test toggle off',
      });
    });
    await page.waitForTimeout(200);
    const count = await page.evaluate(
      () => (window as unknown as { __notificationsCreated: number }).__notificationsCreated,
    );
    expect(count).toBe(0); // toggle off => no browser notification
  });

  test('persists across reload', async ({ page }) => {
    await pushNotification(page, { title: 'Persisted item' });
    await expect.poll(() => unreadBadgeText(page)).toBe('1');

    await page.reload();
    await page.waitForURL(/\/mail/);

    await expect.poll(() => unreadBadgeText(page), { timeout: 5_000 }).toBe('1');
    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Persisted item');
  });
});
