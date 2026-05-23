'use client';

import { useTranslations } from 'next-intl';
import { Row, SectionCard, SectionHeader, Segment, Toggle } from '@/components/settings-view/settingsViewPrimitives';
import type { Folder, FolderNotificationOverride } from '@/lib/api';

interface SettingsNotificationsSectionProps {
  notifPerm: NotificationPermission;
  notifSyncError: string;
  onRequestNotif: () => void;
  browserNotificationsEnabled: boolean;
  setBrowserNotificationsEnabled: (value: boolean) => void;
  notifSound: boolean;
  setNotifSound: (value: boolean) => void;
  notifDetail: 'sender' | 'subject' | 'preview';
  setNotifDetail: (value: 'sender' | 'subject' | 'preview') => void;
  badgeCountMode: 'unread' | 'all' | 'none';
  setBadgeCountMode: (value: 'unread' | 'all' | 'none') => void;
  dndEnabled: boolean;
  setDndEnabled: (value: boolean) => void;
  dndStart: string;
  setDndStart: (value: string) => void;
  dndEnd: string;
  setDndEnd: (value: string) => void;
  folders: Folder[];
  folderOverrides: Record<string, FolderNotificationOverride>;
  setFolderNotificationEnabled: (folderId: string, enabled: boolean) => void;
  webPushEnabled: boolean;
  setWebPushEnabled: (value: boolean) => void;
  webPushSupported: boolean;
}

export function SettingsNotificationsSection({
  notifPerm,
  notifSyncError,
  onRequestNotif,
  browserNotificationsEnabled,
  setBrowserNotificationsEnabled,
  notifSound,
  setNotifSound,
  notifDetail,
  setNotifDetail,
  badgeCountMode,
  setBadgeCountMode,
  dndEnabled,
  setDndEnabled,
  dndStart,
  setDndStart,
  dndEnd,
  setDndEnd,
  folders,
  folderOverrides,
  setFolderNotificationEnabled,
  webPushEnabled,
  setWebPushEnabled,
  webPushSupported,
}: SettingsNotificationsSectionProps) {
  const t = useTranslations();
  const visibleFolders = folders.filter((folder) => folder.type !== 'virtual');
  return (
    <SectionCard>
      <SectionHeader>{t('misc.settingsNotif.section')}</SectionHeader>
      <Row label={t('misc.settingsNotif.browserLabel')} description={notifPerm === 'granted' ? t('misc.settingsNotif.browserGranted') : notifPerm === 'denied' ? t('misc.settingsNotif.browserDenied') : t('misc.settingsNotif.browserPrompt')}>
        {notifPerm === 'granted'
          ? <span style={{ fontSize: '12px', color: 'var(--color-success, #22c55e)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}><span>{t('misc.settingsNotif.granted')}</span></span>
          : notifPerm === 'denied'
          ? <span style={{ fontSize: '12px', color: 'var(--color-destructive)', fontWeight: 500 }}>{t('misc.settingsNotif.denied')}</span>
          : <button onClick={onRequestNotif} style={{ padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '12px', cursor: 'pointer' }}>{t('misc.settingsNotif.allow')}</button>
        }
      </Row>
      {notifSyncError && (
        <div role="alert" style={{ margin: '0 0 12px', color: 'var(--color-destructive)', fontSize: '12px' }}>
          {notifSyncError}
        </div>
      )}
      <Row label={t('misc.settingsNotif.desktopLabel')} description={notifPerm === 'denied' ? t('misc.settingsNotif.desktopDeniedDesc') : t('misc.settingsNotif.desktopDesc')}>
        <Toggle value={browserNotificationsEnabled} onChange={setBrowserNotificationsEnabled} ariaLabel={t('misc.settingsNotif.desktopLabel')} />
      </Row>
      <Row
        label={t('misc.settingsNotif.pushLabel')}
        description={
          !webPushSupported
            ? t('misc.settingsNotif.pushUnsupported')
            : notifPerm === 'denied'
            ? t('misc.settingsNotif.desktopDeniedDesc')
            : t('misc.settingsNotif.pushDesc')
        }
      >
        {webPushSupported
          ? <Toggle
              value={webPushEnabled}
              onChange={setWebPushEnabled}
              ariaLabel={t('misc.settingsNotif.pushLabel')}
            />
          : <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>
              {t('misc.settingsNotif.pushUnsupported')}
            </span>
        }
      </Row>
      <Row label={t('misc.settingsNotif.soundLabel')} description={t('misc.settingsNotif.soundDesc')}>
        <Toggle value={notifSound} onChange={setNotifSound} ariaLabel={t('misc.settingsNotif.soundLabel')} />
      </Row>
      <Row label={t('misc.settingsNotif.detailLabel')} description={t('misc.settingsNotif.detailDesc')}>
        <Segment
          options={[{ value: 'sender' as const, label: t('misc.settingsNotif.detailSender') }, { value: 'subject' as const, label: t('misc.settingsNotif.detailSubject') }, { value: 'preview' as const, label: t('misc.settingsNotif.detailPreview') }]}
          value={notifDetail}
          onChange={setNotifDetail}
        />
      </Row>
      <Row label={t('misc.settingsNotif.badgeLabel')} description={t('misc.settingsNotif.badgeDesc')}>
        <Segment
          options={[
            { value: 'unread' as const, label: t('misc.settingsNotif.badgeUnread') },
            { value: 'all' as const, label: t('misc.settingsNotif.badgeAll') },
            { value: 'none' as const, label: t('misc.settingsNotif.badgeNone') },
          ]}
          value={badgeCountMode}
          onChange={setBadgeCountMode}
        />
      </Row>
      <Row label={t('misc.settingsNotif.dndLabel')} description={t('misc.settingsNotif.dndDesc')}>
        <Toggle value={dndEnabled} onChange={setDndEnabled} ariaLabel={t('misc.settingsNotif.dndLabel')} />
      </Row>
      {dndEnabled && (
        <Row label={t('misc.settingsNotif.dndTimeLabel')} description={t('misc.settingsNotif.dndTimeDesc')} last>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <input type="time" value={dndStart} onChange={(e) => setDndStart(e.target.value)}
              style={{ padding: '4px 8px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '13px' }} />
            <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>~</span>
            <input type="time" value={dndEnd} onChange={(e) => setDndEnd(e.target.value)}
              style={{ padding: '4px 8px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '13px' }} />
          </div>
        </Row>
      )}
      <SectionHeader>{t('misc.settingsNotif.folderSection')}</SectionHeader>
      {visibleFolders.length === 0 ? (
        <div style={{ padding: '14px 20px', fontSize: '12px', color: 'var(--color-text-tertiary)', background: 'var(--color-bg-primary)' }}>
          {t('misc.settingsNotif.folderEmpty')}
        </div>
      ) : visibleFolders.map((folder, index) => {
        const enabled = folderOverrides[folder.id]?.enabled !== false;
        return (
          <Row
            key={folder.id}
            label={folder.name}
            description={t('misc.settingsNotif.folderDesc')}
            last={index === visibleFolders.length - 1}
          >
            <Toggle
              value={enabled}
              onChange={(next) => setFolderNotificationEnabled(folder.id, next)}
              ariaLabel={t('misc.settingsNotif.folderToggleAria', { name: folder.name })}
            />
          </Row>
        );
      })}
    </SectionCard>
  );
}
