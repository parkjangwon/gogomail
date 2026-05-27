'use client';

import { SpotlightItem, sectionLabel } from './spotlight/spotlightHelpers';
import { useSpotlightSearch, type SpotlightSearchProps } from './spotlight/useSpotlightSearch';
import {
  MagnifyingGlassIcon,
  ArrowRightIcon,
  ClockIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';

export function SpotlightSearch(props: SpotlightSearchProps) {
  const {
    t,
    query,
    setQuery,
    scope,
    setScope,
    selIdx,
    setSelIdx,
    searching,
    activeOperators,
    inputRef,
    listRef,
    recentSearches,
    clearRecentSearch,
    isMoveMode,
    visibleItems,
  } = useSpotlightSearch(props);

  const { onClose } = props;

  // Group items by type for section labels
  const groupedItems: { label: string; items: (SpotlightItem & { idx: number })[] }[] = [];
  {
    let globalIdx = 0;
    const seen = new Set<string>();
    for (const item of visibleItems) {
      if (!seen.has(item.type)) {
        seen.add(item.type);
        groupedItems.push({ label: sectionLabel(t, item.type), items: [] });
      }
      groupedItems[groupedItems.length - 1].items.push({ ...item, idx: globalIdx++ });
    }
  }

  return (
    <div
      aria-modal="true"
      role="dialog"
      aria-label={t('dialogLabel')}
      onClick={(e) => { if (e.target === e.currentTarget) onClose(); }}
      style={{
        position: 'fixed', inset: 0, zIndex: 900,
        background: 'rgba(0,0,0,0.45)',
        backdropFilter: 'blur(4px)',
        display: 'flex',
        alignItems: 'flex-start',
        justifyContent: 'center',
        paddingTop: '12vh',
      }}
    >
      <div
        style={{
          width: '100%',
          maxWidth: '600px',
          margin: '0 16px',
          background: 'var(--color-bg-primary)',
          borderRadius: '14px',
          boxShadow: '0 24px 80px rgba(0,0,0,0.3)',
          overflow: 'hidden',
          border: '1px solid var(--color-border-default)',
          animation: 'spotlightIn 120ms cubic-bezier(0.16,1,0.3,1)',
        }}
      >
        {/* Move mode badge */}
        {isMoveMode && (
          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', padding: '8px 18px 0', borderBottom: 'none' }}>
            <ArrowRightIcon style={{ width: 13, height: 13, color: 'var(--color-accent)' }} />
            <span style={{ fontSize: '12px', fontWeight: 600, color: 'var(--color-accent)' }}>{t('moveBadge')}</span>
          </div>
        )}

        {/* Search input */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', padding: isMoveMode ? '8px 18px 14px' : '14px 18px', borderBottom: '1px solid var(--color-border-subtle)' }}>
          {searching
            ? <ArrowRightIcon style={{ width: 20, height: 20, color: 'var(--color-text-tertiary)', flexShrink: 0, animation: 'spin 600ms linear infinite' }} />
            : <MagnifyingGlassIcon style={{ width: 20, height: 20, color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
          }
          <input
            ref={inputRef}
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={isMoveMode ? t('placeholderMove') : t('placeholderSearch')}
            aria-label={isMoveMode ? t('ariaMove') : t('ariaSearch')}
            style={{
              flex: 1,
              border: 'none',
              outline: 'none',
              background: 'transparent',
              fontSize: '16px',
              color: 'var(--color-text-primary)',
              fontFamily: 'inherit',
            }}
          />
          <kbd style={{ fontSize: '11px', padding: '2px 6px', borderRadius: '4px', background: 'var(--color-bg-tertiary)', color: 'var(--color-text-tertiary)', border: '1px solid var(--color-border-default)', flexShrink: 0 }}>Esc</kbd>
        </div>

        {/* Scope filter chips */}
        {!isMoveMode && (
          <div style={{ display: 'flex', gap: '6px', padding: '6px 16px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0, flexWrap: 'wrap' }}>
            {(['all', 'mail', 'contacts', 'calendar', 'drive', 'folders', 'commands', 'settings', 'notifications'] as const).map((s) => {
              const labels: Record<typeof s, string> = { all: t('scope.all'), mail: t('scope.mail'), contacts: t('scope.contacts'), calendar: t('scope.calendar'), drive: t('scope.drive'), folders: t('scope.folders'), commands: t('scope.commands'), settings: t('scope.settings'), notifications: t('scope.notifications') };
              return (
                <button key={s} type="button" onClick={() => setScope(s)}
                  style={{ padding: '3px 10px', borderRadius: '12px', border: 'none', cursor: 'pointer', fontSize: '12px', fontWeight: 500,
                    background: scope === s ? 'var(--color-accent)' : 'var(--color-bg-tertiary)',
                    color: scope === s ? '#fff' : 'var(--color-text-secondary)',
                  }}>
                  {labels[s]}
                </button>
              );
            })}
          </div>
        )}

        {/* Active operator chips */}
        {activeOperators.length > 0 && !isMoveMode && (
          <div style={{ display: 'flex', gap: '6px', padding: '4px 18px', flexWrap: 'wrap' }}>
            {activeOperators.map((op) => (
              <span key={op} style={{ display: 'inline-flex', alignItems: 'center', gap: '3px', fontSize: '11px', fontWeight: 600, color: 'var(--color-accent)', background: 'var(--color-accent-subtle)', borderRadius: '4px', padding: '2px 7px', letterSpacing: '0.02em' }}>
                {op}
              </span>
            ))}
          </div>
        )}

        {/* Recent searches (shown only when empty + no query, not in move mode) */}
        {!query && recentSearches.length > 0 && !isMoveMode && (
          <div style={{ padding: '8px 12px 0' }}>
            <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', padding: '4px 6px', letterSpacing: '0.05em', textTransform: 'uppercase' }}>{t('recentSearches')}</div>
            {recentSearches.map((q) => (
              <div
                key={q}
                style={{ display: 'flex', alignItems: 'center', borderRadius: '6px' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'transparent'; }}
              >
                <button
                  onMouseDown={() => setQuery(q)}
                  style={{ display: 'flex', alignItems: 'center', gap: '8px', flex: 1, padding: '6px 6px', border: 'none', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer', textAlign: 'left' }}
                >
                  <ClockIcon style={{ width: 13, height: 13, color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
                  {q}
                </button>
                <button
                  onMouseDown={(e) => { e.stopPropagation(); clearRecentSearch(q); }}
                  title="삭제"
                  style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '4px 6px', color: 'var(--color-text-tertiary)', display: 'flex', flexShrink: 0, borderRadius: '4px' }}
                >
                  <XMarkIcon style={{ width: 13, height: 13 }} />
                </button>
              </div>
            ))}
          </div>
        )}

        {/* Results */}
        <div ref={listRef} style={{ maxHeight: '420px', overflowY: 'auto', padding: '8px 12px 12px' }}>
          {visibleItems.length === 0 && query && !searching && (
            <div style={{ padding: '32px', textAlign: 'center', fontSize: '14px', color: 'var(--color-text-tertiary)' }}>
              {t('noResults')}
            </div>
          )}
          {groupedItems.map((group) => (
            <div key={group.label}>
              <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', padding: '8px 6px 4px', letterSpacing: '0.05em', textTransform: 'uppercase' }}>
                {group.label}
              </div>
              {group.items.map((item) => {
                const isSel = item.idx === selIdx;
                return (
                  <button
                    key={item.id}
                    data-idx={item.idx}
                    onMouseEnter={() => setSelIdx(item.idx)}
                    onMouseDown={(e) => { e.preventDefault(); item.onSelect(); }}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '10px',
                      width: '100%',
                      padding: '8px 10px',
                      border: 'none',
                      borderRadius: '8px',
                      background: isSel ? 'var(--color-accent)' : 'transparent',
                      color: isSel ? '#fff' : 'var(--color-text-primary)',
                      cursor: 'pointer',
                      textAlign: 'left',
                      transition: 'background 80ms ease',
                    }}
                  >
                    <span style={{ flexShrink: 0, opacity: isSel ? 1 : 0.7, display: 'inline-flex' }}>
                      {item.icon}
                    </span>
                    <span style={{ flex: 1, minWidth: 0 }}>
                      <span style={{ fontSize: '14px', fontWeight: 500, display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {item.title}
                      </span>
                      {item.subtitle && (
                        <span style={{ fontSize: '12px', opacity: isSel ? 0.8 : 0.6, display: 'block', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                          {item.subtitle}
                        </span>
                      )}
                    </span>
                    {item.badge && (
                      <span style={{ fontSize: '11px', opacity: isSel ? 0.8 : 0.5, flexShrink: 0, whiteSpace: 'nowrap' }}>
                        {item.badge}
                      </span>
                    )}
                    {item.type === 'action' && item.subtitle && item.subtitle.length <= 3 && (
                      <kbd style={{ fontSize: '11px', padding: '2px 6px', borderRadius: '4px', background: isSel ? 'rgba(255,255,255,0.2)' : 'var(--color-bg-tertiary)', border: `1px solid ${isSel ? 'rgba(255,255,255,0.2)' : 'var(--color-border-default)'}`, color: isSel ? '#fff' : 'var(--color-text-tertiary)', flexShrink: 0 }}>
                        {item.subtitle}
                      </kbd>
                    )}
                  </button>
                );
              })}
            </div>
          ))}
        </div>

        {/* Footer hint */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '8px 18px', borderTop: '1px solid var(--color-border-subtle)', fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
          <span><kbd style={kbdStyle}>↑↓</kbd> {t('footer.navigate')}</span>
          <span><kbd style={kbdStyle}>↵</kbd> {t('footer.select')}</span>
          {!isMoveMode && <span><kbd style={kbdStyle}>←→</kbd> {t('footer.scope')}</span>}
          <span><kbd style={kbdStyle}>Esc</kbd> {t('footer.close')}</span>
          <span style={{ marginLeft: 'auto' }}>{t('footer.brand')}</span>
        </div>
      </div>

      <style>{`
        @keyframes spotlightIn {
          from { opacity: 0; transform: scale(0.96) translateY(-8px); }
          to   { opacity: 1; transform: scale(1) translateY(0); }
        }
        @keyframes spin {
          from { transform: rotate(0deg); }
          to   { transform: rotate(360deg); }
        }
      `}</style>
    </div>
  );
}

const kbdStyle: React.CSSProperties = {
  display: 'inline-block',
  padding: '1px 5px',
  borderRadius: '4px',
  background: 'var(--color-bg-tertiary)',
  border: '1px solid var(--color-border-default)',
  fontSize: '10px',
  fontFamily: 'inherit',
  color: 'var(--color-text-secondary)',
  marginRight: '3px',
};
