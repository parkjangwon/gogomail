// Web Push subscribe/unsubscribe orchestration shared by the settings toggle
// and the app-boot re-sync. Encapsulates: fetching the server VAPID key,
// ensuring the service worker is registered, creating/removing the browser
// PushManager subscription, and mirroring it to the documented backend API.
import {
  getWebPushConfig,
  upsertWebPushSubscription,
  listWebPushSubscriptions,
  deleteWebPushSubscription,
} from '@/lib/api';
import { webPushPublicKeyToUint8Array, arrayBufferToBase64URL } from '@/lib/webpush';

export function isWebPushSupported(): boolean {
  return typeof window !== 'undefined'
    && 'serviceWorker' in navigator
    && 'PushManager' in window
    && typeof Notification !== 'undefined';
}

async function ensureRegistration(): Promise<ServiceWorkerRegistration> {
  const existing = await navigator.serviceWorker.getRegistration();
  const registration = existing ?? await navigator.serviceWorker.register('/sw.js');
  await navigator.serviceWorker.ready;
  return registration;
}

export class WebPushError extends Error {}

// Subscribes the current browser to Web Push and registers it with the backend.
// Throws WebPushError with a stable code-ish message the UI can surface.
export async function enableWebPush(): Promise<void> {
  if (!isWebPushSupported()) throw new WebPushError('unsupported');

  const permission = Notification.permission === 'granted'
    ? 'granted'
    : await Notification.requestPermission();
  if (permission !== 'granted') throw new WebPushError('permission-denied');

  const { vapidPublicKey } = await getWebPushConfig();
  if (!vapidPublicKey) throw new WebPushError('no-vapid-key');

  const registration = await ensureRegistration();
  const applicationServerKey = webPushPublicKeyToUint8Array(vapidPublicKey);

  let sub = await registration.pushManager.getSubscription();
  if (!sub) {
    sub = await registration.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey,
    });
  }

  const json = sub.toJSON();
  const keys = (json.keys ?? {}) as { p256dh?: string; auth?: string };
  const p256dh = keys.p256dh ?? arrayBufferToBase64URL(sub.getKey('p256dh'));
  const auth = keys.auth ?? arrayBufferToBase64URL(sub.getKey('auth'));
  if (!json.endpoint || !p256dh || !auth) throw new WebPushError('incomplete-subscription');

  await upsertWebPushSubscription({
    endpoint: json.endpoint,
    p256dh,
    auth,
    userAgent: navigator.userAgent.slice(0, 256),
  });
}

// Removes the browser subscription and its backend record(s).
export async function disableWebPush(): Promise<void> {
  if (!isWebPushSupported()) return;

  const registration = await navigator.serviceWorker.getRegistration();
  const sub = registration ? await registration.pushManager.getSubscription() : null;
  const endpoint = sub?.endpoint;

  // Remove the matching backend subscription(s) before unsubscribing locally so
  // the server stops targeting this endpoint even if the local step fails.
  try {
    const subscriptions = await listWebPushSubscriptions();
    const targets = endpoint
      ? subscriptions.filter((s) => s.endpoint === endpoint)
      : subscriptions;
    await Promise.allSettled(targets.map((s) => deleteWebPushSubscription(s.id)));
  } catch {
    // best-effort server cleanup
  }

  if (sub) {
    try { await sub.unsubscribe(); } catch { /* best-effort */ }
  }
}
