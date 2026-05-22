'use client';

import { type Dispatch, type SetStateAction } from 'react';
import { useTranslations } from 'next-intl';
import { XMarkIcon } from '@heroicons/react/24/outline';
import {
  ACCENT_PRESETS,
  Category,
  FilterAction,
  FilterCondition,
  FilterRule,
  LABEL_COLORS,
  WebmailSettings,
  createEmptyRule,
  saveFilterRules,
} from './settings/settingsConfig';
import { setWebmailAvatar } from '@/lib/webmailAvatar';
import { stableId } from '@/lib/stableId';

interface SettingsModalContentProps {
  activeCategory: Category;
  settings: WebmailSettings;
  userEmail?: string;
  avatarUrl: string;
  setAvatarUrl: Dispatch<SetStateAction<string>>;
  update: <K extends keyof WebmailSettings>(key: K, value: WebmailSettings[K]) => void;
  applyTheme: (theme: WebmailSettings['theme']) => void;
  handleNotificationToggle: (checked: boolean) => void;
  filterRules: FilterRule[];
  setFilterRules: Dispatch<SetStateAction<FilterRule[]>>;
  editingRule: FilterRule | null;
  setEditingRule: Dispatch<SetStateAction<FilterRule | null>>;
  newRule: Omit<FilterRule, 'id'>;
  setNewRule: Dispatch<SetStateAction<Omit<FilterRule, 'id'>>>;
}

export function SettingsModalContent({
  activeCategory,
  settings,
  userEmail,
  avatarUrl,
  setAvatarUrl,
  update,
  applyTheme,
  handleNotificationToggle,
  filterRules,
  setFilterRules,
  editingRule,
  setEditingRule,
  newRule,
  setNewRule,
}: SettingsModalContentProps) {
  const t = useTranslations('settingsModal');
  const tFilter = useTranslations('filterRules');
  const labelStyle: React.CSSProperties = {
    fontSize: '13px',
    fontWeight: 500,
    color: 'var(--color-text-primary)',
    marginBottom: '8px',
    display: 'block',
  };

  const sectionStyle: React.CSSProperties = { marginBottom: '24px' };
  const radioGroupStyle: React.CSSProperties = { display: 'flex', flexDirection: 'column', gap: '6px' };
  const radioLabelStyle: React.CSSProperties = { display: 'flex', alignItems: 'center', gap: '8px', fontSize: '13px', color: 'var(--color-text-secondary)', cursor: 'pointer' };

  const readMarkOpts: Array<[WebmailSettings['readMark'], string]> = [['instant', t('readMarkInstant')], ['2s', t('readMark2s')], ['manual', t('readMarkManual')]];
  const listDensityOpts: Array<[WebmailSettings['listDensity'], string]> = [['default', t('listDensityDefault')], ['compact', t('listDensityCompact')]];
  const defaultSortOpts: Array<[WebmailSettings['defaultSort'], string]> = [['newest', t('sortNewest')], ['oldest', t('sortOldest')]];
  const sendDelayOpts: Array<['0' | '5' | '10' | '30', string]> = [['0', t('sendDelayOff')], ['5', t('sendDelay5s')], ['10', t('sendDelay10s')], ['30', t('sendDelay30s')]];
  const themeOpts: Array<[WebmailSettings['theme'], string]> = [['light', t('themeLight')], ['dark', t('themeDark')], ['system', t('themeSystem')]];

  switch (activeCategory) {
    case 'mailbox':
      return (
        <>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('readMark')}</span>
            <div style={radioGroupStyle}>
              {readMarkOpts.map(([val, lbl]) => (
                <label key={val} style={radioLabelStyle}>
                  <input type="radio" name="readMark" value={val} checked={settings.readMark === val} onChange={() => update('readMark', val)} />
                  {lbl}
                </label>
              ))}
            </div>
          </div>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('listDensity')}</span>
            <div style={radioGroupStyle}>
              {listDensityOpts.map(([val, lbl]) => (
                <label key={val} style={radioLabelStyle}>
                  <input type="radio" name="listDensity" value={val} checked={settings.listDensity === val} onChange={() => update('listDensity', val)} />
                  {lbl}
                </label>
              ))}
            </div>
          </div>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('defaultSort')}</span>
            <div style={radioGroupStyle}>
              {defaultSortOpts.map(([val, lbl]) => (
                <label key={val} style={radioLabelStyle}>
                  <input type="radio" name="defaultSort" value={val} checked={settings.defaultSort === val} onChange={() => update('defaultSort', val)} />
                  {lbl}
                </label>
              ))}
            </div>
          </div>
          <div style={sectionStyle}>
            <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
              <input type="checkbox" checked={settings.showPreview ?? true} onChange={(e) => update('showPreview', e.target.checked)} />
              <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('showPreview')}</span>
            </label>
          </div>
          <div style={sectionStyle}>
            <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
              <input type="checkbox" checked={settings.threadView ?? true} onChange={(e) => update('threadView', e.target.checked)} />
              <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('threadView')}</span>
            </label>
          </div>
        </>
      );
    case 'compose':
      return (
        <>
          <div style={sectionStyle}>
            <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
              <input type="checkbox" checked={settings.quoteOnReply} onChange={(e) => update('quoteOnReply', e.target.checked)} />
              <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('quoteOnReply')}</span>
            </label>
          </div>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('signature')}</span>
            <textarea
              value={settings.signature}
              onChange={(e) => update('signature', e.target.value)}
              maxLength={500}
              rows={5}
              placeholder={t('signaturePlaceholder')}
              style={{
                width: '100%',
                fontSize: '13px',
                padding: '8px 10px',
                border: '1px solid var(--color-border-default)',
                borderRadius: '6px',
                background: 'var(--color-bg-secondary)',
                color: 'var(--color-text-primary)',
                resize: 'vertical',
                outline: 'none',
                fontFamily: 'inherit',
                boxSizing: 'border-box',
              }}
            />
            <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '4px', textAlign: 'right' }}>
              {settings.signature.length}/500
            </div>
          </div>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('sendDelay')}</span>
            <div style={radioGroupStyle}>
              {sendDelayOpts.map(([val, lbl]) => (
                <label key={val} style={radioLabelStyle}>
                  <input type="radio" name="sendDelay" value={val} checked={String(settings.sendDelay ?? 0) === val} onChange={() => update('sendDelay', Number(val) as 0 | 5 | 10 | 30)} />
                  {lbl}
                </label>
              ))}
            </div>
          </div>
          <div style={sectionStyle}>
            <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
              <input type="checkbox" checked={settings.autoSaveDraft ?? true} onChange={(e) => update('autoSaveDraft', e.target.checked)} />
              <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('autoSaveDraft')}</span>
            </label>
          </div>
        </>
      );
    case 'theme':
      return (
        <>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('theme')}</span>
            <div style={radioGroupStyle}>
              {themeOpts.map(([val, lbl]) => (
                <label key={val} style={radioLabelStyle}>
                  <input type="radio" name="theme" value={val} data-theme={val} checked={settings.theme === val} onChange={() => applyTheme(val)} />
                  {lbl}
                </label>
              ))}
            </div>
          </div>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('primaryColor')}</span>
            <div style={{ display: 'flex', gap: '10px', flexWrap: 'wrap' }}>
              {ACCENT_PRESETS.map((preset) => (
                <button
                  key={preset.swatch}
                  title={preset.name}
                  onClick={() => { update('accentColor', preset.swatch); }}
                  style={{
                    width: '28px',
                    height: '28px',
                    borderRadius: '50%',
                    background: preset.swatch,
                    border: 'none',
                    cursor: 'pointer',
                    boxShadow: settings.accentColor === preset.swatch ? `0 0 0 2px var(--color-bg-primary), 0 0 0 4px ${preset.swatch}` : 'none',
                    transition: 'box-shadow 100ms ease',
                  }}
                />
              ))}
            </div>
          </div>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('language')}</span>
            <div style={radioGroupStyle}>
              {([['ko', '한국어'], ['en', 'English'], ['ja', '日本語'], ['zh-CN', '中文(简体)']] as const).map(([code, label]) => (
                <label key={code} style={radioLabelStyle}>
                  <input type="radio" name="locale" value={code} checked={settings.locale === code} onChange={() => update('locale', code)} />
                  {label}
                </label>
              ))}
            </div>
          </div>
        </>
      );
    case 'notifications':
      return (
        <div style={sectionStyle}>
          <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
            <input type="checkbox" checked={settings.notifications} onChange={(e) => handleNotificationToggle(e.target.checked)} />
            <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('notifications')}</span>
          </label>
          {settings.notifications && (
            <p style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '8px', marginLeft: '24px' }}>{t('notificationsActive')}</p>
          )}
        </div>
      );
    case 'account':
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
    case 'security':
      return (
        <>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('sessionInfo')}</span>
            <div style={{ fontSize: '13px', color: 'var(--color-text-secondary)', display: 'flex', flexDirection: 'column', gap: '8px' }}>
              {[
                [t('lastLogin'), (() => { try { const v = localStorage.getItem('webmail_login_at'); return v ? new Intl.DateTimeFormat('ko-KR', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(v)) : '-'; } catch { return '-'; } })()],
                [t('lastIp'), (() => { try { return localStorage.getItem('webmail_login_ip') ?? '-'; } catch { return '-'; } })()],
                [t('sessionExpiry'), (() => { try { const v = localStorage.getItem('webmail_token_expires_at'); return v ? new Intl.DateTimeFormat('ko-KR', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(v)) : '-'; } catch { return '-'; } })()],
              ].map(([label, val]) => (
                <div key={label} style={{ display: 'flex', justifyContent: 'space-between', padding: '8px 12px', background: 'var(--color-bg-secondary)', borderRadius: '6px' }}>
                  <span style={{ color: 'var(--color-text-tertiary)' }}>{label}</span>
                  <span style={{ fontWeight: 500, color: 'var(--color-text-primary)' }}>{val}</span>
                </div>
              ))}
            </div>
          </div>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('externalImages')}</span>
            <div style={radioGroupStyle}>
              {([['always', t('externalImagesAlways')], ['ask', t('externalImagesAsk')], ['never', t('externalImagesNever')]] as const).map(([val, lbl]) => (
                <label key={val} style={radioLabelStyle}>
                  <input type="radio" name="externalImages" value={val} checked={settings.externalImages === val} onChange={() => update('externalImages', val)} />
                  {lbl}
                </label>
              ))}
            </div>
          </div>
        </>
      );
    case 'shortcuts':
      return (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
          {[
            ['c', t('shortcuts.compose')],
            ['r', t('shortcuts.reply')],
            ['a', t('shortcuts.replyAll')],
            ['f', t('shortcuts.forward')],
            ['/', t('shortcuts.searchFocus')],
            ['[', t('shortcuts.toggleSidebar')],
            ['j / k', t('shortcuts.nextPrev')],
            ['e', t('shortcuts.archive')],
            ['Delete', t('shortcuts.delete')],
            ['m', t('shortcuts.markRead')],
            ['u', t('shortcuts.markUnread')],
            ['s', t('shortcuts.toggleStar')],
            ['Esc', t('shortcuts.close')],
          ].map(([key, desc]) => (
            <div key={desc} style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '8px 12px', borderRadius: '6px', background: 'var(--color-bg-secondary)', marginBottom: '2px' }}>
              <span style={{ fontSize: '13px', color: 'var(--color-text-secondary)' }}>{desc}</span>
              <kbd style={{ fontSize: '11px', fontFamily: 'monospace', padding: '2px 8px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '4px', color: 'var(--color-text-primary)', fontWeight: 600 }}>{key}</kbd>
            </div>
          ))}
        </div>
      );
    case 'advanced':
      return (
        <>
          <div style={sectionStyle}>
            <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
              <input type="checkbox" checked={settings.threadView ?? true} onChange={(e) => update('threadView', e.target.checked)} />
              <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('threadView')}</span>
            </label>
            <p style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '4px', marginLeft: '24px' }}>{t('threadViewDesc')}</p>
          </div>
          <div style={sectionStyle}>
            <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
              <input type="checkbox" checked={settings.showPreview ?? true} onChange={(e) => update('showPreview', e.target.checked)} />
              <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('showPreviewLabel')}</span>
            </label>
            <p style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '4px', marginLeft: '24px' }}>{t('showPreviewDesc')}</p>
          </div>
          <div style={sectionStyle}>
            <label style={{ ...radioLabelStyle, cursor: 'pointer' }}>
              <input type="checkbox" checked={settings.autoSaveDraft ?? true} onChange={(e) => update('autoSaveDraft', e.target.checked)} />
              <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', fontWeight: 500 }}>{t('autoSaveDraft')}</span>
            </label>
            <p style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '4px', marginLeft: '24px' }}>{t('autoSaveDraftDesc')}</p>
          </div>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('sendDelay')}</span>
            <div style={radioGroupStyle}>
              {sendDelayOpts.map(([val, lbl]) => (
                <label key={val} style={radioLabelStyle}>
                  <input type="radio" name="sendDelay" value={val} checked={String(settings.sendDelay ?? 0) === val} onChange={() => update('sendDelay', Number(val) as 0 | 5 | 10 | 30)} />
                  {lbl}
                </label>
              ))}
            </div>
          </div>
          <div style={sectionStyle}>
            <span style={labelStyle}>{t('fontSize')}</span>
            <div style={radioGroupStyle}>
              {([['small', t('fontSizeSmall')], ['medium', t('fontSizeMedium')], ['large', t('fontSizeLarge')]] as const).map(([val, lbl]) => (
                <label key={val} style={radioLabelStyle}>
                  <input type="radio" name="fontSize" value={val} checked={(settings.fontSize ?? 'medium') === val} onChange={() => update('fontSize', val)} />
                  {lbl}
                </label>
              ))}
            </div>
          </div>
        </>
      );
    case 'filters': {
      const fieldOpts: { value: FilterCondition['field']; label: string; noValue?: boolean }[] = [
        { value: 'from', label: tFilter('fieldFrom') },
        { value: 'to', label: tFilter('fieldTo') },
        { value: 'cc', label: tFilter('fieldCc') },
        { value: 'subject', label: tFilter('fieldSubject') },
        { value: 'body', label: tFilter('fieldBody') },
        { value: 'has_attachment', label: tFilter('fieldHasAttachment'), noValue: true },
        { value: 'is_unread', label: tFilter('fieldIsUnread'), noValue: true },
        { value: 'size_larger', label: tFilter('fieldSizeLarger') },
        { value: 'size_smaller', label: tFilter('fieldSizeSmaller') },
      ];
      const matchOpts: { value: FilterCondition['matchType']; label: string }[] = [
        { value: 'contains', label: tFilter('opContains') },
        { value: 'not_contains', label: tFilter('opNotContains') },
        { value: 'equals', label: tFilter('opEquals') },
        { value: 'starts_with', label: tFilter('opStartsWith') },
        { value: 'ends_with', label: tFilter('opEndsWith') },
        { value: 'regex', label: tFilter('opRegex') },
      ];
      const ist: React.CSSProperties = { border: '1px solid var(--color-border-default)', borderRadius: '6px', padding: '5px 8px', fontSize: '12px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', outline: 'none' };
      const sst: React.CSSProperties = { ...ist, cursor: 'pointer', flexShrink: 0 };
      const doSave = (rule: Omit<FilterRule, 'id'>, id?: string) => {
        const validConds = rule.conditions.filter((c) => fieldOpts.find((f) => f.value === c.field)?.noValue || c.value.trim());
        if (validConds.length === 0) return;
        const clean = { ...rule, conditions: validConds };
        const updated = id
          ? filterRules.map((r) => r.id === id ? { ...clean, id } : r)
          : [...filterRules, { ...clean, id: stableId('filter') }];
        setFilterRules(updated);
        saveFilterRules(updated);
        setEditingRule(null);
        setNewRule(createEmptyRule());
      };
      const cur = editingRule ?? newRule;
      const setCur = (patch: Partial<Omit<FilterRule, 'id'>>) =>
        editingRule ? setEditingRule((r) => r ? { ...r, ...patch } : r) : setNewRule((p) => ({ ...p, ...patch }));
      const toggleEnabled = (rule: FilterRule) => {
        const updated = filterRules.map((r) => r.id === rule.id ? { ...r, enabled: !r.enabled } : r);
        setFilterRules(updated);
        saveFilterRules(updated);
      };
      const condSummary = (rule: FilterRule) =>
        rule.conditions.map((c) => {
          const fo = fieldOpts.find((f) => f.value === c.field);
          if (fo?.noValue) return fo.label;
          return `${fo?.label ?? c.field} ${matchOpts.find((m) => m.value === c.matchType)?.label ?? c.matchType} "${c.value}"`;
        }).join(rule.logic === 'and' ? ' AND ' : ' OR ');
      const actionBadges = (a: FilterAction) => {
        const parts: string[] = [];
        if (a.labelColor) parts.push('라벨');
        if (a.moveToFolder) parts.push(`→ ${a.moveToFolder}`);
        if (a.markRead) parts.push('읽음');
        if (a.markUnread) parts.push('안읽음');
        if (a.markStarred) parts.push('별표');
        if (a.markImportant) parts.push('중요');
        if (a.skipInbox) parts.push('받은편지함 건너뜀');
        if (a.deleteMsg) parts.push('삭제');
        if (a.forwardTo) parts.push(`전달 → ${a.forwardTo}`);
        return parts.join(' · ') || '작업 없음';
      };
      return (
        <>
          <div style={sectionStyle}>
            <span style={labelStyle}>메일 필터 규칙</span>
            <p style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginBottom: '12px' }}>조건에 맞는 수신 메일에 자동으로 작업을 적용합니다. 위에서부터 순서대로 평가됩니다.</p>
            {filterRules.length === 0 && <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', padding: '12px 0' }}>필터 규칙이 없습니다.</div>}
            {filterRules.map((rule, idx) => (
              <div key={rule.id} style={{ display: 'flex', alignItems: 'flex-start', gap: '8px', padding: '10px 12px', borderRadius: '8px', background: rule.enabled ? 'var(--color-bg-secondary)' : 'transparent', marginBottom: '4px', border: `1px solid ${rule.enabled ? 'var(--color-border-subtle)' : 'var(--color-border-subtle)'}`, opacity: rule.enabled ? 1 : 0.5 }}>
                <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0, width: '16px', textAlign: 'center', marginTop: '2px' }}>{idx + 1}</span>
                {rule.action.labelColor && <span style={{ width: '10px', height: '10px', borderRadius: '50%', background: rule.action.labelColor, flexShrink: 0, marginTop: '4px', display: 'inline-block' }} />}
                <div style={{ flex: 1, minWidth: 0 }}>
                  {rule.name && <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-primary)', marginBottom: '2px' }}>{rule.name}</div>}
                  <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{condSummary(rule)}</div>
                  <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{actionBadges(rule.action)}{rule.stopProcessing ? ' · [중단]' : ''}</div>
                </div>
                <button onClick={() => toggleEnabled(rule)} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: rule.enabled ? 'var(--color-accent)' : 'var(--color-text-tertiary)', cursor: 'pointer', flexShrink: 0 }}>{rule.enabled ? '활성' : '비활성'}</button>
                <button onClick={() => setEditingRule(rule)} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', flexShrink: 0 }}>편집</button>
                <button onClick={() => { const next = filterRules.filter((r) => r.id !== rule.id); setFilterRules(next); saveFilterRules(next); }} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer', flexShrink: 0 }}>삭제</button>
              </div>
            ))}
          </div>
          <div style={{ padding: '14px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)' }}>
            <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-secondary)', marginBottom: '12px' }}>{editingRule ? '규칙 편집' : '새 규칙 추가'}</div>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
              <input placeholder="규칙 이름 (선택)" value={cur.name} onChange={(e) => setCur({ name: e.target.value })} style={{ ...ist, width: '100%', padding: '6px 10px', fontSize: '13px' }} />
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>조건 일치:</span>
                {(['and', 'or'] as const).map((l) => (
                  <button key={l} onClick={() => setCur({ logic: l })} style={{ padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: cur.logic === l ? 'var(--color-accent)' : 'transparent', color: cur.logic === l ? '#fff' : 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer' }}>
                    {l === 'and' ? '모두 (AND)' : '하나 이상 (OR)'}
                  </button>
                ))}
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>조건</span>
                {cur.conditions.map((cond, idx) => {
                  const isNoValue = fieldOpts.find((f) => f.value === cond.field)?.noValue;
                  return (
                    <div key={idx} style={{ display: 'flex', gap: '5px', alignItems: 'center' }}>
                      <select value={cond.field} onChange={(e) => { const v = e.target.value as FilterCondition['field']; const updated = cur.conditions.map((c, i) => i === idx ? { ...c, field: v, value: '' } : c); setCur({ conditions: updated }); }} style={sst}>
                        {fieldOpts.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                      </select>
                      {!isNoValue && (
                        <>
                          <select value={cond.matchType} onChange={(e) => { const updated = cur.conditions.map((c, i) => i === idx ? { ...c, matchType: e.target.value as FilterCondition['matchType'] } : c); setCur({ conditions: updated }); }} style={sst}>
                            {matchOpts.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                          </select>
                          <input placeholder={cond.field.startsWith('size') ? '예: 1024' : '예: @naver.com'} value={cond.value} onChange={(e) => { const updated = cur.conditions.map((c, i) => i === idx ? { ...c, value: e.target.value } : c); setCur({ conditions: updated }); }} style={{ ...ist, flex: 1 }} />
                        </>
                      )}
                      {cur.conditions.length > 1 && <button onClick={() => setCur({ conditions: cur.conditions.filter((_, i) => i !== idx) })} style={{ fontSize: '16px', lineHeight: 1, padding: '2px 5px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', flexShrink: 0 }}>×</button>}
                    </div>
                  );
                })}
                <button onClick={() => setCur({ conditions: [...cur.conditions, { field: 'from', matchType: 'contains', value: '' }] })} style={{ alignSelf: 'flex-start', fontSize: '12px', padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-accent)', cursor: 'pointer' }}>+ 조건 추가</button>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '7px' }}>
                <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>작업</span>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={!!cur.action.labelColor} onChange={(e) => setCur({ action: { ...cur.action, labelColor: e.target.checked ? LABEL_COLORS[0] : undefined } })} />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>라벨 색상 적용</span>
                  {cur.action.labelColor && LABEL_COLORS.map((c) => (
                    <button key={c} onClick={(e) => { e.preventDefault(); setCur({ action: { ...cur.action, labelColor: c } }); }} style={{ width: '16px', height: '16px', borderRadius: '50%', background: c, border: cur.action.labelColor === c ? '3px solid var(--color-text-primary)' : '2px solid transparent', cursor: 'pointer', padding: 0, flexShrink: 0 }} />
                  ))}
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={cur.action.moveToFolder !== undefined} onChange={(e) => setCur({ action: { ...cur.action, moveToFolder: e.target.checked ? '' : undefined } })} />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>편지함으로 이동</span>
                  {cur.action.moveToFolder !== undefined && <input placeholder="편지함 이름" value={cur.action.moveToFolder} onChange={(e) => setCur({ action: { ...cur.action, moveToFolder: e.target.value } })} style={{ ...ist, flex: 1 }} />}
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={!!cur.action.markRead} onChange={(e) => setCur({ action: { ...cur.action, markRead: e.target.checked ? true : undefined, markUnread: undefined } })} />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>읽음으로 표시</span>
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={!!cur.action.markUnread} onChange={(e) => setCur({ action: { ...cur.action, markUnread: e.target.checked ? true : undefined, markRead: undefined } })} />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>읽지 않음으로 표시</span>
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={!!cur.action.markStarred} onChange={(e) => setCur({ action: { ...cur.action, markStarred: e.target.checked ? true : undefined } })} />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>별표 표시</span>
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={!!cur.action.markImportant} onChange={(e) => setCur({ action: { ...cur.action, markImportant: e.target.checked ? true : undefined } })} />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>중요 표시</span>
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={!!cur.action.skipInbox} onChange={(e) => setCur({ action: { ...cur.action, skipInbox: e.target.checked ? true : undefined } })} />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>받은편지함 건너뜀 (자동 보관)</span>
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={!!cur.action.deleteMsg} onChange={(e) => setCur({ action: { ...cur.action, deleteMsg: e.target.checked ? true : undefined } })} />
                  <span style={{ fontSize: '12px', color: 'var(--color-destructive)' }}>삭제 (휴지통으로 이동)</span>
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={cur.action.forwardTo !== undefined} onChange={(e) => setCur({ action: { ...cur.action, forwardTo: e.target.checked ? '' : undefined } })} />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>전달</span>
                  {cur.action.forwardTo !== undefined && <input type="email" placeholder="전달받을 이메일 주소" value={cur.action.forwardTo} onChange={(e) => setCur({ action: { ...cur.action, forwardTo: e.target.value } })} style={{ ...ist, flex: 1 }} />}
                </label>
              </div>
              <div style={{ display: 'flex', flexDirection: 'column', gap: '7px', paddingTop: '4px', borderTop: '1px solid var(--color-border-subtle)' }}>
                <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>규칙 옵션</span>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={cur.enabled} onChange={(e) => setCur({ enabled: e.target.checked })} />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>이 규칙 활성화</span>
                </label>
                <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
                  <input type="checkbox" checked={!!cur.stopProcessing} onChange={(e) => setCur({ stopProcessing: e.target.checked ? true : undefined })} />
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>이 규칙 적용 후 이후 규칙 무시</span>
                </label>
              </div>
              <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
                {editingRule && <button onClick={() => { setEditingRule(null); setNewRule(createEmptyRule()); }} style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}>취소</button>}
                <button onClick={() => doSave(cur, editingRule?.id)} style={{ padding: '6px 16px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}>{editingRule ? '저장' : '추가'}</button>
              </div>
            </div>
          </div>
        </>
      );
    }
  }
}
