import { useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { SnoozePopover } from '../SnoozePopover';
import { MessageRowProps } from './messageListTypes';
import { useWebmailAvatar } from '@/lib/webmailAvatar';
import {
  formatDate,
  getAutoCategory,
  avatarColor,
  highlight,
  readingTimeLabel,
} from './messageListTypes';
import {
  StarIcon,
  EnvelopeIcon,
  EnvelopeOpenIcon,
  ArchiveBoxIcon,
  TrashIcon,
  PaperClipIcon,
  CheckIcon as CheckIconOutline,
  ClockIcon,
  BookmarkIcon,
} from '@heroicons/react/24/outline';
import { BookmarkIcon as BookmarkIconSolid } from '@heroicons/react/24/solid';
import { StarIcon as StarIconSolid } from '@heroicons/react/24/solid';

export function MessageRow({
  message,
  isSelected,
  isBulkChecked,
  onSelect,
  onStar,
  onToggleBulk,
  onContextMenu,
  searchQuery,
  compact,
  onDelete,
  onArchiveRow,
  onHoverDelete,
  onHoverArchive,
  onHoverToggleRead,
  onHoverSnooze,
  onHoverPin,
  isPinned,
  threadCount,
  labelColor,
  userEmail,
  showPreview = true,
  hasNote = false,
  isImportant = false,
  folderLabel,
  onAvatarEnter,
  onAvatarLeave,
  onHoverChange,
}: MessageRowProps) {
  const t = useTranslations('mailListFull');
  const q = searchQuery ?? '';
  const isUnread = !message.read;
  const swipeRef = useRef<{ startX: number; startY: number } | null>(null);
  const avatarRef = useRef<HTMLDivElement>(null);
  const [swipeX, setSwipeX] = useState(0);
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const [showSnoozePopover, setShowSnoozePopover] = useState(false);
  const swipeEnabled = onDelete || onArchiveRow;
  const userAvatarUrl = useWebmailAvatar();
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
  return (
    <div
      role="listitem"
      data-message-id={message.id}
      tabIndex={0}
      data-nav-group="message-list"
      data-nav-current={isSelected ? 'true' : undefined}
      onMouseDown={(e) => { if (e.button === 0) e.currentTarget.focus(); }}
      onFocusCapture={() => setFocused(true)}
      onBlurCapture={(e) => {
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setFocused(false);
      }}
      style={{ position: 'relative', overflow: 'hidden', borderLeft: labelColor ? `3px solid ${labelColor}` : '3px solid transparent' }}
    >
      {onArchiveRow && swipeX > 20 && (
        <div aria-hidden="true" style={{ position: 'absolute', left: 0, top: 0, bottom: 0, width: Math.min(120, swipeX), background: 'var(--color-accent)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: '13px', fontWeight: 600, pointerEvents: 'none' }}>
          {swipeX > 70 ? t('row.swipeArchive') : '→'}
        </div>
      )}
      {onDelete && swipeX < -20 && (
        <div aria-hidden="true" style={{ position: 'absolute', right: 0, top: 0, bottom: 0, width: Math.min(120, -swipeX), background: 'var(--color-destructive)', display: 'flex', alignItems: 'center', justifyContent: 'center', color: '#fff', fontSize: '13px', fontWeight: 600, pointerEvents: 'none' }}>
          {-swipeX > 70 ? t('row.swipeDelete') : '←'}
        </div>
      )}
      <div
        draggable={!swipeEnabled}
        onDragStart={!swipeEnabled ? (e) => { e.dataTransfer.setData('text/plain', message.id); e.dataTransfer.effectAllowed = 'move'; } : undefined}
        onTouchStart={swipeEnabled ? (e) => { swipeRef.current = { startX: e.touches[0].clientX, startY: e.touches[0].clientY }; } : undefined}
        onTouchMove={swipeEnabled ? (e) => {
          if (!swipeRef.current) return;
          const dx = e.touches[0].clientX - swipeRef.current.startX;
          const dy = e.touches[0].clientY - swipeRef.current.startY;
          if (Math.abs(dy) > Math.abs(dx)) { swipeRef.current = null; return; }
          e.preventDefault();
          const minX = onDelete ? -120 : 0;
          const maxX = onArchiveRow ? 120 : 0;
          setSwipeX(Math.max(minX, Math.min(maxX, dx)));
        } : undefined}
        onTouchEnd={swipeEnabled ? () => {
          if (swipeX < -70 && onDelete) onDelete(message.id);
          else if (swipeX > 70 && onArchiveRow) onArchiveRow(message.id);
          setSwipeX(0);
          swipeRef.current = null;
        } : undefined}
        onClick={() => { if (swipeX !== 0) { setSwipeX(0); return; } onSelect(message.id); }}
        onContextMenu={onContextMenu ? (e) => { e.preventDefault(); onContextMenu(message.id, e.clientX, e.clientY); } : undefined}
        aria-selected={isSelected}
        style={{
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
          padding: compact ? '4px 16px' : '9px 16px',
          borderBottom: '1px solid var(--color-border-subtle)',
          background: isSelected
            ? 'var(--color-accent-subtle)'
            : focused
              ? 'rgba(37, 99, 235, 0.06)'
              : hovered
                ? 'var(--color-bg-secondary)'
                : 'var(--color-bg-primary)',
          boxShadow: focused ? 'inset 0 0 0 1px rgba(37, 99, 235, 0.14)' : 'none',
          cursor: 'pointer',
          transition: 'background 100ms ease, transform 80ms ease, box-shadow 100ms ease',
          position: 'relative',
          transform: `translateX(${swipeX}px)`,
        }}
        onMouseEnter={() => { setHovered(true); onHoverChange?.(message.id); }}
        onMouseLeave={() => { setHovered(false); onHoverChange?.(null); }}
      >
        <button
          type="button"
          onClick={(e) => { e.stopPropagation(); onToggleBulk(message.id, e.shiftKey); }}
          title={isBulkChecked ? t('row.deselect') : t('row.select')}
          aria-label={isBulkChecked ? t('row.deselect') : t('row.select')}
          style={{
            width: '18px',
            height: '18px',
            flexShrink: 0,
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            cursor: 'pointer',
            alignSelf: 'center',
            border: 'none',
            background: 'transparent',
            padding: 0,
            opacity: hovered || isBulkChecked ? 1 : 0,
            transition: 'opacity 100ms ease',
          }}
        >
          <div
            aria-hidden="true"
            style={{
              width: '14px',
              height: '14px',
              borderRadius: '3px',
              boxSizing: 'border-box',
              border: `1.5px solid ${isBulkChecked ? 'var(--color-accent)' : 'var(--color-border-default)'}`,
              background: isBulkChecked ? 'var(--color-accent)' : 'var(--color-bg-primary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              transition: 'border-color 100ms ease, background 100ms ease, transform 100ms ease',
              transform: hovered || isBulkChecked ? 'scale(1)' : 'scale(0.92)',
            }}
          >
            {isBulkChecked ? (
              <CheckIconOutline style={{ width: '10px', height: '10px', color: '#fff', strokeWidth: 2.5 }} />
            ) : hovered ? (
              <div style={{ width: '6px', height: '6px', borderRadius: '50%', background: isUnread ? 'var(--color-accent)' : 'var(--color-border-default)' }} />
            ) : null}
          </div>
        </button>

        {!compact && (
          <div ref={avatarRef} aria-hidden="true" style={{ width: '32px', height: '32px', borderRadius: '50%', flexShrink: 0, background: avatarColor(message.from_name || message.from_addr), color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '13px', fontWeight: 600, userSelect: 'none', alignSelf: 'center', overflow: 'hidden' }}
            onMouseEnter={() => { if (avatarRef.current && onAvatarEnter) { onAvatarEnter(message.from_name || '', message.from_addr, avatarRef.current.getBoundingClientRect()); } }}
            onMouseLeave={() => onAvatarLeave?.()}
          >
            {(() => {
              const avatarUrl = message.sender_avatar_url || (userEmail && message.from_addr === userEmail ? userAvatarUrl : '');
              if (avatarUrl) {
                return <img src={avatarUrl} alt="" style={{ width: '100%', height: '100%', objectFit: 'cover' }} />;
              }
              return (message.from_name || message.from_addr).charAt(0).toUpperCase();
            })()}
          </div>
        )}

        <div style={{ width: compact ? '100px' : '130px', flexShrink: 0, minWidth: 0, alignSelf: 'center' }}>
          <div style={{ fontSize: '13px', fontWeight: isUnread ? 600 : 400, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
            {highlight(message.from_name || message.from_addr, q)}
          </div>
          {!compact && message.from_name && (
            <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', marginTop: '1px' }}>
              {message.from_addr}
            </div>
          )}
        </div>

        <div style={{ width: '16px', flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center', alignSelf: 'center' }}>
          {message.has_attachment && (
            <PaperClipIcon aria-label={t('row.attachment')} style={{ width: '13px', height: '13px', color: 'var(--color-text-tertiary)' }} />
          )}
        </div>

        <div style={{ flex: 1, minWidth: 0, overflow: 'hidden', alignSelf: 'center' }}>
          <div style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: '13px' }}>
            {isImportant && <span title={t('row.importantTitle')} aria-label={t('row.important')} style={{ color: '#eab308', marginRight: '4px', fontSize: '10px', verticalAlign: 'middle' }}>▶</span>}
            {message.starred && <StarIconSolid aria-label={t('row.starred')} title={t('row.starred')} style={{ width: '12px', height: '12px', color: '#f59e0b', marginRight: '4px', verticalAlign: '-1px', display: 'inline-block' }} />}
            {folderLabel && <span title={t('row.folderBadgeTitle', { label: folderLabel })} style={{ display: 'inline-block', marginRight: '5px', padding: '1px 5px', borderRadius: '999px', background: 'var(--color-bg-tertiary)', color: 'var(--color-text-tertiary)', fontSize: '10px', fontWeight: 600, verticalAlign: '1px' }}>{folderLabel}</span>}
            <span style={{ fontWeight: isUnread ? 600 : 400, color: 'var(--color-text-primary)' }}>
              {highlight(message.subject || t('row.noSubject'), q)}
            </span>
            {threadCount && threadCount > 1 && (
              <span aria-label={t('row.threadCountAria', { count: threadCount })} style={{ marginLeft: '5px', fontSize: '11px', color: (message.unread_count ?? 0) > 0 ? 'var(--color-accent)' : 'var(--color-text-tertiary)', background: (message.unread_count ?? 0) > 0 ? 'var(--color-accent-subtle)' : 'var(--color-bg-tertiary)', borderRadius: '10px', padding: '1px 6px', verticalAlign: 'middle', fontWeight: 500 }}>{threadCount}</span>
            )}
            {(() => {
              const cat = getAutoCategory(message.from_addr, message.subject);
              return cat ? <span style={{ marginLeft: '5px', fontSize: '10px', fontWeight: 600, padding: '1px 5px', borderRadius: '3px', background: cat.color + '1a', color: cat.color, flexShrink: 0, verticalAlign: 'middle', letterSpacing: '0.02em' }}>{cat.label}</span> : null;
            })()}
            {showPreview && message.preview && (
              <span style={{ color: 'var(--color-text-secondary)', fontWeight: 400 }}>
                {' · '}{highlight(message.preview, q)}
              </span>
            )}
          </div>
        </div>

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
                <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}
                  title={new Intl.DateTimeFormat(undefined, { dateStyle: 'full', timeStyle: 'short' }).format(new Date(message.received_at))}>
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
      </div>
    </div>
  );
}
