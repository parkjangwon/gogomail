/**
 * Shared API mocks for the webmail E2E suite.
 *
 * The browser-side code talks to Next.js proxy routes under:
 *   - /api/mail/**  (forwarded to the backend /api/v1/** or /api/mail/**)
 *   - /api/auth/**  (login / mfa / logout / password-reset)
 *
 * These mocks intercept those routes via page.route() so the test suite
 * does NOT require a real backend.  Tests can call `installMocks(page)`
 * for a default canned set, or pass `overrides` to inject test-specific
 * data.  Unmocked routes return 404 with a descriptive error so missing
 * mocks are easy to spot during failures.
 */
import type { Page, Route } from '@playwright/test';

// --- Canned fixture data ---------------------------------------------------

export const SEED_USER_EMAIL = 'pjw@parkjw.org';
const PNG_1X1 = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/p9sAAAAASUVORK5CYII=', 'base64');
const AVATAR_DATA_URL = `data:image/png;base64,${PNG_1X1.toString('base64')}`;

export const DEFAULT_FOLDERS = [
  { id: 'folder-inbox', name: 'INBOX', full_path: 'INBOX', type: 'system', system_type: 'inbox', order_index: 0, total: 3, unread: 2, starred: 1 },
  { id: 'folder-sent', name: 'SENT', full_path: 'SENT', type: 'system', system_type: 'sent', order_index: 1, total: 1, unread: 0, starred: 0 },
  { id: 'folder-drafts', name: 'DRAFTS', full_path: 'DRAFTS', type: 'system', system_type: 'drafts', order_index: 2, total: 1, unread: 0, starred: 0 },
  { id: 'folder-spam', name: 'SPAM', full_path: 'SPAM', type: 'system', system_type: 'spam', order_index: 3, total: 0, unread: 0, starred: 0 },
  { id: 'folder-trash', name: 'TRASH', full_path: 'TRASH', type: 'system', system_type: 'trash', order_index: 4, total: 0, unread: 0, starred: 0 },
  { id: 'folder-archive', name: 'ARCHIVE', full_path: 'ARCHIVE', type: 'system', system_type: 'archive', order_index: 5, total: 0, unread: 0, starred: 0 },
];

export function makeMessage(id: string, overrides: Record<string, unknown> = {}) {
  return {
    id,
    folder_id: 'folder-inbox',
    subject: `Test subject ${id}`,
    preview: `Preview for ${id}`,
    from_addr: 'sender@example.com',
    from_name: 'Sender Example',
    received_at: '2026-01-15T10:00:00Z',
    size: 1024,
    has_attachment: false,
    read: false,
    starred: false,
    ...overrides,
  };
}

export const DEFAULT_MESSAGES = [
  makeMessage('msg-1', { subject: 'Welcome to gogomail', from_name: 'GoGoMail Team', from_addr: 'team@gogomail.dev' }),
  makeMessage('msg-2', { subject: 'Project update', read: true, starred: true }),
  makeMessage('msg-3', { subject: 'Attachment inside', has_attachment: true }),
];

export function makeMessageDetail(id: string, overrides: Record<string, unknown> = {}) {
  const base = DEFAULT_MESSAGES.find((m) => m.id === id) ?? makeMessage(id);
  return {
    message: {
      ...base,
      message_id: `<${id}@example.com>`,
      to_addrs: [{ email: SEED_USER_EMAIL, name: 'You' }],
      cc_addrs: [],
      bcc_addrs: [],
      flags: {},
      storage_path: `/storage/${id}.eml`,
      text_body: `Plain body for ${id}`,
      html_body: `<p>HTML body for <strong>${id}</strong></p>`,
      attachments: (base.has_attachment ? [{
        id: 'att-1',
        message_id: id,
        upload_id: 'upl-1',
        filename: 'attachment.pdf',
        content_type: 'application/pdf',
        size: 2048,
      }] : []),
      ...overrides,
    },
  };
}

export const DEFAULT_USER = {
  id: 'user-1',
  email: SEED_USER_EMAIL,
  name: 'PJW',
  display_name: 'Park Jangwon',
  avatar_url: null,
  locale: 'ko',
  timezone: 'Asia/Seoul',
};

export const DEFAULT_PREFERENCES = {
  preferences: {
    theme: 'system',
    locale: 'ko',
    accent_color: 'blue',
    density: 'comfortable',
    reading_pane: 'right',
    thread_view: true,
    send_delay_seconds: 5,
    mark_read_delay_ms: 1500,
    external_images_policy: 'ask',
    signature: '',
    quick_reply_templates: [],
    filter_rules: [],
  },
};

export const DEFAULT_DRIVE_USAGE = {
  used_bytes: 1_073_741_824,
  total_bytes: 16_106_127_360,
  file_count: 42,
};

export const DEFAULT_DRIVE_NODES = {
  nodes: [
    { id: 'drv-1', parent_id: null, name: 'Documents', type: 'folder', size: 0, modified_at: '2026-01-10T10:00:00Z' },
    { id: 'drv-2', parent_id: null, name: 'photo.jpg', type: 'file', size: 524288, mime_type: 'image/jpeg', modified_at: '2026-01-12T10:00:00Z' },
  ],
};

export const DEFAULT_DM_ROOMS = [
  {
    id: 'dm-room-1',
    company_id: 'company-1',
    domain_id: 'domain-1',
    room_type: 'group',
    visibility: 'private',
    name: 'Launch room',
    owner_id: 'user-1',
    created_by: 'user-1',
    created_at: '2026-05-25T00:00:00Z',
    current_user_id: 'user-1',
    members: [
      { id: 'user-1', display_name: 'Park Jangwon', email: 'pjw@parkjw.org', avatar_url: 'data:image/svg+xml,%3Csvg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 32 32%22%3E%3Crect width=%2232%22 height=%2232%22 fill=%22%232f6ee0%22/%3E%3Ctext x=%2216%22 y=%2221%22 text-anchor=%22middle%22 fill=%22white%22 font-size=%2212%22 font-family=%22Arial%22%3EPJ%3C/text%3E%3C/svg%3E' },
      { id: 'user-2', display_name: 'Kim Chulsoo', email: 'kim.chulsoo@parkjw.org', avatar_url: 'data:image/svg+xml,%3Csvg xmlns=%22http://www.w3.org/2000/svg%22 viewBox=%220 0 32 32%22%3E%3Crect width=%2232%22 height=%2232%22 fill=%22%230d9488%22/%3E%3Ctext x=%2216%22 y=%2221%22 text-anchor=%22middle%22 fill=%22white%22 font-size=%2212%22 font-family=%22Arial%22%3EKC%3C/text%3E%3C/svg%3E' },
    ],
    unread_count: 1,
    member_count: 2,
    last_message: {
      id: 'dm-msg-1',
      room_id: 'dm-room-1',
      sender_id: 'user-2',
      message_type: 'text',
      body: 'DM smoke hello',
      created_at: '2026-05-25T00:01:00Z',
    },
  },
];

export const DEFAULT_DM_MESSAGES = [
  {
    id: 'dm-msg-1',
    room_id: 'dm-room-1',
    sender_id: 'user-2',
    message_type: 'text',
    body: 'DM smoke hello',
    created_at: '2026-05-25T00:01:00Z',
    reactions: [],
  },
];

export const DEFAULT_DIRECTORY_USERS = [
  { id: 'user-2', display_name: 'Kim Chulsoo', email: 'kim.chulsoo@parkjw.org', avatar_url: AVATAR_DATA_URL },
  { id: 'user-3', display_name: 'Lee Younghee', email: 'lee.younghee@parkjw.org', avatar_url: AVATAR_DATA_URL },
];

export const DEFAULT_ORG_UNITS = [
  {
    id: 'org-dev',
    display_name: 'Development Team',
    depth: 0,
    members: [
      { id: 'user-2', display_name: 'Kim Chulsoo', email: 'kim.chulsoo@parkjw.org', avatar_url: AVATAR_DATA_URL },
    ],
  },
  {
    id: 'org-infra',
    display_name: 'Infrastructure Team',
    depth: 0,
    members: [
      { id: 'user-4', display_name: 'Kang Hyunjae', email: 'kang.hyunjae@parkjw.org', avatar_url: AVATAR_DATA_URL },
    ],
  },
];

// --- Mock installation -----------------------------------------------------

type Json = unknown;
type RouteHandler = (route: Route) => unknown | Promise<unknown>;

export interface MockOverrides {
  folders?: typeof DEFAULT_FOLDERS;
  messages?: typeof DEFAULT_MESSAGES;
  preferences?: typeof DEFAULT_PREFERENCES;
  user?: typeof DEFAULT_USER;
  driveNodes?: typeof DEFAULT_DRIVE_NODES;
  driveUsage?: typeof DEFAULT_DRIVE_USAGE;
  dmRooms?: typeof DEFAULT_DM_ROOMS;
  dmMessages?: typeof DEFAULT_DM_MESSAGES;
  directoryUsers?: typeof DEFAULT_DIRECTORY_USERS;
  orgUnits?: typeof DEFAULT_ORG_UNITS;
  notificationPreferences?: Json;
  deliveryStatuses?: Record<string, Json>;
  onNotificationPreferencesPut?: (body: Record<string, unknown>) => void;
  /** Extra raw route handlers — pattern is matched first; falls back to defaults */
  extra?: Array<{ urlPattern: string | RegExp; handler: RouteHandler }>;
  /** If true, /api/mail/folders returns 401 (auth failure) */
  unauthorized?: boolean;
}

function json(route: Route, body: Json, status = 200) {
  return route.fulfill({
    status,
    contentType: 'application/json',
    body: JSON.stringify(body),
  });
}

function empty(route: Route, status = 204) {
  return route.fulfill({ status, body: '' });
}

export async function installMocks(page: Page, overrides: MockOverrides = {}) {
  const folders = overrides.folders ?? DEFAULT_FOLDERS;
  const messages = overrides.messages ?? DEFAULT_MESSAGES;
  const user = overrides.user ?? DEFAULT_USER;
  const preferences = overrides.preferences ?? DEFAULT_PREFERENCES;
  const deliveryStatuses = overrides.deliveryStatuses ?? {};
  const notificationPreferences = overrides.notificationPreferences ?? {
    global_dnd_enabled: false,
    global_dnd_schedule: { weekdays: [], time_ranges: [], timezone: 'Asia/Seoul' },
    folder_overrides: {},
    thread_overrides: {},
    updated_at: '2026-05-23T00:00:00Z',
  };
  const driveNodes = overrides.driveNodes ?? DEFAULT_DRIVE_NODES;
  const driveUsage = overrides.driveUsage ?? DEFAULT_DRIVE_USAGE;
  let dmRooms = [...(overrides.dmRooms ?? DEFAULT_DM_ROOMS)];
  let dmMessages = [...(overrides.dmMessages ?? DEFAULT_DM_MESSAGES)];
  const directoryUsers = overrides.directoryUsers ?? DEFAULT_DIRECTORY_USERS;
  const orgUnits = overrides.orgUnits ?? DEFAULT_ORG_UNITS;
  const allDirectoryUsers = [
    ...directoryUsers,
    ...orgUnits.flatMap((unit) => unit.members ?? []),
  ];

  // Custom user-provided routes win first
  for (const { urlPattern, handler } of overrides.extra ?? []) {
    await page.route(urlPattern, handler);
  }

  // Auth endpoints (Next route handlers under /api/auth/**)
  await page.route('**/api/auth/login', async (route) => {
    if (route.request().method() !== 'POST') return route.fallback();
    const body = route.request().postDataJSON?.() as { email?: string; password?: string } | null;
    if (body && body.password === 'wrong') {
      return json(route, { error_message: '이메일 또는 비밀번호가 올바르지 않습니다.' }, 401);
    }
    return json(route, {
      expires_at: new Date(Date.now() + 3600_000).toISOString(),
      must_change_password: false,
    });
  });
  await page.route('**/api/auth/logout', (route) => empty(route, 200));
  await page.route('**/api/auth/mfa', (route) =>
    json(route, { expires_at: new Date(Date.now() + 3600_000).toISOString() })
  );
  await page.route('**/api/auth/password-reset/request', (route) => empty(route, 204));
  await page.route('**/api/auth/password-reset/confirm', (route) => empty(route, 204));

  // /api/mail/** proxy routes
  await page.route('**/api/mail/**', async (route) => {
    const req = route.request();
    const method = req.method();
    const url = new URL(req.url());
    // Strip query for matching
    const path = url.pathname.replace(/^.*\/api\/mail\//, '');
    const segments = path.split('/').filter(Boolean);
    const search = url.searchParams;

    if (overrides.unauthorized) {
      return json(route, { error_message: 'Unauthorized' }, 401);
    }

    // me
    if (path === 'me' && method === 'GET') return json(route, { user });
    if (path === 'me' && method === 'PATCH') return json(route, { user });
    if (path === 'me/avatar' && method === 'PUT') return json(route, { avatar_url: 'data:image/png;base64,bW9jaw==' });
    if (path === 'me/avatar' && method === 'DELETE') return json(route, { avatar_url: '' });
    if (path === 'me/addresses' && method === 'GET') return json(route, { addresses: [{ email: user.email, primary: true }] });
    if (path === 'me/password' && method === 'POST') return empty(route, 204);

    // preferences
    if (path === 'preferences' && method === 'GET') return json(route, preferences);
    if (path === 'preferences' && method === 'PUT') return json(route, preferences);
    if (path === 'preferences' && method === 'PATCH') return json(route, preferences);
    if (path === 'me/notification-preferences' && method === 'GET') return json(route, notificationPreferences);
    if (path === 'me/notification-preferences' && method === 'PUT') {
      const body = (req.postDataJSON?.() ?? {}) as Record<string, unknown>;
      overrides.onNotificationPreferencesPut?.(body);
      return json(route, { ...body, updated_at: '2026-05-23T00:01:00Z' });
    }

    // folders
    if (path === 'folders' && method === 'GET') return json(route, { folders });
    if (path === 'folders' && method === 'POST') {
      const body = (req.postDataJSON?.() ?? {}) as { name?: string };
      return json(route, { folder: { id: `folder-${Date.now()}`, name: body.name ?? 'New', full_path: body.name ?? 'New', type: 'custom', order_index: 99, total: 0, unread: 0, starred: 0 } });
    }
    if (segments[0] === 'folders' && segments.length === 2 && method === 'PATCH') return json(route, { folder: folders[0] });
    if (segments[0] === 'folders' && segments.length === 2 && method === 'DELETE') return empty(route, 204);

    // messages
    if (path === 'messages' && method === 'GET') {
      const folderId = search.get('folder_id')?.trim() ?? '';
      const filtered = folderId ? messages.filter((m) => m.folder_id === folderId) : messages;
      return json(route, { messages: filtered, has_more: false, next_cursor: '' });
    }
    if (segments[0] === 'messages' && segments.length === 3 && segments[2] === 'delivery-status' && method === 'GET') {
      return json(route, deliveryStatuses[segments[1]] ?? { delivery_status: null });
    }
    if (segments[0] === 'messages' && segments.length === 2 && method === 'GET') {
      const summary = messages.find((m) => m.id === segments[1]);
      return json(route, makeMessageDetail(segments[1], summary ?? {}));
    }
    if (segments[0] === 'messages' && segments.length === 3 && segments[2] === 'flags' && method === 'PATCH') {
      return json(route, { status: 'ok' });
    }
    if (segments[0] === 'messages' && segments.length === 3 && segments[2] === 'folder' && method === 'PATCH') {
      return json(route, { status: 'ok' });
    }
    if (segments[0] === 'messages' && segments[1] === 'bulk') return json(route, { updated: 1 });
    if (segments[0] === 'messages' && segments.length === 2 && method === 'DELETE') return empty(route, 204);
    if (segments[0] === 'messages' && method === 'POST' && segments.length === 1) {
      return json(route, { message: { id: `msg-${Date.now()}` } }, 201);
    }
    if (segments[0] === 'messages' && segments.length === 3 && segments[2] === 'tracking' && method === 'GET') {
      return json(route, { events: [] });
    }

    // threads
    if (path === 'threads' && method === 'GET') return json(route, { threads: [], has_more: false, next_cursor: '' });
    if (segments[0] === 'threads' && segments.length === 3 && segments[2] === 'messages') return json(route, { messages: [] });

    // dm
    if (path === 'dm/rooms' && method === 'GET') {
      return json(route, { rooms: dmRooms });
    }
    if (path === 'dm/rooms/public' && method === 'GET') {
      return json(route, { rooms: [] });
    }
    if (path === 'dm/rooms' && method === 'POST') {
      const body = (req.postDataJSON?.() ?? {}) as { room_type?: string; name?: string; user_ids?: string[]; visibility?: string };
      const room = {
        id: `dm-room-${Date.now()}`,
        company_id: 'company-1',
        domain_id: 'domain-1',
        room_type: body.room_type ?? 'direct',
        visibility: body.visibility ?? 'private',
        name: body.name ?? '',
        owner_id: 'user-1',
        created_by: 'user-1',
        created_at: new Date().toISOString(),
        current_user_id: 'user-1',
        members: [
          { id: 'user-1', display_name: 'Park Jangwon', email: 'pjw@parkjw.org', avatar_url: '' },
          ...allDirectoryUsers.filter((u) => body.user_ids?.includes(u.id)).map((u) => ({ ...u })),
        ],
        unread_count: 0,
        member_count: (body.user_ids?.length ?? 0) + 1,
      };
      dmRooms = [room, ...dmRooms] as typeof DEFAULT_DM_ROOMS;
      return json(route, { room }, 201);
    }
    if (segments[0] === 'dm' && segments[1] === 'rooms' && segments[3] === 'messages' && method === 'GET') {
      return json(route, { messages: dmMessages.filter((m) => m.room_id === segments[2]) });
    }
    if (segments[0] === 'dm' && segments[1] === 'rooms' && segments[3] === 'messages' && method === 'POST') {
      const body = (req.postDataJSON?.() ?? {}) as { body?: string; drive_file_id?: string };
      const message = {
        id: `dm-msg-${Date.now()}`,
        room_id: segments[2],
        sender_id: 'user-1',
        message_type: body.drive_file_id ? 'drive_link' : 'text',
        body: body.body ?? '',
        drive_file_id: body.drive_file_id,
        created_at: new Date().toISOString(),
        reactions: [],
      };
      dmMessages = [...dmMessages, message] as typeof DEFAULT_DM_MESSAGES;
      return json(route, { message }, 201);
    }
    if (segments[0] === 'dm' && segments[1] === 'rooms' && segments[3] === 'attachments' && method === 'POST') {
      const uploadData = req.postData() ?? '';
      const uploadName = /filename="([^"]+)"/.exec(uploadData)?.[1] ?? 'upload.txt';
      const uploadType = /Content-Type: ([^\r\n]+)/.exec(uploadData)?.[1] ?? 'application/octet-stream';
      const id = `dm-file-${Date.now()}`;
      const message = {
        id,
        room_id: segments[2],
        sender_id: 'user-1',
        message_type: 'file',
        body: uploadName,
        attachment_name: uploadName,
        attachment_size: 12,
        attachment_mime_type: uploadType,
        attachment_download_url: uploadType.startsWith('image/') ? `/api/v1/dm/messages/${id}/attachment?token=mock-${id}` : undefined,
        created_at: new Date().toISOString(),
        reactions: [],
      };
      dmMessages = [...dmMessages, message] as typeof DEFAULT_DM_MESSAGES;
      return json(route, { message }, 201);
    }
    if (segments[0] === 'dm' && segments[1] === 'rooms' && segments[3] === 'read' && method === 'POST') return empty(route, 204);
    if (segments[0] === 'dm' && segments[1] === 'rooms' && segments[3] === 'search' && method === 'GET') {
      const q = (search.get('q') ?? '').toLowerCase();
      return json(route, { results: dmMessages.filter((m) => m.body.toLowerCase().includes(q)).map((message) => ({ message })) });
    }
    if (segments[0] === 'dm' && segments[1] === 'rooms' && segments[3] === 'media' && method === 'GET') {
      return json(route, { media: [] });
    }
    if (segments[0] === 'dm' && segments[1] === 'rooms' && segments[3] === 'members' && method === 'POST') return json(route, { messages: [] });
    if (segments[0] === 'dm' && segments[1] === 'rooms' && segments[3] === 'members' && method === 'DELETE') return json(route, { removal: { deleted_room: false } });
    if (segments[0] === 'dm' && segments[1] === 'rooms' && segments[3] === 'owner' && method === 'PATCH') return json(route, { message: dmMessages[0] });
    if (segments[0] === 'dm' && segments[1] === 'rooms' && segments[3] === 'invites' && method === 'POST') return json(route, { invite: { token: 'invite-token', room_id: segments[2], expires_at: '2026-06-01T00:00:00Z' }, invite_url: 'http://localhost:3003/dm/join/invite-token' }, 201);
    if (segments[0] === 'dm' && segments[1] === 'join' && method === 'POST') return json(route, { message: dmMessages[0] });
    if (segments[0] === 'dm' && segments[1] === 'messages' && segments[3] === 'attachment' && method === 'GET') {
      return route.fulfill({
        status: 200,
        contentType: 'image/png',
        headers: { 'Cache-Control': 'no-store', 'X-Content-Type-Options': 'nosniff' },
        body: PNG_1X1,
      });
    }
    if (segments[0] === 'dm' && segments[1] === 'messages' && segments.length === 3 && method === 'PATCH') {
      const body = (req.postDataJSON?.() ?? {}) as { body?: string };
      const message = { ...(dmMessages.find((m) => m.id === segments[2]) ?? dmMessages[0]), body: body.body ?? '', edited_at: new Date().toISOString() };
      dmMessages = dmMessages.map((m) => m.id === message.id ? message : m) as typeof DEFAULT_DM_MESSAGES;
      return json(route, { message });
    }
    if (segments[0] === 'dm' && segments[1] === 'messages' && segments.length === 3 && method === 'DELETE') {
      const message = { ...(dmMessages.find((m) => m.id === segments[2]) ?? dmMessages[0]), deleted_at: new Date().toISOString(), body: '삭제된 메시지입니다.' };
      dmMessages = dmMessages.map((m) => m.id === message.id ? message : m) as typeof DEFAULT_DM_MESSAGES;
      return json(route, { message });
    }
    if (segments[0] === 'dm' && segments[1] === 'messages' && segments[3] === 'reactions' && method === 'PUT') {
      const body = (req.postDataJSON?.() ?? {}) as { emoji?: string };
      const emoji = body.emoji ?? decodeURIComponent(segments[4] ?? '');
      dmMessages = dmMessages.map((m) => {
        if (m.id !== segments[2]) return m;
        const reactions = [...((m.reactions ?? []) as Array<{ emoji: string; count: number; mine?: boolean }>)];
        const index = reactions.findIndex((r) => r.emoji === emoji);
        if (index >= 0) reactions[index] = { ...reactions[index], count: reactions[index].count + 1, mine: true };
        else reactions.push({ emoji, count: 1, mine: true });
        return { ...m, reactions } as typeof m;
      }) as typeof DEFAULT_DM_MESSAGES;
      return empty(route, 204);
    }

    // search
    if (path === 'search' && method === 'GET') {
      const q = search.get('q') ?? '';
      const hasAtt = search.get('has_attachment') === 'true';
      let results = messages.filter((m) => !q || m.subject.toLowerCase().includes(q.toLowerCase()));
      if (hasAtt) results = results.filter((m) => m.has_attachment);
      return json(route, { messages: results, has_more: false, next_cursor: '' });
    }

    // drafts
    if (path === 'drafts' && method === 'GET') return json(route, { drafts: [], has_more: false, next_cursor: '' });
    if (path === 'drafts' && method === 'POST') return json(route, { draft: { id: `draft-${Date.now()}` } }, 201);
    if (segments[0] === 'drafts' && segments.length === 2 && method === 'PATCH') return json(route, { draft: { id: segments[1] } });
    if (segments[0] === 'drafts' && segments.length === 2 && method === 'DELETE') return empty(route, 204);
    if (segments[0] === 'drafts' && segments.length === 3 && segments[2] === 'send') {
      return json(route, { message: { id: `sent-${Date.now()}` } });
    }

    // attachments
    if (path === 'attachments/upload' && method === 'POST') {
      return json(route, { id: 'att-up-1', message_id: '', upload_id: 'upl-up-1', filename: 'upload.bin', content_type: 'application/octet-stream', size: 16 });
    }
    if (segments[0] === 'messages' && segments[2] === 'attachments' && segments[4] === 'download') {
      return route.fulfill({ status: 200, contentType: 'application/octet-stream', body: 'attachment-bytes' });
    }

    // drive
    if (path === 'drive/usage' && method === 'GET') return json(route, driveUsage);
    if (path === 'drive/nodes' && method === 'GET') return json(route, driveNodes);
    if (path === 'drive/nodes' && method === 'POST') return json(route, { node: { id: `drv-${Date.now()}`, name: 'New folder', type: 'folder' } }, 201);
    if (segments[0] === 'drive' && segments[1] === 'folders' && method === 'POST') return json(route, { node: { id: `drv-${Date.now()}`, name: 'New folder', type: 'folder' } });
    if (segments[0] === 'drive' && segments[1] === 'nodes' && segments.length >= 3) {
      const tail = segments[3];
      if (method === 'DELETE') return empty(route, 204);
      if (tail === 'name' || tail === 'parent') return json(route, { node: { id: segments[2], name: 'Renamed' } });
      if (tail === 'trash' || tail === 'restore') return empty(route, 204);
      if (tail === 'download') return route.fulfill({ status: 200, contentType: 'application/octet-stream', body: 'file-bytes' });
      if (tail === 'share-links') return json(route, { share_link: { id: 'link-1', token: 'tok' } });
      if (!tail && method === 'GET') return json(route, { node: { id: segments[2], name: 'node', type: 'folder' } });
    }
    if (path === 'drive/upload-sessions' && method === 'POST') {
      return json(route, { id: `sess-${Date.now()}`, upload_url: '/api/mail/drive/upload-sessions/x/body' });
    }
    if (segments[0] === 'drive' && segments[1] === 'upload-sessions' && segments[3] === 'body') {
      return json(route, { received: 0 });
    }
    if (segments[0] === 'drive' && segments[1] === 'upload-sessions' && segments[3] === 'finalize') {
      return json(route, { node: { id: `drv-${Date.now()}`, name: 'uploaded', type: 'file' } });
    }
    if (segments[0] === 'drive' && segments[1] === 'upload-sessions' && segments.length === 3) {
      if (method === 'GET') return json(route, { id: segments[2], status: 'in_progress', received_bytes: 0 });
      if (method === 'DELETE') return empty(route, 204);
      return json(route, { id: segments[2] });
    }
    if (path === 'drive/share-links' || segments[0] === 'drive' && segments[1] === 'share-links') {
      if (method === 'GET') return json(route, { share_links: [] });
      if (method === 'POST') return json(route, { share_link: { id: 'link-1' } });
      if (method === 'DELETE') return empty(route, 204);
    }

    // calendars / events
    if (path === 'calendars' && method === 'GET') return json(route, { calendars: [{ id: 'cal-1', name: 'Personal', color: '#4F46E5' }] });
    if (segments[0] === 'calendars' && segments[2] === 'events' && method === 'GET') return json(route, { events: [] });
    if (segments[0] === 'calendars' && segments[2] === 'events' && method === 'POST') return json(route, { event: { id: `evt-${Date.now()}` } }, 201);
    if (path === 'events' && method === 'GET') return json(route, { events: [] });
    if (segments[0] === 'events' && segments.length === 2) {
      if (method === 'PATCH') return json(route, { event: { id: segments[1] } });
      if (method === 'DELETE') return empty(route, 204);
    }
    if (segments[0] === 'calendar-subscriptions') return json(route, { events: [] });

    // contacts / address books
    if (path === 'contacts' && method === 'GET') return json(route, { contacts: [] });
    if (path === 'contacts/autocomplete') return json(route, { contacts: [] });
    if (segments[0] === 'addressbooks') return json(route, { addressbooks: [{ id: 'ab-1', name: '내 주소록', description: '' }] });
    if (segments[0] === 'contacts' && segments.length === 2) {
      if (method === 'PATCH' || method === 'PUT') return json(route, { contact: { id: segments[1] } });
      if (method === 'DELETE') return empty(route, 204);
    }

    // directory
    if (segments[0] === 'directory' && segments[1] === 'users') return json(route, { users: directoryUsers });
    if (segments[0] === 'directory' && segments[1] === 'org-tree') return json(route, { units: orgUnits });
    if (segments[0] === 'directory') return json(route, { users: [], nodes: [] });

    // signatures, filter rules, quick reply templates
    if (path === 'signatures' || path === 'me/signatures') return json(route, { signatures: [] });
    if (path === 'filter-rules' || path === 'me/filter-rules') {
      if (method === 'GET') return json(route, { rules: [] });
      if (method === 'POST') return json(route, { rule: { id: 'rule-1' } }, 201);
    }
    if (path === 'quick-reply-templates' || path === 'me/quick-reply-templates') {
      if (method === 'GET') return json(route, { templates: [] });
    }

    // settings
    if (path === 'me/settings' && method === 'GET') return json(route, { settings: preferences.preferences });
    if (path === 'me/settings' && (method === 'PUT' || method === 'PATCH')) return json(route, { settings: preferences.preferences });

    // sessions
    if (path === 'auth/sessions/revoke-all' && method === 'POST') return empty(route, 200);
    if (path === 'auth/mfa/status' && method === 'GET') return json(route, { mfa_status: { enrolled: false, enabled: false } });

    // Folder stats
    if (path === 'folders/stats' && method === 'GET') return json(route, { stats: [] });

    // Fallback: unmocked /api/mail path → 404 with helpful body
    return json(route, {
      error_message: `[mocks.ts] unmocked /api/mail route: ${method} ${path}`,
    }, 404);
  });
}
