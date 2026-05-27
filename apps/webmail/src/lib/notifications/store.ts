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
import type { Notification, NotificationInput } from './types';
import {
  STORAGE_KEY,
  MAX_NOTIFICATIONS,
  buildNotification,
  fireBrowserNotification,
  indexNotifications,
  loadInitial,
  playNotificationSound,
  sanitizeNotifications,
} from './notificationUtils';

// ── Reducer ───────────────────────────────────────────────────────────────────

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

// ── Context ───────────────────────────────────────────────────────────────────

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
    const shouldDedupe = rawInput.dedupe === true;
    const notification = buildNotification(rawInput);
    const existing = notificationsByIDRef.current.get(notification.id);
    if (shouldDedupe && existing) {
      return existing;
    }
    notificationsByIDRef.current = indexNotifications(
      [notification, ...Array.from(notificationsByIDRef.current.values()).filter((n) => n.id !== notification.id)]
        .slice(0, MAX_NOTIFICATIONS),
    );
    dispatch({ type: 'push', notification, dedupe: shouldDedupe });
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
