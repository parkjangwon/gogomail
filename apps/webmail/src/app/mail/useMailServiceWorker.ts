'use client';

import { useEffect, useRef } from 'react';

interface UseMailServiceWorkerParams {
  refreshIntervalSeconds: number;
  onRefresh: () => void;
}

export function useMailServiceWorker(params: UseMailServiceWorkerParams) {
  const { refreshIntervalSeconds, onRefresh } = params;

  // Keep a stable ref to the latest refresh callback
  const refreshRef = useRef(onRefresh);
  useEffect(() => { refreshRef.current = onRefresh; }, [onRefresh]);

  // Periodic background poll
  useEffect(() => {
    const id = setInterval(() => {
      if (document.visibilityState === 'visible') refreshRef.current();
    }, refreshIntervalSeconds * 1000);
    return () => clearInterval(id);
  }, [refreshIntervalSeconds]);

  // Immediate refresh when the tab becomes visible (e.g. user returns after
  // seeing a push notification in another tab/OS notification).
  useEffect(() => {
    let lastRefresh = Date.now();
    function onVisible() {
      if (document.visibilityState !== 'visible') return;
      // Only refresh if it's been more than 10 s since the last poll/refresh
      // to avoid a double-hit when the page first loads.
      if (Date.now() - lastRefresh > 10_000) {
        lastRefresh = Date.now();
        refreshRef.current();
      }
    }
    document.addEventListener('visibilitychange', onVisible);
    return () => document.removeEventListener('visibilitychange', onVisible);
  }, []);

  // Service worker registration is handled app-wide in useServiceWorker (via
  // Providers) so PWA install + offline work regardless of notification
  // permission. Push subscription is layered on separately (useWebPush /
  // settings toggle). This hook only listens for SW-driven mail refreshes.

  // Refresh mail list when the service worker signals a push notification arrived.
  useEffect(() => {
    if (!('serviceWorker' in navigator)) return;
    function onSwMessage(event: MessageEvent) {
      if ((event.data as { type?: string } | null)?.type === 'mail_update') {
        refreshRef.current();
      }
    }
    navigator.serviceWorker.addEventListener('message', onSwMessage);
    return () => navigator.serviceWorker.removeEventListener('message', onSwMessage);
  }, []);

  return { refreshRef };
}
