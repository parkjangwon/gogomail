// Service Worker for GoGoMail WebPush notifications
'use strict';

const UNSAFE_CLICK_URL_CHARS = /[\u0000-\u001F\u007F\\]/;
const UNSAFE_TAG_CHARS = /[\u0000-\u001F\u007F\\]/;
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

function safeNotificationText(value, fallback, maxLength) {
  if (typeof value !== 'string') return fallback;
  if (value.trim() === '') return fallback;
  return value.length > maxLength ? value.slice(0, maxLength) : value;
}

function safeNotificationTag(value) {
  const tag = safeNotificationText(value, DEFAULT_TAG, MAX_TAG_LENGTH);
  return UNSAFE_TAG_CHARS.test(tag) ? DEFAULT_TAG : tag;
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
    self.registration.showNotification(title, options)
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
