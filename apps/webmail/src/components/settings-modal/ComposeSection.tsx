'use client';

import { useTranslations } from 'next-intl';
import { type WebmailSettings } from '../settings/settingsConfig';
import { labelStyle, sectionStyle, radioGroupStyle, radioLabelStyle } from './sharedStyles';

interface Props {
  settings: WebmailSettings;
  update: <K extends keyof WebmailSettings>(key: K, value: WebmailSettings[K]) => void;
}

export function ComposeSection({ settings, update }: Props) {
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
          <input type="checkbox" checked={settings.quoteOnReply} onChange={(e) => update('quoteOnReply', e.target.checked)} />
          <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('quoteOnReply')}</span>
        </label>
      </div>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('signature')}</span>
        <textarea
          value={settings.signature}
          onChange={(e) => update('signature', e.target.value)}
          maxLength={500}
          rows={5}
          placeholder={t('signaturePlaceholder')}
          style={{
            width: '100%',
            fontSize: '13px',
            padding: '8px 10px',
            border: '1px solid var(--color-border-default)',
            borderRadius: '6px',
            background: 'var(--color-bg-secondary)',
            color: 'var(--color-text-primary)',
            resize: 'vertical',
            outline: 'none',
            fontFamily: 'inherit',
            boxSizing: 'border-box',
          }}
        />
        <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '4px', textAlign: 'right' }}>
          {settings.signature.length}/500
        </div>
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
        <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
          <input type="checkbox" checked={settings.autoSaveDraft ?? true} onChange={(e) => update('autoSaveDraft', e.target.checked)} />
          <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('autoSaveDraft')}</span>
        </label>
      </div>
    </>
  );
}
