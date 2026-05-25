'use client';

import type { ClipboardEvent, KeyboardEvent } from 'react';
import { LinkIcon, PaperAirplaneIcon } from '@heroicons/react/24/outline';

type DMComposerProps = {
  composer: string;
  driveFileId: string;
  driveComposerOpen: boolean;
  activeRoomId: string;
  composerComposingRef: React.RefObject<boolean>;
  onChangeComposer: (value: string) => void;
  onChangeDriveFileId: (value: string) => void;
  onToggleDriveComposer: () => void;
  onSend: () => void;
  onPaste: (event: ClipboardEvent<HTMLInputElement>) => void;
  onCompositionStart: () => void;
  onCompositionEnd: () => void;
  t: (key: string, params?: Record<string, string | number>) => string;
};

export function DMComposer({
  composer,
  driveFileId,
  driveComposerOpen,
  activeRoomId,
  composerComposingRef,
  onChangeComposer,
  onChangeDriveFileId,
  onToggleDriveComposer,
  onSend,
  onPaste,
  onCompositionStart,
  onCompositionEnd,
  t,
}: DMComposerProps) {
  function handleKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (e.key === 'Enter' && !e.shiftKey) {
      const nativeEvent = e.nativeEvent as globalThis.KeyboardEvent & { isComposing?: boolean };
      if (nativeEvent.isComposing || nativeEvent.keyCode === 229 || composerComposingRef.current) return;
      e.preventDefault();
      onSend();
    }
  }

  return (
    <footer style={{ borderTop: '1px solid var(--color-border-subtle)', padding: '9px 10px', flexShrink: 0 }}>
      {driveComposerOpen && (
        <input
          value={driveFileId}
          onChange={(e) => onChangeDriveFileId(e.currentTarget.value)}
          placeholder={t('driveFileId')}
          style={{ width: '100%', boxSizing: 'border-box', marginBottom: 8, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '7px 9px', fontSize: 13 }}
        />
      )}
      <div style={{ display: 'flex', gap: 7, minWidth: 0 }}>
        <button
          type="button"
          onClick={onToggleDriveComposer}
          aria-label={t('addDriveFile')}
          style={{ width: 34, minWidth: 34, border: '1px solid var(--color-border-default)', borderRadius: 6, background: driveComposerOpen ? 'var(--color-accent-subtle)' : 'transparent', color: driveComposerOpen ? 'var(--color-accent)' : 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}
        >
          <LinkIcon style={{ width: 16, height: 16 }} />
        </button>
        <input
          value={composer}
          onChange={(e) => onChangeComposer(e.currentTarget.value)}
          onPaste={onPaste}
          onCompositionStart={onCompositionStart}
          onCompositionEnd={onCompositionEnd}
          onKeyDown={handleKeyDown}
          placeholder={t('message')}
          style={{ flex: '1 1 120px', minWidth: 0, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '7px 9px', fontSize: 13 }}
        />
        <button
          type="button"
          onClick={onSend}
          disabled={!composer.trim() && !driveFileId.trim()}
          aria-label={t('sendMessage')}
          style={{ width: 34, minWidth: 34, border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}
        >
          <PaperAirplaneIcon style={{ width: 17, height: 17 }} />
        </button>
      </div>
    </footer>
  );
}
