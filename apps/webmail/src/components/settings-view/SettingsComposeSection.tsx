'use client';

import { useTranslations } from 'next-intl';
import { Row, SectionCard, SectionHeader, Segment, Toggle, saveWmSetting } from './settingsViewPrimitives';
import type { SendDelay, FontSize } from '@/lib/settings/settingsUtils';
import { saveLocalEmailTemplates, normalizeEmailTemplates, type StoredEmailTemplate } from '@/lib/emailTemplates';
import { setPreferences } from '@/lib/api';
import { stableId } from '@/lib/stableId';

export interface SettingsComposeSectionProps {
  sendDelay: SendDelay;
  setSendDelay: (v: SendDelay) => void;
  quoteOnReply: boolean;
  setQuoteOnReply: (v: boolean) => void;
  fontSize: FontSize;
  ccSelf: boolean;
  setCcSelf: (v: boolean) => void;
  defaultBcc: string;
  setDefaultBcc: (v: string) => void;
  confirmBeforeSend: boolean;
  setConfirmBeforeSend: (v: boolean) => void;
  spellCheck: boolean;
  setSpellCheck: (v: boolean) => void;
  templates: StoredEmailTemplate[];
  setTemplates: (v: StoredEmailTemplate[]) => void;
  newTplName: string;
  setNewTplName: (v: string) => void;
  newTplSubject: string;
  setNewTplSubject: (v: string) => void;
  newTplBody: string;
  setNewTplBody: (v: string) => void;
  showNewTpl: boolean;
  setShowNewTpl: (v: boolean) => void;
  applyFontSize: (fs: FontSize) => void;
}

export function SettingsComposeSection({
  sendDelay,
  setSendDelay,
  quoteOnReply,
  setQuoteOnReply,
  fontSize,
  ccSelf,
  setCcSelf,
  defaultBcc,
  setDefaultBcc,
  confirmBeforeSend,
  setConfirmBeforeSend,
  spellCheck,
  setSpellCheck,
  templates,
  setTemplates,
  newTplName,
  setNewTplName,
  newTplSubject,
  setNewTplSubject,
  newTplBody,
  setNewTplBody,
  showNewTpl,
  setShowNewTpl,
  applyFontSize,
}: SettingsComposeSectionProps) {
  const t = useTranslations('settingsView');

  function saveTpl() {
    if (!newTplName.trim()) return;
    const next = normalizeEmailTemplates([
      ...templates.filter((tpl) => tpl.name !== newTplName.trim()),
      { id: stableId('template'), name: newTplName.trim(), subject: newTplSubject.trim(), body: newTplBody.trim() },
    ]);
    setTemplates(next);
    saveLocalEmailTemplates(next);
    setPreferences({ templates: next }).catch(() => {});
    setNewTplName(''); setNewTplSubject(''); setNewTplBody(''); setShowNewTpl(false);
  }

  function deleteTpl(name: string) {
    const next = templates.filter((tpl) => tpl.name !== name);
    setTemplates(next);
    saveLocalEmailTemplates(next);
    setPreferences({ templates: next }).catch(() => {});
  }

  return (
    <>
      <SectionCard>
        <SectionHeader>{t('sectionComposeSettings')}</SectionHeader>
        <Row label={t('sendDelay')} description={t('sendDelayDesc')}>
          <Segment
            options={[{ value: 0 as SendDelay, label: t('sendDelayNone') }, { value: 5 as SendDelay, label: t('sendDelay5s') }, { value: 10 as SendDelay, label: t('sendDelay10s') }, { value: 30 as SendDelay, label: t('sendDelay30s') }]}
            value={sendDelay}
            onChange={(v) => { setSendDelay(v); saveWmSetting('sendDelay', v); }}
          />
        </Row>
        <Row label={t('quoteOnReply')} description={t('quoteOnReplyDesc')}>
          <Toggle value={quoteOnReply} onChange={(v) => { setQuoteOnReply(v); saveWmSetting('quoteOnReply', v); }} />
        </Row>
        <Row label={t('fontSizeDefault')} description={t('fontSizeDefaultDesc')}>
          <Segment
            options={[{ value: 'small' as FontSize, label: t('fontSizeSmall') }, { value: 'medium' as FontSize, label: t('fontSizeMedium') }, { value: 'large' as FontSize, label: t('fontSizeLarge') }]}
            value={fontSize}
            onChange={(v) => applyFontSize(v)}
          />
        </Row>
        <Row label={t('confirmBeforeSend')} description={t('confirmBeforeSendDesc')}>
          <Toggle value={confirmBeforeSend} onChange={(v) => { setConfirmBeforeSend(v); try { localStorage.setItem('webmail_confirm_before_send', v ? '1' : '0'); } catch { /* */ } }} />
        </Row>
        <Row label={t('ccSelf')} description={t('ccSelfDesc')}>
          <Toggle value={ccSelf} onChange={(v) => { setCcSelf(v); try { localStorage.setItem('webmail_cc_self', v ? '1' : '0'); } catch { /* */ } }} />
        </Row>
        <Row label={t('spellCheck')} description={t('spellCheckDesc')}>
          <Toggle value={spellCheck} onChange={(v) => { setSpellCheck(v); try { localStorage.setItem('webmail_spell_check', v ? '1' : '0'); } catch { /* */ } }} />
        </Row>
        <Row label={t('defaultBcc')} description={t('defaultBccDesc')} last>
          <input
            type="email"
            value={defaultBcc}
            onChange={(e) => { setDefaultBcc(e.target.value); try { localStorage.setItem('webmail_default_bcc', e.target.value); } catch { /* */ } }}
            placeholder="bcc@example.com"
            style={{ width: '200px', padding: '5px 10px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none' }}
          />
        </Row>
      </SectionCard>
      <SectionCard>
        <SectionHeader>{t('sectionQuickReplyTemplates')}</SectionHeader>
        {templates.length === 0 && !showNewTpl && (
          <div style={{ padding: '16px 20px', fontSize: '13px', color: 'var(--color-text-tertiary)', background: 'var(--color-bg-primary)' }}>
            {t('noTemplates')}
          </div>
        )}
        {templates.map((tpl, i) => (
          <div key={tpl.name} style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '12px 20px', borderBottom: i < templates.length - 1 || showNewTpl ? '1px solid var(--color-border-subtle)' : 'none', background: 'var(--color-bg-primary)' }}>
            <div style={{ flex: 1, minWidth: 0 }}>
              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{tpl.name}</div>
              {tpl.subject && <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{t('templateSubjectLabel')}: {tpl.subject}</div>}
            </div>
            <button onClick={() => deleteTpl(tpl.name)} style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid rgba(220,38,38,0.3)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '12px', cursor: 'pointer' }}>{t('delete')}</button>
          </div>
        ))}
        {showNewTpl && (
          <div style={{ padding: '14px 20px', background: 'var(--color-bg-secondary)', display: 'flex', flexDirection: 'column', gap: '10px' }}>
            <input value={newTplName} onChange={(e) => setNewTplName(e.target.value)} placeholder={t('tplNamePlaceholder')} style={{ padding: '7px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none' }} />
            <input value={newTplSubject} onChange={(e) => setNewTplSubject(e.target.value)} placeholder={t('tplSubjectPlaceholder')} style={{ padding: '7px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none' }} />
            <textarea value={newTplBody} onChange={(e) => setNewTplBody(e.target.value)} placeholder={t('tplBodyPlaceholder')} rows={4} style={{ padding: '8px 11px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', resize: 'vertical', fontFamily: 'inherit', outline: 'none' }} />
            <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
              <button onClick={() => setShowNewTpl(false)} style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer' }}>{t('cancel')}</button>
              <button onClick={saveTpl} style={{ padding: '6px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', fontWeight: 600, cursor: 'pointer' }}>{t('save')}</button>
            </div>
          </div>
        )}
        {!showNewTpl && (
          <div style={{ padding: '10px 20px', background: 'var(--color-bg-primary)', borderTop: templates.length > 0 ? '1px solid var(--color-border-subtle)' : 'none' }}>
            <button onClick={() => setShowNewTpl(true)} style={{ fontSize: '13px', color: 'var(--color-accent)', background: 'none', border: 'none', cursor: 'pointer', fontWeight: 500, padding: 0 }}>{t('newTemplate')}</button>
          </div>
        )}
      </SectionCard>
    </>
  );
}
