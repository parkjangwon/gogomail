'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Underline from '@tiptap/extension-underline';
import TextAlign from '@tiptap/extension-text-align';
import Placeholder from '@tiptap/extension-placeholder';
import { sendMessage, saveDraft, updateDraft, ComposeIntent, MessageDetail } from '@/lib/api';
import { RecipientChips } from './RecipientChips';

interface ComposeModalProps {
  onClose: () => void;
  intent?: ComposeIntent;
  sourceMessage?: MessageDetail;
  draftMessage?: MessageDetail;
  userEmail?: string;
}

function escapeHtml(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;');
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

export function ComposeModal({ onClose, intent = 'new', sourceMessage, draftMessage, userEmail }: ComposeModalProps) {
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

  const [to, setTo] = useState(draftMessage ? draftTo : replyTo);
  const [cc, setCc] = useState(draftMessage ? draftCc : replyCc);
  const [bcc, setBcc] = useState('');
  const [subject, setSubject] = useState(draftMessage ? (draftMessage.subject ?? '') : replySubject);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState('');
  const [sent, setSent] = useState(false);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [savedAt, setSavedAt] = useState('');
  const [minimized, setMinimized] = useState(false);
  const [showSigEditor, setShowSigEditor] = useState(false);
  const [signature, setSignature] = useState(() => {
    try { return localStorage.getItem('webmail_signature') ?? ''; } catch { return ''; }
  });
  const [recentRecipients] = useState<string[]>(() => {
    try { return JSON.parse(localStorage.getItem('webmail_recent_recipients') ?? '[]'); } catch { return []; }
  });
  const draftIdRef = useRef<string>(draftMessage?.id ?? '');
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

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

  async function handleSend(e: { preventDefault(): void }) {
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
    setSending(true);
    try {
      await sendMessage({
        to: to.split(',').map((a) => ({ address: a.trim() })).filter((a) => a.address),
        ...(cc.trim() && { cc: cc.split(',').map((a) => ({ address: a.trim() })).filter((a) => a.address) }),
        ...(bcc.trim() && { bcc: bcc.split(',').map((a) => ({ address: a.trim() })).filter((a) => a.address) }),
        subject: subject.trim(),
        text_body: bodyText,
        ...(editor && { html_body: editor.getHTML() }),
        ...(intent !== 'new' && sourceMessage && {
          intent,
          source_message_id: sourceMessage.id,
        }),
      });
      // Persist recently used recipients
      try {
        const newAddrs = [...to.split(','), ...cc.split(','), ...bcc.split(',')]
          .map((a) => a.trim()).filter(Boolean);
        const merged = [...new Set([...newAddrs, ...recentRecipients])].slice(0, 30);
        localStorage.setItem('webmail_recent_recipients', JSON.stringify(merged));
      } catch { /* ignore */ }
      setSent(true);
      setTimeout(() => onClose(), 1500);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : '전송에 실패했습니다.';
      setError(message);
    } finally {
      setSending(false);
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
        style={{
          position: 'fixed',
          bottom: '24px',
          insetInlineEnd: '24px',
          width: '560px',
          maxWidth: 'calc(100vw - 48px)',
          background: 'var(--color-bg-primary)',
          border: '1px solid var(--color-border-default)',
          borderRadius: '8px',
          boxShadow: '0 8px 32px rgba(0,0,0,0.16)',
          zIndex: 100,
          display: 'flex',
          flexDirection: 'column',
          animation: 'composeIn 120ms ease-out',
          maxHeight: minimized ? '44px' : '80vh',
          overflow: 'hidden',
          transition: 'max-height 180ms ease',
        }}
      >
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
            <button
              onClick={onClose}
              aria-label="창 닫기"
              style={{
                width: '24px', height: '24px', borderRadius: '4px', border: 'none',
                background: 'transparent', color: 'var(--color-text-secondary)',
                cursor: 'pointer', fontSize: '16px', lineHeight: 1,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >×</button>
          </div>
        </div>

        {/* Form */}
        <form
          onSubmit={handleSend}
          onKeyDown={(e) => { if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') { e.preventDefault(); handleSend(e); } }}
          style={{ display: 'flex', flexDirection: 'column', flex: 1 }}
        >

          {/* To */}
          <div style={{ display: 'flex', alignItems: 'center', borderBottom: `1px solid ${error.includes('받는 사람') ? 'var(--color-destructive)' : 'var(--color-border-subtle)'}`, padding: '0 16px' }}>
            <label htmlFor="compose-to" style={{ fontSize: '13px', color: error.includes('받는 사람') ? 'var(--color-destructive)' : 'var(--color-text-secondary)', width: '68px', flexShrink: 0 }}>받는 사람</label>
            <RecipientChips
              id="compose-to"
              value={to}
              onChange={(v) => { setTo(v); toRef.current = v; if (error) setError(''); triggerAutoSave(v, ccRef.current, bccRef.current, subjectRef.current, editor?.getText() ?? ''); }}
              placeholder="example@domain.com"
              autoFocus
              hasError={error.includes('받는 사람')}
              suggestions={recentRecipients}
            />
          </div>

          {/* CC */}
          <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px' }}>
            <label htmlFor="compose-cc" style={{ fontSize: '13px', color: 'var(--color-text-secondary)', width: '68px', flexShrink: 0 }}>참조</label>
            <RecipientChips
              id="compose-cc"
              value={cc}
              onChange={(v) => { setCc(v); ccRef.current = v; triggerAutoSave(toRef.current, v, bccRef.current, subjectRef.current, editor?.getText() ?? ''); }}
              placeholder="example@domain.com, ..."
              suggestions={recentRecipients}
            />
          </div>

          {/* BCC */}
          <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px' }}>
            <label htmlFor="compose-bcc" style={{ fontSize: '13px', color: 'var(--color-text-secondary)', width: '68px', flexShrink: 0 }}>숨은 참조</label>
            <RecipientChips
              id="compose-bcc"
              value={bcc}
              onChange={(v) => { setBcc(v); bccRef.current = v; triggerAutoSave(toRef.current, ccRef.current, v, subjectRef.current, editor?.getText() ?? ''); }}
              placeholder="example@domain.com, ..."
              suggestions={recentRecipients}
            />
          </div>

          {/* Subject */}
          <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px' }}>
            <label htmlFor="compose-subject" style={{ fontSize: '13px', color: 'var(--color-text-secondary)', width: '68px', flexShrink: 0 }}>제목</label>
            <input
              id="compose-subject"
              type="text"
              value={subject}
              onChange={(e) => { setSubject(e.target.value); subjectRef.current = e.target.value; triggerAutoSave(toRef.current, ccRef.current, bccRef.current, e.target.value, editor?.getText() ?? ''); }}
              placeholder="메일 제목"
              style={{ flex: 1, padding: '10px 0', border: 'none', outline: 'none', fontSize: '14px', background: 'transparent', color: 'var(--color-text-primary)' }}
            />
          </div>

          {/* Toolbar */}
          <div style={{
            display: 'flex',
            gap: '2px',
            padding: '6px 12px',
            borderBottom: '1px solid var(--color-border-subtle)',
            background: 'var(--color-bg-secondary)',
          }}>
            <button type="button" aria-label="굵게" title="굵게 (Ctrl+B)"
              style={toolbarBtnStyle(editor?.isActive('bold'))}
              onClick={() => editor?.chain().focus().toggleBold().run()}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bold') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
            ><b>B</b></button>

            <button type="button" aria-label="기울임" title="기울임 (Ctrl+I)"
              style={toolbarBtnStyle(editor?.isActive('italic'))}
              onClick={() => editor?.chain().focus().toggleItalic().run()}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('italic') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
            ><i>I</i></button>

            <button type="button" aria-label="밑줄" title="밑줄 (Ctrl+U)"
              style={toolbarBtnStyle(editor?.isActive('underline'))}
              onClick={() => editor?.chain().focus().toggleUnderline().run()}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('underline') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
            ><u>U</u></button>

            <div style={{ width: '1px', background: 'var(--color-border-subtle)', margin: '4px 4px' }} />

            <button type="button" aria-label="목록" title="글머리 기호"
              style={toolbarBtnStyle(editor?.isActive('bulletList'))}
              onClick={() => editor?.chain().focus().toggleBulletList().run()}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bulletList') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
            >≡</button>

            <button type="button" aria-label="링크 삽입" title="링크 삽입"
              style={toolbarBtnStyle(editor?.isActive('link'))}
              onClick={handleLinkInsert}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('link') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
            >🔗</button>

            <div style={{ width: '1px', background: 'var(--color-border-subtle)', margin: '4px 4px' }} />

            <button type="button" aria-label="실행 취소" title="실행 취소 (Ctrl+Z)"
              style={toolbarBtnStyle()}
              onClick={() => editor?.chain().focus().undo().run()}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >↩</button>
          </div>

          {/* TipTap editor body */}
          <div style={{ flex: 1, overflowY: 'auto', minHeight: '200px', maxHeight: '320px' }}>
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

          {/* Footer */}
          <div style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '10px 16px',
            borderTop: '1px solid var(--color-border-subtle)',
          }}>
            <div>
              {error && <span role="alert" style={{ fontSize: '13px', color: 'var(--color-destructive)' }}>{error}</span>}
              {sent && <span style={{ fontSize: '13px', color: 'var(--color-success)' }}>전송 완료 ✓</span>}
              {!error && !sent && saveStatus === 'saving' && <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>저장 중...</span>}
              {!error && !sent && saveStatus === 'saved' && <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>임시저장됨 {savedAt}</span>}
            </div>
            <button
              type="button"
              onClick={() => setShowSigEditor((v) => !v)}
              title="서명 관리"
              style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '12px', color: 'var(--color-text-tertiary)', padding: '2px 6px', borderRadius: '4px' }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >서명</button>
            <button
              type="submit"
              disabled={sending || sent}
              style={{
                padding: '8px 20px',
                borderRadius: '6px',
                border: 'none',
                background: sending || sent ? 'var(--color-border-default)' : 'var(--color-accent)',
                color: '#fff',
                fontSize: '14px',
                fontWeight: 500,
                cursor: sending || sent ? 'not-allowed' : 'pointer',
                transition: 'background 100ms ease',
              }}
              onMouseEnter={(e) => { if (!sending && !sent) (e.currentTarget).style.background = 'var(--color-accent-hover)'; }}
              onMouseLeave={(e) => { if (!sending && !sent) (e.currentTarget).style.background = 'var(--color-accent)'; }}
            >
              {sending ? '전송 중...' : sent ? '전송됨' : '보내기'}
            </button>
          </div>
        </form>
      </div>
    </>
  );
}
