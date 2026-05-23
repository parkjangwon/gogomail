'use client';

import { CheckIcon } from '@heroicons/react/24/outline';
import { useTranslations } from 'next-intl';
import { Row, SectionCard, SectionHeader, Segment, Toggle, saveWmSetting } from '@/components/settings-view/settingsViewPrimitives';

interface SettingsPrivacySectionProps {
  blockTrackingPixels: boolean;
  setBlockTrackingPixels: (value: boolean) => void;
  linkPreview: boolean;
  setLinkPreview: (value: boolean) => void;
  requestReadReceipt: boolean;
  setRequestReadReceipt: (value: boolean) => void;
  followUpDays: 0 | 1 | 3 | 7;
  setFollowUpDays: (value: 0 | 1 | 3 | 7) => void;
}

export function SettingsPrivacySection({
  blockTrackingPixels,
  setBlockTrackingPixels,
  linkPreview,
  setLinkPreview,
  requestReadReceipt,
  setRequestReadReceipt,
  followUpDays,
  setFollowUpDays,
}: SettingsPrivacySectionProps) {
  const t = useTranslations();
  return (
    <>
      <SectionCard>
        <SectionHeader>{t('misc.settingsPrivacy.sectionTracking')}</SectionHeader>
        <Row label={t('misc.settingsPrivacy.trackingPixelLabel')} description={t('misc.settingsPrivacy.trackingPixelDesc')}>
          <Toggle value={blockTrackingPixels} onChange={(v) => { setBlockTrackingPixels(v); saveWmSetting('blockTrackingPixels', v); }} />
        </Row>
        <Row label={t('misc.settingsPrivacy.linkPreviewLabel')} description={t('misc.settingsPrivacy.linkPreviewDesc')} last>
          <Toggle value={linkPreview} onChange={(v) => { setLinkPreview(v); saveWmSetting('linkPreview', v); }} />
        </Row>
      </SectionCard>

      <SectionCard>
        <SectionHeader>{t('misc.settingsPrivacy.sectionOutgoing')}</SectionHeader>
        <Row label={t('misc.settingsPrivacy.readReceiptLabel')} description={t('misc.settingsPrivacy.readReceiptDesc')}>
          <Toggle value={requestReadReceipt} onChange={(v) => { setRequestReadReceipt(v); saveWmSetting('requestReadReceipt', v); }} />
        </Row>
        <Row label={t('misc.settingsPrivacy.followUpLabel')} description={t('misc.settingsPrivacy.followUpDesc')} last>
          <Segment<0 | 1 | 3 | 7>
            options={[{ value: 0, label: t('misc.settingsPrivacy.none') }, { value: 1, label: t('misc.settingsPrivacy.day1') }, { value: 3, label: t('misc.settingsPrivacy.day3') }, { value: 7, label: t('misc.settingsPrivacy.week1') }]}
            value={followUpDays}
            onChange={(v) => { setFollowUpDays(v); saveWmSetting('followUpDays', v); }}
          />
        </Row>
      </SectionCard>

      <SectionCard>
        <SectionHeader>{t('misc.settingsPrivacy.sectionData')}</SectionHeader>
        <Row label={t('misc.settingsPrivacy.telemetryLabel')} description={t('misc.settingsPrivacy.telemetryDesc')} last>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', fontSize: '12px', color: '#16a34a', fontWeight: 600 }}>
            <CheckIcon style={{ width: 14, height: 14 }} />
            {t('misc.settingsPrivacy.telemetryStatus')}
          </span>
        </Row>
      </SectionCard>
    </>
  );
}
