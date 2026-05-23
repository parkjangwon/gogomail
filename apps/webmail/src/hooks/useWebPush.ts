'use client';

import { useEffect, useRef } from 'react';
import { webPushPublicKeyToUint8Array } from '@/lib/webpush';

async function fetchVAPIDPublicKey(): Promise<string | null> {
  try {
    const res = await fetch('/api/v1/config/web-push');
    if (!res.ok) return null;
    const data = await res.json() as { vapidPublicKey?: string | null };
    return data.vapidPublicKey ?? null;
  } catch {
    return null;
  }
}

async function saveSubscription(sub: PushSubscription): Promise<void> {
  const json = sub.toJSON();
  const keys = json.keys as { p256dh?: string; auth?: string } | undefined;
  if (!json.endpoint || !keys?.p256dh || !keys?.auth) return;
  try {
    await fetch('/api/v1/me/push-subscriptions', {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({
        endpoint: json.endpoint,
        p256dh: keys.p256dh,
        auth: keys.auth,
        userAgent: navigator.userAgent.slice(0, 256),
      }),
    });
  } catch {
    // best-effort; do not throw
  }
}

async function registerWebPush(vapidPublicKey: string): Promise<void> {
  if (!('serviceWorker' in navigator) || !('PushManager' in window)) return;
  if (Notification.permission !== 'granted') return;

  const registration = await navigator.serviceWorker.register('/sw.js');
  await navigator.serviceWorker.ready;

  const applicationServerKey = webPushPublicKeyToUint8Array(vapidPublicKey);
  let sub = await registration.pushManager.getSubscription();
  if (!sub) {
    sub = await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey,
    });
  }
  await saveSubscription(sub);
}

export function useWebPush(): void {
  const registered = useRef(false);

  useEffect(() => {
    if (registered.current) return;
    registered.current = true;

    void (async () => {
      const vapidPublicKey = await fetchVAPIDPublicKey();
      if (!vapidPublicKey) return;

      await registerWebPush(vapidPublicKey);

      navigator.serviceWorker?.addEventListener('message', (event: MessageEvent) => {
        if ((event.data as { type?: string } | null)?.type === 'pushsubscriptionchange') {
          void registerWebPush(vapidPublicKey);
        }
      });
    })();
  }, []);
}
