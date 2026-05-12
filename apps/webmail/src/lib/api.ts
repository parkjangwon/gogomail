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
  track_opens?: boolean;
}

export interface TrackingEvent {
  recipient_email: string;
  opened_at: string | null;
  open_count: number;
}

export async function getMessageTracking(messageId: string): Promise<TrackingEvent[]> {
  try {
    const data = await request<{ events?: TrackingEvent[] }>(`messages/${encodeURIComponent(messageId)}/tracking`);
    return data.events ?? [];
  } catch { return []; }
}

function clearTokenAndRedirect(): void {
  fetch('/api/auth/logout', { method: 'POST' }).catch(() => {});
  localStorage.removeItem('webmail_authenticated');
  localStorage.removeItem('webmail_email');
  localStorage.removeItem('webmail_token_expires_at');
  localStorage.removeItem('webmail_must_change_password');
  window.location.href = '/login';
}

async function request<T>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const headers = new Headers(options.headers as HeadersInit | undefined);
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
  const res = await fetch('/api/auth/login', {
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

export async function revokeAllSessions(): Promise<boolean> {
  try {
    const res = await fetch('/api/mail/auth/sessions/revoke-all', { method: 'POST' });
    return res.ok;
  } catch { return false; }
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
  attachment_ids?: string[];
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

// ─── Storage / Backup / Restore ───────────────────────────────────────────────

export interface FolderStats {
  id: string;
  name: string;
  system_type?: string;
  total: number;
  unread: number;
  starred: number;
  size_bytes: number;
}

export async function getFolderStats(): Promise<FolderStats[]> {
  const { folders } = await getFolders();
  return folders.map((f) => ({ ...f, size_bytes: f.total * 32 * 1024 }));
}

function formatEml(msg: MessageDetail): string {
  const date = new Date(msg.received_at ?? Date.now()).toUTCString();
  const to = msg.to_addrs.map((a) => (a.name ? `"${a.name}" <${a.address}>` : a.address)).join(', ');
  const from = msg.from_name ? `"${msg.from_name}" <${msg.from_addr}>` : msg.from_addr;
  const body = msg.html_body ?? msg.text_body ?? '';
  return [
    `From: ${from}`,
    `To: ${to}`,
    `Subject: ${msg.subject ?? ''}`,
    `Date: ${date}`,
    `MIME-Version: 1.0`,
    `Content-Type: ${msg.html_body ? 'text/html' : 'text/plain'}; charset=utf-8`,
    ``,
    body,
  ].join('\r\n');
}

async function fetchAllMessages(
  folderId: string,
  onProgress?: (fetched: number, total: number) => void
): Promise<MessageDetail[]> {
  const details: MessageDetail[] = [];
  let cursor = '';
  let estimatedTotal = 0;
  while (true) {
    const { messages, has_more, next_cursor } = await getMessages(folderId, cursor, 50);
    if (!cursor) estimatedTotal = has_more ? messages.length * 2 : messages.length;
    for (const summary of messages) {
      const detail = await getMessage(summary.id);
      details.push(detail);
      onProgress?.(details.length, Math.max(estimatedTotal, details.length));
    }
    if (!has_more) break;
    cursor = next_cursor;
    estimatedTotal = Math.max(estimatedTotal, details.length + 50);
  }
  return details;
}

export async function exportFolderEml(
  folderId: string,
  folderName: string,
  onProgress?: (fetched: number, total: number) => void
): Promise<void> {
  const messages = await fetchAllMessages(folderId, onProgress);
  const mbox = messages
    .map((m) => `From ${m.from_addr} ${new Date(m.received_at).toUTCString()}\r\n${formatEml(m)}`)
    .join('\r\n\r\n');
  const blob = new Blob([mbox], { type: 'application/mbox' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${folderName}-backup.mbox`;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 5000);
}

export async function exportFolderZip(
  folderId: string,
  folderName: string,
  onProgress?: (fetched: number, total: number) => void
): Promise<void> {
  const { zipSync, strToU8 } = await import('fflate');
  const messages = await fetchAllMessages(folderId, onProgress);
  const files: Record<string, Uint8Array> = {};
  messages.forEach((m, i) => {
    const safeSubject = (m.subject ?? 'untitled').replace(/[^\w가-힣\s-]/g, '').trim().slice(0, 48) || 'untitled';
    files[`${String(i + 1).padStart(4, '0')}-${safeSubject}.eml`] = strToU8(formatEml(m));
  });
  const zipped = zipSync(files);
  const blob = new Blob([zipped.buffer as ArrayBuffer], { type: 'application/zip' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = `${folderName}-backup.zip`;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 5000);
}

export async function restoreMailbox(folderId: string, file: File): Promise<{ imported: number }> {
  const formData = new FormData();
  formData.append('file', file);
  formData.append('folder_id', folderId);
  const res = await fetch('/api/mail/messages/restore', {
    method: 'POST',
    body: formData,
  });
  if (!res.ok) {
    let msg = `Restore failed: ${res.status}`;
    try { const e = (await res.json()) as { error?: string }; msg = e.error ?? msg; } catch { /* */ }
    throw new Error(msg);
  }
  return res.json() as Promise<{ imported: number }>;
}

export function sendMessage(data: SendMessageRequest): Promise<void> {
  return apiPost<void>('messages/send', data);
}

export interface ContactSuggestion {
  type?: string;
  display_name: string;
  email: string;
}

export async function autocompleteContacts(q: string, limit = 8): Promise<ContactSuggestion[]> {
  if (!q.trim()) return [];
  const params = new URLSearchParams({ q, limit: String(limit) });
  try {
    const res = await fetch(`/api/mail/contacts/autocomplete?${params}`);
    if (!res.ok) return [];
    const data = await res.json() as { results?: ContactSuggestion[] };
    return data.results ?? [];
  } catch { return []; }
}

export async function uploadAttachment(file: File, draftId?: string): Promise<Attachment> {
  const form = new FormData();
  form.append('file', file);
  if (draftId) form.append('draft_id', draftId);
  const res = await fetch('/api/mail/attachments/upload', {
    method: 'POST',
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
  const res = await fetch(`/api/mail/messages/${messageId}/attachments/${attachmentId}/download`);
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

export async function attachDriveFileToEmail(
  nodeId: string,
  filename: string,
  mimeType: string,
  draftId?: string
): Promise<Attachment | null> {
  const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/download`);
  if (!res.ok) return null;
  const blob = await res.blob();
  const file = new File([blob], filename, { type: mimeType || blob.type });
  try { return await uploadAttachment(file, draftId); } catch { return null; }
}

export async function saveAttachmentToDrive(
  messageId: string,
  attachmentId: string,
  filename: string,
  mimeType: string,
  parentId?: string
): Promise<DriveNode | null> {
  const attachRes = await fetch(`/api/mail/messages/${messageId}/attachments/${attachmentId}/download`);
  if (!attachRes.ok) return null;
  const blob = await attachRes.blob();
  const file = new File([blob], filename, { type: mimeType || blob.type });
  return uploadDriveFile(file, parentId);
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

export async function createAddressBook(name: string, description = ''): Promise<AddressBook> {
  const data = await request<{ address_book: AddressBook }>('addressbooks', {
    method: 'POST',
    body: JSON.stringify({ name, description }),
  });
  return data.address_book;
}

export async function renameAddressBook(id: string, name: string): Promise<AddressBook> {
  const data = await request<{ address_book: AddressBook }>(`addressbooks/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify({ name }),
  });
  return data.address_book;
}

export async function deleteAddressBook(id: string): Promise<void> {
  await request<void>(`addressbooks/${encodeURIComponent(id)}`, { method: 'DELETE' });
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

export interface DeliveryAttempt {
  id: string;
  recipient: string;
  status: string;
  error_message: string;
  attempted_at: string;
}

export interface MessageDeliveryStatus {
  message_id: string;
  delivery_status: string;
  bounce_status: string;
  attempts: DeliveryAttempt[];
  updated_at: string;
}

export async function getMessageDeliveryStatus(messageId: string): Promise<MessageDeliveryStatus | null> {
  try {
    const data = await request<{ delivery_status: MessageDeliveryStatus }>(`messages/${encodeURIComponent(messageId)}/delivery-status`);
    return data.delivery_status ?? null;
  } catch { return null; }
}

export interface UpsertContactFields {
  fn: string;
  email?: string;
  tel?: string;
  org?: string;
  title?: string;
  note?: string;
}

function vcardEscape(s: string): string { return s.replace(/\\/g, '\\\\').replace(/,/g, '\\,').replace(/;/g, '\\;').replace(/\n/g, '\\n'); }

export async function upsertContact(addressBookId: string, objectName: string, fields: UpsertContactFields): Promise<ContactObject> {
  const lines = ['BEGIN:VCARD', 'VERSION:3.0', `FN:${vcardEscape(fields.fn)}`];
  const nameParts = fields.fn.trim().split(/\s+/);
  const last = nameParts.length > 1 ? nameParts[nameParts.length - 1] : '';
  const first = nameParts.length > 1 ? nameParts.slice(0, -1).join(' ') : nameParts[0];
  lines.push(`N:${vcardEscape(last)};${vcardEscape(first)};;;`);
  if (fields.email) lines.push(`EMAIL:${vcardEscape(fields.email)}`);
  if (fields.tel) lines.push(`TEL:${vcardEscape(fields.tel)}`);
  if (fields.org) lines.push(`ORG:${vcardEscape(fields.org)}`);
  if (fields.title) lines.push(`TITLE:${vcardEscape(fields.title)}`);
  if (fields.note) lines.push(`NOTE:${vcardEscape(fields.note)}`);
  lines.push('END:VCARD');
  const vcard = lines.join('\r\n');
  const data = await request<{ contact: ContactObject }>(`addressbooks/${encodeURIComponent(addressBookId)}/contacts/${encodeURIComponent(objectName)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/vcard' },
    body: vcard,
  });
  return data.contact;
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

export async function createCalendar(name: string, color: string, description = ''): Promise<Calendar> {
  const data = await request<{ calendar: Calendar }>('calendars', {
    method: 'POST',
    body: JSON.stringify({ name, color, description }),
  });
  return data.calendar;
}

export async function updateCalendar(id: string, patch: { name?: string; color?: string; description?: string }): Promise<void> {
  await request<unknown>(`calendars/${encodeURIComponent(id)}`, {
    method: 'PATCH',
    body: JSON.stringify(patch),
  });
}

export async function deleteCalendar(id: string): Promise<void> {
  await request<unknown>(`calendars/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export interface CreateCalendarEventRequest {
  title: string;
  start: Date;
  end: Date;
  allDay: boolean;
  location?: string;
  description?: string;
  rrule?: string;
}

function pad2(n: number): string { return String(n).padStart(2, '0'); }
function toICSDate(d: Date): string {
  return `${d.getUTCFullYear()}${pad2(d.getUTCMonth() + 1)}${pad2(d.getUTCDate())}T${pad2(d.getUTCHours())}${pad2(d.getUTCMinutes())}${pad2(d.getUTCSeconds())}Z`;
}
function toICSAllDay(d: Date): string {
  return `${d.getFullYear()}${pad2(d.getMonth() + 1)}${pad2(d.getDate())}`;
}
function icsEscape(s: string): string { return s.replace(/\\/g, '\\\\').replace(/;/g, '\\;').replace(/,/g, '\\,').replace(/\n/g, '\\n'); }

export async function createCalendarEvent(calendarId: string, req: CreateCalendarEventRequest): Promise<void> {
  const uid = `${Date.now()}-${Math.random().toString(36).slice(2)}@gogomail`;
  const objectName = `${uid}.ics`;
  const lines: string[] = [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//GoGoMail//GoGoMail//EN',
    'BEGIN:VEVENT',
    `UID:${uid}`,
    `SUMMARY:${icsEscape(req.title)}`,
  ];
  if (req.allDay) {
    lines.push(`DTSTART;VALUE=DATE:${toICSAllDay(req.start)}`);
    const endDate = new Date(req.end);
    endDate.setDate(endDate.getDate() + 1);
    lines.push(`DTEND;VALUE=DATE:${toICSAllDay(endDate)}`);
  } else {
    lines.push(`DTSTART:${toICSDate(req.start)}`);
    lines.push(`DTEND:${toICSDate(req.end)}`);
  }
  if (req.location) lines.push(`LOCATION:${icsEscape(req.location)}`);
  if (req.description) lines.push(`DESCRIPTION:${icsEscape(req.description)}`);
  if (req.rrule) lines.push(`RRULE:${req.rrule}`);
  lines.push('END:VEVENT', 'END:VCALENDAR');
  const ics = lines.join('\r\n');
  await request<unknown>(`calendars/${encodeURIComponent(calendarId)}/objects/${encodeURIComponent(objectName)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'text/calendar' },
    body: ics,
  });
}

// ── Calendar Todos (VTODO) ───────────────────────────────────────────────────

export interface ParsedVTODOFields {
  summary: string;
  description: string;
  due: string;
  status: string;
}

export function parseVTODOICS(base64ICS: string): ParsedVTODOFields {
  let text = '';
  try { text = atob(base64ICS); } catch { text = base64ICS; }
  text = text.replace(/\r\n[ \t]/g, '').replace(/\n[ \t]/g, '');
  const get = (prop: string): string => {
    const m = text.match(new RegExp(`(?:^|\\n)${prop}(?:;[^\\n:]*)?:([^\\n]*)`, 'im'));
    return m ? m[1].trim() : '';
  };
  return {
    summary: get('SUMMARY'),
    description: get('DESCRIPTION'),
    due: get('DUE'),
    status: get('STATUS') || 'NEEDS-ACTION',
  };
}

export interface CreateTodoRequest {
  title: string;
  due?: Date;
  calendarId: string;
}

export async function createCalendarTodo(req: CreateTodoRequest): Promise<void> {
  const uid = `${Date.now()}-${Math.random().toString(36).slice(2)}@gogomail`;
  const objectName = `${uid}.ics`;
  const lines: string[] = [
    'BEGIN:VCALENDAR', 'VERSION:2.0', 'PRODID:-//GoGoMail//GoGoMail//EN',
    'BEGIN:VTODO',
    `UID:${uid}`,
    `SUMMARY:${icsEscape(req.title)}`,
    'STATUS:NEEDS-ACTION',
  ];
  if (req.due) lines.push(`DUE;VALUE=DATE:${toICSAllDay(req.due)}`);
  lines.push('END:VTODO', 'END:VCALENDAR');
  await request<unknown>(
    `calendars/${encodeURIComponent(req.calendarId)}/objects/${encodeURIComponent(objectName)}`,
    { method: 'PUT', headers: { 'Content-Type': 'text/calendar' }, body: lines.join('\r\n') },
  );
}

export async function setTodoStatus(calendarId: string, obj: CalendarObject, completed: boolean): Promise<void> {
  const f = parseVTODOICS(obj.ICS);
  const lines: string[] = [
    'BEGIN:VCALENDAR', 'VERSION:2.0', 'PRODID:-//GoGoMail//GoGoMail//EN',
    'BEGIN:VTODO',
    `UID:${obj.UID}`,
    `SUMMARY:${icsEscape(f.summary)}`,
    `STATUS:${completed ? 'COMPLETED' : 'NEEDS-ACTION'}`,
  ];
  if (f.due) lines.push(`DUE:${f.due}`);
  if (f.description) lines.push(`DESCRIPTION:${icsEscape(f.description)}`);
  lines.push('END:VTODO', 'END:VCALENDAR');
  await request<unknown>(
    `calendars/${encodeURIComponent(calendarId)}/objects/${encodeURIComponent(obj.ObjectName)}`,
    { method: 'PUT', headers: { 'Content-Type': 'text/calendar' }, body: lines.join('\r\n') },
  );
}

export async function deleteCalendarObject(calendarId: string, objectName: string): Promise<void> {
  await request<unknown>(
    `calendars/${encodeURIComponent(calendarId)}/objects/${encodeURIComponent(objectName)}`,
    { method: 'DELETE' },
  );
}

// ── Calendar Subscriptions ────────────────────────────────────────────────────

export interface CalendarSubscription {
  id: string;
  name: string;
  url: string;
  color: string;
}

export async function listCalendarSubscriptions(): Promise<CalendarSubscription[]> {
  try {
    const data = await request<{ subscriptions?: CalendarSubscription[] }>('calendar-subscriptions');
    return data.subscriptions ?? [];
  } catch { return []; }
}

export async function addCalendarSubscription(
  url: string, name: string, color: string,
): Promise<CalendarSubscription> {
  const data = await request<{ subscription: CalendarSubscription }>('calendar-subscriptions', {
    method: 'POST',
    body: JSON.stringify({ url, name, color }),
  });
  return data.subscription;
}

export async function deleteCalendarSubscription(id: string): Promise<void> {
  await request<unknown>(`calendar-subscriptions/${encodeURIComponent(id)}`, { method: 'DELETE' });
}

export async function fetchSubscriptionICS(id: string): Promise<string> {
  const res = await fetch(`/api/mail/calendar-subscriptions/${encodeURIComponent(id)}/events`);
  if (!res.ok) throw new Error(`Failed to fetch subscription events: ${res.status}`);
  return res.text();
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
    const res = await fetch(`/api/mail/directory/users?${params}`);
    if (!res.ok) return [];
    const data = await res.json() as { users?: DirectoryUser[] };
    return data.users ?? [];
  } catch { return []; }
}

export interface OrgMember {
  id: string;
  display_name: string;
  email: string;
}

export interface OrgUnit {
  id: string;
  display_name: string;
  parent_id?: string;
  depth: number;
  members: OrgMember[];
}

export async function listOrgTree(): Promise<OrgUnit[]> {
  try {
    const res = await fetch('/api/mail/directory/org-tree');
    if (!res.ok) return [];
    const data = await res.json() as { units?: OrgUnit[] };
    return data.units ?? [];
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
    const p = new URLSearchParams();
    if (params.folder_id) p.set('folder_id', params.folder_id);
    if (params.starred !== undefined) p.set('starred', String(params.starred));
    if (params.read !== undefined) p.set('read', String(params.read));
    if (params.limit !== undefined) p.set('limit', String(params.limit));
    if (params.cursor) p.set('cursor', params.cursor);
    const res = await fetch(`/api/mail/threads?${p}`);
    if (!res.ok) return { threads: [], has_more: false, next_cursor: '' };
    return res.json() as Promise<{ threads: ThreadSummary[]; has_more: boolean; next_cursor: string }>;
  } catch { return { threads: [], has_more: false, next_cursor: '' }; }
}

export async function listThreadMessages(threadId: string): Promise<MessageSummary[]> {
  try {
    const res = await fetch(`/api/mail/threads/${encodeURIComponent(threadId)}/messages?limit=50`);
    if (!res.ok) return [];
    const data = await res.json() as { messages?: MessageSummary[] };
    return data.messages ?? [];
  } catch { return []; }
}

// ─── Drive ───────────────────────────────────────────────────────────────────

export interface DriveNode {
  id: string;
  node_type: 'file' | 'folder';
  name: string;
  mime_type?: string;
  size: number;
  status: string;
  parent_id?: string;
  created_at: string;
  updated_at: string;
}

export interface DriveUsage {
  quota_used: number;
  quota_limit: number;
  active_bytes: number;
  trashed_bytes: number;
  folder_count: number;
  file_count: number;
}

export interface DriveShareLink {
  id: string;
  node_id: string;
  token_suffix: string;
  expires_at: string;
}

export async function listDriveNodes(parentId?: string): Promise<DriveNode[]> {
  try {
    const p = new URLSearchParams({ status: 'active' });
    if (parentId) p.set('parent_id', parentId);
    const res = await fetch(`/api/mail/drive/nodes?${p}`);
    if (!res.ok) return [];
    const data = await res.json() as { drive_nodes?: DriveNode[] };
    return data.drive_nodes ?? [];
  } catch { return []; }
}

export async function listTrashedDriveNodes(): Promise<DriveNode[]> {
  try {
    const p = new URLSearchParams({ status: 'trashed' });
    const res = await fetch(`/api/mail/drive/nodes?${p}`);
    if (!res.ok) return [];
    const data = await res.json() as { drive_nodes?: DriveNode[] };
    return data.drive_nodes ?? [];
  } catch { return []; }
}

export async function deleteDriveNodePermanently(nodeId: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}`, {
      method: 'DELETE',
    });
    return res.ok;
  } catch { return false; }
}

export async function getDriveUsage(): Promise<DriveUsage | null> {
  try {
    const res = await fetch('/api/mail/drive/usage');
    if (!res.ok) return null;
    const data = await res.json() as { usage?: DriveUsage };
    return data.usage ?? null;
  } catch { return null; }
}

export async function createDriveFolder(name: string, parentId?: string): Promise<DriveNode | null> {
  try {
    const res = await fetch('/api/mail/drive/folders', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name, parent_id: parentId ?? '' }),
    });
    if (!res.ok) return null;
    const data = await res.json() as { drive_node?: DriveNode };
    return data.drive_node ?? null;
  } catch { return null; }
}

export async function renameDriveNode(nodeId: string, name: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/name`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name }),
    });
    return res.ok;
  } catch { return false; }
}

export async function trashDriveNode(nodeId: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/trash`, {
      method: 'POST',
    });
    return res.ok;
  } catch { return false; }
}

export async function restoreDriveNode(nodeId: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/restore`, {
      method: 'POST',
    });
    return res.ok;
  } catch { return false; }
}

export async function downloadDriveNode(nodeId: string, filename: string): Promise<void> {
  const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/download`);
  if (!res.ok) throw new Error('Download failed');
  const blob = await res.blob();
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url; a.download = filename; a.click();
  URL.revokeObjectURL(url);
}

export async function uploadDriveFile(file: File, parentId?: string): Promise<DriveNode | null> {
  try {
    const sessionRes = await fetch('/api/mail/drive/upload-sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: file.name, mime_type: file.type || 'application/octet-stream', size: file.size, parent_id: parentId ?? '' }),
    });
    if (!sessionRes.ok) return null;
    const { upload_session } = await sessionRes.json() as { upload_session: { id: string } };
    const bodyRes = await fetch(`/api/mail/drive/upload-sessions/${upload_session.id}/body`, {
      method: 'PUT',
      headers: { 'Content-Type': file.type || 'application/octet-stream' },
      body: file,
    });
    if (!bodyRes.ok) return null;
    const finalRes = await fetch('/api/mail/drive/files/finalize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ upload_session_id: upload_session.id }),
    });
    if (!finalRes.ok) return null;
    const data = await finalRes.json() as { drive_node?: DriveNode };
    return data.drive_node ?? null;
  } catch { return null; }
}

export async function createDriveShareLink(nodeId: string, expiresAt: string): Promise<DriveShareLink | null> {
  try {
    const res = await fetch(`/api/mail/drive/nodes/${encodeURIComponent(nodeId)}/share-links`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ expires_at: expiresAt }),
    });
    if (!res.ok) return null;
    const data = await res.json() as { drive_share_link?: DriveShareLink };
    return data.drive_share_link ?? null;
  } catch { return null; }
}

export async function listDriveShareLinks(nodeId: string): Promise<DriveShareLink[]> {
  try {
    const res = await fetch(`/api/mail/drive/share-links?node_id=${encodeURIComponent(nodeId)}`);
    if (!res.ok) return [];
    const data = await res.json() as { drive_share_links?: DriveShareLink[] };
    return data.drive_share_links ?? [];
  } catch { return []; }
}

export async function revokeDriveShareLink(linkId: string): Promise<boolean> {
  try {
    const res = await fetch(`/api/mail/drive/share-links/${encodeURIComponent(linkId)}`, {
      method: 'DELETE',
    });
    return res.ok;
  } catch { return false; }
}

// ─── User profile + password ──────────────────────────────────────────────────

export interface UserProfile {
  user_id: string;
  display_name: string;
  email: string;
  quota_used: number;
  quota_limit: number | null;
}

export async function getUserProfile(): Promise<UserProfile | null> {
  try {
    const res = await fetch('/api/mail/me');
    if (!res.ok) return null;
    const data = await res.json() as { user?: UserProfile };
    return data.user ?? null;
  } catch { return null; }
}

export interface UserAddressEntry {
  id: string;
  address: string;
  is_primary: boolean;
}

export async function listUserAddresses(): Promise<UserAddressEntry[]> {
  try {
    const res = await fetch('/api/mail/me/addresses');
    if (!res.ok) return [];
    const data = await res.json() as { addresses?: UserAddressEntry[] };
    return data.addresses ?? [];
  } catch { return []; }
}

export async function updateUserProfile(fields: { display_name?: string }): Promise<void> {
  const res = await fetch('/api/mail/me', {
    method: 'PATCH',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(fields),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({})) as { error?: string };
    throw new Error(data.error ?? '프로필 업데이트에 실패했습니다.');
  }
}

export async function changePassword(currentPassword: string, newPassword: string): Promise<void> {
  const res = await fetch('/api/mail/me/password', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ current_password: currentPassword, new_password: newPassword }),
  });
  if (!res.ok) {
    const data = await res.json().catch(() => ({})) as { error?: string };
    throw new Error(data.error ?? '비밀번호 변경에 실패했습니다.');
  }
}

// ─── Server-side user preferences ────────────────────────────────────────────

export interface WebmailPreferences {
  settings?: Record<string, unknown>;
  filter_rules?: unknown[];
  blocked_senders?: string[];
  vacation?: Record<string, unknown>;
  signatures?: Record<string, string>;
}

export async function getPreferences(): Promise<WebmailPreferences> {
  try {
    const res = await fetch('/api/mail/preferences');
    if (!res.ok) return {};
    const data = await res.json() as { preferences?: WebmailPreferences };
    return data.preferences ?? {};
  } catch { return {}; }
}

export async function setPreferences(prefs: WebmailPreferences): Promise<WebmailPreferences> {
  try {
    const res = await fetch('/api/mail/preferences', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(prefs),
    });
    if (!res.ok) return prefs;
    const data = await res.json() as { preferences?: WebmailPreferences };
    return data.preferences ?? prefs;
  } catch { return prefs; }
}
