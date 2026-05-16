'use client';

import { useEffect, useState } from 'react';

const WEBMAIL_AVATAR_KEY = 'webmail_avatar';
const WEBMAIL_AVATAR_EVENT = 'webmail-avatar-change';

function readAvatar(): string {
  try {
    return localStorage.getItem(WEBMAIL_AVATAR_KEY) ?? '';
  } catch {
    return '';
  }
}

export function setWebmailAvatar(url: string) {
  try {
    if (url) localStorage.setItem(WEBMAIL_AVATAR_KEY, url);
    else localStorage.removeItem(WEBMAIL_AVATAR_KEY);
  } catch {
    // ignore storage failures
  }
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent<{ avatarUrl: string }>(WEBMAIL_AVATAR_EVENT, { detail: { avatarUrl: url } }));
  }
}

export function useWebmailAvatar(): string {
  const [avatarUrl, setAvatarUrl] = useState(readAvatar);

  useEffect(() => {
    function onStorage(e: StorageEvent) {
      if (e.key === WEBMAIL_AVATAR_KEY) {
        setAvatarUrl(e.newValue ?? '');
      }
    }
    function onAvatarChange(e: Event) {
      const detail = (e as CustomEvent<{ avatarUrl?: string }>).detail;
      setAvatarUrl(detail?.avatarUrl ?? '');
    }

    window.addEventListener('storage', onStorage);
    window.addEventListener(WEBMAIL_AVATAR_EVENT, onAvatarChange as EventListener);
    return () => {
      window.removeEventListener('storage', onStorage);
      window.removeEventListener(WEBMAIL_AVATAR_EVENT, onAvatarChange as EventListener);
    };
  }, []);

  return avatarUrl;
}
