import { describe, expect, it } from 'vitest';
import {
  isEditableTarget,
  normalizeShortcutKey,
  shouldFireShortcut,
  KO_KEY_MAP,
  type EditableTargetLike,
} from '../shortcutFocusGuard';

function el(overrides: Partial<EditableTargetLike>): EditableTargetLike {
  return { tagName: 'DIV', isContentEditable: false, ...overrides };
}

describe('normalizeShortcutKey', () => {
  it('maps Korean jamo to their Latin equivalents', () => {
    expect(normalizeShortcutKey('ㄱ')).toBe('r'); // reply
    expect(normalizeShortcutKey('ㄷ')).toBe('e'); // archive
    expect(normalizeShortcutKey('ㄹ')).toBe('f'); // forward
  });

  it('passes through Latin keys and unmapped keys unchanged', () => {
    expect(normalizeShortcutKey('r')).toBe('r');
    expect(normalizeShortcutKey('Escape')).toBe('Escape');
    expect(normalizeShortcutKey('#')).toBe('#');
    expect(normalizeShortcutKey('ArrowDown')).toBe('ArrowDown');
  });

  it('covers the full documented KO map', () => {
    for (const [jamo, latin] of Object.entries(KO_KEY_MAP)) {
      expect(normalizeShortcutKey(jamo)).toBe(latin);
    }
  });
});

describe('isEditableTarget', () => {
  it('treats INPUT, TEXTAREA and SELECT as editable (case-insensitive)', () => {
    expect(isEditableTarget(el({ tagName: 'INPUT' }))).toBe(true);
    expect(isEditableTarget(el({ tagName: 'textarea' }))).toBe(true);
    expect(isEditableTarget(el({ tagName: 'SELECT' }))).toBe(true);
  });

  it('treats contenteditable regions as editable', () => {
    expect(isEditableTarget(el({ tagName: 'DIV', isContentEditable: true }))).toBe(true);
    expect(
      isEditableTarget(el({ tagName: 'DIV', getAttribute: (n) => (n === 'contenteditable' ? 'true' : null) })),
    ).toBe(true);
  });

  it('treats plain elements as non-editable', () => {
    expect(isEditableTarget(el({ tagName: 'DIV' }))).toBe(false);
    expect(isEditableTarget(el({ tagName: 'BUTTON' }))).toBe(false);
    expect(isEditableTarget(null)).toBe(false);
    expect(isEditableTarget(undefined)).toBe(false);
  });

  it('does not walk ancestors by default but does when matchAncestors is set', () => {
    const nestedInInput = el({
      tagName: 'SPAN',
      closest: (sel: string) => (sel.includes('contenteditable') ? {} : null),
    });
    expect(isEditableTarget(nestedInInput)).toBe(false);
    expect(isEditableTarget(nestedInInput, true)).toBe(true);
  });
});

describe('shouldFireShortcut', () => {
  it('blocks shortcuts while typing into an input', () => {
    expect(shouldFireShortcut({ key: 'r', target: el({ tagName: 'INPUT' }) })).toBe(false);
    expect(shouldFireShortcut({ key: 'e', target: el({ tagName: 'TEXTAREA' }) })).toBe(false);
  });

  it('allows shortcuts on non-editable targets', () => {
    expect(shouldFireShortcut({ key: 'r', target: el({ tagName: 'DIV' }) })).toBe(true);
    expect(shouldFireShortcut({ key: 'j', target: el({ tagName: 'BUTTON' }) })).toBe(true);
  });

  it('always allows Escape through, even from within an input', () => {
    expect(shouldFireShortcut({ key: 'Escape', target: el({ tagName: 'INPUT' }) })).toBe(true);
    expect(shouldFireShortcut({ key: 'Escape', target: el({ tagName: 'TEXTAREA' }) })).toBe(true);
  });

  it('honors allowInEditable override', () => {
    expect(
      shouldFireShortcut({ key: 'r', target: el({ tagName: 'INPUT' }) }, { allowInEditable: true }),
    ).toBe(true);
  });

  it('respects matchAncestors for nested editable targets', () => {
    const nested = el({
      tagName: 'SPAN',
      closest: (sel: string) => (sel.includes('contenteditable') ? {} : null),
    });
    expect(shouldFireShortcut({ key: '#', target: nested })).toBe(true);
    expect(shouldFireShortcut({ key: '#', target: nested }, { matchAncestors: true })).toBe(false);
  });

  it('treats a missing target as non-editable (shortcut fires)', () => {
    expect(shouldFireShortcut({ key: 'j' })).toBe(true);
  });
});
