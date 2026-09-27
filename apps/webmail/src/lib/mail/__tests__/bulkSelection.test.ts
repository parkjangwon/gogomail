import { describe, expect, it } from 'vitest';
import {
  clearSelection,
  emptySelection,
  hasSelection,
  selectAll,
  toggleSelection,
  type SelectionState,
} from '../bulkSelection';

const IDS = ['a', 'b', 'c', 'd', 'e'];

function sel(ids: string[], anchorIndex: number | null = null): SelectionState {
  return { selected: new Set(ids), anchorIndex };
}

describe('emptySelection / clearSelection', () => {
  it('starts empty with no anchor', () => {
    const s = emptySelection();
    expect(s.selected.size).toBe(0);
    expect(s.anchorIndex).toBeNull();
  });

  it('clearSelection resets to empty', () => {
    const s = clearSelection();
    expect(s.selected.size).toBe(0);
    expect(s.anchorIndex).toBeNull();
  });
});

describe('toggleSelection (plain toggle)', () => {
  it('adds an id and sets the anchor to its index', () => {
    const next = toggleSelection(emptySelection(), 'c', IDS);
    expect([...next.selected]).toEqual(['c']);
    expect(next.anchorIndex).toBe(2);
  });

  it('removes an already-selected id', () => {
    const next = toggleSelection(sel(['c'], 2), 'c', IDS);
    expect(next.selected.has('c')).toBe(false);
    expect(next.anchorIndex).toBe(2);
  });

  it('does not mutate the input state', () => {
    const start = emptySelection();
    toggleSelection(start, 'a', IDS);
    expect(start.selected.size).toBe(0);
  });

  it('keeps the previous anchor for an unknown id', () => {
    const next = toggleSelection(sel([], 3), 'zzz', IDS);
    expect(next.selected.has('zzz')).toBe(true);
    expect(next.anchorIndex).toBe(3);
  });
});

describe('toggleSelection (shift-range)', () => {
  it('selects the contiguous range from anchor to target (downward)', () => {
    // anchor at index 1 ('b'), shift-click 'd' (index 3) => b,c,d
    const next = toggleSelection(sel(['b'], 1), 'd', IDS, true);
    expect([...next.selected].sort()).toEqual(['b', 'c', 'd']);
  });

  it('selects the range upward too', () => {
    // anchor at index 3 ('d'), shift-click 'a' (index 0) => a,b,c,d
    const next = toggleSelection(sel(['d'], 3), 'a', IDS, true);
    expect([...next.selected].sort()).toEqual(['a', 'b', 'c', 'd']);
  });

  it('is additive and preserves the anchor', () => {
    const start: SelectionState = { selected: new Set(['e', 'b']), anchorIndex: 1 };
    const next = toggleSelection(start, 'c', IDS, true);
    expect([...next.selected].sort()).toEqual(['b', 'c', 'e']);
    expect(next.anchorIndex).toBe(1);
  });

  it('falls back to plain toggle when there is no anchor', () => {
    const next = toggleSelection(emptySelection(), 'c', IDS, true);
    expect([...next.selected]).toEqual(['c']);
    expect(next.anchorIndex).toBe(2);
  });

  it('falls back to plain toggle when target id is unknown', () => {
    const next = toggleSelection(sel(['b'], 1), 'zzz', IDS, true);
    expect(next.selected.has('zzz')).toBe(true);
    expect(next.anchorIndex).toBe(1);
  });
});

describe('selectAll / hasSelection', () => {
  it('selects everything in view and resets the anchor', () => {
    const next = selectAll(IDS);
    expect([...next.selected].sort()).toEqual([...IDS].sort());
    expect(next.anchorIndex).toBeNull();
  });

  it('hasSelection reflects non-empty state', () => {
    expect(hasSelection(emptySelection())).toBe(false);
    expect(hasSelection(sel(['a']))).toBe(true);
  });
});
