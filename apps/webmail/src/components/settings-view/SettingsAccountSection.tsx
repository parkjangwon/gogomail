'use client';

import { useTranslations } from 'next-intl';
import { CheckIcon } from '@heroicons/react/24/outline';
import { MiniEditor, Row, SectionCard, SectionHeader } from './settingsViewPrimitives';
import { TimezoneSelect } from './TimezoneSelect';
import type { UserProfile } from '@/lib/api';

export interface SettingsAccountSectionProps {
  userEmail?: string;
  userName?: string;
  displayName: string;
  setDisplayName: (v: string) => void;
  nameSaved: boolean;
  avatarUrl: string;
  avatarSaving: boolean;
  avatarError: string;
  recoveryEmail: string;
  setRecoveryEmail: (v: string) => void;
  recoverySaved: boolean;
  recoveryError: string;
  signature: string;
  setSignature: (v: string) => void;
  sigSaved: boolean;
  profile: UserProfile | null;
  timezone: string;
  setTimezone: (v: string) => void;
  pwCurrent: string;
  setPwCurrent: (v: string) => void;
  pwNew: string;
  setPwNew: (v: string) => void;
  pwConfirm: string;
  setPwConfirm: (v: string) => void;
  pwError: string;
  pwSaving: boolean;
  pwSaved: boolean;
  onAvatarUpload: (file: File) => void;
  onAvatarRemove: () => void;
  onSaveName: () => void;
  onSaveRecovery: () => void;
  onSaveSignature: () => void;
  onChangePassword: () => void;
}

export function SettingsAccountSection({
  userEmail,
  userName,
  displayName,
  setDisplayName,
  nameSaved,
  avatarUrl,
  avatarSaving,
  avatarError,
  recoveryEmail,
  setRecoveryEmail,
  recoverySaved,
  recoveryError,
  signature,
  setSignature,
  sigSaved,
  profile,
  timezone,
  setTimezone,
  pwCurrent,
  setPwCurrent,
  pwNew,
  setPwNew,
  pwConfirm,
  setPwConfirm,
  pwError,
  pwSaving,
  pwSaved,
  onAvatarUpload,
  onAvatarRemove,
  onSaveName,
  onSaveRecovery,
  onSaveSignature,
  onChangePassword,
}: SettingsAccountSectionProps) {
  const t = useTranslations('settingsView');

  return (
    <>
      <div style={{ display: 'flex', alignItems: 'center', gap: '16px', padding: '20px', background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-subtle)', borderRadius: '10px', marginBottom: '20px' }}>
        <div style={{ width: '52px', height: '52px', borderRadius: '50%', background: avatarUrl ? 'transparent' : 'var(--color-accent)', color: '#fff', display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '20px', fontWeight: 700, flexShrink: 0, overflow: 'hidden' }}>
          {avatarUrl ? <img src={avatarUrl} alt={t('profilePhotoAlt')} style={{ width: '100%', height: '100%', objectFit: 'cover' }} /> : (displayName || userEmail || '?')[0].toUpperCase()}
        </div>
        <div>
          <div style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{displayName || userName || t('nameEmpty')}</div>
          <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '3px' }}>{userEmail}</div>
        </div>
      </div>
      <SectionCard>
        <SectionHeader>{t('sectionProfile')}</SectionHeader>

        <Row label={t('profilePhoto')} description={t('profilePhotoDesc')}>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <label style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '12px', fontWeight: 600, cursor: avatarSaving ? 'not-allowed' : 'pointer', whiteSpace: 'nowrap', opacity: avatarSaving ? 0.6 : 1 }}>
              {avatarSaving ? t('profilePhotoSaving') : t('profilePhotoUpload')}
              <input type="file" accept="image/png,image/jpeg,image/gif,image/webp" disabled={avatarSaving} style={{ display: 'none' }} onChange={(e) => { const f = e.target.files?.[0]; if (f) onAvatarUpload(f); e.currentTarget.value = ''; }} />
            </label>
            {avatarUrl && (
              <button onClick={onAvatarRemove} disabled={avatarSaving} style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '12px', fontWeight: 600, cursor: avatarSaving ? 'not-allowed' : 'pointer', whiteSpace: 'nowrap' }}>
                {t('profilePhotoRemove')}
              </button>
            )}
            {avatarError && <span style={{ fontSize: '12px', color: 'var(--color-danger, #dc2626)', width: '100%', textAlign: 'right' }}>{avatarError}</span>}
          </div>
        </Row>
        <Row label={t('displayName')} description={t('displayNameDesc')}>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
            <input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder={t('namePlaceholder')} style={{ padding: '6px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', width: '170px', outline: 'none' }} />
            <button onClick={onSaveName} style={{ padding: '6px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '4px', whiteSpace: 'nowrap' }}>
              {nameSaved ? <><CheckIcon style={{ width: 13, height: 13 }} />{t('saved')}</> : t('save')}
            </button>
          </div>
        </Row>
        <Row label={t('emailAddress')} description={t('emailAddressDesc')}>
          <span style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', fontFamily: 'monospace' }}>{userEmail}</span>
        </Row>
        <Row label={t('recoveryEmail')} description={t('recoveryEmailDesc')}>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'center', flexWrap: 'wrap', justifyContent: 'flex-end' }}>
            <input
              type="email"
              value={recoveryEmail}
              onChange={(e) => setRecoveryEmail(e.target.value)}
              placeholder="personal@example.com"
              style={{ padding: '6px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', width: '220px', outline: 'none' }}
            />
            <button onClick={onSaveRecovery} style={{ padding: '6px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '4px', whiteSpace: 'nowrap' }}>
              {recoverySaved ? <><CheckIcon style={{ width: 13, height: 13 }} />{t('saved')}</> : t('save')}
            </button>
            {recoveryError && <span style={{ fontSize: '12px', color: 'var(--color-danger, #dc2626)', width: '100%', textAlign: 'right' }}>{recoveryError}</span>}
          </div>
        </Row>
        <Row label={t('timezone')} description={t('timezoneDesc')} last>
          <TimezoneSelect
            value={timezone}
            onChange={(v) => {
              setTimezone(v);
              try { localStorage.setItem('webmail_timezone', v); } catch { /* */ }
            }}
            placeholder={t('timezonePlaceholder')}
          />
        </Row>
      </SectionCard>
      <SectionCard>
        <SectionHeader>{t('sectionSignature')}</SectionHeader>
        <div style={{ padding: '16px 20px', background: 'var(--color-bg-primary)' }}>
          <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginBottom: '10px' }}>{t('signatureAutoAdd')}</div>
          <MiniEditor
            value={signature}
            onChange={(html) => { setSignature(html); }}
            placeholder={t('signaturePlaceholder')}
          />
          <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '10px' }}>
            <button onClick={onSaveSignature} style={{ padding: '6px 16px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer', display: 'flex', alignItems: 'center', gap: '5px' }}>
              {sigSaved ? <><CheckIcon style={{ width: 13, height: 13 }} />{t('saved')}</> : t('saveSignature')}
            </button>
          </div>
        </div>
      </SectionCard>
      {profile && (
        <SectionCard>
          <SectionHeader>{t('sectionQuota')}</SectionHeader>
          <div style={{ padding: '16px 20px' }}>
            {(() => {
              const used = profile.quota_used;
              const limit = profile.quota_limit;
              const pct = limit && limit > 0 ? Math.min(100, Math.round((used / limit) * 100)) : null;
              const fmt = (b: number) => b >= 1073741824 ? `${(b / 1073741824).toFixed(1)} GB` : b >= 1048576 ? `${(b / 1048576).toFixed(1)} MB` : `${Math.round(b / 1024)} KB`;
              return (
                <>
                  <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '13px', color: 'var(--color-text-secondary)', marginBottom: '8px' }}>
                    <span>{fmt(used)} {t('quotaUsed')}{limit ? ` / ${fmt(limit)}` : ''}</span>
                    {pct !== null && <span>{pct}%</span>}
                  </div>
                  {pct !== null && (
                    <div style={{ height: '6px', borderRadius: '3px', background: 'var(--color-bg-tertiary)', overflow: 'hidden' }}>
                      <div style={{ height: '100%', width: `${pct}%`, background: pct > 90 ? 'var(--color-destructive)' : pct > 75 ? '#f59e0b' : 'var(--color-accent)', borderRadius: '3px', transition: 'width 400ms ease' }} />
                    </div>
                  )}
                </>
              );
            })()}
          </div>
        </SectionCard>
      )}
      <SectionCard>
        <SectionHeader>{t('sectionChangePassword')}</SectionHeader>
        <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: '10px' }}>
          {([t('currentPassword'), t('newPassword'), t('confirmNewPassword')] as const).map((label, i) => (
            <div key={label} style={{ display: 'flex', flexDirection: 'column', gap: '5px' }}>
              <label style={{ fontSize: '12px', color: 'var(--color-text-secondary)', fontWeight: 500 }}>{label}</label>
              <input
                type="password"
                value={[pwCurrent, pwNew, pwConfirm][i]}
                onChange={(e) => [setPwCurrent, setPwNew, setPwConfirm][i](e.target.value)}
                style={{ padding: '7px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none' }}
              />
            </div>
          ))}
          {pwError && <div style={{ fontSize: '12px', color: 'var(--color-destructive)', padding: '6px 10px', background: 'rgba(217,79,61,0.08)', borderRadius: '5px' }}>{pwError}</div>}
          {pwSaved && <div style={{ fontSize: '12px', color: 'var(--color-success, #22c55e)', padding: '6px 10px', background: 'rgba(34,197,94,0.08)', borderRadius: '5px' }}>{t('pwChanged')}</div>}
          <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
            <button onClick={onChangePassword} disabled={pwSaving} style={{ padding: '7px 18px', borderRadius: '6px', border: 'none', background: pwSaving ? 'var(--color-bg-tertiary)' : 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: pwSaving ? 'not-allowed' : 'pointer' }}>
              {pwSaving ? t('pwChanging') : t('pwChange')}
            </button>
          </div>
        </div>
      </SectionCard>
    </>
  );
}
