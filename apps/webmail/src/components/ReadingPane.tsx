'use client';

import { useEffect, useRef, useState, useCallback, useMemo, ReactNode } from 'react';
import { MessageDetail, MessageSummary, Folder, Attachment, MessageDeliveryStatus, listAttachments, downloadAttachment, getMessageDeliveryStatus, saveAttachmentToDrive, listCalendars, createCalendarEvent, sendMessage, uploadAttachment } from '@/lib/api';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import LinkExt from '@tiptap/extension-link';
import Underline from '@tiptap/extension-underline';
import TextAlign from '@tiptap/extension-text-align';
import Placeholder from '@tiptap/extension-placeholder';
import Image from '@tiptap/extension-image';
import {
  ArrowUturnLeftIcon,
  ArrowUturnRightIcon,
  ArchiveBoxIcon,
  ArrowTopRightOnSquareIcon,
  EllipsisHorizontalIcon,
  ArrowLeftIcon,
  ChevronUpIcon,
  ChevronDownIcon,
  PaperClipIcon,
  PhotoIcon,
  DocumentIcon,
  ArrowPathIcon,
  StarIcon,
  NoSymbolIcon,
  LinkIcon,
  ListBulletIcon,
  NumberedListIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';
import { StarIcon as StarIconSolid } from '@heroicons/react/24/solid';

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

function SafeHTMLBody({ html, onMailto, externalImages = 'ask' }: { html: string; onMailto?: (addr: string) => void; externalImages?: string }) {
  const blockTrackingPixels = (() => { try { return JSON.parse(localStorage.getItem('webmail_settings') ?? '{}').blockTrackingPixels !== false; } catch { return true; } })();
  const ref = useRef<HTMLDivElement>(null);
  const [showImages, setShowImages] = useState(externalImages === 'always');
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
      // Rewrite external img src through image proxy (hides user IP/Referer from sender)
      if (showImages) {
        ref.current.querySelectorAll('img[src]').forEach((img) => {
          const src = img.getAttribute('src') ?? '';
          if (src.startsWith('http://') || src.startsWith('https://')) {
            img.setAttribute('src', `/api/image-proxy?url=${encodeURIComponent(src)}`);
          }
        });
      }
      // Remove tracking pixels after proxy rewrite (check original-like patterns in proxied url)
      if (blockTrackingPixels && showImages) {
        ref.current.querySelectorAll('img').forEach((img) => {
          const w = img.getAttribute('width'); const h = img.getAttribute('height');
          const isPixel = (w === '1' || w === '0') && (h === '1' || h === '0');
          const src = img.getAttribute('src') ?? '';
          const isTracker = /track|pixel|beacon|open\.|email\.([a-z]+\.)+[a-z]+\/|\?t=|\.gif\?/i.test(src);
          if (isPixel || isTracker) img.remove();
        });
      }
      // Ensure all external links have rel="noopener noreferrer" and open in new tab
      ref.current.querySelectorAll('a[href]').forEach((el) => {
        const a = el as HTMLAnchorElement;
        const href = a.getAttribute('href') ?? '';
        if (href.startsWith('mailto:')) {
          a.addEventListener('click', (e) => {
            e.preventDefault();
            const addr = href.replace(/^mailto:/i, '').split('?')[0];
            onMailto?.(addr);
          });
        } else if (href.startsWith('http://') || href.startsWith('https://')) {
          a.setAttribute('rel', 'noopener noreferrer');
          a.setAttribute('target', '_blank');
        }
      });
      // Collapse blockquotes when not showing quoted text
      if (hasQuoted && !showQuoted) {
        ref.current.querySelectorAll('blockquote').forEach((bq) => {
          (bq as HTMLElement).style.display = 'none';
        });
      }
    });
  }, [html, showImages, showQuoted, hasQuoted, onMailto]);

  return (
    <>
      {hasImages && !showImages && externalImages !== 'never' && (
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
  onRestore?: () => void;
  onSnooze?: (messageId: string, until: Date) => void;
  onOpenInWindow?: () => void;
  onToggleRead?: () => void;
  isRead?: boolean;
  onStar?: () => void;
  isStarred?: boolean;
  threadMessages?: MessageSummary[];
  onSelectThread?: (id: string) => void;
  userEmail?: string;
  externalImages?: string;
}

function getSmartReplies(subject: string, body: string): string[] {
  const text = ((subject ?? '') + ' ' + (body ?? '')).toLowerCase();
  const replies: string[] = [];
  if (/언제|일정|미팅|회의|가능|schedule|meet|available|when/.test(text))
    replies.push('일정 확인 후 연락드리겠습니다.', '해당 시간에 가능합니다.');
  if (/감사|thanks|thank you|appreciate/.test(text))
    replies.push('천만에요. 도움이 되었으면 합니다.');
  if (/[?？]|알려|문의|질문|어떻게|어디|누가|무엇|왜/.test(text))
    replies.push('확인 후 답변드리겠습니다.', '네, 알겠습니다.');
  if (/검토|확인|리뷰|review|check/.test(text))
    replies.push('검토 후 피드백 드리겠습니다.');
  if (replies.length < 2) replies.push('감사합니다, 확인하겠습니다.', '알겠습니다.');
  if (replies.length < 3) replies.push('좀 더 검토 후 연락드리겠습니다.');
  return [...new Set(replies)].slice(0, 3);
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

// ── InlineCompose ─────────────────────────────────────────────────────────────

function escapeHtmlInline(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function buildInlineQuoteHTML(intent: string, sourceText: string): string {
  const header = intent === 'forward'
    ? '<p><strong>---------- 전달된 메시지 ----------</strong></p>'
    : '<p><strong>--- 원본 메시지 ---</strong></p>';
  const bodyLines = (sourceText || '')
    .split('\n')
    .map((line) => `<p>${escapeHtmlInline(line) || '&nbsp;'}</p>`)
    .join('');
  return `<p></p>${header}<blockquote>${bodyLines}</blockquote>`;
}

const toolbarBtnStyleInline = (active?: boolean): React.CSSProperties => ({
  width: '28px',
  height: '28px',
  borderRadius: '4px',
  border: 'none',
  background: active ? 'var(--color-bg-tertiary)' : 'transparent',
  color: active ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
  cursor: 'pointer',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontSize: '13px',
  fontWeight: 600,
  transition: 'background 80ms ease',
  flexShrink: 0,
});

interface InlineComposeProps {
  intent: 'reply' | 'reply_all' | 'forward';
  to: string;
  subject: string;
  messageId: string;
  sourceText?: string;
  onClose: () => void;
  onOpenFullModal: () => void;
  userEmail?: string;
}

function InlineCompose({ intent, to, subject, messageId, sourceText, onClose, onOpenFullModal }: InlineComposeProps) {
  const [sending, setSending] = useState(false);
  const [sent, setSent] = useState(false);
  const [cc, setCc] = useState('');
  const [showCc, setShowCc] = useState(false);
  const [attachments, setAttachments] = useState<Array<{ id: string; filename: string; size: number; uploading?: boolean }>>([]);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const imageInputRef = useRef<HTMLInputElement>(null);

  const editor = useEditor({
    extensions: [
      StarterKit,
      LinkExt.configure({ openOnClick: false }),
      Underline,
      TextAlign.configure({ types: ['heading', 'paragraph'] }),
      Placeholder.configure({ placeholder: '답장 내용을 입력하세요...' }),
      Image,
    ],
    content: sourceText ? buildInlineQuoteHTML(intent, sourceText) : '<p></p>',
    autofocus: 'start',
  });

  function handleLinkInsert() {
    const url = window.prompt('링크 URL을 입력하세요:');
    if (url && editor) {
      editor.chain().focus().setLink({ href: url }).run();
    }
  }

  async function handleImageFile(file: File) {
    if (!editor) return;
    let src: string;
    if (file.size < 500 * 1024) {
      src = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result as string);
        reader.onerror = reject;
        reader.readAsDataURL(file);
      });
    } else {
      const objectUrl = URL.createObjectURL(file);
      uploadAttachment(file).then((att) => {
        setAttachments((prev) => [...prev, { id: att.id, filename: att.filename, size: att.size }]);
      }).catch(() => {});
      src = objectUrl;
    }
    editor.chain().focus().setImage({ src, alt: file.name }).run();
  }

  async function handleFileAttach(files: FileList) {
    for (const file of Array.from(files)) {
      const tempId = `tmp-${Math.random().toString(36).slice(2)}`;
      setAttachments((prev) => [...prev, { id: tempId, filename: file.name, size: file.size, uploading: true }]);
      try {
        const att = await uploadAttachment(file);
        setAttachments((prev) => prev.map((a) => a.id === tempId ? { id: att.id, filename: att.filename, size: att.size } : a));
      } catch {
        setAttachments((prev) => prev.filter((a) => a.id !== tempId));
      }
    }
  }

  function doSend() {
    if (sending || !editor) return;
    const html = editor.getHTML();
    const toAddrs = to ? to.split(',').map((a) => ({ address: a.trim() })).filter((a) => a.address) : [];
    const ccAddrs = cc ? cc.split(',').map((a) => ({ address: a.trim() })).filter((a) => a.address) : [];
    setSending(true);
    sendMessage({
      to: toAddrs,
      cc: ccAddrs.length ? ccAddrs : undefined,
      subject,
      text_body: '',
      html_body: html,
      source_message_id: messageId,
      attachment_ids: attachments.filter((a) => !a.uploading).map((a) => a.id),
    })
      .then(() => {
        setSent(true);
        setTimeout(() => { onClose(); }, 1500);
      })
      .catch(() => {})
      .finally(() => setSending(false));
  }

  const intentLabel = intent === 'reply' ? '답장' : intent === 'reply_all' ? '전체 답장' : '전달';

  function fmtSize(bytes: number): string {
    if (bytes < 1024) return `${bytes}B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)}KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
  }

  return (
    <div style={{ marginTop: '24px', maxWidth: '680px', borderRadius: '8px 8px 0 0', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', overflow: 'hidden' }}>
      {/* Header */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '10px 14px', borderBottom: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)' }}>
        <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)', flex: 1 }}>
          {intentLabel}: {to}
        </span>
        <button
          type="button"
          aria-label="새창으로 열기"
          title="새창으로 열기"
          onClick={onOpenFullModal}
          style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', padding: '4px 6px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', borderRadius: '4px' }}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
        >
          <ArrowTopRightOnSquareIcon style={{ width: '15px', height: '15px' }} />
        </button>
        <button
          type="button"
          aria-label="닫기"
          onClick={onClose}
          style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', padding: '4px 6px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', borderRadius: '4px' }}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
        >
          <XMarkIcon style={{ width: '15px', height: '15px' }} />
        </button>
      </div>

      {/* CC row */}
      <div style={{ padding: '0 14px', borderBottom: '1px solid var(--color-border-subtle)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', minHeight: '32px' }}>
          <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>참조:</span>
          {showCc ? (
            <input
              type="text"
              value={cc}
              onChange={(e) => setCc(e.target.value)}
              placeholder="참조 주소..."
              style={{ flex: 1, border: 'none', outline: 'none', fontSize: '12px', background: 'transparent', color: 'var(--color-text-primary)' }}
            />
          ) : (
            <button
              type="button"
              onClick={() => setShowCc(true)}
              style={{ fontSize: '12px', color: 'var(--color-accent)', background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
            >
              참조 추가
            </button>
          )}
        </div>
      </div>

      {/* Toolbar */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '2px', padding: '4px 10px', borderBottom: '1px solid var(--color-border-subtle)', flexWrap: 'wrap' }}>
        <button type="button" aria-label="굵게" title="굵게" style={toolbarBtnStyleInline(editor?.isActive('bold'))} onClick={() => editor?.chain().focus().toggleBold().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bold') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><b>B</b></button>
        <button type="button" aria-label="기울임" title="기울임" style={toolbarBtnStyleInline(editor?.isActive('italic'))} onClick={() => editor?.chain().focus().toggleItalic().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('italic') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><i>I</i></button>
        <button type="button" aria-label="밑줄" title="밑줄" style={toolbarBtnStyleInline(editor?.isActive('underline'))} onClick={() => editor?.chain().focus().toggleUnderline().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('underline') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><u>U</u></button>
        <button type="button" aria-label="글머리 목록" title="글머리 목록" style={toolbarBtnStyleInline(editor?.isActive('bulletList'))} onClick={() => editor?.chain().focus().toggleBulletList().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bulletList') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><ListBulletIcon style={{ width: '14px', height: '14px' }} /></button>
        <button type="button" aria-label="번호 목록" title="번호 목록" style={toolbarBtnStyleInline(editor?.isActive('orderedList'))} onClick={() => editor?.chain().focus().toggleOrderedList().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('orderedList') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><NumberedListIcon style={{ width: '14px', height: '14px' }} /></button>
        <button type="button" aria-label="링크" title="링크" style={toolbarBtnStyleInline(editor?.isActive('link'))} onClick={handleLinkInsert} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('link') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><LinkIcon style={{ width: '14px', height: '14px' }} /></button>
        <button type="button" aria-label="이미지 삽입" title="이미지 삽입" style={toolbarBtnStyleInline()} onClick={() => imageInputRef.current?.click()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}><PhotoIcon style={{ width: '14px', height: '14px' }} /></button>
        <div style={{ width: '1px', height: '16px', background: 'var(--color-border-subtle)', margin: '0 2px' }} />
        <button type="button" aria-label="파일 첨부" title="파일 첨부" style={toolbarBtnStyleInline()} onClick={() => fileInputRef.current?.click()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}><PaperClipIcon style={{ width: '14px', height: '14px' }} /></button>
      </div>

      {/* Hidden file inputs */}
      <input ref={fileInputRef} type="file" multiple style={{ display: 'none' }} onChange={(e) => { if (e.target.files?.length) { void handleFileAttach(e.target.files); e.target.value = ''; } }} />
      <input ref={imageInputRef} type="file" accept="image/*" style={{ display: 'none' }} onChange={(e) => { if (e.target.files?.[0]) { void handleImageFile(e.target.files[0]); e.target.value = ''; } }} />

      {/* Editor body */}
      <div
        style={{ minHeight: '120px', padding: '12px 14px', cursor: 'text' }}
        onClick={() => editor?.commands.focus()}
      >
        <EditorContent
          editor={editor}
          style={{ outline: 'none', fontSize: '14px', lineHeight: 1.6, color: 'var(--color-text-primary)' }}
        />
      </div>

      {/* Attachment chips */}
      {attachments.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', padding: '0 14px 8px' }}>
          {attachments.map((att) => (
            <span key={att.id} style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', padding: '3px 8px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)', borderRadius: '4px', fontSize: '12px', color: 'var(--color-text-secondary)' }}>
              <PaperClipIcon style={{ width: '12px', height: '12px' }} />
              {att.filename} {att.uploading ? '(업로드 중...)' : `(${fmtSize(att.size)})`}
              {!att.uploading && (
                <button type="button" onClick={() => setAttachments((prev) => prev.filter((a) => a.id !== att.id))} style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, lineHeight: 1, color: 'var(--color-text-tertiary)' }}>×</button>
              )}
            </span>
          ))}
        </div>
      )}

      {/* Footer / send */}
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', padding: '8px 14px', background: 'var(--color-bg-secondary)', borderTop: '1px solid var(--color-border-subtle)' }}>
        <button
          type="button"
          disabled={sending}
          onClick={doSend}
          style={{ padding: '6px 20px', borderRadius: '5px', border: 'none', background: sending ? 'var(--color-border-default)' : 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 500, cursor: sending ? 'not-allowed' : 'pointer' }}
        >
          {sent ? '전송됨 ✓' : sending ? '전송 중...' : '전송'}
        </button>
      </div>
    </div>
  );
}

export function ReadingPane({
  message,
  folders = [],
  onArchive,
  onSpam,
  onNotSpam,
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
  onRestore,
  onSnooze,
  onOpenInWindow,
  onToggleRead,
  isRead,
  onStar,
  isStarred,
  threadMessages,
  onSelectThread,
  userEmail,
  externalImages = 'ask',
}: ReadingPaneProps) {
  const [showMoreMenu, setShowMoreMenu] = useState(false);
  const moreMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!showMoreMenu) return;
    function onDown(e: MouseEvent) {
      if (moreMenuRef.current && !moreMenuRef.current.contains(e.target as Node)) setShowMoreMenu(false);
    }
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [showMoreMenu]);
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

  const [savedContact, setSavedContact] = useState(false);
  const [scrollProgress, setScrollProgress] = useState(0);
  function handleReadingScroll() {
    const el = scrollContainerRef.current;
    if (!el) return;
    const max = el.scrollHeight - el.clientHeight;
    setScrollProgress(max > 0 ? Math.round((el.scrollTop / max) * 100) : 0);
  }

  function handleSaveContact() {
    if (!message) return;
    try {
      const contacts: Record<string, string> = JSON.parse(localStorage.getItem('webmail_contacts') ?? '{}');
      contacts[message.from_addr.toLowerCase()] = message.from_name || message.from_addr;
      localStorage.setItem('webmail_contacts', JSON.stringify(contacts));
    } catch { /* ignore */ }
    setSavedContact(true);
    setTimeout(() => setSavedContact(false), 2000);
  }

  const isContactSaved = useMemo(() => {
    if (!message) return false;
    try {
      const contacts: Record<string, string> = JSON.parse(localStorage.getItem('webmail_contacts') ?? '{}');
      return message.from_addr.toLowerCase() in contacts;
    } catch { return false; }
  }, [message, savedContact]);

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
  const [deliveryStatus, setDeliveryStatus] = useState<MessageDeliveryStatus | null>(null);
  const [deliveryOpen, setDeliveryOpen] = useState(false);

  useEffect(() => {
    if (!message?.has_attachment || !message.id) { setAttachments([]); return; }
    setAttachmentsLoading(true);
    listAttachments(message.id)
      .then(setAttachments)
      .catch(() => setAttachments([]))
      .finally(() => setAttachmentsLoading(false));
  }, [message?.id, message?.has_attachment]);

  interface ICSEvent { summary: string; dtstart: string; dtend?: string; location?: string; description?: string; }
  const [icsEvents, setIcsEvents] = useState<ICSEvent[]>([]);
  const [addingCalendarId, setAddingCalendarId] = useState<string | null>(null);
  const [calendarAdded, setCalendarAdded] = useState<string | null>(null);

  useEffect(() => {
    if (attachments.length === 0) { setIcsEvents([]); return; }
    const icsAtts = attachments.filter((a) => a.filename.toLowerCase().endsWith('.ics') || a.mime_type === 'text/calendar');
    if (icsAtts.length === 0) { setIcsEvents([]); return; }
    Promise.all(icsAtts.map(async (att) => {
      if (!message) return null;
      try {
        const resp = await fetch(`/api/mail/messages/${message.id}/attachments/${att.id}/download`);
        if (!resp.ok) return null;
        const text = await resp.text();
        const get = (key: string) => { const m = text.match(new RegExp(`^${key}[;:][^:]*:?(.+)$`, 'mi')); return m ? m[1].trim() : undefined; };
        const summary = get('SUMMARY');
        const dtstart = get('DTSTART');
        if (!summary || !dtstart) return null;
        return { summary, dtstart, dtend: get('DTEND'), location: get('LOCATION'), description: get('DESCRIPTION') } as ICSEvent;
      } catch { return null; }
    })).then((results) => setIcsEvents(results.filter(Boolean) as ICSEvent[]));
  }, [attachments, message]);

  async function handleAddToCalendar(ev: ICSEvent) {
    setAddingCalendarId(ev.dtstart);
    try {
      const cals = await listCalendars();
      const cal = cals[0];
      if (!cal) return;
      const parseDate = (s: string) => {
        const clean = s.replace(/[TZ]/g, (c) => c === 'T' ? 'T' : '');
        if (s.length === 8) return new Date(`${s.slice(0,4)}-${s.slice(4,6)}-${s.slice(6,8)}T00:00:00`);
        return new Date(`${clean.slice(0,4)}-${clean.slice(4,6)}-${clean.slice(6,8)}T${clean.slice(9,11)}:${clean.slice(11,13)}:${clean.slice(13,15)}`);
      };
      const start = parseDate(ev.dtstart);
      const end = ev.dtend ? parseDate(ev.dtend) : new Date(start.getTime() + 3600000);
      await createCalendarEvent(cal.ID, { title: ev.summary, start, end, allDay: ev.dtstart.length === 8, location: ev.location, description: ev.description });
      setCalendarAdded(ev.dtstart);
      setTimeout(() => setCalendarAdded(null), 3000);
    } catch { /* ignore */ }
    finally { setAddingCalendarId(null); }
  }

  const isSent = userEmail && message?.from_addr
    ? message.from_addr.toLowerCase() === userEmail.toLowerCase()
    : false;

  useEffect(() => {
    setDeliveryStatus(null);
    setDeliveryOpen(false);
    if (!message?.id || !isSent) return;
    getMessageDeliveryStatus(message.id).then(setDeliveryStatus).catch(() => {});
  }, [message?.id, isSent]);

  const handleDownload = useCallback(async (att: Attachment) => {
    if (!message) return;
    setDownloadingId(att.id);
    try { await downloadAttachment(message.id, att.id, att.filename); } catch { /* ignore */ }
    finally { setDownloadingId(null); }
  }, [message]);

  const [savingToDriveId, setSavingToDriveId] = useState<string | null>(null);
  const [driveToast, setDriveToast] = useState('');

  const handleSaveToDrive = useCallback(async (att: Attachment) => {
    if (!message) return;
    setSavingToDriveId(att.id);
    try {
      const node = await saveAttachmentToDrive(message.id, att.id, att.filename, att.mime_type);
      setDriveToast(node ? `"${att.filename}" 드라이브에 저장됨` : '드라이브 저장 실패');
      setTimeout(() => setDriveToast(''), 3000);
    } catch { setDriveToast('드라이브 저장 실패'); setTimeout(() => setDriveToast(''), 3000); }
    finally { setSavingToDriveId(null); }
  }, [message]);

  const [imagePreviews, setImagePreviews] = useState<Record<string, string>>({});
  const imagePreviewsRef = useRef<Record<string, string>>({});
  const [lightbox, setLightbox] = useState<{ url: string; filename: string; attId: string } | null>(null);
  useEffect(() => {
    if (!lightbox) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setLightbox(null); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [lightbox]);

  const [pdfPreview, setPdfPreview] = useState<{ url: string; filename: string } | null>(null);
  const [pdfPreviewLoadingId, setPdfPreviewLoadingId] = useState<string | null>(null);
  useEffect(() => {
    if (!pdfPreview) return;
    const onKey = (e: KeyboardEvent) => { if (e.key === 'Escape') setPdfPreview(null); };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [pdfPreview]);
  useEffect(() => {
    const url = pdfPreview?.url;
    return () => { if (url) URL.revokeObjectURL(url); };
  }, [pdfPreview]);

  const handlePdfPreview = useCallback(async (att: Attachment) => {
    if (!message) return;
    setPdfPreviewLoadingId(att.id);
    try {
      const res = await fetch(`/api/mail/messages/${message.id}/attachments/${att.id}/download`);
      if (!res.ok) return;
      const blob = await res.blob();
      setPdfPreview({ url: URL.createObjectURL(blob), filename: att.filename });
    } catch { /* ignore */ }
    finally { setPdfPreviewLoadingId(null); }
  }, [message]);

  const [emailDarkMode, setEmailDarkMode] = useState(false);

  useEffect(() => {
    const imageAtts = attachments.filter((a) => a.mime_type.startsWith('image/') && a.status === 'stored');
    if (!message || imageAtts.length === 0) return;
    let cancelled = false;
    imageAtts.forEach((att) => {
      if (imagePreviewsRef.current[att.id]) return;
      fetch(`/api/mail/messages/${message.id}/attachments/${att.id}/download`)
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

  // Inline compose state
  const [inlineCompose, setInlineCompose] = useState<{
    intent: 'reply' | 'reply_all' | 'forward';
    to: string;
    subject: string;
  } | null>(null);

  useEffect(() => {
    setInlineCompose(null);
  }, [message?.id]);

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
      {/* Reading progress bar */}
      <div aria-hidden="true" style={{ position: 'sticky', top: 0, left: 0, height: '2px', width: `${scrollProgress}%`, background: 'var(--color-accent)', zIndex: 10, transition: 'width 80ms linear', flexShrink: 0, marginBottom: '-2px' }} />
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
            style={{ ...iconStyle, marginRight: 'auto', color: 'var(--color-text-secondary)', display: 'inline-flex', alignItems: 'center', gap: '4px' }}
          ><ArrowLeftIcon style={{ width: '16px', height: '16px' }} /> 뒤로</button>
        )}
        {(onPrev || onNext) && !onBack && <div style={{ marginRight: 'auto' }} />}
        {onPrev && (
          <button aria-label="이전 메일" title="이전 메일 (k)" onClick={onPrev}
            style={{ ...iconStyle, color: 'var(--color-text-secondary)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          ><ChevronUpIcon style={{ width: '16px', height: '16px' }} /></button>
        )}
        {messageIndex !== undefined && messageTotal !== undefined && (
          <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', minWidth: '40px', textAlign: 'center' }}>
            {messageIndex + 1} / {messageTotal}
          </span>
        )}
        {onNext && (
          <button aria-label="다음 메일" title="다음 메일 (j)" onClick={onNext}
            style={{ ...iconStyle, color: 'var(--color-text-secondary)', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          ><ChevronDownIcon style={{ width: '16px', height: '16px' }} /></button>
        )}
        {/* Icon-only primary actions */}
        {(
          [
            {
              icon: <ArrowUturnLeftIcon style={{ width: '16px', height: '16px' }} />,
              label: '답장 (R)',
              action: onReply,
              intent: 'reply' as const,
            },
            {
              icon: <ArrowUturnLeftIcon style={{ width: '16px', height: '16px', opacity: 0.7 }} />,
              label: '전체 답장 (A)',
              action: onReplyAll,
              intent: 'reply_all' as const,
            },
            {
              icon: <ArrowUturnRightIcon style={{ width: '16px', height: '16px' }} />,
              label: '전달 (F)',
              action: onForward,
              intent: 'forward' as const,
            },
          ] as Array<{ icon: ReactNode; label: string; action: (() => void) | undefined; intent: 'reply' | 'reply_all' | 'forward' }>
        ).map(({ icon, label, action, intent }) => action ? (
          <button key={label} aria-label={label} title={label} onClick={() => {
            const to = intent === 'reply'
              ? message.from_addr
              : intent === 'reply_all'
              ? [message.from_addr, ...(message.to_addrs ?? []).map((t) => t.address), ...(message.cc_addrs ?? []).map((t) => t.address)].filter((a, i, arr) => arr.indexOf(a) === i).join(', ')
              : '';
            const subject = intent === 'forward'
              ? (message.subject?.startsWith('Fwd:') ? message.subject : `Fwd: ${message.subject ?? ''}`)
              : (message.subject?.startsWith('Re:') ? message.subject : `Re: ${message.subject ?? ''}`);
            setInlineCompose({ intent, to, subject });
            setTimeout(() => {
              scrollContainerRef.current?.scrollTo({ top: scrollContainerRef.current.scrollHeight, behavior: 'smooth' });
            }, 50);
          }}
            style={{ ...iconStyle, padding: '5px 8px', border: 'none', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >{icon}</button>
        ) : null)}

        <div style={{ width: '1px', height: '16px', background: 'var(--color-border-subtle)', margin: '0 2px' }} />

        {/* Star */}
        {onStar && (
          <button aria-label={isStarred ? '별표 해제' : '별표'} title={isStarred ? '별표 해제 (S)' : '별표 (S)'} onClick={onStar}
            style={{ ...iconStyle, border: 'none', padding: '5px 8px', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', color: isStarred ? '#f59e0b' : 'var(--color-text-secondary)' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >{isStarred ? <StarIconSolid style={{ width: '16px', height: '16px' }} /> : <StarIcon style={{ width: '16px', height: '16px' }} />}</button>
        )}

        {/* Archive */}
        {onArchive && (
          <button aria-label="아카이브" title="아카이브 (E)" onClick={onArchive}
            style={{ ...iconStyle, border: 'none', padding: '5px 8px', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          ><ArchiveBoxIcon style={{ width: '16px', height: '16px' }} /></button>
        )}

        {/* Unsubscribe — detect link from html body */}
        {(() => {
          if (!message?.html_body) return null;
          const match = message.html_body.match(/href=["']([^"']*(?:unsubscribe|opt.?out|수신거부|구독취소)[^"']*)["']/i);
          if (!match) return null;
          const url = match[1].replace(/&amp;/g, '&');
          return (
            <button
              aria-label="구독 취소"
              title="구독 취소"
              onClick={() => window.open(url, '_blank', 'noopener,noreferrer')}
              style={{ ...iconStyle, border: '1px solid rgba(220,38,38,0.3)', padding: '4px 10px', display: 'inline-flex', alignItems: 'center', gap: '4px', borderRadius: '5px', background: 'rgba(220,38,38,0.04)', color: 'var(--color-destructive)', fontSize: '12px', fontWeight: 500 }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(220,38,38,0.1)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(220,38,38,0.04)'; }}
            >
              <NoSymbolIcon style={{ width: 13, height: 13 }} />
              구독 취소
            </button>
          );
        })()}

        {/* Open in window */}
        {onOpenInWindow && (
          <button aria-label="새 창으로 열기" title="새 창으로 열기" onClick={onOpenInWindow}
            style={{ ...iconStyle, border: 'none', padding: '5px 8px', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          ><ArrowTopRightOnSquareIcon style={{ width: '16px', height: '16px' }} /></button>
        )}

        {/* More menu */}
        <div ref={moreMenuRef} style={{ position: 'relative' }}>
          <button aria-label="더 보기" title="더 보기" onClick={() => setShowMoreMenu((v) => !v)}
            style={{ ...iconStyle, border: 'none', padding: '5px 8px', display: 'inline-flex', alignItems: 'center', justifyContent: 'center' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          ><EllipsisHorizontalIcon style={{ width: '18px', height: '18px' }} /></button>
          {showMoreMenu && (
            <div style={{ position: 'absolute', top: '100%', right: 0, marginTop: '4px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '8px', boxShadow: '0 4px 20px rgba(0,0,0,0.14)', zIndex: 300, minWidth: '200px', overflow: 'hidden' }}>
              {/* Move to folder */}
              {onMove && folders.length > 0 && (
                <>
                  <div style={{ padding: '6px 14px 2px', fontSize: '11px', color: 'var(--color-text-tertiary)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em' }}>이동</div>
                  {folders.map((f) => (
                    <button key={f.id} onClick={() => { onMove(f.id); setShowMoreMenu(false); }}
                      style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                    >{f.name}</button>
                  ))}
                  <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '4px 0' }} />
                </>
              )}
              {/* Snooze */}
              {onSnooze && message && (
                <>
                  <div style={{ padding: '6px 14px 2px', fontSize: '11px', color: 'var(--color-text-tertiary)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em' }}>스누즈</div>
                  {[
                    { label: '1시간 후', ms: 60 * 60 * 1000 },
                    { label: '4시간 후', ms: 4 * 60 * 60 * 1000 },
                    { label: '오늘 저녁 (18:00)', ms: (() => { const d = new Date(); d.setHours(18,0,0,0); return d.getTime() > Date.now() ? d.getTime() - Date.now() : 24 * 3600000; })() },
                    { label: '내일 오전 (09:00)', ms: (() => { const d = new Date(); d.setDate(d.getDate() + 1); d.setHours(9,0,0,0); return d.getTime() - Date.now(); })() },
                  ].map((opt) => (
                    <button key={opt.label} onClick={() => { onSnooze(message.id, new Date(Date.now() + opt.ms)); setShowMoreMenu(false); }}
                      style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                    >{opt.label}</button>
                  ))}
                  <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '4px 0' }} />
                </>
              )}
              {/* Font size */}
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px' }}>
                <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flex: 1 }}>글자 크기</span>
                <button onClick={() => setFontSize((f) => Math.max(11, f - 1))} style={{ fontSize: '12px', padding: '2px 7px', border: '1px solid var(--color-border-default)', borderRadius: '4px', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>A-</button>
                <span style={{ fontSize: '12px', color: 'var(--color-text-primary)', minWidth: '20px', textAlign: 'center' }}>{fontSize}</span>
                <button onClick={() => setFontSize((f) => Math.min(24, f + 1))} style={{ fontSize: '12px', padding: '2px 7px', border: '1px solid var(--color-border-default)', borderRadius: '4px', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>A+</button>
              </div>
              {/* Print */}
              {onPrint && (
                <button onClick={() => { onPrint(); setShowMoreMenu(false); }}
                  style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >인쇄</button>
              )}
              {/* Toggle read */}
              {onToggleRead && (
                <button onClick={() => { onToggleRead(); setShowMoreMenu(false); }}
                  style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >{isRead ? '안읽음으로 표시' : '읽음으로 표시'}</button>
              )}
              {/* Spam / Not Spam / Restore */}
              {(onSpam || onNotSpam || onRestore) && <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '4px 0' }} />}
              {onSpam && <button onClick={() => { onSpam(); setShowMoreMenu(false); }} style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', cursor: 'pointer' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>스팸 신고</button>}
              {onNotSpam && <button onClick={() => { onNotSpam(); setShowMoreMenu(false); }} style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>스팸 아님</button>}
              {onRestore && <button onClick={() => { onRestore(); setShowMoreMenu(false); }} style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>복구</button>}
              {/* Unsubscribe */}
              {unsubscribeUrl && (
                <>
                  <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '4px 0' }} />
                  <button onClick={() => { if (window.confirm('수신거부 링크를 열겠습니까?')) window.open(unsubscribeUrl, '_blank', 'noopener,noreferrer'); setShowMoreMenu(false); }}
                    style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', cursor: 'pointer' }}
                    onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                  >수신거부</button>
                </>
              )}
            </div>
          )}
        </div>
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
              {!isContactSaved && (
                <button
                  onClick={handleSaveContact}
                  title="연락처에 추가"
                  style={{ background: 'none', border: '1px solid var(--color-border-default)', borderRadius: '4px', cursor: 'pointer', fontSize: '11px', color: savedContact ? 'var(--color-accent)' : 'var(--color-text-tertiary)', padding: '1px 6px', marginInlineStart: '4px', lineHeight: 1.4 }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >{savedContact ? '저장됨 ✓' : '연락처에 추가'}</button>
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

        {/* Delivery status — only for sent messages */}
        {isSent && deliveryStatus && (
          <div style={{ marginBottom: '16px', maxWidth: '680px' }}>
            <button
              onClick={() => setDeliveryOpen((v) => !v)}
              style={{ display: 'flex', alignItems: 'center', gap: '6px', background: 'none', border: 'none', cursor: 'pointer', padding: 0, fontSize: '12px', fontWeight: 600, color: 'var(--color-text-tertiary)', letterSpacing: '0.05em', textTransform: 'uppercase', marginBottom: deliveryOpen ? '8px' : 0 }}
            >
              <span style={{ fontSize: '11px', transform: deliveryOpen ? 'rotate(90deg)' : 'rotate(0deg)', display: 'inline-block', transition: 'transform 150ms' }}>▶</span>
              배달 현황
              <span style={{ fontWeight: 400, textTransform: 'none', letterSpacing: 0, fontSize: '12px', color: deliveryStatus.delivery_status === 'delivered' ? 'var(--color-success, #22c55e)' : deliveryStatus.delivery_status === 'failed' ? 'var(--color-destructive)' : 'var(--color-text-tertiary)' }}>
                ({deliveryStatus.delivery_status === 'delivered' ? '전달됨' : deliveryStatus.delivery_status === 'failed' ? '실패' : deliveryStatus.delivery_status === 'partial' ? '일부 실패' : '대기 중'})
              </span>
            </button>
            {deliveryOpen && (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                {deliveryStatus.attempts.length === 0 ? (
                  <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', padding: '6px 0' }}>배달 기록이 없습니다.</div>
                ) : deliveryStatus.attempts.map((att, i) => {
                  const isOk = att.status === 'delivered' || att.status === 'success';
                  const isFail = att.status === 'failed' || att.status === 'bounced' || att.status === 'error';
                  const statusColor = isOk ? 'var(--color-success, #22c55e)' : isFail ? 'var(--color-destructive)' : 'var(--color-text-tertiary)';
                  const statusLabel = isOk ? '전달됨' : isFail ? '실패' : att.status === 'pending' ? '대기 중' : att.status;
                  const dot = isOk ? '●' : isFail ? '●' : '○';
                  return (
                    <div key={i} style={{ display: 'flex', alignItems: 'flex-start', gap: '8px', padding: '6px 10px', borderRadius: '5px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)' }}>
                      <span style={{ color: statusColor, fontSize: '10px', marginTop: '2px' }}>{dot}</span>
                      <div style={{ flex: 1, minWidth: 0 }}>
                        <div style={{ fontSize: '13px', color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{att.recipient}</div>
                        {att.error_message && <div style={{ fontSize: '11px', color: 'var(--color-destructive)', marginTop: '2px' }}>{att.error_message}</div>}
                      </div>
                      <div style={{ flexShrink: 0, textAlign: 'right' }}>
                        <div style={{ fontSize: '11px', fontWeight: 600, color: statusColor }}>{statusLabel}</div>
                        {att.attempted_at && <div style={{ fontSize: '10px', color: 'var(--color-text-tertiary)', marginTop: '1px' }}>{new Intl.DateTimeFormat('ko-KR', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(att.attempted_at))}</div>}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}

        {/* Calendar invite cards */}
        {icsEvents.length > 0 && (
          <div style={{ marginBottom: '16px', maxWidth: '680px', display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {icsEvents.map((ev) => {
              const fmtDt = (s: string) => {
                try {
                  const clean = s.replace('Z','');
                  const d = s.length === 8
                    ? new Date(`${s.slice(0,4)}-${s.slice(4,6)}-${s.slice(6,8)}`)
                    : new Date(`${clean.slice(0,4)}-${clean.slice(4,6)}-${clean.slice(6,8)}T${clean.slice(9,11)}:${clean.slice(11,13)}:${clean.slice(13,15)}`);
                  return new Intl.DateTimeFormat('ko-KR', { dateStyle: 'medium', timeStyle: s.length === 8 ? undefined : 'short', hour12: false }).format(d);
                } catch { return s; }
              };
              const added = calendarAdded === ev.dtstart;
              const adding = addingCalendarId === ev.dtstart;
              return (
                <div key={ev.dtstart} style={{ display: 'flex', alignItems: 'flex-start', gap: '12px', padding: '12px 14px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)' }}>
                  <div style={{ flexShrink: 0, width: '40px', height: '40px', borderRadius: '8px', background: 'var(--color-accent)', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '20px' }}>📅</div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontWeight: 600, fontSize: '14px', color: 'var(--color-text-primary)', marginBottom: '3px' }}>{ev.summary}</div>
                    <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{fmtDt(ev.dtstart)}{ev.dtend ? ` ~ ${fmtDt(ev.dtend)}` : ''}</div>
                    {ev.location && <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>📍 {ev.location}</div>}
                  </div>
                  <button
                    onClick={() => handleAddToCalendar(ev)}
                    disabled={adding || added}
                    style={{ flexShrink: 0, padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: added ? 'var(--color-accent-subtle)' : 'transparent', color: added ? 'var(--color-accent)' : 'var(--color-text-primary)', fontSize: '12px', cursor: adding || added ? 'default' : 'pointer', fontWeight: 500, whiteSpace: 'nowrap' }}
                  >{adding ? '추가 중...' : added ? '✓ 추가됨' : '캘린더에 추가'}</button>
                </div>
              );
            })}
          </div>
        )}

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
                <PaperClipIcon aria-hidden="true" style={{ width: '14px', height: '14px', flexShrink: 0 }} />
                <span>첨부파일을 불러올 수 없습니다</span>
              </div>
            ) : (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                {attachments.map((att) => {
                  const isImg = att.mime_type.startsWith('image/');
                  const isPdf = att.mime_type === 'application/pdf';
                  const icon = isImg
                    ? <PhotoIcon style={{ width: '14px', height: '14px' }} />
                    : isPdf
                    ? <DocumentIcon style={{ width: '14px', height: '14px' }} />
                    : <PaperClipIcon style={{ width: '14px', height: '14px' }} />;
                  const kb = att.size < 1024 * 1024 ? `${Math.round(att.size / 1024)} KB` : `${(att.size / 1024 / 1024).toFixed(1)} MB`;
                  const previewUrl = isImg ? imagePreviews[att.id] : undefined;
                  return (
                    <div key={att.id} style={{ display: 'inline-flex', flexDirection: 'column', gap: '4px', maxWidth: '220px' }}>
                      {previewUrl && (
                        <button
                          onClick={() => setLightbox({ url: previewUrl, filename: att.filename, attId: att.id })}
                          title={`${att.filename} 미리보기`}
                          style={{ border: '1px solid var(--color-border-default)', borderRadius: '6px', overflow: 'hidden', background: 'none', cursor: 'zoom-in', padding: 0, display: 'block' }}
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
                        <span aria-hidden="true" style={{ display: 'inline-flex', alignItems: 'center' }}>{downloadingId === att.id ? <ArrowPathIcon style={{ width: '14px', height: '14px', animation: 'spin 1s linear infinite' }} /> : icon}</span>
                        <span style={{ flex: 1, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0 }}>{att.filename}</span>
                        <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>{kb}</span>
                      </button>
                      {isPdf && (
                        <button
                          onClick={() => handlePdfPreview(att)}
                          disabled={pdfPreviewLoadingId === att.id}
                          title="PDF 미리보기"
                          style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '4px', padding: '4px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '11px', cursor: pdfPreviewLoadingId === att.id ? 'wait' : 'pointer', width: '100%' }}
                          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                        >
                          {pdfPreviewLoadingId === att.id ? <ArrowPathIcon style={{ width: '11px', height: '11px', animation: 'spin 1s linear infinite' }} /> : '👁'}
                          {pdfPreviewLoadingId === att.id ? '로딩 중...' : 'PDF 미리보기'}
                        </button>
                      )}
                      <button
                        onClick={() => handleSaveToDrive(att)}
                        disabled={savingToDriveId === att.id}
                        title="드라이브에 저장"
                        style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', gap: '4px', padding: '4px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '11px', cursor: savingToDriveId === att.id ? 'wait' : 'pointer', width: '100%' }}
                        onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                        onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                      >
                        {savingToDriveId === att.id ? <ArrowPathIcon style={{ width: '11px', height: '11px', animation: 'spin 1s linear infinite' }} /> : '☁'}
                        {savingToDriveId === att.id ? '저장 중...' : '드라이브에 저장'}
                      </button>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}

        {/* Body */}
        {message.html_body && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginBottom: '4px' }}>
            <button
              onClick={() => setEmailDarkMode((v) => !v)}
              title={emailDarkMode ? '라이트 모드로 보기' : '다크 모드로 보기'}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: emailDarkMode ? 'var(--color-bg-tertiary)' : 'transparent', color: 'var(--color-text-secondary)', fontSize: '11px', cursor: 'pointer' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = emailDarkMode ? 'var(--color-bg-tertiary)' : 'transparent'; }}
            >
              {emailDarkMode ? '☀ 라이트' : '🌙 다크'}
            </button>
          </div>
        )}
        <div
          style={{
            maxWidth: '680px',
            fontSize: `${fontSize}px`,
            lineHeight: 1.6,
            color: 'var(--color-text-primary)',
            ...(emailDarkMode ? { filter: 'invert(1) hue-rotate(180deg)', background: '#000', borderRadius: '8px', padding: '12px' } : {}),
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
              {linkify(message.text_body || '(내용 없음)')}
            </pre>
          )}
        </div>

        {/* Thread view */}
        {threadMessages && threadMessages.length > 1 && (
          <div style={{ marginTop: '32px', borderTop: '1px solid var(--color-border-subtle)', paddingTop: '16px', maxWidth: '680px' }}>
            <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.07em', marginBottom: '12px' }}>
              대화 {threadMessages.length}개
            </div>
            <div style={{ position: 'relative' }}>
              <div style={{ position: 'absolute', left: '15px', top: '16px', bottom: '16px', width: '1px', background: 'var(--color-border-subtle)' }} />
              {threadMessages.map((msg) => {
                const isCurrent = msg.id === message.id;
                const isMine = userEmail ? msg.from_addr.toLowerCase() === userEmail.toLowerCase() : false;
                const date = new Intl.DateTimeFormat('ko-KR', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(msg.received_at));
                const initial = (msg.from_name || msg.from_addr)[0]?.toUpperCase() ?? '?';
                return (
                  <div
                    key={msg.id}
                    onClick={() => !isCurrent && onSelectThread?.(msg.id)}
                    style={{ display: 'flex', flexDirection: isMine ? 'row-reverse' : 'row', gap: '10px', padding: '4px 0', cursor: isCurrent ? 'default' : 'pointer', alignItems: 'flex-end' }}
                  >
                    {/* Avatar */}
                    <div style={{ flexShrink: 0, width: '26px', height: '26px', borderRadius: '50%', background: isCurrent ? 'var(--color-accent)' : 'var(--color-bg-tertiary)', color: isCurrent ? '#fff' : 'var(--color-text-secondary)', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '11px', fontWeight: 700, border: `2px solid ${isCurrent ? 'var(--color-accent)' : 'var(--color-border-default)'}`, boxSizing: 'border-box' }}>
                      {initial}
                    </div>
                    {/* Bubble */}
                    <div
                      style={{ maxWidth: '75%', background: isCurrent ? 'var(--color-accent)' : isMine ? 'var(--color-bg-secondary)' : 'var(--color-bg-tertiary)', borderRadius: isMine ? '12px 12px 4px 12px' : '12px 12px 12px 4px', padding: '8px 12px', opacity: isCurrent ? 1 : 0.88, boxShadow: isCurrent ? '0 1px 4px rgba(0,0,0,0.15)' : 'none' }}
                      onMouseEnter={(e) => { if (!isCurrent) (e.currentTarget as HTMLDivElement).style.opacity = '1'; }}
                      onMouseLeave={(e) => { if (!isCurrent) (e.currentTarget as HTMLDivElement).style.opacity = '0.88'; }}
                    >
                      <div style={{ display: 'flex', alignItems: 'baseline', gap: '8px', marginBottom: '3px' }}>
                        {!isMine && <span style={{ fontSize: '11px', fontWeight: 600, color: isCurrent ? 'rgba(255,255,255,0.9)' : 'var(--color-text-secondary)' }}>{msg.from_name || msg.from_addr}</span>}
                        <span style={{ fontSize: '10px', color: isCurrent ? 'rgba(255,255,255,0.7)' : 'var(--color-text-tertiary)', marginLeft: 'auto' }}>{date}</span>
                      </div>
                      <div style={{ fontSize: '12px', color: isCurrent ? '#fff' : 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '280px' }}>
                        {msg.preview || msg.subject || '(내용 없음)'}
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* Quick reply */}
        {onQuickReply && (
          <div style={{ marginTop: '24px', borderTop: '1px solid var(--color-border-subtle)', paddingTop: '16px' }}>
            {!quickReplyOpen ? (
              <>
                <div style={{ display: 'flex', gap: '8px', flexWrap: 'wrap', marginBottom: '10px' }}>
                  {getSmartReplies(message.subject, message.text_body || '').map((reply) => (
                    <button
                      key={reply}
                      onClick={() => { setQuickReplyText(reply); setQuickReplyOpen(true); setTimeout(() => quickReplyRef.current?.focus(), 50); }}
                      style={{ padding: '6px 12px', borderRadius: '16px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer', flexShrink: 0, transition: 'border-color 120ms, color 120ms' }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.borderColor = 'var(--color-accent)'; (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-accent)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.borderColor = 'var(--color-border-default)'; (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-secondary)'; }}
                    >
                      {reply}
                    </button>
                  ))}
                </div>
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
              </>
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

        {/* Inline compose editor */}
        {inlineCompose && (
          <InlineCompose
            intent={inlineCompose.intent}
            to={inlineCompose.to}
            subject={inlineCompose.subject}
            messageId={message.id}
            sourceText={message.text_body}
            onClose={() => setInlineCompose(null)}
            onOpenFullModal={() => {
              const cb = inlineCompose.intent === 'reply' ? onReply
                : inlineCompose.intent === 'reply_all' ? onReplyAll
                : onForward;
              setInlineCompose(null);
              cb?.();
            }}
            userEmail={userEmail}
          />
        )}
      </div>

      {/* Drive save toast */}
      {driveToast && (
        <div style={{ position: 'fixed', bottom: '24px', left: '50%', transform: 'translateX(-50%)', background: 'var(--color-text-primary)', color: 'var(--color-bg-primary)', fontSize: '13px', padding: '8px 18px', borderRadius: '20px', zIndex: 600, boxShadow: '0 4px 12px rgba(0,0,0,0.2)', whiteSpace: 'nowrap', pointerEvents: 'none' }}>
          {driveToast}
        </div>
      )}

      {/* PDF preview modal */}
      {pdfPreview && (
        <>
          <div aria-hidden="true" onClick={() => setPdfPreview(null)} style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.7)', zIndex: 500, cursor: 'pointer' }} />
          <div role="dialog" aria-label={pdfPreview.filename} aria-modal="true" style={{ position: 'fixed', inset: '32px', zIndex: 501, display: 'flex', flexDirection: 'column', borderRadius: '10px', overflow: 'hidden', boxShadow: '0 16px 48px rgba(0,0,0,0.5)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '10px 16px', background: 'var(--color-bg-secondary)', borderBottom: '1px solid var(--color-border-default)' }}>
              <span style={{ flex: 1, fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{pdfPreview.filename}</span>
              <button onClick={() => { const a = attachments.find((x) => pdfPreview && x.filename === pdfPreview.filename); if (a) handleDownload(a); }} style={{ padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '12px', cursor: 'pointer' }}>다운로드</button>
              <button onClick={() => setPdfPreview(null)} aria-label="닫기" style={{ padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '12px', cursor: 'pointer' }}>닫기</button>
            </div>
            <iframe src={pdfPreview.url} title={pdfPreview.filename} style={{ flex: 1, border: 'none', background: '#fff' }} />
          </div>
        </>
      )}

      {/* Image lightbox */}
      {lightbox && (
        <>
          <div
            aria-hidden="true"
            onClick={() => setLightbox(null)}
            style={{ position: 'fixed', inset: 0, background: 'rgba(0,0,0,0.85)', zIndex: 500, cursor: 'zoom-out' }}
          />
          <div
            role="dialog"
            aria-label={lightbox.filename}
            aria-modal="true"
            style={{ position: 'fixed', inset: 0, zIndex: 501, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: '12px', padding: '24px', pointerEvents: 'none' }}
          >
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={lightbox.url}
              alt={lightbox.filename}
              style={{ maxWidth: '90vw', maxHeight: '80vh', objectFit: 'contain', borderRadius: '6px', boxShadow: '0 8px 32px rgba(0,0,0,0.4)', pointerEvents: 'auto' }}
            />
            <div style={{ display: 'flex', alignItems: 'center', gap: '12px', pointerEvents: 'auto' }}>
              <span style={{ color: 'rgba(255,255,255,0.8)', fontSize: '13px', maxWidth: '300px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{lightbox.filename}</span>
              <button
                onClick={() => { const a = attachments.find((x) => x.id === lightbox.attId); if (a) handleDownload(a); }}
                style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid rgba(255,255,255,0.3)', background: 'rgba(255,255,255,0.1)', color: '#fff', fontSize: '13px', cursor: 'pointer' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.2)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.1)'; }}
              >다운로드</button>
              <button
                onClick={() => setLightbox(null)}
                aria-label="닫기"
                style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid rgba(255,255,255,0.3)', background: 'rgba(255,255,255,0.1)', color: '#fff', fontSize: '13px', cursor: 'pointer' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.2)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'rgba(255,255,255,0.1)'; }}
              >닫기</button>
            </div>
          </div>
        </>
      )}
    </main>
  );
}
