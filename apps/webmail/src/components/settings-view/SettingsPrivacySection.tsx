'use client';

import { CheckIcon } from '@heroicons/react/24/outline';
import { Row, SectionCard, SectionHeader, Segment, Toggle, saveWmSetting } from '@/components/settings-view/settingsViewPrimitives';

interface SettingsPrivacySectionProps {
  blockTrackingPixels: boolean;
  setBlockTrackingPixels: (value: boolean) => void;
  linkPreview: boolean;
  setLinkPreview: (value: boolean) => void;
  requestReadReceipt: boolean;
  setRequestReadReceipt: (value: boolean) => void;
  followUpDays: 0 | 1 | 3 | 7;
  setFollowUpDays: (value: 0 | 1 | 3 | 7) => void;
}

export function SettingsPrivacySection({
  blockTrackingPixels,
  setBlockTrackingPixels,
  linkPreview,
  setLinkPreview,
  requestReadReceipt,
  setRequestReadReceipt,
  followUpDays,
  setFollowUpDays,
}: SettingsPrivacySectionProps) {
  return (
    <>
      <SectionCard>
        <SectionHeader>추적 방지</SectionHeader>
        <Row label="추적 픽셀 차단" description="메일에 삽입된 1×1 추적 이미지를 자동으로 차단합니다. 발신자가 읽음 여부를 알 수 없습니다.">
          <Toggle value={blockTrackingPixels} onChange={(v) => { setBlockTrackingPixels(v); saveWmSetting('blockTrackingPixels', v); }} />
        </Row>
        <Row label="링크 미리보기" description="링크 위에 마우스를 올렸을 때 미리보기를 표시합니다." last>
          <Toggle value={linkPreview} onChange={(v) => { setLinkPreview(v); saveWmSetting('linkPreview', v); }} />
        </Row>
      </SectionCard>

      <SectionCard>
        <SectionHeader>발신 메일 설정</SectionHeader>
        <Row label="읽음 확인 요청" description="보내는 메일에 읽음 확인 요청을 자동으로 포함합니다.">
          <Toggle value={requestReadReceipt} onChange={(v) => { setRequestReadReceipt(v); saveWmSetting('requestReadReceipt', v); }} />
        </Row>
        <Row label="답장 미수신 시 알림" description="보낸 메일에 답장이 없을 경우 지정한 기간 후 알림을 받습니다." last>
          <Segment<0 | 1 | 3 | 7>
            options={[{ value: 0, label: '없음' }, { value: 1, label: '1일' }, { value: 3, label: '3일' }, { value: 7, label: '1주일' }]}
            value={followUpDays}
            onChange={(v) => { setFollowUpDays(v); saveWmSetting('followUpDays', v); }}
          />
        </Row>
      </SectionCard>

      <SectionCard>
        <SectionHeader>데이터 및 개인정보</SectionHeader>
        <Row label="GoGoMail 텔레메트리" description="GoGoMail은 사용자 데이터를 수집하거나 외부 서버로 전송하지 않습니다." last>
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', fontSize: '12px', color: '#16a34a', fontWeight: 600 }}>
            <CheckIcon style={{ width: 14, height: 14 }} />
            완전 로컬 처리
          </span>
        </Row>
      </SectionCard>
    </>
  );
}
