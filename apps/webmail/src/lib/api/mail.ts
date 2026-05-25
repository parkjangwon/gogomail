import { request, apiGet, apiPost, apiPatch, apiDelete, clearTokenAndRedirect, responseErrorMessage } from './http';
import type {
  Folder,
  MessageAddress,
  MessageSummary,
  MessageDetail,
  Attachment,
  ComposeIntent,
  ThreadSummary,
} from './types';
// Cross-domain bridge: these two functions straddle mail and drive.
// They live here because compose owns the UX flow; drive provides storage.
import type { DriveNode, DriveUploadCapabilities } from './drive';
import { uploadDriveFile } from './drive';

export type { Folder, MessageAddress, MessageSummary, MessageDetail, Attachment, ComposeIntent, ThreadSummary };

// ─── Address normalization helpers ────────────────────────────────────────────

type ComposeAddressLike = {
  address?: string;
  email?: string;
  name?: string;
};

type BackendComposeAddress = {
  email: string;
  name?: string;
};

function normalizeComposeAddress(address: ComposeAddressLike): BackendComposeAddress {
  return {
    email: address.email ?? address.address ?? '',
    ...(address.name ? { name: address.name } : {}),
  };
}

function normalizeComposeAddresses(addresses?: ComposeAddressLike[]): BackendComposeAddress[] | undefined {
  if (!addresses) return undefined;
  return addresses
    .map((address) => normalizeComposeAddress(address))
    .filter((address) => address.email.trim() !== '');
}

function normalizeSendRequestPayload<T extends { to: ComposeAddressLike[]; cc?: ComposeAddressLike[]; bcc?: ComposeAddressLike[] }>(
  payload: T
): Omit<T, 'to' | 'cc' | 'bcc'> & { to: BackendComposeAddress[]; cc?: BackendComposeAddress[]; bcc?: BackendComposeAddress[] } {
  return {
    ...payload,
    to: normalizeComposeAddresses(payload.to) ?? [],
    ...(payload.cc ? { cc: normalizeComposeAddresses(payload.cc) } : {}),
    ...(payload.bcc ? { bcc: normalizeComposeAddresses(payload.bcc) } : {}),
  };
}

// ─── Core mail types ──────────────────────────────────────────────────────────

export interface MailSendRequest {
  to: ComposeAddressLike[];
  cc?: ComposeAddressLike[];
  bcc?: ComposeAddressLike[];
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

export interface SendMessageResult {
  id: string;
  message_id: string;
  farm: string;
  send_status: string;
  delivery_status: string;
  bounce_status: string;
}

export interface SendMessageEnvelope {
  message: SendMessageResult;
}

export interface TrackingEvent {
  recipient_email: string;
  opened_at: string | null;
  open_count: number;
}

export interface DraftData {
  draft_id?: string;
  intent: ComposeIntent;
  source_message_id?: string;
  attachment_ids?: string[];
  from?: string;
  to: ComposeAddressLike[];
  cc?: ComposeAddressLike[];
  bcc?: ComposeAddressLike[];
  subject: string;
  text_body: string;
  html_body?: string;
  track_opens?: boolean;
  scheduled_at?: string;
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

export interface GetMessagesOptions {
  starred?: boolean;
  read?: boolean;
  has_attachment?: boolean;
}

export interface FolderStats {
  id: string;
  name: string;
  system_type?: string;
  total: number;
  unread: number;
  starred: number;
  size_bytes: number;
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

// ─── Message operations ───────────────────────────────────────────────────────

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
  return apiGet<{ messages: MessageSummary[]; has_more: boolean; next_cursor: string }>('search', p);
}

export function getFolders(): Promise<{ folders: Folder[] }> {
  return apiGet<{ folders: Folder[] }>('folders');
}

export function getMessages(
  folderId: string,
  cursor = '',
  limit = 50,
  options: GetMessagesOptions = {}
): Promise<{ messages: MessageSummary[]; has_more: boolean; next_cursor: string }> {
  const params: Record<string, string> = {
    limit: String(limit),
  };
  const trimmedFolderId = folderId.trim();
  if (trimmedFolderId) params.folder_id = trimmedFolderId;
  if (cursor) params.cursor = cursor;
  if (options.starred !== undefined) params.starred = String(options.starred);
  if (options.read !== undefined) params.read = String(options.read);
  if (options.has_attachment !== undefined) params.has_attachment = String(options.has_attachment);
  return apiGet<{ messages: MessageSummary[]; has_more: boolean; next_cursor: string }>('messages', params);
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

export function deleteMessage(id: string): Promise<void> {
  return apiDelete<void>(`messages/${id}`);
}

export function restoreMessage(id: string): Promise<void> {
  return apiPost<void>(`messages/${id}/restore`, {});
}

export function bulkRestoreMessages(ids: string[]): Promise<void> {
  return apiPost<void>('messages/bulk/restore', { message_ids: ids });
}

export function sendMessage(data: MailSendRequest): Promise<SendMessageEnvelope> {
  return apiPost<SendMessageEnvelope>('messages/send', normalizeSendRequestPayload(data));
}

export async function getMessageTracking(messageId: string): Promise<TrackingEvent[]> {
  try {
    const data = await request<{ events?: TrackingEvent[] }>(`messages/${encodeURIComponent(messageId)}/tracking`);
    return data.events ?? [];
  } catch { return []; }
}

export async function getMessageDeliveryStatus(messageId: string): Promise<MessageDeliveryStatus | null> {
  try {
    const data = await request<{ delivery_status: MessageDeliveryStatus }>(`messages/${encodeURIComponent(messageId)}/delivery-status`);
    return data.delivery_status ?? null;
  } catch { return null; }
}

// ─── Folder operations ────────────────────────────────────────────────────────

export function createFolder(name: string): Promise<{ folder: Folder }> {
  return apiPost<{ folder: Folder }>('folders', { name });
}

export function renameFolder(id: string, name: string): Promise<{ folder: Folder }> {
  return apiPatch<{ folder: Folder }>(`folders/${id}`, { name });
}

export function deleteFolder(id: string): Promise<void> {
  return apiDelete<void>(`folders/${id}`);
}

// ─── Draft operations ─────────────────────────────────────────────────────────

export function saveDraft(data: DraftData): Promise<{ draft: { id: string } }> {
  return apiPost<{ draft: { id: string } }>('drafts', normalizeSendRequestPayload(data));
}

export function updateDraft(draftId: string, data: DraftData): Promise<{ draft: { id: string } }> {
  return apiPatch<{ draft: { id: string } }>(`drafts/${draftId}`, normalizeSendRequestPayload(data));
}

export function deleteDraft(draftId: string): Promise<void> {
  return apiDelete<void>(`drafts/${draftId}`);
}

export function sendDraft(draftId: string): Promise<SendMessageEnvelope> {
  return apiPost<SendMessageEnvelope>(`drafts/${draftId}/send`);
}

// ─── Attachments ──────────────────────────────────────────────────────────────

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


// ─── Storage / Backup / Restore ───────────────────────────────────────────────

export async function getFolderStats(): Promise<FolderStats[]> {
  const { folders } = await getFolders();
  return folders.map((f) => ({ ...f, size_bytes: f.total * 32 * 1024 }));
}

function formatEml(msg: MessageDetail): string {
  const date = new Date(msg.received_at ?? Date.now()).toUTCString();
  const to = msg.to_addrs.map((a) => {
    const email = a.email || a.address || '';
    return a.name ? `"${a.name}" <${email}>` : email;
  }).join(', ');
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

// ─── Thread operations ────────────────────────────────────────────────────────

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

export type { DriveUploadCapabilities };

// ─── Capabilities ─────────────────────────────────────────────────────────────

export interface WebmailCapabilitiesEnvelope {
  webmail_capabilities?: {
    drive?: DriveUploadCapabilities;
  };
}

export async function getWebmailCapabilities(): Promise<WebmailCapabilitiesEnvelope['webmail_capabilities'] | null> {
  try {
    const data = await request<WebmailCapabilitiesEnvelope>('webmail/capabilities');
    return data.webmail_capabilities ?? null;
  } catch {
    return null;
  }
}

// ─── Cross-domain: drive ↔ mail attachment helpers ────────────────────────────

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
