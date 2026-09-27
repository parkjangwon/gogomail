'use client';

import { useEffect } from 'react';

// Registers the service worker app-wide so PWA install + offline app-shell work
// regardless of notification permission. Push subscription is layered on top of
// the same registration (see useWebPush / settings), but must not gate it —
// otherwise the app is not installable and offline navigation breaks for users
// who never enable notifications.
export function useServiceWorker(): void {
  useEffect(() => {
    if (typeof navigator === 'undefined' || !('serviceWorker' in navigator)) return;
    // Only register over secure contexts (https or localhost). Browsers block
    // SW registration otherwise, and attempting it logs noisy errors.
    if (!window.isSecureContext) return;

    let cancelled = false;
    const register = async () => {
      try {
        const registration = await navigator.serviceWorker.register('/sw.js');
        if (cancelled) return;
        // If a new SW is waiting, ask it to activate immediately so the offline
        // shell and caches stay fresh after a deploy.
        if (registration.waiting) {
          registration.waiting.postMessage({ type: 'SKIP_WAITING' });
        }
        registration.addEventListener('updatefound', () => {
          const installing = registration.installing;
          if (!installing) return;
          installing.addEventListener('statechange', () => {
            if (installing.state === 'installed' && navigator.serviceWorker.controller) {
              installing.postMessage({ type: 'SKIP_WAITING' });
            }
          });
        });
      } catch {
        // Service worker is a progressive enhancement; ignore failures.
      }
    };

    // Register after load to avoid contending with initial page resources.
    if (document.readyState === 'complete') {
      void register();
    } else {
      window.addEventListener('load', register, { once: true });
    }

    return () => { cancelled = true; };
  }, []);
}
