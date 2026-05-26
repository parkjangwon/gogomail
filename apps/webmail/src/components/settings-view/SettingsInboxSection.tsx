'use client';

import { useTranslations } from 'next-intl';
import { Row, SectionCard, SectionHeader, Segment, Toggle, saveWmSetting } from './settingsViewPrimitives';

export interface SettingsInboxSectionProps {
  convMode: boolean;
  setConvMode: (v: boolean) => void;
  compact: boolean;
  setCompact: (v: boolean) => void;
  showPreview: boolean;
  setShowPreview: (v: boolean) => void;
  refreshInterval: 30 | 60 | 300;
  setRefreshInterval: (v: 30 | 60 | 300) => void;
  importanceMarkers: boolean;
  setImportanceMarkers: (v: boolean) => void;
  groupByDate: boolean;
  setGroupByDate: (v: boolean) => void;
  focusMode: boolean;
  setFocusMode: (v: boolean) => void;
  swipeLeft: 'archive' | 'delete' | 'snooze' | 'star';
  setSwipeLeft: (v: 'archive' | 'delete' | 'snooze' | 'star') => void;
  swipeRight: 'archive' | 'delete' | 'snooze' | 'star';
  setSwipeRight: (v: 'archive' | 'delete' | 'snooze' | 'star') => void;
}

export function SettingsInboxSection({
  convMode,
  setConvMode,
  compact,
  setCompact,
  showPreview,
  setShowPreview,
  refreshInterval,
  setRefreshInterval,
  importanceMarkers,
  setImportanceMarkers,
  groupByDate,
  setGroupByDate,
  focusMode,
  setFocusMode,
  swipeLeft,
  setSwipeLeft,
  swipeRight,
  setSwipeRight,
}: SettingsInboxSectionProps) {
  const t = useTranslations('settingsView');

  return (
    <SectionCard>
      <SectionHeader>{t('sectionInboxSettings')}</SectionHeader>
      <Row label={t('convMode')} description={t('convModeDesc')}>
        <Toggle value={convMode} onChange={(v) => { setConvMode(v); try { localStorage.setItem('webmail_conv_mode', v ? '1' : '0'); } catch { /* */ } }} />
      </Row>
      <Row label={t('compactView')} description={t('compactViewDesc')}>
        <Toggle value={compact} onChange={(v) => { setCompact(v); try { localStorage.setItem('webmail_compact', v ? '1' : '0'); } catch { /* */ } }} />
      </Row>
      <Row label={t('previewText')} description={t('previewTextDesc')}>
        <Toggle value={showPreview} onChange={(v) => { setShowPreview(v); saveWmSetting('showPreview', v); }} />
      </Row>
      <Row label={t('autoRefresh')} description={t('autoRefreshDesc')}>
        <Segment
          options={[{ value: 30 as 30, label: t('sec30') }, { value: 60 as 60, label: t('min1') }, { value: 300 as 300, label: t('min5') }]}
          value={refreshInterval}
          onChange={(v) => {
            setRefreshInterval(v);
            try {
              localStorage.setItem('webmail_refresh_interval', String(v));
              window.dispatchEvent(new StorageEvent('storage', { key: 'webmail_refresh_interval', newValue: String(v) }));
            } catch { /* */ }
          }}
        />
      </Row>
      <Row label={t('groupByDate')} description={t('groupByDateDesc')}>
        <Toggle value={groupByDate} onChange={(v) => { setGroupByDate(v); try { localStorage.setItem('webmail_group_by_date', v ? '1' : '0'); } catch { /* */ } }} />
      </Row>
      <Row label={t('importanceMarkers')} description={t('importanceMarkersDesc')}>
        <Toggle value={importanceMarkers} onChange={(v) => { setImportanceMarkers(v); try { localStorage.setItem('webmail_importance_markers', v ? '1' : '0'); } catch { /* */ } }} />
      </Row>
      <Row label={t('focusMode')} description={t('focusModeDesc')}>
        <Toggle value={focusMode} onChange={(v) => { setFocusMode(v); try { localStorage.setItem('webmail_focus_mode', v ? '1' : '0'); } catch { /* */ } }} />
      </Row>
      <Row label={t('swipeLeft')} description={t('swipeLeftDesc')}>
        <Segment
          options={[{ value: 'archive' as const, label: t('swipeArchive') }, { value: 'delete' as const, label: t('swipeDelete') }, { value: 'snooze' as const, label: t('swipeSnooze') }, { value: 'star' as const, label: t('swipeStar') }]}
          value={swipeLeft}
          onChange={(v) => { setSwipeLeft(v); try { localStorage.setItem('webmail_swipe_left', v); } catch { /* */ } }}
        />
      </Row>
      <Row label={t('swipeRight')} description={t('swipeRightDesc')} last>
        <Segment
          options={[{ value: 'archive' as const, label: t('swipeArchive') }, { value: 'delete' as const, label: t('swipeDelete') }, { value: 'snooze' as const, label: t('swipeSnooze') }, { value: 'star' as const, label: t('swipeStar') }]}
          value={swipeRight}
          onChange={(v) => { setSwipeRight(v); try { localStorage.setItem('webmail_swipe_right', v); } catch { /* */ } }}
        />
      </Row>
    </SectionCard>
  );
}
