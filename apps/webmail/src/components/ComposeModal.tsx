'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Underline from '@tiptap/extension-underline';
import TextAlign from '@tiptap/extension-text-align';
import Placeholder from '@tiptap/extension-placeholder';
import { sendMessage, saveDraft, updateDraft, uploadAttachment, ComposeIntent, MessageDetail, SendMessageRequest } from '@/lib/api';
import { RecipientChips } from './RecipientChips';
import {
  PaperClipIcon,
  LinkIcon,
  PencilSquareIcon as PencilSquareIconHero,
  ClipboardDocumentIcon,
  ClockIcon,
  ExclamationTriangleIcon,
  ArrowPathIcon,
} from '@heroicons/react/24/outline';

interface ComposeModalProps {
  onClose: () => void;
  intent?: ComposeIntent;
  sourceMessage?: MessageDetail;
  draftMessage?: MessageDetail;
  userEmail?: string;
  initialTo?: string;
  isMobile?: boolean;
  windowOffset?: number;
}

function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
}

function parseAddr(raw: string): { address: string; name?: string } {
  const m = raw.match(/^(.+?)\s*<([^>]+)>$/);
  if (m) return { name: m[1].trim() || undefined, address: m[2].trim() };
  return { address: raw.trim() };
}

function parseAddrs(raw: string): { address: string; name?: string }[] {
  // Split on commas not inside angle brackets
  const parts: string[] = [];
  let depth = 0, start = 0;
  for (let i = 0; i < raw.length; i++) {
    if (raw[i] === '<') depth++;
    else if (raw[i] === '>') depth--;
    else if (raw[i] === ',' && depth === 0) {
      parts.push(raw.slice(start, i));
      start = i + 1;
    }
  }
  parts.push(raw.slice(start));
  return parts.map((p) => parseAddr(p.trim())).filter((a) => a.address);
}

function buildQuoteHTML(intent: string, source: MessageDetail): string {
  const from = source.from_name
    ? `${escapeHtml(source.from_name)} &lt;${escapeHtml(source.from_addr)}&gt;`
    : escapeHtml(source.from_addr);
  const date = new Intl.DateTimeFormat('ko-KR', {
    year: 'numeric', month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false,
  }).format(new Date(source.received_at));
  const bodyLines = (source.text_body || '')
    .split('\n')
    .map((line) => `<p>${escapeHtml(line) || '&nbsp;'}</p>`)
    .join('');
  const header = intent === 'forward'
    ? '<p><strong>---------- 전달된 메시지 ----------</strong></p>'
    : '<p><strong>--- 원본 메시지 ---</strong></p>';
  return `<p></p>${header}<blockquote><p><strong>보낸 사람:</strong> ${from}</p><p><strong>날짜:</strong> ${escapeHtml(date)}</p><p><strong>제목:</strong> ${escapeHtml(source.subject || '(제목 없음)')}</p><p>&nbsp;</p>${bodyLines}</blockquote>`;
}

const toolbarBtnStyle = (active?: boolean): React.CSSProperties => ({
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
});

export function ComposeModal({ onClose, intent = 'new', sourceMessage, draftMessage, userEmail, initialTo, isMobile, windowOffset = 0 }: ComposeModalProps) {
  const replyTo = intent === 'reply' || intent === 'reply_all'
    ? sourceMessage?.from_addr ?? ''
    : '';
  const replyCc = intent === 'reply_all' && sourceMessage
    ? (sourceMessage.to_addrs ?? [])
        .map((a) => a.address)
        .filter((addr) => !userEmail || addr.toLowerCase() !== userEmail.toLowerCase())
        .join(', ')
    : '';
  const replySubject = sourceMessage
    ? intent === 'forward'
      ? `Fwd: ${sourceMessage.subject}`
      : `Re: ${sourceMessage.subject}`
    : '';

  const draftTo = draftMessage ? (draftMessage.to_addrs ?? []).map((a) => a.address).join(', ') : '';
  const draftCc = draftMessage ? (draftMessage.cc_addrs ?? []).map((a) => a.address).join(', ') : '';

  const [to, setTo] = useState(draftMessage ? draftTo : (initialTo ?? replyTo));
  const [cc, setCc] = useState(draftMessage ? draftCc : replyCc);
  const [bcc, setBcc] = useState('');
  const [showCc, setShowCc] = useState(!!(draftMessage ? draftCc : replyCc));
  const [showBcc, setShowBcc] = useState(false);
  const [subject, setSubject] = useState(draftMessage ? (draftMessage.subject ?? '') : replySubject);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState('');
  const [sent, setSent] = useState(false);
  const [sendCountdown, setSendCountdown] = useState<number | null>(null);
  const pendingMsgRef = useRef<SendMessageRequest | null>(null);
  const [scheduledAt, setScheduledAt] = useState('');
  const [showSchedule, setShowSchedule] = useState(false);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [savedAt, setSavedAt] = useState('');
  const [minimized, setMinimized] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const [confirmClose, setConfirmClose] = useState(false);
  const [showSigEditor, setShowSigEditor] = useState(false);
  const [signature, setSignature] = useState(() => {
    try { return localStorage.getItem('webmail_signature') ?? ''; } catch { return ''; }
  });
  const [recentRecipients] = useState<string[]>(() => {
    try {
      const recents: string[] = JSON.parse(localStorage.getItem('webmail_recent_recipients') ?? '[]');
      const contacts: Record<string, string> = JSON.parse(localStorage.getItem('webmail_contacts') ?? '{}');
      // Enrich plain email entries with stored contact names
      const enriched = recents.map((r) => {
        if (r.includes('<')) return r;
        const name = contacts[r.toLowerCase()];
        return name ? `${name} <${r}>` : r;
      });
      // Add contacts not yet in recents
      const recentEmails = new Set(recents.map((r) => { const m = r.match(/<([^>]+)>/); return (m ? m[1] : r).toLowerCase(); }));
      Object.entries(contacts).forEach(([email, name]) => {
        if (!recentEmails.has(email)) enriched.push(`${name} <${email}>`);
      });
      return enriched.slice(0, 50);
    } catch { return []; }
  });
  const draftIdRef = useRef<string>(draftMessage?.id ?? '');
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploadedAttachments, setUploadedAttachments] = useState<Array<{ id: string; filename: string; size: number; uploading?: boolean; error?: string }>>([]);
  const [dragOver, setDragOver] = useState(false);
  const dragCounterRef = useRef(0);
  const [showTemplates, setShowTemplates] = useState(false);
  const [templates, setTemplates] = useState<Array<{ name: string; subject: string; body: string }>>(() => {
    try { return JSON.parse(localStorage.getItem('webmail_templates') ?? '[]'); } catch { return []; }
  });

  const handleFileSelect = useCallback(async (files: FileList) => {
    const newFiles = Array.from(files);
    for (const file of newFiles) {
      const tempId = `tmp-${Math.random().toString(36).slice(2)}`;
      setUploadedAttachments((prev) => [...prev, { id: tempId, filename: file.name, size: file.size, uploading: true }]);
      try {
        const att = await uploadAttachment(file, draftIdRef.current || undefined);
        setUploadedAttachments((prev) => prev.map((a) => a.id === tempId ? { id: att.id, filename: att.filename, size: att.size } : a));
      } catch {
        setUploadedAttachments((prev) => prev.map((a) => a.id === tempId ? { ...a, uploading: false, error: '업로드 실패' } : a));
      }
    }
  }, []);

  const triggerAutoSave = useCallback((toVal: string, ccVal: string, bccVal: string, subjectVal: string, bodyText: string) => {
    if (saveTimerRef.current) clearTimeout(saveTimerRef.current);
    saveTimerRef.current = setTimeout(async () => {
      if (!toVal.trim() && !subjectVal.trim() && !bodyText.trim()) return;
      setSaveStatus('saving');
      try {
        const data = {
          intent,
          ...(intent !== 'new' && sourceMessage && { source_message_id: sourceMessage.id }),
          to: toVal.trim() ? [{ address: toVal.trim() }] : [],
          ...(ccVal.trim() && { cc: ccVal.split(',').map((a) => ({ address: a.trim() })).filter((a) => a.address) }),
          ...(bccVal.trim() && { bcc: bccVal.split(',').map((a) => ({ address: a.trim() })).filter((a) => a.address) }),
          subject: subjectVal,
          text_body: bodyText,
        };
        if (draftIdRef.current) {
          await updateDraft(draftIdRef.current, data);
        } else {
          const res = await saveDraft(data);
          draftIdRef.current = res.draft.id;
        }
        const now = new Date();
        setSavedAt(`${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`);
        setSaveStatus('saved');
      } catch {
        setSaveStatus('idle');
      }
    }, 3000);
  }, [intent, sourceMessage]);

  useEffect(() => {
    return () => { if (saveTimerRef.current) clearTimeout(saveTimerRef.current); };
  }, []);

  const toRef = useRef(draftMessage ? draftTo : replyTo);
  const ccRef = useRef(draftMessage ? draftCc : replyCc);
  const bccRef = useRef('');
  const subjectRef = useRef(draftMessage ? (draftMessage.subject ?? '') : replySubject);

  const sigHTML = signature.trim()
    ? `<p></p><p>--</p><p>${signature.trim().split('\n').map((l) => escapeHtml(l)).join('</p><p>')}</p>`
    : '';

  const initialContent = draftMessage
    ? (draftMessage.html_body ?? (draftMessage.text_body
        ? draftMessage.text_body.split('\n').map((l) => `<p>${escapeHtml(l) || '&nbsp;'}</p>`).join('')
        : ''))
    : (sourceMessage && (intent === 'reply' || intent === 'reply_all' || intent === 'forward')
        ? `<p></p>${sigHTML ? sigHTML + '<p></p>' : ''}${buildQuoteHTML(intent, sourceMessage)}`
        : `<p></p>${sigHTML}`);

  const editor = useEditor({
    extensions: [
      StarterKit,
      Underline,
      Link.configure({ openOnClick: false }),
      TextAlign.configure({ types: ['heading', 'paragraph'] }),
      Placeholder.configure({ placeholder: '메시지를 입력하세요...' }),
    ],
    content: initialContent,
    editorProps: {
      attributes: {
        style: [
          'min-height: 200px',
          'padding: 12px 16px',
          'outline: none',
          'font-size: 14px',
          'line-height: 1.6',
          'color: var(--color-text-primary)',
          'font-family: inherit',
        ].join(';'),
        'aria-label': '메일 본문',
        role: 'textbox',
        'aria-multiline': 'true',
      },
    },
    onUpdate: ({ editor: e }) => {
      triggerAutoSave(toRef.current, ccRef.current, bccRef.current, subjectRef.current, e.getText());
    },
    immediatelyRender: false,
  });

  // Move cursor to start so user types above the quoted text
  useEffect(() => {
    if (editor && initialContent) {
      editor.commands.focus('start');
    }
  }, [editor, initialContent]);

  useEffect(() => {
    if (sendCountdown === null) return;
    if (sendCountdown === 0) {
      const msg = pendingMsgRef.current;
      if (!msg) return;
      setSending(true);
      sendMessage(msg)
        .then(() => {
          try {
            const newAddrs = [...(msg.to ?? []), ...(msg.cc ?? []), ...(msg.bcc ?? [])]
              .map((a) => a.name ? `${a.name} <${a.address}>` : a.address).filter(Boolean);
            const merged = [...new Set([...newAddrs, ...recentRecipients])].slice(0, 30);
            localStorage.setItem('webmail_recent_recipients', JSON.stringify(merged));
          } catch { /* */ }
          setSent(true);
          setTimeout(() => onClose(), 1500);
        })
        .catch((err: unknown) => {
          const message = err instanceof Error ? err.message : '전송에 실패했습니다.';
          setError(message);
          setSendCountdown(null);
        })
        .finally(() => setSending(false));
      return;
    }
    const t = setTimeout(() => setSendCountdown((n) => (n !== null ? n - 1 : null)), 1000);
    return () => clearTimeout(t);
  }, [sendCountdown, onClose, recentRecipients]);

  const handleManualSave = useCallback(async () => {
    const bodyText = editor?.getText() ?? '';
    if (!to.trim() && !subject.trim() && !bodyText.trim()) return;
    setSaveStatus('saving');
    try {
      const data = {
        intent,
        ...(intent !== 'new' && sourceMessage && { source_message_id: sourceMessage.id }),
        to: to.trim() ? [{ address: to.trim() }] : [],
        ...(cc.trim() && { cc: cc.split(',').map((a) => ({ address: a.trim() })).filter((a) => a.address) }),
        ...(bcc.trim() && { bcc: bcc.split(',').map((a) => ({ address: a.trim() })).filter((a) => a.address) }),
        subject,
        text_body: bodyText,
      };
      if (draftIdRef.current) await updateDraft(draftIdRef.current, data);
      else { const r = await saveDraft(data); draftIdRef.current = r.draft.id; }
      const now = new Date();
      setSavedAt(`${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`);
      setSaveStatus('saved');
    } catch { setSaveStatus('idle'); }
  }, [to, cc, bcc, subject, editor, intent, sourceMessage]);

  const saveTemplate = useCallback(() => {
    const bodyText = editor?.getText() ?? '';
    if (!subject.trim() && !bodyText.trim()) return;
    const name = window.prompt('템플릿 이름을 입력하세요:');
    if (!name?.trim()) return;
    const entry = { name: name.trim(), subject, body: editor?.getHTML() ?? bodyText };
    setTemplates((prev) => {
      const next = [...prev.filter((t) => t.name !== entry.name), entry];
      try { localStorage.setItem('webmail_templates', JSON.stringify(next)); } catch { /* */ }
      return next;
    });
  }, [subject, editor]);

  const loadTemplate = useCallback((t: { name: string; subject: string; body: string }) => {
    setSubject(t.subject);
    subjectRef.current = t.subject;
    editor?.commands.setContent(t.body);
    setShowTemplates(false);
  }, [editor]);

  const deleteTemplate = useCallback((name: string) => {
    setTemplates((prev) => {
      const next = prev.filter((t) => t.name !== name);
      try { localStorage.setItem('webmail_templates', JSON.stringify(next)); } catch { /* */ }
      return next;
    });
  }, []);

  function handleSend(e: { preventDefault(): void }) {
    e.preventDefault();
    if (!to.trim()) {
      setError('받는 사람 주소를 입력하세요.');
      return;
    }
    const bodyText = editor?.getText() ?? '';
    if (!bodyText.trim() && !subject.trim()) {
      setError('제목 또는 본문을 입력하세요.');
      return;
    }
    setError('');
    const readyAttachmentIds = uploadedAttachments.filter((a) => !a.uploading && !a.error).map((a) => a.id);
    const msg: SendMessageRequest = {
      to: parseAddrs(to),
      ...(cc.trim() && { cc: parseAddrs(cc) }),
      ...(bcc.trim() && { bcc: parseAddrs(bcc) }),
      subject: subject.trim(),
      text_body: bodyText,
      ...(editor && { html_body: editor.getHTML() }),
      ...(intent !== 'new' && sourceMessage && { intent, source_message_id: sourceMessage.id }),
      ...(readyAttachmentIds.length > 0 && { attachment_ids: readyAttachmentIds }),
      ...(scheduledAt && { scheduled_at: new Date(scheduledAt).toISOString() }),
    };
    pendingMsgRef.current = msg;
    if (scheduledAt) {
      // Scheduled sends bypass the undo countdown and go immediately
      setSending(true);
      sendMessage(msg)
        .then(() => { setSent(true); setTimeout(() => onClose(), 1500); })
        .catch((err: unknown) => {
          const message = err instanceof Error ? err.message : '전송에 실패했습니다.';
          setError(message);
        })
        .finally(() => setSending(false));
    } else {
      setSendCountdown(5);
    }
  }

  function handleLinkInsert() {
    const url = window.prompt('링크 URL을 입력하세요:');
    if (url && editor) {
      editor.chain().focus().setLink({ href: url }).run();
    }
  }

  return (
    <>
      <div aria-hidden="true" style={{ position: 'fixed', inset: 0, zIndex: 99, pointerEvents: 'none' }} />

      <div
        role="dialog"
        aria-label="새 메시지 작성"
        aria-modal="true"
        onDragEnter={(e) => { e.preventDefault(); dragCounterRef.current++; setDragOver(true); }}
        onDragLeave={() => { dragCounterRef.current--; if (dragCounterRef.current <= 0) { dragCounterRef.current = 0; setDragOver(false); } }}
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => { e.preventDefault(); dragCounterRef.current = 0; setDragOver(false); if (e.dataTransfer.files.length) handleFileSelect(e.dataTransfer.files); }}
        style={{
          position: 'fixed',
          ...(isMobile
            ? { inset: 0, borderRadius: 0, width: '100%', maxWidth: 'none', maxHeight: '100dvh', height: '100dvh' }
            : fullscreen
              ? { inset: '16px', width: 'auto', maxWidth: 'none', bottom: '16px' }
              : { bottom: '24px', insetInlineEnd: `${24 + windowOffset * 576}px`, width: '560px', maxWidth: 'calc(100vw - 48px)' }
          ),
          background: 'var(--color-bg-primary)',
          border: `1px solid ${dragOver ? 'var(--color-accent)' : isMobile ? 'transparent' : 'var(--color-border-default)'}`,
          borderRadius: isMobile ? 0 : '8px',
          boxShadow: isMobile ? 'none' : dragOver ? '0 0 0 2px var(--color-accent-subtle)' : '0 8px 32px rgba(0,0,0,0.16)',
          zIndex: 100,
          display: 'flex',
          flexDirection: 'column',
          animation: 'composeIn 120ms ease-out',
          maxHeight: isMobile ? '100dvh' : minimized ? '44px' : fullscreen ? 'none' : '80vh',
          height: isMobile || (fullscreen && !minimized) ? '100%' : undefined,
          overflow: 'hidden',
          transition: 'max-height 180ms ease, border-color 100ms ease, box-shadow 100ms ease',
        }}
      >
        {dragOver && !minimized && (
          <div style={{ position: 'absolute', inset: 0, zIndex: 200, background: 'var(--color-accent-subtle)', display: 'flex', alignItems: 'center', justifyContent: 'center', pointerEvents: 'none', borderRadius: '8px' }}>
            <div style={{ textAlign: 'center', color: 'var(--color-accent)', fontSize: '15px', fontWeight: 500 }}>
              <PaperClipIcon style={{ width: '40px', height: '40px', marginBottom: '8px' }} />
              파일을 여기에 놓으세요
            </div>
          </div>
        )}
        <style>{`
          @keyframes composeIn {
            from { opacity: 0; transform: scale(0.97) translateY(8px); }
            to   { opacity: 1; transform: scale(1) translateY(0); }
          }
          .tiptap p.is-editor-empty:first-child::before {
            content: attr(data-placeholder);
            float: left;
            color: var(--color-text-tertiary);
            pointer-events: none;
            height: 0;
          }
          .tiptap a { color: var(--color-accent); text-decoration: underline; }
          .tiptap p { margin: 0 0 4px; }
          .tiptap ul, .tiptap ol { padding-left: 20px; }
.tiptap blockquote { border-left: 3px solid var(--color-border-default); margin: 4px 0; padding: 4px 12px; color: var(--color-text-secondary); }
.tiptap code { background: var(--color-bg-secondary); border: 1px solid var(--color-border-subtle); border-radius: 3px; padding: 1px 4px; font-family: monospace; font-size: 12px; }
        `}</style>

        {/* Header */}
        <div
          onClick={minimized ? () => setMinimized(false) : undefined}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '10px 16px',
            borderBottom: minimized ? 'none' : '1px solid var(--color-border-subtle)',
            background: 'var(--color-bg-secondary)',
            borderRadius: minimized ? '8px' : '8px 8px 0 0',
            cursor: minimized ? 'pointer' : 'default',
            flexShrink: 0,
          }}
        >
          <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1, minWidth: 0 }}>
            {minimized && subject ? subject : (intent === 'reply' || intent === 'reply_all' ? '답장' : intent === 'forward' ? '전달' : '새 메시지')}
          </span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flexShrink: 0, marginLeft: '8px' }}>
            {!isMobile && <>
            <button
              onClick={(e) => { e.stopPropagation(); setFullscreen((v) => !v); if (minimized) setMinimized(false); }}
              aria-label={fullscreen ? '창 축소' : '전체화면'}
              title={fullscreen ? '창 축소' : '전체화면'}
              style={{
                width: '24px', height: '24px', borderRadius: '4px', border: 'none',
                background: 'transparent', color: 'var(--color-text-secondary)',
                cursor: 'pointer', fontSize: '12px', lineHeight: 1,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >{fullscreen ? '⊡' : '⊞'}</button>
            <button
              onClick={(e) => { e.stopPropagation(); setMinimized((v) => !v); }}
              aria-label={minimized ? '창 복원' : '창 최소화'}
              style={{
                width: '24px', height: '24px', borderRadius: '4px', border: 'none',
                background: 'transparent', color: 'var(--color-text-secondary)',
                cursor: 'pointer', fontSize: '14px', lineHeight: 1,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >{minimized ? '□' : '─'}</button>
            </>}
            <button
              onClick={() => {
                const hasContent = !sent && (to.trim() || subject.trim() || (editor && editor.getText().trim()));
                if (hasContent) setConfirmClose(true); else onClose();
              }}
              aria-label="창 닫기"
              style={{
                width: '24px', height: '24px', borderRadius: '4px', border: 'none',
                background: 'transparent', color: 'var(--color-text-secondary)',
                cursor: 'pointer', fontSize: isMobile ? '20px' : '16px', lineHeight: 1,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >{isMobile ? '←' : '×'}</button>
          </div>
        </div>

        {/* Close confirmation panel */}
        {confirmClose && (
          <div style={{ padding: '10px 16px', borderBottom: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)', display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
            <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', flex: 1 }}>임시저장 후 닫으시겠습니까?</span>
            <button
              type="button"
              onClick={async () => {
                const bodyText = editor?.getText() ?? '';
                if (to.trim() || subject.trim() || bodyText.trim()) {
                  const data = {
                    intent,
                    to: to.trim() ? [{ address: to.trim() }] : [],
                    subject,
                    text_body: bodyText,
                    ...(editor && { html_body: editor.getHTML() }),
                  };
                  try {
                    if (draftIdRef.current) await updateDraft(draftIdRef.current, data);
                    else { const r = await saveDraft(data); draftIdRef.current = r.draft.id; }
                  } catch { /* ignore */ }
                }
                onClose();
              }}
              style={{ fontSize: '12px', padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', cursor: 'pointer' }}
            >임시저장</button>
            <button
              type="button"
              onClick={onClose}
              style={{ fontSize: '12px', padding: '4px 10px', borderRadius: '5px', border: '1px solid rgba(217,79,61,0.4)', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer' }}
            >버리기</button>
            <button
              type="button"
              onClick={() => setConfirmClose(false)}
              style={{ fontSize: '12px', padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}
            >취소</button>
          </div>
        )}

        {/* Form */}
        <form
          onSubmit={handleSend}
          onKeyDown={(e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') { e.preventDefault(); handleSend(e); }
            if ((e.ctrlKey || e.metaKey) && e.key === 's') { e.preventDefault(); void handleManualSave(); }
          }}
          style={{ display: 'flex', flexDirection: 'column', flex: 1 }}
        >

          {/* From (display only) */}
          {userEmail && (
            <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '6px 16px', gap: '8px', flexShrink: 0, background: 'var(--color-bg-secondary)' }}>
              <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>보내는 사람</span>
              <span style={{ fontSize: '13px', color: 'var(--color-text-secondary)' }}>{userEmail}</span>
            </div>
          )}

          {/* To */}
          <div style={{ display: 'flex', alignItems: 'center', borderBottom: `1px solid ${error.includes('받는 사람') ? 'var(--color-destructive)' : 'var(--color-border-subtle)'}`, padding: '0 16px', flexShrink: 0 }}>
            <label htmlFor="compose-to" style={{ fontSize: '13px', color: error.includes('받는 사람') ? 'var(--color-destructive)' : 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>받는 사람</label>
            <RecipientChips
              id="compose-to"
              value={to}
              onChange={(v) => { setTo(v); toRef.current = v; if (error) setError(''); triggerAutoSave(v, ccRef.current, bccRef.current, subjectRef.current, editor?.getText() ?? ''); }}
              placeholder="example@domain.com"
              autoFocus
              hasError={error.includes('받는 사람')}
              suggestions={recentRecipients}
            />
            <div style={{ display: 'flex', gap: '4px', flexShrink: 0, marginLeft: '4px' }}>
              {!showCc && (
                <button type="button" onClick={() => setShowCc(true)}
                  style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', background: 'none', border: 'none', cursor: 'pointer', padding: '2px 6px', borderRadius: '4px', fontWeight: 500 }}
                  onMouseEnter={(e) => { (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget).style.color = 'var(--color-text-tertiary)'; }}
                >Cc</button>
              )}
              {!showBcc && (
                <button type="button" onClick={() => setShowBcc(true)}
                  style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', background: 'none', border: 'none', cursor: 'pointer', padding: '2px 6px', borderRadius: '4px', fontWeight: 500 }}
                  onMouseEnter={(e) => { (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget).style.color = 'var(--color-text-tertiary)'; }}
                >Bcc</button>
              )}
            </div>
          </div>

          {/* CC — only when toggled */}
          {showCc && (
            <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px', flexShrink: 0 }}>
              <label htmlFor="compose-cc" style={{ fontSize: '13px', color: 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>Cc</label>
              <RecipientChips
                id="compose-cc"
                value={cc}
                onChange={(v) => { setCc(v); ccRef.current = v; triggerAutoSave(toRef.current, v, bccRef.current, subjectRef.current, editor?.getText() ?? ''); }}
                placeholder="example@domain.com, ..."
                suggestions={recentRecipients}
              />
            </div>
          )}

          {/* BCC — only when toggled */}
          {showBcc && (
            <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px', flexShrink: 0 }}>
              <label htmlFor="compose-bcc" style={{ fontSize: '13px', color: 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>Bcc</label>
              <RecipientChips
                id="compose-bcc"
                value={bcc}
                onChange={(v) => { setBcc(v); bccRef.current = v; triggerAutoSave(toRef.current, ccRef.current, v, subjectRef.current, editor?.getText() ?? ''); }}
                placeholder="example@domain.com, ..."
                suggestions={recentRecipients}
              />
            </div>
          )}

          {/* Subject */}
          <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px', flexShrink: 0 }}>
            <input
              id="compose-subject"
              type="text"
              value={subject}
              onChange={(e) => { setSubject(e.target.value); subjectRef.current = e.target.value; triggerAutoSave(toRef.current, ccRef.current, bccRef.current, e.target.value, editor?.getText() ?? ''); }}
              placeholder="제목"
              style={{ flex: 1, padding: '10px 0', border: 'none', outline: 'none', fontSize: '14px', background: 'transparent', color: 'var(--color-text-primary)', fontWeight: 500 }}
            />
          </div>

          {/* TipTap editor body */}
          <div style={{ flex: 1, overflowY: 'auto', minHeight: '160px' }}>
            <EditorContent editor={editor} />
          </div>

          {/* Signature editor */}
          {showSigEditor && (
            <div style={{ padding: '8px 16px', borderTop: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)' }}>
              <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginBottom: '4px', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em' }}>서명</div>
              <textarea
                value={signature}
                onChange={(e) => setSignature(e.target.value)}
                onBlur={() => { try { localStorage.setItem('webmail_signature', signature); } catch { /* ignore */ } }}
                placeholder="서명을 입력하세요..."
                rows={3}
                style={{ width: '100%', padding: '6px 8px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', resize: 'vertical', outline: 'none', boxSizing: 'border-box', fontFamily: 'inherit' }}
              />
              <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>변경 사항은 다음 메시지 작성 시 적용됩니다</div>
            </div>
          )}

          {/* Attachment chips */}
          {uploadedAttachments.length > 0 && (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', padding: '6px 16px', borderTop: '1px solid var(--color-border-subtle)' }}>
              {uploadedAttachments.map((att) => {
                const kb = att.size < 1024 * 1024 ? `${Math.round(att.size / 1024)} KB` : `${(att.size / 1024 / 1024).toFixed(1)} MB`;
                return (
                  <div key={att.id} style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', padding: '3px 8px', borderRadius: '12px', border: `1px solid ${att.error ? 'rgba(217,79,61,0.4)' : 'var(--color-border-default)'}`, background: 'var(--color-bg-secondary)', fontSize: '12px', color: att.error ? 'var(--color-destructive)' : 'var(--color-text-primary)' }}>
                    <span style={{ display: 'inline-flex', alignItems: 'center' }}>{att.uploading ? <ArrowPathIcon style={{ width: '12px', height: '12px', animation: 'spin 1s linear infinite' }} /> : att.error ? <ExclamationTriangleIcon style={{ width: '12px', height: '12px' }} /> : <PaperClipIcon style={{ width: '12px', height: '12px' }} />}</span>
                    <span style={{ maxWidth: '160px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{att.filename}</span>
                    {!att.uploading && <span style={{ color: 'var(--color-text-tertiary)' }}>{kb}</span>}
                    <button
                      type="button"
                      onClick={() => setUploadedAttachments((prev) => prev.filter((a) => a.id !== att.id))}
                      style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', fontSize: '14px', lineHeight: 1, padding: '0 2px' }}
                    >×</button>
                  </div>
                );
              })}
            </div>
          )}

          {/* Footer — send left, icons right */}
          <div style={{
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            padding: '8px 12px',
            borderTop: '1px solid var(--color-border-subtle)',
            flexShrink: 0,
          }}>
            {/* Send button — left */}
            <button
              type="submit"
              disabled={sending || sent || uploadedAttachments.some((a) => a.uploading)}
              style={{
                padding: '7px 18px',
                borderRadius: '20px',
                border: 'none',
                background: sending || sent || uploadedAttachments.some((a) => a.uploading) ? 'var(--color-border-default)' : 'var(--color-accent)',
                color: '#fff',
                fontSize: '13px',
                fontWeight: 500,
                cursor: sending || sent || uploadedAttachments.some((a) => a.uploading) ? 'not-allowed' : 'pointer',
                transition: 'background 100ms ease',
                flexShrink: 0,
              }}
              onMouseEnter={(e) => { if (!sending && !sent) (e.currentTarget).style.background = 'var(--color-accent-hover)'; }}
              onMouseLeave={(e) => { if (!sending && !sent) (e.currentTarget).style.background = 'var(--color-accent)'; }}
            >
              {sending ? '전송 중...' : sent ? '전송됨 ✓' : uploadedAttachments.some((a) => a.uploading) ? '업로드 중...' : scheduledAt ? '예약 전송' : '전송'}
            </button>

            {/* Status messages */}
            {error && <span role="alert" style={{ fontSize: '12px', color: 'var(--color-destructive)', flex: 1 }}>{error}</span>}
            {!error && !sent && saveStatus === 'saving' && <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>저장 중...</span>}
            {!error && !sent && saveStatus === 'saved' && <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>임시저장 {savedAt}</span>}
            <div style={{ flex: 1 }} />

            {/* Right-side icon actions */}
            <input
              ref={fileInputRef}
              type="file"
              multiple
              style={{ display: 'none' }}
              onChange={(e) => { if (e.target.files?.length) { handleFileSelect(e.target.files); e.target.value = ''; } }}
            />
            {/* Formatting icons */}
            <button type="button" aria-label="굵게" title="굵게" style={toolbarBtnStyle(editor?.isActive('bold'))} onClick={() => editor?.chain().focus().toggleBold().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bold') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><b>B</b></button>
            <button type="button" aria-label="기울임" title="기울임" style={toolbarBtnStyle(editor?.isActive('italic'))} onClick={() => editor?.chain().focus().toggleItalic().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('italic') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><i>I</i></button>
            <button type="button" aria-label="밑줄" title="밑줄" style={toolbarBtnStyle(editor?.isActive('underline'))} onClick={() => editor?.chain().focus().toggleUnderline().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('underline') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><u>U</u></button>
            <button type="button" aria-label="목록" title="목록" style={toolbarBtnStyle(editor?.isActive('bulletList'))} onClick={() => editor?.chain().focus().toggleBulletList().run()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bulletList') ? 'var(--color-bg-tertiary)' : 'transparent'; }}>≡</button>
            <button type="button" aria-label="링크" title="링크" style={toolbarBtnStyle(editor?.isActive('link'))} onClick={handleLinkInsert} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('link') ? 'var(--color-bg-tertiary)' : 'transparent'; }}><LinkIcon style={{ width: '14px', height: '14px' }} /></button>

            <div style={{ width: '1px', height: '16px', background: 'var(--color-border-subtle)' }} />

            {/* Utility icons */}
            <button type="button" onClick={() => fileInputRef.current?.click()} title="파일 첨부" style={toolbarBtnStyle()} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}><PaperClipIcon style={{ width: '14px', height: '14px' }} /></button>
            <button type="button" onClick={() => setShowSigEditor((v) => !v)} title="서명" style={toolbarBtnStyle(showSigEditor)} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = showSigEditor ? 'var(--color-bg-tertiary)' : 'transparent'; }}><PencilSquareIconHero style={{ width: '14px', height: '14px' }} /></button>
            <div style={{ position: 'relative' }}>
              <button type="button" onClick={() => setShowTemplates((v) => !v)} title="템플릿" style={toolbarBtnStyle(showTemplates)} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = showTemplates ? 'var(--color-bg-tertiary)' : 'transparent'; }}><ClipboardDocumentIcon style={{ width: '14px', height: '14px' }} /></button>
              {showTemplates && (
                <div style={{ position: 'absolute', bottom: '100%', right: 0, marginBottom: '4px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '6px', boxShadow: '0 4px 16px rgba(0,0,0,0.12)', zIndex: 300, minWidth: '200px', overflow: 'hidden' }}>
                  {templates.length === 0 && <div style={{ padding: '10px 14px', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>저장된 템플릿 없음</div>}
                  {templates.map((t) => (
                    <div key={t.name} style={{ display: 'flex', alignItems: 'center', gap: '4px', padding: '0 4px' }}>
                      <button type="button" onClick={() => loadTemplate(t)} style={{ flex: 1, textAlign: 'left', padding: '8px 10px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}>{t.name}</button>
                      <button type="button" onClick={() => deleteTemplate(t.name)} title="삭제" style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-destructive)', fontSize: '14px', padding: '4px 6px', lineHeight: 1, flexShrink: 0 }}>×</button>
                    </div>
                  ))}
                  <div style={{ borderTop: '1px solid var(--color-border-subtle)', padding: '4px' }}>
                    <button type="button" onClick={saveTemplate} style={{ width: '100%', textAlign: 'left', padding: '7px 10px', border: 'none', background: 'transparent', color: 'var(--color-accent)', fontSize: '13px', cursor: 'pointer', fontWeight: 500 }} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}>+ 현재 내용 저장</button>
                  </div>
                </div>
              )}
            </div>
            <button type="button" onClick={() => { setShowSchedule((v) => !v); if (showSchedule) setScheduledAt(''); }} title="나중에 보내기" style={toolbarBtnStyle(showSchedule)} onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }} onMouseLeave={(e) => { (e.currentTarget).style.background = showSchedule ? 'var(--color-bg-tertiary)' : 'transparent'; }}><ClockIcon style={{ width: '14px', height: '14px' }} /></button>
            {showSchedule && (
              <input type="datetime-local" value={scheduledAt} onChange={(e) => setScheduledAt(e.target.value)} min={new Date(Date.now() + 60000).toISOString().slice(0, 16)} style={{ fontSize: '12px', padding: '3px 6px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', outline: 'none' }} />
            )}
          </div>
        </form>
        {sendCountdown !== null && sendCountdown > 0 && (
          <div style={{
            display: 'flex', alignItems: 'center', justifyContent: 'space-between',
            padding: '10px 16px', background: 'var(--color-accent-subtle)',
            borderTop: '1px solid var(--color-border-default)',
            fontSize: '13px', color: 'var(--color-text-primary)',
          }}>
            <span>{sendCountdown}초 후 전송됩니다...</span>
            <button
              onClick={() => { setSendCountdown(null); pendingMsgRef.current = null; }}
              style={{ padding: '4px 12px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', fontSize: '13px', color: 'var(--color-text-primary)' }}
            >취소</button>
          </div>
        )}
      </div>
    </>
  );
}
