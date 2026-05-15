'use client';

import { Row, SectionCard, SectionHeader, Segment, Toggle } from '@/components/settings-view/settingsViewPrimitives';

interface SettingsNotificationsSectionProps {
  notifPerm: NotificationPermission;
  onRequestNotif: () => void;
  notifSound: boolean;
  setNotifSound: (value: boolean) => void;
  notifDetail: 'sender' | 'subject' | 'preview';
  setNotifDetail: (value: 'sender' | 'subject' | 'preview') => void;
  dndEnabled: boolean;
  setDndEnabled: (value: boolean) => void;
  dndStart: string;
  setDndStart: (value: string) => void;
  dndEnd: string;
  setDndEnd: (value: string) => void;
}

export function SettingsNotificationsSection({
  notifPerm,
  onRequestNotif,
  notifSound,
  setNotifSound,
  notifDetail,
  setNotifDetail,
  dndEnabled,
  setDndEnabled,
  dndStart,
  setDndStart,
  dndEnd,
  setDndEnd,
}: SettingsNotificationsSectionProps) {
  return (
    <SectionCard>
      <SectionHeader>알림 설정</SectionHeader>
      <Row label="브라우저 알림" description={notifPerm === 'granted' ? '새 메일 알림이 허용되어 있습니다' : notifPerm === 'denied' ? '알림이 차단됨 — 브라우저 설정에서 변경하세요' : '새 메일 도착 시 데스크탑 알림을 보냅니다'}>
        {notifPerm === 'granted'
          ? <span style={{ fontSize: '12px', color: 'var(--color-success, #22c55e)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '4px' }}><span>허용됨</span></span>
          : notifPerm === 'denied'
          ? <span style={{ fontSize: '12px', color: 'var(--color-destructive)', fontWeight: 500 }}>차단됨</span>
          : <button onClick={onRequestNotif} style={{ padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '12px', cursor: 'pointer' }}>허용하기</button>
        }
      </Row>
      <Row label="알림 소리" description="새 메일 도착 시 알림음을 재생합니다">
        <Toggle value={notifSound} onChange={setNotifSound} />
      </Row>
      <Row label="알림 표시 수준" description="알림 팝업에 표시할 정보 수준을 선택합니다">
        <Segment
          options={[{ value: 'sender' as const, label: '발신자' }, { value: 'subject' as const, label: '제목' }, { value: 'preview' as const, label: '미리보기' }]}
          value={notifDetail}
          onChange={setNotifDetail}
        />
      </Row>
      <Row label="방해 금지 모드" description="지정한 시간대에 알림을 무음으로 처리합니다">
        <Toggle value={dndEnabled} onChange={setDndEnabled} />
      </Row>
      {dndEnabled && (
        <Row label="방해 금지 시간대" description="알림을 억제할 시작·종료 시간" last>
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <input type="time" value={dndStart} onChange={(e) => setDndStart(e.target.value)}
              style={{ padding: '4px 8px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '13px' }} />
            <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)' }}>~</span>
            <input type="time" value={dndEnd} onChange={(e) => setDndEnd(e.target.value)}
              style={{ padding: '4px 8px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '13px' }} />
          </div>
        </Row>
      )}
      {!dndEnabled && <div style={{ height: '1px' }} />}
    </SectionCard>
  );
}
