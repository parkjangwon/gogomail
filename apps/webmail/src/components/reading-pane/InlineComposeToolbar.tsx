'use client';

import { type RefObject } from 'react';
import { useTranslations } from 'next-intl';
import type { Editor } from '@tiptap/react';
import {
  LinkIcon,
  PaperClipIcon,
  PhotoIcon,
} from '@heroicons/react/24/outline';
import { ListBulletIcon, NumberedListIcon } from '@heroicons/react/20/solid';
import { toolbarBtnStyleInline } from './readingPaneHelpers';

interface InlineComposeToolbarProps {
  editor: Editor | null;
  sending: boolean;
  sent: boolean;
  doSend: () => void;
  fileInputRef: RefObject<HTMLInputElement | null>;
  imageInputRef: RefObject<HTMLInputElement | null>;
  handleLinkInsert: () => void;
}

export function InlineComposeToolbar({
  editor,
  sending,
  sent,
  doSend,
  fileInputRef,
  imageInputRef,
  handleLinkInsert,
}: InlineComposeToolbarProps) {
  const t = useTranslations('readingFull');
  const T = toolbarBtnStyleInline;

  return (
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
  );
}
