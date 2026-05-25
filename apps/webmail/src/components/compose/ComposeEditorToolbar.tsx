'use client';

import { useTranslations } from 'next-intl';
import { type Editor } from '@tiptap/react';
import {
  LinkIcon,
  ListBulletIcon,
  NumberedListIcon,
} from '@heroicons/react/24/outline';
import { toolbarBtnStyle } from './toolbarBtnStyle';

interface ComposeEditorToolbarProps {
  editor: Editor | null;
}

export function ComposeEditorToolbar({ editor }: ComposeEditorToolbarProps) {
  const t = useTranslations('composeFull');

  return (
    <>
      <button
        type="button"
        aria-label={t('bold')}
        title={t('bold')}
        style={toolbarBtnStyle(editor?.isActive('bold'))}
        onClick={() => editor?.chain().focus().toggleBold().run()}
        onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
        onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bold') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
      >
        <b>B</b>
      </button>
      <button
        type="button"
        aria-label={t('italic')}
        title={t('italic')}
        style={toolbarBtnStyle(editor?.isActive('italic'))}
        onClick={() => editor?.chain().focus().toggleItalic().run()}
        onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
        onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('italic') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
      >
        <i>I</i>
      </button>
      <button
        type="button"
        aria-label={t('underline')}
        title={t('underline')}
        style={toolbarBtnStyle(editor?.isActive('underline'))}
        onClick={() => editor?.chain().focus().toggleUnderline().run()}
        onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
        onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('underline') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
      >
        <u>U</u>
      </button>
      <button
        type="button"
        aria-label={t('bulletList')}
        title={t('bulletList')}
        style={toolbarBtnStyle(editor?.isActive('bulletList'))}
        onClick={() => editor?.chain().focus().toggleBulletList().run()}
        onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
        onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('bulletList') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
      >
        <ListBulletIcon style={{ width: '14px', height: '14px' }} />
      </button>
      <button
        type="button"
        aria-label={t('numberedList')}
        title={t('numberedList')}
        style={toolbarBtnStyle(editor?.isActive('orderedList'))}
        onClick={() => editor?.chain().focus().toggleOrderedList().run()}
        onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
        onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('orderedList') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
      >
        <NumberedListIcon style={{ width: '14px', height: '14px' }} />
      </button>
      <button
        type="button"
        aria-label={t('link')}
        title={t('link')}
        style={toolbarBtnStyle(editor?.isActive('link'))}
        onClick={() => {
          const url = window.prompt(t('linkPrompt'));
          if (url && editor) editor.chain().focus().setLink({ href: url }).run();
        }}
        onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
        onMouseLeave={(e) => { (e.currentTarget).style.background = editor?.isActive('link') ? 'var(--color-bg-tertiary)' : 'transparent'; }}
      >
        <LinkIcon style={{ width: '14px', height: '14px' }} />
      </button>
    </>
  );
}
