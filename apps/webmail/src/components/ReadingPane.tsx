'use client';

import { useEffect, useRef, useState, useCallback, useMemo, ReactNode } from 'react';
import { MessageDetail, Folder, Attachment, listAttachments, downloadAttachment } from '@/lib/api';

const URL_RE = /https?:\/\/[^\s<>"']+/g;
function linkify(text: string): ReactNode[] {
  const parts: ReactNode[] = [];
  let last = 0;
  let match: RegExpExecArray | null;
  URL_RE.lastIndex = 0;
  while ((match = URL_RE.exec(text)) !== null) {
    if (match.index > last) parts.push(text.slice(last, match.index));
    const url = match[0];
    parts.push(
      <a key={match.index} href={url} target="_blank" rel="noopener noreferrer"
        style={{ color: 'var(--color-accent)', wordBreak: 'break-all' }}>
        {url}
      </a>
    );
    last = match.index + url.length;
  }
  if (last < text.length) parts.push(text.slice(last));
  return parts;
}

function SafeHTMLBody({ html }: { html: string }) {
  const ref = useRef<HTMLDivElement>(null);
  const [showImages, setShowImages] = useState(false);
  const [showQuoted, setShowQuoted] = useState(false);
  const hasImages = /<img\s/i.test(html);
  const hasQuoted = /<blockquote/i.test(html);

  useEffect(() => { setShowQuoted(false); setShowImages(false); }, [html]);

  useEffect(() => {
    if (!ref.current) return;
    import('dompurify').then(({ default: DOMPurify }) => {
      if (!ref.current) return;
      const forbidTags: string[] = ['script', 'style', 'iframe', 'form', 'input'];
      if (!showImages) forbidTags.push('img');
      const clean = DOMPurify.sanitize(html, {
        USE_PROFILES: { html: true },
        FORBID_TAGS: forbidTags,
        FORBID_ATTR: ['onerror', 'onload', 'onclick', 'onmouseover'],
      });
      ref.current.innerHTML = clean;
      // Collapse blockquotes when not showing quoted text
      if (hasQuoted && !showQuoted) {
        ref.current.querySelectorAll('blockquote').forEach((bq) => {
          (bq as HTMLElement).style.display = 'none';
        });
      }
    });
  }, [html, showImages, showQuoted, hasQuoted]);

  return (
    <>
      {hasImages && !showImages && (
        <div style={{
          display: 'flex',
          alignItems: 'center',
          gap: '10px',
          padding: '8px 16px',
          background: 'var(--color-bg-secondary)',
          borderBottom: '1px solid var(--color-border-subtle)',
          fontSize: '13px',
          color: 'var(--color-text-secondary)',
        }}>
          <span>원격 이미지가 차단됨</span>
          <button
            onClick={() => setShowImages(true)}
            style={{ fontSize: '13px', color: 'var(--color-accent)', background: 'none', border: 'none', cursor: 'pointer', padding: 0, fontWeight: 500 }}
          >
            이미지 표시
          </button>
        </div>
      )}
      <div ref={ref} style={{ wordBreak: 'break-word', lineHeight: 1.6 }} />
      {hasQuoted && (
        <button
          onClick={() => setShowQuoted((v) => !v)}
          style={{ marginTop: '8px', fontSize: '12px', color: 'var(--color-text-tertiary)', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-default)', borderRadius: '4px', cursor: 'pointer', padding: '3px 10px' }}
        >
          {showQuoted ? '인용문 숨기기' : '원본 메시지 보기 ···'}
        </button>
      )}
    </>
  );
}

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
  onMarkUnread?: () => void;
  onMove?: (folderId: string) => void;
  onPrint?: () => void;
  loading?: boolean;
  onBack?: () => void;
  isStarred?: boolean;
  onStar?: (starred: boolean) => void;
  onQuickReply?: (body: string) => Promise<void>;
  onPrev?: () => void;
  onNext?: () => void;
  messageIndex?: number;
  messageTotal?: number;
  onComposeToAddress?: (address: string) => void;
  onRestore?: () => void;
  onSnooze?: (messageId: string, until: Date) => void;
}

function readingTime(text: string): string {
  const words = text.trim().split(/\s+/).filter(Boolean).length;
  const mins = Math.ceil(words / 200);
  return mins <= 1 ? '약 1분' : `약 ${mins}분`;
}

function formatFullDate(receivedAt: string): string {
  return new Intl.DateTimeFormat('ko-KR', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    hour12: false,
  }).format(new Date(receivedAt));
}

const iconStyle: React.CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  padding: '5px 10px',
  borderRadius: '5px',
  border: '1px solid var(--color-border-default)',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  fontSize: '13px',
  cursor: 'pointer',
  transition: 'background 100ms ease, color 100ms ease',
};

function ActionButton({
  label,
  onClick,
  danger,
}: {
  label: string;
  onClick?: () => void;
  danger?: boolean;
}) {
  return (
    <button
      onClick={onClick}
      title={label}
      aria-label={label}
      style={{
        ...iconStyle,
        color: danger ? 'var(--color-destructive)' : 'var(--color-text-secondary)',
        borderColor: danger ? 'rgba(217,79,61,0.3)' : 'var(--color-border-default)',
      }}
      onMouseEnter={(e) => {
        (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
      }}
    >
      {label}
    </button>
  );
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
  onMarkUnread,
  onMove,
  onPrint,
  loading,
  onBack,
  isStarred,
  onStar,
  onQuickReply,
  onPrev,
  onNext,
  messageIndex,
  messageTotal,
  onComposeToAddress,
  onRestore,
  onSnooze,
}: ReadingPaneProps) {
  const [showMoveMenu, setShowMoveMenu] = useState(false);
  const [showSnoozeMenu, setShowSnoozeMenu] = useState(false);
  const [quickReplyOpen, setQuickReplyOpen] = useState(false);
  const [quickReplyText, setQuickReplyText] = useState('');
  const [quickReplySending, setQuickReplySending] = useState(false);
  const [quickReplySent, setQuickReplySent] = useState(false);
  const quickReplyRef = useRef<HTMLTextAreaElement>(null);
  const [fontSize, setFontSize] = useState(() => {
    try { return parseInt(localStorage.getItem('webmail_font_size') ?? '14', 10) || 14; } catch { return 14; }
  });

  useEffect(() => {
    localStorage.setItem('webmail_font_size', String(fontSize));
  }, [fontSize]);

  const unsubscribeUrl = useMemo(() => {
    if (!message) return null;
    if (message.html_body) {
      try {
        const doc = new DOMParser().parseFromString(message.html_body, 'text/html');
        const link = Array.from(doc.querySelectorAll('a')).find((a) =>
          /unsubscribe|opt.?out|수신\s*거부/i.test(a.textContent ?? '') ||
          /unsubscribe|optout/i.test(a.getAttribute('href') ?? '')
        );
        if (link?.href) return link.href;
      } catch { /* ignore */ }
    }
    if (message.text_body) {
      const m = message.text_body.match(/https?:\/\/[^\s<>"']*unsubscribe[^\s<>"']*/i);
      if (m) return m[0];
    }
    return null;
  }, [message?.id, message?.html_body, message?.text_body]);

  useEffect(() => {
    setQuickReplyOpen(false);
    setQuickReplyText('');
    setQuickReplySent(false);
  }, [message?.id]);
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [attachmentsLoading, setAttachmentsLoading] = useState(false);
  const [downloadingId, setDownloadingId] = useState<string | null>(null);

  useEffect(() => {
    if (!message?.has_attachment || !message.id) { setAttachments([]); return; }
    setAttachmentsLoading(true);
    listAttachments(message.id)
      .then(setAttachments)
      .catch(() => setAttachments([]))
      .finally(() => setAttachmentsLoading(false));
  }, [message?.id, message?.has_attachment]);

  const handleDownload = useCallback(async (att: Attachment) => {
    if (!message) return;
    setDownloadingId(att.id);
    try { await downloadAttachment(message.id, att.id, att.filename); } catch { /* ignore */ }
    finally { setDownloadingId(null); }
  }, [message]);

  const [imagePreviews, setImagePreviews] = useState<Record<string, string>>({});
  const imagePreviewsRef = useRef<Record<string, string>>({});

  useEffect(() => {
    const imageAtts = attachments.filter((a) => a.mime_type.startsWith('image/') && a.status === 'stored');
    if (!message || imageAtts.length === 0) return;
    const token = typeof window !== 'undefined' ? localStorage.getItem('webmail_token') : null;
    const headers: HeadersInit = token ? { Authorization: `Bearer ${token}` } : {};
    let cancelled = false;
    imageAtts.forEach((att) => {
      if (imagePreviewsRef.current[att.id]) return;
      fetch(`/api/mail/messages/${message.id}/attachments/${att.id}/download`, { headers })
        .then((r) => r.ok ? r.blob() : null)
        .then((blob) => {
          if (cancelled || !blob) return;
          const url = URL.createObjectURL(blob);
          imagePreviewsRef.current[att.id] = url;
          setImagePreviews((prev) => ({ ...prev, [att.id]: url }));
        })
        .catch(() => {});
    });
    return () => { cancelled = true; };
  }, [attachments, message]);

  useEffect(() => {
    const urls = imagePreviewsRef.current;
    return () => { Object.values(urls).forEach((u) => URL.revokeObjectURL(u)); };
  }, []);

  const [copiedEmail, setCopiedEmail] = useState('');
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const copyEmail = useCallback((email: string) => {
    navigator.clipboard.writeText(email).catch(() => {});
    setCopiedEmail(email);
    if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
    copyTimerRef.current = setTimeout(() => setCopiedEmail(''), 2000);
  }, []);

  const scrollContainerRef = useRef<HTMLElement>(null);
  useEffect(() => {
    scrollContainerRef.current?.scrollTo({ top: 0 });
  }, [message?.id]);
  if (loading) {
    return (
      <main
        aria-label="메일 읽기"
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
        aria-label="메일 읽기"
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
        <p style={{ fontSize: '14px' }}>메시지를 선택하세요</p>
      </main>
    );
  }

  const toList = (message.to_addrs ?? [])
    .map((t) => (t.name ? `${t.name} <${t.address}>` : t.address))
    .join(', ');
  const ccList = (message.cc_addrs ?? [])
    .map((t) => (t.name ? `${t.name} <${t.address}>` : t.address))
    .join(', ');

  return (
    <main
      ref={scrollContainerRef}
      aria-label="메일 읽기"
      style={{
        flex: 1,
        minWidth: 0,
        height: '100%',
        overflowY: 'auto',
        background: 'var(--color-bg-primary)',
        display: 'flex',
        flexDirection: 'column',
      }}
    >
      {/* Toolbar */}
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
            aria-label="뒤로"
            onClick={onBack}
            style={{ ...iconStyle, marginRight: 'auto', color: 'var(--color-text-secondary)' }}
          >← 뒤로</button>
        )}
        {(onPrev || onNext) && !onBack && <div style={{ marginRight: 'auto' }} />}
        {onPrev && (
          <button aria-label="이전 메일" title="이전 메일 (k)" onClick={onPrev}
            style={{ ...iconStyle, color: 'var(--color-text-secondary)' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >↑</button>
        )}
        {messageIndex !== undefined && messageTotal !== undefined && (
          <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', minWidth: '40px', textAlign: 'center' }}>
            {messageIndex + 1} / {messageTotal}
          </span>
        )}
        {onNext && (
          <button aria-label="다음 메일" title="다음 메일 (j)" onClick={onNext}
            style={{ ...iconStyle, color: 'var(--color-text-secondary)' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >↓</button>
        )}
        <ActionButton label="답장" onClick={onReply} />
        <ActionButton label="전체 답장" onClick={onReplyAll} />
        <ActionButton label="전달" onClick={onForward} />
        <ActionButton label="읽지 않음으로" onClick={onMarkUnread} />
        {onStar && (
          <button
            onClick={() => onStar(!isStarred)}
            title={isStarred ? '별표 해제' : '별표'}
            aria-label={isStarred ? '별표 해제' : '별표'}
            style={{
              ...iconStyle,
              color: isStarred ? '#f59e0b' : 'var(--color-text-secondary)',
              borderColor: isStarred ? 'rgba(245,158,11,0.4)' : 'var(--color-border-default)',
              fontSize: '15px',
            }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >
            {isStarred ? '★' : '☆'}
          </button>
        )}
        <ActionButton label="인쇄" onClick={onPrint} />
        <div style={{ display: 'flex', alignItems: 'center', gap: '2px', marginLeft: '4px' }}>
          <button
            aria-label="글자 크기 줄이기"
            onClick={() => setFontSize((f) => Math.max(11, f - 1))}
            style={{ ...iconStyle, fontSize: '11px', padding: '5px 7px' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >A-</button>
          <button
            aria-label="글자 크기 늘리기"
            onClick={() => setFontSize((f) => Math.min(24, f + 1))}
            style={{ ...iconStyle, fontSize: '13px', padding: '5px 7px' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >A+</button>
        </div>
        {unsubscribeUrl && (
          <button
            onClick={() => {
              if (window.confirm('수신거부 링크를 열겠습니까?')) window.open(unsubscribeUrl, '_blank', 'noopener,noreferrer');
            }}
            title="수신거부"
            aria-label="수신거부"
            style={{ ...iconStyle, fontSize: '12px', color: 'var(--color-destructive)', borderColor: 'rgba(217,79,61,0.3)', padding: '5px 8px' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >수신거부</button>
        )}
        {onMove && folders.length > 0 && (
          <div style={{ position: 'relative' }}>
            <ActionButton label="이동" onClick={() => setShowMoveMenu((v) => !v)} />
            {showMoveMenu && (
              <div
                style={{
                  position: 'absolute',
                  top: '100%',
                  right: 0,
                  marginTop: '4px',
                  background: 'var(--color-bg-primary)',
                  border: '1px solid var(--color-border-default)',
                  borderRadius: '6px',
                  boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
                  zIndex: 200,
                  minWidth: '160px',
                  overflow: 'hidden',
                }}
              >
                {folders.map((f) => (
                  <button
                    key={f.id}
                    onClick={() => { onMove(f.id); setShowMoveMenu(false); }}
                    style={{
                      display: 'block',
                      width: '100%',
                      textAlign: 'left',
                      padding: '8px 14px',
                      border: 'none',
                      background: 'transparent',
                      color: 'var(--color-text-primary)',
                      fontSize: '13px',
                      cursor: 'pointer',
                    }}
                    onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-secondary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
                  >
                    {f.name}
                  </button>
                ))}
              </div>
            )}
          </div>
        )}
        {onSnooze && message && (
          <div style={{ position: 'relative' }}>
            <ActionButton label="스누즈" onClick={() => setShowSnoozeMenu((v) => !v)} />
            {showSnoozeMenu && (
              <div style={{ position: 'absolute', top: '100%', right: 0, marginTop: '4px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '6px', boxShadow: '0 4px 16px rgba(0,0,0,0.12)', zIndex: 200, minWidth: '180px', overflow: 'hidden' }}>
                {[
                  { label: '1시간 후', ms: 60 * 60 * 1000 },
                  { label: '4시간 후', ms: 4 * 60 * 60 * 1000 },
                  { label: '오늘 저녁 (18:00)', ms: (() => { const d = new Date(); d.setHours(18,0,0,0); return d.getTime() > Date.now() ? d.getTime() - Date.now() : 24 * 3600000; })() },
                  { label: '내일 오전 (09:00)', ms: (() => { const d = new Date(); d.setDate(d.getDate() + 1); d.setHours(9,0,0,0); return d.getTime() - Date.now(); })() },
                  { label: '다음 주 월요일', ms: (() => { const d = new Date(); const daysUntilMon = (8 - d.getDay()) % 7 || 7; d.setDate(d.getDate() + daysUntilMon); d.setHours(9,0,0,0); return d.getTime() - Date.now(); })() },
                ].map((opt) => (
                  <button
                    key={opt.label}
                    onClick={() => { onSnooze(message.id, new Date(Date.now() + opt.ms)); setShowSnoozeMenu(false); }}
                    style={{ display: 'block', width: '100%', textAlign: 'left', padding: '8px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
                    onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                  >{opt.label}</button>
                ))}
              </div>
            )}
          </div>
        )}
        {onArchive && <ActionButton label="아카이브" onClick={onArchive} />}
        {onSpam && <ActionButton label="스팸 신고" onClick={onSpam} />}
        {onNotSpam && <ActionButton label="스팸 아님" onClick={onNotSpam} />}
        {onRestore && <ActionButton label="복구" onClick={onRestore} />}
        <ActionButton label="삭제" onClick={onDelete} danger />
      </div>

      {/* Message content */}
      <div style={{ padding: '20px 24px', flex: 1 }}>
        {/* Subject */}
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

        {/* Sender row */}
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            gap: '16px',
            marginBottom: '8px',
          }}
        >
          <div>
            <div style={{ fontSize: '14px', fontWeight: 500, color: 'var(--color-text-primary)' }}>
              <span
                title="클릭하면 주소 복사"
                onClick={() => copyEmail(message.from_addr)}
                style={{ cursor: 'pointer', borderRadius: '3px', padding: '0 2px' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLSpanElement).style.background = 'var(--color-bg-secondary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLSpanElement).style.background = 'transparent'; }}
              >
                {copiedEmail === message.from_addr ? '복사됨 ✓' : (message.from_name || message.from_addr)}
              </span>
              {message.from_name && (
                <span
                  title="클릭하면 주소 복사"
                  onClick={() => copyEmail(message.from_addr)}
                  style={{ fontSize: '13px', fontWeight: 400, color: 'var(--color-text-secondary)', marginInlineStart: '6px', cursor: 'pointer', borderRadius: '3px', padding: '0 2px' }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLSpanElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLSpanElement).style.background = 'transparent'; }}
                >
                  {copiedEmail === message.from_addr ? '' : `<${message.from_addr}>`}
                </span>
              )}
              {onComposeToAddress && (
                <button
                  onClick={() => onComposeToAddress(message.from_addr)}
                  title={`${message.from_addr}에게 새 메일 작성`}
                  style={{ background: 'none', border: '1px solid var(--color-border-default)', borderRadius: '4px', cursor: 'pointer', fontSize: '11px', color: 'var(--color-text-tertiary)', padding: '1px 6px', marginInlineStart: '6px', lineHeight: 1.4 }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >메일 보내기</button>
              )}
            </div>
            {toList && (
              <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
                받는 사람:{' '}
                {(message.to_addrs ?? []).map((t, i) => (
                  <span key={t.address}>
                    {i > 0 && ', '}
                    <span
                      title="클릭하면 주소 복사"
                      onClick={() => copyEmail(t.address)}
                      style={{ cursor: 'pointer', borderRadius: '3px', padding: '0 2px' }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLSpanElement).style.background = 'var(--color-bg-tertiary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLSpanElement).style.background = 'transparent'; }}
                    >
                      {copiedEmail === t.address ? '복사됨 ✓' : (t.name ? `${t.name} <${t.address}>` : t.address)}
                    </span>
                  </span>
                ))}
              </div>
            )}
            {ccList && (
              <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)', marginTop: '2px' }}>
                참조:{' '}
                {(message.cc_addrs ?? []).map((t, i) => (
                  <span key={t.address}>
                    {i > 0 && ', '}
                    <span
                      title="클릭하면 주소 복사"
                      onClick={() => copyEmail(t.address)}
                      style={{ cursor: 'pointer', borderRadius: '3px', padding: '0 2px' }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLSpanElement).style.background = 'var(--color-bg-tertiary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLSpanElement).style.background = 'transparent'; }}
                    >
                      {copiedEmail === t.address ? '복사됨 ✓' : (t.name ? `${t.name} <${t.address}>` : t.address)}
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

        {/* Divider */}
        <hr
          style={{
            border: 'none',
            borderTop: '1px solid var(--color-border-subtle)',
            margin: '16px 0',
          }}
        />

        {/* Attachments */}
        {message.has_attachment && (
          <div style={{ marginBottom: '16px', maxWidth: '680px' }}>
            <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-tertiary)', letterSpacing: '0.05em', textTransform: 'uppercase', marginBottom: '8px' }}>
              첨부파일 {attachments.length > 0 ? `(${attachments.length})` : ''}
            </div>
            {attachmentsLoading ? (
              <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)' }}>로딩 중...</div>
            ) : attachments.length === 0 ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', fontSize: '13px', color: 'var(--color-text-secondary)' }}>
                <span aria-hidden="true">📎</span>
                <span>첨부파일을 불러올 수 없습니다</span>
              </div>
            ) : (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                {attachments.map((att) => {
                  const isImg = att.mime_type.startsWith('image/');
                  const isPdf = att.mime_type === 'application/pdf';
                  const icon = isImg ? '🖼️' : isPdf ? '📄' : '📎';
                  const kb = att.size < 1024 * 1024 ? `${Math.round(att.size / 1024)} KB` : `${(att.size / 1024 / 1024).toFixed(1)} MB`;
                  const previewUrl = isImg ? imagePreviews[att.id] : undefined;
                  return (
                    <div key={att.id} style={{ display: 'inline-flex', flexDirection: 'column', gap: '4px', maxWidth: '200px' }}>
                      {previewUrl && (
                        <button
                          onClick={() => handleDownload(att)}
                          title={`${att.filename} 다운로드`}
                          style={{ border: '1px solid var(--color-border-default)', borderRadius: '6px', overflow: 'hidden', background: 'none', cursor: 'pointer', padding: 0, display: 'block' }}
                        >
                          {/* eslint-disable-next-line @next/next/no-img-element */}
                          <img src={previewUrl} alt={att.filename} style={{ width: '100%', height: '120px', objectFit: 'cover', display: 'block' }} />
                        </button>
                      )}
                      <button
                        onClick={() => handleDownload(att)}
                        disabled={downloadingId === att.id}
                        title={`${att.filename} (${kb}) 다운로드`}
                        style={{
                          display: 'inline-flex',
                          alignItems: 'center',
                          gap: '6px',
                          padding: '6px 12px',
                          borderRadius: '6px',
                          border: '1px solid var(--color-border-default)',
                          background: downloadingId === att.id ? 'var(--color-bg-tertiary)' : 'var(--color-bg-secondary)',
                          color: 'var(--color-text-primary)',
                          fontSize: '13px',
                          cursor: downloadingId === att.id ? 'wait' : 'pointer',
                          maxWidth: '100%',
                          textAlign: 'left',
                        }}
                        onMouseEnter={(e) => { if (downloadingId !== att.id) (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                        onMouseLeave={(e) => { if (downloadingId !== att.id) (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                      >
                        <span aria-hidden="true">{downloadingId === att.id ? '⏳' : icon}</span>
                        <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0 }}>{att.filename}</span>
                        <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>{kb}</span>
                      </button>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}

        {/* Body */}
        <div
          style={{
            maxWidth: '680px',
            fontSize: `${fontSize}px`,
            lineHeight: 1.6,
            color: 'var(--color-text-primary)',
          }}
        >
          {message.html_body ? (
            <SafeHTMLBody html={message.html_body} />
          ) : (
            <pre
              style={{
                fontFamily: 'inherit',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-word',
                margin: 0,
              }}
            >
              {linkify(message.text_body || '(내용 없음)')}
            </pre>
          )}
        </div>

        {/* Quick reply */}
        {onQuickReply && (
          <div style={{ marginTop: '24px', borderTop: '1px solid var(--color-border-subtle)', paddingTop: '16px' }}>
            {!quickReplyOpen ? (
              <button
                onClick={() => { setQuickReplyOpen(true); setTimeout(() => quickReplyRef.current?.focus(), 50); }}
                style={{
                  width: '100%',
                  maxWidth: '680px',
                  textAlign: 'left',
                  padding: '10px 14px',
                  borderRadius: '6px',
                  border: '1px solid var(--color-border-default)',
                  background: 'var(--color-bg-secondary)',
                  color: 'var(--color-text-tertiary)',
                  fontSize: '14px',
                  cursor: 'text',
                }}
              >
                ← 답장하기...
              </button>
            ) : (
              <div style={{ maxWidth: '680px', border: '1px solid var(--color-accent)', borderRadius: '6px', overflow: 'hidden' }}>
                <textarea
                  ref={quickReplyRef}
                  value={quickReplyText}
                  onChange={(e) => setQuickReplyText(e.target.value)}
                  placeholder="답장 내용을 입력하세요..."
                  rows={4}
                  onKeyDown={(e) => {
                    if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') {
                      e.preventDefault();
                      if (!quickReplySending && quickReplyText.trim()) {
                        setQuickReplySending(true);
                        onQuickReply(quickReplyText.trim())
                          .then(() => { setQuickReplySent(true); setQuickReplyText(''); setTimeout(() => { setQuickReplySent(false); setQuickReplyOpen(false); }, 1500); })
                          .catch(() => {})
                          .finally(() => setQuickReplySending(false));
                      }
                    }
                    if (e.key === 'Escape') { setQuickReplyOpen(false); setQuickReplyText(''); }
                  }}
                  style={{ width: '100%', padding: '12px 14px', border: 'none', outline: 'none', resize: 'vertical', fontSize: '14px', lineHeight: 1.6, background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', boxSizing: 'border-box' }}
                />
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', padding: '8px 12px', background: 'var(--color-bg-secondary)', borderTop: '1px solid var(--color-border-subtle)' }}>
                  <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>Ctrl+Enter로 전송 · Escape로 취소</span>
                  <div style={{ display: 'flex', gap: '8px' }}>
                    <button
                      onClick={() => { setQuickReplyOpen(false); setQuickReplyText(''); }}
                      style={{ padding: '5px 12px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}
                    >취소</button>
                    <button
                      disabled={quickReplySending || !quickReplyText.trim()}
                      onClick={() => {
                        if (quickReplySending || !quickReplyText.trim()) return;
                        setQuickReplySending(true);
                        onQuickReply(quickReplyText.trim())
                          .then(() => { setQuickReplySent(true); setQuickReplyText(''); setTimeout(() => { setQuickReplySent(false); setQuickReplyOpen(false); }, 1500); })
                          .catch(() => {})
                          .finally(() => setQuickReplySending(false));
                      }}
                      style={{ padding: '5px 14px', borderRadius: '5px', border: 'none', background: quickReplySending || !quickReplyText.trim() ? 'var(--color-border-default)' : 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 500, cursor: quickReplySending || !quickReplyText.trim() ? 'not-allowed' : 'pointer' }}
                    >
                      {quickReplySent ? '전송됨 ✓' : quickReplySending ? '전송 중...' : '보내기'}
                    </button>
                  </div>
                </div>
              </div>
            )}
          </div>
        )}
      </div>
    </main>
  );
}
