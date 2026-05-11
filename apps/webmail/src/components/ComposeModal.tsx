'use client';

import { useState } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Underline from '@tiptap/extension-underline';
import TextAlign from '@tiptap/extension-text-align';
import Placeholder from '@tiptap/extension-placeholder';
import { sendMessage, ComposeIntent, MessageDetail } from '@/lib/api';

interface ComposeModalProps {
  onClose: () => void;
  intent?: ComposeIntent;
  sourceMessage?: MessageDetail;
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

export function ComposeModal({ onClose, intent = 'new', sourceMessage }: ComposeModalProps) {
  const replyTo = intent === 'reply' || intent === 'reply_all'
    ? sourceMessage?.from_addr ?? ''
    : '';
  const replySubject = sourceMessage
    ? intent === 'forward'
      ? `Fwd: ${sourceMessage.subject}`
      : `Re: ${sourceMessage.subject}`
    : '';

  const [to, setTo] = useState(replyTo);
  const [subject, setSubject] = useState(replySubject);
  const [sending, setSending] = useState(false);
  const [error, setError] = useState('');
  const [sent, setSent] = useState(false);

  const editor = useEditor({
    extensions: [
      StarterKit,
      Underline,
      Link.configure({ openOnClick: false }),
      TextAlign.configure({ types: ['heading', 'paragraph'] }),
      Placeholder.configure({ placeholder: '메시지를 입력하세요...' }),
    ],
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
    immediatelyRender: false,
  });

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
        to: [{ address: to.trim() }],
        subject: subject.trim(),
        text_body: bodyText,
        ...(intent !== 'new' && sourceMessage && {
          intent,
          source_message_id: sourceMessage.id,
        }),
      });
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
        <div style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: '10px 16px',
          borderBottom: '1px solid var(--color-border-subtle)',
          background: 'var(--color-bg-secondary)',
          borderRadius: '8px 8px 0 0',
        }}>
          <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>
            {intent === 'reply' || intent === 'reply_all' ? '답장' : intent === 'forward' ? '전달' : '새 메시지'}
          </span>
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

        {/* Form */}
        <form onSubmit={handleSend} style={{ display: 'flex', flexDirection: 'column', flex: 1 }}>

          {/* To */}
          <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px' }}>
            <label htmlFor="compose-to" style={{ fontSize: '13px', color: 'var(--color-text-secondary)', width: '68px', flexShrink: 0 }}>받는 사람</label>
            <input
              id="compose-to"
              type="email"
              value={to}
              onChange={(e) => setTo(e.target.value)}
              placeholder="example@domain.com"
              autoFocus
              style={{ flex: 1, padding: '10px 0', border: 'none', outline: 'none', fontSize: '14px', background: 'transparent', color: 'var(--color-text-primary)' }}
            />
          </div>

          {/* Subject */}
          <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px' }}>
            <label htmlFor="compose-subject" style={{ fontSize: '13px', color: 'var(--color-text-secondary)', width: '68px', flexShrink: 0 }}>제목</label>
            <input
              id="compose-subject"
              type="text"
              value={subject}
              onChange={(e) => setSubject(e.target.value)}
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
            </div>
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
