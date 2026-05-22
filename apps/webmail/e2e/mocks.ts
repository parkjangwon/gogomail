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
  const driveNodes = overrides.driveNodes ?? DEFAULT_DRIVE_NODES;
  const driveUsage = overrides.driveUsage ?? DEFAULT_DRIVE_USAGE;

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
    if (path === 'me/addresses' && method === 'GET') return json(route, { addresses: [{ email: user.email, primary: true }] });
    if (path === 'me/password' && method === 'POST') return empty(route, 204);

    // preferences
    if (path === 'preferences' && method === 'GET') return json(route, preferences);
    if (path === 'preferences' && method === 'PUT') return json(route, preferences);
    if (path === 'preferences' && method === 'PATCH') return json(route, preferences);

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
      const folderId = search.get('folder_id') ?? 'folder-inbox';
      const filtered = messages.filter((m) => m.folder_id === folderId || folderId === 'folder-inbox');
      return json(route, { messages: filtered, has_more: false, next_cursor: '' });
    }
    if (segments[0] === 'messages' && segments.length === 2 && method === 'GET') {
      return json(route, makeMessageDetail(segments[1]));
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
