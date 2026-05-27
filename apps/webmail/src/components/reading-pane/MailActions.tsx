'use client';

import { useCallback, useMemo, type CSSProperties } from 'react';
import { useTranslations } from 'next-intl';
import type { MessageDetail, Folder } from '@/lib/api';
import { emailOf } from '@/lib/message/messageUtils';
import {
  ArchiveBoxIcon,
  ArrowLeftIcon,
  ArrowTopRightOnSquareIcon,
  ArrowUturnLeftIcon,
  ArrowUturnRightIcon,
  ChevronDownIcon,
  ChevronUpIcon,
  MoonIcon,
  NoSymbolIcon,
  StarIcon,
  SunIcon,
} from '@heroicons/react/24/outline';
import { StarIcon as StarIconSolid } from '@heroicons/react/24/solid';
import { MailActionsMoreMenu } from './MailActionsMoreMenu';

interface MailActionsProps {
  message: MessageDetail;
  folders: Folder[];
  onBack?: () => void;
  onPrev?: () => void;
  onNext?: () => void;
  messageIndex?: number;
  messageTotal?: number;
  onReply?: () => void;
  onReplyAll?: () => void;
  onForward?: () => void;
  onMove?: (folderId: string) => void;
  onOpenInWindow?: () => void;
  onStar?: () => void;
  isStarred?: boolean;
  onArchive?: () => void;
  onPrint?: () => void;
  onToggleRead?: () => void;
  isRead?: boolean;
  onToggleThreadMute?: () => void;
  isThreadMuted?: boolean;
  onSnooze?: (messageId: string, until: Date) => void;
  onSpam?: () => void;
  onNotSpam?: () => void;
  onRestore?: () => void;
  unsubscribeUrl: string | null;
  onOpenInlineCompose: (intent: 'reply' | 'reply_all' | 'forward', to: string, subject: string) => void;
  fontSize: number;
  onIncreaseFontSize: () => void;
  onDecreaseFontSize: () => void;
  emailDarkMode?: boolean;
  onToggleEmailDark?: () => void;
}

const iconButtonStyle: CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  gap: '4px',
  cursor: 'pointer',
  borderRadius: '5px',
  border: '1px solid var(--color-border-default)',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  fontSize: '13px',
  transition: 'background 100ms ease, color 100ms ease',
};

export function MailActions({
  message,
  folders,
  onBack,
  onPrev,
  onNext,
  messageIndex,
  messageTotal,
  onReply,
  onReplyAll,
  onForward,
  onMove,
  onOpenInWindow,
  onStar,
  isStarred,
  onArchive,
  onPrint,
  onToggleRead,
  isRead = false,
  onToggleThreadMute,
  isThreadMuted = false,
  onSnooze,
  onSpam,
  onNotSpam,
  onRestore,
  unsubscribeUrl,
  onOpenInlineCompose,
  fontSize,
  onIncreaseFontSize,
  onDecreaseFontSize,
  emailDarkMode = false,
  onToggleEmailDark,
}: MailActionsProps) {
  const t = useTranslations('mail');

  const replyTargets = useMemo(() => {
    return {
      reply: message.from_addr,
      replyAll: [
        message.from_addr,
        ...(message.to_addrs ?? []).map(emailOf),
        ...(message.cc_addrs ?? []).map(emailOf),
      ].filter((address, index, array) => !!address && array.indexOf(address) === index),
    };
  }, [message.from_addr, message.to_addrs, message.cc_addrs]);

  const onQuickIntent = useCallback((intent: 'reply' | 'reply_all' | 'forward') => {
    const to = intent === 'reply'
      ? replyTargets.reply
      : intent === 'reply_all'
      ? replyTargets.replyAll.join(', ')
      : '';
    const subject = intent === 'forward'
      ? (message.subject?.startsWith('Fwd:') ? message.subject : `Fwd: ${message.subject ?? ''}`)
      : (message.subject?.startsWith('Re:') ? message.subject : `Re: ${message.subject ?? ''}`);
    onOpenInlineCompose(intent, to, subject);
  }, [message.subject, onOpenInlineCompose, replyTargets]);

  const quickActions = useMemo(
    () => [
      {
        icon: <ArrowUturnLeftIcon style={{ width: '16px', height: '16px' }} />,
        label: t('reply') + ' (R)',
        action: onReply,
        intent: 'reply' as const,
      },
      {
        icon: <ArrowUturnLeftIcon style={{ width: '16px', height: '16px', opacity: 0.7 }} />,
        label: t('replyAll') + ' (A)',
        action: onReplyAll,
        intent: 'reply_all' as const,
      },
      {
        icon: <ArrowUturnRightIcon style={{ width: '16px', height: '16px' }} />,
        label: t('forward') + ' (F)',
        action: onForward,
        intent: 'forward' as const,
      },
    ],
    [onForward, onReply, onReplyAll, t]
  );

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'flex-end',
        gap: '8px',
        padding: '12px 24px',
        borderBottom: '1px solid var(--color-border-subtle)',
        flexShrink: 0,
      }}
    >
      {onBack && (
        <button
          aria-label={t('back')}
          onClick={onBack}
          style={{ ...iconButtonStyle, marginRight: 'auto', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
        ><ArrowLeftIcon style={{ width: '16px', height: '16px' }} /> {t('back')}</button>
      )}
      {(onPrev || onNext) && !onBack && <div style={{ marginRight: 'auto' }} />}
      {onPrev && (
        <button
          aria-label={t('prevMessage')}
          title={`${t('prevMessage')} (k)`}
          onClick={onPrev}
          style={{ ...iconButtonStyle, border: 'none', padding: '5px' }}
          onMouseEnter={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
          }}
          onMouseLeave={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
          }}
        ><ChevronUpIcon style={{ width: '16px', height: '16px' }} /></button>
      )}
      {messageIndex !== undefined && messageTotal !== undefined && (
        <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', minWidth: '40px', textAlign: 'center' }}>
          {messageIndex + 1} / {messageTotal}
        </span>
      )}
      {onNext && (
        <button
          aria-label={t('nextMessage')}
          title={`${t('nextMessage')} (j)`}
          onClick={onNext}
          style={{ ...iconButtonStyle, border: 'none', padding: '5px' }}
          onMouseEnter={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
          }}
          onMouseLeave={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
          }}
        ><ChevronDownIcon style={{ width: '16px', height: '16px' }} /></button>
      )}

      {quickActions.map(({ icon, label, action, intent }) => action ? (
        <button
          key={label}
          aria-label={label}
          title={label}
          onClick={() => onQuickIntent(intent)}
          style={{ ...iconButtonStyle, padding: '5px 8px', border: 'none' }}
          onMouseEnter={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
          }}
          onMouseLeave={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
          }}
        >{icon}</button>
      ) : null)}

      <div style={{ width: '1px', height: '16px', background: 'var(--color-border-subtle)', margin: '0 2px' }} />

      {onStar && (
        <button
          aria-label={isStarred ? t('unstar') : t('star')}
          title={isStarred ? t('unstar') + ' (S)' : t('star') + ' (S)'}
          onClick={onStar}
          style={{
            ...iconButtonStyle,
            border: 'none',
            padding: '5px 8px',
            color: isStarred ? '#f59e0b' : 'var(--color-text-secondary)',
          }}
          onMouseEnter={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
          }}
          onMouseLeave={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
          }}
        >
          {isStarred ? (
            <StarIconSolid style={{ width: '16px', height: '16px' }} />
          ) : (
            <StarIcon style={{ width: '16px', height: '16px' }} />
          )}
        </button>
      )}

      {onArchive && (
        <button
          aria-label={t('archive')}
          title={`${t('archive')} (E)`}
          onClick={onArchive}
          style={{ ...iconButtonStyle, border: 'none', padding: '5px 8px' }}
          onMouseEnter={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
          }}
          onMouseLeave={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
          }}
        ><ArchiveBoxIcon style={{ width: '16px', height: '16px' }} /></button>
      )}

      {(() => {
        if (!message.html_body) return null;
        const match = message.html_body.match(/href=["']([^"']*(?:unsubscribe|opt.?out|수신거부|구독취소)[^"']*)["']/i);
        if (!match) return null;
        const url = match[1].replace(/&amp;/g, '&');
        return (
          <button
            aria-label={t('unsubscribe')}
            title={t('unsubscribe')}
            onClick={() => window.open(url, '_blank', 'noopener,noreferrer')}
            style={{
              ...iconButtonStyle,
              border: '1px solid rgba(220,38,38,0.3)',
              padding: '4px 10px',
              color: 'var(--color-destructive)',
              fontSize: '12px',
              fontWeight: 500,
              background: 'rgba(220,38,38,0.04)',
            }}
            onMouseEnter={(e) => {
              (e.currentTarget as HTMLButtonElement).style.background = 'rgba(220,38,38,0.1)';
            }}
            onMouseLeave={(e) => {
              (e.currentTarget as HTMLButtonElement).style.background = 'rgba(220,38,38,0.04)';
            }}
          ><NoSymbolIcon style={{ width: 13, height: 13 }} /> {t('unsubscribe')}</button>
        );
      })()}

      {onOpenInWindow && (
        <button
          aria-label={t('openInWindow')}
          title={t('openInWindow')}
          onClick={onOpenInWindow}
          style={{ ...iconButtonStyle, border: 'none', padding: '5px 8px' }}
          onMouseEnter={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
          }}
          onMouseLeave={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
          }}
        ><ArrowTopRightOnSquareIcon style={{ width: '16px', height: '16px' }} /></button>
      )}

      {onToggleEmailDark && message.html_body && (
        <button
          aria-label={emailDarkMode ? t('emailLightMode') : t('emailDarkMode')}
          title={emailDarkMode ? t('emailLightMode') : t('emailDarkMode')}
          onClick={onToggleEmailDark}
          style={{
            ...iconButtonStyle,
            border: 'none',
            padding: '5px 8px',
            color: emailDarkMode ? 'var(--color-accent)' : 'var(--color-text-secondary)',
          }}
          onMouseEnter={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
          }}
          onMouseLeave={(e) => {
            (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
          }}
        >
          {emailDarkMode ? <SunIcon style={{ width: '16px', height: '16px' }} /> : <MoonIcon style={{ width: '16px', height: '16px' }} />}
        </button>
      )}

      <MailActionsMoreMenu
        message={message}
        folders={folders}
        onMove={onMove}
        onSnooze={onSnooze}
        onPrint={onPrint}
        onToggleRead={onToggleRead}
        isRead={isRead}
        onToggleThreadMute={onToggleThreadMute}
        isThreadMuted={isThreadMuted}
        onSpam={onSpam}
        onNotSpam={onNotSpam}
        onRestore={onRestore}
        unsubscribeUrl={unsubscribeUrl}
        fontSize={fontSize}
        onIncreaseFontSize={onIncreaseFontSize}
        onDecreaseFontSize={onDecreaseFontSize}
      />
    </div>
  );
}
