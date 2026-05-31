import { useState, useEffect, useRef } from 'react';
import { registerWebPushDevice, getNotificationPreferences, setNotificationPreferences, getFolders, type NotificationPreferences, type FolderNotificationOverride, type Folder } from '@/lib/api';
import { ignoreNonCritical } from '@/lib/promise';
import { webPushPublicKeyToUint8Array } from '@/lib/webpush';

const NOTIFICATION_FOLDER_OVERRIDES_KEY = 'webmail_notification_folder_overrides';
const BADGE_COUNT_MODE_KEY = 'webmail_badge_count_mode';
const BROWSER_NOTIF_ENABLED_KEY = 'webmail_browser_notifications_enabled';
type BadgeCountMode = 'unread' | 'all' | 'none';

function currentTimeZone(): string {
  try {
    return Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC';
  } catch {
    return 'UTC';
  }
}

function quietHoursPreferences(
  base: NotificationPreferences | null,
  folderOverrides: Record<string, FolderNotificationOverride>,
  enabled: boolean,
  start: string,
  end: string,
): NotificationPreferences {
  return {
    global_dnd_enabled: enabled,
    global_dnd_schedule: {
      weekdays: enabled ? [0, 1, 2, 3, 4, 5, 6] : [],
      time_ranges: enabled ? [{ start, end }] : [],
      timezone: base?.global_dnd_schedule?.timezone || currentTimeZone(),
    },
    folder_overrides: folderOverrides ?? base?.folder_overrides ?? {},
    thread_overrides: base?.thread_overrides ?? {},
  };
}

function emptyDNDSchedule() {
  return { weekdays: [], time_ranges: [], timezone: '' };
}

export interface UseSettingsNotificationsParams {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
}

// eslint-disable-next-line @typescript-eslint/explicit-module-boundary-types
export function useSettingsNotifications({ t }: UseSettingsNotificationsParams) {
  const [notifPerm, setNotifPerm] = useState<NotificationPermission>('default');
  const [notifSyncError, setNotifSyncError] = useState('');
  const [browserNotificationsEnabled, setBrowserNotificationsEnabled] = useState(true);
  const [notifSound, setNotifSound] = useState(false);
  const [notifDetail, setNotifDetail] = useState<'sender' | 'subject' | 'preview'>('subject');
  const [badgeCountMode, setBadgeCountMode] = useState<BadgeCountMode>('unread');
  const [dndEnabled, setDndEnabled] = useState(false);
  const [dndStart, setDndStart] = useState('22:00');
  const [dndEnd, setDndEnd] = useState('08:00');
  const [webPushEnabled, setWebPushEnabled] = useState<boolean>(() => {
    try { return localStorage.getItem('webmail_webpush_enabled') === 'true'; } catch { return false; }
  });
  const [webPushSupported] = useState<boolean>(() => {
    if (typeof window === 'undefined') return false;
    return 'serviceWorker' in navigator && 'PushManager' in window;
  });
  const [notificationPrefsLoaded, setNotificationPrefsLoaded] = useState(false);
  const notificationPrefsBaseRef = useRef<NotificationPreferences | null>(null);
  const skipNotificationPrefsInitialSaveRef = useRef(true);
  const [notificationFolderOverrides, setNotificationFolderOverrides] = useState<Record<string, FolderNotificationOverride>>({});
  const [notificationFolders, setNotificationFolders] = useState<Folder[]>([]);

  useEffect(() => {
    getFolders()
      .then((data) => {
        const folders = data.folders ?? [];
        const seen = new Set<string>();
        setNotificationFolders(folders.filter((f) => { if (seen.has(f.id)) return false; seen.add(f.id); return true; }));
      })
      .catch(() => setNotificationFolders([]));
  }, []);

  useEffect(() => {
    let cancelled = false;
    getNotificationPreferences()
      .then((prefs) => {
        if (cancelled) return;
        notificationPrefsBaseRef.current = prefs;
        setNotificationFolderOverrides(prefs.folder_overrides ?? {});
        setDndEnabled(prefs.global_dnd_enabled);
        const firstRange = prefs.global_dnd_schedule?.time_ranges?.[0];
        if (firstRange?.start) setDndStart(firstRange.start);
        if (firstRange?.end) setDndEnd(firstRange.end);
        try {
          localStorage.setItem('webmail_dnd', prefs.global_dnd_enabled ? '1' : '0');
          localStorage.setItem(NOTIFICATION_FOLDER_OVERRIDES_KEY, JSON.stringify(prefs.folder_overrides ?? {}));
          if (firstRange?.start) localStorage.setItem('webmail_dnd_start', firstRange.start);
          if (firstRange?.end) localStorage.setItem('webmail_dnd_end', firstRange.end);
        } catch {
          // local settings cache is best-effort
        }
      })
      .catch(() => {
        // Older backends may not expose server-side notification preferences.
      })
      .finally(() => {
        if (!cancelled) setNotificationPrefsLoaded(true);
      });
    return () => { cancelled = true; };
  }, []);

  useEffect(() => {
    if (!notificationPrefsLoaded) return;
    if (skipNotificationPrefsInitialSaveRef.current) {
      skipNotificationPrefsInitialSaveRef.current = false;
      return;
    }
    const timer = setTimeout(() => {
      const next = quietHoursPreferences(notificationPrefsBaseRef.current, notificationFolderOverrides, dndEnabled, dndStart, dndEnd);
      ignoreNonCritical(setNotificationPreferences(next)
        .then((saved) => {
          notificationPrefsBaseRef.current = saved;
          try {
            localStorage.setItem(NOTIFICATION_FOLDER_OVERRIDES_KEY, JSON.stringify(saved.folder_overrides ?? {}));
          } catch {
            // local settings cache is best-effort
          }
        }), 'settings.notifications.savePreferences');
    }, 800);
    return () => clearTimeout(timer);
  }, [notificationPrefsLoaded, notificationFolderOverrides, dndEnabled, dndStart, dndEnd]);

  async function requestNotif() {
    if (typeof Notification === 'undefined') return;
    const p = await Notification.requestPermission();
    setNotifPerm(p);
    setNotifSyncError('');
    if (p === 'granted') {
      setBrowserNotificationsEnabled(true);
      try {
        localStorage.setItem(BROWSER_NOTIF_ENABLED_KEY, 'true');
        window.dispatchEvent(new StorageEvent('storage', { key: BROWSER_NOTIF_ENABLED_KEY, newValue: 'true' }));
      } catch {
        // local settings cache is best-effort
      }
    }
    if (p === 'granted' && 'serviceWorker' in navigator && 'PushManager' in window) {
      try {
        const reg = await navigator.serviceWorker.register('/sw.js');
        const vapidKey = process.env.NEXT_PUBLIC_VAPID_PUBLIC_KEY;
        if (vapidKey) {
          const sub = await reg.pushManager.subscribe({
            userVisibleOnly: true,
            applicationServerKey: webPushPublicKeyToUint8Array(vapidKey),
          });
          await registerWebPushDevice(sub);
        }
      } catch {
        setNotifSyncError(t('pushRegisterFailed'));
      }
    }
  }

  function setFolderNotificationEnabled(folderId: string, enabled: boolean) {
    setNotificationFolderOverrides((prev) => {
      const next = { ...prev };
      if (enabled) {
        delete next[folderId];
      } else {
        next[folderId] = { enabled: false, dnd_inherit: true, dnd_schedule: emptyDNDSchedule() };
      }
      try {
        localStorage.setItem(NOTIFICATION_FOLDER_OVERRIDES_KEY, JSON.stringify(next));
      } catch { /* */ }
      return next;
    });
  }

  function setBrowserNotificationsEnabledWithStorage(enabled: boolean) {
    setBrowserNotificationsEnabled(enabled);
    try {
      localStorage.setItem(BROWSER_NOTIF_ENABLED_KEY, enabled ? 'true' : 'false');
      window.dispatchEvent(new StorageEvent('storage', { key: BROWSER_NOTIF_ENABLED_KEY, newValue: enabled ? 'true' : 'false' }));
    } catch { /* */ }
  }

  function setNotifSoundWithStorage(v: boolean) {
    setNotifSound(v);
    try { localStorage.setItem('webmail_notif_sound', v ? '1' : '0'); } catch { /* */ }
  }

  function setNotifDetailWithStorage(v: 'sender' | 'subject' | 'preview') {
    setNotifDetail(v);
    try { localStorage.setItem('webmail_notif_detail', v); } catch { /* */ }
  }

  function setBadgeCountModeWithStorage(mode: BadgeCountMode) {
    setBadgeCountMode(mode);
    try {
      localStorage.setItem(BADGE_COUNT_MODE_KEY, mode);
      window.dispatchEvent(new StorageEvent('storage', { key: BADGE_COUNT_MODE_KEY, newValue: mode }));
    } catch { /* */ }
  }

  function setDndEnabledWithStorage(v: boolean) {
    setDndEnabled(v);
    try { localStorage.setItem('webmail_dnd', v ? '1' : '0'); } catch { /* */ }
  }

  function setDndStartWithStorage(v: string) {
    setDndStart(v);
    try { localStorage.setItem('webmail_dnd_start', v); } catch { /* */ }
  }

  function setDndEndWithStorage(v: string) {
    setDndEnd(v);
    try { localStorage.setItem('webmail_dnd_end', v); } catch { /* */ }
  }

  function setWebPushEnabledWithStorage(v: boolean) {
    setWebPushEnabled(v);
    try { localStorage.setItem('webmail_webpush_enabled', v ? 'true' : 'false'); } catch { /* */ }
  }

  return {
    // Notifications state
    notifPerm, setNotifPerm,
    notifSyncError, setNotifSyncError,
    browserNotificationsEnabled, setBrowserNotificationsEnabled,
    notifSound, setNotifSound,
    notifDetail, setNotifDetail,
    badgeCountMode, setBadgeCountMode,
    dndEnabled, setDndEnabled,
    dndStart, setDndStart,
    dndEnd, setDndEnd,
    webPushEnabled, setWebPushEnabled,
    webPushSupported,
    notificationPrefsLoaded,
    notificationFolderOverrides, setNotificationFolderOverrides,
    notificationFolders, setNotificationFolders,
    // Handlers
    requestNotif,
    setFolderNotificationEnabled,
    setBrowserNotificationsEnabledWithStorage,
    setNotifSoundWithStorage,
    setNotifDetailWithStorage,
    setBadgeCountModeWithStorage,
    setDndEnabledWithStorage,
    setDndStartWithStorage,
    setDndEndWithStorage,
    setWebPushEnabledWithStorage,
  };
}
