'use client';

import { useEffect, useRef } from 'react';
import { enableWebPush, isWebPushSupported } from '@/lib/webpush/subscription';
import { ignoreNonCritical } from '@/lib/promise';

// Re-syncs the Web Push subscription on app boot so a returning user's browser
// endpoint stays registered with the backend (endpoints can rotate). This runs
// only when the user previously opted in (settings toggle → localStorage flag)
// AND notification permission is already granted, so it never surprises users
// with a permission prompt on load. Initial opt-in and unsubscribe are driven
// by the settings toggle (useSettingsNotifications).
export function useWebPush(): void {
  const ran = useRef(false);

  useEffect(() => {
    if (ran.current) return;
    ran.current = true;

    if (!isWebPushSupported()) return;

    let optedIn = false;
    try { optedIn = localStorage.getItem('webmail_webpush_enabled') === 'true'; } catch { optedIn = false; }
    if (!optedIn) return;
    if (Notification.permission !== 'granted') return;

    // enableWebPush is idempotent: it reuses the existing PushManager
    // subscription when present and upserts it against the documented
    // POST /api/v1/me/push-subscriptions endpoint (via the /api/mail proxy).
    ignoreNonCritical(enableWebPush(), 'webpush.resync');

    const onSwMessage = (event: MessageEvent) => {
      if ((event.data as { type?: string } | null)?.type === 'pushsubscriptionchange') {
        ignoreNonCritical(enableWebPush(), 'webpush.subscriptionChange');
      }
    };
    navigator.serviceWorker?.addEventListener('message', onSwMessage);
    return () => navigator.serviceWorker?.removeEventListener('message', onSwMessage);
  }, []);
}
