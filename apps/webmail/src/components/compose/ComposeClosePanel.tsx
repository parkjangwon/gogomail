'use client';

import { useTranslations } from 'next-intl';

interface ComposeClosePanelProps {
  closeSaveInProgress: boolean;
  closeSavePrompt: string;
  closeSaveButtonAriaLabel: string;
  closeSaveButtonLabel: string;
  onSaveDraft: () => void;
  onDiscard: () => void;
  onCancel: () => void;
}

export function ComposeClosePanel({
  closeSaveInProgress,
  closeSavePrompt,
  closeSaveButtonAriaLabel,
  closeSaveButtonLabel,
  onSaveDraft,
  onDiscard,
  onCancel,
}: ComposeClosePanelProps) {
  const t = useTranslations('composeFull');

  return (
    <div
      role="alertdialog"
      aria-modal="false"
      aria-labelledby="compose-close-save-title"
      aria-busy={closeSaveInProgress}
      onKeyDown={(e) => {
        if (e.key === 'Escape') {
          e.stopPropagation();
          if (closeSaveInProgress) return;
          onCancel();
        }
      }}
      style={{
        padding: '10px 16px',
        borderBottom: '1px solid var(--color-border-subtle)',
        background: 'var(--color-bg-secondary)',
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        flexShrink: 0,
      }}
    >
      <span id="compose-close-save-title" style={{ fontSize: '13px', color: 'var(--color-text-primary)', flex: 1 }}>
        {closeSavePrompt}
      </span>
      <button
        type="button"
        aria-label={closeSaveButtonAriaLabel}
        disabled={closeSaveInProgress}
        onClick={() => { void onSaveDraft(); }}
        style={{ fontSize: '12px', padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', cursor: closeSaveInProgress ? 'not-allowed' : 'pointer' }}
      >
        {closeSaveButtonLabel}
      </button>
      <button
        type="button"
        aria-label={t('discardAria')}
        disabled={closeSaveInProgress}
        onClick={onDiscard}
        style={{ fontSize: '12px', padding: '4px 10px', borderRadius: '5px', border: '1px solid rgba(217,79,61,0.4)', background: 'transparent', color: 'var(--color-destructive)', cursor: closeSaveInProgress ? 'not-allowed' : 'pointer' }}
      >
        {t('discard')}
      </button>
      <button
        type="button"
        aria-label={t('cancelCloseAria')}
        disabled={closeSaveInProgress}
        onClick={onCancel}
        style={{ fontSize: '12px', padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: closeSaveInProgress ? 'not-allowed' : 'pointer' }}
      >
        {t('cancel')}
      </button>
    </div>
  );
}
