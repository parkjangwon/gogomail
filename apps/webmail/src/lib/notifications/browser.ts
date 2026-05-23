'use client';

// TODO(push-api): When tab is closed, only Push API + ServiceWorker can
// deliver. Current implementation handles tab-open / minimized cases only.
// Integration point: register a service worker at /sw.js and subscribe via
// pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: VAPID }).
// Server endpoint: POST /api/v1/me/push-subscriptions { endpoint, keys }.

import { useCallback, useEffect, useState } from 'react';

export type BrowserNotificationPermission = 'default' | 'granted' | 'denied' | 'unsupported';

export interface BrowserNotificationState {
  permission: BrowserNotificationPermission;
  /** User toggle (default true when granted). */
  enabled: boolean;
  request: () => Promise<BrowserNotificationPermission>;
  setEnabled: (enabled: boolean) => void;
}

export const BROWSER_NOTIF_ENABLED_KEY = 'webmail_browser_notifications_enabled';

/** SSR-safe permission read. */
function readPermission(): BrowserNotificationPermission {
  if (typeof window === 'undefined') return 'unsupported';
  if (typeof Notification === 'undefined') return 'unsupported';
  const p = Notification.permission;
  if (p === 'granted' || p === 'denied' || p === 'default') return p;
  return 'default';
}

/** SSR-safe enabled-flag read. Defaults to true when unset. */
function readEnabled(): boolean {
  if (typeof window === 'undefined') return true;
  try {
    const raw = window.localStorage.getItem(BROWSER_NOTIF_ENABLED_KEY);
    if (raw === null) return true;
    return raw !== 'false';
  } catch {
    return true;
  }
}

export function useBrowserNotifications(): BrowserNotificationState {
  const [permission, setPermission] = useState<BrowserNotificationPermission>('unsupported');
  const [enabled, setEnabledState] = useState<boolean>(true);

  // Initialize after mount (avoid SSR mismatch).
  useEffect(() => {
    setPermission(readPermission());
    setEnabledState(readEnabled());
  }, []);

  // Cross-tab sync for the enabled flag.
  useEffect(() => {
    if (typeof window === 'undefined') return;
    const onStorage = (e: StorageEvent) => {
      if (e.key !== BROWSER_NOTIF_ENABLED_KEY) return;
      setEnabledState(readEnabled());
    };
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  // Re-read permission when tab regains focus (user may have toggled in browser
  // settings while away).
  useEffect(() => {
    if (typeof window === 'undefined') return;
    const onFocus = () => setPermission(readPermission());
    window.addEventListener('focus', onFocus);
    document.addEventListener('visibilitychange', onFocus);
    return () => {
      window.removeEventListener('focus', onFocus);
      document.removeEventListener('visibilitychange', onFocus);
    };
  }, []);

  const request = useCallback(async (): Promise<BrowserNotificationPermission> => {
    if (typeof window === 'undefined' || typeof Notification === 'undefined') {
      return 'unsupported';
    }
    try {
      const result = await Notification.requestPermission();
      const mapped: BrowserNotificationPermission =
        result === 'granted' || result === 'denied' || result === 'default' ? result : 'default';
      setPermission(mapped);
      return mapped;
    } catch {
      // Some browsers throw when called outside a user gesture or in incognito.
      const current = readPermission();
      setPermission(current);
      return current;
    }
  }, []);

  const setEnabled = useCallback((next: boolean) => {
    setEnabledState(next);
    if (typeof window === 'undefined') return;
    try {
      window.localStorage.setItem(BROWSER_NOTIF_ENABLED_KEY, next ? 'true' : 'false');
    } catch {
      // ignore
    }
  }, []);

  return { permission, enabled, request, setEnabled };
}
