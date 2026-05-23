'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import { BellIcon, XMarkIcon } from '@heroicons/react/24/outline';
import { useNotifications } from '@/lib/notifications/store';
import { useBrowserNotifications } from '@/lib/notifications/browser';
import { NotificationItem } from './NotificationItem';

const BANNER_DISMISSED_KEY = 'webmail_browser_banner_dismissed';

type FilterMode = 'all' | 'unread';

interface NotificationCenterProps {
  open: boolean;
  onClose: () => void;
}

export function NotificationCenter({ open, onClose }: NotificationCenterProps) {
  const t = useTranslations('notifications');
  const { notifications, unreadCount, markAsRead, markAllRead, dismiss, clearAll } = useNotifications();
  const browser = useBrowserNotifications();
  const [filter, setFilter] = useState<FilterMode>('all');
  const [bannerDismissed, setBannerDismissed] = useState<boolean>(false);
  const panelRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    try {
      setBannerDismissed(window.localStorage.getItem(BANNER_DISMISSED_KEY) === 'true');
    } catch {
      // ignore
    }
  }, []);

  const dismissBanner = useCallback(() => {
    setBannerDismissed(true);
    if (typeof window === 'undefined') return;
    try {
      window.localStorage.setItem(BANNER_DISMISSED_KEY, 'true');
    } catch {
      // ignore
    }
  }, []);

  const onEnableBrowser = useCallback(async () => {
    await browser.request();
  }, [browser]);

  // Close on Escape
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  // Click outside to close
  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      const el = panelRef.current;
      if (!el) return;
      const target = e.target as Node | null;
      if (target && !el.contains(target)) {
        // Ignore clicks on the bell trigger (it toggles itself)
        const trigger = (target as HTMLElement | null)?.closest?.('[data-notification-trigger]');
        if (trigger) return;
        onClose();
      }
    };
    // delay to avoid catching the click that opened the panel
    const id = window.setTimeout(() => document.addEventListener('mousedown', onDown), 0);
    return () => {
      window.clearTimeout(id);
      document.removeEventListener('mousedown', onDown);
    };
  }, [open, onClose]);

  const visible = useMemo(() => {
    if (filter === 'unread') return notifications.filter((n) => !n.read);
    return notifications;
  }, [notifications, filter]);

  return (
    <div
      ref={panelRef}
      role="dialog"
      aria-modal="false"
      aria-label={t('center.title')}
      style={{
        position: 'fixed',
        top: 0,
        right: 0,
        bottom: 0,
        width: '340px',
        maxWidth: '92vw',
        background: 'var(--color-bg-primary)',
        borderLeft: '1px solid var(--color-border-default)',
        boxShadow: open ? '0 10px 30px rgba(0,0,0,0.16)' : 'none',
        transform: open ? 'translateX(0)' : 'translateX(100%)',
        transition: 'transform 180ms ease',
        zIndex: 7000,
        display: 'flex',
        flexDirection: 'column',
        pointerEvents: open ? 'auto' : 'none',
        visibility: open ? 'visible' : 'hidden',
      }}
    >
      {/* Header */}
      <div
        style={{
          padding: '14px 16px',
          borderBottom: '1px solid var(--color-border-subtle)',
          display: 'flex',
          alignItems: 'center',
          gap: '8px',
        }}
      >
        <h2 style={{ flex: 1, margin: 0, fontSize: '14px', fontWeight: 600, color: 'var(--color-text-primary)' }}>
          {t('center.title')}
        </h2>
        <button
          type="button"
          aria-label="close"
          onClick={onClose}
          style={{
            border: 'none',
            background: 'transparent',
            color: 'var(--color-text-tertiary)',
            cursor: 'pointer',
            padding: 4,
            borderRadius: 6,
            display: 'inline-flex',
            alignItems: 'center',
            justifyContent: 'center',
          }}
        >
          <XMarkIcon style={{ width: 18, height: 18 }} />
        </button>
      </div>

      {/* Browser-permission banner / hint */}
      {browser.permission === 'default' && !bannerDismissed && (
        <div
          style={{
            padding: '12px 14px',
            display: 'flex',
            alignItems: 'flex-start',
            gap: '10px',
            background: 'var(--color-accent-subtle, rgba(99,102,241,0.10))',
            borderBottom: '1px solid var(--color-border-subtle)',
          }}
        >
          <BellIcon style={{ width: 18, height: 18, color: 'var(--color-accent)', flexShrink: 0, marginTop: 2 }} />
          <div style={{ flex: 1, minWidth: 0 }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)', marginBottom: 2 }}>
              {t('browser.enable')}
            </div>
            <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)', marginBottom: 8 }}>
              {t('browser.enableHint')}
            </div>
            <button
              type="button"
              onClick={onEnableBrowser}
              style={{
                border: 'none',
                background: 'var(--color-accent)',
                color: '#fff',
                fontSize: '12px',
                fontWeight: 600,
                padding: '5px 12px',
                borderRadius: 6,
                cursor: 'pointer',
              }}
            >
              {t('browser.enableBtn')}
            </button>
          </div>
          <button
            type="button"
            aria-label={t('browser.dismiss')}
            onClick={dismissBanner}
            style={{
              border: 'none',
              background: 'transparent',
              color: 'var(--color-text-tertiary)',
              cursor: 'pointer',
              padding: 2,
              borderRadius: 4,
              display: 'inline-flex',
            }}
          >
            <XMarkIcon style={{ width: 16, height: 16 }} />
          </button>
        </div>
      )}
      {browser.permission === 'denied' && (
        <div
          style={{
            padding: '8px 14px',
            fontSize: '11px',
            color: 'var(--color-text-tertiary)',
            background: 'var(--color-bg-tertiary, rgba(0,0,0,0.03))',
            borderBottom: '1px solid var(--color-border-subtle)',
          }}
        >
          {t('browser.deniedHint')}
        </div>
      )}

      {/* Filters + actions */}
      <div
        style={{
          padding: '8px 12px',
          display: 'flex',
          alignItems: 'center',
          gap: '6px',
          borderBottom: '1px solid var(--color-border-subtle)',
          flexWrap: 'wrap',
        }}
      >
        <FilterButton
          active={filter === 'all'}
          onClick={() => setFilter('all')}
          label={t('center.filters.all')}
        />
        <FilterButton
          active={filter === 'unread'}
          onClick={() => setFilter('unread')}
          label={`${t('center.filters.unread')}${unreadCount > 0 ? ` (${unreadCount})` : ''}`}
        />
        <div style={{ flex: 1 }} />
        <button
          type="button"
          onClick={markAllRead}
          disabled={unreadCount === 0}
          style={miniButtonStyle(unreadCount === 0)}
        >
          {t('center.markAllRead')}
        </button>
        <button
          type="button"
          onClick={clearAll}
          disabled={notifications.length === 0}
          style={miniButtonStyle(notifications.length === 0)}
        >
          {t('center.clearAll')}
        </button>
      </div>
      {browser.permission === 'granted' && (
        <div
          style={{
            padding: '6px 14px',
            display: 'flex',
            alignItems: 'center',
            gap: '8px',
            borderBottom: '1px solid var(--color-border-subtle)',
            fontSize: '12px',
            color: 'var(--color-text-secondary)',
          }}
        >
          <label style={{ display: 'inline-flex', alignItems: 'center', gap: 8, cursor: 'pointer' }}>
            <input
              type="checkbox"
              checked={browser.enabled}
              onChange={(e) => browser.setEnabled(e.target.checked)}
              style={{ cursor: 'pointer' }}
            />
            <span>{t('browser.toggle')}</span>
          </label>
        </div>
      )}

      {/* List */}
      <div style={{ flex: 1, overflowY: 'auto' }}>
        {visible.length === 0 ? (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '12px',
              padding: '48px 16px',
              color: 'var(--color-text-tertiary)',
              textAlign: 'center',
            }}
          >
            <BellIcon style={{ width: 36, height: 36, opacity: 0.5 }} />
            <div style={{ fontSize: '13px' }}>{t('center.empty')}</div>
          </div>
        ) : (
          visible.map((n) => (
            <NotificationItem
              key={n.id}
              notification={n}
              onRead={markAsRead}
              onDismiss={dismiss}
              onAfterNavigate={onClose}
            />
          ))
        )}
      </div>
    </div>
  );
}

function miniButtonStyle(disabled: boolean): React.CSSProperties {
  return {
    border: 'none',
    background: 'transparent',
    color: disabled ? 'var(--color-text-tertiary)' : 'var(--color-accent)',
    fontSize: '12px',
    cursor: disabled ? 'default' : 'pointer',
    padding: '4px 6px',
    borderRadius: 4,
    opacity: disabled ? 0.5 : 1,
  };
}

function FilterButton({ active, onClick, label }: { active: boolean; onClick: () => void; label: string }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={active}
      style={{
        border: 'none',
        background: active ? 'var(--color-accent-subtle, rgba(99,102,241,0.12))' : 'transparent',
        color: active ? 'var(--color-accent)' : 'var(--color-text-secondary)',
        fontSize: '12px',
        fontWeight: 500,
        cursor: 'pointer',
        padding: '4px 10px',
        borderRadius: 999,
      }}
    >
      {label}
    </button>
  );
}
