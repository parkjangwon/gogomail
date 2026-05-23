'use client';

import { useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import type { MessageDetail } from '@/lib/api';
import { getSmartReplies } from './readingPaneHelpers';

interface QuickReplyPanelProps {
  message: MessageDetail;
  onQuickReply: (body: string) => Promise<void>;
}

export function QuickReplyPanel({ message, onQuickReply }: QuickReplyPanelProps) {
  const t = useTranslations('quickReply');
  const [quickReplyOpen, setQuickReplyOpen] = useState(false);
  const [quickReplyText, setQuickReplyText] = useState('');
  const [quickReplySending, setQuickReplySending] = useState(false);
  const [quickReplySent, setQuickReplySent] = useState(false);
  const quickReplyRef = useRef<HTMLTextAreaElement>(null);

  async function sendQuickReply() {
    if (quickReplySending || !quickReplyText.trim()) return;
    setQuickReplySending(true);
    try {
      await onQuickReply(quickReplyText.trim());
      setQuickReplySent(true);
      setQuickReplyText('');
      setTimeout(() => {
        setQuickReplySent(false);
        setQuickReplyOpen(false);
      }, 1500);
    } finally {
      setQuickReplySending(false);
    }
  }

  return (
    <div style={{ marginTop: '24px', borderTop: '1px solid var(--color-border-subtle)', paddingTop: '16px' }}>
      {!quickReplyOpen ? (
        <>
          <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '10px' }}>
            {getSmartReplies(message.subject, message.text_body || '').map((reply) => (
              <button
                key={reply}
                onClick={() => {
                  setQuickReplyText(reply);
                  setQuickReplyOpen(true);
                  setTimeout(() => quickReplyRef.current?.focus(), 50);
                }}
                style={{ padding: '6px 12px', borderRadius: '16px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer', flexShrink: 0, transition: 'border-color 120ms, color 120ms' }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.borderColor = 'var(--color-accent)';
                  (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-accent)';
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.borderColor = 'var(--color-border-default)';
                  (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-secondary)';
                }}
              >
                {reply}
              </button>
            ))}
          </div>
          <button
            onClick={() => {
              setQuickReplyOpen(true);
              setTimeout(() => quickReplyRef.current?.focus(), 50);
            }}
            style={{ width: '100%', maxWidth: '680px', textAlign: 'left', padding: '10px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-tertiary)', fontSize: '14px', cursor: 'text' }}
          >
            {t('replyButton')}
          </button>
        </>
      ) : (
        <div style={{ maxWidth: '680px', border: '1px solid var(--color-accent)', borderRadius: '6px', overflow: 'hidden' }}>
          <textarea
            ref={quickReplyRef}
            value={quickReplyText}
            onChange={(e) => setQuickReplyText(e.target.value)}
            placeholder={t('placeholder')}
            rows={4}
            onKeyDown={(e) => {
              if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
                e.preventDefault();
                if (!quickReplySending && quickReplyText.trim()) {
                  void sendQuickReply();
                }
              }
              if (e.key === 'Escape') {
                setQuickReplyOpen(false);
                setQuickReplyText('');
              }
            }}
            style={{ width: '100%', padding: '12px 14px', border: 'none', outline: 'none', resize: 'vertical', fontSize: '14px', lineHeight: 1.6, background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', boxSizing: 'border-box' }}
          />
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', background: 'var(--color-bg-secondary)', borderTop: '1px solid var(--color-border-subtle)' }}>
            <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>{t('hint')}</span>
            <div style={{ display: 'flex', gap: '8px' }}>
              <button
                onClick={() => {
                  setQuickReplyOpen(false);
                  setQuickReplyText('');
                }}
                style={{ padding: '5px 12px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}
              >{t('cancel')}</button>
              <button
                disabled={quickReplySending || !quickReplyText.trim()}
                onClick={() => void sendQuickReply()}
                style={{ padding: '5px 14px', borderRadius: '5px', border: 'none', background: quickReplySending || !quickReplyText.trim() ? 'var(--color-border-default)' : 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 500, cursor: quickReplySending || !quickReplyText.trim() ? 'not-allowed' : 'pointer' }}
              >
                {quickReplySent ? t('sent') : quickReplySending ? t('sending') : t('send')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
