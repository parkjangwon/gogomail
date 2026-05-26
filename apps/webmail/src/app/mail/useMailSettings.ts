'use client';

import { useState, useEffect } from 'react';
import { type RefreshIntervalSeconds } from '@/hooks/useMailList';
import { type SectionId } from '@/components/settings-view/settingsViewConfig';
import {
  BADGE_COUNT_MODE_KEY,
  REFRESH_INTERVAL_KEY,
  readBadgeCountMode,
  readRefreshIntervalSeconds,
  type BadgeCountMode,
} from './mailPageHelpers';
import { type ThreadNotificationOverride } from '@/lib/api';

export function useMailSettings() {
  const [badgeCountMode, setBadgeCountMode] = useState<BadgeCountMode>(readBadgeCountMode);
  const [refreshIntervalSeconds, setRefreshIntervalSeconds] = useState<RefreshIntervalSeconds>(readRefreshIntervalSeconds);
  const [threadNotificationOverrides, setThreadNotificationOverrides] = useState<Record<string, ThreadNotificationOverride>>({});
  const [settingsInitialSection, setSettingsInitialSection] = useState<SectionId | undefined>(undefined);

  const [wmSettings, setWmSettings] = useState<{ showPreview: boolean; externalImages: string }>(() => {
    try {
      const s = JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as Record<string, unknown>;
      return { showPreview: s.showPreview !== false, externalImages: (s.externalImages as string) ?? 'ask' };
    } catch { return { showPreview: true, externalImages: 'ask' }; }
  });

  useEffect(() => {
    function onStorage(e: StorageEvent) {
      if (e.key !== 'webmail_settings') return;
      try {
        const s = JSON.parse(e.newValue ?? '{}') as Record<string, unknown>;
        setWmSettings({ showPreview: s.showPreview !== false, externalImages: (s.externalImages as string) ?? 'ask' });
      } catch { /* */ }
    }
    window.addEventListener('storage', onStorage);
    return () => window.removeEventListener('storage', onStorage);
  }, []);

  useEffect(() => {
    const onBadgeModeChange = (event: StorageEvent) => {
      if (event.key === BADGE_COUNT_MODE_KEY) setBadgeCountMode(readBadgeCountMode());
    };
    window.addEventListener('storage', onBadgeModeChange);
    return () => window.removeEventListener('storage', onBadgeModeChange);
  }, []);

  useEffect(() => {
    const onRefreshIntervalChange = (event: StorageEvent) => {
      if (event.key === REFRESH_INTERVAL_KEY) setRefreshIntervalSeconds(readRefreshIntervalSeconds());
    };
    window.addEventListener('storage', onRefreshIntervalChange);
    return () => window.removeEventListener('storage', onRefreshIntervalChange);
  }, []);

  return {
    badgeCountMode,
    setBadgeCountMode,
    refreshIntervalSeconds,
    setRefreshIntervalSeconds,
    threadNotificationOverrides,
    setThreadNotificationOverrides,
    wmSettings,
    setWmSettings,
    settingsInitialSection,
    setSettingsInitialSection,
  };
}
