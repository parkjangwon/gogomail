'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type KeyboardEvent as ReactKeyboardEvent } from 'react';
import { useTranslations } from 'next-intl';
import {
  Attachment,
  MessageDeliveryStatus,
  MessageDetail,
  MessageSummary,
  Folder,
  TrackingEvent,
  createCalendarEvent,
  downloadAttachment,
  getMessageDeliveryStatus,
  getMessageTracking,
  listAttachments,
  listCalendars,
  saveAttachmentToDrive,
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
import { ICSEvent } from './reading-pane/readingPaneTypes';
import { SafeHTMLBody } from './reading-pane/SafeHTMLBody';
import { ReadingPaneOverlays } from './reading-pane/ReadingPaneOverlays';

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
  const [fontSize, setFontSize] = useState(() => {
    try { return parseInt(localStorage.getItem('webmail_font_size') ?? '14', 10) || 14; } catch { return 14; }
  });
  const [savedContact, setSavedContact] = useState(false);
  const [scrollProgress, setScrollProgress] = useState(0);
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [attachmentsLoading, setAttachmentsLoading] = useState(false);
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const [deliveryStatus, setDeliveryStatus] = useState<MessageDeliveryStatus | null>(null);
  const [deliveryOpen, setDeliveryOpen] = useState(false);
  const [icsEvents, setIcsEvents] = useState<ICSEvent[]>([]);
  const [addingCalendarId, setAddingCalendarId] = useState<string | null>(null);
  const [calendarAdded, setCalendarAdded] = useState<string | null>(null);
  const [trackingEvents, setTrackingEvents] = useState<TrackingEvent[] | null>(null);
  const [trackingOpen, setTrackingOpen] = useState(false);
  const [savingToDriveId, setSavingToDriveId] = useState<string | null>(null);
  const [driveToast, setDriveToast] = useState('');
  const [imagePreviews, setImagePreviews] = useState<Record<string, string>>({});
  const [lightbox, setLightbox] = useState<{ url: string; filename: string; attId: string } | null>(null);
  const [pdfPreview, setPdfPreview] = useState<{ url: string; filename: string } | null>(null);
  const [pdfPreviewLoadingId, setPdfPreviewLoadingId] = useState<string | null>(null);
  const [emailDarkMode, setEmailDarkMode] = useState(false);
  const [copiedEmail, setCopiedEmail] = useState('');
  const [inlineCompose, setInlineCompose] = useState<{
    intent: 'reply' | 'reply_all' | 'forward';
    to: string;
    subject: string;
  } | null>(null);

  const scrollContainerRef = useRef<HTMLElement>(null);
  const imagePreviewsRef = useRef<Record<string, string>>({});
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    localStorage.setItem('webmail_font_size', String(fontSize));
  }, [fontSize]);

  useEffect(() => {
    const el = scrollContainerRef.current;
    if (!el) return;
    el.scrollTo({ top: 0 });
    window.requestAnimationFrame(() => el.focus({ preventScroll: true }));
  }, [message?.id]);

  useEffect(() => {
    setInlineCompose(null);
  }, [message?.id]);

  useEffect(() => {
    if (!message?.has_attachment || !message.id) {
      setAttachments([]);
      return;
    }

    setAttachmentsLoading(true);
    listAttachments(message.id)
      .then((result) => setAttachments(result))
      .catch(() => setAttachments([]))
      .finally(() => setAttachmentsLoading(false));
  }, [message?.id, message?.has_attachment]);

  useEffect(() => {
    if (attachments.length === 0) {
      setIcsEvents([]);
      return;
    }
    const icsAtts = attachments.filter((a) => a.filename.toLowerCase().endsWith('.ics') || a.mime_type === 'text/calendar');
    if (icsAtts.length === 0) {
      setIcsEvents([]);
      return;
    }
    Promise.all(
      icsAtts.map(async (att) => {
        if (!message) return null;
        try {
          const resp = await fetch(`/api/mail/messages/${message.id}/attachments/${att.id}/download`);
          if (!resp.ok) return null;
          const text = await resp.text();
          const get = (key: string) => {
            const m = text.match(new RegExp(`^${key}[;:][^:]*:?(.+)$`, 'mi'));
            return m ? m[1].trim() : undefined;
          };
          const summary = get('SUMMARY');
          const dtstart = get('DTSTART');
          if (!summary || !dtstart) return null;
          return {
            summary,
            dtstart,
            dtend: get('DTEND'),
            location: get('LOCATION'),
            description: get('DESCRIPTION'),
          } as ICSEvent;
        } catch {
          return null;
        }
      }),
    ).then((results) => {
      setIcsEvents(results.filter(Boolean) as ICSEvent[]);
    });
  }, [attachments, message]);

  useEffect(() => {
    // Delivery tracking is only meaningful when viewing an outgoing message
    // from the Sent folder. Avoid showing it for self-sent emails sitting in
    // the inbox — the sender/recipient coincidence makes isSent=true even though
    // the user is reading it as a recipient, not as the original sender.
    const senderMatch = message?.from_addr && userEmail
      ? message.from_addr.toLowerCase() === userEmail.toLowerCase()
      : false;
    const folderSystemType = folders?.find((f) => f.id === message?.folder_id)?.system_type;
    const isSentView = senderMatch && folderSystemType === 'sent';

    setDeliveryStatus(null);
    setDeliveryOpen(false);
    setTrackingEvents(null);
    setTrackingOpen(false);

    if (!message?.id || !isSentView) return;

    getMessageDeliveryStatus(message.id)
      .then(setDeliveryStatus)
      .catch(() => {});
    getMessageTracking(message.id)
      .then((events) => {
        if (events.length > 0) {
          setTrackingEvents(events);
        }
      })
      .catch(() => {});
  }, [message?.id, message?.from_addr, userEmail]);

  useEffect(() => {
    if (!lightbox) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setLightbox(null);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [lightbox]);

  useEffect(() => {
    if (!pdfPreview) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setPdfPreview(null);
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [pdfPreview]);

  useEffect(() => {
    const url = pdfPreview?.url;
    return () => {
      if (url) URL.revokeObjectURL(url);
    };
  }, [pdfPreview]);

  useEffect(() => {
    if (!message || attachments.length === 0) return;
    const imageAttachments = attachments.filter((a) => a.mime_type.startsWith('image/') && a.status === 'stored');
    const previous = imagePreviewsRef.current;
    let cancelled = false;
    imageAttachments.forEach((att) => {
      if (previous[att.id]) return;
      fetch(`/api/mail/messages/${message.id}/attachments/${att.id}/download`)
        .then((response) => response.ok ? response.blob() : null)
        .then((blob) => {
          if (!blob || cancelled) return;
          const url = URL.createObjectURL(blob);
          imagePreviewsRef.current[att.id] = url;
          setImagePreviews((current) => ({ ...current, [att.id]: url }));
        })
        .catch(() => {});
    });
    return () => {
      cancelled = true;
    };
  }, [attachments, message]);

  useEffect(() => {
    const urls = imagePreviewsRef.current;
    return () => {
      Object.values(urls).forEach((url) => URL.revokeObjectURL(url));
    };
  }, []);

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

  const increaseFontSize = useCallback(() => {
    setFontSize((current) => Math.min(24, current + 1));
  }, []);

  const decreaseFontSize = useCallback(() => {
    setFontSize((current) => Math.max(11, current - 1));
  }, []);

  const handleReadingScroll = () => {
    const container = scrollContainerRef.current;
    if (!container) return;
    const max = container.scrollHeight - container.clientHeight;
    setScrollProgress(max > 0 ? Math.round((container.scrollTop / max) * 100) : 0);
  };

  const handleSaveContact = () => {
    if (!message) return;
    try {
      const contacts: Record<string, string> = JSON.parse(localStorage.getItem('webmail_contacts') ?? '{}');
      contacts[message.from_addr.toLowerCase()] = message.from_name || message.from_addr;
      localStorage.setItem('webmail_contacts', JSON.stringify(contacts));
    } catch {
      // ignore
    }
    setSavedContact(true);
    setTimeout(() => setSavedContact(false), 2000);
  };

  const parseIcsDate = (value: string): Date | null => {
    try {
      const clean = value.trim().replace(/z$/i, '');
      if (clean.length === 8) {
        return new Date(`${clean.slice(0, 4)}-${clean.slice(4, 6)}-${clean.slice(6, 8)}T00:00:00`);
      }
      if (clean.includes('T')) {
        return new Date(`${clean.slice(0, 4)}-${clean.slice(4, 6)}-${clean.slice(6, 8)}T${clean.slice(9, 11)}:${clean.slice(11, 13)}:${clean.slice(13, 15)}`);
      }
      return new Date(clean);
    } catch {
      return null;
    }
  };

  const handleAddToCalendar = async (event: ICSEvent) => {
    setAddingCalendarId(event.dtstart);
    try {
      const calendars = await listCalendars();
      const cal = calendars[0];
      if (!cal) return;
      const start = parseIcsDate(event.dtstart) ?? new Date();
      const end = parseIcsDate(event.dtend || '') ?? new Date(start.getTime() + 60 * 60 * 1000);
      await createCalendarEvent(cal.ID, {
        title: event.summary,
        start,
        end,
        allDay: event.dtstart.length === 8,
        location: event.location,
        description: event.description,
      });
      setCalendarAdded(event.dtstart);
      setTimeout(() => setCalendarAdded(null), 3000);
    } catch {
      // ignore
    } finally {
      setAddingCalendarId(null);
    }
  };

  const handleDownload = useCallback(async (att: Attachment) => {
    if (!message) return;
    setDownloadingId(att.id);
    try {
      await downloadAttachment(message.id, att.id, att.filename);
    } catch {
      // ignore
    } finally {
      setDownloadingId(null);
    }
  }, [message]);

  const handleSaveToDrive = useCallback(async (att: Attachment) => {
    if (!message) return;
    setSavingToDriveId(att.id);
    try {
      const node = await saveAttachmentToDrive(message.id, att.id, att.filename, att.mime_type);
      setDriveToast(node ? t('misc.readingPane.savedToDrive', { filename: att.filename }) : t('misc.readingPane.driveSaveFailed'));
      setTimeout(() => setDriveToast(''), 3000);
    } catch {
      setDriveToast(t('misc.readingPane.driveSaveFailed'));
      setTimeout(() => setDriveToast(''), 3000);
    } finally {
      setSavingToDriveId(null);
    }
  }, [message]);

  const handlePdfPreview = useCallback(async (att: Attachment) => {
    if (!message) return;
    setPdfPreviewLoadingId(att.id);
    try {
      const res = await fetch(`/api/mail/messages/${message.id}/attachments/${att.id}/download`);
      if (!res.ok) return;
      const blob = await res.blob();
      setPdfPreview({ url: URL.createObjectURL(blob), filename: att.filename });
    } catch {
      // ignore
    } finally {
      setPdfPreviewLoadingId(null);
    }
  }, [message]);

  const copyEmail = useCallback((address: string) => {
    navigator.clipboard.writeText(address).catch(() => {});
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

  const onOpenImage = (url: string, filename: string, attId: string) => {
    setLightbox({ url, filename, attId });
  };

  const onOpenFullCompose = (intent: 'reply' | 'reply_all' | 'forward') => {
    setInlineCompose(null);
    const action = intent === 'reply'
      ? onReply
      : intent === 'reply_all'
      ? onReplyAll
      : onForward;
    action?.();
  };

  const handleReadingPaneKeyDown = (event: ReactKeyboardEvent<HTMLElement>) => {
    if ((event.target as HTMLElement | null)?.closest('input, textarea, select, [contenteditable="true"]')) return;
    if (event.metaKey || event.ctrlKey || event.altKey) return;

    const container = scrollContainerRef.current;
    const stop = () => {
      event.preventDefault();
      event.stopPropagation();
      event.nativeEvent.stopImmediatePropagation?.();
    };
    const scrollBy = (top: number) => {
      stop();
      container?.scrollBy({ top, behavior: 'smooth' });
    };
    const scrollTo = (top: number) => {
      stop();
      container?.scrollTo({ top, behavior: 'smooth' });
    };

    if (event.key === 'ArrowDown') {
      scrollBy(80);
      return;
    }
    if (event.key === 'ArrowUp') {
      scrollBy(-80);
      return;
    }
    if (event.key === 'PageDown') {
      scrollBy(Math.max(120, (container?.clientHeight ?? 0) * 0.85));
      return;
    }
    if (event.key === 'PageUp') {
      scrollBy(-Math.max(120, (container?.clientHeight ?? 0) * 0.85));
      return;
    }
    if (event.key === 'Home') {
      scrollTo(0);
      return;
    }
    if (event.key === 'End') {
      scrollTo(container?.scrollHeight ?? 0);
      return;
    }
    if (event.key === 'Escape') {
      if (!onBack) return;
      stop();
      onBack();
      return;
    }
    if (event.key === 'Delete' || event.key === 'Backspace' || event.key === '#') {
      if (!onDelete) return;
      stop();
      onDelete();
      return;
    }

    const key = event.key.toLowerCase();
    if (key === 'r') {
      stop();
      onOpenFullCompose('reply');
      return;
    }
    if (key === 'a') {
      stop();
      onOpenFullCompose('reply_all');
      return;
    }
    if (key === 'f') {
      stop();
      onOpenFullCompose('forward');
      return;
    }
    if (key === 's') {
      if (!onStar) return;
      stop();
      onStar();
      return;
    }
    if (key === 'e') {
      if (!onArchive) return;
      stop();
      onArchive();
      return;
    }
    if (key === 'm') {
      if (!onToggleRead) return;
      stop();
      onToggleRead();
      return;
    }
  };

  if (loading) {
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

  if (!message) {
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
