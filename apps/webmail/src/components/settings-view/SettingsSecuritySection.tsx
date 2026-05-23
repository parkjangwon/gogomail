'use client';

import { useState, useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { ExclamationTriangleIcon } from '@heroicons/react/24/outline';
import { Row, SectionCard, SectionHeader } from '@/components/settings-view/settingsViewPrimitives';
import {
  getMFAStatus,
  startMFASetup,
  confirmMFASetup,
  disableMFA,
  type MFAStatus,
  type MFASetupResponse,
} from '@/lib/api';

interface SettingsSecuritySectionProps {
  userEmail?: string;
  revokingAll: boolean;
  revokeAllError?: string;
  onRevokeAll: () => void;
}

export function SettingsSecuritySection({
  userEmail,
  revokingAll,
  revokeAllError,
  onRevokeAll,
}: SettingsSecuritySectionProps) {
  const t = useTranslations('settingsView');
  const [mfaStatus, setMfaStatus] = useState<MFAStatus | null>(null);
  const [mfaPanel, setMfaPanel] = useState<'idle' | 'setup' | 'confirm' | 'codes' | 'disable'>('idle');
  const [setupData, setSetupData] = useState<MFASetupResponse | null>(null);
  const [confirmCode, setConfirmCode] = useState('');
  const [mfaError, setMfaError] = useState('');
  const [mfaLoading, setMfaLoading] = useState(false);

  useEffect(() => {
    getMFAStatus().then(setMfaStatus).catch(() => null);
  }, []);

  async function handleStartSetup() {
    setMfaError('');
    setMfaLoading(true);
    try {
      const data = await startMFASetup('GoGoMail', userEmail);
      setSetupData(data);
      setMfaPanel('setup');
    } catch (e: unknown) {
      setMfaError(e instanceof Error ? e.message : t('mfaSetupStartFailed'));
    } finally {
      setMfaLoading(false);
    }
  }

  async function handleConfirm() {
    setMfaError('');
    const code = confirmCode.trim();
    if (!code) { setMfaError(t('mfaCodeRequired')); return; }
    setMfaLoading(true);
    try {
      await confirmMFASetup(code);
      setMfaPanel('codes');
      setMfaStatus({ enrolled: true, enabled: true });
    } catch (e: unknown) {
      setMfaError(e instanceof Error ? e.message : t('mfaCodeConfirmFailed'));
    } finally {
      setMfaLoading(false);
    }
  }

  async function handleDisable() {
    setMfaError('');
    setMfaLoading(true);
    try {
      await disableMFA();
      setMfaPanel('idle');
      setMfaStatus({ enrolled: false, enabled: false });
      setSetupData(null);
      setConfirmCode('');
    } catch (e: unknown) {
      setMfaError(e instanceof Error ? e.message : t('mfaDisableFailed'));
    } finally {
      setMfaLoading(false);
    }
  }

  return (
    <>
      <SectionCard>
        <SectionHeader>{t('sectionMfa')}</SectionHeader>

        {mfaPanel === 'idle' && (
          <Row
            label={mfaStatus?.enabled ? t('mfaEnabled') : t('mfaDisabled')}
            description={mfaStatus?.enabled ? t('mfaEnabledDesc') : t('mfaDisabledDesc')}
            last
          >
            {mfaStatus?.enabled
              ? (
                <button
                  onClick={() => { setMfaPanel('disable'); setMfaError(''); }}
                  style={{ fontSize: '12px', padding: '5px 14px', borderRadius: '6px', border: '1px solid rgba(220,38,38,0.35)', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer' }}
                >
                  {t('mfaDisable')}
                </button>
              )
              : (
                <button
                  onClick={handleStartSetup}
                  disabled={mfaLoading}
                  style={{ fontSize: '12px', padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-accent)', background: 'transparent', color: 'var(--color-accent)', cursor: mfaLoading ? 'wait' : 'pointer', fontWeight: 600 }}
                >
                  {mfaLoading ? t('mfaPreparing') : t('mfaSetupBtn')}
                </button>
              )
            }
          </Row>
        )}

        {mfaPanel === 'setup' && setupData && (
          <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <p style={{ fontSize: '13px', color: 'var(--color-text-secondary)', margin: 0 }}>
              {t('mfaScanInstructions')}
            </p>
            <div style={{ textAlign: 'center' }}>
              <img
                src={setupData.qr_image}
                alt="MFA QR Code"
                width={180}
                height={180}
                style={{ borderRadius: '8px', border: '1px solid var(--color-border-default)' }}
              />
            </div>
            <div>
              <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginBottom: '4px' }}>{t('mfaSecretLabel')}</div>
              <code style={{ fontSize: '12px', fontFamily: 'monospace', background: 'var(--color-bg-tertiary)', padding: '6px 10px', borderRadius: '4px', display: 'block', wordBreak: 'break-all', color: 'var(--color-text-secondary)' }}>
                {setupData.secret}
              </code>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              <label style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>{t('mfaSixDigitLabel')}</label>
              <input
                type="text"
                inputMode="numeric"
                pattern="[0-9]*"
                maxLength={6}
                value={confirmCode}
                onChange={(e) => setConfirmCode(e.target.value)}
                placeholder="000000"
                style={{ padding: '10px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '18px', letterSpacing: '0.4em', textAlign: 'center', outline: 'none', fontFamily: 'monospace' }}
                onFocus={(e) => { e.target.style.borderColor = 'var(--color-accent)'; }}
                onBlur={(e) => { e.target.style.borderColor = 'var(--color-border-default)'; }}
                onKeyDown={(e) => { if (e.key === 'Enter') void handleConfirm(); }}
              />
            </div>
            {mfaError && <div role="alert" style={{ fontSize: '13px', color: 'var(--color-destructive)', background: 'rgba(217,79,61,0.08)', border: '1px solid rgba(217,79,61,0.2)', borderRadius: '6px', padding: '8px 12px' }}>{mfaError}</div>}
            <div style={{ display: 'flex', gap: '8px' }}>
              <button
                onClick={handleConfirm}
                disabled={mfaLoading}
                style={{ flex: 1, padding: '10px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '14px', fontWeight: 500, cursor: mfaLoading ? 'not-allowed' : 'pointer' }}
              >
                {mfaLoading ? t('mfaConfirming') : t('mfaConfirmActivate')}
              </button>
              <button
                onClick={() => { setMfaPanel('idle'); setMfaError(''); setConfirmCode(''); }}
                style={{ padding: '10px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '14px', cursor: 'pointer' }}
              >
                {t('cancel')}
              </button>
            </div>
          </div>
        )}

        {mfaPanel === 'codes' && setupData && (
          <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <p style={{ fontSize: '13px', color: 'var(--color-text-secondary)', margin: 0 }}>
              {t('mfaCodesIntro')}
            </p>
            <div style={{ background: 'var(--color-bg-tertiary)', borderRadius: '8px', padding: '16px', display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '8px' }}>
              {setupData.recovery_codes.map((code) => (
                <code key={code} style={{ fontFamily: 'monospace', fontSize: '12px', color: 'var(--color-text-primary)' }}>{code}</code>
              ))}
            </div>
            <button
              onClick={() => {
                try { navigator.clipboard.writeText(setupData.recovery_codes.join('\n')); } catch { /* */ }
              }}
              style={{ padding: '8px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}
            >
              {t('mfaCopy')}
            </button>
            <button
              onClick={() => setMfaPanel('idle')}
              style={{ padding: '10px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '14px', fontWeight: 500, cursor: 'pointer' }}
            >
              {t('mfaDone')}
            </button>
          </div>
        )}

        {mfaPanel === 'disable' && (
          <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <p style={{ fontSize: '13px', color: 'var(--color-text-secondary)', margin: 0 }}>
              {t('mfaDisableWarning')}
            </p>
            {mfaError && <div role="alert" style={{ fontSize: '13px', color: 'var(--color-destructive)', background: 'rgba(217,79,61,0.08)', border: '1px solid rgba(217,79,61,0.2)', borderRadius: '6px', padding: '8px 12px' }}>{mfaError}</div>}
            <div style={{ display: 'flex', gap: '8px' }}>
              <button
                onClick={handleDisable}
                disabled={mfaLoading}
                style={{ flex: 1, padding: '10px', borderRadius: '6px', border: 'none', background: 'rgba(220,38,38,0.85)', color: '#fff', fontSize: '14px', fontWeight: 500, cursor: mfaLoading ? 'not-allowed' : 'pointer' }}
              >
                {mfaLoading ? t('mfaProcessing') : t('mfaConfirmDisable')}
              </button>
              <button
                onClick={() => { setMfaPanel('idle'); setMfaError(''); }}
                style={{ padding: '10px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '14px', cursor: 'pointer' }}
              >
                {t('cancel')}
              </button>
            </div>
          </div>
        )}
      </SectionCard>

      <SectionCard>
        <SectionHeader>{t('sectionSessions')}</SectionHeader>
        <Row
          label={t('currentSession')}
          description={t('currentSessionDesc')}
          last
        >
          <span style={{ fontSize: '11px', color: 'var(--color-success, #22c55e)', fontWeight: 600, background: 'rgba(34,197,94,0.1)', padding: '2px 8px', borderRadius: '10px' }}>{t('currentBadge')}</span>
        </Row>
      </SectionCard>

      <SectionCard>
        <SectionHeader>{t('sectionDangerZone')}</SectionHeader>
        {revokeAllError && (
          <div role="alert" style={{ margin: '0 20px 12px', fontSize: '13px', color: 'var(--color-destructive)', background: 'rgba(217,79,61,0.08)', border: '1px solid rgba(217,79,61,0.2)', borderRadius: '6px', padding: '8px 12px' }}>
            {revokeAllError}
          </div>
        )}
        <Row label={t('revokeAll')} description={t('revokeAllDesc')} last>
          <button
            onClick={onRevokeAll}
            disabled={revokingAll}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 14px', borderRadius: '6px', border: '1px solid rgba(220,38,38,0.35)', background: 'rgba(220,38,38,0.04)', color: 'var(--color-destructive)', fontSize: '12px', fontWeight: 600, cursor: revokingAll ? 'wait' : 'pointer' }}
          >
            <ExclamationTriangleIcon style={{ width: 13, height: 13 }} />
            {revokingAll ? t('revokeAllProcessing') : t('revokeAllBtn')}
          </button>
        </Row>
      </SectionCard>
    </>
  );
}
