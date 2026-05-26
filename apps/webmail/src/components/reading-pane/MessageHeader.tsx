'use client';

import { useTranslations } from 'next-intl';
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
  onBlockSender?: (addr: string) => void;
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
  onBlockSender,
}: MessageHeaderProps) {
  const t = useTranslations('readingFull');
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
        {message.subject || t('header.noSubject')}
      </h1>

      <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px', marginBottom: '8px' }}>
        <div>
          <div style={{ fontSize: '14px', fontWeight: 500, color: 'var(--color-text-primary)' }}>
            <span
              title={t('header.copyTitle')}
              onClick={() => onCopyEmail(message.from_addr)}
              style={{ cursor: 'pointer', borderRadius: '3px', padding: '0 2px' }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLSpanElement).style.background = 'var(--color-bg-secondary)';
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLSpanElement).style.background = 'transparent';
              }}
            >
              {copiedEmail === message.from_addr ? t('header.copiedSuffix') : (message.from_name || message.from_addr)}
            </span>
            {message.from_name && (
              <span
                title={t('header.copyTitle')}
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
                title={t('header.composeToTitle', { addr: message.from_addr })}
                style={{
                  background: 'var(--color-accent)',
                  border: 'none',
                  borderRadius: '5px',
                  cursor: 'pointer',
                  fontSize: '12px',
                  fontWeight: 500,
                  color: '#fff',
                  padding: '3px 10px',
                  marginInlineStart: '8px',
                  lineHeight: 1.5,
                  transition: 'opacity 100ms ease',
                }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.opacity = '0.85';
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.opacity = '1';
                }}
              >{t('header.composeToLabel')}</button>
            )}
            {!isContactSaved && (
              <button
                onClick={onSaveContact}
                title={t('header.addContactTitle')}
                style={{
                  background: 'none',
                  border: '1px solid var(--color-border-default)',
                  borderRadius: '5px',
                  cursor: 'pointer',
                  fontSize: '12px',
                  fontWeight: 500,
                  color: savedContact ? 'var(--color-accent)' : 'var(--color-text-secondary)',
                  padding: '3px 10px',
                  marginInlineStart: '6px',
                  lineHeight: 1.5,
                  transition: 'background 100ms ease',
                }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
                }}
              >{savedContact ? t('header.savedContactLabel') : t('header.addContactLabel')}</button>
            )}
            {onBlockSender && (
              <button
                onClick={() => onBlockSender(message.from_addr)}
                title={t('header.blockSenderTitle', { addr: message.from_addr })}
                style={{
                  background: 'color-mix(in srgb, var(--color-destructive) 8%, transparent)',
                  border: '1px solid color-mix(in srgb, var(--color-destructive) 35%, transparent)',
                  borderRadius: '5px',
                  cursor: 'pointer',
                  fontSize: '12px',
                  fontWeight: 500,
                  color: 'var(--color-destructive)',
                  padding: '3px 10px',
                  marginInlineStart: '6px',
                  lineHeight: 1.5,
                  transition: 'background 100ms ease',
                }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'color-mix(in srgb, var(--color-destructive) 15%, transparent)';
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'color-mix(in srgb, var(--color-destructive) 8%, transparent)';
                }}
              >{t('header.blockSenderLabel')}</button>
            )}
          </div>

          {toList && (
            <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              {t('header.toLabel')}:{' '}
              {(message.to_addrs ?? []).map((target, index) => (
                <span key={emailOf(target)}>
                  {index > 0 && ', '}
                  <span
                    title={t('header.copyTitle')}
                    onClick={() => onCopyEmail(emailOf(target))}
                    style={{ cursor: 'pointer', borderRadius: '3px', padding: '0 2px' }}
                    onMouseEnter={(e) => {
                      (e.currentTarget as HTMLSpanElement).style.background = 'var(--color-bg-tertiary)';
                    }}
                    onMouseLeave={(e) => {
                      (e.currentTarget as HTMLSpanElement).style.background = 'transparent';
                    }}
                  >
                    {copiedEmail === emailOf(target) ? t('header.copiedSuffix') : (target.name ? `${target.name} <${emailOf(target)}>` : emailOf(target))}
                  </span>
                </span>
              ))}
            </div>
          )}
          {ccList && (
            <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
              {t('header.ccLabel')}:{' '}
              {(message.cc_addrs ?? []).map((target, index) => (
                <span key={emailOf(target)}>
                  {index > 0 && ', '}
                  <span
                    title={t('header.copyTitle')}
                    onClick={() => onCopyEmail(emailOf(target))}
                    style={{ cursor: 'pointer', borderRadius: '3px', padding: '0 2px' }}
                    onMouseEnter={(e) => {
                      (e.currentTarget as HTMLSpanElement).style.background = 'var(--color-bg-tertiary)';
                    }}
                    onMouseLeave={(e) => {
                      (e.currentTarget as HTMLSpanElement).style.background = 'transparent';
                    }}
                  >
                    {copiedEmail === emailOf(target) ? t('header.copiedSuffix') : (target.name ? `${target.name} <${emailOf(target)}>` : emailOf(target))}
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
              {t('header.readingPrefix')} {readingTime(message.text_body || '')}
            </span>
          )}
        </div>
      </div>
    </>
  );
}
