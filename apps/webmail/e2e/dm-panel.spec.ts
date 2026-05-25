import { test, expect } from '@playwright/test';
import { setupAuthedPage } from './helpers';
import { DEFAULT_DM_ROOMS } from './mocks';

test.describe('DM panel', () => {
  test('opens from the lower rail as a modal and sends a message', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();

    await expect(page.getByRole('heading', { name: 'DM' })).toBeVisible();
    await expect(page.getByText('Launch room').first()).toBeVisible();
    await page.getByRole('button', { name: /Launch room/ }).click();
    await expect(page.locator('main').getByText('DM smoke hello').first()).toBeVisible();

    await page.getByPlaceholder(/Message|메시지|メッセージ|消息/).fill('Browser smoke reply');
    await page.getByRole('button', { name: /Send message|메시지 보내기|メッセージを送信|发送消息/ }).click();

    await expect(page.locator('main').getByText('Browser smoke reply').first()).toBeVisible();
    const incoming = await page.locator('main').getByText('DM smoke hello').first().boundingBox();
    const outgoing = await page.locator('main').getByText('Browser smoke reply').first().boundingBox();
    expect(outgoing && incoming ? outgoing.x > incoming.x : true).toBe(true);
  });

  test('resizes from every modal edge without collapsing the DM layout', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();
    const dialog = page.getByRole('dialog', { name: 'DM' });
    await expect(dialog).toBeVisible();

    const before = await dialog.boundingBox();
    expect(before).not.toBeNull();
    if (!before) return;

    await page.mouse.move(before.x + 2, before.y + before.height / 2);
    await page.mouse.down();
    await page.mouse.move(before.x - 80, before.y + before.height / 2);
    await page.mouse.up();

    const afterWestResize = await dialog.boundingBox();
    expect(afterWestResize).not.toBeNull();
    if (!afterWestResize) return;
    expect(afterWestResize.width).toBeGreaterThan(before.width + 50);

    await page.mouse.move(afterWestResize.x + afterWestResize.width - 2, afterWestResize.y + afterWestResize.height / 2);
    await page.mouse.down();
    await page.mouse.move(afterWestResize.x + 260, afterWestResize.y + afterWestResize.height / 2);
    await page.mouse.up();

    await page.mouse.move(afterWestResize.x + afterWestResize.width / 2, afterWestResize.y + 2);
    await page.mouse.down();
    await page.mouse.move(afterWestResize.x + afterWestResize.width / 2, afterWestResize.y + afterWestResize.height + 200);
    await page.mouse.up();

    const compact = await dialog.boundingBox();
    expect(compact).not.toBeNull();
    if (!compact) return;
    expect(compact.width).toBeGreaterThanOrEqual(320);
    expect(compact.height).toBeGreaterThanOrEqual(360);
    await expect(page.getByRole('heading', { name: 'DM' })).toBeVisible();
    await expect(page.getByRole('button', { name: /New DM|새 DM|新規DM|新建私信/ })).toBeVisible();
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
          { id: 'user-1', display_name: 'Park Jangwon', email: 'pjw@parkjw.org', avatar_url: '' },
          { id: 'user-2', display_name: 'Kim Chulsoo', email: 'kim.chulsoo@parkjw.org', avatar_url: '' },
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
          { id: 'user-1', display_name: 'Park Jangwon', email: 'pjw@parkjw.org', avatar_url: '' },
          { id: 'user-2', display_name: 'Kim Chulsoo', email: 'kim.chulsoo@parkjw.org', avatar_url: '' },
          { id: 'user-3', display_name: 'Lee Younghee', email: 'lee.younghee@parkjw.org', avatar_url: '' },
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
    await page.getByRole('button', { name: /Launch room/ }).click();
    await page.getByRole('button', { name: /Conversation details|대화 상세|会話の詳細|会话详情/ }).click();

    await expect(page.getByText(/Members|멤버|メンバー|成员/).first()).toBeVisible();
    await expect(page.locator('main').getByText('Kim Chulsoo').first()).toBeVisible();
    await expect(page.getByText('kim.chulsoo@parkjw.org').first()).toBeVisible();
    await expect(page.getByAltText('Kim Chulsoo').first()).toBeVisible();
  });

  test('opens compose from a member email and focuses the subject', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();
    await page.getByRole('button', { name: /Launch room/ }).click();
    await page.getByRole('button', { name: /Conversation details|대화 상세|会話の詳細|会话详情/ }).click();
    await page.getByText('kim.chulsoo@parkjw.org').first().click();

    const compose = page.getByRole('dialog').filter({ has: page.locator('#compose-subject') }).first();
    await expect(compose).toBeVisible();
    await expect(compose.getByText('kim.chulsoo@parkjw.org').first()).toBeVisible();
    await expect(page.locator('#compose-subject')).toBeFocused();
  });

  test('opens a flexible emoji reaction picker', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();
    await page.getByRole('button', { name: /Launch room/ }).click();
    await page.getByRole('button', { name: /Add reaction|공감 추가|リアクションを追加|添加回应/ }).first().click();
    await page.getByRole('button', { name: '🎉' }).click();

    await expect(page.getByText('🎉 1').first()).toBeVisible();
  });

  test('closes the emoji picker on outside click and after reaction errors', async ({ page }) => {
    await setupAuthedPage(page);
    await page.route(/\/api\/mail\/dm\/messages\/[^/]+\/reactions(?:\?|$)/, (route) =>
      route.fulfill({ status: 503, contentType: 'application/json', body: JSON.stringify({ error: { message: 'reaction failed' } }) })
    );

    await page.getByRole('button', { name: /^DM/ }).click();
    await page.getByRole('button', { name: /Launch room/ }).click();
    await page.getByRole('button', { name: /Add reaction|공감 추가|リアクションを追加|添加回应/ }).first().click();
    await expect(page.getByRole('button', { name: '🎉' })).toBeVisible();
    await page.mouse.click(520, 180);
    await expect(page.getByRole('button', { name: '🎉' })).not.toBeVisible();

    await page.getByRole('button', { name: /Add reaction|공감 추가|リアクションを追加|添加回应/ }).first().click();
    await page.getByRole('button', { name: '🎉' }).click();
    await expect(page.getByText('reaction failed')).toBeVisible();
    await expect(page.getByRole('button', { name: '🎉' })).not.toBeVisible();
  });

  test('keeps unsent text as a per-room draft while switching rooms', async ({ page }) => {
    await setupAuthedPage(page, {
      dmRooms: [
        { ...DEFAULT_DM_ROOMS[0], id: 'room-a', name: 'A room' },
        { ...DEFAULT_DM_ROOMS[0], id: 'room-b', name: 'B room' },
      ],
      dmMessages: [],
    });

    await page.getByRole('button', { name: /^DM/ }).click();
    await page.getByRole('button', { name: /A room/ }).click();
    const input = page.getByPlaceholder(/Message|메시지|メッセージ|消息/);
    await input.fill('draft for A');
    await page.getByRole('button', { name: /Back to conversations|대화 목록으로 돌아가기|会話一覧に戻る|返回会话列表/ }).click();
    await page.getByRole('button', { name: /B room/ }).click();
    await expect(input).toHaveValue('');
    await input.fill('draft for B');
    await page.getByRole('button', { name: /Back to conversations|대화 목록으로 돌아가기|会話一覧に戻る|返回会话列表/ }).click();
    await page.getByRole('button', { name: /A room/ }).click();
    await expect(input).toHaveValue('draft for A');
    await page.getByRole('button', { name: /Back to conversations|대화 목록으로 돌아가기|会話一覧に戻る|返回会话列表/ }).click();
    await page.getByRole('button', { name: /B room/ }).click();
    await expect(input).toHaveValue('draft for B');
  });

  test('uploads pasted clipboard images from the DM composer', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();
    await page.getByRole('button', { name: /Launch room/ }).click();
    const input = page.getByPlaceholder(/Message|메시지|メッセージ|消息/);
    const imageResponse = page.waitForResponse((response) =>
      response.url().includes('/api/mail/dm/messages/')
      && response.url().includes('/attachment?token=')
      && response.status() === 200
    );
    await input.evaluate((node) => {
      const file = new File([new Uint8Array([137, 80, 78, 71])], 'clip.png', { type: 'image/png' });
      const data = new DataTransfer();
      data.items.add(file);
      node.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }));
    });

    await expect(page.getByRole('dialog', { name: /Attach this image|해당 이미지를 첨부하시겠습니까|この画像を添付しますか|要附加这张图片吗/ })).toBeVisible();
    await page.getByRole('button', { name: /Attach image|이미지 첨부|画像を添付|附加图片/ }).click();
    await expect(page.getByText('clip.png').first()).toBeVisible();
    await imageResponse;
    const image = page.locator('main').getByRole('img', { name: 'clip.png' }).first();
    await expect(image).toBeVisible();
    await expect(page.getByRole('button', { name: /Download|다운로드|ダウンロード|下载/ }).first()).toBeVisible();
    await expect(page.getByRole('button', { name: /Copy image|이미지 복사|画像をコピー|复制图片/ }).first()).toBeVisible();
    await image.click({ button: 'right' });
    await expect(page.getByRole('menuitem', { name: /Copy image|이미지 복사|画像をコピー|复制图片/ })).toBeVisible();
    await page.mouse.click(520, 180);
    await image.click();
    const preview = page.getByRole('dialog', { name: 'clip.png' });
    await expect(preview).toBeVisible();
    await expect(preview.getByRole('img', { name: 'clip.png' })).toBeVisible();
  });

  test('hides group-only controls in direct rooms', async ({ page }) => {
    await setupAuthedPage(page, {
      dmRooms: [{
        ...DEFAULT_DM_ROOMS[0],
        id: 'dm-direct-1',
        room_type: 'direct',
        name: '',
        owner_id: '',
        members: [
          { id: 'user-1', display_name: 'Park Jangwon', email: 'pjw@parkjw.org', avatar_url: '' },
          { id: 'user-2', display_name: 'Kim Chulsoo', email: 'kim.chulsoo@parkjw.org', avatar_url: '' },
        ],
        unread_count: 0,
        member_count: 2,
      }],
      dmMessages: [],
    });

    await page.getByRole('button', { name: /^DM/ }).click();
    await page.getByRole('button', { name: /Kim Chulsoo/ }).click();
    await page.getByRole('button', { name: /Conversation details|대화 상세|会話の詳細|会话详情/ }).click();

    await expect(page.getByRole('button', { name: /Create invite|초대 링크 만들기|招待リンクを作成|创建邀请链接/ })).not.toBeVisible();
    await expect(page.getByRole('button', { name: /Add members|멤버 추가|メンバーを追加|添加成员/ })).not.toBeVisible();
    await expect(page.getByPlaceholder(/Owner user ID|소유자 사용자 ID|オーナーユーザーID|所有者用户 ID/)).not.toBeVisible();
    await expect(page.getByRole('button', { name: /Remove member|멤버 제거|メンバーを削除|移除成员/ })).toHaveCount(1);
  });

  test('creates a direct room from directory users', async ({ page }) => {
    await setupAuthedPage(page);

    await page.getByRole('button', { name: /^DM/ }).click();
    await page.getByRole('button', { name: /New DM|새 DM|新規DM|新建私信/ }).click();
    await page.getByPlaceholder(/Search people|사람 검색|ユーザーを検索|搜索联系人/).fill('Kim');
    await expect(page.getByText('kim.chulsoo@parkjw.org').first()).toBeVisible();
    await page.keyboard.press('ArrowDown');
    await page.keyboard.press('ArrowUp');
    await page.keyboard.press('Enter');
    await page.getByRole('button', { name: /Create room|대화방 만들기|ルームを作成|创建会话/ }).click();

    await expect(page.locator('main').getByText('Kim Chulsoo').first()).toBeVisible();
  });
});
