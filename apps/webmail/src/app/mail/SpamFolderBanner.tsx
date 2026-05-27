'use client';

import { moveMessage, type MessageSummary, type Folder } from '@/lib/api';
import type { ToastItem } from '@/components/Toast';

interface SpamFolderBannerProps {
  activeFolderSystemType: string | undefined;
  messages: MessageSummary[];
  folders: Folder[];
  removeVisibleMessages: (ids: string[]) => void;
  setSelectedMessageId: (id: string | null) => void;
  handleBulkDelete: (ids: string[]) => void;
  addToast: (message: string, type?: ToastItem['type']) => void;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
}

export function SpamFolderBanner({
  activeFolderSystemType,
  messages,
  folders,
  removeVisibleMessages,
  setSelectedMessageId,
  handleBulkDelete,
  addToast,
  t,
}: SpamFolderBannerProps) {
  if (activeFolderSystemType !== 'spam' && activeFolderSystemType !== 'junk') return null;

  return (
    <div style={{
      display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap',
      padding: '9px 16px',
      background: 'color-mix(in srgb, var(--color-warning) 10%, transparent)',
      borderBottom: '1px solid color-mix(in srgb, var(--color-warning) 25%, transparent)',
      flexShrink: 0,
    }}>
      <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flex: 1, minWidth: 120 }}>
        {t('misc.mailPage.spamAutoDelete')}
      </span>
      <div style={{ display: 'flex', gap: '6px', flexShrink: 0 }}>
        {messages.length > 0 && (
          <button
            onClick={async () => {
              const inboxFolder = folders.find((f) => f.system_type === 'inbox');
              if (!inboxFolder) return;
              const ids = messages.map((m) => m.id);
              removeVisibleMessages(ids);
              setSelectedMessageId(null);
              await Promise.allSettled(ids.map((id) => moveMessage(id, inboxFolder.id)));
              addToast(t('misc.mailPage.allNotSpam', { count: ids.length }), 'info');
            }}
            style={{ padding: '4px 12px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer', whiteSpace: 'nowrap' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >
            {t('misc.mailPage.markAllNotSpam')}
          </button>
        )}
        {messages.length > 0 && (
          <button
            onClick={() => {
              const ids = messages.map((m) => m.id);
              handleBulkDelete(ids);
            }}
            style={{ padding: '4px 12px', borderRadius: '5px', border: '1px solid var(--color-destructive)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '12px', cursor: 'pointer', whiteSpace: 'nowrap' }}
            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'color-mix(in srgb, var(--color-destructive) 10%, transparent)'; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
          >
            {t('misc.mailPage.emptySpam')}
          </button>
        )}
      </div>
    </div>
  );
}
