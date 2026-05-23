'use client';

import { useState } from 'react';
import { useTranslations } from 'next-intl';

import { SectionCard, SectionHeader, Row } from '@/components/settings-view/settingsViewPrimitives';

export function SettingsAboutSection() {
  const t = useTranslations();
  const [importStatus, setImportStatus] = useState<{ type: 'error' | 'success'; message: string } | null>(null);

  return (
    <>
      <SectionCard>
        <SectionHeader>{t('misc.settingsAbout.section')}</SectionHeader>
        <Row label={t('misc.settingsAbout.productName')} description={t('misc.settingsAbout.productDesc')} last>
          <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', fontFamily: 'monospace' }}>Next.js 15 · TS · Tailwind v4</span>
        </Row>
      </SectionCard>

      <SectionCard>
        <SectionHeader>{t('misc.settingsAbout.exportSection')}</SectionHeader>
        <div style={{ padding: '0 20px 12px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
          {t('misc.settingsAbout.exportIntro')}
        </div>
        <Row label={t('misc.settingsAbout.exportLabel')} description={t('misc.settingsAbout.exportDesc')}>
          <button
            onClick={() => {
              const keys = ['webmail_settings', 'webmail_filter_rules', 'webmail_blocked_senders', 'webmail_vacation', 'webmail_templates', 'webmail_theme', 'webmail_accent', 'webmail_compact', 'webmail_conv_mode', 'webmail_display_name', 'webmail_signature', 'webmail_notif_sound', 'webmail_notif_detail', 'webmail_dnd', 'webmail_dnd_start', 'webmail_dnd_end', 'webmail_focus_mode', 'webmail_importance_markers', 'webmail_swipe_left', 'webmail_swipe_right', 'webmail_cc_self', 'webmail_default_bcc', 'webmail_confirm_before_send', 'webmail_spell_check', 'webmail_smart_reply', 'webmail_reading_time', 'webmail_reading_pane', 'webmail_pinned', 'webmail_important', 'webmail_snoozed', 'webmail_labels', 'webmail_tasks', 'webmail_notes', 'webmail_recent_recipients'];
              const data: Record<string, unknown> = { _version: 1, _exportedAt: new Date().toISOString() };
              keys.forEach((k) => { try { const v = localStorage.getItem(k); if (v !== null) data[k] = JSON.parse(v); } catch { /* ignore */ } });
              const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
              const url = URL.createObjectURL(blob);
              const a = document.createElement('a'); a.href = url; a.download = 'gogomail-settings.json'; a.click();
              URL.revokeObjectURL(url);
            }}
            style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer', fontWeight: 500 }}
          >{t('misc.settingsAbout.exportButton')}</button>
        </Row>
        <Row label={t('misc.settingsAbout.importLabel')} description={t('misc.settingsAbout.importDesc')} last>
          <label style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '13px', cursor: 'pointer', fontWeight: 500, display: 'inline-block' }}>
            {t('misc.settingsAbout.importButton')}
            <input
              type="file"
              accept=".json"
              style={{ display: 'none' }}
              onChange={(e) => {
                setImportStatus(null);
                const file = e.target.files?.[0];
                if (!file) return;
                const reader = new FileReader();
                reader.onload = (ev) => {
                  try {
                    const data = JSON.parse(ev.target?.result as string) as Record<string, unknown>;
                    Object.entries(data).forEach(([k, v]) => {
                      if (k.startsWith('webmail_')) localStorage.setItem(k, JSON.stringify(v));
                    });
                    setImportStatus({ type: 'success', message: t('misc.settingsAbout.importSuccess') });
                    window.location.reload();
                  } catch {
                    setImportStatus({ type: 'error', message: t('misc.settingsAbout.importInvalid') });
                  } finally {
                    e.target.value = '';
                  }
                };
                reader.onerror = () => {
                  setImportStatus({ type: 'error', message: t('misc.settingsAbout.importReadFailed') });
                  e.target.value = '';
                };
                reader.readAsText(file);
              }}
            />
          </label>
        </Row>
        {importStatus && (
          <div
            role={importStatus.type === 'error' ? 'alert' : 'status'}
            style={{
              margin: '0 20px 16px',
              padding: '9px 12px',
              borderRadius: '6px',
              border: `1px solid ${importStatus.type === 'error' ? 'var(--color-danger, #dc2626)' : 'var(--color-border-default)'}`,
              color: importStatus.type === 'error' ? 'var(--color-danger, #dc2626)' : 'var(--color-text-secondary)',
              background: 'var(--color-bg-secondary)',
              fontSize: '12px',
              lineHeight: 1.5,
            }}
          >
            {importStatus.message}
          </div>
        )}
      </SectionCard>
    </>
  );
}
