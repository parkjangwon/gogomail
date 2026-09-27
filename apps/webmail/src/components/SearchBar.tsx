'use client';

import { useState, useRef, useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { MagnifyingGlassIcon, XMarkIcon, AdjustmentsHorizontalIcon } from '@heroicons/react/24/outline';
import { AdvancedFilters } from '@/components/Sidebar';
import {
  buildActiveFilterChips,
  clearFilterField,
  type FilterField,
} from '@/lib/mail/searchFilters';

const RECENT_SEARCHES_KEY = 'webmail_recent_searches';
const MAX_RECENT = 5;

function loadRecentSearches(): string[] {
  try { return JSON.parse(localStorage.getItem(RECENT_SEARCHES_KEY) ?? '[]') as string[]; } catch { return []; }
}
function saveRecentSearch(q: string): string[] {
  const t = q.trim();
  if (!t) return loadRecentSearches();
  const prev = loadRecentSearches().filter((x) => x !== t);
  const next = [t, ...prev].slice(0, MAX_RECENT);
  localStorage.setItem(RECENT_SEARCHES_KEY, JSON.stringify(next));
  return next;
}

interface SearchBarFolderOption {
  id: string;
  name: string;
  system_type?: string;
}

interface SearchBarProps {
  value: string;
  onChange: (q: string) => void;
  advancedFilters?: AdvancedFilters;
  onAdvancedFilterChange?: (filters: AdvancedFilters) => void;
  /** Folders offered by the "folder" operator in the builder. */
  folders?: SearchBarFolderOption[];
  /** Distinct label colors offered by the "label" operator. */
  labelColors?: string[];
}

export function SearchBar({ value, onChange, advancedFilters = {}, onAdvancedFilterChange, folders = [], labelColors = [] }: SearchBarProps) {
  const t = useTranslations();
  const [focused, setFocused] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [showSuggestions, setShowSuggestions] = useState(false);
  const [recentSearches, setRecentSearches] = useState<string[]>([]);
  const [draft, setDraft] = useState<AdvancedFilters>(advancedFilters);
  const inputRef = useRef<HTMLInputElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => { setRecentSearches(loadRecentSearches()); }, []);

  // sync draft when external filters change
  useEffect(() => { setDraft(advancedFilters); }, [advancedFilters]);

  useEffect(() => {
    if (!showAdvanced && !showSuggestions) return;
    function onDown(e: MouseEvent) {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setShowAdvanced(false);
        setShowSuggestions(false);
      }
    }
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [showAdvanced, showSuggestions]);

  function handleClear() {
    onChange('');
    onAdvancedFilterChange?.({});
    setDraft({});
    inputRef.current?.focus();
  }

  function handleSubmitAdvanced() {
    onAdvancedFilterChange?.(draft);
    setShowAdvanced(false);
  }

  const hasActive = value.trim().length > 0 || Object.values(advancedFilters).some(Boolean);
  const dropdownOpen = showAdvanced || (showSuggestions && recentSearches.length > 0 && !value.trim());

  const folderNameById = (id: string) => folders.find((f) => f.id === id)?.name;
  const activeChips = buildActiveFilterChips(advancedFilters, {
    folderName: folderNameById,
    colorName: (c) => c,
  });
  function removeChip(field: FilterField) {
    const next = clearFilterField(advancedFilters, field);
    setDraft(next);
    onAdvancedFilterChange?.(next);
  }

  const fieldRow: React.CSSProperties = {
    display: 'grid',
    gridTemplateColumns: '110px 1fr',
    alignItems: 'center',
    gap: '12px',
    padding: '10px 0',
    borderBottom: '1px solid var(--color-border-subtle)',
  };
  const fieldLabel: React.CSSProperties = {
    fontSize: '13px',
    color: 'var(--color-text-secondary)',
    textAlign: 'right',
  };
  const fieldInput: React.CSSProperties = {
    border: 'none',
    borderBottom: '1px solid var(--color-border-default)',
    background: 'transparent',
    color: 'var(--color-text-primary)',
    fontSize: '14px',
    outline: 'none',
    padding: '2px 0',
    width: '100%',
  };

  return (
    <div ref={containerRef} style={{ position: 'relative', flex: 1 }}>
      {/* Search input pill */}
      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        background: focused || dropdownOpen ? 'var(--color-bg-primary)' : 'var(--color-bg-secondary)',
        borderRadius: dropdownOpen ? '24px 24px 0 0' : '24px',
        padding: '10px 16px',
        boxShadow: focused || dropdownOpen ? '0 1px 6px rgba(0,0,0,0.1)' : 'none',
        transition: 'background 150ms ease, border-radius 100ms ease',
      }}>
        <MagnifyingGlassIcon style={{ width: '20px', height: '20px', color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
        <input
          ref={inputRef}
          type="search"
          placeholder={t('misc.searchBar.placeholder')}
          aria-label={t('misc.searchBar.aria')}
          value={value}
          onChange={(e) => {
            onChange(e.target.value);
            setShowSuggestions(!e.target.value.trim());
          }}
          onFocus={() => {
            setFocused(true);
            if (!value.trim() && !showAdvanced) setShowSuggestions(true);
          }}
          onBlur={() => {
            setFocused(false);
            setTimeout(() => { if (!showAdvanced) setShowSuggestions(false); }, 150);
          }}
          onKeyDown={(e) => {
            if (e.key === 'Enter' && value.trim()) { setRecentSearches(saveRecentSearch(value)); setShowSuggestions(false); }
            if (e.key === 'Escape') handleClear();
          }}
          style={{ flex: 1, border: 'none', outline: 'none', background: 'transparent', fontSize: '16px', color: 'var(--color-text-primary)' }}
        />
        {hasActive && (
          <button
            onClick={handleClear}
            aria-label={t('misc.searchBar.clear')}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: 0, display: 'inline-flex', flexShrink: 0 }}
          >
            <XMarkIcon style={{ width: '18px', height: '18px' }} />
          </button>
        )}
        <button
          onClick={() => { setShowAdvanced((v) => !v); setShowSuggestions(false); }}
          aria-label={t('misc.searchBar.advancedAria')}
          title={t('misc.searchBar.advancedTitle')}
          style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '2px', color: showAdvanced ? 'var(--color-accent)' : 'var(--color-text-tertiary)', display: 'inline-flex', flexShrink: 0 }}
        >
          <AdjustmentsHorizontalIcon style={{ width: '18px', height: '18px' }} />
        </button>
      </div>

      {/* Active filter chips */}
      {activeChips.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', padding: '8px 4px 0' }}>
          {activeChips.map((chip) => (
            <span
              key={chip.field}
              style={{
                display: 'inline-flex', alignItems: 'center', gap: '6px',
                padding: '3px 8px', borderRadius: '14px',
                background: 'var(--color-accent-subtle)', color: 'var(--color-text-primary)',
                fontSize: '12px', lineHeight: 1.4, maxWidth: '220px',
              }}
            >
              <span style={{ color: 'var(--color-text-tertiary)', flexShrink: 0 }}>{t(`misc.searchBar.${chip.labelKey}`)}:</span>
              {chip.field === 'label' ? (
                <span aria-hidden="true" style={{ width: '12px', height: '12px', borderRadius: '50%', background: chip.value, flexShrink: 0 }} />
              ) : (
                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{chip.value}</span>
              )}
              <button
                type="button"
                onClick={() => removeChip(chip.field)}
                aria-label={t('misc.searchBar.clear')}
                style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'inline-flex', padding: 0, flexShrink: 0 }}
              >
                <XMarkIcon style={{ width: '13px', height: '13px' }} />
              </button>
            </span>
          ))}
        </div>
      )}

      {/* Recent searches */}
      {showSuggestions && !showAdvanced && recentSearches.length > 0 && !value.trim() && (
        <div style={{
          position: 'absolute', top: '100%', left: 0, right: 0, zIndex: 400,
          background: 'var(--color-bg-primary)',
          border: '1px solid var(--color-border-default)', borderTop: 'none',
          borderRadius: '0 0 16px 16px',
          boxShadow: '0 8px 24px rgba(0,0,0,0.12)',
          overflow: 'hidden',
        }}>
          <div style={{ padding: '8px 20px 4px', fontSize: '11px', color: 'var(--color-text-tertiary)', fontWeight: 600, letterSpacing: '0.05em', textTransform: 'uppercase' }}>{t('misc.searchBar.recent')}</div>
          {recentSearches.map((q) => (
            <div
              key={q}
              style={{ display: 'flex', alignItems: 'center', padding: '0 12px 0 20px' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'transparent'; }}
            >
              <button
                onMouseDown={() => { onChange(q); setRecentSearches(saveRecentSearch(q)); setShowSuggestions(false); }}
                style={{ display: 'flex', alignItems: 'center', gap: '10px', flex: 1, textAlign: 'left', padding: '9px 0', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '14px', cursor: 'pointer' }}
              >
                <MagnifyingGlassIcon style={{ width: '14px', height: '14px', color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
                {q}
              </button>
              <button
                onMouseDown={(e) => {
                  e.stopPropagation();
                  const next = recentSearches.filter((x) => x !== q);
                  localStorage.setItem(RECENT_SEARCHES_KEY, JSON.stringify(next));
                  setRecentSearches(next);
                }}
                title="최근 검색어 삭제"
                style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '4px', color: 'var(--color-text-tertiary)', display: 'flex', flexShrink: 0, borderRadius: '4px' }}
              >
                <XMarkIcon style={{ width: '14px', height: '14px' }} />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Advanced filter panel */}
      {showAdvanced && (
        <div style={{
          position: 'absolute', top: '100%', left: 0, right: 0, zIndex: 400,
          background: 'var(--color-bg-primary)',
          border: '1px solid var(--color-border-default)', borderTop: 'none',
          borderRadius: '0 0 16px 16px',
          boxShadow: '0 8px 24px rgba(0,0,0,0.12)',
          padding: '4px 24px 20px',
        }}>
          <div style={fieldRow}>
            <span style={fieldLabel}>{t('misc.searchBar.from')}</span>
            <input
              type="text"
              value={draft.from ?? ''}
              onChange={(e) => setDraft((d) => ({ ...d, from: e.target.value || undefined }))}
              style={fieldInput}
            />
          </div>
          <div style={fieldRow}>
            <span style={fieldLabel}>{t('misc.searchBar.to')}</span>
            <input
              type="text"
              value={draft.to ?? ''}
              onChange={(e) => setDraft((d) => ({ ...d, to: e.target.value || undefined }))}
              style={fieldInput}
            />
          </div>
          <div style={fieldRow}>
            <span style={fieldLabel}>{t('misc.searchBar.subject')}</span>
            <input
              type="text"
              value={draft.subject ?? ''}
              onChange={(e) => setDraft((d) => ({ ...d, subject: e.target.value || undefined }))}
              style={fieldInput}
            />
          </div>
          <div style={fieldRow}>
            <span style={fieldLabel}>{t('misc.searchBar.wordsContains')}</span>
            <input
              type="text"
              value={value}
              onChange={(e) => onChange(e.target.value)}
              style={fieldInput}
            />
          </div>
          <div style={fieldRow}>
            <span style={fieldLabel}>{t('misc.searchBar.dateRange')}</span>
            <div style={{ display: 'flex', gap: '8px', alignItems: 'center' }}>
              <input
                type="date"
                value={draft.since ?? ''}
                onChange={(e) => setDraft((d) => ({ ...d, since: e.target.value || undefined }))}
                style={{ ...fieldInput, flex: 1 }}
              />
              <span style={{ fontSize: '12px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>~</span>
              <input
                type="date"
                value={draft.until ?? ''}
                onChange={(e) => setDraft((d) => ({ ...d, until: e.target.value || undefined }))}
                style={{ ...fieldInput, flex: 1 }}
              />
            </div>
          </div>
          <div style={{ ...fieldRow, borderBottom: 'none', paddingBottom: '8px' }}>
            <span style={fieldLabel} />
            <label style={{ display: 'flex', alignItems: 'center', gap: '8px', fontSize: '13px', color: 'var(--color-text-secondary)', cursor: 'pointer' }}>
              <input
                type="checkbox"
                checked={draft.has_attachment ?? false}
                onChange={(e) => setDraft((d) => ({ ...d, has_attachment: e.target.checked || undefined }))}
              />
              {t('misc.searchBar.hasAttachment')}
            </label>
          </div>
          {folders.length > 0 && (
            <div style={fieldRow}>
              <span style={fieldLabel}>{t('misc.searchBar.folder')}</span>
              <select
                value={draft.folder_id ?? ''}
                onChange={(e) => setDraft((d) => ({ ...d, folder_id: e.target.value || undefined }))}
                style={{ ...fieldInput, cursor: 'pointer' }}
              >
                <option value="">{t('misc.searchBar.folderAny')}</option>
                {folders.map((f) => (
                  <option key={f.id} value={f.id}>{f.name}</option>
                ))}
              </select>
            </div>
          )}
          {labelColors.length > 0 && (
            <div style={fieldRow}>
              <span style={fieldLabel}>{t('misc.searchBar.label')}</span>
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexWrap: 'wrap' }}>
                {labelColors.map((color) => {
                  const active = draft.label === color;
                  return (
                    <button
                      key={color}
                      type="button"
                      aria-pressed={active}
                      aria-label={t('misc.searchBar.label')}
                      onClick={() => setDraft((d) => ({ ...d, label: active ? undefined : color }))}
                      style={{
                        width: '18px', height: '18px', borderRadius: '50%', background: color,
                        border: active ? '2px solid var(--color-text-primary)' : '2px solid transparent',
                        boxShadow: active ? '0 0 0 1px var(--color-bg-primary)' : 'none',
                        cursor: 'pointer', flexShrink: 0, padding: 0,
                      }}
                    />
                  );
                })}
                {draft.label && (
                  <button
                    type="button"
                    onClick={() => setDraft((d) => ({ ...d, label: undefined }))}
                    style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'inline-flex', padding: '2px' }}
                    aria-label={t('misc.searchBar.clear')}
                  >
                    <XMarkIcon style={{ width: '14px', height: '14px' }} />
                  </button>
                )}
              </div>
            </div>
          )}
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', paddingTop: '12px', borderTop: '1px solid var(--color-border-subtle)' }}>
            <button
              onClick={() => { setShowAdvanced(false); setDraft(advancedFilters); }}
              style={{ padding: '8px 20px', borderRadius: '20px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}
            >
              {t('misc.searchBar.cancel')}
            </button>
            <button
              onClick={handleSubmitAdvanced}
              style={{ padding: '8px 28px', borderRadius: '20px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}
            >
              {t('misc.searchBar.search')}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
