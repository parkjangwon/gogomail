'use client';
import { BuildingOffice2Icon } from '@heroicons/react/24/outline';
import { useTranslations } from 'next-intl';
export function OrgChartPlaceholder() {
  const t = useTranslations('placeholders');
  return (
    <div style={{ flex: 1, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', gap: '12px', color: 'var(--color-text-tertiary)' }}>
      <BuildingOffice2Icon style={{ width: '48px', height: '48px', opacity: 0.4 }} />
      <span style={{ fontSize: '16px', fontWeight: 500 }}>{t('orgChart')}</span>
      <span style={{ fontSize: '13px' }}>{t('notReady')}</span>
    </div>
  );
}
