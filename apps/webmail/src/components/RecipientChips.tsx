'use client';

import { useState, useRef, useEffect, KeyboardEvent, ClipboardEvent } from 'react';
import { XMarkIcon } from '@heroicons/react/24/outline';
import { autocompleteContacts, ContactSuggestion } from '@/lib/api';

interface RecipientChipsProps {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  id?: string;
  autoFocus?: boolean;
  hasError?: boolean;
  suggestions?: string[];
}

function parseEmails(raw: string): string[] {
  return raw.split(/[,;\s]+/).map((s) => s.trim()).filter(Boolean);
}

/** Format a ContactSuggestion as a chip string. */
function formatSuggestion(s: ContactSuggestion): string {
  return s.display_name ? `${s.display_name} <${s.email}>` : s.email;
}

export function RecipientChips({ value, onChange, placeholder, id, autoFocus, hasError, suggestions = [] }: RecipientChipsProps) {
  const [chips, setChips] = useState<string[]>(() => (value ? parseEmails(value) : []));
  const [input, setInput] = useState('');
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const [activeIdx, setActiveIdx] = useState(-1);
  const [apiResults, setApiResults] = useState<ContactSuggestion[]>([]);
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Debounced API autocomplete
  useEffect(() => {
    if (debounceRef.current) clearTimeout(debounceRef.current);
    const q = input.trim();
    if (!q) { setApiResults([]); return; }
    debounceRef.current = setTimeout(async () => {
      const results = await autocompleteContacts(q, 8);
      setApiResults(results);
    }, 200);
    return () => { if (debounceRef.current) clearTimeout(debounceRef.current); };
  }, [input]);

  // Merge local suggestions and API results into a unified dropdown list.
  // Local suggestions first (already formatted as strings), then API results
  // not already covered by local suggestions.
  const chipSet = new Set(chips.map((c) => c.toLowerCase()));
  const localFiltered = input.trim().length > 0
    ? suggestions.filter(
        (s) => s.toLowerCase().includes(input.toLowerCase()) && !chipSet.has(s.toLowerCase())
      ).slice(0, 6)
    : [];

  // Build merged list: { label: string (what to display), value: string (what to commit) }
  interface DropdownItem { label: string; sublabel?: string; value: string; type?: string; }
  const localItems: DropdownItem[] = localFiltered.map((s) => ({ label: s, value: s }));
  const apiEmails = new Set(localFiltered.map((s) => {
    const m = s.match(/<([^>]+)>/);
    return (m ? m[1] : s).toLowerCase();
  }));
  const apiItems: DropdownItem[] = apiResults
    .filter((r) => r.email && !apiEmails.has(r.email.toLowerCase()) && !chipSet.has(r.email.toLowerCase()))
    .map((r) => ({
      label: r.display_name || r.email,
      sublabel: r.display_name ? r.email : undefined,
      value: formatSuggestion(r),
      type: r.type,
    }));
  const dropdownItems: DropdownItem[] = [...localItems, ...apiItems].slice(0, 8);

  function commit(raw: string) {
    const emails = parseEmails(raw);
    if (emails.length === 0) return;
    const next = [...chips, ...emails];
    setChips(next);
    setInput('');
    setDropdownOpen(false);
    setActiveIdx(-1);
    setApiResults([]);
    onChange(next.join(', '));
  }

  function removeChip(i: number) {
    const next = chips.filter((_, idx) => idx !== i);
    setChips(next);
    onChange(next.join(', '));
  }

  function onPaste(e: ClipboardEvent<HTMLInputElement>) {
    const text = e.clipboardData.getData('text');
    if (text.includes(',') || text.includes(';') || text.includes('\n')) {
      e.preventDefault();
      commit(text);
    }
  }

  function onKeyDown(e: KeyboardEvent<HTMLInputElement>) {
    if (dropdownOpen && dropdownItems.length > 0) {
      if (e.key === 'ArrowDown') { e.preventDefault(); setActiveIdx((i) => Math.min(i + 1, dropdownItems.length - 1)); return; }
      if (e.key === 'ArrowUp') { e.preventDefault(); setActiveIdx((i) => Math.max(i - 1, -1)); return; }
      if ((e.key === 'Enter' || e.key === 'Tab') && activeIdx >= 0) {
        e.preventDefault();
        commit(dropdownItems[activeIdx].value);
        return;
      }
      if (e.key === 'Escape') { setDropdownOpen(false); setActiveIdx(-1); return; }
    }
    if ((e.key === 'Enter' || e.key === 'Tab' || e.key === ',') && input.trim()) {
      e.preventDefault();
      commit(input);
    } else if (e.key === 'Backspace' && input === '' && chips.length > 0) {
      removeChip(chips.length - 1);
    }
  }

  return (
    <div
      onClick={() => inputRef.current?.focus()}
      style={{ display: 'flex', flexWrap: 'wrap', gap: '4px', padding: '5px 0', flex: 1, minHeight: '36px', cursor: 'text', position: 'relative' }}
    >
      {chips.map((chip, i) => (
        <span
          key={chip + i}
          style={{
            display: 'inline-flex',
            alignItems: 'center',
            gap: '3px',
            padding: '2px 6px 2px 8px',
            borderRadius: '12px',
            background: hasError ? 'rgba(217,79,61,0.12)' : 'var(--color-bg-tertiary)',
            color: 'var(--color-text-primary)',
            fontSize: '13px',
            maxWidth: '220px',
            border: hasError ? '1px solid rgba(217,79,61,0.3)' : '1px solid transparent',
          }}
        >
          <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{chip}</span>
          <button
            type="button"
            onClick={(e) => { e.stopPropagation(); removeChip(i); }}
            aria-label={`${chip} 제거`}
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-secondary)', padding: '0 1px', lineHeight: 1, flexShrink: 0, display: 'inline-flex' }}
          ><XMarkIcon style={{ width: '12px', height: '12px' }} /></button>
        </span>
      ))}
      <input
        ref={inputRef}
        id={id}
        type="email"
        value={input}
        onChange={(e) => { setInput(e.target.value); setDropdownOpen(true); setActiveIdx(-1); }}
        onKeyDown={onKeyDown}
        onPaste={onPaste}
        onFocus={() => setDropdownOpen(true)}
        onBlur={() => {
          setTimeout(() => {
            setDropdownOpen(false);
            setActiveIdx(-1);
            if (input.trim()) commit(input);
          }, 150);
        }}
        placeholder={chips.length === 0 ? placeholder : ''}
        autoFocus={autoFocus}
        autoComplete="off"
        style={{ flex: 1, minWidth: '120px', border: 'none', outline: 'none', fontSize: '14px', background: 'transparent', color: 'var(--color-text-primary)', padding: '2px 0' }}
      />
      {dropdownOpen && dropdownItems.length > 0 && (
        <div
          style={{
            position: 'absolute',
            top: '100%',
            left: 0,
            right: 0,
            zIndex: 300,
            background: 'var(--color-bg-primary)',
            border: '1px solid var(--color-border-default)',
            borderRadius: '8px',
            boxShadow: '0 4px 16px rgba(0,0,0,0.12)',
            overflow: 'hidden',
            marginTop: '2px',
          }}
        >
          {dropdownItems.map((item, i) => (
            <button
              key={item.value + i}
              type="button"
              onMouseDown={(e) => { e.preventDefault(); commit(item.value); }}
              style={{
                display: 'flex',
                alignItems: 'center',
                width: '100%',
                textAlign: 'left',
                padding: '7px 12px',
                border: 'none',
                background: i === activeIdx ? 'var(--color-bg-secondary)' : 'transparent',
                cursor: 'pointer',
              }}
              onMouseEnter={() => setActiveIdx(i)}
              onMouseLeave={() => setActiveIdx(-1)}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: '6px', width: '100%', minWidth: 0 }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%', display: 'block' }}>{item.label}</span>
                  {item.sublabel && (
                    <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', maxWidth: '100%', display: 'block' }}>{item.sublabel}</span>
                  )}
                </div>
                {item.type === 'group' && (
                  <span style={{ fontSize: '10px', padding: '1px 6px', borderRadius: '8px', background: 'var(--color-accent-subtle, rgba(47,110,224,0.1))', color: 'var(--color-accent)', fontWeight: 600, flexShrink: 0 }}>그룹</span>
                )}
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}
