import type { CSSProperties } from 'react';
import { type AppId } from '@/components/AppIconBar';
import { type RefreshIntervalSeconds } from '@/hooks/useMailList';
import { focusNavGroup } from '@/lib/navKeyboard';

export const WEBMAIL_ACTIVE_APP_KEY = 'webmail_active_app';
export const NOTIFICATION_FOLDER_OVERRIDES_KEY = 'webmail_notification_folder_overrides';
export const NOTIFICATION_THREAD_OVERRIDES_KEY = 'webmail_notification_thread_overrides';
export const BADGE_COUNT_MODE_KEY = 'webmail_badge_count_mode';
export const REFRESH_INTERVAL_KEY = 'webmail_refresh_interval';
export const DM_MODAL_MIN_WIDTH = 320;
export const DM_MODAL_MIN_HEIGHT = 360;
export const DM_MODAL_DEFAULT_WIDTH = 480;
export const DM_MODAL_DEFAULT_HEIGHT = 680;
export const DM_MODAL_MARGIN = 12;
export type DMModalRect = { left: number; top: number; width: number; height: number };
export type DMResizeEdge = 'n' | 's' | 'e' | 'w' | 'ne' | 'nw' | 'se' | 'sw';
export type BadgeCountMode = 'unread' | 'all' | 'none';
export type NavigatorWithBadging = Navigator & {
  setAppBadge?: (contents?: number) => Promise<void>;
  clearAppBadge?: () => Promise<void>;
};

export const DM_RESIZE_HANDLES: Array<{ edge: DMResizeEdge; cursor: string; style: CSSProperties }> = [
  { edge: 'n', cursor: 'ns-resize', style: { top: -5, left: 10, right: 10, height: 10 } },
  { edge: 's', cursor: 'ns-resize', style: { bottom: -5, left: 10, right: 10, height: 10 } },
  { edge: 'e', cursor: 'ew-resize', style: { top: 10, right: -5, bottom: 10, width: 10 } },
  { edge: 'w', cursor: 'ew-resize', style: { top: 10, left: -5, bottom: 10, width: 10 } },
  { edge: 'ne', cursor: 'nesw-resize', style: { top: -7, right: -7, width: 18, height: 18 } },
  { edge: 'nw', cursor: 'nwse-resize', style: { top: -7, left: -7, width: 18, height: 18 } },
  { edge: 'se', cursor: 'nwse-resize', style: { bottom: -7, right: -7, width: 18, height: 18 } },
  { edge: 'sw', cursor: 'nesw-resize', style: { bottom: -7, left: -7, width: 18, height: 18 } },
];

export function getDefaultDMModalRect(): DMModalRect {
  if (typeof window === 'undefined') return { left: 56, top: 48, width: DM_MODAL_DEFAULT_WIDTH, height: DM_MODAL_DEFAULT_HEIGHT };
  const maxWidth = Math.max(DM_MODAL_MIN_WIDTH, window.innerWidth - 80);
  const maxHeight = Math.max(DM_MODAL_MIN_HEIGHT, window.innerHeight - 48);
  const width = Math.min(DM_MODAL_DEFAULT_WIDTH, maxWidth);
  const height = Math.min(DM_MODAL_DEFAULT_HEIGHT, maxHeight);
  return {
    left: Math.max(DM_MODAL_MARGIN, 56),
    top: Math.max(DM_MODAL_MARGIN, window.innerHeight - 24 - height),
    width,
    height,
  };
}

export function isAppId(value: string | null): value is AppId {
  return value === 'mail' || value === 'calendar' || value === 'contacts' || value === 'drive' || value === 'settings';
}

export function getInitialActiveApp(): AppId {
  if (typeof window === 'undefined') return 'mail';
  try {
    const urlApp = new URLSearchParams(window.location.search).get('app');
    if (isAppId(urlApp)) return urlApp;
    const stored = window.localStorage.getItem(WEBMAIL_ACTIVE_APP_KEY);
    if (isAppId(stored)) return stored;
  } catch {
    // ignore
  }
  return 'mail';
}

export function folderNotificationsEnabled(folderId: string): boolean {
  if (!folderId || typeof window === 'undefined') return true;
  try {
    const raw = window.localStorage.getItem(NOTIFICATION_FOLDER_OVERRIDES_KEY);
    if (!raw) return true;
    const overrides = JSON.parse(raw) as Record<string, { enabled?: boolean }>;
    return overrides[folderId]?.enabled !== false;
  } catch {
    return true;
  }
}

export function threadNotificationsEnabled(threadId: string | undefined, messageId: string): boolean {
  if (typeof window === 'undefined') return true;
  const key = threadId || messageId;
  if (!key) return true;
  try {
    const raw = window.localStorage.getItem(NOTIFICATION_THREAD_OVERRIDES_KEY);
    if (!raw) return true;
    const overrides = JSON.parse(raw) as Record<string, { enabled?: boolean }>;
    return overrides[key]?.enabled !== false;
  } catch {
    return true;
  }
}

export function readBadgeCountMode(): BadgeCountMode {
  if (typeof window === 'undefined') return 'unread';
  try {
    const value = window.localStorage.getItem(BADGE_COUNT_MODE_KEY);
    return value === 'all' || value === 'none' ? value : 'unread';
  } catch {
    return 'unread';
  }
}

export function readRefreshIntervalSeconds(): RefreshIntervalSeconds {
  if (typeof window === 'undefined') return 30;
  try {
    const value = Number(window.localStorage.getItem(REFRESH_INTERVAL_KEY) ?? 30);
    return value === 60 || value === 300 ? value : 30;
  } catch {
    return 30;
  }
}

export function getFocusedNavGroup(): string | null {
  if (typeof document === 'undefined') return null;
  const active = document.activeElement as HTMLElement | null;
  return active?.closest<HTMLElement>('[data-nav-group]')?.dataset.navGroup ?? null;
}

export function getMailNavGroups(readingPaneOpen: boolean): string[] {
  return readingPaneOpen ? ['sidebar-nav', 'message-list', 'reading-pane'] : ['sidebar-nav', 'message-list'];
}

export function moveMailPanelFocus(direction: 'prev' | 'next', readingPaneOpen: boolean): boolean {
  const groups = getMailNavGroups(readingPaneOpen);
  const currentGroup = getFocusedNavGroup();
  const currentIndex = currentGroup ? groups.indexOf(currentGroup) : -1;
  const fallbackIndex = direction === 'next' ? 0 : groups.length - 1;
  const nextIndex = currentIndex === -1
    ? fallbackIndex
    : Math.max(0, Math.min(groups.length - 1, currentIndex + (direction === 'next' ? 1 : -1)));
  return !!focusNavGroup(groups[nextIndex]);
}
