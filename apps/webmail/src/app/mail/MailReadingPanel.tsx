'use client';

import React from 'react';
import { useTranslations } from 'next-intl';
import {
  MessageSummary,
  Folder,
  sendMessage,
} from '@/lib/api';
import { ReadingPane } from '@/components/ReadingPane';
import { useMessage } from '@/hooks/useMessage';
import { type ToastItem } from '@/components/Toast';
import { type ComposeContext } from './useMailCompose';

type ComposeOpts = ComposeContext;

interface MailReadingPanelProps {
  selectedMessageId: string | null;
  selectedMessage: ReturnType<typeof useMessage>['message'];
  messages: MessageSummary[];
  searchResults: MessageSummary[] | null;
  isMobile: boolean;
  readingPaneWidth: number;
  setReadingPaneWidth: (w: number) => void;
  messageLoading: boolean;
  folders: Folder[];
  activeFolderSystemType: string | null | undefined;
  wmSettings: { externalImages: string };
  swipeDeltaX: number;
  setSwipeDeltaX: (v: number) => void;
  swipeTouchStartRef: React.MutableRefObject<number | null>;
  onClose: () => void;
  onSelectMessage: (id: string) => void;
  onOpenCompose: (opts: ComposeOpts) => void;
  // Reading pane action props
  onArchive?: () => void;
  onSpam?: () => void;
  onNotSpam?: () => void;
  onDelete: () => void;
  onMove: (folderId: string) => void;
  onPrint?: () => void;
  onRestore?: () => void;
  onComposeToAddress: (addr: string) => void;
  onBlockSender: (addr: string) => void;
  onSnooze?: (id: string, until: Date) => void;
  onToggleRead?: () => void;
  isRead?: boolean;
  onStar?: () => void;
  isStarred?: boolean;
  onToggleThreadMute?: () => void;
  isThreadMuted?: boolean;
  threadMessages?: MessageSummary[];
  onSelectThread: (id: string) => void;
  userEmail?: string;
  addToast: (msg: string, type?: ToastItem['type']) => void;
}

export function MailReadingPanel({
  selectedMessageId,
  selectedMessage,
  messages,
  searchResults,
  isMobile,
  readingPaneWidth,
  setReadingPaneWidth,
  messageLoading,
  folders,
  activeFolderSystemType,
  wmSettings,
  swipeDeltaX,
  setSwipeDeltaX,
  swipeTouchStartRef,
  onClose,
  onSelectMessage,
  onOpenCompose,
  onArchive,
  onSpam,
  onNotSpam,
  onDelete,
  onMove,
  onPrint,
  onRestore,
  onComposeToAddress,
  onBlockSender,
  onSnooze,
  onToggleRead,
  isRead,
  onStar,
  isStarred,
  onToggleThreadMute,
  isThreadMuted,
  threadMessages,
  onSelectThread,
  userEmail,
  addToast,
}: MailReadingPanelProps) {
  const t = useTranslations('misc');

  const msgList = searchResults ?? messages;
  const curIdx = msgList.findIndex((m) => m.id === selectedMessageId);
  const prevId = curIdx > 0 ? msgList[curIdx - 1].id : null;
  const nextId = curIdx !== -1 && curIdx < msgList.length - 1 ? msgList[curIdx + 1].id : null;
  const panelOpen = !!selectedMessageId;

  return (
    <>
      {/* backdrop — semi-transparent, click closes panel */}
      <div
        aria-hidden="true"
        onClick={onClose}
        style={{
          position: 'fixed', inset: 0, zIndex: 49,
          background: 'rgba(0,0,0,0.15)',
          opacity: panelOpen ? 1 : 0,
          pointerEvents: panelOpen ? 'auto' : 'none',
          transition: 'opacity 200ms ease',
        }}
      />
      <div
        role="region"
        aria-label={t('mailPage.readingRegion')}
        onTouchStart={isMobile ? (e) => { swipeTouchStartRef.current = e.touches[0].clientX; } : undefined}
        onTouchMove={isMobile ? (e) => {
          if (swipeTouchStartRef.current === null) return;
          const delta = e.touches[0].clientX - swipeTouchStartRef.current;
          if (delta > 0) setSwipeDeltaX(delta);
        } : undefined}
        onTouchEnd={isMobile ? () => {
          if (swipeDeltaX > 80) onClose();
          setSwipeDeltaX(0);
          swipeTouchStartRef.current = null;
        } : undefined}
        style={{
          position: 'fixed',
          top: 0,
          right: 0,
          height: '100dvh',
          width: isMobile ? '100%' : readingPaneWidth > 0 ? `${readingPaneWidth}px` : 'min(720px, 55vw)',
          transform: panelOpen
            ? (isMobile && swipeDeltaX > 0 ? `translateX(${swipeDeltaX}px)` : 'translateX(0)')
            : 'translateX(100%)',
          transition: swipeDeltaX > 0 ? 'none' : 'transform 220ms cubic-bezier(0.4,0,0.2,1)',
          zIndex: 50,
          display: 'flex',
          flexDirection: 'column',
          background: 'var(--color-bg-primary)',
          borderLeft: isMobile ? 'none' : '1px solid var(--color-border-default)',
          boxShadow: panelOpen ? '-8px 0 32px rgba(0,0,0,0.12)' : 'none',
        }}
      >
        {/* Resize handle — left edge */}
        {!isMobile && panelOpen && (
          <div
            aria-hidden="true"
            style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: '5px', cursor: 'col-resize', zIndex: 10, transition: 'background 150ms ease' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-accent)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'transparent'; }}
            onMouseDown={(e) => {
              e.preventDefault();
              const startX = e.clientX;
              const startW = readingPaneWidth > 0 ? readingPaneWidth : Math.min(720, window.innerWidth * 0.55);
              let lastW = startW;
              const onMove = (ev: MouseEvent) => {
                lastW = Math.min(window.innerWidth - 300, Math.max(380, startW - (ev.clientX - startX)));
                setReadingPaneWidth(lastW);
              };
              const onUp = () => {
                document.removeEventListener('mousemove', onMove);
                document.removeEventListener('mouseup', onUp);
                try { localStorage.setItem('webmail_reading_pane_width', String(lastW)); } catch { /* */ }
              };
              document.addEventListener('mousemove', onMove);
              document.addEventListener('mouseup', onUp);
            }}
          />
        )}
        <ReadingPane
          message={selectedMessage}
          folders={folders}
          onArchive={onArchive}
          onSpam={onSpam}
          onNotSpam={onNotSpam}
          onDelete={onDelete}
          onReply={() => selectedMessage && onOpenCompose({ intent: 'reply', source: selectedMessage })}
          onReplyAll={() => selectedMessage && onOpenCompose({ intent: 'reply_all', source: selectedMessage })}
          onForward={() => selectedMessage && onOpenCompose({ intent: 'forward', source: selectedMessage })}
          onMove={onMove}
          onPrint={onPrint}
          loading={messageLoading}
          onBack={onClose}
          onPrev={prevId ? () => onSelectMessage(prevId) : undefined}
          onNext={nextId ? () => onSelectMessage(nextId) : undefined}
          messageIndex={curIdx >= 0 ? curIdx : undefined}
          messageTotal={curIdx >= 0 ? msgList.length : undefined}
          onQuickReply={selectedMessage ? async (body) => {
            await sendMessage({
              to: [{ address: selectedMessage.from_addr, name: selectedMessage.from_name || undefined }],
              subject: `Re: ${selectedMessage.subject || ''}`,
              text_body: body,
              intent: 'reply',
              source_message_id: selectedMessage.id,
            });
            addToast(t('mailPage.replySent'));
          } : undefined}
          onRestore={onRestore}
          onComposeToAddress={onComposeToAddress}
          onBlockSender={onBlockSender}
          onSnooze={onSnooze}
          onOpenInWindow={selectedMessageId ? () => window.open(`/mail/${selectedMessageId}`, '_blank', 'width=900,height=700,menubar=no,toolbar=no') : undefined}
          onToggleRead={onToggleRead}
          isRead={isRead}
          onStar={onStar}
          isStarred={isStarred}
          onToggleThreadMute={onToggleThreadMute}
          isThreadMuted={isThreadMuted}
          threadMessages={threadMessages}
          onSelectThread={onSelectThread}
          userEmail={userEmail}
          externalImages={wmSettings.externalImages}
        />
      </div>
    </>
  );
}
