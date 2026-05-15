'use client';

import { ExclamationTriangleIcon } from '@heroicons/react/24/outline';
import { Row, SectionCard, SectionHeader } from '@/components/settings-view/settingsViewPrimitives';

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
  const apiToken = (() => {
    try {
      return btoa(`${userEmail ?? 'user'}:${Date.now().toString(36)}`).slice(0, 32);
    } catch {
      return 'token-unavailable';
    }
  })();

  const loginHistory = [
    { device: '현재 기기', location: '서울, 대한민국', time: '지금', current: true },
    { device: 'Chrome on macOS', location: '서울, 대한민국', time: '2일 전', current: false },
    { device: 'Safari on iPhone', location: '부산, 대한민국', time: '5일 전', current: false },
  ];

  return (
    <>
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
        <SectionHeader>API 액세스 토큰</SectionHeader>
        <div style={{ padding: '0 20px 12px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
          외부 앱이나 스크립트에서 GoGoMail API에 접근할 때 사용합니다.
        </div>
        <Row label="액세스 토큰" description="Bearer 토큰으로 API 요청에 포함하세요" last>
          <div style={{ display: 'flex', gap: '6px', alignItems: 'center' }}>
            <code style={{ fontSize: '11px', fontFamily: 'monospace', background: 'var(--color-bg-tertiary)', padding: '4px 8px', borderRadius: '4px', color: 'var(--color-text-secondary)', maxWidth: '160px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{apiToken}…</code>
            <button onClick={() => { try { navigator.clipboard.writeText(apiToken); } catch { /* */ } }}
              style={{ fontSize: '12px', padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>복사</button>
          </div>
        </Row>
      </SectionCard>

      <SectionCard>
        <SectionHeader>위험 구역</SectionHeader>
        <Row label="2단계 인증 (2FA)" description="TOTP 앱을 사용한 추가 인증 레이어 (엔터프라이즈 기능)">
          <button style={{ fontSize: '12px', padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-accent)', background: 'transparent', color: 'var(--color-accent)', cursor: 'pointer', fontWeight: 600 }}>설정하기</button>
        </Row>
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
