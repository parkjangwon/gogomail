'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';
import { MessageSummary } from '@/lib/api';
import { formatDate, readingTimeLabel } from './messageListTypes';
import {
  StarIcon,
  EnvelopeIcon,
  EnvelopeOpenIcon,
  ArchiveBoxIcon,
  TrashIcon,
  ClockIcon,
  BookmarkIcon,
} from '@heroicons/react/24/outline';
import { BookmarkIcon as BookmarkIconSolid, StarIcon as StarIconSolid } from '@heroicons/react/24/solid';
import { SnoozePopover } from '../SnoozePopover';

interface MessageRowActionsProps {
  message: MessageSummary;
  hovered: boolean;
  isPinned: boolean;
  hasNote: boolean;
  compact: boolean;
  onStar?: (id: string, starred: boolean) => void;
  onHoverToggleRead?: (id: string, read: boolean) => void;
  onHoverArchive?: (id: string) => void;
  onHoverSnooze?: (id: string, until: Date) => void;
  onHoverPin?: (id: string) => void;
  onHoverDelete?: (id: string) => void;
}

const hoverActionStyle = {
  background: 'none',
  border: 'none',
  cursor: 'pointer',
  padding: '4px 4px 2px',
  color: 'var(--color-text-tertiary)',
  borderRadius: '4px',
  display: 'inline-flex',
  flexDirection: 'column' as const,
  alignItems: 'center',
  gap: '1px',
};

export function MessageRowActions({
  message,
  hovered,
  isPinned,
  hasNote,
  compact,
  onStar,
  onHoverToggleRead,
  onHoverArchive,
  onHoverSnooze,
  onHoverPin,
  onHoverDelete,
}: MessageRowActionsProps) {
  const t = useTranslations('mailListFull');
  const [showSnoozePopover, setShowSnoozePopover] = useState(false);

  return (
    <div style={{ width: '120px', flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: '1px', alignSelf: 'center', position: 'relative', zIndex: 1, pointerEvents: 'auto' }}>
      {hovered ? (
        <>
          {onStar && (
            <button
              type="button"
              aria-label={message.starred ? t('row.starRemoveTitle') : t('row.starAddTitle')}
              title={message.starred ? t('row.starRemoveTitle') : t('row.starAddTitle')}
              draggable={false}
              onMouseDown={(e) => e.stopPropagation()}
              onClick={(e) => { e.stopPropagation(); onStar(message.id, !message.starred); }}
              style={{ ...hoverActionStyle, color: message.starred ? '#f59e0b' : 'var(--color-text-tertiary)' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
            >
              {message.starred ? <StarIconSolid style={{ width: '14px', height: '14px' }} /> : <StarIcon style={{ width: '14px', height: '14px' }} />}
            </button>
          )}
          {onHoverToggleRead && (
            <button
              type="button"
              aria-label={message.read ? t('row.toggleReadToUnread') : t('row.toggleReadToRead')}
              title={message.read ? t('row.toggleReadTitleToUnread') : t('row.toggleReadTitleToRead')}
              draggable={false}
              onMouseDown={(e) => e.stopPropagation()}
              onClick={(e) => { e.stopPropagation(); onHoverToggleRead(message.id, !message.read); }}
              style={hoverActionStyle}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
            >
              {message.read ? <EnvelopeOpenIcon style={{ width: '14px', height: '14px' }} /> : <EnvelopeIcon style={{ width: '14px', height: '14px' }} />}
              <kbd style={{ fontSize: '8px', lineHeight: 1, color: 'var(--color-text-tertiary)', background: 'none', border: 'none', fontFamily: 'monospace', fontWeight: 700 }}>M</kbd>
            </button>
          )}
          {onHoverArchive && (
            <button
              type="button"
              aria-label={t('row.archive')}
              title={t('row.archiveTitle')}
              draggable={false}
              onMouseDown={(e) => e.stopPropagation()}
              onClick={(e) => { e.stopPropagation(); onHoverArchive(message.id); }}
              style={hoverActionStyle}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
            >
              <ArchiveBoxIcon style={{ width: '14px', height: '14px' }} />
              <kbd style={{ fontSize: '8px', lineHeight: 1, color: 'var(--color-text-tertiary)', background: 'none', border: 'none', fontFamily: 'monospace', fontWeight: 700 }}>E</kbd>
            </button>
          )}
          {onHoverSnooze && (
            <div style={{ position: 'relative' }}>
              <button
                type="button"
                aria-label={t('row.snooze')}
                title={t('row.snoozeTitle')}
                draggable={false}
                onMouseDown={(e) => e.stopPropagation()}
                onClick={(e) => { e.stopPropagation(); setShowSnoozePopover((v) => !v); }}
                style={hoverActionStyle}
                onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
              >
                <ClockIcon style={{ width: '14px', height: '14px' }} />
                <kbd style={{ fontSize: '8px', lineHeight: 1, color: 'var(--color-text-tertiary)', background: 'none', border: 'none', fontFamily: 'monospace', fontWeight: 700 }}>Z</kbd>
              </button>
              {showSnoozePopover && (
                <SnoozePopover
                  onSnooze={(until) => onHoverSnooze(message.id, until)}
                  onClose={() => setShowSnoozePopover(false)}
                  align="right"
                />
              )}
            </div>
          )}
          {onHoverPin && (
            <button
              type="button"
              aria-label={isPinned ? t('row.unpin') : t('row.pin')}
              title={isPinned ? t('row.unpinTitle') : t('row.pinTitle')}
              draggable={false}
              onMouseDown={(e) => e.stopPropagation()}
              onClick={(e) => { e.stopPropagation(); onHoverPin(message.id); }}
              style={{ ...hoverActionStyle, color: isPinned ? 'var(--color-accent)' : 'var(--color-text-tertiary)' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
            >
              {isPinned ? <BookmarkIconSolid style={{ width: '14px', height: '14px' }} /> : <BookmarkIcon style={{ width: '14px', height: '14px' }} />}
              <kbd style={{ fontSize: '8px', lineHeight: 1, color: 'var(--color-text-tertiary)', background: 'none', border: 'none', fontFamily: 'monospace', fontWeight: 700 }}>P</kbd>
            </button>
          )}
          {onHoverDelete && (
            <button
              type="button"
              aria-label={t('row.delete')}
              title={t('row.deleteTitle')}
              draggable={false}
              onMouseDown={(e) => e.stopPropagation()}
              onClick={(e) => { e.stopPropagation(); onHoverDelete(message.id); }}
              style={hoverActionStyle}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
            >
              <TrashIcon style={{ width: '14px', height: '14px' }} />
              <kbd style={{ fontSize: '8px', lineHeight: 1, color: 'var(--color-text-tertiary)', background: 'none', border: 'none', fontFamily: 'monospace', fontWeight: 700 }}>#</kbd>
            </button>
          )}
        </>
      ) : (
        <>
          {isPinned && <BookmarkIconSolid style={{ width: '12px', height: '12px', color: 'var(--color-accent)', marginRight: '2px', flexShrink: 0 }} />}
          {message.starred && <StarIconSolid style={{ width: '12px', height: '12px', color: '#f59e0b', marginRight: '2px', flexShrink: 0 }} />}
          {hasNote && <span title={t('row.noteHover')} style={{ width: '6px', height: '6px', borderRadius: '50%', background: '#a78bfa', display: 'inline-block', marginRight: '3px', flexShrink: 0 }} />}
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'flex-end', gap: '1px' }}>
            <span
              style={{ fontSize: '12px', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}
              title={new Intl.DateTimeFormat(undefined, { dateStyle: 'full', timeStyle: 'short' }).format(new Date(message.received_at))}
            >
              {formatDate(message.received_at)}
            </span>
            {!compact && message.preview && (
              <span style={{ fontSize: '10px', color: 'var(--color-text-tertiary)', whiteSpace: 'nowrap' }}>
                {readingTimeLabel(message.preview)}
              </span>
            )}
          </div>
        </>
      )}
    </div>
  );
}
