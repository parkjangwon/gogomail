'use client';

import { useTranslations } from 'next-intl';
import { type WebmailSettings } from '../settings/settingsConfig';
import { labelStyle, sectionStyle, radioGroupStyle, radioLabelStyle } from './sharedStyles';

interface Props {
  settings: WebmailSettings;
  update: <K extends keyof WebmailSettings>(key: K, value: WebmailSettings[K]) => void;
}

export function SecuritySection({ settings, update }: Props) {
  const t = useTranslations('settingsModal');

  return (
    <>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('sessionInfo')}</span>
        <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)', display: 'flex', flexDirection: 'column', gap: '8px' }}>
          {[
            [t('lastLogin'), (() => { try { const v = localStorage.getItem('webmail_login_at'); return v ? new Intl.DateTimeFormat('ko-KR', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(v)) : '-'; } catch { return '-'; } })()],
            [t('lastIp'), (() => { try { return localStorage.getItem('webmail_login_ip') ?? '-'; } catch { return '-'; } })()],
            [t('sessionExpiry'), (() => { try { const v = localStorage.getItem('webmail_token_expires_at'); return v ? new Intl.DateTimeFormat('ko-KR', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(v)) : '-'; } catch { return '-'; } })()],
          ].map(([label, val]) => (
            <div key={label} style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 12px', background: 'var(--color-bg-secondary)', borderRadius: '6px' }}>
              <span style={{ color: 'var(--color-text-tertiary)' }}>{label}</span>
              <span style={{ fontWeight: 500, color: 'var(--color-text-primary)' }}>{val}</span>
            </div>
          ))}
        </div>
      </div>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('externalImages')}</span>
        <div style={radioGroupStyle}>
          {([['always', t('externalImagesAlways')], ['ask', t('externalImagesAsk')], ['never', t('externalImagesNever')]] as const).map(([val, lbl]) => (
            <label key={val} style={radioLabelStyle}>
              <input type="radio" name="externalImages" value={val} checked={settings.externalImages === val} onChange={() => update('externalImages', val)} />
              {lbl}
            </label>
          ))}
        </div>
      </div>
    </>
  );
}
