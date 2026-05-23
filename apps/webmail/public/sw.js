// Service Worker for GoGoMail WebPush notifications
'use strict';

function safeNotificationClickUrl(value) {
  if (typeof value !== 'string') return '/mail';
  if (!value.startsWith('/') || value.startsWith('//')) return '/mail';
  return value;
}

function safeNotificationText(value, fallback) {
  if (typeof value !== 'string') return fallback;
  if (value.trim() === '') return fallback;
  return value;
}

function safeNotificationPayload(value) {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
  return value;
}

self.addEventListener('push', (event) => {
  let data = {};
  try {
    data = safeNotificationPayload(event.data?.json());
  } catch {
    data = { title: event.data?.text() ?? '새 메일' };
  }

  const title = safeNotificationText(data.title, '새 메일');
  const options = {
    body: safeNotificationText(data.body, ''),
    icon: '/favicon.ico',
    badge: '/favicon.ico',
    data: data,
    tag: safeNotificationText(data.tag, 'gogomail-notification'),
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
        if (client.url.includes('/mail') && 'focus' in client) {
          return client.focus();
        }
      }
      if (clients.openWindow) {
        return clients.openWindow(url);
      }
    })
  );
});
