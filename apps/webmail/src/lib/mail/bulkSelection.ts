// Pure, testable bulk-selection reducers for the message list. These contain no
// React state or DOM access so they can be unit-tested directly (see
// __tests__/bulkSelection.test.ts). useMessageListSelection.ts and
// MessageList.tsx delegate their toggle/range/select-all logic here so the
// selection semantics live in exactly one place.

export interface SelectionState {
  /** Currently selected message ids. */
  selected: Set<string>;
  /** Index (into the ordered id list) of the last individually-toggled row.
   *  Used as the anchor for shift-click range selection. null when no anchor. */
  anchorIndex: number | null;
}

export function emptySelection(): SelectionState {
  return { selected: new Set<string>(), anchorIndex: null };
}

/**
 * Toggle a single id, or — when `shiftKey` is set and an anchor exists — add the
 * contiguous range between the anchor and the target id to the selection.
 *
 * `orderedIds` is the visible message order (post-filter/sort). Shift-range uses
 * this order so the range matches what the user sees.
 *
 * Behavior mirrors the previous inline implementation:
 *  - Plain toggle flips membership of the id and moves the anchor to it.
 *  - Shift+toggle with a valid anchor unions the [anchor..target] range into the
 *    existing selection (additive; never removes) and does NOT move the anchor.
 *  - Shift+toggle without a valid anchor (or unknown id) falls back to a plain
 *    toggle.
 */
export function toggleSelection(
  state: SelectionState,
  id: string,
  orderedIds: string[],
  shiftKey = false,
): SelectionState {
  const idx = orderedIds.indexOf(id);

  if (shiftKey && state.anchorIndex !== null && idx !== -1) {
    const from = Math.min(state.anchorIndex, idx);
    const to = Math.max(state.anchorIndex, idx);
    const next = new Set(state.selected);
    for (let i = from; i <= to; i++) {
      const rid = orderedIds[i];
      if (rid !== undefined) next.add(rid);
    }
    return { selected: next, anchorIndex: state.anchorIndex };
  }

  const next = new Set(state.selected);
  if (next.has(id)) next.delete(id);
  else next.add(id);
  return { selected: next, anchorIndex: idx !== -1 ? idx : state.anchorIndex };
}

/** Select every id currently in view. Anchor is reset. */
export function selectAll(orderedIds: string[]): SelectionState {
  return { selected: new Set(orderedIds), anchorIndex: null };
}

/** Clear the whole selection and the anchor. */
export function clearSelection(): SelectionState {
  return emptySelection();
}

/** Convenience: is anything selected? */
export function hasSelection(state: SelectionState): boolean {
  return state.selected.size > 0;
}
