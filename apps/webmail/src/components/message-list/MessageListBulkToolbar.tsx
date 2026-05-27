'use client';

import { useCallback } from 'react';
import { useTranslations } from 'next-intl';
import {
  ArchiveBoxIcon,
  ClockIcon,
  EnvelopeIcon,
  EnvelopeOpenIcon,
  StarIcon,
  TrashIcon,
  XMarkIcon,
  BookmarkIcon,
} from '@heroicons/react/24/outline';
import { BookmarkIcon as BookmarkIconSolid, StarIcon as StarIconSolid } from '@heroicons/react/24/solid';

interface MessageListBulkToolbarProps {
  bulkSelected: Set<string>;
  bulkSelectedSize: number;
  clearAll: () => void;
  onBulkMarkRead?: (ids: string[]) => void;
  onBulkToggleRead?: (ids: string[], read: boolean) => void;
  onBulkStar?: (ids: string[], starred: boolean) => void;
  onBulkArchive?: (ids: string[]) => void;
  onBulkSnooze?: (ids: string[], until: Date) => void;
  onBulkPin?: (ids: string[]) => void;
  onBulkMove?: (ids: string[], folderId: string) => void;
  onBulkRestore?: (ids: string[]) => void;
  onBulkLabel?: (ids: string[], color: string | null) => void;
  onBulkDelete?: (ids: string[]) => void;
  folders?: { id: string; name: string; system_type?: string }[];
  bulkMoveOpen: boolean;
  setBulkMoveOpen: (value: boolean) => void;
  bulkReadTarget?: boolean;
  bulkStarTarget?: boolean;
  bulkPinned?: boolean;
}

export function MessageListBulkToolbar({
  bulkSelected,
  bulkSelectedSize,
  clearAll,
  onBulkMarkRead,
  onBulkToggleRead,
  onBulkStar,
  onBulkArchive,
  onBulkSnooze,
  onBulkPin,
  onBulkMove,
  onBulkRestore,
  onBulkLabel,
  onBulkDelete,
  folders,
  bulkMoveOpen,
  setBulkMoveOpen,
  bulkReadTarget = true,
  bulkStarTarget = true,
  bulkPinned = false,
}: MessageListBulkToolbarProps) {
  const t = useTranslations('mailListFull');
  const tSidebar = useTranslations('sidebar');
  const SYSTEM_TYPE_KEYS: Record<string, string> = {
    inbox: 'system.inbox', sent: 'system.sent', drafts: 'system.drafts',
    trash: 'system.trash', spam: 'system.spam', archive: 'system.archive',
  };
  const localizedFolderName = useCallback((f: { name: string; system_type?: string }): string => {
    if (f.system_type && SYSTEM_TYPE_KEYS[f.system_type]) {
      try { return tSidebar(SYSTEM_TYPE_KEYS[f.system_type] as Parameters<typeof tSidebar>[0]); } catch { /* */ }
    }
    return f.name;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tSidebar]);

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0, background: 'var(--color-accent-subtle)' }}>
      <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', flex: 1 }}>{t('bulk.selectedCount', { count: bulkSelectedSize })}</span>
      {(onBulkToggleRead || onBulkMarkRead) && (
        <button
          onClick={() => {
            const ids = [...bulkSelected];
            if (onBulkToggleRead) onBulkToggleRead(ids, bulkReadTarget);
            else if (bulkReadTarget) onBulkMarkRead?.(ids);
            clearAll();
          }}
          title={bulkReadTarget ? t('bulk.markRead') : t('bulk.markUnread')}
          style={{ padding: '4px 8px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}
        >
          {bulkReadTarget ? <EnvelopeIcon style={{ width: '13px', height: '13px' }} /> : <EnvelopeOpenIcon style={{ width: '13px', height: '13px' }} />}
        </button>
      )}
      {onBulkStar && (
        <button onClick={() => { onBulkStar([...bulkSelected], bulkStarTarget); clearAll(); }} title={bulkStarTarget ? t('bulk.addStar') : t('bulk.removeStar')} style={{ padding: '4px 8px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', color: bulkStarTarget ? '#f59e0b' : 'var(--color-text-tertiary)', display: 'inline-flex', alignItems: 'center' }}>
          {bulkStarTarget ? <StarIconSolid style={{ width: '13px', height: '13px' }} /> : <StarIcon style={{ width: '13px', height: '13px' }} />}
        </button>
      )}
      {onBulkArchive && (
        <button onClick={() => { onBulkArchive([...bulkSelected]); clearAll(); }} title={t('bulk.archive')} style={{ padding: '4px 8px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}>
          <ArchiveBoxIcon style={{ width: '13px', height: '13px' }} />
        </button>
      )}
      {onBulkSnooze && (
        <button onClick={() => { onBulkSnooze([...bulkSelected], new Date(Date.now() + 60 * 60 * 1000)); clearAll(); }} title={t('bulk.snooze1h')} style={{ padding: '4px 8px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}>
          <ClockIcon style={{ width: '13px', height: '13px' }} />
        </button>
      )}
      {onBulkPin && (
        <button onClick={() => { onBulkPin([...bulkSelected]); clearAll(); }} title={bulkPinned ? t('bulk.unpin') : t('bulk.pin')} style={{ padding: '4px 8px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: bulkPinned ? 'var(--color-accent)' : 'var(--color-text-tertiary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}>
          {bulkPinned ? <BookmarkIconSolid style={{ width: '13px', height: '13px' }} /> : <BookmarkIcon style={{ width: '13px', height: '13px' }} />}
        </button>
      )}
      {onBulkMove && folders && folders.length > 0 && (
        <div style={{ position: 'relative' }}>
          <button onClick={() => setBulkMoveOpen(!bulkMoveOpen)} style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
            {t('bulk.move')}
          </button>
          {bulkMoveOpen && (
            <div style={{ position: 'absolute', top: '100%', left: 0, marginTop: '4px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '6px', boxShadow: '0 4px 16px rgba(0,0,0,0.12)', zIndex: 200, minWidth: '140px', overflow: 'hidden' }}>
              {folders.map((f) => (
                <button
                  key={f.id}
                  onClick={() => { onBulkMove([...bulkSelected], f.id); clearAll(); setBulkMoveOpen(false); }}
                  style={{ display: 'block', width: '100%', textAlign: 'left', padding: '8px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >
                  {localizedFolderName(f)}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
      {onBulkRestore && (
        <button onClick={() => { onBulkRestore([...bulkSelected]); clearAll(); }} style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
          {t('bulk.restore')}
        </button>
      )}
      {onBulkLabel && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
          {['#ef4444', '#f97316', '#eab308', '#22c55e', '#3b82f6', '#a855f7'].map((color) => (
            <button
              key={color}
              title={t('bulk.applyLabel')}
              onClick={() => { onBulkLabel([...bulkSelected], color); clearAll(); }}
              style={{ width: '14px', height: '14px', borderRadius: '50%', background: color, border: 'none', cursor: 'pointer', flexShrink: 0 }}
            />
          ))}
          <button
            title={t('bulk.removeLabel')}
            onClick={() => { onBulkLabel([...bulkSelected], null); clearAll(); }}
            style={{ padding: '3px 6px', borderRadius: '10px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}
          >
            <XMarkIcon style={{ width: '11px', height: '11px' }} />
          </button>
        </div>
      )}
      {onBulkDelete && (
        <button onClick={() => { onBulkDelete([...bulkSelected]); clearAll(); }} title={t('bulk.deleteTitle')} style={{ padding: '4px 8px', borderRadius: '12px', border: '1px solid rgba(217,79,61,0.4)', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}>
          <TrashIcon style={{ width: '13px', height: '13px' }} />
        </button>
      )}
      <button onClick={clearAll} style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '12px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
        {t('bulk.cancel')}
      </button>
    </div>
  );
}
