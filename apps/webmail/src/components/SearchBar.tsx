'use client';

import { useState, useRef, useEffect } from 'react';
import { MagnifyingGlassIcon, XMarkIcon, AdjustmentsHorizontalIcon } from '@heroicons/react/24/outline';
import { AdvancedFilters } from '@/components/Sidebar';

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

interface SearchBarProps {
  value: string;
  onChange: (q: string) => void;
  advancedFilters?: AdvancedFilters;
  onAdvancedFilterChange?: (filters: AdvancedFilters) => void;
}

export function SearchBar({ value, onChange, advancedFilters = {}, onAdvancedFilterChange }: SearchBarProps) {
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
          placeholder="메일 검색"
          aria-label="메일 검색"
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
            aria-label="검색 지우기"
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: 0, display: 'inline-flex', flexShrink: 0 }}
          >
            <XMarkIcon style={{ width: '18px', height: '18px' }} />
          </button>
        )}
        <button
          onClick={() => { setShowAdvanced((v) => !v); setShowSuggestions(false); }}
          aria-label="상세 검색 필터"
          title="상세 검색"
          style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '2px', color: showAdvanced ? 'var(--color-accent)' : 'var(--color-text-tertiary)', display: 'inline-flex', flexShrink: 0 }}
        >
          <AdjustmentsHorizontalIcon style={{ width: '18px', height: '18px' }} />
        </button>
      </div>

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
          <div style={{ padding: '8px 20px 4px', fontSize: '11px', color: 'var(--color-text-tertiary)', fontWeight: 600, letterSpacing: '0.05em', textTransform: 'uppercase' }}>최근 검색</div>
          {recentSearches.map((q) => (
            <button
              key={q}
              onMouseDown={() => { onChange(q); setRecentSearches(saveRecentSearch(q)); setShowSuggestions(false); }}
              style={{ display: 'flex', alignItems: 'center', gap: '10px', width: '100%', textAlign: 'left', padding: '9px 20px', border: 'none', background: 'transparent', color: 'var(--color-text-primary)', fontSize: '14px', cursor: 'pointer' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
            >
              <MagnifyingGlassIcon style={{ width: '14px', height: '14px', color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
              {q}
            </button>
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
            <span style={fieldLabel}>보낸사람</span>
            <input
              type="text"
              value={draft.from ?? ''}
              onChange={(e) => setDraft((d) => ({ ...d, from: e.target.value || undefined }))}
              style={fieldInput}
            />
          </div>
          <div style={fieldRow}>
            <span style={fieldLabel}>받는사람</span>
            <input type="text" disabled style={{ ...fieldInput, opacity: 0.35, cursor: 'not-allowed' }} />
          </div>
          <div style={fieldRow}>
            <span style={fieldLabel}>제목</span>
            <input
              type="text"
              value={draft.subject ?? ''}
              onChange={(e) => setDraft((d) => ({ ...d, subject: e.target.value || undefined }))}
              style={fieldInput}
            />
          </div>
          <div style={fieldRow}>
            <span style={fieldLabel}>포함하는 단어</span>
            <input
              type="text"
              value={value}
              onChange={(e) => onChange(e.target.value)}
              style={fieldInput}
            />
          </div>
          <div style={fieldRow}>
            <span style={fieldLabel}>기간</span>
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
              첨부파일 있음
            </label>
          </div>
          <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '8px', paddingTop: '12px', borderTop: '1px solid var(--color-border-subtle)' }}>
            <button
              onClick={() => { setShowAdvanced(false); setDraft(advancedFilters); }}
              style={{ padding: '8px 20px', borderRadius: '20px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}
            >
              취소
            </button>
            <button
              onClick={handleSubmitAdvanced}
              style={{ padding: '8px 28px', borderRadius: '20px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 600, cursor: 'pointer' }}
            >
              검색
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
