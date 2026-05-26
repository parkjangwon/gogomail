'use client';

import { useTranslations } from 'next-intl';
import { Row, SectionCard, SectionHeader, Segment, saveWmSetting } from './settingsViewPrimitives';

type DriveSort = 'typeName' | 'name' | 'updated' | 'size';

export interface SettingsDriveSectionProps {
  driveSort: DriveSort;
  setDriveSort: (v: DriveSort) => void;
}

export function SettingsDriveSection({ driveSort, setDriveSort }: SettingsDriveSectionProps) {
  const t = useTranslations('settingsView');

  return (
    <SectionCard>
      <SectionHeader>{t('sectionDriveSettings')}</SectionHeader>
      <Row label={t('driveSort')} description={t('driveSortDesc')} last>
        <Segment
          options={[{ value: 'typeName' as DriveSort, label: t('driveSortTypeName') }, { value: 'name' as DriveSort, label: t('driveSortName') }, { value: 'updated' as DriveSort, label: t('driveSortUpdated') }, { value: 'size' as DriveSort, label: t('driveSortSize') }]}
          value={driveSort}
          onChange={(v) => { setDriveSort(v); saveWmSetting('driveSort', v); }}
        />
      </Row>
    </SectionCard>
  );
}
