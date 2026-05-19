'use client';

import { useState, useEffect } from 'react';
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
  onRevokeAll: () => void;
}

export function SettingsSecuritySection({
  userEmail,
  revokingAll,
  onRevokeAll,
}: SettingsSecuritySectionProps) {
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
      setMfaError(e instanceof Error ? e.message : 'MFA 설정을 시작할 수 없습니다.');
    } finally {
      setMfaLoading(false);
    }
  }

  async function handleConfirm() {
    setMfaError('');
    const code = confirmCode.trim();
    if (!code) { setMfaError('코드를 입력하세요.'); return; }
    setMfaLoading(true);
    try {
      await confirmMFASetup(code);
      setMfaPanel('codes');
      setMfaStatus({ enrolled: true, enabled: true });
    } catch (e: unknown) {
      setMfaError(e instanceof Error ? e.message : '코드 확인에 실패했습니다.');
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
      setMfaError(e instanceof Error ? e.message : 'MFA를 비활성화할 수 없습니다.');
    } finally {
      setMfaLoading(false);
    }
  }

  const loginHistory = [
    { device: '현재 기기', location: '서울, 대한민국', time: '지금', current: true },
    { device: 'Chrome on macOS', location: '서울, 대한민국', time: '2일 전', current: false },
    { device: 'Safari on iPhone', location: '부산, 대한민국', time: '5일 전', current: false },
  ];

  return (
    <>
      <SectionCard>
        <SectionHeader>2단계 인증 (MFA)</SectionHeader>

        {mfaPanel === 'idle' && (
          <Row
            label={mfaStatus?.enabled ? 'TOTP 인증 활성화됨' : '2단계 인증 비활성화됨'}
            description={mfaStatus?.enabled
              ? '로그인 시 인증 앱 코드가 필요합니다.'
              : 'TOTP 앱을 사용한 추가 인증 레이어를 활성화하세요.'}
            last
          >
            {mfaStatus?.enabled
              ? (
                <button
                  onClick={() => { setMfaPanel('disable'); setMfaError(''); }}
                  style={{ fontSize: '12px', padding: '5px 14px', borderRadius: '6px', border: '1px solid rgba(220,38,38,0.35)', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer' }}
                >
                  비활성화
                </button>
              )
              : (
                <button
                  onClick={handleStartSetup}
                  disabled={mfaLoading}
                  style={{ fontSize: '12px', padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-accent)', background: 'transparent', color: 'var(--color-accent)', cursor: mfaLoading ? 'wait' : 'pointer', fontWeight: 600 }}
                >
                  {mfaLoading ? '준비 중...' : '설정하기'}
                </button>
              )
            }
          </Row>
        )}

        {mfaPanel === 'setup' && setupData && (
          <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <p style={{ fontSize: '13px', color: 'var(--color-text-secondary)', margin: 0 }}>
              인증 앱(Google Authenticator, Authy 등)으로 아래 QR 코드를 스캔하거나 비밀 키를 직접 입력하세요.
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
              <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginBottom: '4px' }}>비밀 키 (수동 입력용)</div>
              <code style={{ fontSize: '12px', fontFamily: 'monospace', background: 'var(--color-bg-tertiary)', padding: '6px 10px', borderRadius: '4px', display: 'block', wordBreak: 'break-all', color: 'var(--color-text-secondary)' }}>
                {setupData.secret}
              </code>
            </div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              <label style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)' }}>인증 앱의 6자리 코드 입력</label>
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
                {mfaLoading ? '확인 중...' : '확인 및 활성화'}
              </button>
              <button
                onClick={() => { setMfaPanel('idle'); setMfaError(''); setConfirmCode(''); }}
                style={{ padding: '10px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '14px', cursor: 'pointer' }}
              >
                취소
              </button>
            </div>
          </div>
        )}

        {mfaPanel === 'codes' && setupData && (
          <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <p style={{ fontSize: '13px', color: 'var(--color-text-secondary)', margin: 0 }}>
              MFA가 활성화되었습니다. 아래 복구 코드를 안전한 곳에 저장하세요. 인증 앱을 분실한 경우 사용할 수 있습니다.
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
              복사
            </button>
            <button
              onClick={() => setMfaPanel('idle')}
              style={{ padding: '10px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '14px', fontWeight: 500, cursor: 'pointer' }}
            >
              완료
            </button>
          </div>
        )}

        {mfaPanel === 'disable' && (
          <div style={{ padding: '16px 20px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
            <p style={{ fontSize: '13px', color: 'var(--color-text-secondary)', margin: 0 }}>
              2단계 인증을 비활성화하면 비밀번호만으로 로그인할 수 있게 됩니다. 계속하시겠습니까?
            </p>
            {mfaError && <div role="alert" style={{ fontSize: '13px', color: 'var(--color-destructive)', background: 'rgba(217,79,61,0.08)', border: '1px solid rgba(217,79,61,0.2)', borderRadius: '6px', padding: '8px 12px' }}>{mfaError}</div>}
            <div style={{ display: 'flex', gap: '8px' }}>
              <button
                onClick={handleDisable}
                disabled={mfaLoading}
                style={{ flex: 1, padding: '10px', borderRadius: '6px', border: 'none', background: 'rgba(220,38,38,0.85)', color: '#fff', fontSize: '14px', fontWeight: 500, cursor: mfaLoading ? 'not-allowed' : 'pointer' }}
              >
                {mfaLoading ? '처리 중...' : '비활성화 확인'}
              </button>
              <button
                onClick={() => { setMfaPanel('idle'); setMfaError(''); }}
                style={{ padding: '10px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '14px', cursor: 'pointer' }}
              >
                취소
              </button>
            </div>
          </div>
        )}
      </SectionCard>

      <SectionCard>
        <SectionHeader>세션 관리</SectionHeader>
        {loginHistory.map((session, i) => (
          <Row key={session.device} label={session.device} description={`${session.location} · ${session.time}`} last={i === loginHistory.length - 1}>
            {session.current
              ? <span style={{ fontSize: '11px', color: 'var(--color-success, #22c55e)', fontWeight: 600, background: 'rgba(34,197,94,0.1)', padding: '2px 8px', borderRadius: '10px' }}>현재</span>
              : <button style={{ fontSize: '12px', color: 'var(--color-destructive)', background: 'transparent', border: '1px solid rgba(220,38,38,0.3)', borderRadius: '5px', padding: '3px 10px', cursor: 'pointer' }}>종료</button>
            }
          </Row>
        ))}
      </SectionCard>

      <SectionCard>
        <SectionHeader>위험 구역</SectionHeader>
        <Row label="모든 기기에서 로그아웃" description="현재 기기를 포함한 모든 활성 세션을 즉시 종료합니다" last>
          <button
            onClick={onRevokeAll}
            disabled={revokingAll}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 14px', borderRadius: '6px', border: '1px solid rgba(220,38,38,0.35)', background: 'rgba(220,38,38,0.04)', color: 'var(--color-destructive)', fontSize: '12px', fontWeight: 600, cursor: revokingAll ? 'wait' : 'pointer' }}
          >
            <ExclamationTriangleIcon style={{ width: 13, height: 13 }} />
            {revokingAll ? '처리 중...' : '전체 로그아웃'}
          </button>
        </Row>
      </SectionCard>
    </>
  );
}
