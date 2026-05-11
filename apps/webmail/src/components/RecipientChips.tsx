'use client';

import { useState, useRef, KeyboardEvent, ClipboardEvent } from 'react';

interface RecipientChipsProps {
  value: string;
  onChange: (v: string) => void;
  placeholder?: string;
  id?: string;
  autoFocus?: boolean;
  hasError?: boolean;
}

function parseEmails(raw: string): string[] {
  return raw.split(/[,;\s]+/).map((s) => s.trim()).filter(Boolean);
}

export function RecipientChips({ value, onChange, placeholder, id, autoFocus, hasError }: RecipientChipsProps) {
  const [chips, setChips] = useState<string[]>(() => (value ? parseEmails(value) : []));
  const [input, setInput] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  function commit(raw: string) {
    const emails = parseEmails(raw);
    if (emails.length === 0) return;
    const next = [...chips, ...emails];
    setChips(next);
    setInput('');
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
      style={{
        display: 'flex',
        flexWrap: 'wrap',
        gap: '4px',
        padding: '5px 0',
        flex: 1,
        minHeight: '36px',
        cursor: 'text',
      }}
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
            style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-secondary)', padding: '0 1px', lineHeight: 1, fontSize: '14px', flexShrink: 0 }}
          >×</button>
        </span>
      ))}
      <input
        ref={inputRef}
        id={id}
        type="email"
        value={input}
        onChange={(e) => setInput(e.target.value)}
        onKeyDown={onKeyDown}
        onPaste={onPaste}
        onBlur={() => { if (input.trim()) commit(input); }}
        placeholder={chips.length === 0 ? placeholder : ''}
        autoFocus={autoFocus}
        style={{ flex: 1, minWidth: '120px', border: 'none', outline: 'none', fontSize: '14px', background: 'transparent', color: 'var(--color-text-primary)', padding: '2px 0' }}
      />
    </div>
  );
}
