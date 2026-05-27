'use client';

import { useTranslations } from 'next-intl';

export function ReadingPaneLoading() {
  const t = useTranslations();
  return (
    <main
      aria-label={t('misc.readingPane.region')}
      data-print-reading-pane
      style={{
        flex: 1,
        minWidth: 0,
        height: '100%',
        overflowY: 'auto',
        padding: '20px 24px',
        background: 'var(--color-bg-primary)',
        display: 'flex',
        flexDirection: 'column',
        gap: '16px',
      }}
    >
      {[100, 60, 80, 40, 70, 90].map((w, i) => (
        <div
          key={i}
          style={{
            height: i === 0 ? '24px' : '14px',
            background: 'var(--color-bg-tertiary)',
            borderRadius: '4px',
            width: `${w}%`,
          }}
        />
      ))}
    </main>
  );
}

export function ReadingPaneEmpty() {
  const t = useTranslations();
  return (
    <main
      aria-label={t('misc.readingPane.region')}
      data-print-reading-pane
      style={{
        flex: 1,
        minWidth: 0,
        height: '100%',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        gap: '8px',
        background: 'var(--color-bg-primary)',
        color: 'var(--color-text-tertiary)',
      }}
    >
      <svg
        width="40"
        height="40"
        viewBox="0 0 24 24"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        aria-hidden="true"
      >
        <rect x="2" y="4" width="20" height="16" rx="2" />
        <path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7" />
      </svg>
      <p style={{ fontSize: '14px' }}>{t('misc.readingPane.selectMessage')}</p>
    </main>
  );
}
