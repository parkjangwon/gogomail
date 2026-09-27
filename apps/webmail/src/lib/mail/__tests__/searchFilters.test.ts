import { describe, expect, it } from 'vitest';
import {
  applyLabelFilter,
  buildActiveFilterChips,
  clearFilterField,
  hasActiveFilters,
} from '../searchFilters';
import type { AdvancedFilters } from '@/components/Sidebar';
import type { MessageSummary } from '@/lib/api';

function msg(id: string): MessageSummary {
  // Only the id is exercised by the label filter; cast the rest.
  return { id } as MessageSummary;
}

describe('buildActiveFilterChips', () => {
  it('produces no chips for an empty filter set', () => {
    expect(buildActiveFilterChips({})).toEqual([]);
  });

  it('ignores empty strings and false booleans', () => {
    const filters: AdvancedFilters = { from: '', subject: undefined, has_attachment: false };
    expect(buildActiveFilterChips(filters)).toEqual([]);
  });

  it('builds chips in the documented field order', () => {
    const filters: AdvancedFilters = {
      until: '2026-01-31',
      from: 'alice@example.com',
      has_attachment: true,
      subject: 'invoice',
    };
    expect(buildActiveFilterChips(filters).map((c) => c.field)).toEqual([
      'from',
      'subject',
      'until',
      'has_attachment',
    ]);
  });

  it('resolves folder_id via folderName and label via colorName', () => {
    const filters: AdvancedFilters = { folder_id: 'f1', label: '#ef4444' };
    const chips = buildActiveFilterChips(filters, {
      folderName: (id) => (id === 'f1' ? 'Projects' : undefined),
      colorName: (c) => `color(${c})`,
    });
    expect(chips.find((c) => c.field === 'folder_id')?.value).toBe('Projects');
    expect(chips.find((c) => c.field === 'label')?.value).toBe('color(#ef4444)');
  });

  it('falls back to the raw id when no folderName resolver is given', () => {
    const chips = buildActiveFilterChips({ folder_id: 'raw-id' });
    expect(chips[0].value).toBe('raw-id');
  });

  it('renders has_attachment as a check mark', () => {
    const chips = buildActiveFilterChips({ has_attachment: true });
    expect(chips[0].value).toBe('✓');
  });
});

describe('clearFilterField / hasActiveFilters', () => {
  it('removes a single field immutably', () => {
    const filters: AdvancedFilters = { from: 'a', subject: 'b' };
    const next = clearFilterField(filters, 'from');
    expect(next).toEqual({ subject: 'b' });
    expect(filters.from).toBe('a'); // original untouched
  });

  it('hasActiveFilters reflects presence of any chip', () => {
    expect(hasActiveFilters({})).toBe(false);
    expect(hasActiveFilters({ has_attachment: false })).toBe(false);
    expect(hasActiveFilters({ subject: 'x' })).toBe(true);
  });
});

describe('applyLabelFilter', () => {
  const messages = [msg('a'), msg('b'), msg('c')];
  const labels: Record<string, string> = { a: '#ef4444', b: '#3b82f6', c: '#ef4444' };

  it('returns all messages when no label is requested', () => {
    expect(applyLabelFilter(messages, undefined, labels)).toBe(messages);
  });

  it('keeps only messages carrying the requested label color', () => {
    const result = applyLabelFilter(messages, '#ef4444', labels);
    expect(result.map((m) => m.id)).toEqual(['a', 'c']);
  });

  it('returns an empty list when nothing matches', () => {
    expect(applyLabelFilter(messages, '#000000', labels)).toEqual([]);
  });
});
