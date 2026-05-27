'use client';

import type { Dispatch, SetStateAction } from 'react';
import type { Editor } from '@tiptap/react';
import { useTranslations } from 'next-intl';
import type { UIComposeIntent } from '@/lib/api';

interface ComposeModalHeaderProps {
  minimized: boolean;
  setMinimized: Dispatch<SetStateAction<boolean>>;
  fullscreen: boolean;
  setFullscreen: Dispatch<SetStateAction<boolean>>;
  isMobile?: boolean;
  intent: UIComposeIntent;
  subject: string;
  sent: boolean;
  to: string;
  editor: Editor | null;
  setConfirmClose: Dispatch<SetStateAction<boolean>>;
  onClose: () => void;
  startDrag: (e: React.MouseEvent<HTMLDivElement>) => void;
}

export function ComposeModalHeader({
  minimized,
  setMinimized,
  fullscreen,
  setFullscreen,
  isMobile,
  intent,
  subject,
  sent,
  to,
  editor,
  setConfirmClose,
  onClose,
  startDrag,
}: ComposeModalHeaderProps) {
  const t = useTranslations('composeFull');

  return (
    <div
      onClick={minimized ? () => setMinimized(false) : undefined}
      onMouseDown={startDrag}
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '10px 16px',
        borderBottom: minimized ? 'none' : '1px solid var(--color-border-subtle)',
        background: 'var(--color-bg-secondary)',
        borderRadius: minimized ? '8px' : '8px 8px 0 0',
        cursor: minimized ? 'pointer' : (fullscreen || isMobile ? 'default' : 'move'),
        flexShrink: 0,
      }}
    >
      <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1, minWidth: 0 }}>
        {minimized && subject ? subject : (intent === 'reply' || intent === 'reply_all' ? t('titleReply') : intent === 'forward' ? t('titleForward') : t('titleNew'))}
      </span>
      <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flexShrink: 0, marginLeft: '8px' }}>
        {!isMobile && (
          <>
            <button
              onClick={(e) => { e.stopPropagation(); setFullscreen((v) => !v); if (minimized) setMinimized(false); }}
              aria-label={fullscreen ? t('shrinkWindow') : t('fullscreen')}
              title={fullscreen ? t('shrinkWindow') : t('fullscreen')}
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
              aria-label={minimized ? t('restoreWindow') : t('minimizeWindow')}
              style={{
                width: '24px', height: '24px', borderRadius: '4px', border: 'none',
                background: 'transparent', color: 'var(--color-text-secondary)',
                cursor: 'pointer', fontSize: '14px', lineHeight: 1,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >{minimized ? '□' : '─'}</button>
          </>
        )}
        <button
          onClick={() => {
            const hasContent = !sent && (to.trim() || subject.trim() || (editor && editor.getText().trim()));
            if (hasContent) setConfirmClose(true); else onClose();
          }}
          aria-label={t('closeWindow')}
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
  );
}
