'use client';

import { useTranslations } from 'next-intl';
import { Row, SectionCard, SectionHeader, Segment, Toggle } from '@/components/settings-view/settingsViewPrimitives';

interface SettingsNotificationsSectionProps {
  notifPerm: NotificationPermission;
  notifSyncError: string;
  onRequestNotif: () => void;
  notifSound: boolean;
  setNotifSound: (value: boolean) => void;
  notifDetail: 'sender' | 'subject' | 'preview';
  setNotifDetail: (value: 'sender' | 'subject' | 'preview') => void;
  dndEnabled: boolean;
  setDndEnabled: (value: boolean) => void;
  dndStart: string;
  setDndStart: (value: string) => void;
  dndEnd: string;
  setDndEnd: (value: string) => void;
}

export function SettingsNotificationsSection({
  notifPerm,
  notifSyncError,
  onRequestNotif,
  notifSound,
  setNotifSound,
  notifDetail,
  setNotifDetail,
  dndEnabled,
  setDndEnabled,
  dndStart,
  setDndStart,
  dndEnd,
  setDndEnd,
}: SettingsNotificationsSectionProps) {
  const t = useTranslations();
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
      {!dndEnabled && <div style={{ height: '1px' }} />}
    </SectionCard>
  );
}
