'use client';

import { useTranslations } from 'next-intl';
import { Kbd, SectionCard, SectionHeader } from './settingsViewPrimitives';
import { SHORTCUT_GROUPS } from './settingsViewConfig';

export function SettingsShortcutsSection() {
  const t = useTranslations('settingsView');

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px' }}>
      {SHORTCUT_GROUPS.map((group) => (
        <SectionCard key={group.titleKey}>
          <SectionHeader>{t(group.titleKey)}</SectionHeader>
          <div style={{ background: 'var(--color-bg-primary)' }}>
            {group.items.map(([key, descKey], i) => (
              <div key={key} style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', padding: '9px 16px', borderBottom: i < group.items.length - 1 ? '1px solid var(--color-border-subtle)' : 'none' }}>
                <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flex: 1 }}>{t(descKey)}</span>
                <Kbd k={key} />
              </div>
            ))}
          </div>
        </SectionCard>
      ))}
    </div>
  );
}
