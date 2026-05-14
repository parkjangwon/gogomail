'use client';

import { SectionCard, SectionHeader, Row } from '@/components/settings-view/settingsViewPrimitives';

export function SettingsAboutSection() {
  return (
    <>
      <SectionCard>
        <SectionHeader>정보</SectionHeader>
        <Row label="GoGoMail Webmail" description="오픈소스 엔터프라이즈 메일 클라이언트" last>
          <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', fontFamily: 'monospace' }}>Next.js 15 · TS · Tailwind v4</span>
        </Row>
      </SectionCard>

      <SectionCard>
        <SectionHeader>설정 내보내기 / 가져오기</SectionHeader>
        <div style={{ padding: '0 20px 12px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
          설정을 JSON 파일로 저장하거나 다른 기기에서 가져올 수 있습니다. 필터, 차단 목록, 템플릿 등 모든 설정이 포함됩니다.
        </div>
        <Row label="설정 내보내기" description="현재 모든 설정을 JSON 파일로 저장합니다">
          <button
            onClick={() => {
              const keys = ['webmail_settings', 'webmail_filter_rules', 'webmail_blocked_senders', 'webmail_vacation', 'webmail_templates', 'webmail_theme', 'webmail_accent', 'webmail_compact', 'webmail_conv_mode', 'webmail_display_name', 'webmail_signature', 'webmail_notif_sound', 'webmail_notif_detail', 'webmail_notif_detail', 'webmail_dnd', 'webmail_dnd_start', 'webmail_dnd_end', 'webmail_focus_mode', 'webmail_importance_markers', 'webmail_swipe_left', 'webmail_swipe_right', 'webmail_cc_self', 'webmail_default_bcc', 'webmail_confirm_before_send', 'webmail_spell_check', 'webmail_smart_reply', 'webmail_reading_time', 'webmail_reading_pane', 'webmail_pinned', 'webmail_important', 'webmail_snoozed', 'webmail_labels', 'webmail_tasks', 'webmail_notes', 'webmail_recent_recipients'];
              const data: Record<string, unknown> = { _version: 1, _exportedAt: new Date().toISOString() };
              keys.forEach((k) => { try { const v = localStorage.getItem(k); if (v !== null) data[k] = JSON.parse(v); } catch { /* ignore */ } });
              const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
              const url = URL.createObjectURL(blob);
              const a = document.createElement('a'); a.href = url; a.download = 'gogomail-settings.json'; a.click();
              URL.revokeObjectURL(url);
            }}
            style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer', fontWeight: 500 }}
          >내보내기</button>
        </Row>
        <Row label="설정 가져오기" description="gogomail-settings.json 파일에서 설정을 불러옵니다. 현재 설정이 대체됩니다." last>
          <label style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer', fontWeight: 500, display: 'inline-block' }}>
            가져오기
            <input
              type="file"
              accept=".json"
              style={{ display: 'none' }}
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (!file) return;
                const reader = new FileReader();
                reader.onload = (ev) => {
                  try {
                    const data = JSON.parse(ev.target?.result as string) as Record<string, unknown>;
                    Object.entries(data).forEach(([k, v]) => {
                      if (k.startsWith('webmail_')) localStorage.setItem(k, JSON.stringify(v));
                    });
                    window.location.reload();
                  } catch {
                    window.alert('올바르지 않은 설정 파일입니다.');
                  }
                };
                reader.readAsText(file);
              }}
            />
          </label>
        </Row>
      </SectionCard>
    </>
  );
}
