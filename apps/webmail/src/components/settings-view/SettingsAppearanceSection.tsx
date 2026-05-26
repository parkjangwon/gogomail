'use client';

import { useTranslations } from 'next-intl';
import { Row, SectionCard, SectionHeader, Segment, Toggle } from './settingsViewPrimitives';
import { ACCENT_COLORS, type Theme, type FontSize } from '@/lib/settings/settingsUtils';

export interface SettingsAppearanceSectionProps {
  theme: Theme;
  accent: string;
  customAccent: string;
  setCustomAccent: (v: string) => void;
  compact: boolean;
  setCompact: (v: boolean) => void;
  fontSize: FontSize;
  applyTheme: (t: Theme) => void;
  applyAccent: (color: string) => void;
  applyFontSize: (fs: FontSize) => void;
  setAccent: (v: string) => void;
}

export function SettingsAppearanceSection({
  theme,
  accent,
  customAccent,
  setCustomAccent,
  compact,
  setCompact,
  fontSize,
  applyTheme,
  applyAccent,
  applyFontSize,
  setAccent,
}: SettingsAppearanceSectionProps) {
  const t = useTranslations('settingsView');

  return (
    <>
      <SectionCard>
        <SectionHeader>{t('sectionTheme')}</SectionHeader>
        <Row label={t('themeMode')} description={t('themeModeDesc')} last>
          <Segment
            options={[{ value: 'light' as Theme, label: t('themeLight') }, { value: 'dark' as Theme, label: t('themeDark') }, { value: 'system' as Theme, label: t('themeSystem') }]}
            value={theme}
            onChange={applyTheme}
          />
        </Row>
      </SectionCard>
      <SectionCard>
        <SectionHeader>{t('sectionAccent')}</SectionHeader>
        <div style={{ padding: '16px 20px', background: 'var(--color-bg-primary)' }}>
          <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginBottom: '14px' }}>{t('accentDesc')}</div>
          <div style={{ display: 'flex', gap: '10px', alignItems: 'center', flexWrap: 'wrap' }}>
            {ACCENT_COLORS.map((c) => (
              <button
                key={c.value}
                title={c.label}
                onClick={() => applyAccent(c.value)}
                style={{ width: '28px', height: '28px', borderRadius: '50%', background: c.value, border: `2.5px solid ${accent === c.value ? 'var(--color-text-primary)' : 'transparent'}`, cursor: 'pointer', padding: 0, boxShadow: accent === c.value ? `0 0 0 1.5px ${c.value}` : 'none', transition: 'border-color 120ms ease', flexShrink: 0 }}
              />
            ))}
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px', marginLeft: '4px' }}>
              <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('accentCustom')}</span>
              <input
                type="text"
                value={customAccent}
                onChange={(e) => setCustomAccent(e.target.value)}
                placeholder="#2563eb"
                style={{ width: '80px', padding: '4px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '12px', fontFamily: 'monospace', outline: 'none' }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    const hex = customAccent.startsWith('#') ? customAccent : `#${customAccent}`;
                    if (/^#[0-9a-f]{6}$/i.test(hex)) { applyAccent(hex); setAccent(hex); }
                  }
                }}
              />
            </div>
          </div>
        </div>
      </SectionCard>
      <SectionCard>
        <SectionHeader>{t('sectionDensityFont')}</SectionHeader>
        <Row label={t('compactView')} description={t('compactViewDesc')}>
          <Toggle value={compact} onChange={(v) => { setCompact(v); try { localStorage.setItem('webmail_compact', v ? '1' : '0'); } catch { /* */ } }} />
        </Row>
        <Row label={t('fontSizeBody')} description={t('fontSizeBodyDesc')} last>
          <Segment
            options={[{ value: 'small' as FontSize, label: t('fontSizeSmallPx') }, { value: 'medium' as FontSize, label: t('fontSizeMediumPx') }, { value: 'large' as FontSize, label: t('fontSizeLargePx') }]}
            value={fontSize}
            onChange={applyFontSize}
          />
        </Row>
      </SectionCard>
    </>
  );
}
