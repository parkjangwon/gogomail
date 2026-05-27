'use client';

import { useCallback, useEffect, useRef, useState, type CSSProperties } from 'react';
import { useTranslations } from 'next-intl';
import type { MessageDetail, Folder } from '@/lib/api';
import { EllipsisHorizontalIcon } from '@heroicons/react/24/outline';

interface MailActionsMoreMenuProps {
  message: MessageDetail;
  folders: Folder[];
  onMove?: (folderId: string) => void;
  onSnooze?: (messageId: string, until: Date) => void;
  onPrint?: () => void;
  onToggleRead?: () => void;
  isRead: boolean;
  onToggleThreadMute?: () => void;
  isThreadMuted: boolean;
  onSpam?: () => void;
  onNotSpam?: () => void;
  onRestore?: () => void;
  unsubscribeUrl: string | null;
  fontSize: number;
  onIncreaseFontSize: () => void;
  onDecreaseFontSize: () => void;
}

const triggerButtonStyle: CSSProperties = {
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  cursor: 'pointer',
  borderRadius: '5px',
  border: 'none',
  background: 'transparent',
  color: 'var(--color-text-secondary)',
  padding: '5px 8px',
  transition: 'background 100ms ease',
};

const SYSTEM_TYPE_KEYS: Record<string, string> = {
  inbox: 'system.inbox', sent: 'system.sent', drafts: 'system.drafts',
  trash: 'system.trash', spam: 'system.spam', archive: 'system.archive',
};

export function MailActionsMoreMenu({
  message,
  folders,
  onMove,
  onSnooze,
  onPrint,
  onToggleRead,
  isRead,
  onToggleThreadMute,
  isThreadMuted,
  onSpam,
  onNotSpam,
  onRestore,
  unsubscribeUrl,
  fontSize,
  onIncreaseFontSize,
  onDecreaseFontSize,
}: MailActionsMoreMenuProps) {
  const t = useTranslations('mail');
  const tSidebar = useTranslations('sidebar');

  const [showMoreMenu, setShowMoreMenu] = useState(false);
  const moreMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!showMoreMenu) return;
    const onMouseDown = (event: MouseEvent) => {
      if (moreMenuRef.current && !moreMenuRef.current.contains(event.target as Node)) {
        setShowMoreMenu(false);
      }
    };
    document.addEventListener('mousedown', onMouseDown);
    return () => document.removeEventListener('mousedown', onMouseDown);
  }, [showMoreMenu]);

  const localizedFolderName = useCallback((f: Folder): string => {
    if (f.system_type && SYSTEM_TYPE_KEYS[f.system_type]) {
      try { return tSidebar(SYSTEM_TYPE_KEYS[f.system_type] as Parameters<typeof tSidebar>[0]); } catch { /* */ }
    }
    return f.name || f.full_path;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tSidebar]);

  const onOpenDateForSnooze = useCallback((ms: number) => {
    if (ms < 24 * 60 * 60 * 1000) {
      const date = new Date();
      date.setTime(Date.now() + ms);
      return date;
    }
    const date = new Date(ms);
    return date;
  }, []);

  return (
    <div ref={moreMenuRef} style={{ position: 'relative' }}>
      <button
        aria-label={t('moreActions')}
        title={t('moreActions')}
        onClick={() => setShowMoreMenu((v) => !v)}
        style={triggerButtonStyle}
        onMouseEnter={(e) => {
          (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
        }}
        onMouseLeave={(e) => {
          (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
        }}
      ><EllipsisHorizontalIcon style={{ width: '18px', height: '18px' }} /></button>
      {showMoreMenu && (
        <div
          style={{
            position: 'absolute',
            top: '100%',
            right: 0,
            marginTop: '4px',
            background: 'var(--color-bg-primary)',
            border: '1px solid var(--color-border-default)',
            borderRadius: '8px',
            boxShadow: '0 4px 20px rgba(0,0,0,0.14)',
            zIndex: 300,
            minWidth: '200px',
            overflow: 'hidden',
          }}
        >
          {onMove && folders.length > 0 && (
            <>
              <div style={{ padding: '6px 14px 2px', fontSize: '11px', color: 'var(--color-text-tertiary)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em' }}>{t('moveTo')}</div>
              {folders.map((folder) => (
                <button
                  key={folder.id}
                  onClick={() => {
                    onMove(folder.id);
                    setShowMoreMenu(false);
                  }}
                  style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
                  onMouseEnter={(e) => {
                    (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
                  }}
                  onMouseLeave={(e) => {
                    (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
                  }}
                >{localizedFolderName(folder)}</button>
              ))}
              <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '4px 0' }} />
            </>
          )}

          {onSnooze && (
            <>
              <div style={{ padding: '6px 14px 2px', fontSize: '11px', color: 'var(--color-text-tertiary)', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.06em' }}>{t('snooze')}</div>
              {[
                { label: t('snooze1h'), ms: 60 * 60 * 1000 },
                { label: t('snooze4h'), ms: 4 * 60 * 60 * 1000 },
                { label: t('snoozeTonight'), ms: (() => { const d = new Date(); d.setHours(18, 0, 0, 0); return d.getTime() > Date.now() ? d.getTime() - Date.now() : 24 * 3600000; })() },
                { label: t('snoozeTomorrow'), ms: (() => { const d = new Date(); d.setDate(d.getDate() + 1); d.setHours(9, 0, 0, 0); return d.getTime() - Date.now(); })() },
              ].map((option) => (
                <button
                  key={option.label}
                  onClick={() => {
                    onSnooze(message.id, onOpenDateForSnooze(option.ms));
                    setShowMoreMenu(false);
                  }}
                  style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
                  onMouseEnter={(e) => {
                    (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
                  }}
                  onMouseLeave={(e) => {
                    (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
                  }}
                >{option.label}</button>
              ))}
              <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '4px 0' }} />
            </>
          )}

          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '6px 14px' }}>
            <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flex: 1 }}>{t('fontSize')}</span>
            <button onClick={onDecreaseFontSize} style={{ fontSize: '12px', padding: '2px 7px', border: '1px solid var(--color-border-default)', borderRadius: '4px', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>A-</button>
            <span style={{ fontSize: '12px', color: 'var(--color-text-primary)', minWidth: '20px', textAlign: 'center' }}>{fontSize}</span>
            <button onClick={onIncreaseFontSize} style={{ fontSize: '12px', padding: '2px 7px', border: '1px solid var(--color-border-default)', borderRadius: '4px', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>A+</button>
          </div>

          {onPrint && (
            <button
              onClick={() => {
                onPrint();
                setShowMoreMenu(false);
              }}
              style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
              }}
            >{t('print')}</button>
          )}

          {onToggleRead && (
            <button
              onClick={() => {
                onToggleRead();
                setShowMoreMenu(false);
              }}
              style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
              }}
            >{isRead ? t('markUnread') : t('markRead')}</button>
          )}

          {onToggleThreadMute && (
            <button
              onClick={() => {
                onToggleThreadMute();
                setShowMoreMenu(false);
              }}
              style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }}
              onMouseEnter={(e) => {
                (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
              }}
              onMouseLeave={(e) => {
                (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
              }}
            >{isThreadMuted ? t('unmuteThread') : t('muteThread')}</button>
          )}

          {(onSpam || onNotSpam || onRestore) && <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '4px 0' }} />}
          {onSpam && <button onClick={() => { onSpam(); setShowMoreMenu(false); }} style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', cursor: 'pointer' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>{t('reportSpam')}</button>}
          {onNotSpam && <button onClick={() => { onNotSpam(); setShowMoreMenu(false); }} style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>{t('notSpam')}</button>}
          {onRestore && <button onClick={() => { onRestore(); setShowMoreMenu(false); }} style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer' }} onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }} onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}>{t('restore')}</button>}

          {unsubscribeUrl && (
            <>
              <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '4px 0' }} />
              <button
                onClick={() => {
                  if (window.confirm(t('unsubscribeConfirm'))) {
                    window.open(unsubscribeUrl, '_blank', 'noopener,noreferrer');
                  }
                  setShowMoreMenu(false);
                }}
                style={{ display: 'block', width: '100%', textAlign: 'left', padding: '7px 14px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', cursor: 'pointer' }}
                onMouseEnter={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
                }}
                onMouseLeave={(e) => {
                  (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
                }}
              >{t('unsubscribe')}</button>
            </>
          )}
        </div>
      )}
    </div>
  );
}
