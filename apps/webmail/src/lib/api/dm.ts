import { request, apiGet, apiPost, apiPatch, apiDelete, clearTokenAndRedirect, responseErrorMessage } from './http';
import type { Attachment } from './types';

export type { Attachment };

export interface DMUser {
  id: string;
  company_id?: string;
  domain_id?: string;
  display_name: string;
  email?: string;
  avatar_url?: string;
}

export interface DMReaction {
  emoji: string;
  count: number;
  mine?: boolean;
}

export interface DMMessage {
  id: string;
  room_id: string;
  sender_id?: string;
  message_type: 'text' | 'file' | 'drive_link' | 'system';
  body: string;
  attachment_name?: string;
  attachment_size?: number;
  attachment_mime_type?: string;
  attachment_download_url?: string;
  drive_file_id?: string;
  created_at: string;
  edited_at?: string;
  deleted_at?: string;
  reactions?: DMReaction[];
  read_count?: number;
}

export interface DMRoom {
  id: string;
  company_id: string;
  domain_id: string;
  room_type: 'direct' | 'group';
  visibility?: 'public' | 'private';
  name?: string;
  owner_id?: string;
  created_by: string;
  created_at: string;
  members?: DMUser[];
  unread_count?: number;
  last_message?: DMMessage;
  member_count?: number;
  last_read_message_id?: string;
  current_user_id?: string;
}

export interface DMInvite {
  token: string;
  room_id: string;
  expires_at: string;
}

export interface DMMediaItem {
  message_id: string;
  message_type: string;
  sender_id?: string;
  url?: string;
  attachment_name?: string;
  attachment_size?: number;
  attachment_mime_type?: string;
  download_url?: string;
  drive_file_id?: string;
  drive_name?: string;
  created_at: string;
}

export interface DMSearchResult {
  message: DMMessage;
  before?: string;
  after?: string;
}

function normalizeDMAttachmentDownloadURL(raw?: string): string | undefined {
  if (!raw) return raw;
  if (raw.startsWith('data:') || raw.startsWith('blob:') || raw.startsWith('/api/mail/')) return raw;
  if (raw.startsWith('/api/v1/dm/')) return `/api/mail/${raw.slice('/api/v1/'.length)}`;
  try {
    const url = new URL(raw);
    if (url.pathname.startsWith('/api/v1/dm/')) {
      return `/api/mail/${url.pathname.slice('/api/v1/'.length)}${url.search}${url.hash}`;
    }
  } catch {
    return raw;
  }
  return raw;
}

function normalizeDMMessage(message: DMMessage): DMMessage {
  return {
    ...message,
    attachment_download_url: normalizeDMAttachmentDownloadURL(message.attachment_download_url),
  };
}

function normalizeDMRoom(room: DMRoom): DMRoom {
  return {
    ...room,
    last_message: room.last_message ? normalizeDMMessage(room.last_message) : room.last_message,
  };
}

function normalizeDMMediaItem(item: DMMediaItem): DMMediaItem {
  return {
    ...item,
    download_url: normalizeDMAttachmentDownloadURL(item.download_url),
  };
}

export function listDMRooms(): Promise<DMRoom[]> {
  return apiGet<{ rooms: DMRoom[] }>('dm/rooms').then((r) => (r.rooms ?? []).map(normalizeDMRoom));
}

export function listPublicDMRooms(): Promise<DMRoom[]> {
  return apiGet<{ rooms: DMRoom[] }>('dm/rooms/public').then((r) => (r.rooms ?? []).map(normalizeDMRoom));
}

export function createDMRoom(input: {
  room_type: 'direct' | 'group';
  user_ids: string[];
  name?: string;
  visibility?: 'public' | 'private';
}): Promise<DMRoom> {
  return apiPost<{ room: DMRoom }>('dm/rooms', input).then((r) => normalizeDMRoom(r.room));
}

export function addDMMembers(roomId: string, userIds: string[]): Promise<DMMessage[]> {
  return apiPost<{ messages: DMMessage[] }>(`dm/rooms/${encodeURIComponent(roomId)}/members`, { user_ids: userIds })
    .then((r) => (r.messages ?? []).map(normalizeDMMessage));
}

export function removeDMMember(roomId: string, userId: string): Promise<{ deleted_room: boolean; system_message?: DMMessage }> {
  return apiDelete<{ removal: { deleted_room: boolean; system_message?: DMMessage } }>(
    `dm/rooms/${encodeURIComponent(roomId)}/members/${encodeURIComponent(userId)}`
  ).then((r) => r.removal);
}

export function transferDMOwner(roomId: string, userId: string): Promise<DMMessage> {
  return apiPatch<{ message: DMMessage }>(`dm/rooms/${encodeURIComponent(roomId)}/owner`, { user_id: userId })
    .then((r) => normalizeDMMessage(r.message));
}

export function createDMInvite(roomId: string): Promise<{ invite: DMInvite; invite_url: string }> {
  return apiPost<{ invite: DMInvite; invite_url: string }>(`dm/rooms/${encodeURIComponent(roomId)}/invites`);
}

export function joinDMInvite(token: string): Promise<DMMessage> {
  return apiPost<{ message: DMMessage }>(`dm/join/${encodeURIComponent(token)}`).then((r) => normalizeDMMessage(r.message));
}

export function listDMMessages(roomId: string, params: { before?: string; after?: string; limit?: number } = {}): Promise<DMMessage[]> {
  const search: Record<string, string> = {};
  if (params.before) search.before = params.before;
  if (params.after) search.after = params.after;
  if (params.limit) search.limit = String(params.limit);
  return apiGet<{ messages: DMMessage[] }>(`dm/rooms/${encodeURIComponent(roomId)}/messages`, search).then((r) => (r.messages ?? []).map(normalizeDMMessage));
}

export function sendDMMessage(roomId: string, body: string, driveFileId?: string): Promise<DMMessage> {
  return apiPost<{ message: DMMessage }>(`dm/rooms/${encodeURIComponent(roomId)}/messages`, {
    body,
    ...(driveFileId ? { drive_file_id: driveFileId } : {}),
  }).then((r) => normalizeDMMessage(r.message));
}

export async function uploadDMAttachment(roomId: string, file: File): Promise<DMMessage> {
  const form = new FormData();
  form.append('file', file);
  const res = await fetch(`/api/mail/dm/rooms/${encodeURIComponent(roomId)}/attachments`, {
    method: 'POST',
    body: form,
  });
  if (res.status === 401) { clearTokenAndRedirect(); throw new Error('Unauthorized'); }
  if (!res.ok) {
    throw new Error(await responseErrorMessage(res, `Upload failed: ${res.status}`));
  }
  const data = await res.json() as { message: DMMessage };
  return normalizeDMMessage(data.message);
}

export function markDMRead(roomId: string, lastMessageId: string): Promise<void> {
  return apiPost<void>(`dm/rooms/${encodeURIComponent(roomId)}/read`, { last_message_id: lastMessageId });
}

export function searchDMMessages(roomId: string, q: string, before?: string, limit = 20): Promise<DMSearchResult[]> {
  const params: Record<string, string> = { q, limit: String(limit) };
  if (before) params.before = before;
  return apiGet<{ results: DMSearchResult[] }>(`dm/rooms/${encodeURIComponent(roomId)}/search`, params)
    .then((r) => (r.results ?? []).map((result) => ({ ...result, message: normalizeDMMessage(result.message) })));
}

export function listDMMedia(roomId: string, type: 'files' | 'links' | 'drive' = 'files', before?: string, limit = 30): Promise<DMMediaItem[]> {
  const params: Record<string, string> = { type, limit: String(limit) };
  if (before) params.before = before;
  return apiGet<{ media: DMMediaItem[] }>(`dm/rooms/${encodeURIComponent(roomId)}/media`, params).then((r) => (r.media ?? []).map(normalizeDMMediaItem));
}

export function editDMMessage(messageId: string, body: string): Promise<DMMessage> {
  return apiPatch<{ message: DMMessage }>(`dm/messages/${encodeURIComponent(messageId)}`, { body }).then((r) => normalizeDMMessage(r.message));
}

export function deleteDMMessage(messageId: string): Promise<DMMessage> {
  return apiDelete<{ message: DMMessage }>(`dm/messages/${encodeURIComponent(messageId)}`).then((r) => normalizeDMMessage(r.message));
}

export function toggleDMReaction(messageId: string, emoji: string): Promise<void> {
  return request<void>(`dm/messages/${encodeURIComponent(messageId)}/reactions`, {
    method: 'PUT',
    body: JSON.stringify({ emoji }),
  });
}

export async function exportDMRoom(roomId: string, timezone?: string): Promise<{ blob: Blob; filename: string }> {
  const tz = timezone ?? (() => { try { return Intl.DateTimeFormat().resolvedOptions().timeZone; } catch { return 'UTC'; } })();
  const url = `/api/mail/dm/rooms/${encodeURIComponent(roomId)}/export?timezone=${encodeURIComponent(tz)}`;
  const res = await fetch(url, {
    signal: AbortSignal.timeout(60_000),
  });
  if (res.status === 401) {
    clearTokenAndRedirect();
    throw new Error('Unauthorized');
  }
  if (!res.ok) {
    throw new Error(await responseErrorMessage(res, `Export failed: ${res.status}`));
  }
  // Extract filename from Content-Disposition before consuming the body as a blob.
  // blob: URLs don't preserve original response headers, so a.download = '' would
  // fall back to the blob URL (which contains the room UUID as the path).
  const cd = res.headers.get('Content-Disposition') ?? '';
  const match = cd.match(/filename[^;=\n]*=(['"]?)([^'"\n;]+)\1/);
  const filename = match?.[2]?.trim() || `dm-export.txt`;
  const blob = await res.blob();
  return { blob, filename };
}
