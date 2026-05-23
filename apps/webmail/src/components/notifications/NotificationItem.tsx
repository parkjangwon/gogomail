'use client';

import { useMemo, useState } from 'react';
import { useLocale, useTranslations } from 'next-intl';
import { useRouter } from 'next/navigation';
import { XMarkIcon } from '@heroicons/react/24/outline';
import type { Notification } from '@/lib/notifications/types';
import { formatRelativeTime } from '@/lib/notifications/relativeTime';
import { CategoryIcon } from './categoryIcon';

interface NotificationItemProps {
  notification: Notification;
  onRead: (id: string) => void;
  onDismiss: (id: string) => void;
  onAfterNavigate?: () => void;
}

export function NotificationItem({ notification, onRead, onDismiss, onAfterNavigate }: NotificationItemProps) {
  const router = useRouter();
  const locale = useLocale();
  const t = useTranslations('notifications');
  const [hover, setHover] = useState(false);

  const relativeTime = useMemo(
    () =>
      formatRelativeTime(notification.timestamp, locale, (key, vars) =>
        t(`timeAgo.${key}`, vars as Record<string, string | number> | undefined),
      ),
    [notification.timestamp, locale, t],
  );

  const handleClick = () => {
    onRead(notification.id);
    if (notification.actionUrl) {
      try {
        router.push(notification.actionUrl);
      } catch {
        if (typeof window !== 'undefined') window.location.href = notification.actionUrl;
      }
      onAfterNavigate?.();
    }
  };

  const handleDismiss = (e: React.MouseEvent) => {
    e.stopPropagation();
    onDismiss(notification.id);
  };

  return (
    <div
      role="group"
      aria-label={notification.title}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
      style={{
        position: 'relative',
        display: 'flex',
        alignItems: 'flex-start',
        gap: '10px',
        width: '100%',
        textAlign: 'left',
        padding: '10px 12px 10px 18px',
        border: 'none',
        borderBottom: '1px solid var(--color-border-subtle)',
        background: hover
          ? 'var(--color-bg-tertiary)'
          : notification.read
            ? 'transparent'
            : 'var(--color-accent-subtle, rgba(99,102,241,0.06))',
        transition: 'background 100ms ease',
        color: 'var(--color-text-primary)',
      }}
    >
      {!notification.read && (
        <span
          aria-hidden="true"
          style={{
            position: 'absolute',
            left: 6,
            top: 16,
            width: 6,
            height: 6,
            borderRadius: '50%',
            background: 'var(--color-accent)',
          }}
        />
      )}
      <button
        type="button"
        onClick={handleClick}
        aria-label={notification.title}
        style={{
          border: 'none',
          background: 'transparent',
          color: 'inherit',
          cursor: 'pointer',
          display: 'flex',
          alignItems: 'flex-start',
          gap: '10px',
          flex: 1,
          minWidth: 0,
          padding: 0,
          textAlign: 'left',
        }}
      >
        <div style={{ flexShrink: 0, marginTop: '2px' }}>
          <CategoryIcon category={notification.category} severity={notification.severity} />
        </div>
        <div style={{ flex: 1, minWidth: 0 }}>
          <div
            style={{
              fontSize: '13px',
              fontWeight: notification.read ? 500 : 700,
              color: 'var(--color-text-primary)',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {notification.title}
          </div>
          {notification.body && (
            <div
              style={{
                marginTop: '2px',
                fontSize: '12px',
                color: 'var(--color-text-secondary)',
                display: '-webkit-box',
                WebkitLineClamp: 2,
                WebkitBoxOrient: 'vertical',
                overflow: 'hidden',
                lineHeight: 1.4,
              }}
            >
              {notification.body}
            </div>
          )}
          <div style={{ marginTop: '4px', fontSize: '11px', color: 'var(--color-text-tertiary)' }}>{relativeTime}</div>
        </div>
      </button>
      <button
        type="button"
        aria-label={t('dismissItem', { title: notification.title })}
        onClick={handleDismiss}
        style={{
          border: 'none',
          background: 'transparent',
          flexShrink: 0,
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          width: 22,
          height: 22,
          borderRadius: 6,
          color: 'var(--color-text-tertiary)',
          opacity: hover ? 1 : 0.6,
          cursor: 'pointer',
        }}
      >
        <XMarkIcon style={{ width: 14, height: 14 }} />
      </button>
    </div>
  );
}
