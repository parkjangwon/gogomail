import { test, expect, type Page } from '@playwright/test';
import { setupAuthedPage } from './helpers';
import { makeMessage } from './mocks';

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
  data: { id?: string; title: string; body?: string; category?: string; severity?: string; actionUrl?: string; dedupe?: boolean },
) {
  await page.evaluate((d) => {
    const w = window as unknown as {
      __webmailNotifications?: {
        push: (input: Record<string, unknown>) => { id: string };
      };
    };
    if (!w.__webmailNotifications) throw new Error('notifications store not available');
    return w.__webmailNotifications.push({
      id: d.id,
      category: d.category ?? 'system',
      severity: d.severity ?? 'info',
      title: d.title,
      body: d.body,
      actionUrl: d.actionUrl,
      dedupe: d.dedupe,
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

  test('deduplicates repeated event notifications by id', async ({ page }) => {
    await pushNotification(page, { id: 'mail-42', title: 'First copy', body: 'original body', category: 'mail_received', dedupe: true });
    await pushNotification(page, { id: 'mail-42', title: 'Second copy', body: 'duplicate body', category: 'mail_received', dedupe: true });

    await expect.poll(() => unreadBadgeText(page), { timeout: 5_000 }).toBe('1');
    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('First copy');
    await expect(dialog).not.toContainText('Second copy');
  });

  test('search and category filters narrow a busy notification list', async ({ page }) => {
    await pushNotification(page, { title: 'Quarterly report uploaded', body: 'Drive file is ready', category: 'drive_share' });
    await pushNotification(page, { title: 'Deployment finished', body: 'System job succeeded', category: 'system' });
    await pushNotification(page, { title: 'Inbox delivery', body: 'Mail from Finance', category: 'mail_received' });

    const { dialog } = await openCenter(page);
    const search = dialog.getByPlaceholder(/Search notifications|알림 검색|通知を検索|搜索通知/i);
    await search.fill('deploy');
    await expect(dialog).toContainText('Deployment finished');
    await expect(dialog).not.toContainText('Quarterly report uploaded');

    await search.fill('');
    await dialog.getByRole('button', { name: /Mail|메일|メール|邮件/i }).click();
    await expect(dialog).toContainText('Inbox delivery');
    await expect(dialog).not.toContainText('Deployment finished');
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

    const removeRow = dialog.locator('[aria-label="RemoveMe"]').first();
    await removeRow.getByRole('button', { name: /dismiss|닫기|閉じる|关闭/i }).click();

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

  test('respects quiet hours for browser notification mirroring', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        Object.defineProperty(window.Notification, 'permission', { value: 'granted', configurable: true });
      } catch {
        // ignore
      }
      localStorage.setItem('webmail_browser_notifications_enabled', 'true');
      localStorage.setItem('webmail_dnd', '1');
      localStorage.setItem('webmail_dnd_start', '00:00');
      localStorage.setItem('webmail_dnd_end', '23:59');
      (window as unknown as { __notificationsCreated: number }).__notificationsCreated = 0;
      const Orig = window.Notification;
      window.Notification = new Proxy(Orig, {
        construct(target, args) {
          (window as unknown as { __notificationsCreated: number }).__notificationsCreated++;
          return new (target as unknown as new (...a: unknown[]) => Notification)(...args);
        },
      }) as typeof window.Notification;
    });
    await setupAuthedPage(page, {
      notificationPreferences: {
        global_dnd_enabled: true,
        global_dnd_schedule: {
          weekdays: [0, 1, 2, 3, 4, 5, 6],
          time_ranges: [{ start: '00:00', end: '23:59' }],
          timezone: 'Asia/Seoul',
        },
        folder_overrides: {},
        updated_at: '2026-05-23T00:00:00Z',
      },
    });
    await page.evaluate(() => {
      const w = window as unknown as {
        __webmailNotifications?: { push: (input: Record<string, unknown>) => unknown };
      };
      w.__webmailNotifications?.push({
        category: 'system',
        severity: 'error',
        title: 'Quiet hours should suppress this',
      });
    });
    await page.waitForTimeout(200);
    const count = await page.evaluate(
      () => (window as unknown as { __notificationsCreated: number }).__notificationsCreated,
    );
    expect(count).toBe(0);
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

test.describe('Mail arrival notifications', () => {
  test('respects browser-notification toggle during message refresh', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        Object.defineProperty(window.Notification, 'permission', { value: 'granted', configurable: true });
        Object.defineProperty(document, 'hidden', { get: () => true, configurable: true });
        Object.defineProperty(document, 'visibilityState', { get: () => 'hidden', configurable: true });
      } catch {
        // ignore
      }
      localStorage.setItem('webmail_browser_notifications_enabled', 'false');
      (window as unknown as { __notificationsCreated: number }).__notificationsCreated = 0;
      const Orig = window.Notification;
      window.Notification = new Proxy(Orig, {
        construct(target, args) {
          (window as unknown as { __notificationsCreated: number }).__notificationsCreated++;
          return new (target as unknown as new (...a: unknown[]) => Notification)(...args);
        },
      }) as typeof window.Notification;
    });

    const messages = [
      makeMessage('refresh-seed', { read: true, subject: 'Already seen' }),
    ];
    await setupAuthedPage(page, { messages });
    await page.evaluate(() => localStorage.removeItem('webmail_notifications'));

    messages.unshift(makeMessage('refresh-new', {
      read: false,
      subject: 'Fresh launch mail',
      from_name: 'Launch Desk',
    }));
    await page.getByRole('button', { name: /새로고침|Refresh|更新|刷新/i }).click();

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __notificationsCreated: number }).__notificationsCreated),
      { timeout: 5_000 },
    ).toBe(0);

    await expect.poll(() => unreadBadgeText(page), { timeout: 5_000 }).toBe('1');
  });

  test('skips notification-center entry for muted folders during message refresh', async ({ page }) => {
    const messages = [
      makeMessage('muted-seed', { read: true, subject: 'Already seen muted folder' }),
    ];
    await setupAuthedPage(page, {
      messages,
      notificationPreferences: {
        global_dnd_enabled: false,
        global_dnd_schedule: { weekdays: [], time_ranges: [], timezone: 'Asia/Seoul' },
        folder_overrides: {
          'folder-inbox': {
            enabled: false,
            dnd_inherit: true,
            dnd_schedule: { weekdays: [], time_ranges: [], timezone: '' },
          },
        },
        updated_at: '2026-05-23T00:00:00Z',
      },
    });
    await page.evaluate(() => localStorage.removeItem('webmail_notifications'));
    await expect.poll(() => page.evaluate(() => localStorage.getItem('webmail_notification_folder_overrides')), {
      timeout: 5_000,
    }).toContain('"folder-inbox"');

    messages.unshift(makeMessage('muted-new', {
      read: false,
      subject: 'Muted folder arrival',
      from_name: 'Muted Sender',
    }));
    await page.getByRole('button', { name: /새로고침|Refresh|更新|刷新/i }).click();

    await expect.poll(() => unreadBadgeText(page), { timeout: 5_000 }).toBeNull();
    const { dialog } = await openCenter(page);
    await expect(dialog).not.toContainText('Muted Sender');
  });
});
