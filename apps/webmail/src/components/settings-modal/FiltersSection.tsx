'use client';

import { type Dispatch, type SetStateAction } from 'react';
import { useTranslations } from 'next-intl';
import {
  type FilterAction,
  type FilterCondition,
  type FilterRule,
  LABEL_COLORS,
  createEmptyRule,
  saveFilterRules,
} from '../settings/settingsConfig';
import { stableId } from '@/lib/stableId';
import { labelStyle, sectionStyle } from './sharedStyles';

interface Props {
  filterRules: FilterRule[];
  setFilterRules: Dispatch<SetStateAction<FilterRule[]>>;
  editingRule: FilterRule | null;
  setEditingRule: Dispatch<SetStateAction<FilterRule | null>>;
  newRule: Omit<FilterRule, 'id'>;
  setNewRule: Dispatch<SetStateAction<Omit<FilterRule, 'id'>>>;
}

export function FiltersSection({
  filterRules,
  setFilterRules,
  editingRule,
  setEditingRule,
  newRule,
  setNewRule,
}: Props) {
  const tFilter = useTranslations('filterRules');

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
    if (a.labelColor) parts.push(tFilter('badgeLabel'));
    if (a.moveToFolder) parts.push(`→ ${a.moveToFolder}`);
    if (a.markRead) parts.push(tFilter('badgeRead'));
    if (a.markUnread) parts.push(tFilter('badgeUnread'));
    if (a.markStarred) parts.push(tFilter('badgeStar'));
    if (a.markImportant) parts.push(tFilter('badgeImportant'));
    if (a.skipInbox) parts.push(tFilter('badgeSkipInbox'));
    if (a.deleteMsg) parts.push(tFilter('badgeDelete'));
    if (a.forwardTo) parts.push(`${tFilter('actionForward')} → ${a.forwardTo}`);
    return parts.join(' · ') || tFilter('badgeNoAction');
  };

  return (
    <>
      <div style={sectionStyle}>
        <span style={labelStyle}>{tFilter('sectionTitle')}</span>
        <p style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginBottom: '12px' }}>{tFilter('sectionDesc')}</p>
        {filterRules.length === 0 && <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', padding: '12px 0' }}>{tFilter('noRules')}</div>}
        {filterRules.map((rule, idx) => (
          <div key={rule.id} style={{ display: 'flex', alignItems: 'flex-start', gap: '8px', padding: '10px 12px', borderRadius: '8px', background: rule.enabled ? 'var(--color-bg-secondary)' : 'transparent', marginBottom: '4px', border: `1px solid ${rule.enabled ? 'var(--color-border-subtle)' : 'var(--color-border-subtle)'}`, opacity: rule.enabled ? 1 : 0.5 }}>
            <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0, width: '16px', textAlign: 'center', marginTop: '2px' }}>{idx + 1}</span>
            {rule.action.labelColor && <span style={{ width: '10px', height: '10px', borderRadius: '50%', background: rule.action.labelColor, flexShrink: 0, marginTop: '4px', display: 'inline-block' }} />}
            <div style={{ flex: 1, minWidth: 0 }}>
              {rule.name && <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-primary)', marginBottom: '2px' }}>{rule.name}</div>}
              <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{condSummary(rule)}</div>
              <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{actionBadges(rule.action)}{rule.stopProcessing ? ` · [${tFilter('badgeStop')}]` : ''}</div>
            </div>
            <button onClick={() => toggleEnabled(rule)} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: rule.enabled ? 'var(--color-accent)' : 'var(--color-text-tertiary)', cursor: 'pointer', flexShrink: 0 }}>{rule.enabled ? tFilter('active') : tFilter('inactive')}</button>
            <button onClick={() => setEditingRule(rule)} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', flexShrink: 0 }}>{tFilter('edit')}</button>
            <button onClick={() => { const next = filterRules.filter((r) => r.id !== rule.id); setFilterRules(next); saveFilterRules(next); }} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer', flexShrink: 0 }}>{tFilter('delete')}</button>
          </div>
        ))}
      </div>
      <div style={{ padding: '14px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)' }}>
        <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-secondary)', marginBottom: '12px' }}>{editingRule ? tFilter('editRuleTitle') : tFilter('newRuleTitle')}</div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <input placeholder={tFilter('ruleNamePlaceholder')} value={cur.name} onChange={(e) => setCur({ name: e.target.value })} style={{ ...ist, width: '100%', padding: '6px 10px', fontSize: '13px' }} />
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>{tFilter('conditionMatch')}</span>
            {(['and', 'or'] as const).map((l) => (
              <button key={l} onClick={() => setCur({ logic: l })} style={{ padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: cur.logic === l ? 'var(--color-accent)' : 'transparent', color: cur.logic === l ? '#fff' : 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer' }}>
                {l === 'and' ? tFilter('matchAll') : tFilter('matchAny')}
              </button>
            ))}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>{tFilter('conditionsHeader')}</span>
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
                      <input placeholder={cond.field.startsWith('size') ? tFilter('sizePlaceholder') : tFilter('valuePlaceholder')} value={cond.value} onChange={(e) => { const updated = cur.conditions.map((c, i) => i === idx ? { ...c, value: e.target.value } : c); setCur({ conditions: updated }); }} style={{ ...ist, flex: 1 }} />
                    </>
                  )}
                  {cur.conditions.length > 1 && <button onClick={() => setCur({ conditions: cur.conditions.filter((_, i) => i !== idx) })} style={{ fontSize: '16px', lineHeight: 1, padding: '2px 5px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', flexShrink: 0 }}>×</button>}
                </div>
              );
            })}
            <button onClick={() => setCur({ conditions: [...cur.conditions, { field: 'from', matchType: 'contains', value: '' }] })} style={{ alignSelf: 'flex-start', fontSize: '12px', padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-accent)', cursor: 'pointer' }}>{tFilter('addCondition')}</button>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '7px' }}>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>{tFilter('actionsHeader')}</span>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.labelColor} onChange={(e) => setCur({ action: { ...cur.action, labelColor: e.target.checked ? LABEL_COLORS[0] : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{tFilter('actionLabelColor')}</span>
              {cur.action.labelColor && LABEL_COLORS.map((c) => (
                <button key={c} onClick={(e) => { e.preventDefault(); setCur({ action: { ...cur.action, labelColor: c } }); }} style={{ width: '16px', height: '16px', borderRadius: '50%', background: c, border: cur.action.labelColor === c ? '3px solid var(--color-text-primary)' : '2px solid transparent', cursor: 'pointer', padding: 0, flexShrink: 0 }} />
              ))}
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={cur.action.moveToFolder !== undefined} onChange={(e) => setCur({ action: { ...cur.action, moveToFolder: e.target.checked ? '' : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>{tFilter('actionMoveToFolder')}</span>
              {cur.action.moveToFolder !== undefined && <input placeholder={tFilter('folderNamePlaceholder')} value={cur.action.moveToFolder} onChange={(e) => setCur({ action: { ...cur.action, moveToFolder: e.target.value } })} style={{ ...ist, flex: 1 }} />}
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.markRead} onChange={(e) => setCur({ action: { ...cur.action, markRead: e.target.checked ? true : undefined, markUnread: undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{tFilter('actionMarkRead')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.markUnread} onChange={(e) => setCur({ action: { ...cur.action, markUnread: e.target.checked ? true : undefined, markRead: undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{tFilter('actionMarkUnread')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.markStarred} onChange={(e) => setCur({ action: { ...cur.action, markStarred: e.target.checked ? true : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{tFilter('actionMarkStarred')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.markImportant} onChange={(e) => setCur({ action: { ...cur.action, markImportant: e.target.checked ? true : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{tFilter('actionMarkImportant')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.skipInbox} onChange={(e) => setCur({ action: { ...cur.action, skipInbox: e.target.checked ? true : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{tFilter('actionSkipInbox')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.deleteMsg} onChange={(e) => setCur({ action: { ...cur.action, deleteMsg: e.target.checked ? true : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-destructive)' }}>{tFilter('actionDelete')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={cur.action.forwardTo !== undefined} onChange={(e) => setCur({ action: { ...cur.action, forwardTo: e.target.checked ? '' : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>{tFilter('actionForward')}</span>
              {cur.action.forwardTo !== undefined && <input type="email" placeholder={tFilter('forwardEmailPlaceholder')} value={cur.action.forwardTo} onChange={(e) => setCur({ action: { ...cur.action, forwardTo: e.target.value } })} style={{ ...ist, flex: 1 }} />}
            </label>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '7px', paddingTop: '4px', borderTop: '1px solid var(--color-border-subtle)' }}>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>{tFilter('ruleOptionsHeader')}</span>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={cur.enabled} onChange={(e) => setCur({ enabled: e.target.checked })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{tFilter('enableRule')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.stopProcessing} onChange={(e) => setCur({ stopProcessing: e.target.checked ? true : undefined })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{tFilter('stopProcessing')}</span>
            </label>
          </div>
          <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
            {editingRule && <button onClick={() => { setEditingRule(null); setNewRule(createEmptyRule()); }} style={{ padding: '6px 14px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}>{tFilter('cancel')}</button>}
            <button onClick={() => doSave(cur, editingRule?.id)} style={{ padding: '6px 16px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}>{editingRule ? tFilter('save') : tFilter('add')}</button>
          </div>
        </div>
      </div>
    </>
  );
}
