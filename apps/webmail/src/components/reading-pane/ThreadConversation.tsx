'use client';

import { useTranslations, useLocale } from 'next-intl';
import type { MessageSummary } from '@/lib/api';

interface ThreadConversationProps {
  messages: MessageSummary[];
  currentMessageId: string;
  userEmail?: string;
  onSelectThread: (id: string) => void;
}

export function ThreadConversation({ messages, currentMessageId, userEmail, onSelectThread }: ThreadConversationProps) {
  const t = useTranslations('thread');
  const locale = useLocale();
  if (!messages.length) return null;

  return (
    <div style={{ marginTop: '32px', borderTop: '1px solid var(--color-border-subtle)', paddingTop: '16px', maxWidth: '680px' }}>
      <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.07em', marginBottom: '12px' }}>
        {t('conversationCount', { count: messages.length })}
      </div>
      <div style={{ position: 'relative' }}>
        <div style={{ position: 'absolute', left: '15px', top: '16px', bottom: '16px', width: '1px', background: 'var(--color-border-subtle)' }} />
        {messages.map((message) => {
          const isCurrent = message.id === currentMessageId;
          const isMine = userEmail ? message.from_addr.toLowerCase() === userEmail.toLowerCase() : false;
          const _tz = (() => { try { return localStorage.getItem('webmail_timezone') || undefined; } catch { return undefined; } })();
          const date = new Intl.DateTimeFormat(locale, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false, ...(_tz ? { timeZone: _tz } : {}) }).format(new Date(message.received_at));
          const initial = (message.from_name || message.from_addr)[0]?.toUpperCase() ?? '?';

          return (
            <div
              key={message.id}
              onClick={() => {
                if (!isCurrent) onSelectThread(message.id);
              }}
              style={{ display: 'flex', flexDirection: isMine ? 'row-reverse' : 'row', gap: '10px', padding: '4px 0', cursor: isCurrent ? 'default' : 'pointer', alignItems: 'flex-end' }}
            >
              <div style={{ flexShrink: 0, width: '26px', height: '26px', borderRadius: '50%', background: isCurrent ? 'var(--color-accent)' : 'var(--color-bg-tertiary)', color: isCurrent ? '#fff' : 'var(--color-text-secondary)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '11px', fontWeight: 700, border: `2px solid ${isCurrent ? 'var(--color-accent)' : 'var(--color-border-default)'}`, boxSizing: 'border-box' }}>
                {initial}
              </div>
              <div
                style={{
                  maxWidth: '75%',
                  background: isCurrent ? 'var(--color-accent)' : isMine ? 'var(--color-bg-secondary)' : 'var(--color-bg-tertiary)',
                  borderRadius: isMine ? '12px 12px 4px 12px' : '12px 12px 12px 4px',
                  padding: '8px 12px',
                  opacity: isCurrent ? 1 : 0.88,
                  boxShadow: isCurrent ? '0 1px 4px rgba(0,0,0,0.15)' : 'none',
                }}
                onMouseEnter={(e) => {
                  if (!isCurrent) (e.currentTarget as HTMLDivElement).style.opacity = '1';
                }}
                onMouseLeave={(e) => {
                  if (!isCurrent) (e.currentTarget as HTMLDivElement).style.opacity = '0.88';
                }}
              >
                <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px', marginBottom: '3px' }}>
                  {!isMine && (
                    <span style={{ fontSize: '11px', fontWeight: 600, color: isCurrent ? 'rgba(255,255,255,0.9)' : 'var(--color-text-secondary)' }}>{message.from_name || message.from_addr}</span>
                  )}
                  <span style={{ fontSize: '10px', color: isCurrent ? 'rgba(255,255,255,0.7)' : 'var(--color-text-tertiary)', marginLeft: 'auto' }}>{date}</span>
                </div>
                <div style={{ fontSize: '12px', color: isCurrent ? '#fff' : 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '280px' }}>
                  {message.preview || message.subject || t('noContent')}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
