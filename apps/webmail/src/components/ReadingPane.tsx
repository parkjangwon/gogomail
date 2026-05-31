'use client';

import { useCallback, useEffect, useMemo } from 'react';
import { useTranslations } from 'next-intl';
import {
  MessageDetail,
  MessageSummary,
  Folder,
} from '@/lib/api';
import { emailOf, linkify } from '@/lib/message/messageUtils';
import { MailActions } from './reading-pane/MailActions';
import { MessageHeader } from './reading-pane/MessageHeader';
import { DeliveryTrackingPanels } from './reading-pane/DeliveryTrackingPanels';
import { CalendarInvites } from './reading-pane/CalendarInvites';
import { AttachmentPanel } from './reading-pane/AttachmentPanel';
import { QuickReplyPanel } from './reading-pane/QuickReplyPanel';
import { ThreadConversation } from './reading-pane/ThreadConversation';
import { InlineCompose } from './reading-pane/InlineCompose';
import { SafeHTMLBody } from './reading-pane/SafeHTMLBody';
import { ReadingPaneOverlays } from './reading-pane/ReadingPaneOverlays';
import { ReadingPaneLoading, ReadingPaneEmpty } from './reading-pane/ReadingPaneStatus';
import { useReadingPaneAttachments } from './reading-pane/useReadingPaneAttachments';
import { useReadingPaneMedia } from './reading-pane/useReadingPaneMedia';
import { useReadingPaneCalendar } from './reading-pane/useReadingPaneCalendar';
import { useReadingPane } from './reading-pane/useReadingPane';
import { useReadingPaneKeyboard } from './reading-pane/useReadingPaneKeyboard';
import { ignoreNonCritical } from '@/lib/promise';

interface ReadingPaneProps {
  message: MessageDetail | null;
  folders?: Folder[];
  onArchive?: () => void;
  onSpam?: () => void;
  onNotSpam?: () => void;
  onDelete?: () => void;
  onReply?: () => void;
  onReplyAll?: () => void;
  onForward?: () => void;
  onMove?: (folderId: string) => void;
  onPrint?: () => void;
  loading?: boolean;
  onBack?: () => void;
  onQuickReply?: (body: string) => Promise<void>;
  onPrev?: () => void;
  onNext?: () => void;
  messageIndex?: number;
  messageTotal?: number;
  onComposeToAddress?: (address: string) => void;
  onBlockSender?: (addr: string) => void;
  onRestore?: () => void;
  onSnooze?: (messageId: string, until: Date) => void;
  onOpenInWindow?: () => void;
  onToggleRead?: () => void;
  isRead?: boolean;
  onStar?: () => void;
  isStarred?: boolean;
  onToggleThreadMute?: () => void;
  isThreadMuted?: boolean;
  threadMessages?: MessageSummary[];
  onSelectThread?: (id: string) => void;
  userEmail?: string;
  externalImages?: string;
}

export function ReadingPane({
  message,
  folders = [],
  onArchive,
  onSpam,
  onNotSpam,
  onDelete,
  onReply,
  onReplyAll,
  onForward,
  onMove,
  onPrint,
  loading,
  onBack,
  onQuickReply,
  onPrev,
  onNext,
  messageIndex,
  messageTotal,
  onComposeToAddress,
  onBlockSender,
  onRestore,
  onSnooze,
  onOpenInWindow,
  onToggleRead,
  isRead,
  onStar,
  isStarred,
  onToggleThreadMute,
  isThreadMuted,
  threadMessages,
  onSelectThread,
  userEmail,
  externalImages = 'ask',
}: ReadingPaneProps) {
  const t = useTranslations();
  const {
    fontSize,
    savedContact,
    setSavedContact,
    scrollProgress,
    emailDarkMode,
    setEmailDarkMode,
    copiedEmail,
    setCopiedEmail,
    inlineCompose,
    setInlineCompose,
    scrollContainerRef,
    copyTimerRef,
    increaseFontSize,
    decreaseFontSize,
    handleReadingScroll,
    handleSaveContact,
  } = useReadingPane({ message });

  const folderSystemType = folders?.find((f) => f.id === message?.folder_id)?.system_type;

  const {
    attachments,
    attachmentsLoading,
    downloadingId,
    savingToDriveId,
    driveToast,
    handleDownload,
    handleSaveToDrive,
  } = useReadingPaneAttachments({
    messageId: message?.id,
    hasAttachment: message?.has_attachment,
    t,
  });

  const {
    imagePreviews,
    lightbox,
    setLightbox,
    pdfPreview,
    setPdfPreview,
    pdfPreviewLoadingId,
    onOpenImage,
    handlePdfPreview,
  } = useReadingPaneMedia({
    messageId: message?.id,
    attachments,
  });

  const {
    icsEvents,
    addingCalendarId,
    calendarAdded,
    deliveryStatus,
    deliveryOpen,
    setDeliveryOpen,
    trackingEvents,
    trackingOpen,
    setTrackingOpen,
    handleAddToCalendar,
  } = useReadingPaneCalendar({
    messageId: message?.id,
    fromAddr: message?.from_addr,
    userEmail,
    folderId: message?.folder_id,
    folderSystemType,
    attachments,
  });

  useEffect(() => {
    const el = scrollContainerRef.current;
    if (!el) return;
    el.scrollTo({ top: 0 });
    window.requestAnimationFrame(() => el.focus({ preventScroll: true }));
  }, [message?.id]);

  const isContactSaved = useMemo(() => {
    if (!message) return false;
    try {
      const contacts: Record<string, string> = JSON.parse(localStorage.getItem('webmail_contacts') ?? '{}');
      return message.from_addr.toLowerCase() in contacts;
    } catch {
      return false;
    }
  }, [message, savedContact]);

  const unsubscribeUrl = useMemo(() => {
    if (!message) return null;
    if (message.html_body) {
      try {
        const doc = new DOMParser().parseFromString(message.html_body, 'text/html');
        const anchor = Array.from(doc.querySelectorAll('a')).find((a) =>
          /unsubscribe|opt.?out|수신\s*거부/i.test(a.textContent ?? '') ||
          /unsubscribe|optout/i.test(a.getAttribute('href') ?? ''),
        );
        if (anchor?.href) return anchor.href;
      } catch {
        // ignore
      }
    }
    if (message.text_body) {
      const m = message.text_body.match(/https?:\/\/[^\s<>"']*unsubscribe[^\s<>"']*/i);
      if (m) return m[0];
    }
    return null;
  }, [message?.id, message?.html_body, message?.text_body]);

  const isSent = (userEmail && message?.from_addr
    ? message.from_addr.toLowerCase() === userEmail.toLowerCase()
    : false) && folders?.find((f) => f.id === message?.folder_id)?.system_type === 'sent';

  const copyEmail = useCallback((address: string) => {
    ignoreNonCritical(navigator.clipboard.writeText(address), 'readingPane.email.copy');
    setCopiedEmail(address);
    if (copyTimerRef.current) {
      clearTimeout(copyTimerRef.current);
    }
    copyTimerRef.current = setTimeout(() => setCopiedEmail(''), 2000);
  }, []);

  const openInlineCompose = (intent: 'reply' | 'reply_all' | 'forward', to: string, subject: string) => {
    setInlineCompose({ intent, to, subject });
    setTimeout(() => {
      scrollContainerRef.current?.scrollTo({
        top: scrollContainerRef.current.scrollHeight,
        behavior: 'smooth',
      });
    }, 50);
  };

  const onOpenFullCompose = useCallback((intent: 'reply' | 'reply_all' | 'forward') => {
    setInlineCompose(null);
    const action = intent === 'reply'
      ? onReply
      : intent === 'reply_all'
      ? onReplyAll
      : onForward;
    action?.();
  }, [onReply, onReplyAll, onForward, setInlineCompose]);

  const { handleReadingPaneKeyDown } = useReadingPaneKeyboard({
    scrollContainerRef,
    onBack,
    onDelete,
    onStar,
    onArchive,
    onToggleRead,
    onOpenFullCompose,
  });

  if (loading) return <ReadingPaneLoading />;
  if (!message) return <ReadingPaneEmpty />;

  const toList = (message.to_addrs ?? [])
    .map((address) => {
      const email = emailOf(address);
      return address.name ? `${address.name} <${email}>` : email;
    })
    .join(', ');
  const ccList = (message.cc_addrs ?? [])
    .map((address) => {
      const email = emailOf(address);
      return address.name ? `${address.name} <${email}>` : email;
    })
    .join(', ');
  const visibleThread = threadMessages?.filter((item) => item.id) ?? [];

  return (
    <main
      ref={scrollContainerRef}
      aria-label={t('misc.readingPane.region')}
      data-print-reading-pane
      tabIndex={0}
      data-nav-group="reading-pane"
      onKeyDown={handleReadingPaneKeyDown}
      onScroll={handleReadingScroll}
      style={{
        flex: 1,
        minWidth: 0,
        height: '100%',
        overflowY: 'auto',
        background: 'var(--color-bg-primary)',
        display: 'flex',
        flexDirection: 'column',
        position: 'relative',
      }}
    >
      <div
        aria-hidden="true"
        style={{
          position: 'sticky',
          top: 0,
          left: 0,
          height: '2px',
          width: `${scrollProgress}%`,
          background: 'var(--color-accent)',
          zIndex: 10,
          transition: 'width 80ms linear',
          flexShrink: 0,
          marginBottom: '-2px',
        }}
      />

      <MailActions
        message={message}
        folders={folders}
        onBack={onBack}
        onPrev={onPrev}
        onNext={onNext}
        messageIndex={messageIndex}
        messageTotal={messageTotal}
        onReply={onReply}
        onReplyAll={onReplyAll}
        onForward={onForward}
        onMove={onMove}
        onOpenInWindow={onOpenInWindow}
        onStar={onStar}
        isStarred={isStarred}
        onArchive={onArchive}
        onPrint={onPrint}
        onToggleRead={onToggleRead}
        isRead={isRead}
        onToggleThreadMute={onToggleThreadMute}
        isThreadMuted={isThreadMuted}
        onSnooze={onSnooze}
        onSpam={onSpam}
        onNotSpam={onNotSpam}
        onRestore={onRestore}
        unsubscribeUrl={unsubscribeUrl}
        onOpenInlineCompose={openInlineCompose}
        fontSize={fontSize}
        onIncreaseFontSize={increaseFontSize}
        onDecreaseFontSize={decreaseFontSize}
        emailDarkMode={emailDarkMode}
        onToggleEmailDark={() => setEmailDarkMode((v) => !v)}
      />

      {/* "스팸 아님" banner — shown when viewing email in spam folder */}
      {onNotSpam && (
        <div style={{
          display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px',
          padding: '10px 20px',
          background: 'color-mix(in srgb, var(--color-warning) 12%, transparent)',
          borderBottom: '1px solid color-mix(in srgb, var(--color-warning) 30%, transparent)',
          flexShrink: 0,
        }}>
          <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', lineHeight: 1.4 }}>
            {t('readingFull.notSpamBannerText')}
          </span>
          <button
            onClick={onNotSpam}
            style={{
              padding: '5px 14px', borderRadius: '6px', border: 'none',
              background: 'var(--color-accent)', color: '#fff',
              fontSize: '12px', fontWeight: 600, cursor: 'pointer', flexShrink: 0, whiteSpace: 'nowrap',
            }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '0.88'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
          >
            {t('readingFull.notSpamBannerAction')}
          </button>
        </div>
      )}

      <div style={{ padding: '20px 24px', flex: 1 }}>
        <MessageHeader
          message={message}
          toList={toList}
          ccList={ccList}
          copiedEmail={copiedEmail}
          onCopyEmail={copyEmail}
          onBlockSender={onBlockSender}
          isContactSaved={isContactSaved}
          savedContact={savedContact}
          onSaveContact={handleSaveContact}
        />

        <hr
          style={{
            border: 'none',
            borderTop: '1px solid var(--color-border-subtle)',
            margin: '16px 0',
          }}
        />

        <DeliveryTrackingPanels
          isSent={isSent}
          deliveryStatus={deliveryStatus}
          deliveryOpen={deliveryOpen}
          setDeliveryOpen={setDeliveryOpen}
          trackingEvents={trackingEvents}
          trackingOpen={trackingOpen}
          setTrackingOpen={setTrackingOpen}
        />

        <CalendarInvites
          events={icsEvents}
          onAddToCalendar={handleAddToCalendar}
          addingCalendarId={addingCalendarId}
          calendarAdded={calendarAdded}
        />

        <AttachmentPanel
          attachments={attachments}
          hasAttachment={message.has_attachment}
          attachmentsLoading={attachmentsLoading}
          downloadingId={downloadingId}
          pdfPreviewLoadingId={pdfPreviewLoadingId}
          savingToDriveId={savingToDriveId}
          imagePreviews={imagePreviews}
          onDownload={handleDownload}
          onPdfPreview={handlePdfPreview}
          onSaveToDrive={handleSaveToDrive}
          onOpenImage={onOpenImage}
        />

        <div
          style={{
            maxWidth: '680px',
            fontSize: `${fontSize}px`,
            lineHeight: 1.6,
            color: 'var(--color-text-primary)',
            ...(emailDarkMode
              ? {
                  filter: 'invert(1) hue-rotate(180deg)',
                  background: '#000',
                  borderRadius: '8px',
                  padding: '12px',
                }
              : {}),
          }}
        >
          {message.html_body ? (
            <SafeHTMLBody html={message.html_body} onMailto={onComposeToAddress} externalImages={externalImages} />
          ) : (
            <pre
              style={{
                fontFamily: 'inherit',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
                margin: 0,
              }}
            >
              {linkify(message.text_body || t('misc.readingPane.noContent'))}
            </pre>
          )}
        </div>

        {visibleThread.length > 1 && (
          <ThreadConversation
            messages={visibleThread}
            currentMessageId={message.id}
            userEmail={userEmail}
            onSelectThread={onSelectThread || (() => {})}
          />
        )}

        {onQuickReply && (
          <QuickReplyPanel
            message={message}
            onQuickReply={(body) => onQuickReply(body)}
          />
        )}

        {inlineCompose && (
          <InlineCompose
            intent={inlineCompose.intent}
            to={inlineCompose.to}
            subject={inlineCompose.subject}
            messageId={message.id}
            sourceText={message.text_body}
            onClose={() => setInlineCompose(null)}
            onOpenFullModal={() => onOpenFullCompose(inlineCompose.intent)}
            userEmail={userEmail}
          />
        )}
      </div>

      <ReadingPaneOverlays
        driveToast={driveToast}
        pdfPreview={pdfPreview}
        setPdfPreview={setPdfPreview}
        attachments={attachments}
        onDownloadAttachment={handleDownload}
        lightbox={lightbox}
        setLightbox={setLightbox}
      />
    </main>
  );
}
