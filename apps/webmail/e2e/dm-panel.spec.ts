import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';
import { DEFAULT_DM_ROOMS } from './mocks';

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
    const incoming = await page.getByText('DM smoke hello').first().boundingBox();
    const outgoing = await page.getByText('Browser smoke reply').first().boundingBox();
    expect(outgoing && incoming ? outgoing.x > incoming.x : true).toBe(true);
  });

  test('uses the other participant as the direct room title', async ({ page }) => {
    await setupAuthedPage(page, {
      dmRooms: [{
        ...DEFAULT_DM_ROOMS[0],
        id: 'dm-direct-1',
        room_type: 'direct',
        name: '',
        owner_id: '',
        members: [
          { id: 'user-1', display_name: 'Park Jangwon', avatar_url: '' },
          { id: 'user-2', display_name: 'Kim Chulsoo', avatar_url: '' },
        ],
        unread_count: 0,
        member_count: 2,
      }],
      dmMessages: [],
    });

    await page.getByRole('button', { name: /^DM/ }).click();

    await expect(page.getByText('Kim Chulsoo').first()).toBeVisible();
    await expect(page.getByText('Park Jangwon').first()).not.toBeVisible();
  });

  test('prefers the configured group room name for participants', async ({ page }) => {
    await setupAuthedPage(page, {
      dmRooms: [{
        ...DEFAULT_DM_ROOMS[0],
        id: 'dm-group-1',
        room_type: 'group',
        name: '프로젝트 TF',
        owner_id: 'user-2',
        created_by: 'user-2',
        members: [
          { id: 'user-1', display_name: 'Park Jangwon', avatar_url: '' },
          { id: 'user-2', display_name: 'Kim Chulsoo', avatar_url: '' },
          { id: 'user-3', display_name: 'Lee Younghee', avatar_url: '' },
        ],
        unread_count: 0,
        member_count: 3,
      }],
      dmMessages: [],
    });

    await page.getByRole('button', { name: /^DM/ }).click();

    await expect(page.getByText('프로젝트 TF').first()).toBeVisible();
    await expect(page.getByText(/Kim Chulsoo/).first()).not.toBeVisible();
  });

  test('shows group members with profile photos in the details panel', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();
    await page.getByRole('button', { name: /Conversation details|대화 상세|会話の詳細|会话详情/ }).click();

    await expect(page.getByText(/Members|멤버|メンバー|成员/).first()).toBeVisible();
    await expect(page.getByText('Kim Chulsoo').first()).toBeVisible();
    await expect(page.getByText('user-2').first()).toBeVisible();
    await expect(page.getByAltText('Kim Chulsoo').first()).toBeVisible();
  });

  test('opens a flexible emoji reaction picker', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();
    await page.getByRole('button', { name: /Add reaction|공감 추가|リアクションを追加|添加回应/ }).first().click();
    await page.getByRole('button', { name: '🎉' }).click();

    await expect(page.getByText('🎉 1').first()).toBeVisible();
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
