'use client';

import type { DMMessage, DMRoom, DMUser, DirectoryUser } from '@/lib/api';

export type MediaTab = 'files' | 'links' | 'drive';
export type DMDraft = { body: string; driveFileId: string };

export const DEV_CURRENT_USER_ID = process.env.NEXT_PUBLIC_GOGOMAIL_DEV_USER_ID ?? '';
export const DM_DRAFT_STORAGE_KEY = 'webmail_dm_drafts_v1';

export function matchesDirectoryUser(user: DirectoryUser, query: string): boolean {
  if (!query) return true;
  const needle = query.toLowerCase();
  return [user.display_name, user.email, user.org_unit_name]
    .some((value) => (value ?? '').toLowerCase().includes(needle));
}

export function messagePreview(message: DMMessage | undefined, labels: { deleted: string; file: string; drive: string }): string {
  if (!message) return '';
  if (message.deleted_at) return labels.deleted;
  if (message.message_type === 'file') return message.attachment_name || message.body || labels.file;
  if (message.message_type === 'drive_link') return message.body || message.drive_file_id || labels.drive;
  return message.body;
}

export function roomTitle(
  room: DMRoom,
  currentUserId: string,
  labels: { direct: string; group: string; groupOthers: (name: string, count: number) => string },
): string {
  const otherNames = (room.members ?? [])
    .filter((member) => member.id !== currentUserId)
    .map((member) => member.display_name || member.id)
    .filter(Boolean);
  if (room.room_type === 'direct') return otherNames[0] || room.name?.trim() || labels.direct;
  if (room.name?.trim()) return room.name;
  if (otherNames.length > 1) return labels.groupOthers(otherNames[0], otherNames.length - 1);
  return otherNames[0] || labels.group;
}

export function mergeMessage(existing: DMMessage[], next: DMMessage): DMMessage[] {
  const index = existing.findIndex((m) => m.id === next.id);
  if (index === -1) return [...existing, next].sort((a, b) => Date.parse(a.created_at) - Date.parse(b.created_at));
  const merged = [...existing];
  merged[index] = next;
  return merged;
}

export function readDMDrafts(): Record<string, DMDraft> {
  try {
    if (typeof window === 'undefined') return {};
    const parsed = JSON.parse(localStorage.getItem(DM_DRAFT_STORAGE_KEY) ?? '{}') as Record<string, DMDraft>;
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch { return {}; }
}

export function writeDMDrafts(drafts: Record<string, DMDraft>) {
  try {
    if (typeof window === 'undefined') return;
    localStorage.setItem(DM_DRAFT_STORAGE_KEY, JSON.stringify(drafts));
  } catch { /* best-effort */ }
}

// Re-export types used by sub-hooks for convenience
export type { DMMessage, DMRoom, DMUser, DirectoryUser };
