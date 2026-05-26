'use client';

import { useTranslations } from 'next-intl';
import { Row, SectionCard, SectionHeader, Segment, Toggle, saveWmSetting } from './settingsViewPrimitives';
import type { ReadMark, ExternalImages } from '@/lib/settings/settingsUtils';

export interface SettingsReadingSectionProps {
  readMark: ReadMark;
  setReadMark: (v: ReadMark) => void;
  externalImages: ExternalImages;
  setExternalImages: (v: ExternalImages) => void;
  inlineImagePreview: boolean;
  setInlineImagePreview: (v: boolean) => void;
  smartReplySuggestions: boolean;
  setSmartReplySuggestions: (v: boolean) => void;
  showReadingTime: boolean;
  setShowReadingTime: (v: boolean) => void;
  readingPanePosition: 'right' | 'bottom' | 'hidden';
  setReadingPanePosition: (v: 'right' | 'bottom' | 'hidden') => void;
}

export function SettingsReadingSection({
  readMark,
  setReadMark,
  externalImages,
  setExternalImages,
  inlineImagePreview,
  setInlineImagePreview,
  smartReplySuggestions,
  setSmartReplySuggestions,
  showReadingTime,
  setShowReadingTime,
  readingPanePosition,
  setReadingPanePosition,
}: SettingsReadingSectionProps) {
  const t = useTranslations('settingsView');

  return (
    <SectionCard>
      <SectionHeader>{t('sectionReadingSettings')}</SectionHeader>
      <Row label={t('readMark')} description={t('readMarkDesc')}>
        <Segment
          options={[{ value: 'instant' as ReadMark, label: t('readMarkInstant') }, { value: '2s' as ReadMark, label: t('readMark2s') }, { value: 'manual' as ReadMark, label: t('readMarkManual') }]}
          value={readMark}
          onChange={(v) => { setReadMark(v); saveWmSetting('readMark', v); }}
        />
      </Row>
      <Row label={t('externalImages')} description={t('externalImagesDesc')}>
        <Segment
          options={[{ value: 'always' as ExternalImages, label: t('externalImagesAlways') }, { value: 'ask' as ExternalImages, label: t('externalImagesAsk') }, { value: 'never' as ExternalImages, label: t('externalImagesNever') }]}
          value={externalImages}
          onChange={(v) => { setExternalImages(v); saveWmSetting('externalImages', v); }}
        />
      </Row>
      <Row label={t('inlineImagePreview')} description={t('inlineImagePreviewDesc')}>
        <Toggle value={inlineImagePreview} onChange={(v) => { setInlineImagePreview(v); saveWmSetting('inlineImagePreview', v); }} />
      </Row>
      <Row label={t('smartReply')} description={t('smartReplyDesc')}>
        <Toggle value={smartReplySuggestions} onChange={(v) => { setSmartReplySuggestions(v); try { localStorage.setItem('webmail_smart_reply', v ? '1' : '0'); } catch { /* */ } }} />
      </Row>
      <Row label={t('readingTime')} description={t('readingTimeDesc')}>
        <Toggle value={showReadingTime} onChange={(v) => { setShowReadingTime(v); try { localStorage.setItem('webmail_reading_time', v ? '1' : '0'); } catch { /* */ } }} />
      </Row>
      <Row label={t('readingPane')} description={t('readingPaneDesc')} last>
        <Segment
          options={[{ value: 'right' as const, label: t('paneRight') }, { value: 'bottom' as const, label: t('paneBottom') }, { value: 'hidden' as const, label: t('paneHidden') }]}
          value={readingPanePosition}
          onChange={(v) => { setReadingPanePosition(v); try { localStorage.setItem('webmail_reading_pane', v); } catch { /* */ } }}
        />
      </Row>
    </SectionCard>
  );
}
