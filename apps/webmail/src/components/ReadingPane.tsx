'use client';

import { useEffect, useRef, useState, useCallback, ReactNode } from 'react';
import { MessageDetail, Folder } from '@/lib/api';

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
  const hasImages = /<img\s/i.test(html);

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
    });
  }, [html, showImages]);

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
    </>
  );
}

interface ReadingPaneProps {
  message: MessageDetail | null;
  folders?: Folder[];
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
}: ReadingPaneProps) {
  const [showMoveMenu, setShowMoveMenu] = useState(false);
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

  useEffect(() => {
    setQuickReplyOpen(false);
    setQuickReplyText('');
    setQuickReplySent(false);
  }, [message?.id]);
  const [copiedEmail, setCopiedEmail] = useState('');
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const copyEmail = useCallback((email: string) => {
    navigator.clipboard.writeText(email).catch(() => {});
    setCopiedEmail(email);
    if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
    copyTimerRef.current = setTimeout(() => setCopiedEmail(''), 2000);
  }, []);
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

        {/* Attachment notice */}
        {message.has_attachment && (
          <div style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            padding: '8px 12px',
            marginBottom: '12px',
            maxWidth: '680px',
            borderRadius: '6px',
            border: '1px solid var(--color-border-default)',
            background: 'var(--color-bg-secondary)',
            fontSize: '13px',
            color: 'var(--color-text-secondary)',
          }}>
            <span aria-hidden="true" style={{ fontSize: '16px' }}>📎</span>
            <span>이 메시지에 첨부파일이 있습니다</span>
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
