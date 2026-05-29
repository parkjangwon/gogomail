'use client';

import { useState, useEffect, useCallback } from 'react';
import { useTranslations } from 'next-intl';
import { LABEL_COLORS, type FilterAction, type FilterCondition, type FilterRule, saveFilterRules } from '@/lib/settings/settingsUtils';
import { SectionCard, SectionHeader } from '@/components/settings-view/settingsViewPrimitives';
import { stableId } from '@/lib/stableId';
import { type Folder, type MessageSummary, getFolders, getMessages, markRead, starMessage, moveMessage, deleteMessage } from '@/lib/api';

interface FilterRulesSectionProps {
  filterRules: FilterRule[];
  setFilterRules: (rules: FilterRule[]) => void;
}

function emptyRule(): Omit<FilterRule, 'id'> {
  return { name: '', enabled: true, logic: 'and', conditions: [{ field: 'from', matchType: 'contains', value: '' }], action: {} };
}

// ─── Condition matching (MessageSummary level) ─────────────────────────────

function matchStr(text: string, type: FilterCondition['matchType'], val: string): boolean {
  const t = text.toLowerCase();
  const v = val.toLowerCase();
  switch (type) {
    case 'contains': return t.includes(v);
    case 'not_contains': return !t.includes(v);
    case 'equals': return t === v;
    case 'starts_with': return t.startsWith(v);
    case 'ends_with': return t.endsWith(v);
    case 'regex': try { return new RegExp(val, 'i').test(text); } catch { return false; }
  }
}

function messageMatchesCondition(msg: MessageSummary, cond: FilterCondition): boolean {
  switch (cond.field) {
    case 'has_attachment': return !!msg.has_attachment;
    case 'is_unread': return !msg.read;
    case 'from': return matchStr(`${msg.from_name ?? ''} ${msg.from_addr}`, cond.matchType, cond.value);
    case 'subject': return matchStr(msg.subject ?? '', cond.matchType, cond.value);
    case 'size_larger': return msg.size > parseInt(cond.value || '0') * 1024;
    case 'size_smaller': return msg.size < parseInt(cond.value || '0') * 1024;
    // to, cc, body require full message — skip at summary level
    case 'to':
    case 'cc':
    case 'body': return true; // optimistic: let pass, can't check from summary
  }
}

function messageMatchesRule(msg: MessageSummary, rule: FilterRule): boolean {
  if (rule.conditions.length === 0) return false;
  return rule.logic === 'and'
    ? rule.conditions.every((c) => messageMatchesCondition(msg, c))
    : rule.conditions.some((c) => messageMatchesCondition(msg, c));
}

// ─── Component ──────────────────────────────────────────────────────────────

export function FilterRulesSection({ filterRules, setFilterRules }: FilterRulesSectionProps) {
  const t = useTranslations('filterRules');
  const tSidebar = useTranslations('sidebar');
  const [editingRule, setEditingRule] = useState<FilterRule | null>(null);
  const [newRule, setNewRule] = useState<Omit<FilterRule, 'id'>>(emptyRule);
  const [folders, setFolders] = useState<Folder[]>([]);
  const [folderLoadError, setFolderLoadError] = useState(false);
  const [applyingId, setApplyingId] = useState<string | null>(null);
  const [applyResult, setApplyResult] = useState<Record<string, string>>({});

  useEffect(() => {
    getFolders()
      .then((d) => setFolders(d.folders))
      .catch((err) => {
        setFolderLoadError(true);
        console.error('Failed to load folders for filter rules:', err instanceof Error ? err.message : err);
      });
  }, []);

  // Folders usable as move targets (exclude trash/spam)
  const movableFolders = folders.filter((f) => f.system_type !== 'trash' && f.system_type !== 'spam');

  const fieldOpts: { value: FilterCondition['field']; label: string; noValue?: boolean }[] = [
    { value: 'from', label: t('fieldFrom') },
    { value: 'to', label: t('fieldTo') },
    { value: 'cc', label: t('fieldCc') },
    { value: 'subject', label: t('fieldSubject') },
    { value: 'body', label: t('fieldBody') },
    { value: 'has_attachment', label: t('fieldHasAttachment'), noValue: true },
    { value: 'is_unread', label: t('fieldIsUnread'), noValue: true },
    { value: 'size_larger', label: t('fieldSizeLarger') },
    { value: 'size_smaller', label: t('fieldSizeSmaller') },
  ];
  const matchOpts: { value: FilterCondition['matchType']; label: string }[] = [
    { value: 'contains', label: t('opContains') },
    { value: 'not_contains', label: t('opNotContains') },
    { value: 'equals', label: t('opEquals') },
    { value: 'starts_with', label: t('opStartsWith') },
    { value: 'ends_with', label: t('opEndsWith') },
    { value: 'regex', label: t('opRegex') },
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

  const SYSTEM_TYPE_KEYS: Record<string, string> = {
    inbox: 'system.inbox',
    sent: 'system.sent',
    drafts: 'system.drafts',
    trash: 'system.trash',
    spam: 'system.spam',
    archive: 'system.archive',
  };

  const localizedFolderName = useCallback((f: Folder): string => {
    if (f.system_type && SYSTEM_TYPE_KEYS[f.system_type]) {
      try { return tSidebar(SYSTEM_TYPE_KEYS[f.system_type] as Parameters<typeof tSidebar>[0]); } catch { /* */ }
    }
    return f.name || f.full_path;
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [tSidebar]);

  const folderName = useCallback((id: string): string => {
    const f = folders.find((x) => x.id === id);
    return f ? localizedFolderName(f) : id;
  }, [folders, localizedFolderName]);

  const actionBadges = (a: FilterAction) => {
    const parts: string[] = [];
    if (a.labelColor) parts.push(t('badgeLabel'));
    if (a.moveToFolder) parts.push(`→ ${folderName(a.moveToFolder)}`);
    if (a.markRead) parts.push(t('badgeRead'));
    if (a.markUnread) parts.push(t('badgeUnread'));
    if (a.markStarred) parts.push(t('badgeStar'));
    if (a.markImportant) parts.push(t('badgeImportant'));
    if (a.skipInbox) parts.push(t('badgeSkipInbox'));
    if (a.deleteMsg) parts.push(t('badgeDelete'));
    if (a.forwardTo) parts.push(`${t('actionForward')} → ${a.forwardTo}`);
    return parts.join(' · ') || t('badgeNoAction');
  };

  // ─── Apply rule to existing messages ──────────────────────────────────────

  const applyRule = useCallback(async (rule: FilterRule) => {
    if (applyingId) return;
    setApplyingId(rule.id);
    setApplyResult((prev) => ({ ...prev, [rule.id]: '' }));

    const targetFolderId = rule.action.moveToFolder
      ? (folders.find((f) => f.id === rule.action.moveToFolder)?.id ?? null)
      : null;

    let matched = 0;
    let cursor = '';

    try {
      // Fetch all messages (all folders) page by page
      while (true) {
        const data = await getMessages('', cursor, 200);
        const msgs: MessageSummary[] = data.messages ?? [];

        // Filter matching messages
        const hits = msgs.filter((m) => messageMatchesRule(m, rule));

        // Apply actions concurrently per message
        await Promise.allSettled(
          hits.map(async (msg) => {
            const ops: Promise<unknown>[] = [];
            if (rule.action.markRead !== undefined) ops.push(markRead(msg.id, !!rule.action.markRead));
            if (rule.action.markUnread) ops.push(markRead(msg.id, false));
            if (rule.action.markStarred && !msg.starred) ops.push(starMessage(msg.id, true));
            if (targetFolderId && msg.folder_id !== targetFolderId) ops.push(moveMessage(msg.id, targetFolderId));
            if (rule.action.deleteMsg) ops.push(deleteMessage(msg.id));
            await Promise.allSettled(ops);
          })
        );

        matched += hits.length;
        if (!data.has_more) break;
        cursor = data.next_cursor;
      }
    } catch {
      // best-effort
    }

    setApplyingId(null);
    setApplyResult((prev) => ({
      ...prev,
      [rule.id]: matched > 0 ? t('applyDone', { count: matched }) : t('applyDoneNone'),
    }));
  }, [applyingId, folders, t]);

  return (
    <>
      <div style={{ background: '#eff6ff', border: '1px solid #bfdbfe', borderRadius: '8px', padding: '12px 16px', marginBottom: '16px', fontSize: '13px', color: '#1e40af', display: 'flex', alignItems: 'flex-start', gap: '8px' }}>
        <span style={{ flexShrink: 0 }}>i</span>
        <span>{t('infoNote')}</span>
      </div>
      <SectionCard>
        <SectionHeader>{t('sectionTitle')}</SectionHeader>
        <div style={{ padding: '0 20px 12px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
          {t('sectionDesc')}
        </div>
        {filterRules.length === 0 && (
          <div style={{ padding: '8px 20px 16px', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>{t('noRules')}</div>
        )}
        {filterRules.map((rule, idx) => (
          <div key={rule.id} style={{ borderTop: idx === 0 ? 'none' : '1px solid var(--color-border-subtle)' }}>
            <div style={{ display: 'flex', alignItems: 'flex-start', gap: '8px', padding: '10px 20px', opacity: rule.enabled ? 1 : 0.5 }}>
              <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0, width: '16px', textAlign: 'center', marginTop: '2px' }}>{idx + 1}</span>
              {rule.action.labelColor && <span style={{ width: '10px', height: '10px', borderRadius: '50%', background: rule.action.labelColor, flexShrink: 0, marginTop: '4px', display: 'inline-block' }} />}
              <div style={{ flex: 1, minWidth: 0 }}>
                {rule.name && <div style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-text-primary)', marginBottom: '2px' }}>{rule.name}</div>}
                <div style={{ fontSize: '12px', color: 'var(--color-text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{condSummary(rule)}</div>
                <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{actionBadges(rule.action)}{rule.stopProcessing ? ` · [${t('badgeStop')}]` : ''}</div>
              </div>
              <button onClick={() => toggleEnabled(rule)} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: rule.enabled ? 'var(--color-accent)' : 'var(--color-text-tertiary)', cursor: 'pointer', flexShrink: 0 }}>{rule.enabled ? t('active') : t('inactive')}</button>
              <button onClick={() => setEditingRule(rule)} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer', flexShrink: 0 }}>{t('edit')}</button>
              <button onClick={() => { const next = filterRules.filter((r) => r.id !== rule.id); setFilterRules(next); saveFilterRules(next); }} style={{ fontSize: '11px', padding: '2px 7px', borderRadius: '4px', border: 'none', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer', flexShrink: 0 }}>{t('delete')}</button>
            </div>
            {/* Apply Now row */}
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '4px 20px 10px', paddingLeft: '44px' }}>
              <button
                onClick={() => applyRule(rule)}
                disabled={applyingId === rule.id}
                style={{
                  fontSize: '11px', padding: '3px 10px', borderRadius: '4px',
                  border: '1px solid var(--color-border-default)',
                  background: applyingId === rule.id ? 'var(--color-bg-secondary)' : 'transparent',
                  color: applyingId === rule.id ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)',
                  cursor: applyingId === rule.id ? 'default' : 'pointer',
                  flexShrink: 0,
                }}
              >
                {applyingId === rule.id ? t('applying') : t('applyNow')}
              </button>
              {applyResult[rule.id] && (
                <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>{applyResult[rule.id]}</span>
              )}
            </div>
          </div>
        ))}
      </SectionCard>

      <SectionCard>
        <SectionHeader>{editingRule ? t('editRuleTitle') : t('newRuleTitle')}</SectionHeader>
        <div style={{ padding: '0 20px 20px', display: 'flex', flexDirection: 'column', gap: '12px' }}>
          <input placeholder={t('ruleNamePlaceholder')} value={cur.name} onChange={(e) => setCur({ name: e.target.value })} style={{ ...ist, width: '100%', padding: '6px 10px', fontSize: '13px' }} />
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>{t('conditionMatch')}</span>
            {(['and', 'or'] as const).map((l) => (
              <button key={l} onClick={() => setCur({ logic: l })} style={{ padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: cur.logic === l ? 'var(--color-accent)' : 'transparent', color: cur.logic === l ? '#fff' : 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer' }}>
                {l === 'and' ? t('matchAll') : t('matchAny')}
              </button>
            ))}
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>{t('conditionsHeader')}</span>
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
                      <input placeholder={cond.field.startsWith('size') ? t('sizePlaceholder') : t('valuePlaceholder')} value={cond.value} onChange={(e) => { const updated = cur.conditions.map((c, i) => i === idx ? { ...c, value: e.target.value } : c); setCur({ conditions: updated }); }} style={{ ...ist, flex: 1 }} />
                    </>
                  )}
                  {cur.conditions.length > 1 && <button onClick={() => setCur({ conditions: cur.conditions.filter((_, i) => i !== idx) })} style={{ fontSize: '16px', lineHeight: 1, padding: '2px 5px', border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', flexShrink: 0 }}>×</button>}
                </div>
              );
            })}
            <button onClick={() => setCur({ conditions: [...cur.conditions, { field: 'from', matchType: 'contains', value: '' }] })} style={{ alignSelf: 'flex-start', fontSize: '12px', padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-accent)', cursor: 'pointer' }}>{t('addCondition')}</button>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '7px' }}>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>{t('actionsHeader')}</span>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.labelColor} onChange={(e) => setCur({ action: { ...cur.action, labelColor: e.target.checked ? LABEL_COLORS[0] : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{t('actionLabelColor')}</span>
              {cur.action.labelColor && LABEL_COLORS.map((c) => (
                <button key={c} onClick={(e) => { e.preventDefault(); setCur({ action: { ...cur.action, labelColor: c } }); }} style={{ width: '16px', height: '16px', borderRadius: '50%', background: c, border: cur.action.labelColor === c ? '3px solid var(--color-text-primary)' : '2px solid transparent', cursor: 'pointer', padding: 0, flexShrink: 0 }} />
              ))}
            </label>
            {/* Move to folder — select dropdown */}
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={cur.action.moveToFolder !== undefined} onChange={(e) => setCur({ action: { ...cur.action, moveToFolder: e.target.checked ? (movableFolders[0]?.id ?? '') : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>{t('actionMoveToFolder')}</span>
              {cur.action.moveToFolder !== undefined && (
                <select
                  value={cur.action.moveToFolder}
                  onChange={(e) => setCur({ action: { ...cur.action, moveToFolder: e.target.value } })}
                  style={{ ...sst, flex: 1 }}
                >
                  {movableFolders.length === 0 && !folderLoadError && (
                    <option value="">{t('folderNamePlaceholder')}</option>
                  )}
                  {movableFolders.map((f) => (
                    <option key={f.id} value={f.id}>{localizedFolderName(f)}</option>
                  ))}
                </select>
              )}
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.markRead} onChange={(e) => setCur({ action: { ...cur.action, markRead: e.target.checked ? true : undefined, markUnread: undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{t('actionMarkRead')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.markUnread} onChange={(e) => setCur({ action: { ...cur.action, markUnread: e.target.checked ? true : undefined, markRead: undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{t('actionMarkUnread')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.markStarred} onChange={(e) => setCur({ action: { ...cur.action, markStarred: e.target.checked ? true : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{t('actionMarkStarred')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.markImportant} onChange={(e) => setCur({ action: { ...cur.action, markImportant: e.target.checked ? true : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{t('actionMarkImportant')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.skipInbox} onChange={(e) => setCur({ action: { ...cur.action, skipInbox: e.target.checked ? true : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{t('actionSkipInbox')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.action.deleteMsg} onChange={(e) => setCur({ action: { ...cur.action, deleteMsg: e.target.checked ? true : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-destructive)' }}>{t('actionDelete')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={cur.action.forwardTo !== undefined} onChange={(e) => setCur({ action: { ...cur.action, forwardTo: e.target.checked ? '' : undefined } })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', flexShrink: 0 }}>{t('actionForward')}</span>
              {cur.action.forwardTo !== undefined && <input type="email" placeholder={t('forwardEmailPlaceholder')} value={cur.action.forwardTo} onChange={(e) => setCur({ action: { ...cur.action, forwardTo: e.target.value } })} style={{ ...ist, flex: 1 }} />}
            </label>
          </div>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '7px', paddingTop: '4px', borderTop: '1px solid var(--color-border-subtle)' }}>
            <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.06em' }}>{t('ruleOptionsHeader')}</span>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={cur.enabled} onChange={(e) => setCur({ enabled: e.target.checked })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{t('enableRule')}</span>
            </label>
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', cursor: 'pointer' }}>
              <input type="checkbox" checked={!!cur.stopProcessing} onChange={(e) => setCur({ stopProcessing: e.target.checked ? true : undefined })} />
              <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)' }}>{t('stopProcessing')}</span>
            </label>
          </div>
          <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end' }}>
            {editingRule && <button onClick={() => { setEditingRule(null); setNewRule(emptyRule()); }} style={{ padding: '6px 16px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}>{t('cancel')}</button>}
            <button onClick={() => doSave(cur, editingRule?.id)} style={{ padding: '6px 18px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}>{editingRule ? t('save') : t('add')}</button>
          </div>
        </div>
      </SectionCard>
    </>
  );
}
