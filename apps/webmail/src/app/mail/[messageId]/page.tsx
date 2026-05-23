'use client';

import { useEffect, useCallback } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { useTranslations } from 'next-intl';
import { useMessage } from '@/hooks/useMessage';
import { ReadingPane } from '@/components/ReadingPane';
import { deleteMessage } from '@/lib/api';
import { ThemeToggle } from '@/components/ThemeToggle';
import { LocaleSelector } from '@/components/common/LocaleSelector';

export default function MessagePage() {
  const router = useRouter();
  const t = useTranslations();
  const params = useParams();
  const messageId = params?.messageId as string;

  useEffect(() => {
    if (typeof window === 'undefined') return;
    const authenticated = localStorage.getItem('webmail_authenticated');
    if (authenticated !== '1') router.push('/login');
  }, [router]);

  const { message, loading } = useMessage(messageId ?? null);

  const handleDelete = useCallback(async () => {
    if (!messageId) return;
    try {
      await deleteMessage(messageId);
    } catch {
      // ignore — navigate back regardless
    }
    router.push('/mail');
  }, [messageId, router]);

  return (
    <div
      style={{
        height: '100vh',
        display: 'flex',
        flexDirection: 'column',
        background: 'var(--color-bg-primary)',
      }}
    >
      {/* Top bar */}
      <div
        style={{
          padding: '10px 24px',
          borderBottom: '1px solid var(--color-border-subtle)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          flexShrink: 0,
        }}
      >
        <button
          onClick={() => router.push('/mail')}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            padding: '6px 12px',
            borderRadius: '6px',
            border: '1px solid var(--color-border-default)',
            background: 'transparent',
            color: 'var(--color-text-secondary)',
            fontSize: '13px',
            cursor: 'pointer',
            transition: 'background 100ms ease',
          }}
          onMouseEnter={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
          }}
          onMouseLeave={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
          }}
        >
          {t('misc.messagePage.backToInbox')}
        </button>

        <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
          <LocaleSelector />
          <ThemeToggle inline />
        </div>
      </div>

      {/* Message */}
      <div style={{ flex: 1, overflow: 'hidden' }}>
        <ReadingPane message={message} loading={loading} onDelete={handleDelete} />
      </div>
    </div>
  );
}
