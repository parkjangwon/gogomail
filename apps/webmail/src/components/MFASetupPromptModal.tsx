'use client';

import { useEffect, useState, useCallback } from 'react';
import { ShieldExclamationIcon } from '@heroicons/react/24/outline';
import { useTranslations } from 'next-intl';

const STORAGE_KEY = 'webmail_mfa_setup_required';
const DISMISSED_KEY = 'webmail_mfa_setup_dismissed_at';
const DISMISS_COOLDOWN_MS = 24 * 60 * 60 * 1000; // re-show after 24h if still required

interface MFASetupPromptModalProps {
  onGoToSettings: () => void;
}

export function MFASetupPromptModal({ onGoToSettings }: MFASetupPromptModalProps) {
  const t = useTranslations('mfa');
  const [visible, setVisible] = useState(false);

  useEffect(() => {
    const required = localStorage.getItem(STORAGE_KEY) === '1';
    if (!required) return;

    const dismissedAt = localStorage.getItem(DISMISSED_KEY);
    if (dismissedAt) {
      const elapsed = Date.now() - parseInt(dismissedAt, 10);
      if (elapsed < DISMISS_COOLDOWN_MS) return;
    }

    setVisible(true);
  }, []);

  const handleDismiss = useCallback(() => {
    localStorage.setItem(DISMISSED_KEY, Date.now().toString());
    setVisible(false);
  }, []);

  const handleGoToSettings = useCallback(() => {
    localStorage.setItem(DISMISSED_KEY, Date.now().toString());
    setVisible(false);
    onGoToSettings();
  }, [onGoToSettings]);

  if (!visible) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="mfa-prompt-title"
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 9000,
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: '24px',
        background: 'rgba(0,0,0,0.45)',
        backdropFilter: 'blur(2px)',
      }}
      onClick={(e) => { if (e.target === e.currentTarget) handleDismiss(); }}
    >
      <div
        style={{
          width: '100%',
          maxWidth: '420px',
          background: 'var(--color-bg-primary)',
          borderRadius: '12px',
          border: '1px solid var(--color-border-default)',
          boxShadow: '0 16px 48px rgba(0,0,0,0.24)',
          overflow: 'hidden',
        }}
      >
        {/* Header */}
        <div style={{ padding: '24px 24px 0', display: 'flex', alignItems: 'flex-start', gap: '14px' }}>
          <div
            style={{
              flexShrink: 0,
              width: 40,
              height: 40,
              borderRadius: '10px',
              background: 'rgba(234,179,8,0.12)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <ShieldExclamationIcon style={{ width: 22, height: 22, color: '#ca8a04' }} />
          </div>
          <div>
            <h2 id="mfa-prompt-title" style={{ margin: 0, fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)' }}>
              {t('title')}
            </h2>
            <p style={{ margin: '6px 0 0', fontSize: '13px', color: 'var(--color-text-secondary)', lineHeight: 1.5 }}>
              {t('description')}
            </p>
          </div>
        </div>

        {/* Actions */}
        <div style={{ padding: '20px 24px 24px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
          <button
            onClick={handleGoToSettings}
            style={{
              width: '100%',
              padding: '11px 16px',
              borderRadius: '8px',
              border: 'none',
              background: 'var(--color-accent)',
              color: '#fff',
              fontSize: '14px',
              fontWeight: 600,
              cursor: 'pointer',
              transition: 'opacity 100ms',
            }}
            onMouseEnter={(e) => { (e.target as HTMLButtonElement).style.opacity = '0.88'; }}
            onMouseLeave={(e) => { (e.target as HTMLButtonElement).style.opacity = '1'; }}
          >
            {t('setupNow')}
          </button>
          <button
            onClick={handleDismiss}
            style={{
              width: '100%',
              padding: '10px 16px',
              borderRadius: '8px',
              border: '1px solid var(--color-border-default)',
              background: 'transparent',
              color: 'var(--color-text-secondary)',
              fontSize: '13px',
              cursor: 'pointer',
            }}
          >
            {t('later')}
          </button>
        </div>
      </div>
    </div>
  );
}
