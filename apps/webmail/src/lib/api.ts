export interface Folder {
  id: string;
  name: string;
  full_path: string;
  type: string;
  system_type?: string;
  total: number;
  unread: number;
  starred: number;
}

export interface MessageSummary {
  id: string;
  subject: string;
  from_addr: string;
  from_name: string;
  received_at: string;
  read: boolean;
  starred: boolean;
  has_attachment: boolean;
  preview: string;
  // Thread view optional fields
  thread_id?: string;
  message_count?: number;
  unread_count?: number;
}

export interface MessageDetail {
  id: string;
  subject: string;
  from_addr: string;
  from_name: string;
  to_addrs: { address: string; name?: string }[];
  cc_addrs?: { address: string; name?: string }[];
  received_at: string;
  text_body: string;
  html_body?: string;
  has_attachment: boolean;
}

export interface AuthTokenResponse {
  token: string;
  expires_at: string;
  must_change_password: boolean;
  client_ip?: string;
}

export interface Attachment {
  id: string;
  message_id: string;
  filename: string;
  size: number;
  mime_type: string;
  status: 'uploading' | 'stored' | 'deleted';
  created_at: string;
}

export type ComposeIntent = 'new' | 'reply' | 'reply_all' | 'forward';

export interface SendMessageRequest {
  to: { address: string; name?: string }[];
  cc?: { address: string; name?: string }[];
  bcc?: { address: string; name?: string }[];
  subject: string;
  text_body: string;
  html_body?: string;
  from?: string;
  intent?: ComposeIntent;
  source_message_id?: string;
  attachment_ids?: string[];
  scheduled_at?: string;
}

function getToken(): string | null {
  if (typeof window === 'undefined') return null;
  return localStorage.getItem('webmail_token');
}

function clearTokenAndRedirect(): void {
  localStorage.removeItem('webmail_token');
  window.location.href = '/login';
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getToken();
  const headers = new Headers(options.headers as HeadersInit | undefined);

  if (token) {
    headers.set('Authorization', `Bearer ${token}`);
  }
  if (!headers.has('Content-Type') && options.body) {
    headers.set('Content-Type', 'application/json');
  }

  const res = await fetch(`/api/mail/${path}`, {
    ...options,
    headers,
  });

  if (res.status === 401) {
    clearTokenAndRedirect();
    throw new Error('Unauthorized');
  }

  if (!res.ok) {
    let message = `Request failed: ${res.status}`;
    try {
      const errBody = (await res.json()) as { error?: string; message?: string };
      message = errBody.error ?? errBody.message ?? message;
    } catch {
      // ignore parse error
    }
    throw new Error(message);
  }

  if (res.status === 204) {
    return undefined as unknown as T;
  }

  return res.json() as Promise<T>;
}

export async function loginUser(
  email: string,
  password: string
): Promise<AuthTokenResponse> {
  const res = await fetch('/api/mail/auth/token', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ email, password }),
  });

  if (!res.ok) {
    let message = '로그인에 실패했습니다.';
    try {
      const errBody = (await res.json()) as { error?: string; message?: string };
      message = errBody.error ?? errBody.message ?? message;
    } catch {
      // ignore
    }
    throw new Error(message);
  }

  return res.json() as Promise<AuthTokenResponse>;
}

function apiGet<T>(path: string, params?: Record<string, string>): Promise<T> {
  const search = params ? '?' + new URLSearchParams(params).toString() : '';
  return request<T>(`${path}${search}`, { method: 'GET' });
}

function apiPost<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'POST',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

function apiPatch<T>(path: string, body?: unknown): Promise<T> {
  return request<T>(path, {
    method: 'PATCH',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
}

function apiDelete<T>(path: string): Promise<T> {
  return request<T>(path, { method: 'DELETE' });
}

export interface SearchParams {
  q?: string;
  folder_id?: string;
  from?: string;
  to?: string;
  subject?: string;
  since?: string;
  until?: string;
  has_attachment?: boolean;
  limit?: number;
  cursor?: string;
}

export function searchMessages(
  params: SearchParams
): Promise<{ messages: MessageSummary[]; has_more: boolean; next_cursor: string }> {
  const p: Record<string, string> = {};
  if (params.q) p.q = params.q;
  if (params.folder_id) p.folder_id = params.folder_id;
  if (params.from) p.from = params.from;
  if (params.to) p.to = params.to;
  if (params.subject) p.subject = params.subject;
  if (params.since) p.since = params.since;
  if (params.until) p.until = params.until;
  if (params.has_attachment !== undefined) p.has_attachment = String(params.has_attachment);
  if (params.limit) p.limit = String(params.limit);
  if (params.cursor) p.cursor = params.cursor;
  return apiGet<{ messages: MessageSummary[]; has_more: boolean; next_cursor: string }>(
    'search',
    p
  );
}

export function getFolders(): Promise<{ folders: Folder[] }> {
  return apiGet<{ folders: Folder[] }>('folders');
}

export function getMessages(
  folderId: string,
  cursor = '',
  limit = 50
): Promise<{ messages: MessageSummary[]; has_more: boolean; next_cursor: string }> {
  const params: Record<string, string> = {
    folder_id: folderId,
    limit: String(limit),
  };
  if (cursor) params.cursor = cursor;
  return apiGet<{ messages: MessageSummary[]; has_more: boolean; next_cursor: string }>(
    'messages',
    params
  );
}

export async function getMessage(id: string): Promise<MessageDetail> {
  const res = await apiGet<{ message: MessageDetail }>(`messages/${id}`);
  return res.message;
}

export function markRead(id: string, value: boolean): Promise<{ status: string }> {
  return apiPatch<{ status: string }>(`messages/${id}/flags`, { flag: 'read', value });
}

export function starMessage(id: string, value: boolean): Promise<{ status: string }> {
  return apiPatch<{ status: string }>(`messages/${id}/flags`, { flag: 'starred', value });
}

export function moveMessage(id: string, folderId: string): Promise<{ status: string }> {
  return apiPatch<{ status: string }>(`messages/${id}/folder`, { folder_id: folderId });
}

export function bulkMarkRead(ids: string[], value: boolean): Promise<{ updated: number }> {
  return apiPatch<{ updated: number }>('messages/bulk/flags', { message_ids: ids, flag: 'read', value });
}

export interface DraftData {
  draft_id?: string;
  intent: ComposeIntent;
  source_message_id?: string;
  to: { address: string; name?: string }[];
  cc?: { address: string; name?: string }[];
  bcc?: { address: string; name?: string }[];
  subject: string;
  text_body: string;
}

export function saveDraft(data: DraftData): Promise<{ draft: { id: string } }> {
  return apiPost<{ draft: { id: string } }>('drafts', data);
}

export function updateDraft(draftId: string, data: DraftData): Promise<{ draft: { id: string } }> {
  return apiPatch<{ draft: { id: string } }>(`drafts/${draftId}`, data);
}

export function deleteMessage(id: string): Promise<void> {
  return apiDelete<void>(`messages/${id}`);
}

export function createFolder(name: string): Promise<{ folder: Folder }> {
  return apiPost<{ folder: Folder }>('folders', { name });
}

export function renameFolder(id: string, name: string): Promise<{ folder: Folder }> {
  return apiPatch<{ folder: Folder }>(`folders/${id}`, { name });
}

export function deleteFolder(id: string): Promise<void> {
  return apiDelete<void>(`folders/${id}`);
}

export function restoreMessage(id: string): Promise<void> {
  return apiPost<void>(`messages/${id}/restore`, {});
}

export function bulkRestoreMessages(ids: string[]): Promise<void> {
  return apiPost<void>('messages/bulk/restore', { message_ids: ids });
}

export function sendMessage(data: SendMessageRequest): Promise<void> {
  return apiPost<void>('messages/send', data);
}

export interface ContactSuggestion {
  display_name: string;
  email: string;
}

export async function autocompleteContacts(q: string, limit = 8): Promise<ContactSuggestion[]> {
  if (!q.trim()) return [];
  const params = new URLSearchParams({ q, limit: String(limit) });
  try {
    const token = getToken();
    const res = await fetch(`/api/mail/contacts/autocomplete?${params}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    });
    if (!res.ok) return [];
    const data = await res.json() as { results?: ContactSuggestion[] };
    return data.results ?? [];
  } catch { return []; }
}

export async function uploadAttachment(file: File, draftId?: string): Promise<Attachment> {
  const token = getToken();
  const form = new FormData();
  form.append('file', file);
  if (draftId) form.append('draft_id', draftId);
  const res = await fetch('/api/mail/attachments/upload', {
    method: 'POST',
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    body: form,
  });
  if (res.status === 401) { clearTokenAndRedirect(); throw new Error('Unauthorized'); }
  if (!res.ok) {
    let msg = `Upload failed: ${res.status}`;
    try { const b = (await res.json()) as { error?: string }; msg = b.error ?? msg; } catch { /* */ }
    throw new Error(msg);
  }
  const body = (await res.json()) as { attachment: Attachment };
  return body.attachment;
}

export function listAttachments(messageId: string): Promise<Attachment[]> {
  return request<{ attachments: Attachment[] }>(`messages/${messageId}/attachments`).then((r) => r.attachments);
}

export async function downloadAttachment(messageId: string, attachmentId: string, filename: string): Promise<void> {
  const token = getToken();
  const res = await fetch(`/api/mail/messages/${messageId}/attachments/${attachmentId}/download`, {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
  });
  if (!res.ok) throw new Error(`Download failed: ${res.status}`);
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  document.body.appendChild(a);
  a.click();
  setTimeout(() => { document.body.removeChild(a); URL.revokeObjectURL(url); }, 1000);
}

// ── Contacts / Address Books ─────────────────────────────────────────────────

export interface AddressBook {
  ID: string;
  Name: string;
  Description: string;
  UserID: string;
}

export interface ContactObject {
  ID: string;
  AddressBookID: string;
  ObjectName: string;
  UID: string;
  VCard: string; // base64-encoded vCard bytes
  CreatedAt: string;
  UpdatedAt: string;
}

/** Parse vCard fields from base64-encoded vCard data. */
export function parseVCard(base64VCard: string): {
  fn: string; email: string; tel: string; org: string; note: string; title: string;
} {
  let text = '';
  try { text = atob(base64VCard); } catch { text = base64VCard; }
  const get = (prop: string) => {
    const m = text.match(new RegExp(`(?:^|\\n)${prop}[;:][^\\n]*:([^\\n]*)`, 'im'));
    return m ? m[1].trim() : '';
  };
  return {
    fn: get('FN'),
    email: get('EMAIL'),
    tel: get('TEL'),
    org: get('ORG'),
    title: get('TITLE'),
    note: get('NOTE'),
  };
}

export async function listAddressBooks(): Promise<AddressBook[]> {
  try {
    const data = await request<{ address_books?: AddressBook[] }>('addressbooks');
    return data.address_books ?? [];
  } catch { return []; }
}

export async function listContacts(addressBookId: string): Promise<ContactObject[]> {
  try {
    const data = await request<{ contacts?: ContactObject[] }>(`addressbooks/${encodeURIComponent(addressBookId)}/contacts`);
    return data.contacts ?? [];
  } catch { return []; }
}

export async function deleteContact(addressBookId: string, objectName: string): Promise<void> {
  await request<void>(`addressbooks/${encodeURIComponent(addressBookId)}/contacts/${encodeURIComponent(objectName)}`, { method: 'DELETE' });
}

// ── Calendars ────────────────────────────────────────────────────────────────

export interface Calendar {
  ID: string;
  UserID: string;
  Name: string;
  Color: string;
  Description: string;
  SyncToken: string;
  CreatedAt: string;
  UpdatedAt: string;
}

export interface CalendarObject {
  ID: string;
  UserID: string;
  CalendarID: string;
  ObjectName: string;
  UID: string;
  Component: string;
  ETag: string;
  Size: number;
  ICS: string; // base64-encoded iCalendar bytes
  CreatedAt: string;
  UpdatedAt: string;
}

/** Parse key iCal fields from base64-encoded ICS data. */
export function parseICS(base64ICS: string): {
  summary: string;
  description: string;
  location: string;
  dtstart: string;
  dtend: string;
  allDay: boolean;
} {
  let text = '';
  try { text = atob(base64ICS); } catch { text = base64ICS; }

  // Unfold long lines (RFC 5545 line folding: CRLF + whitespace)
  text = text.replace(/\r\n[ \t]/g, '').replace(/\n[ \t]/g, '');

  const get = (prop: string): string => {
    const m = text.match(new RegExp(`(?:^|\\n)${prop}(?:;[^\\n:]*)?:([^\\n]*)`, 'im'));
    return m ? m[1].trim() : '';
  };

  const dtstart = get('DTSTART');
  const dtend = get('DTEND');
  // All-day events use DATE format (8 digits, no T)
  const allDay = /^\d{8}$/.test(dtstart);

  return {
    summary: get('SUMMARY'),
    description: get('DESCRIPTION'),
    location: get('LOCATION'),
    dtstart,
    dtend,
    allDay,
  };
}

/** Convert iCal date/datetime string to JS Date. */
export function icalDateToDate(dtStr: string): Date | null {
  if (!dtStr) return null;
  // DATE format: YYYYMMDD
  if (/^\d{8}$/.test(dtStr)) {
    const y = parseInt(dtStr.slice(0, 4), 10);
    const mo = parseInt(dtStr.slice(4, 6), 10) - 1;
    const d = parseInt(dtStr.slice(6, 8), 10);
    return new Date(y, mo, d);
  }
  // DATETIME format: YYYYMMDDTHHmmss[Z]
  const m = dtStr.match(/^(\d{4})(\d{2})(\d{2})T(\d{2})(\d{2})(\d{2})(Z?)$/);
  if (m) {
    const [, y, mo, d, h, min, s, z] = m;
    if (z === 'Z') {
      return new Date(Date.UTC(+y, +mo - 1, +d, +h, +min, +s));
    }
    return new Date(+y, +mo - 1, +d, +h, +min, +s);
  }
  return null;
}

export async function listCalendars(): Promise<Calendar[]> {
  try {
    const data = await request<{ calendars?: Calendar[] }>('calendars');
    return data.calendars ?? [];
  } catch { return []; }
}

export async function listCalendarObjects(calendarId: string): Promise<CalendarObject[]> {
  try {
    const data = await request<{ objects?: CalendarObject[] }>(`calendars/${encodeURIComponent(calendarId)}/objects`);
    return data.objects ?? [];
  } catch { return []; }
}

export interface DirectoryUser {
  id: string;
  display_name: string;
  email: string;
}

export async function listDirectoryUsers(q?: string, limit = 50): Promise<DirectoryUser[]> {
  try {
    const params = new URLSearchParams();
    if (q) params.set('q', q);
    params.set('limit', String(limit));
    const token = localStorage.getItem('webmail_token');
    const res = await fetch(`/api/v1/directory/users?${params}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    });
    if (!res.ok) return [];
    const data = await res.json() as { users?: DirectoryUser[] };
    return data.users ?? [];
  } catch { return []; }
}

// ── Threads ──────────────────────────────────────────────────────────────────

export interface ThreadSummary {
  id: string;
  subject: string;
  preview: string;
  message_count: number;
  unread_count: number;
  latest_message_id: string;
  latest_from_addr: string;
  latest_at: string;
  has_attachment: boolean;
  starred: boolean;
}

export async function listThreads(params: {
  folder_id?: string;
  starred?: boolean;
  read?: boolean;
  limit?: number;
  cursor?: string;
}): Promise<{ threads: ThreadSummary[]; has_more: boolean; next_cursor: string }> {
  try {
    const token = typeof window !== 'undefined' ? localStorage.getItem('webmail_token') : null;
    const p = new URLSearchParams();
    if (params.folder_id) p.set('folder_id', params.folder_id);
    if (params.starred !== undefined) p.set('starred', String(params.starred));
    if (params.read !== undefined) p.set('read', String(params.read));
    if (params.limit !== undefined) p.set('limit', String(params.limit));
    if (params.cursor) p.set('cursor', params.cursor);
    const res = await fetch(`/api/v1/threads?${p}`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    });
    if (!res.ok) return { threads: [], has_more: false, next_cursor: '' };
    return res.json() as Promise<{ threads: ThreadSummary[]; has_more: boolean; next_cursor: string }>;
  } catch { return { threads: [], has_more: false, next_cursor: '' }; }
}

export async function listThreadMessages(threadId: string): Promise<MessageSummary[]> {
  try {
    const token = typeof window !== 'undefined' ? localStorage.getItem('webmail_token') : null;
    const res = await fetch(`/api/v1/threads/${encodeURIComponent(threadId)}/messages?limit=50`, {
      headers: token ? { Authorization: `Bearer ${token}` } : {},
    });
    if (!res.ok) return [];
    const data = await res.json() as { messages?: MessageSummary[] };
    return data.messages ?? [];
  } catch { return []; }
}
