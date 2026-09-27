'use client';

import { useEffect, useState, useCallback } from 'react';
import { useTranslations } from 'next-intl';

// Minimal typing for the non-standard beforeinstallprompt event.
interface BeforeInstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

const DISMISS_KEY = 'webmail_pwa_install_dismissed_at';
// Re-show the banner at most once per 14 days after a dismissal so it never
// becomes nagware.
const DISMISS_COOLDOWN_MS = 14 * 24 * 60 * 60 * 1000;

function recentlyDismissed(): boolean {
  try {
    const raw = localStorage.getItem(DISMISS_KEY);
    if (!raw) return false;
    const at = Number(raw);
    if (!Number.isFinite(at)) return false;
    return Date.now() - at < DISMISS_COOLDOWN_MS;
  } catch {
    return false;
  }
}

export function InstallPrompt() {
  const t = useTranslations('pwa');
  const [deferred, setDeferred] = useState<BeforeInstallPromptEvent | null>(null);
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    // Already installed (standalone) → never prompt.
    const standalone = window.matchMedia('(display-mode: standalone)').matches
      || (window.navigator as unknown as { standalone?: boolean }).standalone === true;
    if (standalone) return;

    const onBeforeInstall = (e: Event) => {
      e.preventDefault();
      if (recentlyDismissed()) return;
      setDeferred(e as BeforeInstallPromptEvent);
      setVisible(true);
    };
    const onInstalled = () => {
      setVisible(false);
      setDeferred(null);
    };
    window.addEventListener('beforeinstallprompt', onBeforeInstall);
    window.addEventListener('appinstalled', onInstalled);
    return () => {
      window.removeEventListener('beforeinstallprompt', onBeforeInstall);
      window.removeEventListener('appinstalled', onInstalled);
    };
  }, []);

  const dismiss = useCallback(() => {
    setVisible(false);
    try { localStorage.setItem(DISMISS_KEY, String(Date.now())); } catch { /* best-effort */ }
  }, []);

  const install = useCallback(async () => {
    if (!deferred) return;
    try {
      await deferred.prompt();
      await deferred.userChoice;
    } catch {
      // user dismissed the native prompt; ignore
    }
    setDeferred(null);
    setVisible(false);
  }, [deferred]);

  if (!visible || !deferred) return null;

  return (
    <div
      role="dialog"
      aria-label={t('installTitle')}
      style={{
        position: 'fixed',
        left: '50%',
        transform: 'translateX(-50%)',
        bottom: 'calc(16px + env(safe-area-inset-bottom))',
        zIndex: 400,
        width: 'min(440px, calc(100vw - 24px))',
        display: 'flex',
        alignItems: 'center',
        gap: '12px',
        padding: '12px 14px',
        borderRadius: '14px',
        background: 'var(--color-bg-secondary)',
        border: '1px solid var(--color-border-default)',
        boxShadow: '0 12px 40px rgba(0,0,0,0.22)',
      }}
    >
      <div
        aria-hidden="true"
        style={{
          flexShrink: 0, width: '40px', height: '40px', borderRadius: '10px',
          background: 'var(--color-accent)', display: 'grid', placeItems: 'center',
        }}
      >
        <svg viewBox="0 0 24 24" width="22" height="22" fill="none" stroke="#fff" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
          <rect x="3" y="5" width="18" height="14" rx="2" />
          <path d="m3 7 9 6 9-6" />
        </svg>
      </div>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: '14px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{t('installTitle')}</div>
        <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)', lineHeight: 1.4 }}>{t('installDesc')}</div>
      </div>
      <button
        type="button"
        onClick={install}
        style={{
          flexShrink: 0, minHeight: '44px', padding: '0 16px', borderRadius: '10px',
          border: 'none', background: 'var(--color-accent)', color: '#fff',
          fontSize: '13px', fontWeight: 600, cursor: 'pointer',
        }}
      >
        {t('installAction')}
      </button>
      <button
        type="button"
        onClick={dismiss}
        aria-label={t('installDismiss')}
        style={{
          flexShrink: 0, width: '44px', minHeight: '44px', borderRadius: '10px',
          border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)',
          fontSize: '20px', cursor: 'pointer', lineHeight: 1,
        }}
      >
        ×
      </button>
    </div>
  );
}
