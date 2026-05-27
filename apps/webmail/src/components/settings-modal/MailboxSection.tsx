'use client';

import { useTranslations } from 'next-intl';
import { type WebmailSettings } from '../settings/settingsConfig';
import { labelStyle, sectionStyle, radioGroupStyle, radioLabelStyle } from './sharedStyles';

interface Props {
  settings: WebmailSettings;
  update: <K extends keyof WebmailSettings>(key: K, value: WebmailSettings[K]) => void;
}

export function MailboxSection({ settings, update }: Props) {
  const t = useTranslations('settingsModal');

  const readMarkOpts: Array<[WebmailSettings['readMark'], string]> = [
    ['instant', t('readMarkInstant')],
    ['2s', t('readMark2s')],
    ['manual', t('readMarkManual')],
  ];
  const listDensityOpts: Array<[WebmailSettings['listDensity'], string]> = [
    ['default', t('listDensityDefault')],
    ['compact', t('listDensityCompact')],
  ];
  const defaultSortOpts: Array<[WebmailSettings['defaultSort'], string]> = [
    ['newest', t('sortNewest')],
    ['oldest', t('sortOldest')],
  ];

  return (
    <>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('readMark')}</span>
        <div style={radioGroupStyle}>
          {readMarkOpts.map(([val, lbl]) => (
            <label key={val} style={radioLabelStyle}>
              <input type="radio" name="readMark" value={val} checked={settings.readMark === val} onChange={() => update('readMark', val)} />
              {lbl}
            </label>
          ))}
        </div>
      </div>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('listDensity')}</span>
        <div style={radioGroupStyle}>
          {listDensityOpts.map(([val, lbl]) => (
            <label key={val} style={radioLabelStyle}>
              <input type="radio" name="listDensity" value={val} checked={settings.listDensity === val} onChange={() => update('listDensity', val)} />
              {lbl}
            </label>
          ))}
        </div>
      </div>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('defaultSort')}</span>
        <div style={radioGroupStyle}>
          {defaultSortOpts.map(([val, lbl]) => (
            <label key={val} style={radioLabelStyle}>
              <input type="radio" name="defaultSort" value={val} checked={settings.defaultSort === val} onChange={() => update('defaultSort', val)} />
              {lbl}
            </label>
          ))}
        </div>
      </div>
      <div style={sectionStyle}>
        <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
          <input type="checkbox" checked={settings.showPreview ?? true} onChange={(e) => update('showPreview', e.target.checked)} />
          <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('showPreview')}</span>
        </label>
      </div>
      <div style={sectionStyle}>
        <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
          <input type="checkbox" checked={settings.threadView ?? true} onChange={(e) => update('threadView', e.target.checked)} />
          <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('threadView')}</span>
        </label>
      </div>
    </>
  );
}
