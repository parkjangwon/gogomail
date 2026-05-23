'use client';

/*
 * Notification center store.
 *
 * Future-feature integration:
 *
 *   1. import { useNotifications } from '@/lib/notifications/store';
 *   2. const { push } = useNotifications();
 *   3. push({
 *        category: 'drive_share',
 *        severity: 'info',
 *        title: 'Alice shared "Q4 plan"',
 *        body: 'Tap to open the file',
 *        actionUrl: '/drive/abc123',
 *      });
 *
 * Adding new categories: extend `NotificationCategory` in `./types.ts`.
 * Store, bell, and item components handle unknown categories gracefully.
 *
 * TODO(server-stream): a future task will add SSE/WebSocket subscription
 *   to a backend endpoint (planned shape):
 *     GET  /api/notifications/stream    text/event-stream
 *     POST /api/notifications/ack       { id }   // mark read server-side
 *   The handler will simply call `push(...)` per incoming event.
 */

import {
  createContext,
  createElement,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  type ReactNode,
} from 'react';
import { stableId } from '@/lib/stableId';
import type { Notification, NotificationCategory, NotificationInput, NotificationSeverity } from './types';

const STORAGE_KEY = 'webmail_notifications';
const BROWSER_NOTIF_ENABLED_KEY = 'webmail_browser_notifications_enabled';
const DND_ENABLED_KEY = 'webmail_dnd';
const DND_START_KEY = 'webmail_dnd_start';
const DND_END_KEY = 'webmail_dnd_end';
const NOTIF_SOUND_KEY = 'webmail_notif_sound';
const MAX_NOTIFICATIONS = 500;
const MAX_ID_LENGTH = 128;
const MAX_TITLE_LENGTH = 160;
const MAX_BODY_LENGTH = 500;
const MAX_ICON_NAME_LENGTH = 64;
const MAX_STORED_AGE_MS = 90 * 24 * 60 * 60 * 1000;
const MAX_STORED_FUTURE_SKEW_MS = 24 * 60 * 60 * 1000;
const MAX_METADATA_KEYS = 20;
const MAX_METADATA_KEY_LENGTH = 64;
const MAX_METADATA_STRING_LENGTH = 200;
const FALLBACK_TITLE = 'Notification';
const UNSAFE_ACTION_URL_CHARS = /[\u0000-\u001F\u007F\\]/;
const VALID_CATEGORIES = new Set<NotificationCategory>([
  'mail_received',
  'mail_sent',
  'mail_send_failed',
  'mail_bounced',
  'calendar_reminder',
  'calendar_invite',
  'drive_share',
  'system',
  'custom',
]);
const VALID_SEVERITIES = new Set<NotificationSeverity>(['info', 'success', 'warning', 'error']);

/**
 * Attempt to fire an OS-level browser Notification mirroring an in-app one.
 *
 * Gated by:
 *   - Notification API is available
 *   - permission === 'granted'
 *   - localStorage toggle (default true)
 *   - document.hidden OR severity is 'warning' / 'error' (errors always show)
 *
 * Safe to call from any environment — short-circuits on SSR / unsupported.
 */
function fireBrowserNotification(
  n: Notification,
  markAsRead: (id: string) => void,
): void {
  if (typeof window === 'undefined') return;
  // `Notification` (imported above) shadows the DOM type name; access the
  // browser API via window to avoid the collision.
  const NotificationCtor = (window as unknown as { Notification?: typeof window.Notification }).Notification;
  if (!NotificationCtor) return;
  try {
    if (NotificationCtor.permission !== 'granted') return;
    let enabled = true;
    try {
      const raw = window.localStorage.getItem(BROWSER_NOTIF_ENABLED_KEY);
      if (raw === 'false') enabled = false;
    } catch {
      // localStorage may be blocked; fall back to enabled=true
    }
    if (!enabled) return;
    if (isQuietHoursActive(new Date())) return;

    const severity = n.severity;
    const shouldShow =
      severity === 'warning' || severity === 'error' || (typeof document !== 'undefined' && document.hidden);
    if (!shouldShow) return;

    const browserNotif = new NotificationCtor(n.title, {
      body: n.body,
      tag: `${n.category}-${n.id}`,
      icon: '/favicon.ico',
      data: { actionUrl: n.actionUrl, id: n.id },
      silent: !isNotificationSoundEnabled(),
    });

    browserNotif.onclick = () => {
      try {
        window.focus();
        const data = browserNotif.data as { actionUrl?: string; id?: string } | undefined;
        if (data?.id) markAsRead(data.id);
        if (data?.actionUrl) {
          window.location.assign(data.actionUrl);
        }
        browserNotif.close();
      } catch {
        // ignore handler errors
      }
    };
  } catch (err) {
    // Some browsers reject construction in certain states (e.g. mobile Safari
    // requires Push API, focused-tab restrictions). Don't break the in-app
    // notification flow.
    // eslint-disable-next-line no-console
    console.warn('[notifications] failed to create browser Notification:', err);
  }
}

function parseHHMM(value: string | null): number | null {
  if (!value) return null;
  const match = /^(\d{2}):(\d{2})$/.exec(value);
  if (!match) return null;
  const hours = Number(match[1]);
  const minutes = Number(match[2]);
  if (!Number.isInteger(hours) || !Number.isInteger(minutes) || hours > 23 || minutes > 59) return null;
  return hours * 60 + minutes;
}

function isQuietHoursActive(now: Date): boolean {
  if (typeof window === 'undefined') return false;
  try {
    if (window.localStorage.getItem(DND_ENABLED_KEY) !== '1') return false;
    const start = parseHHMM(window.localStorage.getItem(DND_START_KEY)) ?? 22 * 60;
    const end = parseHHMM(window.localStorage.getItem(DND_END_KEY)) ?? 8 * 60;
    const current = now.getHours() * 60 + now.getMinutes();
    if (start === end) return true;
    if (start < end) return current >= start && current <= end;
    return current >= start || current <= end;
  } catch {
    return false;
  }
}

function isNotificationSoundEnabled(): boolean {
  if (typeof window === 'undefined') return false;
  try {
    return window.localStorage.getItem(NOTIF_SOUND_KEY) === '1';
  } catch {
    return false;
  }
}

function playNotificationSound(): void {
  if (typeof window === 'undefined') return;
  if (!isNotificationSoundEnabled()) return;
  if (isQuietHoursActive(new Date())) return;
  try {
    const AudioContextCtor = window.AudioContext
      ?? (window as unknown as { webkitAudioContext?: typeof AudioContext }).webkitAudioContext;
    if (!AudioContextCtor) return;
    const ctx = new AudioContextCtor();
    const oscillator = ctx.createOscillator();
    const gain = ctx.createGain();
    oscillator.type = 'sine';
    oscillator.frequency.setValueAtTime(880, ctx.currentTime);
    gain.gain.setValueAtTime(0.0001, ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.08, ctx.currentTime + 0.015);
    gain.gain.exponentialRampToValueAtTime(0.0001, ctx.currentTime + 0.18);
    oscillator.connect(gain);
    gain.connect(ctx.destination);
    oscillator.start();
    oscillator.stop(ctx.currentTime + 0.2);
    window.setTimeout(() => {
      void ctx.close().catch(() => {});
    }, 300);
  } catch {
    // Browser autoplay policy or unsupported audio contexts must not break notifications.
  }
}

type State = { notifications: Notification[] };

type Action =
  | { type: 'hydrate'; notifications: Notification[] }
  | { type: 'push'; notification: Notification; dedupe?: boolean }
  | { type: 'markRead'; id: string }
  | { type: 'markAllRead' }
  | { type: 'dismiss'; id: string }
  | { type: 'clearAll' };

function reducer(state: State, action: Action): State {
  switch (action.type) {
    case 'hydrate':
      return { notifications: action.notifications };
    case 'push': {
      if (action.dedupe && state.notifications.some((n) => n.id === action.notification.id)) {
        return state;
      }
      // newest-first
      const next = [
        action.notification,
        ...state.notifications.filter((n) => n.id !== action.notification.id),
      ].slice(0, MAX_NOTIFICATIONS);
      return { notifications: next };
    }
    case 'markRead':
      return {
        notifications: state.notifications.map((n) => (n.id === action.id ? { ...n, read: true } : n)),
      };
    case 'markAllRead':
      return { notifications: state.notifications.map((n) => (n.read ? n : { ...n, read: true })) };
    case 'dismiss':
      return { notifications: state.notifications.filter((n) => n.id !== action.id) };
    case 'clearAll':
      return { notifications: [] };
    default:
      return state;
  }
}

function loadInitial(): Notification[] {
  if (typeof window === 'undefined') return [];
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    return sanitizeNotifications(JSON.parse(raw) as unknown);
  } catch {
    return [];
  }
}

function isSafeActionUrl(value: unknown): boolean {
  if (value === undefined) return true;
  if (typeof value !== 'string') return false;
  return value.startsWith('/')
    && !value.startsWith('//')
    && !UNSAFE_ACTION_URL_CHARS.test(value);
}

function safeActionUrl(value: unknown): string | undefined {
  return isSafeActionUrl(value) && typeof value === 'string' ? value : undefined;
}

function truncateText(value: string, maxLength: number): string {
  return value.length > maxLength ? value.slice(0, maxLength) : value;
}

function safeNotificationBody(value: unknown): string | undefined {
  return typeof value === 'string' ? truncateText(value, MAX_BODY_LENGTH) : undefined;
}

function safeIconName(value: unknown): string | undefined {
  return typeof value === 'string' && value.length <= MAX_ICON_NAME_LENGTH ? value : undefined;
}

function safeNotificationMetadata(value: unknown): Record<string, unknown> | undefined {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return undefined;
  const output: Record<string, string | number | boolean> = {};
  for (const [key, raw] of Object.entries(value as Record<string, unknown>)) {
    if (Object.keys(output).length >= MAX_METADATA_KEYS) break;
    if (!key || key.length > MAX_METADATA_KEY_LENGTH) continue;
    if (typeof raw === 'string') {
      output[key] = truncateText(raw, MAX_METADATA_STRING_LENGTH);
      continue;
    }
    if (typeof raw === 'number' && Number.isFinite(raw)) {
      output[key] = raw;
      continue;
    }
    if (typeof raw === 'boolean') {
      output[key] = raw;
    }
  }
  return Object.keys(output).length > 0 ? output : undefined;
}

function safeNotificationId(value: unknown): string {
  return typeof value === 'string' && value.trim() !== '' && value.length <= MAX_ID_LENGTH ? value : makeId();
}

function safeNotificationTitle(value: unknown): string {
  return typeof value === 'string' && value.trim() !== '' ? truncateText(value, MAX_TITLE_LENGTH) : FALLBACK_TITLE;
}

function safeNotificationCategory(value: unknown): NotificationCategory {
  return typeof value === 'string' && VALID_CATEGORIES.has(value as NotificationCategory)
    ? value as NotificationCategory
    : 'system';
}

function safeNotificationSeverity(value: unknown): NotificationSeverity {
  return typeof value === 'string' && VALID_SEVERITIES.has(value as NotificationSeverity)
    ? value as NotificationSeverity
    : 'info';
}

function isSafeStoredTimestamp(value: unknown, now: number): value is number {
  return typeof value === 'number'
    && Number.isFinite(value)
    && value >= now - MAX_STORED_AGE_MS
    && value <= now + MAX_STORED_FUTURE_SKEW_MS;
}

function sanitizeNotifications(input: unknown): Notification[] {
  if (!Array.isArray(input)) return [];
  const seen = new Set<string>();
  const sanitized: Notification[] = [];
  const now = Date.now();
  for (const n of input) {
    if (sanitized.length >= MAX_NOTIFICATIONS) break;
    if (!n || typeof n !== 'object') continue;
    const o = n as Record<string, unknown>;
    if (!(typeof o.id === 'string'
      && o.id.trim() !== ''
      && o.id.length <= MAX_ID_LENGTH
      && typeof o.title === 'string'
      && o.title.trim() !== ''
      && typeof o.category === 'string'
      && VALID_CATEGORIES.has(o.category as NotificationCategory)
      && typeof o.severity === 'string'
      && VALID_SEVERITIES.has(o.severity as NotificationSeverity)
      && (o.body === undefined || typeof o.body === 'string')
      && isSafeActionUrl(o.actionUrl)
      && isSafeStoredTimestamp(o.timestamp, now)
      && typeof o.read === 'boolean')) {
      continue;
    }
    if (seen.has(o.id)) continue;
    seen.add(o.id);
    sanitized.push({
      id: o.id,
      category: o.category as NotificationCategory,
      severity: o.severity as NotificationSeverity,
      title: safeNotificationTitle(o.title),
      body: safeNotificationBody(o.body),
      actionUrl: safeActionUrl(o.actionUrl),
      timestamp: o.timestamp,
      read: o.read,
    });
  }
  return sanitized;
}

function makeId(): string {
  return stableId('n');
}

function indexNotifications(notifications: Notification[]): Map<string, Notification> {
  return new Map(notifications.map((n) => [n.id, n]));
}

export interface NotificationsContextValue {
  notifications: Notification[];
  unreadCount: number;
  push: (input: NotificationInput) => Notification;
  markAsRead: (id: string) => void;
  markAllRead: () => void;
  dismiss: (id: string) => void;
  clearAll: () => void;
}

const NotificationsContext = createContext<NotificationsContextValue | null>(null);

export function NotificationProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reducer, undefined, () => ({ notifications: loadInitial() }));
  const notificationsByIDRef = useRef(indexNotifications(state.notifications));

  useEffect(() => {
    notificationsByIDRef.current = indexNotifications(state.notifications);
  }, [state.notifications]);

  // Persist to localStorage (debounce-free; the reducer is cheap and writes are small)
  useEffect(() => {
    try {
      window.localStorage.setItem(STORAGE_KEY, JSON.stringify(state.notifications));
    } catch {
      // localStorage may be unavailable or full — ignore
    }
  }, [state.notifications]);

  // Cross-tab sync
  useEffect(() => {
    if (typeof window === 'undefined') return;
    const onStorage = (e: StorageEvent) => {
      if (e.key !== STORAGE_KEY) return;
      try {
        const parsed = e.newValue ? JSON.parse(e.newValue) as unknown : [];
        const notifications = sanitizeNotifications(parsed);
        notificationsByIDRef.current = indexNotifications(notifications);
        dispatch({ type: 'hydrate', notifications });
      } catch {
        notificationsByIDRef.current = new Map();
        dispatch({ type: 'hydrate', notifications: [] });
      }
    };
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  const push = useCallback<NotificationsContextValue['push']>((input) => {
    const rawInput = input as Record<string, unknown>;
    const id = safeNotificationId(rawInput.id);
    const existing = notificationsByIDRef.current.get(id);
    if (input.dedupe && existing) {
      return existing;
    }
    const notification: Notification = {
      id,
      category: safeNotificationCategory(rawInput.category),
      severity: safeNotificationSeverity(rawInput.severity),
      title: safeNotificationTitle(rawInput.title),
      body: safeNotificationBody(rawInput.body),
      timestamp: Date.now(),
      read: false,
      actionUrl: safeActionUrl(rawInput.actionUrl),
      iconName: safeIconName(rawInput.iconName),
      metadata: safeNotificationMetadata(rawInput.metadata),
    };
    notificationsByIDRef.current = indexNotifications(
      [notification, ...Array.from(notificationsByIDRef.current.values()).filter((n) => n.id !== id)]
        .slice(0, MAX_NOTIFICATIONS),
    );
    dispatch({ type: 'push', notification, dedupe: input.dedupe });
    playNotificationSound();
    // Mirror to OS-level browser notification (gated by permission + toggle).
    fireBrowserNotification(notification, (markId) => dispatch({ type: 'markRead', id: markId }));
    return notification;
  }, []);

  const markAsRead = useCallback((id: string) => {
    const current = notificationsByIDRef.current.get(id);
    if (current) notificationsByIDRef.current.set(id, { ...current, read: true });
    dispatch({ type: 'markRead', id });
  }, []);
  const markAllRead = useCallback(() => {
    notificationsByIDRef.current = new Map(
      Array.from(notificationsByIDRef.current.entries()).map(([id, n]) => [id, n.read ? n : { ...n, read: true }]),
    );
    dispatch({ type: 'markAllRead' });
  }, []);
  const dismiss = useCallback((id: string) => {
    notificationsByIDRef.current.delete(id);
    dispatch({ type: 'dismiss', id });
  }, []);
  const clearAll = useCallback(() => {
    notificationsByIDRef.current.clear();
    dispatch({ type: 'clearAll' });
  }, []);

  const value = useMemo<NotificationsContextValue>(() => {
    const unreadCount = state.notifications.reduce((acc, n) => acc + (n.read ? 0 : 1), 0);
    return {
      notifications: state.notifications,
      unreadCount,
      push,
      markAsRead,
      markAllRead,
      dismiss,
      clearAll,
    };
  }, [state.notifications, push, markAsRead, markAllRead, dismiss, clearAll]);

  // Expose a tiny window helper for E2E + future server-pushed events.
  useEffect(() => {
    if (typeof window === 'undefined') return;
    const w = window as unknown as { __webmailNotifications?: NotificationsContextValue };
    w.__webmailNotifications = value;
    return () => {
      if (w.__webmailNotifications === value) delete w.__webmailNotifications;
    };
  }, [value]);

  return createElement(NotificationsContext.Provider, { value }, children);
}

export function useNotifications(): NotificationsContextValue {
  const ctx = useContext(NotificationsContext);
  if (!ctx) {
    throw new Error('useNotifications must be used within <NotificationProvider>');
  }
  return ctx;
}

/** Read-only access that does not throw outside the provider. */
export function useOptionalNotifications(): NotificationsContextValue | null {
  return useContext(NotificationsContext);
}
