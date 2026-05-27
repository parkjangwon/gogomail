'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useNotifications } from '@/lib/notifications/store';
import { useBrowserNotifications } from '@/lib/notifications/browser';
import type { NotificationCategory } from '@/lib/notifications/types';

export type FilterMode = 'all' | 'unread';
export type CategoryFilter = 'all' | NotificationCategory;

const BANNER_DISMISSED_KEY = 'webmail_browser_banner_dismissed';

interface UseNotificationCenterParams {
  open: boolean;
  onClose: (options?: { restoreFocus?: boolean }) => void;
}

function categoryOrder(category: NotificationCategory): number {
  const order: NotificationCategory[] = [
    'mail_received',
    'mail_sent',
    'mail_send_failed',
    'mail_bounced',
    'calendar_reminder',
    'calendar_invite',
    'drive_share',
    'system',
    'custom',
  ];
  return order.indexOf(category) === -1 ? order.length : order.indexOf(category);
}

export function useNotificationCenter({ open, onClose }: UseNotificationCenterParams) {
  const { notifications, unreadCount, markAsRead, markAllRead, dismiss, clearAll } = useNotifications();
  const browser = useBrowserNotifications();
  const [filter, setFilter] = useState<FilterMode>('all');
  const [categoryFilter, setCategoryFilter] = useState<CategoryFilter>('all');
  const [query, setQuery] = useState('');
  const [bannerDismissed, setBannerDismissed] = useState<boolean>(false);
  const panelRef = useRef<HTMLDivElement | null>(null);
  const searchRef = useRef<HTMLInputElement | null>(null);

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
        onClose({ restoreFocus: true });
      }
    };
    document.addEventListener('keydown', onKey);
    return () => document.removeEventListener('keydown', onKey);
  }, [open, onClose]);

  // Reset state and focus search on open
  useEffect(() => {
    if (!open) return;
    setQuery('');
    setFilter('all');
    setCategoryFilter('all');
    const id = window.setTimeout(() => searchRef.current?.focus(), 0);
    return () => window.clearTimeout(id);
  }, [open]);

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
    const q = query.trim().toLocaleLowerCase();
    return notifications.filter((n) => {
      if (filter === 'unread' && n.read) return false;
      if (categoryFilter !== 'all' && n.category !== categoryFilter) return false;
      if (!q) return true;
      return `${n.title} ${n.body ?? ''}`.toLocaleLowerCase().includes(q);
    });
  }, [notifications, filter, categoryFilter, query]);

  const categoryCounts = useMemo(() => {
    const q = query.trim().toLocaleLowerCase();
    const counts = new Map<NotificationCategory, number>();
    notifications
      .filter((n) => filter === 'all' || !n.read)
      .filter((n) => !q || `${n.title} ${n.body ?? ''}`.toLocaleLowerCase().includes(q))
      .forEach((n) => counts.set(n.category, (counts.get(n.category) ?? 0) + 1));
    return Array.from(counts.entries()).sort(([a], [b]) => categoryOrder(a) - categoryOrder(b));
  }, [filter, notifications, query]);

  const hasActiveFilter = filter !== 'all' || categoryFilter !== 'all' || query.trim() !== '';
  const visibleUnread = useMemo(() => visible.filter((n) => !n.read), [visible]);
  const markReadActionDisabled = hasActiveFilter ? visibleUnread.length === 0 : unreadCount === 0;
  const clearActionDisabled = hasActiveFilter
    ? visible.length === 0
    : notifications.length === 0;

  const handleMarkAllRead = useCallback(() => {
    if (!hasActiveFilter) {
      markAllRead();
      setFilter('all');
      return;
    }

    visibleUnread.forEach((n) => markAsRead(n.id));
    if (filter === 'unread' && unreadCount === visibleUnread.length) {
      setFilter('all');
    }
  }, [filter, hasActiveFilter, markAllRead, markAsRead, unreadCount, visibleUnread]);

  const handleClear = useCallback(() => {
    if (!hasActiveFilter) {
      clearAll();
      return;
    }
    visible.forEach((n) => dismiss(n.id));
    if (notifications.length > visible.length) {
      setQuery('');
      setFilter('all');
      setCategoryFilter('all');
    }
  }, [clearAll, dismiss, hasActiveFilter, notifications.length, visible]);

  // Auto-reset category filter when it disappears from available options
  useEffect(() => {
    if (categoryFilter === 'all') return;
    if (categoryCounts.some(([category]) => category === categoryFilter)) return;
    setCategoryFilter('all');
  }, [categoryCounts, categoryFilter]);

  // Auto-reset unread filter when no unread notifications remain
  useEffect(() => {
    if (filter !== 'unread') return;
    if (unreadCount > 0) return;
    if (notifications.length === 0) return;
    setFilter('all');
  }, [filter, notifications.length, unreadCount]);

  return {
    filter,
    setFilter,
    categoryFilter,
    setCategoryFilter,
    query,
    setQuery,
    bannerDismissed,
    panelRef,
    searchRef,
    dismissBanner,
    onEnableBrowser,
    visible,
    categoryCounts,
    hasActiveFilter,
    visibleUnread,
    markReadActionDisabled,
    clearActionDisabled,
    handleMarkAllRead,
    handleClear,
    browser,
    notifications,
    unreadCount,
    markAsRead,
    dismiss,
  };
}
