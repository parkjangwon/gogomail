'use client';

import { type Dispatch, type SetStateAction } from 'react';
import { useTranslations } from 'next-intl';
import { setWebmailAvatar } from '@/lib/webmailAvatar';
import { labelStyle, sectionStyle } from './sharedStyles';

interface Props {
  userEmail?: string;
  avatarUrl: string;
  setAvatarUrl: Dispatch<SetStateAction<string>>;
}

export function AccountSection({ userEmail, avatarUrl, setAvatarUrl }: Props) {
  const t = useTranslations('settingsModal');

  return (
    <>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('profilePicture')}</span>
        <div style={{ display: 'flex', alignItems: 'center', gap: '16px' }}>
          <div style={{ width: '72px', height: '72px', borderRadius: '50%', background: avatarUrl ? 'transparent' : 'var(--color-accent)', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '24px', fontWeight: 700, overflow: 'hidden', flexShrink: 0, border: '2px solid var(--color-border-subtle)' }}>
            {avatarUrl ? <img src={avatarUrl} alt={t('profileAlt')} style={{ width: '100%', height: '100%', objectFit: 'cover' }} /> : (userEmail ?? '?').charAt(0).toUpperCase()}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            <label style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '7px 14px', background: 'var(--color-bg-tertiary)', border: '1px solid var(--color-border-default)', borderRadius: '6px', fontSize: '13px', cursor: 'pointer', color: 'var(--color-text-primary)', fontWeight: 500 }}>
              {t('uploadPhoto')}
              <input type="file" accept="image/*" style={{ display: 'none' }} onChange={(e) => {
                const file = e.target.files?.[0];
                if (!file) return;
                const reader = new FileReader();
                reader.onload = (ev) => {
                  const url = ev.target?.result as string;
                  setAvatarUrl(url);
                  setWebmailAvatar(url);
                };
                reader.readAsDataURL(file);
              }} />
            </label>
            {avatarUrl && (
              <button onClick={() => { setAvatarUrl(''); setWebmailAvatar(''); }} style={{ padding: '7px 14px', background: 'transparent', border: '1px solid var(--color-border-default)', borderRadius: '6px', fontSize: '13px', cursor: 'pointer', color: 'var(--color-destructive)' }}>
                {t('removePhoto')}
              </button>
            )}
          </div>
        </div>
      </div>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('email')}</span>
        <input type="email" readOnly value={userEmail ?? ''} style={{ width: '100%', fontSize: '13px', padding: '8px 10px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-tertiary)', color: 'var(--color-text-secondary)', outline: 'none', boxSizing: 'border-box', cursor: 'default' }} />
      </div>
      <div style={sectionStyle}>
        <span style={labelStyle}>{t('displayName')}</span>
        <input type="text" readOnly value={userEmail ? userEmail.split('@')[0] : ''} style={{ width: '100%', fontSize: '13px', padding: '8px 10px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-tertiary)', color: 'var(--color-text-secondary)', outline: 'none', boxSizing: 'border-box', cursor: 'default' }} />
      </div>
    </>
  );
}
