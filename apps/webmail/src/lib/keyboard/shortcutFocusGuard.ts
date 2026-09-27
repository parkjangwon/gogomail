// Pure, framework-agnostic helpers that decide whether a keyboard shortcut is
// allowed to fire, and normalize Korean-IME key values back to their Latin
// equivalents. These functions have no React/DOM-lifecycle dependencies so they
// can be unit-tested in isolation (see __tests__/shortcutFocusGuard.test.ts).
//
// The behavior here is intentionally identical to the inline guards that used
// to live in useMailKeyboardShortcuts.ts and useMessageListSelection.ts — those
// hooks now delegate to this module so the focus-guard rule lives in one place.

// Korean QWERTY → Latin normalization so shortcuts still work while a Korean
// IME is active (the browser reports the composed jamo as event.key).
export const KO_KEY_MAP: Record<string, string> = {
  'ㄷ': 'e', 'ㄱ': 'r', 'ㅅ': 't', 'ㅛ': 'y', 'ㅕ': 'u', 'ㅑ': 'i', 'ㅐ': 'o', 'ㅔ': 'p',
  'ㅁ': 'a', 'ㄴ': 's', 'ㅇ': 'd', 'ㄹ': 'f', 'ㅎ': 'g', 'ㅗ': 'h', 'ㅓ': 'j', 'ㅏ': 'k', 'ㅣ': 'l',
  'ㅋ': 'z', 'ㅌ': 'x', 'ㅊ': 'c', 'ㅍ': 'v', 'ㅠ': 'b', 'ㅜ': 'n', 'ㅡ': 'm',
  'ㅂ': 'q', 'ㅈ': 'w',
};

/** Normalize a raw KeyboardEvent.key, mapping Korean jamo to Latin letters. */
export function normalizeShortcutKey(key: string): string {
  return KO_KEY_MAP[key] ?? key;
}

// The minimal shape of an element we need to inspect. Keeping this structural
// (rather than requiring a real HTMLElement) lets tests pass plain objects and
// keeps the module usable in any environment.
export interface EditableTargetLike {
  tagName?: string;
  isContentEditable?: boolean;
  getAttribute?: (name: string) => string | null;
  closest?: (selector: string) => unknown;
}

const EDITABLE_TAGS = new Set(['INPUT', 'TEXTAREA', 'SELECT']);

/**
 * True when the event target is (or is inside) a field that swallows typing —
 * text inputs, textareas, selects, or contenteditable regions. Shortcuts must
 * NOT fire while the user is typing into such a field.
 *
 * `matchAncestors` controls whether we walk up the DOM (via closest) to catch
 * targets nested inside an editable container (e.g. a span inside a
 * contenteditable div). The window-level guard uses direct-target matching to
 * mirror the previous inline behavior; the capture-phase list guard walks
 * ancestors.
 */
export function isEditableTarget(
  target: EditableTargetLike | null | undefined,
  matchAncestors = false,
): boolean {
  if (!target) return false;

  const tag = (target.tagName ?? '').toUpperCase();
  if (EDITABLE_TAGS.has(tag)) return true;
  if (target.isContentEditable === true) return true;
  if (target.getAttribute?.('contenteditable') === 'true') return true;

  if (matchAncestors && typeof target.closest === 'function') {
    if (target.closest('input, textarea, select, [contenteditable="true"]')) return true;
  }

  return false;
}

/**
 * Central decision: should a keyboard shortcut be allowed to fire for this
 * event? Shortcuts are blocked while typing into an editable field. `Escape` is
 * always allowed through so dialogs can close even from within an input.
 */
export function shouldFireShortcut(
  event: { key: string; target?: EditableTargetLike | null },
  options: { allowInEditable?: boolean; matchAncestors?: boolean } = {},
): boolean {
  const { allowInEditable = false, matchAncestors = false } = options;
  // Escape must always be deliverable so modal/dialog close handlers work even
  // when focus is inside an input.
  if (event.key === 'Escape') return true;
  if (allowInEditable) return true;
  return !isEditableTarget(event.target ?? null, matchAncestors);
}
