'use client';

import { useTranslations } from 'next-intl';
import { type WebmailSettings } from '../settings/settingsConfig';
import { labelStyle, sectionStyle, radioGroupStyle, radioLabelStyle } from './sharedStyles';

interface Props {
  settings: WebmailSettings;
  update: <K extends keyof WebmailSettings>(key: K, value: WebmailSettings[K]) => void;
}

export function AdvancedSection({ settings, update }: Props) {
  const t = useTranslations('settingsModal');

  const sendDelayOpts: Array<['0' | '5' | '10' | '30', string]> = [
    ['0', t('sendDelayOff')],
    ['5', t('sendDelay5s')],
    ['10', t('sendDelay10s')],
    ['30', t('sendDelay30s')],
  ];

  return (
    <>
      <div style={sectionStyle}>
        <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
          <input type="checkbox" checked={settings.threadView ?? true} onChange={(e) => update('threadView', e.target.checked)} />
          <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('threadView')}</span>
        </label>
        <p style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '4px', marginLeft: '24px' }}>{t('threadViewDesc')}</p>
      </div>
      <div style={sectionStyle}>
        <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
          <input type="checkbox" checked={settings.showPreview ?? true} onChange={(e) => update('showPreview', e.target.checked)} />
          <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('showPreviewLabel')}</span>
        </label>
        <p style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '4px', marginLeft: '24px' }}>{t('showPreviewDesc')}</p>
      </div>
      <div style={sectionStyle}>
        <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
          <input type="checkbox" checked={settings.autoSaveDraft ?? true} onChange={(e) => update('autoSaveDraft', e.target.checked)} />
          <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('autoSaveDraft')}</span>
        </label>
        <p style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '4px', marginLeft: '24px' }}>{t('autoSaveDraftDesc')}</p>
      </div>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('sendDelay')}</span>
        <div style={radioGroupStyle}>
          {sendDelayOpts.map(([val, lbl]) => (
            <label key={val} style={radioLabelStyle}>
              <input type="radio" name="sendDelay" value={val} checked={String(settings.sendDelay ?? 0) === val} onChange={() => update('sendDelay', Number(val) as 0 | 5 | 10 | 30)} />
              {lbl}
            </label>
          ))}
        </div>
      </div>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('fontSize')}</span>
        <div style={radioGroupStyle}>
          {([['small', t('fontSizeSmall')], ['medium', t('fontSizeMedium')], ['large', t('fontSizeLarge')]] as const).map(([val, lbl]) => (
            <label key={val} style={radioLabelStyle}>
              <input type="radio" name="fontSize" value={val} checked={(settings.fontSize ?? 'medium') === val} onChange={() => update('fontSize', val)} />
              {lbl}
            </label>
          ))}
        </div>
      </div>
    </>
  );
}
