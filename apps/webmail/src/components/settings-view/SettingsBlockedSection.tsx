'use client';
import React from 'react';
import { useTranslations } from 'next-intl';
import { CheckCircleIcon, GlobeAltIcon, MagnifyingGlassIcon, NoSymbolIcon } from '@heroicons/react/24/outline';
import { setPreferences } from '@/lib/api';
import { Row, SectionCard, SectionHeader, Segment, Toggle } from '@/components/settings-view/settingsViewPrimitives';

interface SettingsBlockedSectionProps {
  blockedSenders: string[];
  setBlockedSenders: (v: string[]) => void;
  blockedMeta: Record<string, string>;
  setBlockedMeta: (v: Record<string, string>) => void;
  newBlockedInput: string;
  setNewBlockedInput: (v: string) => void;
  blockedSearch: string;
  setBlockedSearch: (v: string) => void;
  blockedPage: number;
  setBlockedPage: (v: number) => void;
  spamAutoDeleteDays: number;
  setSpamAutoDeleteDays: (v: number) => void;
  spamAutoBlock: boolean;
  setSpamAutoBlock: (v: boolean) => void;
  allowedSenders: string[];
  setAllowedSenders: (v: string[]) => void;
  allowedMeta: Record<string, string>;
  setAllowedMeta: (v: Record<string, string>) => void;
  newAllowedInput: string;
  setNewAllowedInput: (v: string) => void;
  allowedSearch: string;
  setAllowedSearch: (v: string) => void;
  allowedPage: number;
  setAllowedPage: (v: number) => void;
}

export function SettingsBlockedSection({
  blockedSenders,
  setBlockedSenders,
  blockedMeta,
  setBlockedMeta,
  newBlockedInput,
  setNewBlockedInput,
  blockedSearch,
  setBlockedSearch,
  blockedPage,
  setBlockedPage,
  spamAutoDeleteDays,
  setSpamAutoDeleteDays,
  spamAutoBlock,
  setSpamAutoBlock,
  allowedSenders,
  setAllowedSenders,
  allowedMeta,
  setAllowedMeta,
  newAllowedInput,
  setNewAllowedInput,
  allowedSearch,
  setAllowedSearch,
  allowedPage,
  setAllowedPage,
}: SettingsBlockedSectionProps) {
  const t = useTranslations('settingsView');

  const PAGE_SIZE = 5;
  const q = blockedSearch.trim().toLowerCase();
  const filteredSenders = q ? blockedSenders.filter((a) => a.includes(q)) : blockedSenders;
  const totalPages = Math.ceil(filteredSenders.length / PAGE_SIZE);
  const safePage = Math.min(blockedPage, Math.max(0, totalPages - 1));
  const pageItems = filteredSenders.slice(safePage * PAGE_SIZE, (safePage + 1) * PAGE_SIZE);

  function saveBlocked(next: string[], meta?: Record<string, string>) {
    try { localStorage.setItem('webmail_blocked_senders', JSON.stringify(next)); } catch { /* ignore */ }
    setBlockedSenders(next);
    if (meta !== undefined) {
      try { localStorage.setItem('webmail_blocked_meta', JSON.stringify(meta)); } catch { /* ignore */ }
      setBlockedMeta(meta);
    }
    void setPreferences({ blocked_senders: next });
  }
  function addBlocked() {
    const val = newBlockedInput.trim().toLowerCase();
    if (!val || blockedSenders.includes(val)) return;
    const now = new Date().toISOString();
    const nextMeta = { ...blockedMeta, [val]: now };
    saveBlocked([...blockedSenders, val], nextMeta);
    setNewBlockedInput('');
    // Jump to last page to show newly added entry
    setBlockedPage(Math.floor(blockedSenders.length / PAGE_SIZE));
  }
  function removeBlocked(addr: string) {
    const next = blockedSenders.filter((a) => a !== addr);
    const nextMeta = { ...blockedMeta };
    delete nextMeta[addr];
    saveBlocked(next, nextMeta);
    // Keep page in range
    const newTotal = Math.ceil(next.length / PAGE_SIZE);
    if (safePage >= newTotal && safePage > 0) setBlockedPage(safePage - 1);
  }
  function formatBlockedDate(addr: string): string {
    const iso = blockedMeta[addr];
    if (!iso) return '—';
    try {
      return new Intl.DateTimeFormat(undefined, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(iso));
    } catch { return iso.slice(0, 10); }
  }

  const autoDeleteOptions: { value: number; labelKey: string }[] = [
    { value: 14, labelKey: 'spamDelete14' },
    { value: 30, labelKey: 'spamDelete30' },
    { value: 60, labelKey: 'spamDelete60' },
    { value: 90, labelKey: 'spamDelete90' },
    { value: 0, labelKey: 'spamDeleteNever' },
  ];

  const thSt: React.CSSProperties = {
    padding: '8px 14px', textAlign: 'left', fontSize: '11px', fontWeight: 700,
    letterSpacing: '0.06em', textTransform: 'uppercase',
    color: 'var(--color-text-tertiary)',
    borderBottom: '1px solid var(--color-border-default)',
    whiteSpace: 'nowrap', background: 'var(--color-bg-secondary)',
  };
  const tdSt: React.CSSProperties = {
    padding: '9px 14px', fontSize: '13px',
    color: 'var(--color-text-primary)',
    borderBottom: '1px solid var(--color-border-subtle)',
    verticalAlign: 'middle',
  };

  const aq = allowedSearch.trim().toLowerCase();
  const filteredAllowed = aq ? allowedSenders.filter((a) => a.includes(aq)) : allowedSenders;
  const allowedTotalPages = Math.ceil(filteredAllowed.length / PAGE_SIZE);
  const safeAllowedPage = Math.min(allowedPage, Math.max(0, allowedTotalPages - 1));
  const allowedPageItems = filteredAllowed.slice(safeAllowedPage * PAGE_SIZE, (safeAllowedPage + 1) * PAGE_SIZE);

  function saveAllowed(next: string[], meta?: Record<string, string>) {
    try { localStorage.setItem('webmail_allowed_senders', JSON.stringify(next)); } catch { /* */ }
    setAllowedSenders(next);
    if (meta !== undefined) {
      try { localStorage.setItem('webmail_allowed_meta', JSON.stringify(meta)); } catch { /* */ }
      setAllowedMeta(meta);
    }
    void setPreferences({ allowed_senders: next });
  }
  function addAllowed() {
    const val = newAllowedInput.trim().toLowerCase();
    if (!val || allowedSenders.includes(val)) return;
    const now = new Date().toISOString();
    saveAllowed([...allowedSenders, val], { ...allowedMeta, [val]: now });
    setNewAllowedInput('');
    setAllowedPage(Math.floor(allowedSenders.length / PAGE_SIZE));
  }
  function removeAllowed(addr: string) {
    const next = allowedSenders.filter((a) => a !== addr);
    const nextMeta = { ...allowedMeta };
    delete nextMeta[addr];
    saveAllowed(next, nextMeta);
    const newTotal = Math.ceil(next.filter((a) => aq ? a.includes(aq) : true).length / PAGE_SIZE);
    if (safeAllowedPage >= newTotal && safeAllowedPage > 0) setAllowedPage(safeAllowedPage - 1);
  }
  function formatAllowedDate(addr: string): string {
    const iso = allowedMeta[addr];
    if (!iso) return '—';
    try {
      return new Intl.DateTimeFormat(undefined, { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', hour12: false }).format(new Date(iso));
    } catch { return iso.slice(0, 10); }
  }
  const allowedValTrimmed = newAllowedInput.trim().toLowerCase();

  return (
    <>
      {/* ── 스팸 필터 설정 ── */}
      <SectionCard>
        <SectionHeader>{t('sectionSpamFilter')}</SectionHeader>
        <Row label={t('spamAutoDelete')} description={t('spamAutoDeleteDesc')}>
          <Segment
            value={String(spamAutoDeleteDays)}
            onChange={(v) => {
              const days = Number(v);
              setSpamAutoDeleteDays(days);
              try { localStorage.setItem('webmail_spam_autodelete_days', String(days)); } catch { /* */ }
            }}
            options={autoDeleteOptions.map((o) => ({ value: String(o.value), label: t(o.labelKey) }))}
          />
        </Row>
        <Row label={t('spamAutoBlock')} description={t('spamAutoBlockDesc')}>
          <Toggle
            value={spamAutoBlock}
            onChange={(v) => {
              setSpamAutoBlock(v);
              try { localStorage.setItem('webmail_spam_auto_block', v ? 'true' : 'false'); } catch { /* */ }
            }}
          />
        </Row>
      </SectionCard>

      {/* ── 차단된 발신자 목록 (table + pagination) ── */}
      <SectionCard>
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px', padding: '16px 20px 0', flexWrap: 'wrap' }}>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{t('sectionBlockedSenders')}</div>
            <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{t('blockedSendersDesc')}</div>
          </div>
          {/* Search input */}
          {blockedSenders.length > 0 && (
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
              <div style={{ position: 'relative' }}>
                <MagnifyingGlassIcon style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', width: 13, height: 13, color: 'var(--color-text-tertiary)', pointerEvents: 'none' }} />
                <input
                  type="text"
                  value={blockedSearch}
                  onChange={(e) => { setBlockedSearch(e.target.value); setBlockedPage(0); }}
                  placeholder={t('blockedSearchPlaceholder')}
                  style={{
                    paddingLeft: 26, paddingRight: 8, paddingTop: 5, paddingBottom: 5,
                    width: 190, fontSize: '12px',
                    border: '1px solid var(--color-border-default)',
                    borderRadius: '6px',
                    background: 'var(--color-bg-secondary)',
                    color: 'var(--color-text-primary)',
                    outline: 'none',
                    fontFamily: 'monospace',
                  }}
                  onFocus={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-accent)'; }}
                  onBlur={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-border-default)'; }}
                />
                {blockedSearch && (
                  <button
                    onClick={() => { setBlockedSearch(''); setBlockedPage(0); }}
                    style={{ position: 'absolute', right: 6, top: '50%', transform: 'translateY(-50%)', background: 'none', border: 'none', cursor: 'pointer', padding: 0, color: 'var(--color-text-tertiary)', lineHeight: 1, fontSize: 14 }}
                    aria-label={t('blockedSearchClear')}
                  >×</button>
                )}
              </div>
              <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', whiteSpace: 'nowrap' }}>
                {q ? t('blockedSearchCount', { found: filteredSenders.length, total: blockedSenders.length }) : t('blockedCount', { count: blockedSenders.length })}
              </span>
            </div>
          )}
        </div>

        <div style={{ overflowX: 'auto', margin: '12px 0 0' }}>
          {blockedSenders.length === 0 ? (
            <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>
              {t('noBlocked')}
            </div>
          ) : filteredSenders.length === 0 ? (
            <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>
              {t('blockedSearchEmpty')}
            </div>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse', tableLayout: 'fixed' }}>
              <colgroup>
                <col style={{ width: '40px' }} />
                <col />
                <col style={{ width: '160px' }} />
                <col style={{ width: '72px' }} />
              </colgroup>
              <thead>
                <tr>
                  <th style={thSt} />
                  <th style={thSt}>{t('blockedColAddr')}</th>
                  <th style={thSt}>{t('blockedColDate')}</th>
                  <th style={{ ...thSt, textAlign: 'center' }}>{t('blockedColAction')}</th>
                </tr>
              </thead>
              <tbody>
                {pageItems.map((addr) => {
                  const isDomain = addr.startsWith('@');
                  return (
                    <tr key={addr}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'var(--color-bg-secondary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'transparent'; }}
                    >
                      <td style={{ ...tdSt, textAlign: 'center' }}>
                        {isDomain
                          ? <GlobeAltIcon style={{ width: 14, height: 14, color: 'var(--color-warning)', display: 'inline-block' }} />
                          : <NoSymbolIcon style={{ width: 14, height: 14, color: 'var(--color-destructive)', display: 'inline-block' }} />
                        }
                      </td>
                      <td style={{ ...tdSt, fontFamily: 'monospace', fontSize: '12px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {addr}
                      </td>
                      <td style={{ ...tdSt, fontSize: '12px', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                        {formatBlockedDate(addr)}
                      </td>
                      <td style={{ ...tdSt, textAlign: 'center' }}>
                        <button
                          onClick={() => removeBlocked(addr)}
                          style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer' }}
                          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'color-mix(in srgb, var(--color-destructive) 10%, transparent)'; }}
                          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                        >{t('unblock')}</button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: '6px', padding: '10px 16px', borderTop: '1px solid var(--color-border-subtle)' }}>
            <button
              onClick={() => setBlockedPage((p) => Math.max(0, p - 1))}
              disabled={safePage === 0}
              style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: safePage === 0 ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)', cursor: safePage === 0 ? 'default' : 'pointer', fontSize: '12px' }}
            >‹</button>
            <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', minWidth: '80px', textAlign: 'center' }}>
              {safePage + 1} / {totalPages}
            </span>
            <button
              onClick={() => setBlockedPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={safePage === totalPages - 1}
              style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: safePage === totalPages - 1 ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)', cursor: safePage === totalPages - 1 ? 'default' : 'pointer', fontSize: '12px' }}
            >›</button>
          </div>
        )}
      </SectionCard>

      {/* ── 발신자/도메인 차단 추가 ── */}
      <SectionCard>
        <SectionHeader>{t('sectionAddBlockedSender')}</SectionHeader>
        <div style={{ padding: '4px 20px 8px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
          {t('blockedInputHint')}
        </div>
        <div style={{ padding: '0 20px 20px' }}>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'stretch' }}>
            <div style={{ flex: 1, position: 'relative' }}>
              <input
                value={newBlockedInput}
                onChange={(e) => setNewBlockedInput(e.target.value)}
                onKeyDown={(e) => { if (e.key === 'Enter') addBlocked(); }}
                placeholder={t('blockedInputPlaceholder')}
                style={{
                  width: '100%', boxSizing: 'border-box',
                  padding: '9px 12px',
                  border: '1px solid var(--color-border-default)',
                  borderRadius: '7px',
                  background: 'var(--color-bg-primary)',
                  color: 'var(--color-text-primary)',
                  fontSize: '13px', outline: 'none',
                  fontFamily: 'monospace',
                  transition: 'border-color 120ms',
                }}
                onFocus={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-accent)'; }}
                onBlur={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-border-default)'; }}
              />
            </div>
            <button
              onClick={addBlocked}
              disabled={!newBlockedInput.trim() || blockedSenders.includes(newBlockedInput.trim().toLowerCase())}
              style={{
                padding: '9px 20px', borderRadius: '7px', border: 'none',
                background: 'var(--color-accent)', color: '#fff',
                fontSize: '13px', fontWeight: 600,
                cursor: newBlockedInput.trim() && !blockedSenders.includes(newBlockedInput.trim().toLowerCase()) ? 'pointer' : 'default',
                opacity: newBlockedInput.trim() && !blockedSenders.includes(newBlockedInput.trim().toLowerCase()) ? 1 : 0.4,
                flexShrink: 0, whiteSpace: 'nowrap',
                transition: 'opacity 120ms',
              }}
              onMouseEnter={(e) => { if (!(!newBlockedInput.trim() || blockedSenders.includes(newBlockedInput.trim().toLowerCase()))) (e.currentTarget as HTMLButtonElement).style.opacity = '0.88'; }}
              onMouseLeave={(e) => { if (!(!newBlockedInput.trim() || blockedSenders.includes(newBlockedInput.trim().toLowerCase()))) (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
            >{t('block')}</button>
          </div>
          {newBlockedInput.trim() && blockedSenders.includes(newBlockedInput.trim().toLowerCase()) && (
            <div style={{ marginTop: '6px', fontSize: '12px', color: 'var(--color-warning)' }}>
              {t('blockedAlready')}
            </div>
          )}
        </div>
      </SectionCard>

      {/* ── 허용된 발신자 목록 ── */}
      <SectionCard>
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px', padding: '16px 20px 0', flexWrap: 'wrap' }}>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>{t('sectionAllowedSenders')}</div>
            <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>{t('allowedSendersDesc')}</div>
          </div>
          {allowedSenders.length > 0 && (
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
              <div style={{ position: 'relative' }}>
                <MagnifyingGlassIcon style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', width: 13, height: 13, color: 'var(--color-text-tertiary)', pointerEvents: 'none' }} />
                <input
                  type="text"
                  value={allowedSearch}
                  onChange={(e) => { setAllowedSearch(e.target.value); setAllowedPage(0); }}
                  placeholder={t('blockedSearchPlaceholder')}
                  style={{ paddingLeft: 26, paddingRight: 8, paddingTop: 5, paddingBottom: 5, width: 190, fontSize: '12px', border: '1px solid var(--color-border-default)', borderRadius: '6px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', outline: 'none', fontFamily: 'monospace' }}
                  onFocus={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-accent)'; }}
                  onBlur={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-border-default)'; }}
                />
                {allowedSearch && (
                  <button onClick={() => { setAllowedSearch(''); setAllowedPage(0); }} style={{ position: 'absolute', right: 6, top: '50%', transform: 'translateY(-50%)', background: 'none', border: 'none', cursor: 'pointer', padding: 0, color: 'var(--color-text-tertiary)', lineHeight: 1, fontSize: 14 }}>×</button>
                )}
              </div>
              <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', whiteSpace: 'nowrap' }}>
                {aq ? t('blockedSearchCount', { found: filteredAllowed.length, total: allowedSenders.length }) : t('blockedCount', { count: allowedSenders.length })}
              </span>
            </div>
          )}
        </div>

        <div style={{ overflowX: 'auto', margin: '12px 0 0' }}>
          {allowedSenders.length === 0 ? (
            <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>
              {t('noAllowed')}
            </div>
          ) : filteredAllowed.length === 0 ? (
            <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>
              {t('blockedSearchEmpty')}
            </div>
          ) : (
            <table style={{ width: '100%', borderCollapse: 'collapse', tableLayout: 'fixed' }}>
              <colgroup>
                <col style={{ width: '40px' }} />
                <col />
                <col style={{ width: '160px' }} />
                <col style={{ width: '72px' }} />
              </colgroup>
              <thead>
                <tr>
                  <th style={thSt} />
                  <th style={thSt}>{t('allowedColAddr')}</th>
                  <th style={thSt}>{t('allowedColDate')}</th>
                  <th style={{ ...thSt, textAlign: 'center' }}>{t('blockedColAction')}</th>
                </tr>
              </thead>
              <tbody>
                {allowedPageItems.map((addr) => {
                  const isDomain = addr.startsWith('@');
                  return (
                    <tr key={addr}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'var(--color-bg-secondary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'transparent'; }}
                    >
                      <td style={{ ...tdSt, textAlign: 'center' }}>
                        {isDomain
                          ? <GlobeAltIcon style={{ width: 14, height: 14, color: 'var(--color-accent)', display: 'inline-block' }} />
                          : <CheckCircleIcon style={{ width: 14, height: 14, color: 'var(--color-accent)', display: 'inline-block' }} />
                        }
                      </td>
                      <td style={{ ...tdSt, fontFamily: 'monospace', fontSize: '12px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {addr}
                      </td>
                      <td style={{ ...tdSt, fontSize: '12px', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                        {formatAllowedDate(addr)}
                      </td>
                      <td style={{ ...tdSt, textAlign: 'center' }}>
                        <button
                          onClick={() => removeAllowed(addr)}
                          style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}
                          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                        >{t('disallow')}</button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
        </div>

        {allowedTotalPages > 1 && (
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'flex-end', gap: '6px', padding: '10px 16px', borderTop: '1px solid var(--color-border-subtle)' }}>
            <button onClick={() => setAllowedPage((p) => Math.max(0, p - 1))} disabled={safeAllowedPage === 0} style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: safeAllowedPage === 0 ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)', cursor: safeAllowedPage === 0 ? 'default' : 'pointer', fontSize: '12px' }}>‹</button>
            <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', minWidth: '80px', textAlign: 'center' }}>{safeAllowedPage + 1} / {allowedTotalPages}</span>
            <button onClick={() => setAllowedPage((p) => Math.min(allowedTotalPages - 1, p + 1))} disabled={safeAllowedPage === allowedTotalPages - 1} style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: safeAllowedPage === allowedTotalPages - 1 ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)', cursor: safeAllowedPage === allowedTotalPages - 1 ? 'default' : 'pointer', fontSize: '12px' }}>›</button>
          </div>
        )}
      </SectionCard>

      {/* ── 허용 발신자 추가 ── */}
      <SectionCard>
        <SectionHeader>{t('sectionAddAllowedSender')}</SectionHeader>
        <div style={{ padding: '4px 20px 8px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
          {t('allowedInputHint')}
        </div>
        <div style={{ padding: '0 20px 20px' }}>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'stretch' }}>
            <input
              value={newAllowedInput}
              onChange={(e) => setNewAllowedInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') addAllowed(); }}
              placeholder={t('allowedInputPlaceholder')}
              style={{ flex: 1, boxSizing: 'border-box', padding: '9px 12px', border: '1px solid var(--color-border-default)', borderRadius: '7px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', outline: 'none', fontFamily: 'monospace', transition: 'border-color 120ms' }}
              onFocus={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-accent)'; }}
              onBlur={(e) => { (e.currentTarget as HTMLInputElement).style.borderColor = 'var(--color-border-default)'; }}
            />
            <button
              onClick={addAllowed}
              disabled={!allowedValTrimmed || allowedSenders.includes(allowedValTrimmed)}
              style={{ padding: '9px 20px', borderRadius: '7px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: allowedValTrimmed && !allowedSenders.includes(allowedValTrimmed) ? 'pointer' : 'default', opacity: allowedValTrimmed && !allowedSenders.includes(allowedValTrimmed) ? 1 : 0.4, flexShrink: 0, whiteSpace: 'nowrap', transition: 'opacity 120ms' }}
              onMouseEnter={(e) => { if (allowedValTrimmed && !allowedSenders.includes(allowedValTrimmed)) (e.currentTarget as HTMLButtonElement).style.opacity = '0.88'; }}
              onMouseLeave={(e) => { if (allowedValTrimmed && !allowedSenders.includes(allowedValTrimmed)) (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
            >{t('allow')}</button>
          </div>
          {allowedValTrimmed && allowedSenders.includes(allowedValTrimmed) && (
            <div style={{ marginTop: '6px', fontSize: '12px', color: 'var(--color-warning)' }}>
              {t('allowedAlready')}
            </div>
          )}
        </div>
      </SectionCard>
    </>
  );
}
