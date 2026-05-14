'use client';

import type { MessageDetail } from '@/lib/api';
import { emailOf } from '@/lib/message/messageUtils';
import { formatFullDate, readingTime } from './readingPaneHelpers';

interface MessageHeaderProps {
  message: MessageDetail;
  toList: string;
  ccList: string;
  copiedEmail: string;
  onCopyEmail: (email: string) => void;
  onComposeToAddress?: (address: string) => void;
  isContactSaved: boolean;
  savedContact: boolean;
  onSaveContact: () => void;
}

export function MessageHeader({
  message,
  toList,
  ccList,
  copiedEmail,
  onCopyEmail,
  onComposeToAddress,
  isContactSaved,
  savedContact,
  onSaveContact,
}: MessageHeaderProps) {
  return (
    <>
      <h1
        style={{
          fontSize: '18px',
          fontWeight: 600,
          color: 'var(--color-text-primary)',
          lineHeight: 1.4,
          marginBottom: '16px',
        }}
      >
        {message.subject || '(제목 없음)'}
      </h1>

      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px', marginBottom: '8px' }}>
        <div>
          <div style={{ fontSize: '14px', fontWeight: 500, color: 'var(--color-text-primary)' }}>
            <span
              title="클릭하면 주소 복사"
              onClick={() => onCopyEmail(message.from_addr)}
              style={{ cursor: 'pointer', borderRadius: '3px', padding: '0 2px' }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLSpanElement).style.background = 'var(--color-bg-secondary)';
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLSpanElement).style.background = 'transparent';
              }}
            >
              {copiedEmail === message.from_addr ? '복사됨 ✓' : (message.from_name || message.from_addr)}
            </span>
            {message.from_name && (
              <span
                title="클릭하면 주소 복사"
                onClick={() => onCopyEmail(message.from_addr)}
                style={{ fontSize: '13px', fontWeight: 400, color: 'var(--color-text-secondary)', marginInlineStart: '6px', cursor: 'pointer', borderRadius: '3px', padding: '0 2px' }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLSpanElement).style.background = 'var(--color-bg-secondary)';
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLSpanElement).style.background = 'transparent';
                }}
              >
                {copiedEmail === message.from_addr ? '' : `<${message.from_addr}>`}
              </span>
            )}
            {onComposeToAddress && (
              <button
                onClick={() => onComposeToAddress(message.from_addr)}
                title={`${message.from_addr}에게 새 메일 작성`}
                style={{ background: 'none', border: '1px solid var(--color-border-default)', borderRadius: '4px', cursor: 'pointer', fontSize: '11px', color: 'var(--color-text-tertiary)', padding: '1px 6px', marginInlineStart: '6px', lineHeight: 1.4 }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
                }}
              >메일 보내기</button>
            )}
            {!isContactSaved && (
              <button
                onClick={onSaveContact}
                title="연락처에 추가"
                style={{ background: 'none', border: '1px solid var(--color-border-default)', borderRadius: '4px', cursor: 'pointer', fontSize: '11px', color: savedContact ? 'var(--color-accent)' : 'var(--color-text-tertiary)', padding: '1px 6px', marginInlineStart: '4px', lineHeight: 1.4 }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
                }}
              >{savedContact ? '저장됨 ✓' : '연락처에 추가'}</button>
            )}
          </div>

          {toList && (
            <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              받는 사람:{' '}
              {(message.to_addrs ?? []).map((target, index) => (
                <span key={emailOf(target)}>
                  {index > 0 && ', '}
                  <span
                    title="클릭하면 주소 복사"
                    onClick={() => onCopyEmail(emailOf(target))}
                    style={{ cursor: 'pointer', borderRadius: '3px', padding: '0 2px' }}
                    onMouseEnter={(e) => {
                      (e.currentTarget as HTMLSpanElement).style.background = 'var(--color-bg-tertiary)';
                    }}
                    onMouseLeave={(e) => {
                      (e.currentTarget as HTMLSpanElement).style.background = 'transparent';
                    }}
                  >
                    {copiedEmail === emailOf(target) ? '복사됨 ✓' : (target.name ? `${target.name} <${emailOf(target)}>` : emailOf(target))}
                  </span>
                </span>
              ))}
            </div>
          )}
          {ccList && (
            <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              참조:{' '}
              {(message.cc_addrs ?? []).map((target, index) => (
                <span key={emailOf(target)}>
                  {index > 0 && ', '}
                  <span
                    title="클릭하면 주소 복사"
                    onClick={() => onCopyEmail(emailOf(target))}
                    style={{ cursor: 'pointer', borderRadius: '3px', padding: '0 2px' }}
                    onMouseEnter={(e) => {
                      (e.currentTarget as HTMLSpanElement).style.background = 'var(--color-bg-tertiary)';
                    }}
                    onMouseLeave={(e) => {
                      (e.currentTarget as HTMLSpanElement).style.background = 'transparent';
                    }}
                  >
                    {copiedEmail === emailOf(target) ? '복사됨 ✓' : (target.name ? `${target.name} <${emailOf(target)}>` : emailOf(target))}
                  </span>
                </span>
              ))}
            </div>
          )}
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: '2px', flexShrink: 0 }}>
          <span style={{ fontSize: '13px', color: 'var(--color-text-secondary)' }}>
            {formatFullDate(message.received_at)}
          </span>
          {(message.text_body || '').trim().length > 50 && (
            <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
              읽기 {readingTime(message.text_body || '')}
            </span>
          )}
        </div>
      </div>
    </>
  );
}
