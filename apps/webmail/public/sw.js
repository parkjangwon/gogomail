// Service Worker for GoGoMail: Web Push notifications + PWA app-shell/offline.
'use strict';

// ─── PWA app-shell + runtime caching ───────────────────────────────────────
//
// Strategy:
//   • Precache the offline fallback + core icons on install (app shell).
//   • Navigations: network-first, fall back to the last-good cached page, then
//     to /offline.html when nothing is cached.
//   • Next.js build assets (/_next/static/*, immutable, content-hashed):
//     cache-first — safe because filenames change on every deploy.
//   • Same-origin images/fonts: stale-while-revalidate.
//   • API requests (/api/*): always network — mail data offline caching is
//     handled at the app layer (IndexedDB) so we never serve stale no-store
//     API JSON from the SW.
//
// Bump CACHE_VERSION to invalidate old caches on the next activate.
const CACHE_VERSION = 'v1';
const SHELL_CACHE = `gogomail-shell-${CACHE_VERSION}`;
const RUNTIME_CACHE = `gogomail-runtime-${CACHE_VERSION}`;
const ASSET_CACHE = `gogomail-assets-${CACHE_VERSION}`;
const OFFLINE_URL = '/offline.html';
const RUNTIME_MAX_ENTRIES = 60;

const PRECACHE_URLS = [
  OFFLINE_URL,
  '/manifest.webmanifest',
  '/icons/icon-192.png',
  '/icons/icon-512.png',
];

self.addEventListener('install', (event) => {
  event.waitUntil(
    caches.open(SHELL_CACHE)
      .then((cache) => cache.addAll(PRECACHE_URLS))
      .then(() => self.skipWaiting())
      .catch(() => self.skipWaiting())
  );
});

self.addEventListener('activate', (event) => {
  const keep = new Set([SHELL_CACHE, RUNTIME_CACHE, ASSET_CACHE]);
  event.waitUntil(
    caches.keys()
      .then((keys) => Promise.all(keys.map((k) => (keep.has(k) ? null : caches.delete(k)))))
      .then(() => self.clients.claim())
  );
});

async function trimCache(cacheName, maxEntries) {
  try {
    const cache = await caches.open(cacheName);
    const keys = await cache.keys();
    if (keys.length <= maxEntries) return;
    for (let i = 0; i < keys.length - maxEntries; i += 1) {
      await cache.delete(keys[i]);
    }
  } catch {
    // best-effort
  }
}

async function handleNavigation(request) {
  try {
    const response = await fetch(request);
    if (response && response.ok) {
      const cache = await caches.open(RUNTIME_CACHE);
      cache.put(request, response.clone());
      void trimCache(RUNTIME_CACHE, RUNTIME_MAX_ENTRIES);
    }
    return response;
  } catch {
    const cached = await caches.match(request, { ignoreSearch: true });
    if (cached) return cached;
    const offline = await caches.match(OFFLINE_URL);
    if (offline) return offline;
    return new Response('Offline', { status: 503, statusText: 'Offline' });
  }
}

async function handleAsset(request) {
  const cache = await caches.open(ASSET_CACHE);
  const cached = await cache.match(request);
  if (cached) return cached;
  try {
    const response = await fetch(request);
    if (response && (response.ok || response.type === 'opaque')) {
      cache.put(request, response.clone());
    }
    return response;
  } catch {
    if (cached) return cached;
    throw new Error('asset fetch failed and no cache');
  }
}

async function handleImage(request) {
  const cache = await caches.open(ASSET_CACHE);
  const cached = await cache.match(request);
  const network = fetch(request)
    .then((response) => {
      if (response && (response.ok || response.type === 'opaque')) {
        cache.put(request, response.clone());
        void trimCache(ASSET_CACHE, RUNTIME_MAX_ENTRIES);
      }
      return response;
    })
    .catch(() => cached);
  return cached || network;
}

self.addEventListener('fetch', (event) => {
  const { request } = event;
  if (request.method !== 'GET') return;

  const url = new URL(request.url);
  const sameOrigin = url.origin === self.location.origin;

  // Never intercept API traffic — offline mail data is handled by the app-layer
  // IndexedDB cache, and API responses are marked no-store by the proxy.
  if (sameOrigin && (url.pathname.startsWith('/api/'))) return;

  if (request.mode === 'navigate') {
    event.respondWith(handleNavigation(request));
    return;
  }

  if (sameOrigin && url.pathname.startsWith('/_next/static/')) {
    event.respondWith(handleAsset(request));
    return;
  }

  if (sameOrigin && (url.pathname.startsWith('/icons/') || /\.(?:png|jpg|jpeg|gif|svg|webp|ico|woff2?)$/.test(url.pathname))) {
    event.respondWith(handleImage(request));
    return;
  }
});

// Allow the page to trigger an immediate SW takeover after an update.
self.addEventListener('message', (event) => {
  if (event.data && event.data.type === 'SKIP_WAITING') {
    self.skipWaiting();
  }
});

// ─── Web Push notifications ─────────────────────────────────────────────────

const UNSAFE_CLICK_URL_CHARS = /[\u0000-\u001F\u007F\\]/;
const UNSAFE_TAG_CHARS = /[\u0000-\u001F\u007F\\]/;
const UNSAFE_DISPLAY_TEXT_CHARS = /[\u0000-\u001F\u007F]+/g;
const MAX_CLICK_URL_LENGTH = 2048;
const MAX_TITLE_LENGTH = 160;
const MAX_BODY_LENGTH = 500;
const MAX_TAG_LENGTH = 128;
const DEFAULT_TAG = 'gogomail-notification';

function safeNotificationClickUrl(value) {
  if (typeof value !== 'string') return '/mail';
  if (
    !value.startsWith('/')
    || value.startsWith('//')
    || value.length > MAX_CLICK_URL_LENGTH
    || UNSAFE_CLICK_URL_CHARS.test(value)
  ) return '/mail';
  return value;
}

function truncateText(value, maxLength) {
  if (value.length <= maxLength) return value;
  const truncated = value.slice(0, maxLength);
  const lastCodeUnit = truncated.charCodeAt(truncated.length - 1);
  return lastCodeUnit >= 0xD800 && lastCodeUnit <= 0xDBFF
    ? truncated.slice(0, -1)
    : truncated;
}

function safeNotificationText(value, fallback, maxLength) {
  if (typeof value !== 'string') return fallback;
  if (value.trim() === '') return fallback;
  const normalized = value.replace(UNSAFE_DISPLAY_TEXT_CHARS, ' ');
  if (normalized.trim() === '') return fallback;
  return truncateText(normalized, maxLength);
}

function safeNotificationTag(value) {
  if (typeof value !== 'string') return DEFAULT_TAG;
  if (value.trim() === '') return DEFAULT_TAG;
  if (UNSAFE_TAG_CHARS.test(value)) return DEFAULT_TAG;
  return truncateText(value, MAX_TAG_LENGTH);
}

function safeNotificationPayload(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
  return value;
}

function safeNotificationData(payload) {
  if (typeof payload.url !== 'string') return {};
  return { url: safeNotificationClickUrl(payload.url) };
}

function isMailClientUrl(value) {
  try {
    const url = new URL(value);
    return url.pathname === '/mail' || url.pathname.startsWith('/mail/');
  } catch {
    return false;
  }
}

self.addEventListener('push', (event) => {
  let data = {};
  try {
    data = safeNotificationPayload(event.data?.json());
  } catch {
    data = { title: event.data?.text() ?? '새 메일' };
  }

  const title = safeNotificationText(data.title, '새 메일', MAX_TITLE_LENGTH);
  const options = {
    body: safeNotificationText(data.body, '', MAX_BODY_LENGTH),
    icon: '/favicon.ico',
    badge: '/favicon.ico',
    data: safeNotificationData(data),
    tag: safeNotificationTag(data.tag),
    renotify: true,
  };

  event.waitUntil(
    self.registration.showNotification(title, options).then(() =>
      // Notify all open clients to refresh the mail list
      clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
        for (const client of clientList) {
          client.postMessage({ type: 'mail_update' });
        }
      })
    )
  );
});

self.addEventListener('notificationclick', (event) => {
  event.notification.close();
  const url = safeNotificationClickUrl(event.notification.data?.url);
  event.waitUntil(
    clients.matchAll({ type: 'window', includeUncontrolled: true }).then((clientList) => {
      for (const client of clientList) {
        if (isMailClientUrl(client.url) && 'focus' in client) {
          if ('navigate' in client) {
            return client.navigate(url).then((navigatedClient) => {
              if (navigatedClient && 'focus' in navigatedClient) {
                return navigatedClient.focus();
              }
              return client.focus();
            }).catch(() => client.focus());
          }
          return client.focus();
        }
      }
      if (clients.openWindow) {
        return clients.openWindow(url);
      }
    })
  );
});
