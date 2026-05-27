'use client';

import type { Editor } from '@tiptap/react';
import { useTranslations } from 'next-intl';

interface ComposeImageResizeToolbarProps {
  editor: Editor | null;
  imageResizeToolbar: { top: number; left: number } | null;
}

export function ComposeImageResizeToolbar({
  editor,
  imageResizeToolbar,
}: ComposeImageResizeToolbarProps) {
  const t = useTranslations('composeFull');

  if (!imageResizeToolbar || !editor?.isActive('image')) return null;

  const sizes: [string, string][] = [
    [t('imgSmall'), '25%'],
    [t('imgMedium'), '50%'],
    [t('imgLarge'), '75%'],
    [t('imgOriginal'), '100%'],
  ];

  return (
    <div
      style={{
        position: 'fixed',
        top: imageResizeToolbar.top,
        left: imageResizeToolbar.left,
        zIndex: 500,
        display: 'flex',
        gap: '2px',
        background: 'var(--color-bg-primary)',
        border: '1px solid var(--color-border-default)',
        borderRadius: '6px',
        boxShadow: '0 4px 16px rgba(0,0,0,0.16)',
        padding: '3px',
      }}
    >
      {sizes.map(([label, pct]) => (
        <button
          key={label}
          type="button"
          onMouseDown={(e) => {
            e.preventDefault();
            editor.chain().focus().updateAttributes('image', { style: `width: ${pct}` }).run();
          }}
          style={{
            padding: '2px 8px',
            fontSize: '11px',
            fontWeight: 500,
            borderRadius: '4px',
            border: 'none',
            background: 'transparent',
            color: 'var(--color-text-secondary)',
            cursor: 'pointer',
          }}
          onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
          onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
        >
          {label}
        </button>
      ))}
    </div>
  );
}
