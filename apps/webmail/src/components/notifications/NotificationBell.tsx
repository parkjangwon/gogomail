'use client';

import { useCallback, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { BellIcon } from '@heroicons/react/24/outline';
import { BellIcon as BellIconSolid } from '@heroicons/react/24/solid';
import { useNotifications } from '@/lib/notifications/store';
import { NotificationCenter } from './NotificationCenter';

export function NotificationBell() {
  const t = useTranslations('notifications');
  const { unreadCount } = useNotifications();
  const [open, setOpen] = useState(false);
  const buttonRef = useRef<HTMLButtonElement | null>(null);
  const badgeLabel = unreadCount > 0 ? (unreadCount > 99 ? '99+' : String(unreadCount)) : '';
  const closeCenter = useCallback((options?: { restoreFocus?: boolean }) => {
    setOpen(false);
    if (options?.restoreFocus) {
      window.setTimeout(() => buttonRef.current?.focus(), 0);
    }
  }, []);

  return (
    <>
      <button
        type="button"
        ref={buttonRef}
        data-notification-trigger
        aria-label={t('openCenter')}
        title={t('openCenter')}
        aria-expanded={open}
        onClick={() => setOpen((v) => !v)}
        onMouseDown={(e) => {
          e.currentTarget.blur();
        }}
        style={{
          width: 36,
          height: 36,
          borderRadius: 8,
          border: 'none',
          background: open ? 'var(--color-accent-subtle)' : 'transparent',
          color: open ? 'var(--color-accent)' : 'var(--color-text-tertiary)',
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          transition: 'background 100ms ease, color 100ms ease',
          position: 'relative',
        }}
        onMouseEnter={(e) => {
          if (!open) {
            e.currentTarget.style.background = 'var(--color-bg-tertiary)';
            e.currentTarget.style.color = 'var(--color-text-secondary)';
          }
        }}
        onMouseLeave={(e) => {
          if (!open) {
            e.currentTarget.style.background = 'transparent';
            e.currentTarget.style.color = 'var(--color-text-tertiary)';
          }
        }}
      >
        {open ? (
          <BellIconSolid style={{ width: 20, height: 20 }} />
        ) : (
          <BellIcon style={{ width: 20, height: 20 }} />
        )}
        {badgeLabel && (
          <span
            aria-hidden="true"
            style={{
              position: 'absolute',
              top: 2,
              right: 2,
              minWidth: 14,
              height: 14,
              borderRadius: 7,
              background: 'var(--color-destructive, #dc2626)',
              color: '#fff',
              fontSize: 9,
              fontWeight: 700,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: '0 3px',
              lineHeight: 1,
              pointerEvents: 'none',
            }}
          >
            {badgeLabel}
          </span>
        )}
      </button>
      <NotificationCenter open={open} onClose={closeCenter} />
    </>
  );
}
