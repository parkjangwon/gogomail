'use client';

import { useTranslations } from 'next-intl';
import { ACCENT_PRESETS, type WebmailSettings } from '../settings/settingsConfig';
import { labelStyle, sectionStyle, radioGroupStyle, radioLabelStyle } from './sharedStyles';

interface Props {
  settings: WebmailSettings;
  update: <K extends keyof WebmailSettings>(key: K, value: WebmailSettings[K]) => void;
  applyTheme: (theme: WebmailSettings['theme']) => void;
}

export function ThemeSection({ settings, update, applyTheme }: Props) {
  const t = useTranslations('settingsModal');
  const tSV = useTranslations('settingsView');

  const themeOpts: Array<[WebmailSettings['theme'], string]> = [
    ['light', t('themeLight')],
    ['dark', t('themeDark')],
    ['system', t('themeSystem')],
  ];

  return (
    <>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('theme')}</span>
        <div style={radioGroupStyle}>
          {themeOpts.map(([val, lbl]) => (
            <label key={val} style={radioLabelStyle}>
              <input type="radio" name="theme" value={val} data-theme={val} checked={settings.theme === val} onChange={() => applyTheme(val)} />
              {lbl}
            </label>
          ))}
        </div>
      </div>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('primaryColor')}</span>
        <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
          {ACCENT_PRESETS.map((preset) => (
            <button
              key={preset.swatch}
              title={tSV(preset.nameKey)}
              onClick={() => { update('accentColor', preset.swatch); }}
              style={{
                width: '28px',
                height: '28px',
                borderRadius: '50%',
                background: preset.swatch,
                border: 'none',
                cursor: 'pointer',
                boxShadow: settings.accentColor === preset.swatch ? `0 0 0 2px var(--color-bg-primary), 0 0 0 4px ${preset.swatch}` : 'none',
                transition: 'box-shadow 100ms ease',
              }}
            />
          ))}
        </div>
      </div>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('language')}</span>
        <div style={radioGroupStyle}>
          {([['ko', '한국어'], ['en', 'English'], ['ja', '日本語'], ['zh-CN', '中文(简体)']] as const).map(([code, label]) => (
            <label key={code} style={radioLabelStyle}>
              <input type="radio" name="locale" value={code} checked={settings.locale === code} onChange={() => update('locale', code)} />
              {label}
            </label>
          ))}
        </div>
      </div>
    </>
  );
}
