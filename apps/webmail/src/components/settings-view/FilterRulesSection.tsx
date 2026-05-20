'use client';

import { useState } from 'react';
import { LABEL_COLORS, type FilterAction, type FilterCondition, type FilterRule, saveFilterRules } from '@/lib/settings/settingsUtils';
import { SectionCard, SectionHeader, Toggle } from '@/components/settings-view/settingsViewPrimitives';
import { stableId } from '@/lib/stableId';

interface FilterRulesSectionProps {
  filterRules: FilterRule[];
  setFilterRules: (rules: FilterRule[]) => void;
}

function emptyRule(): Omit<FilterRule, 'id'> {
  return { name: '', enabled: true, logic: 'and', conditions: [{ field: 'from', matchType: 'contains', value: '' }], action: {} };
}

export function FilterRulesSection({ filterRules, setFilterRules }: FilterRulesSectionProps) {
  const [editingRule, setEditingRule] = useState<FilterRule | null>(null);
  const [newRule, setNewRule] = useState<Omit<FilterRule, 'id'>>(emptyRule);

  const fieldOpts: { value: FilterCondition['field']; label: string; noValue?: boolean }[] = [
    { value: 'from', label: '보낸사람' },
    { value: 'to', label: '받는사람' },
    { value: 'cc', label: '참조(CC)' },
    { value: 'subject', label: '제목' },
    { value: 'body', label: '본문' },
    { value: 'has_attachment', label: '첨부파일 있음', noValue: true },
    { value: 'is_unread', label: '읽지 않음', noValue: true },
    { value: 'size_larger', label: '크기 초과(KB)' },
    { value: 'size_smaller', label: '크기 이하(KB)' },
  ];
  const matchOpts: { value: FilterCondition['matchType']; label: string }[] = [
    { value: 'contains', label: '포함' },
    { value: 'not_contains', label: '포함 안 함' },
    { value: 'equals', label: '일치' },
    { value: 'starts_with', label: '시작' },
    { value: 'ends_with', label: '끝' },
    { value: 'regex', label: '정규식' },
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
    setNewRule(emptyRule());
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
      <div style={{ background: '#fefce8', border: '1px solid #fde68a', borderRadius: '8px', padding: '12px 16px', marginBottom: '16px', fontSize: '13px', color: '#92400e', display: 'flex', alignItems: 'flex-start', gap: '8px' }}>
        <span style={{ flexShrink: 0 }}>⚠️</span>
        <span>필터 규칙은 현재 개발 중입니다. 설정은 저장되지만 실제 메일 필터링은 적용되지 않습니다.</span>
      </div>
      <SectionCard>
        <SectionHeader>메일 필터 규칙</SectionHeader>
        <div style={{ padding: '0 20px 12px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
          조건에 맞는 수신 메일에 자동으로 작업을 적용합니다. 위에서부터 순서대로 평가됩니다.
        </div>
        {filterRules.length === 0 && (
          <div style={{ padding: '8px 20px 16px', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>필터 규칙이 없습니다. 아래에서 새 규칙을 추가하세요.</div>
        )}
        {filterRules.map((rule, idx) => (
          <div key={rule.id} style={{ display: 'flex', alignItems: 'flex-start', gap: '8px', padding: '10px 20px', borderTop: idx === 0 ? 'none' : '1px solid var(--color-border-subtle)', opacity: rule.enabled ? 1 : 0.5 }}>
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
      </SectionCard>

      <SectionCard>
        <SectionHeader>{editingRule ? '규칙 편집' : '새 규칙 추가'}</SectionHeader>
        <div style={{ padding: '0 20px 20px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
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
            {editingRule && <button onClick={() => { setEditingRule(null); setNewRule(emptyRule()); }} style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}>취소</button>}
            <button onClick={() => doSave(cur, editingRule?.id)} style={{ padding: '6px 18px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}>{editingRule ? '저장' : '추가'}</button>
          </div>
        </div>
      </SectionCard>
    </>
  );
}
