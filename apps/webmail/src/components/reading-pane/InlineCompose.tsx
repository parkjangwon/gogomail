'use client';

import { useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { parseAddrs } from '@/lib/message/messageUtils';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Underline from '@tiptap/extension-underline';
import TextAlign from '@tiptap/extension-text-align';
import Placeholder from '@tiptap/extension-placeholder';
import Image from '@tiptap/extension-image';
import {
  ArrowTopRightOnSquareIcon,
  PaperClipIcon,
  PhotoIcon,
  LinkIcon,
  ListBulletIcon,
  NumberedListIcon,
  XMarkIcon,
  UsersIcon,
} from '@heroicons/react/24/outline';
import { parseToPickerItems, pickerItemsToString } from '@/lib/mail-address';
import { RecipientChips } from '@/components/RecipientChips';
import { OrgPickerModal } from '@/components/OrgPickerModal';
import { buildInlineQuoteHTML, toolbarBtnStyleInline } from './readingPaneHelpers';
import { useInlineComposeSend } from './useInlineComposeSend';
import { useInlineComposeAttachments } from './useInlineComposeAttachments';

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

export function InlineCompose({
  intent,
  to: initTo,
  subject: initSubject,
  messageId,
  sourceText,
  onClose,
  onOpenFullModal,
  userEmail,
}: InlineComposeProps) {
  const t = useTranslations('readingFull');
  const [to, setTo] = useState(initTo);
  const [subject, setSubject] = useState(initSubject);
  const [cc, setCc] = useState('');
  const [bcc, setBcc] = useState('');
  const [showCc, setShowCc] = useState(false);
  const [showBcc, setShowBcc] = useState(false);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const imageInputRef = useRef<HTMLInputElement>(null);
  const [orgPickerOpen, setOrgPickerOpen] = useState(false);

  const editor = useEditor({
    extensions: [
      StarterKit,
      Link.configure({ openOnClick: false }),
      Underline,
      TextAlign.configure({ types: ['heading', 'paragraph'] }),
      Placeholder.configure({ placeholder: t('compose.replyPlaceholder') }),
      Image,
    ],
    content: sourceText ? buildInlineQuoteHTML(intent, sourceText) : '<p></p>',
    immediatelyRender: false,
    autofocus: 'start',
  });

  const { attachments, setAttachments, handleFileAttach, handleImageFile } =
    useInlineComposeAttachments(editor);

  const { sending, sent, doSend } = useInlineComposeSend({
    editor,
    to,
    cc,
    bcc,
    subject,
    messageId,
    intent,
    attachments,
    onClose,
  });

  function handleLinkInsert() {
    const url = window.prompt(t('compose.linkPrompt'));
    if (url && editor) editor.chain().focus().setLink({ href: url }).run();
  }

  const intentLabel = intent === 'reply' ? t('compose.replyLabel') : intent === 'reply_all' ? t('compose.replyAllLabel') : t('compose.forwardLabel');
  const T = toolbarBtnStyleInline;

  function fmtSize(bytes: number): string {
    if (bytes < 1024) return `${bytes}B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(0)}KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
  }

  return (
    <div style={{ marginTop: '24px', borderRadius: '8px 8px 0 0', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', overflow: 'hidden' }}>
      <div style={{ display: 'flex', alignItems: 'center', padding: '10px 16px', borderBottom: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)' }}>
        <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', flex: 1 }}>{intentLabel}</span>
        <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
          <button
            type="button"
            aria-label={t('compose.openNewWindow')}
            title={t('compose.openNewWindow')}
            onClick={onOpenFullModal}
            style={{
              width: '24px',
              height: '24px',
              borderRadius: '4px',
              border: 'none',
              background: 'transparent',
              color: 'var(--color-text-secondary)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
            onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
          >
            <ArrowTopRightOnSquareIcon style={{ width: '14px', height: '14px' }} />
          </button>
          <button
            type="button"
            aria-label={t('compose.close')}
            onClick={onClose}
            style={{
              width: '24px',
              height: '24px',
              borderRadius: '4px',
              border: 'none',
              background: 'transparent',
              color: 'var(--color-text-secondary)',
              cursor: 'pointer',
              fontSize: '16px',
              lineHeight: 1,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
            onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
          >
            ×
          </button>
        </div>
      </div>

      {userEmail && (
        <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '6px 16px', gap: '8px', background: 'var(--color-bg-secondary)' }}>
          <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>{t('compose.fromLabel')}</span>
          <span style={{ fontSize: '13px', color: 'var(--color-text-secondary)' }}>{userEmail}</span>
        </div>
      )}

      <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px' }}>
        <label style={{ fontSize: '13px', color: 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>{t('compose.toLabel')}</label>
        <RecipientChips value={to} onChange={setTo} placeholder="example@domain.com" autoFocus />
        <div style={{ display: 'flex', gap: '2px', flexShrink: 0, alignItems: 'center' }}>
          <button
            type="button"
            onClick={() => setOrgPickerOpen(true)}
            title={t('compose.orgPicker')}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}
          >
            <UsersIcon style={{ width: '15px', height: '15px' }} />
          </button>
          <button
            type="button"
            onClick={() => { setShowCc((v) => !v); if (showCc) setCc(''); }}
            style={{ fontSize: '12px', color: showCc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)', background: 'none', border: 'none', cursor: 'pointer', padding: '2px 6px', borderRadius: '4px', fontWeight: 500, flexShrink: 0 }}
            onMouseEnter={(e) => { (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.color = showCc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)'; }}
          >
            Cc
          </button>
          <button
            type="button"
            onClick={() => { setShowBcc((v) => !v); if (showBcc) setBcc(''); }}
            style={{ fontSize: '12px', color: showBcc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)', background: 'none', border: 'none', cursor: 'pointer', padding: '2px 6px', borderRadius: '4px', fontWeight: 500, flexShrink: 0 }}
            onMouseEnter={(e) => { (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.color = showBcc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)'; }}
          >
            Bcc
          </button>
        </div>
      </div>

      {showCc && (
        <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px' }}>
          <label style={{ fontSize: '13px', color: 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>Cc</label>
          <RecipientChips value={cc} onChange={setCc} placeholder="example@domain.com, ..." />
          <button
            type="button"
            onClick={() => setOrgPickerOpen(true)}
            title={t('compose.orgPicker')}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}
          >
            <UsersIcon style={{ width: '15px', height: '15px' }} />
          </button>
          <button
            type="button"
            onClick={() => { setShowCc(false); setCc(''); }}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}
          >
            <XMarkIcon style={{ width: '13px', height: '13px' }} />
          </button>
        </div>
      )}

      {showBcc && (
        <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px' }}>
          <label style={{ fontSize: '13px', color: 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>Bcc</label>
          <RecipientChips value={bcc} onChange={setBcc} placeholder="example@domain.com, ..." />
          <button
            type="button"
            onClick={() => setOrgPickerOpen(true)}
            title={t('compose.orgPicker')}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}
          >
            <UsersIcon style={{ width: '15px', height: '15px' }} />
          </button>
          <button
            type="button"
            onClick={() => { setShowBcc(false); setBcc(''); }}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}
          >
            <XMarkIcon style={{ width: '13px', height: '13px' }} />
          </button>
        </div>
      )}

      {orgPickerOpen && (
        <OrgPickerModal
          initialTo={parseToPickerItems(to)}
          initialCc={parseToPickerItems(cc)}
          initialBcc={parseToPickerItems(bcc)}
          onClose={() => setOrgPickerOpen(false)}
          onConfirm={({ to: t, cc: c, bcc: b }) => {
            const newTo = pickerItemsToString(t);
            const newCc = pickerItemsToString(c);
            const newBcc = pickerItemsToString(b);
            setTo(newTo);
            if (newCc) { setShowCc(true); setCc(newCc); }
            if (newBcc) { setShowBcc(true); setBcc(newBcc); }
            setOrgPickerOpen(false);
          }}
        />
      )}

      <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px' }}>
        <input
          type="text"
          value={subject}
          onChange={(e) => setSubject(e.target.value)}
          placeholder={t('compose.subjectPlaceholder')}
          style={{ flex: 1, padding: '10px 0', border: 'none', outline: 'none', fontSize: '14px', background: 'transparent', color: 'var(--color-text-primary)', fontWeight: 500 }}
        />
      </div>

      <input
        ref={fileInputRef}
        type="file"
        multiple
        style={{ display: 'none' }}
        onChange={(e) => { if (e.target.files?.length) { void handleFileAttach(e.target.files); e.target.value = ''; } }}
      />
      <input
        ref={imageInputRef}
        type="file"
        accept="image/*"
        style={{ display: 'none' }}
        onChange={(e) => { if (e.target.files?.[0]) { void handleImageFile(e.target.files[0]); e.target.value = ''; } }}
      />

      <div style={{ minHeight: '140px', padding: '12px 16px', cursor: 'text' }} onClick={() => editor?.commands.focus()}>
        <EditorContent editor={editor} style={{ outline: 'none', fontSize: '14px', lineHeight: 1.6, color: 'var(--color-text-primary)' }} />
      </div>

      {attachments.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', padding: '0 16px 8px' }}>
          {attachments.map((att) => (
            <span
              key={att.id}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
                padding: '3px 8px',
                background: 'var(--color-bg-secondary)',
                border: '1px solid var(--color-border-subtle)',
                borderRadius: '12px',
                fontSize: '12px',
                color: 'var(--color-text-secondary)',
              }}
            >
              <PaperClipIcon style={{ width: '12px', height: '12px' }} />
              {att.filename} {att.uploading ? `(${t('compose.uploading')})` : `(${fmtSize(att.size)})`}
              {!att.uploading && (
                <button
                  type="button"
                  onClick={() => setAttachments((prev) => prev.filter((a) => a.id !== att.id))}
                  style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0, lineHeight: 1, color: 'var(--color-text-tertiary)', display: 'inline-flex' }}
                >
                  <XMarkIcon style={{ width: '12px', height: '12px' }} />
                </button>
              )}
            </span>
          ))}
        </div>
      )}

      <div style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '8px 12px', borderTop: '1px solid var(--color-border-subtle)' }}>
        <button
          type="button"
          disabled={sending || sent}
          onClick={doSend}
          style={{
            padding: '7px 16px',
            borderRadius: '20px',
            border: 'none',
            background: (sending || sent) ? 'var(--color-border-default)' : 'var(--color-accent)',
            color: '#fff',
            fontSize: '13px',
            fontWeight: 500,
            cursor: (sending || sent) ? 'not-allowed' : 'pointer',
            flexShrink: 0,
          }}
        >
          {sent ? t('compose.sentDone') : sending ? t('compose.sending') : t('compose.send')}
        </button>
        <div style={{ flex: 1 }} />
        <button
          type="button"
          title={t('compose.bold')}
          style={T(editor?.isActive('bold'))}
          onClick={() => editor?.chain().focus().toggleBold().run()}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bold') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
        >
          <b>B</b>
        </button>
        <button
          type="button"
          title={t('compose.italic')}
          style={T(editor?.isActive('italic'))}
          onClick={() => editor?.chain().focus().toggleItalic().run()}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('italic') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
        >
          <i>I</i>
        </button>
        <button
          type="button"
          title={t('compose.underline')}
          style={T(editor?.isActive('underline'))}
          onClick={() => editor?.chain().focus().toggleUnderline().run()}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('underline') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
        >
          <u>U</u>
        </button>
        <button
          type="button"
          title={t('compose.bulletList')}
          style={T(editor?.isActive('bulletList'))}
          onClick={() => editor?.chain().focus().toggleBulletList().run()}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bulletList') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
        >
          <ListBulletIcon style={{ width: '14px', height: '14px' }} />
        </button>
        <button
          type="button"
          title={t('compose.orderedList')}
          style={T(editor?.isActive('orderedList'))}
          onClick={() => editor?.chain().focus().toggleOrderedList().run()}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('orderedList') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
        >
          <NumberedListIcon style={{ width: '14px', height: '14px' }} />
        </button>
        <button
          type="button"
          title={t('compose.link')}
          style={T(editor?.isActive('link'))}
          onClick={handleLinkInsert}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('link') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
        >
          <LinkIcon style={{ width: '14px', height: '14px' }} />
        </button>
        <button
          type="button"
          title={t('compose.insertImage')}
          style={T()}
          onClick={() => imageInputRef.current?.click()}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
        >
          <PhotoIcon style={{ width: '14px', height: '14px' }} />
        </button>
        <div style={{ width: '1px', height: '16px', background: 'var(--color-border-subtle)' }} />
        <button
          type="button"
          title={t('compose.fileAttach')}
          style={T()}
          onClick={() => fileInputRef.current?.click()}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
        >
          <PaperClipIcon style={{ width: '14px', height: '14px' }} />
        </button>
      </div>
    </div>
  );
}
