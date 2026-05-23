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
  await expect.poll(
    () => page.evaluate(() => Boolean((window as unknown as { __webmailNotifications?: unknown }).__webmailNotifications)),
    { timeout: 5_000 },
  ).toBe(true);

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

async function serviceWorkerOpenedUrls(page: Page, notificationData: Record<string, unknown>) {
  return await page.evaluate(async (data) => {
    const source = await fetch('/sw.js').then((response) => response.text());
    const listeners: Record<string, (event: unknown) => void> = {};
    const openedUrls: unknown[] = [];
    const fakeSelf = {
      addEventListener(type: string, handler: (event: unknown) => void) {
        listeners[type] = handler;
      },
      registration: {
        showNotification: () => Promise.resolve(),
      },
    };
    const fakeClients = {
      matchAll: async () => [],
      openWindow: async (url: unknown) => {
        openedUrls.push(url);
        return null;
      },
    };
    new Function('self', 'clients', source)(fakeSelf, fakeClients);

    let waited: Promise<unknown> | undefined;
    const event = {
      notification: {
        close: () => undefined,
        data,
      },
      waitUntil: (promise: Promise<unknown>) => {
        waited = Promise.resolve(promise);
      },
    };

    listeners.notificationclick(event);
    await waited;
    return openedUrls;
  }, notificationData);
}

async function serviceWorkerExistingClientClickResult(page: Page, notificationData: Record<string, unknown>) {
  return await page.evaluate(async (data) => {
    const source = await fetch('/sw.js').then((response) => response.text());
    const listeners: Record<string, (event: unknown) => void> = {};
    const result = { focused: 0, navigatedTo: null as unknown, opened: [] as unknown[] };
    const fakeSelf = {
      addEventListener(type: string, handler: (event: unknown) => void) {
        listeners[type] = handler;
      },
      registration: {
        showNotification: () => Promise.resolve(),
      },
    };
    const fakeWindowClient = {
      url: 'https://app.example/mail',
      focus: async () => {
        result.focused++;
        return fakeWindowClient;
      },
      navigate: async (url: unknown) => {
        result.navigatedTo = url;
        return fakeWindowClient;
      },
    };
    const fakeClients = {
      matchAll: async () => [fakeWindowClient],
      openWindow: async (url: unknown) => {
        result.opened.push(url);
        return null;
      },
    };
    new Function('self', 'clients', source)(fakeSelf, fakeClients);

    let waited: Promise<unknown> | undefined;
    const event = {
      notification: {
        close: () => undefined,
        data,
      },
      waitUntil: (promise: Promise<unknown>) => {
        waited = Promise.resolve(promise);
      },
    };

    listeners.notificationclick(event);
    await waited;
    return result;
  }, notificationData);
}

async function serviceWorkerShownNotification(page: Page, pushData: unknown) {
  return await page.evaluate(async (data) => {
    const source = await fetch('/sw.js').then((response) => response.text());
    const listeners: Record<string, (event: unknown) => void> = {};
    let shown: { title: unknown; options: Record<string, unknown> } | undefined;
    const fakeSelf = {
      addEventListener(type: string, handler: (event: unknown) => void) {
        listeners[type] = handler;
      },
      registration: {
        showNotification: async (title: unknown, options: Record<string, unknown>) => {
          shown = { title, options };
        },
      },
    };
    const fakeClients = {
      matchAll: async () => [],
      openWindow: async () => null,
    };
    new Function('self', 'clients', source)(fakeSelf, fakeClients);

    let waited: Promise<unknown> | undefined;
    const event = {
      data: {
        json: () => data,
      },
      waitUntil: (promise: Promise<unknown>) => {
        waited = Promise.resolve(promise);
      },
    };

    listeners.push(event);
    await waited;
    return shown;
  }, pushData);
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

  test('focuses search field when opened for keyboard filtering', async ({ page }) => {
    const { dialog } = await openCenter(page);
    const search = dialog.getByPlaceholder(/Search notifications|알림 검색|通知を検索|搜索通知/i);
    await expect(search).toBeFocused();
  });

  test('returns focus to the bell when closed with Escape', async ({ page }) => {
    const { bell, dialog } = await openCenter(page);
    const search = dialog.getByPlaceholder(/Search notifications|알림 검색|通知を検索|搜索通知/i);
    await expect(search).toBeFocused();

    await page.keyboard.press('Escape');
    await expect(dialog).not.toBeVisible();
    await expect(bell).toBeFocused();
  });

  test('uses localized accessible name for the panel close button', async ({ page }) => {
    const { dialog } = await openCenter(page);
    await expect(dialog.getByRole('button', { name: /^알림 닫기$/ })).toBeVisible();
  });

  test('pushed notification appears in list and increments badge', async ({ page }) => {
    await pushNotification(page, { title: 'Hello world', body: 'first body' });

    // Badge should show "1"
    await expect.poll(() => unreadBadgeText(page), { timeout: 5_000 }).toBe('1');

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Hello world');
    await expect(dialog).toContainText('first body');
  });

  test('announces unread count on the notification bell', async ({ page }) => {
    await pushNotification(page, { title: 'Accessible unread count' });

    await expect(
      page.getByRole('button', { name: /알림 열기, 읽지 않음 1개|Open notifications, 1 unread/i }).first(),
    ).toBeVisible();
  });

  test('sanitizes cross-tab storage notification hydration', async ({ page }) => {
    await page.evaluate(() => {
      const payload = Array.from({ length: 501 }, (_, i) => ({
        id: `storage-${i}`,
        category: 'system',
        severity: 'info',
        title: `Storage item ${i}`,
        timestamp: Date.now() - i,
        read: false,
      }));
      payload.unshift({ id: 'broken', category: 'system', severity: 'info', title: '', timestamp: Number.NaN, read: false });
      const raw = JSON.stringify(payload);
      localStorage.setItem('webmail_notifications', raw);
      window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_notifications', newValue: raw }));
    });

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __webmailNotifications?: { notifications: unknown[] } }).__webmailNotifications?.notifications.length ?? -1),
      { timeout: 5_000 },
    ).toBe(500);
  });

  test('clears stale state when cross-tab notification storage is invalid JSON', async ({ page }) => {
    await pushNotification(page, { title: 'Stale before corrupt sync' });
    await expect.poll(() => unreadBadgeText(page), { timeout: 5_000 }).toBe('1');

    await page.evaluate(() => {
      localStorage.setItem('webmail_notifications', '{not-json');
      window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_notifications', newValue: '{not-json' }));
    });

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __webmailNotifications?: { notifications: unknown[] } }).__webmailNotifications?.notifications.length ?? -1),
      { timeout: 5_000 },
    ).toBe(0);
  });

  test('rejects non-finite timestamps during storage hydration', async ({ page }) => {
    await page.evaluate(() => {
      const raw = `[
        {"id":"bad-infinite","category":"system","severity":"info","title":"Bad infinite timestamp","timestamp":1e999,"read":false},
        {"id":"good-finite","category":"system","severity":"info","title":"Good finite timestamp","timestamp":${Date.now()},"read":false}
      ]`;
      localStorage.setItem('webmail_notifications', raw);
      window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_notifications', newValue: raw }));
    });

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __webmailNotifications?: { notifications: unknown[] } }).__webmailNotifications?.notifications.length ?? -1),
      { timeout: 5_000 },
    ).toBe(1);

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Good finite timestamp');
    await expect(dialog).not.toContainText('Bad infinite timestamp');
  });

  test('rejects blank identifiers and titles during storage hydration', async ({ page }) => {
    await page.evaluate(() => {
      const now = Date.now();
      const raw = JSON.stringify([
        { id: '', category: 'system', severity: 'info', title: 'Blank id', timestamp: now, read: false },
        { id: 'blank-title', category: 'system', severity: 'info', title: '   ', timestamp: now, read: false },
        { id: 'valid-title', category: 'system', severity: 'info', title: 'Valid stored alert', timestamp: now, read: false },
      ]);
      localStorage.setItem('webmail_notifications', raw);
      window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_notifications', newValue: raw }));
    });

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __webmailNotifications?: { notifications: unknown[] } }).__webmailNotifications?.notifications.length ?? -1),
      { timeout: 5_000 },
    ).toBe(1);

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Valid stored alert');
    await expect(dialog).not.toContainText('Blank id');
  });

  test('deduplicates repeated identifiers during storage hydration', async ({ page }) => {
    await page.evaluate(() => {
      const now = Date.now();
      const raw = JSON.stringify([
        { id: 'stored-repeat', category: 'system', severity: 'info', title: 'Newest stored copy', timestamp: now, read: false },
        { id: 'stored-repeat', category: 'system', severity: 'info', title: 'Stale stored copy', timestamp: now - 1, read: false },
      ]);
      localStorage.setItem('webmail_notifications', raw);
      window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_notifications', newValue: raw }));
    });

    await expect.poll(
      () => page.evaluate(() => {
        const notifications = (window as unknown as {
          __webmailNotifications?: { notifications: Array<{ id: string; title: string }> };
        }).__webmailNotifications?.notifications ?? [];
        return notifications
          .filter((n) => n.id === 'stored-repeat')
          .map((n) => n.title);
      }),
      { timeout: 5_000 },
    ).toEqual(['Newest stored copy']);

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Newest stored copy');
    await expect(dialog).not.toContainText('Stale stored copy');
  });

  test('does not suppress a deduped push immediately after storage removes that id', async ({ page }) => {
    await pushNotification(page, { id: 'storage-race', title: 'Before storage removal', category: 'system', dedupe: true });
    await expect.poll(() => unreadBadgeText(page), { timeout: 5_000 }).toBe('1');

    await page.evaluate(() => {
      const raw = JSON.stringify([]);
      localStorage.setItem('webmail_notifications', raw);
      window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_notifications', newValue: raw }));
      const w = window as unknown as {
        __webmailNotifications?: { push: (input: Record<string, unknown>) => unknown };
      };
      w.__webmailNotifications?.push({
        id: 'storage-race',
        category: 'system',
        severity: 'info',
        title: 'After storage removal',
        dedupe: true,
      });
    });

    await expect.poll(
      () => page.evaluate(() => {
        const notifications = (window as unknown as {
          __webmailNotifications?: { notifications: Array<{ id: string; title: string }> };
        }).__webmailNotifications?.notifications ?? [];
        return notifications.map((n) => [n.id, n.title]);
      }),
      { timeout: 5_000 },
    ).toEqual([['storage-race', 'After storage removal']]);
  });

  test('rejects non-boolean read flags during storage hydration', async ({ page }) => {
    await page.evaluate(() => {
      const now = Date.now();
      const raw = JSON.stringify([
        { id: 'bad-read', category: 'system', severity: 'info', title: 'Bad read flag', timestamp: now, read: 'false' },
        { id: 'good-read', category: 'system', severity: 'info', title: 'Good unread flag', timestamp: now, read: false },
      ]);
      localStorage.setItem('webmail_notifications', raw);
      window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_notifications', newValue: raw }));
    });

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __webmailNotifications?: { notifications: unknown[] } }).__webmailNotifications?.notifications.length ?? -1),
      { timeout: 5_000 },
    ).toBe(1);
    await expect.poll(() => unreadBadgeText(page), { timeout: 5_000 }).toBe('1');

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Good unread flag');
    await expect(dialog).not.toContainText('Bad read flag');
  });

  test('rejects unsupported categories and severities during storage hydration', async ({ page }) => {
    await page.evaluate(() => {
      const now = Date.now();
      const raw = JSON.stringify([
        { id: 'bad-category', category: 'billing', severity: 'info', title: 'Bad category', timestamp: now, read: false },
        { id: 'bad-severity', category: 'system', severity: 'critical', title: 'Bad severity', timestamp: now, read: false },
        { id: 'good-category-severity', category: 'system', severity: 'warning', title: 'Good category severity', timestamp: now, read: false },
      ]);
      localStorage.setItem('webmail_notifications', raw);
      window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_notifications', newValue: raw }));
    });

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __webmailNotifications?: { notifications: unknown[] } }).__webmailNotifications?.notifications.length ?? -1),
      { timeout: 5_000 },
    ).toBe(1);

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Good category severity');
    await expect(dialog).not.toContainText('Bad category');
    await expect(dialog).not.toContainText('Bad severity');
  });

  test('rejects non-string bodies during storage hydration', async ({ page }) => {
    await page.evaluate(() => {
      const now = Date.now();
      const raw = JSON.stringify([
        { id: 'bad-body', category: 'system', severity: 'info', title: 'Bad body', body: { text: 'object body' }, timestamp: now, read: false },
        { id: 'good-body', category: 'system', severity: 'info', title: 'Good body', body: 'plain body', timestamp: now, read: false },
      ]);
      localStorage.setItem('webmail_notifications', raw);
      window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_notifications', newValue: raw }));
    });

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __webmailNotifications?: { notifications: unknown[] } }).__webmailNotifications?.notifications.length ?? -1),
      { timeout: 5_000 },
    ).toBe(1);

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Good body');
    await expect(dialog).toContainText('plain body');
    await expect(dialog).not.toContainText('Bad body');
  });

  test('rejects unsafe action URLs during storage hydration', async ({ page }) => {
    await page.evaluate(() => {
      const now = Date.now();
      const raw = JSON.stringify([
        { id: 'bad-action-object', category: 'system', severity: 'info', title: 'Bad action object', actionUrl: { href: '/mail' }, timestamp: now, read: false },
        { id: 'bad-action-scheme', category: 'system', severity: 'info', title: 'Bad action scheme', actionUrl: 'javascript:alert(1)', timestamp: now, read: false },
        { id: 'bad-action-host', category: 'system', severity: 'info', title: 'Bad action host', actionUrl: '//evil.example/path', timestamp: now, read: false },
        { id: 'bad-action-backslash', category: 'system', severity: 'info', title: 'Bad action backslash', actionUrl: '/\\\\evil.example/path', timestamp: now, read: false },
        { id: 'bad-action-control', category: 'system', severity: 'info', title: 'Bad action control', actionUrl: '/mail\nSet-Cookie:x', timestamp: now, read: false },
        { id: 'good-action', category: 'system', severity: 'info', title: 'Good action URL', actionUrl: '/mail?from=notification', timestamp: now, read: false },
      ]);
      localStorage.setItem('webmail_notifications', raw);
      window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_notifications', newValue: raw }));
    });

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __webmailNotifications?: { notifications: unknown[] } }).__webmailNotifications?.notifications.length ?? -1),
      { timeout: 5_000 },
    ).toBe(1);

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Good action URL');
    await expect(dialog).not.toContainText('Bad action object');
    await expect(dialog).not.toContainText('Bad action scheme');
    await expect(dialog).not.toContainText('Bad action host');
    await expect(dialog).not.toContainText('Bad action backslash');
    await expect(dialog).not.toContainText('Bad action control');
  });

  test('drops unsafe action URLs when notifications are pushed', async ({ page }) => {
    await pushNotification(page, { id: 'runtime-bad-action', title: 'Runtime unsafe action', actionUrl: 'javascript:alert(1)' });
    await pushNotification(page, { id: 'runtime-bad-backslash', title: 'Runtime unsafe backslash action', actionUrl: '/\\evil.example/path' });
    await pushNotification(page, { id: 'runtime-bad-control', title: 'Runtime unsafe control action', actionUrl: '/mail\nSet-Cookie:x' });
    await pushNotification(page, { id: 'runtime-good-action', title: 'Runtime safe action', actionUrl: '/mail?from=runtime' });

    await expect.poll(
      () => page.evaluate(() => {
        const notifications = (window as unknown as { __webmailNotifications?: { notifications: Array<{ id: string; actionUrl?: string }> } })
          .__webmailNotifications?.notifications ?? [];
        return notifications
          .filter((n) => n.id.startsWith('runtime-'))
          .map((n) => [n.id, n.actionUrl ?? null]);
      }),
      { timeout: 5_000 },
    ).toEqual([
      ['runtime-good-action', '/mail?from=runtime'],
      ['runtime-bad-control', null],
      ['runtime-bad-backslash', null],
      ['runtime-bad-action', null],
    ]);
  });

  test('service worker notification clicks fall back for unsafe target URLs', async ({ page }) => {
    await expect(serviceWorkerOpenedUrls(page, { url: 'https://evil.example/phish' })).resolves.toEqual(['/mail']);
    await expect(serviceWorkerOpenedUrls(page, { url: '//evil.example/phish' })).resolves.toEqual(['/mail']);
    await expect(serviceWorkerOpenedUrls(page, { url: '/\\evil.example/phish' })).resolves.toEqual(['/mail']);
    await expect(serviceWorkerOpenedUrls(page, { url: '/mail\nSet-Cookie:x' })).resolves.toEqual(['/mail']);
    await expect(serviceWorkerOpenedUrls(page, { url: { href: '/mail' } })).resolves.toEqual(['/mail']);
    await expect(serviceWorkerOpenedUrls(page, { url: '/mail?from=webpush' })).resolves.toEqual(['/mail?from=webpush']);
  });

  test('service worker notification clicks navigate an existing mail window to the target URL', async ({ page }) => {
    await expect(serviceWorkerExistingClientClickResult(page, { url: '/mail/thread-123' })).resolves.toEqual({
      focused: 1,
      navigatedTo: '/mail/thread-123',
      opened: [],
    });
  });

  test('service worker push payload fields are normalized before showing notifications', async ({ page }) => {
    await expect(
      serviceWorkerShownNotification(page, {
        title: { text: 'object title' },
        body: { text: 'object body' },
        tag: { value: 'object-tag' },
      }),
    ).resolves.toMatchObject({
      title: '새 메일',
      options: {
        body: '',
        tag: 'gogomail-notification',
      },
    });

    await expect(
      serviceWorkerShownNotification(page, {
        title: '  ',
        body: 'Body text',
        tag: 'custom-tag',
      }),
    ).resolves.toMatchObject({
      title: '새 메일',
      options: {
        body: 'Body text',
        tag: 'custom-tag',
      },
    });
  });

  test('service worker push handles non-object JSON payloads before showing notifications', async ({ page }) => {
    await expect(serviceWorkerShownNotification(page, null)).resolves.toMatchObject({
      title: '새 메일',
      options: {
        body: '',
        data: {},
        tag: 'gogomail-notification',
      },
    });

    const shown = await serviceWorkerShownNotification(page, ['not', 'an', 'object']);
    expect(shown).toMatchObject({
      title: '새 메일',
      options: {
        body: '',
        tag: 'gogomail-notification',
      },
    });
    expect(Array.isArray(shown?.options.data)).toBe(false);
    expect(shown?.options.data).toEqual({});
  });

  test('normalizes malformed runtime notification fields before rendering', async ({ page }) => {
    await page.evaluate(() => {
      const w = window as unknown as {
        __webmailNotifications?: { push: (input: Record<string, unknown>) => unknown };
      };
      w.__webmailNotifications?.push({
        id: { nested: 'bad-id' },
        category: 'unsupported-category',
        severity: 'critical',
        title: '   ',
        body: { text: 'object body' },
      });
    });

    await expect.poll(
      () => page.evaluate(() => {
        const notification = (window as unknown as {
          __webmailNotifications?: {
            notifications: Array<{ id: unknown; category: unknown; severity: unknown; title: unknown; body?: unknown }>;
          };
        }).__webmailNotifications?.notifications[0];
        return notification
          ? {
              idType: typeof notification.id,
              category: notification.category,
              severity: notification.severity,
              title: notification.title,
              body: notification.body ?? null,
            }
          : null;
      }),
      { timeout: 5_000 },
    ).toEqual({
      idType: 'string',
      category: 'system',
      severity: 'info',
      title: 'Notification',
      body: null,
    });

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Notification');
    await expect(dialog).not.toContainText('object body');
  });

  test('deduplicates repeated event notifications by id', async ({ page }) => {
    await pushNotification(page, { id: 'mail-42', title: 'First copy', body: 'original body', category: 'mail_received', dedupe: true });
    await pushNotification(page, { id: 'mail-42', title: 'Second copy', body: 'duplicate body', category: 'mail_received', dedupe: true });

    await expect.poll(() => unreadBadgeText(page), { timeout: 5_000 }).toBe('1');
    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('First copy');
    await expect(dialog).not.toContainText('Second copy');
  });

  test('allows deduped notifications to return after they were evicted by the retention limit', async ({ page }) => {
    await page.evaluate(() => {
      const w = window as unknown as {
        __webmailNotifications?: { push: (input: Record<string, unknown>) => unknown };
      };
      if (!w.__webmailNotifications) throw new Error('notifications store not available');
      for (let i = 0; i < 501; i++) {
        w.__webmailNotifications.push({
          id: `retention-${i}`,
          category: 'system',
          severity: 'info',
          title: `Retention notice ${i}`,
          dedupe: true,
        });
      }
      w.__webmailNotifications.push({
        id: 'retention-0',
        category: 'system',
        severity: 'info',
        title: 'Retention notice returned',
        dedupe: true,
      });
    });

    await expect.poll(
      () => page.evaluate(() => {
        const notifications = (window as unknown as {
          __webmailNotifications?: { notifications: Array<{ id: string; title: string }> };
        }).__webmailNotifications?.notifications ?? [];
        return {
          count: notifications.length,
          first: notifications[0]?.title ?? null,
          hasReturned: notifications.some((n) => n.id === 'retention-0' && n.title === 'Retention notice returned'),
        };
      }),
      { timeout: 5_000 },
    ).toEqual({
      count: 500,
      first: 'Retention notice returned',
      hasReturned: true,
    });
  });

  test('keeps runtime notification identifiers unique even without explicit dedupe', async ({ page }) => {
    await pushNotification(page, { id: 'runtime-repeat', title: 'First runtime copy', category: 'system' });
    await pushNotification(page, { id: 'runtime-repeat', title: 'Updated runtime copy', category: 'system' });

    await expect.poll(
      () => page.evaluate(() => {
        const notifications = (window as unknown as {
          __webmailNotifications?: { notifications: Array<{ id: string; title: string }> };
        }).__webmailNotifications?.notifications ?? [];
        return notifications
          .filter((n) => n.id === 'runtime-repeat')
          .map((n) => n.title);
      }),
      { timeout: 5_000 },
    ).toEqual(['Updated runtime copy']);

    const { dialog } = await openCenter(page);
    await expect(dialog).toContainText('Updated runtime copy');
    await expect(dialog).not.toContainText('First runtime copy');
  });

  test('deduplicates browser notification mirroring by id', async ({ page }) => {
    await page.addInitScript(() => {
      try {
        Object.defineProperty(window.Notification, 'permission', { value: 'granted', configurable: true });
      } catch {
        // ignore
      }
      localStorage.setItem('webmail_browser_notifications_enabled', 'true');
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

    await pushNotification(page, { id: 'dedupe-browser-1', title: 'First urgent copy', category: 'system', severity: 'error', dedupe: true });
    await pushNotification(page, { id: 'dedupe-browser-1', title: 'Duplicate urgent copy', category: 'system', severity: 'error', dedupe: true });

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __notificationsCreated: number }).__notificationsCreated),
    ).toBe(1);
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

  test('clears stale search when the notification center is reopened', async ({ page }) => {
    await pushNotification(page, { title: 'Old deploy notice' });

    const { bell, dialog } = await openCenter(page);
    const search = dialog.getByPlaceholder(/Search notifications|알림 검색|通知を検索|搜索通知/i);
    await search.fill('deploy');
    await page.keyboard.press('Escape');
    await expect(dialog).not.toBeVisible();
    await expect(bell).toBeFocused();

    await pushNotification(page, { title: 'Fresh billing notice' });
    await bell.click();

    await expect(search).toBeFocused();
    await expect(search).toHaveValue('');
    await expect(dialog).toContainText('Fresh billing notice');
    await expect(dialog).toContainText('Old deploy notice');
  });

  test('clears stale category filter when the notification center is reopened', async ({ page }) => {
    await pushNotification(page, { title: 'Old mail notice', category: 'mail_received' });

    const { bell, dialog } = await openCenter(page);
    await dialog.getByRole('button', { name: /^(Mail|메일|メール|邮件) \(1\)$/i }).click();
    await expect(dialog).toContainText('Old mail notice');
    await page.keyboard.press('Escape');
    await expect(dialog).not.toBeVisible();
    await expect(bell).toBeFocused();

    await pushNotification(page, { title: 'Fresh system notice', category: 'system' });
    await bell.click();

    await expect(dialog.getByRole('button', { name: /^(all|전체|すべて|全部)$/i })).toHaveAttribute('aria-pressed', 'true');
    await expect(dialog).toContainText('Fresh system notice');
    await expect(dialog).toContainText('Old mail notice');
  });

  test('clears a selected category filter when its last notification is dismissed', async ({ page }) => {
    await pushNotification(page, { title: 'Persistent system alert', category: 'system' });
    await pushNotification(page, { title: 'Only inbox alert', category: 'mail_received' });

    const { dialog } = await openCenter(page);
    await dialog.getByRole('button', { name: /^(Mail|메일|メール|邮件) \(1\)$/i }).click();
    await expect(dialog).toContainText('Only inbox alert');
    await expect(dialog).not.toContainText('Persistent system alert');

    const mailRow = dialog.locator('[aria-label="Only inbox alert"]').first();
    await mailRow.getByRole('button', { name: /dismiss|닫기|閉じる|关闭/i }).click();

    await expect(dialog).toContainText('Persistent system alert');
    await expect(dialog).not.toContainText(/no notifications match|조건에 맞는 알림|一致する通知|没有符合/i);
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

  test('returns to all notifications after marking all unread-filtered items read', async ({ page }) => {
    await pushNotification(page, { title: 'Unread filtered A' });
    await pushNotification(page, { title: 'Unread filtered B' });

    const { dialog } = await openCenter(page);
    await dialog.getByRole('button', { name: /^(unread|읽지 않음|未読|未读)( \(2\))?$/i }).click();
    await dialog.getByRole('button', { name: /mark all read|모두 읽음|すべて既読|全部标记为已读/i }).click();

    await expect.poll(() => unreadBadgeText(page)).toBeNull();
    await expect(dialog).toContainText('Unread filtered A');
    await expect(dialog).toContainText('Unread filtered B');
    await expect(dialog).not.toContainText(/no notifications match|조건에 맞는 알림|一致する通知|没有符合/i);
  });

  test('mark-all-read only marks visible notifications when filters are active', async ({ page }) => {
    await pushNotification(page, { title: 'Deployment finished', body: 'System job succeeded', category: 'system' });
    await pushNotification(page, { title: 'Inbox delivery', body: 'Mail from Finance', category: 'mail_received' });
    await expect.poll(() => unreadBadgeText(page)).toBe('2');

    const { dialog } = await openCenter(page);
    const search = dialog.getByPlaceholder(/Search notifications|알림 검색|通知を検索|搜索通知/i);
    await search.fill('deploy');
    await expect(dialog).toContainText('Deployment finished');
    await expect(dialog).not.toContainText('Inbox delivery');

    await dialog.getByRole('button', { name: /mark all read|모두 읽음|すべて既読|全部标记为已读/i }).click();
    await expect.poll(() => unreadBadgeText(page)).toBe('1');

    await search.fill('');
    await expect(dialog).toContainText('Inbox delivery');
    await dialog.getByRole('button', { name: /^(unread|읽지 않음|未読|未读)( \(1\))?$/i }).click();
    await expect(dialog).toContainText('Inbox delivery');
    await expect(dialog).not.toContainText('Deployment finished');
  });

  test('returns to all notifications after opening the last unread-filtered item', async ({ page }) => {
    await pushNotification(page, { id: 'already-read-notification', title: 'Already read fallback' });
    await pushNotification(page, { id: 'last-unread-notification', title: 'Last unread item' });
    await page.evaluate(() => {
      (window as unknown as {
        __webmailNotifications?: { markAsRead: (id: string) => void };
      }).__webmailNotifications?.markAsRead('already-read-notification');
    });

    const { dialog } = await openCenter(page);
    await dialog.getByRole('button', { name: /^(unread|읽지 않음|未読|未读)( \(1\))?$/i }).click();
    await expect(dialog).toContainText('Last unread item');
    await expect(dialog).not.toContainText('Already read fallback');

    await dialog.locator('[aria-label="Last unread item"]').first().click();

    await expect.poll(() => unreadBadgeText(page)).toBeNull();
    await expect(dialog).toContainText('Already read fallback');
    await expect(dialog).toContainText('Last unread item');
    await expect(dialog).not.toContainText(/no notifications match|조건에 맞는 알림|一致する通知|没有符合/i);
  });

  test('unread filtering hides categories that only contain read notifications', async ({ page }) => {
    await pushNotification(page, { id: 'read-mail-category', title: 'Read mail category', category: 'mail_received' });
    await pushNotification(page, { id: 'unread-system-category', title: 'Unread system category', category: 'system' });
    await page.evaluate(() => {
      (window as unknown as {
        __webmailNotifications?: { markAsRead: (id: string) => void };
      }).__webmailNotifications?.markAsRead('read-mail-category');
    });

    const { dialog } = await openCenter(page);
    await dialog.getByRole('button', { name: /^(unread|읽지 않음|未読|未读)( \(1\))?$/i }).click();

    await expect(dialog).toContainText('Unread system category');
    await expect(dialog).not.toContainText('Read mail category');
    await expect(dialog.getByRole('button', { name: /^(Mail|메일|メール|邮件) \(1\)$/i })).toHaveCount(0);
  });

  test('search filtering hides categories with no matching notifications', async ({ page }) => {
    await pushNotification(page, { title: 'Deployment finished', body: 'System job succeeded', category: 'system' });
    await pushNotification(page, { title: 'Inbox delivery', body: 'Mail from Finance', category: 'mail_received' });

    const { dialog } = await openCenter(page);
    const search = dialog.getByPlaceholder(/Search notifications|알림 검색|通知を検索|搜索通知/i);
    await search.fill('deploy');

    await expect(dialog).toContainText('Deployment finished');
    await expect(dialog).not.toContainText('Inbox delivery');
    await expect(dialog.getByRole('button', { name: /^(Mail|메일|メール|邮件) \(1\)$/i })).toHaveCount(0);
  });

  test('clear-all only clears visible notifications when filters are active', async ({ page }) => {
    await pushNotification(page, { title: 'Deployment finished', body: 'System job succeeded', category: 'system' });
    await pushNotification(page, { title: 'Inbox delivery', body: 'Mail from Finance', category: 'mail_received' });

    const { dialog } = await openCenter(page);
    const search = dialog.getByPlaceholder(/Search notifications|알림 검색|通知を検索|搜索通知/i);
    await search.fill('deploy');
    await expect(dialog).toContainText('Deployment finished');
    await expect(dialog).not.toContainText('Inbox delivery');

    await dialog.getByRole('button', { name: /clear all|모두 지우기|すべて消去|全部清除/i }).click();
    await expect(dialog).not.toContainText('Deployment finished');

    await search.fill('');
    await expect(dialog).toContainText('Inbox delivery');
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

  test('names item dismiss buttons with the notification title', async ({ page }) => {
    await pushNotification(page, { title: 'Targeted alert' });

    const { dialog } = await openCenter(page);
    await expect(
      dialog.getByRole('button', { name: /알림 닫기: Targeted alert|Dismiss notification: Targeted alert/i }),
    ).toBeVisible();
  });

  test('does not nest item dismiss buttons inside notification action buttons', async ({ page }) => {
    await pushNotification(page, { title: 'Nested action guard' });

    const { dialog } = await openCenter(page);
    await expect
      .poll(
        () =>
          dialog.evaluate((el) =>
            Array.from(el.querySelectorAll('[role="button"], button')).some((candidate) =>
              candidate.querySelector('button'),
            ),
          ),
        { timeout: 5_000 },
      )
      .toBe(false);
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

  test('uses contextual accessible name for browser notification banner dismiss', async ({ page }) => {
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

    const { dialog } = await openCenter(page);
    await expect(
      dialog.getByRole('button', { name: /브라우저 알림 안내 닫기|Dismiss browser notification prompt/i }),
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

  test('plays in-app notification sound when enabled', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('webmail_notif_sound', '1');
      (window as unknown as { __notificationSoundStarts: number }).__notificationSoundStarts = 0;
      class FakeOscillator {
        type = 'sine';
        frequency = { setValueAtTime() { /* noop */ } };
        connect() { return this; }
        start() {
          (window as unknown as { __notificationSoundStarts: number }).__notificationSoundStarts++;
        }
        stop() { /* noop */ }
      }
      class FakeGain {
        gain = {
          setValueAtTime() { /* noop */ },
          exponentialRampToValueAtTime() { /* noop */ },
        };
        connect() { return this; }
      }
      class FakeAudioContext {
        currentTime = 0;
        destination = {};
        createOscillator() { return new FakeOscillator(); }
        createGain() { return new FakeGain(); }
        close() { return Promise.resolve(); }
      }
      Object.defineProperty(window, 'AudioContext', { value: FakeAudioContext, configurable: true });
    });
    await setupAuthedPage(page);
    await page.evaluate(() => {
      const w = window as unknown as {
        __webmailNotifications?: { push: (input: Record<string, unknown>) => unknown };
      };
      w.__webmailNotifications?.push({
        category: 'system',
        severity: 'info',
        title: 'Sound check',
      });
    });

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __notificationSoundStarts: number }).__notificationSoundStarts),
      { timeout: 5_000 },
    ).toBe(1);
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
  test('mutes and unmutes a thread from the reading pane actions', async ({ page }) => {
    const threadId = '22222222-3333-4444-5555-666666666666';
    let savedBody: Record<string, unknown> | null = null;
    await setupAuthedPage(page, {
      messages: [
        makeMessage('thread-action-message', {
          subject: 'Thread action target',
          thread_id: threadId,
        }),
      ],
      notificationPreferences: {
        global_dnd_enabled: false,
        global_dnd_schedule: { weekdays: [], time_ranges: [], timezone: 'Asia/Seoul' },
        folder_overrides: {},
        thread_overrides: {},
        updated_at: '2026-05-23T00:00:00Z',
      },
      onNotificationPreferencesPut: (body) => { savedBody = body; },
    });

    await page.getByText('Thread action target').first().click();
    await page.getByRole('button', { name: /더 보기|More actions|その他の操作|更多操作/i }).last().click();
    await page.getByRole('button', { name: /스레드 알림 끄기|Mute thread notifications/i }).click();

    await expect.poll(() => savedBody, { timeout: 5_000 }).toMatchObject({
      thread_overrides: {
        [threadId]: { enabled: false },
      },
    });

    await page.getByRole('button', { name: /더 보기|More actions|その他の操作|更多操作/i }).last().click();
    await page.getByRole('button', { name: /스레드 알림 켜기|Unmute thread notifications/i }).click();

    await expect.poll(() => savedBody, { timeout: 5_000 }).toMatchObject({
      thread_overrides: {},
    });
  });

  test('mirrors badge count mode to the native Badging API when available', async ({ page }) => {
    await page.addInitScript(() => {
      const calls: Array<{ method: string; count?: number }> = [];
      (window as unknown as { __badgeCalls: typeof calls }).__badgeCalls = calls;
      Object.defineProperty(navigator, 'setAppBadge', {
        configurable: true,
        value: (count?: number) => {
          calls.push({ method: 'setAppBadge', count });
          return Promise.resolve();
        },
      });
      Object.defineProperty(navigator, 'clearAppBadge', {
        configurable: true,
        value: () => {
          calls.push({ method: 'clearAppBadge' });
          return Promise.resolve();
        },
      });
    });
    await setupAuthedPage(page);

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __badgeCalls: Array<{ method: string; count?: number }> }).__badgeCalls),
      { timeout: 5_000 },
    ).toContainEqual({ method: 'setAppBadge', count: 2 });

    await page.evaluate(() => {
      localStorage.setItem('webmail_badge_count_mode', 'none');
      window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_badge_count_mode', newValue: 'none' }));
    });

    await expect.poll(
      () => page.evaluate(() => (window as unknown as { __badgeCalls: Array<{ method: string; count?: number }> }).__badgeCalls),
      { timeout: 5_000 },
    ).toContainEqual({ method: 'clearAppBadge' });
  });

  test('respects badge count mode none in document title', async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem('webmail_badge_count_mode', 'none');
    });
    await setupAuthedPage(page);

    await expect.poll(() => page.title(), { timeout: 5_000 }).toBe('GoGoMail');
  });

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

  test('skips notification-center entry for muted threads during message refresh', async ({ page }) => {
    const threadId = '11111111-2222-3333-4444-555555555555';
    const messages = [
      makeMessage('muted-thread-seed', { read: true, subject: 'Already seen muted thread', thread_id: threadId }),
    ];
    await setupAuthedPage(page, {
      messages,
      notificationPreferences: {
        global_dnd_enabled: false,
        global_dnd_schedule: { weekdays: [], time_ranges: [], timezone: 'Asia/Seoul' },
        folder_overrides: {},
        thread_overrides: {
          [threadId]: { enabled: false },
        },
        updated_at: '2026-05-23T00:00:00Z',
      },
    });
    await page.evaluate(() => localStorage.removeItem('webmail_notifications'));
    await expect.poll(() => page.evaluate(() => localStorage.getItem('webmail_notification_thread_overrides')), {
      timeout: 5_000,
    }).toContain(threadId);

    messages.unshift(makeMessage('muted-thread-new', {
      read: false,
      subject: 'Muted thread arrival',
      from_name: 'Muted Thread Sender',
      thread_id: threadId,
    }));
    await page.getByRole('button', { name: /새로고침|Refresh|更新|刷新/i }).click();

    await expect.poll(() => unreadBadgeText(page), { timeout: 5_000 }).toBeNull();
    const { dialog } = await openCenter(page);
    await expect(dialog).not.toContainText('Muted Thread Sender');
  });
});
