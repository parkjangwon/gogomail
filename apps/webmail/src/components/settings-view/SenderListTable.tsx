'use client';
import React from 'react';
import { useTranslations } from 'next-intl';
import { CheckCircleIcon, GlobeAltIcon, MagnifyingGlassIcon, NoSymbolIcon } from '@heroicons/react/24/outline';
import { SectionCard, SectionHeader } from '@/components/settings-view/settingsViewPrimitives';

interface SenderListTableProps {
  // Variant identification
  variant: 'blocked' | 'allowed';
  // Data
  senders: string[];
  meta: Record<string, string>; // addr → ISO date
  // Controls
  search: string;
  setSearch: (v: string) => void;
  setPage: React.Dispatch<React.SetStateAction<number>>;
  newInput: string;
  setNewInput: (v: string) => void;
  // Actions
  onAdd: () => void;
  onRemove: (addr: string) => void;
  formatDate: (addr: string) => string;
  // Derived
  filteredSenders: string[];
  pageItems: string[];
  totalPages: number;
  safePage: number;
}

export function SenderListTable({
  variant,
  senders,
  search,
  setSearch,
  setPage,
  newInput,
  setNewInput,
  onAdd,
  onRemove,
  formatDate,
  filteredSenders,
  pageItems,
  totalPages,
  safePage,
}: SenderListTableProps) {
  const t = useTranslations('settingsView');
  const isBlocked = variant === 'blocked';
  const q = search.trim().toLowerCase();
  const valTrimmed = newInput.trim().toLowerCase();

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

  return (
    <>
      {/* ── List table ── */}
      <SectionCard>
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px', padding: '16px 20px 0', flexWrap: 'wrap' }}>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>
              {t(isBlocked ? 'sectionBlockedSenders' : 'sectionAllowedSenders')}
            </div>
            <div style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>
              {t(isBlocked ? 'blockedSendersDesc' : 'allowedSendersDesc')}
            </div>
          </div>
          {/* Search input */}
          {senders.length > 0 && (
            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
              <div style={{ position: 'relative' }}>
                <MagnifyingGlassIcon style={{ position: 'absolute', left: 8, top: '50%', transform: 'translateY(-50%)', width: 13, height: 13, color: 'var(--color-text-tertiary)', pointerEvents: 'none' }} />
                <input
                  type="text"
                  value={search}
                  onChange={(e) => { setSearch(e.target.value); setPage(0); }}
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
                {search && (
                  <button
                    onClick={() => { setSearch(''); setPage(0); }}
                    style={{ position: 'absolute', right: 6, top: '50%', transform: 'translateY(-50%)', background: 'none', border: 'none', cursor: 'pointer', padding: 0, color: 'var(--color-text-tertiary)', lineHeight: 1, fontSize: 14 }}
                    aria-label={t('blockedSearchClear')}
                  >×</button>
                )}
              </div>
              <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', whiteSpace: 'nowrap' }}>
                {q
                  ? t('blockedSearchCount', { found: filteredSenders.length, total: senders.length })
                  : t('blockedCount', { count: senders.length })}
              </span>
            </div>
          )}
        </div>

        <div style={{ overflowX: 'auto', margin: '12px 0 0' }}>
          {senders.length === 0 ? (
            <div style={{ padding: '20px', textAlign: 'center', fontSize: '13px', color: 'var(--color-text-tertiary)' }}>
              {t(isBlocked ? 'noBlocked' : 'noAllowed')}
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
                  <th style={thSt}>{t(isBlocked ? 'blockedColAddr' : 'allowedColAddr')}</th>
                  <th style={thSt}>{t(isBlocked ? 'blockedColDate' : 'allowedColDate')}</th>
                  <th style={{ ...thSt, textAlign: 'center' }}>{t('blockedColAction')}</th>
                </tr>
              </thead>
              <tbody>
                {pageItems.map((addr) => {
                  const isDomain = addr.startsWith('@');
                  return (
                    <tr
                      key={addr}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'var(--color-bg-secondary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLTableRowElement).style.background = 'transparent'; }}
                    >
                      <td style={{ ...tdSt, textAlign: 'center' }}>
                        {isBlocked ? (
                          isDomain
                            ? <GlobeAltIcon style={{ width: 14, height: 14, color: 'var(--color-warning)', display: 'inline-block' }} />
                            : <NoSymbolIcon style={{ width: 14, height: 14, color: 'var(--color-destructive)', display: 'inline-block' }} />
                        ) : (
                          isDomain
                            ? <GlobeAltIcon style={{ width: 14, height: 14, color: 'var(--color-accent)', display: 'inline-block' }} />
                            : <CheckCircleIcon style={{ width: 14, height: 14, color: 'var(--color-accent)', display: 'inline-block' }} />
                        )}
                      </td>
                      <td style={{ ...tdSt, fontFamily: 'monospace', fontSize: '12px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {addr}
                      </td>
                      <td style={{ ...tdSt, fontSize: '12px', color: 'var(--color-text-secondary)', whiteSpace: 'nowrap' }}>
                        {formatDate(addr)}
                      </td>
                      <td style={{ ...tdSt, textAlign: 'center' }}>
                        {isBlocked ? (
                          <button
                            onClick={() => onRemove(addr)}
                            style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-destructive)', cursor: 'pointer' }}
                            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'color-mix(in srgb, var(--color-destructive) 10%, transparent)'; }}
                            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                          >{t('unblock')}</button>
                        ) : (
                          <button
                            onClick={() => onRemove(addr)}
                            style={{ fontSize: '12px', padding: '3px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: 'pointer' }}
                            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                          >{t('disallow')}</button>
                        )}
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
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              disabled={safePage === 0}
              style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: safePage === 0 ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)', cursor: safePage === 0 ? 'default' : 'pointer', fontSize: '12px' }}
            >‹</button>
            <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', minWidth: '80px', textAlign: 'center' }}>
              {safePage + 1} / {totalPages}
            </span>
            <button
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              disabled={safePage === totalPages - 1}
              style={{ padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: safePage === totalPages - 1 ? 'var(--color-text-tertiary)' : 'var(--color-text-secondary)', cursor: safePage === totalPages - 1 ? 'default' : 'pointer', fontSize: '12px' }}
            >›</button>
          </div>
        )}
      </SectionCard>

      {/* ── Add input form ── */}
      <SectionCard>
        <SectionHeader>{t(isBlocked ? 'sectionAddBlockedSender' : 'sectionAddAllowedSender')}</SectionHeader>
        <div style={{ padding: '4px 20px 8px', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>
          {t(isBlocked ? 'blockedInputHint' : 'allowedInputHint')}
        </div>
        <div style={{ padding: '0 20px 20px' }}>
          <div style={{ display: 'flex', gap: '8px', alignItems: 'stretch' }}>
            <input
              value={newInput}
              onChange={(e) => setNewInput(e.target.value)}
              onKeyDown={(e) => { if (e.key === 'Enter') onAdd(); }}
              placeholder={t(isBlocked ? 'blockedInputPlaceholder' : 'allowedInputPlaceholder')}
              style={{
                flex: 1, boxSizing: 'border-box',
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
            <button
              onClick={onAdd}
              disabled={!valTrimmed || senders.includes(valTrimmed)}
              style={{
                padding: '9px 20px', borderRadius: '7px', border: 'none',
                background: 'var(--color-accent)', color: '#fff',
                fontSize: '13px', fontWeight: 600,
                cursor: valTrimmed && !senders.includes(valTrimmed) ? 'pointer' : 'default',
                opacity: valTrimmed && !senders.includes(valTrimmed) ? 1 : 0.4,
                flexShrink: 0, whiteSpace: 'nowrap',
                transition: 'opacity 120ms',
              }}
              onMouseEnter={(e) => { if (valTrimmed && !senders.includes(valTrimmed)) (e.currentTarget as HTMLButtonElement).style.opacity = '0.88'; }}
              onMouseLeave={(e) => { if (valTrimmed && !senders.includes(valTrimmed)) (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
            >{t(isBlocked ? 'block' : 'allow')}</button>
          </div>
          {valTrimmed && senders.includes(valTrimmed) && (
            <div style={{ marginTop: '6px', fontSize: '12px', color: 'var(--color-warning)' }}>
              {t(isBlocked ? 'blockedAlready' : 'allowedAlready')}
            </div>
          )}
        </div>
      </SectionCard>
    </>
  );
}
