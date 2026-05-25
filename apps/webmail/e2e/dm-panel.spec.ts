import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';

test.describe('DM panel', () => {
  test('opens from the lower rail as a modal and sends a message', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();

    await expect(page.getByRole('heading', { name: 'DM' })).toBeVisible();
    await expect(page.getByText('Launch room').first()).toBeVisible();
    await expect(page.getByText('DM smoke hello').first()).toBeVisible();

    await page.getByPlaceholder(/Message|메시지|メッセージ|消息/).fill('Browser smoke reply');
    await page.getByRole('button', { name: /Send message|메시지 보내기|メッセージを送信|发送消息/ }).click();

    await expect(page.getByText('Browser smoke reply').first()).toBeVisible();
  });

  test('creates a direct room from directory users', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();
    await page.getByRole('button', { name: /New DM|새 DM|新規DM|新建私信/ }).click();
    await page.getByPlaceholder(/Search people|사람 검색|ユーザーを検索|搜索联系人/).fill('Kim');
    await page.getByText('kim.chulsoo@parkjw.org').first().click();
    await page.getByRole('button', { name: /Create room|대화방 만들기|ルームを作成|创建会话/ }).click();

    await expect(page.getByText('Kim Chulsoo').first()).toBeVisible();
  });
});
