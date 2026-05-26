'use client';

import { useTranslations } from 'next-intl';
import { Row, SectionCard, SectionHeader, Segment, Toggle, saveWmSetting } from './settingsViewPrimitives';

type ContactsSort = 'name' | 'email' | 'company';
type ContactsDensity = 'comfortable' | 'compact';

export interface SettingsContactsSectionProps {
  contactsSort: ContactsSort;
  setContactsSort: (v: ContactsSort) => void;
  contactsDensity: ContactsDensity;
  setContactsDensity: (v: ContactsDensity) => void;
  contactsShowCompany: boolean;
  setContactsShowCompany: (v: boolean) => void;
}

export function SettingsContactsSection({
  contactsSort,
  setContactsSort,
  contactsDensity,
  setContactsDensity,
  contactsShowCompany,
  setContactsShowCompany,
}: SettingsContactsSectionProps) {
  const t = useTranslations('settingsView');

  return (
    <SectionCard>
      <SectionHeader>{t('sectionContactsSettings')}</SectionHeader>
      <Row label={t('contactsSort')} description={t('contactsSortDesc')}>
        <Segment
          options={[{ value: 'name' as ContactsSort, label: t('contactsSortName') }, { value: 'email' as ContactsSort, label: t('contactsSortEmail') }, { value: 'company' as ContactsSort, label: t('contactsSortCompany') }]}
          value={contactsSort}
          onChange={(v) => { setContactsSort(v); saveWmSetting('contactsSort', v); }}
        />
      </Row>
      <Row label={t('contactsDensity')} description={t('contactsDensityDesc')}>
        <Segment
          options={[{ value: 'comfortable' as ContactsDensity, label: t('densityComfortable') }, { value: 'compact' as ContactsDensity, label: t('densityCompact') }]}
          value={contactsDensity}
          onChange={(v) => { setContactsDensity(v); saveWmSetting('contactsDensity', v); }}
        />
      </Row>
      <Row label={t('contactsShowCompany')} description={t('contactsShowCompanyDesc')} last>
        <Toggle value={contactsShowCompany} onChange={(v) => { setContactsShowCompany(v); saveWmSetting('contactsShowCompany', v); }} />
      </Row>
    </SectionCard>
  );
}
