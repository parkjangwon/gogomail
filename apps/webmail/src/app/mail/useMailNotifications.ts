'use client';

import { useEffect, useCallback, useRef } from 'react';
import { MessageSummary, getNotificationPreferences, setNotificationPreferences, type ThreadNotificationOverride } from '@/lib/api';
import { type NotificationInput } from '@/lib/notifications/types';
import { ignoreNonCritical } from '@/lib/promise';
import {
  NOTIFICATION_FOLDER_OVERRIDES_KEY,
  NOTIFICATION_THREAD_OVERRIDES_KEY,
  folderNotificationsEnabled,
  threadNotificationsEnabled,
} from './mailPageHelpers';

interface UseMailNotificationsParams {
  messages: MessageSummary[];
  activeFolderId: string;
  selectedNotificationThreadId: string;
  selectedThreadMuted: boolean;
  threadNotificationOverrides: Record<string, ThreadNotificationOverride>;
  setThreadNotificationOverrides: (v: Record<string, ThreadNotificationOverride>) => void;
  pushNotification: (n: NotificationInput) => void;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  tNotif: (key: string, values?: Record<string, any>) => string;
}

export function useMailNotifications(params: UseMailNotificationsParams) {
  const {
    messages,
    activeFolderId,
    selectedNotificationThreadId,
    selectedThreadMuted,
    threadNotificationOverrides,
    setThreadNotificationOverrides,
    pushNotification,
    t,
    tNotif,
  } = params;

  // Detect new unread messages after refresh and notify
  const seenMsgIdsRef = useRef<Set<string> | null>(null);
  useEffect(() => {
    if (messages.length === 0) return;
    if (seenMsgIdsRef.current === null) {
      seenMsgIdsRef.current = new Set(messages.map((m) => m.id));
      return;
    }
    const newUnread = messages.filter((m) =>
      !m.read &&
      !seenMsgIdsRef.current!.has(m.id) &&
      folderNotificationsEnabled(m.folder_id) &&
      threadNotificationsEnabled(m.thread_id, m.id)
    );
    messages.forEach((m) => seenMsgIdsRef.current!.add(m.id));
    // In-app notification center push is independent of OS-level permission/DnD.
    // Browser mirroring is centralized in the notification store so user toggles,
    // quiet hours, and click handling stay consistent across event sources.
    if (newUnread.length > 0) {
      for (const m of newUnread) {
        const sender = m.from_name || m.from_addr || '';
        let detail: 'sender' | 'subject' | 'preview' = 'subject';
        try {
          const stored = localStorage.getItem('webmail_notif_detail');
          if (stored === 'sender' || stored === 'subject' || stored === 'preview') detail = stored;
        } catch {
          // keep default detail
        }
        const body = detail === 'sender'
          ? undefined
          : ((detail === 'preview' ? m.preview : m.subject) || t('misc.mailPage.noSubject')).slice(0, 120);
        pushNotification({
          id: `mail_received_${m.id}`,
          category: 'mail_received',
          severity: 'info',
          title: tNotif('mailReceived', { sender }),
          body,
          actionUrl: `/mail/${m.id}`,
          metadata: { messageId: m.id },
        });
      }
    }
  }, [messages, pushNotification, tNotif]);

  // Reset seen IDs when folder changes (avoid false notifications on folder switch)
  useEffect(() => { seenMsgIdsRef.current = null; }, [activeFolderId]);

  // Load notification preferences from server and sync to localStorage
  useEffect(() => {
    ignoreNonCritical(getNotificationPreferences()
      .then((prefs) => {
        try {
          const threadOverrides = prefs.thread_overrides ?? {};
          setThreadNotificationOverrides(threadOverrides);
          window.localStorage.setItem(NOTIFICATION_FOLDER_OVERRIDES_KEY, JSON.stringify(prefs.folder_overrides ?? {}));
          window.localStorage.setItem(NOTIFICATION_THREAD_OVERRIDES_KEY, JSON.stringify(threadOverrides));
          window.localStorage.setItem('webmail_dnd', prefs.global_dnd_enabled ? '1' : '0');
          const firstRange = prefs.global_dnd_schedule?.time_ranges?.[0];
          if (firstRange?.start) window.localStorage.setItem('webmail_dnd_start', firstRange.start);
          if (firstRange?.end) window.localStorage.setItem('webmail_dnd_end', firstRange.end);
        } catch {
          // local notification policy cache is best-effort
        }
      }), 'mail.notifications.loadPreferences');
  }, []);

  const handleToggleThreadMute = useCallback(async () => {
    if (!selectedNotificationThreadId) return;
    const nextMuted = !selectedThreadMuted;
    const previous = threadNotificationOverrides;
    const next = { ...previous };
    if (nextMuted) {
      next[selectedNotificationThreadId] = { enabled: false };
    } else {
      delete next[selectedNotificationThreadId];
    }
    setThreadNotificationOverrides(next);
    try {
      window.localStorage.setItem(NOTIFICATION_THREAD_OVERRIDES_KEY, JSON.stringify(next));
      const base = await getNotificationPreferences();
      const saved = await setNotificationPreferences({
        ...base,
        thread_overrides: next,
      });
      const savedThreads = saved.thread_overrides ?? next;
      setThreadNotificationOverrides(savedThreads);
      window.localStorage.setItem(NOTIFICATION_THREAD_OVERRIDES_KEY, JSON.stringify(savedThreads));
    } catch {
      setThreadNotificationOverrides(previous);
      try {
        window.localStorage.setItem(NOTIFICATION_THREAD_OVERRIDES_KEY, JSON.stringify(previous));
      } catch {
        // local notification policy cache is best-effort
      }
    }
  }, [selectedNotificationThreadId, selectedThreadMuted, threadNotificationOverrides]);

  return { handleToggleThreadMute };
}
