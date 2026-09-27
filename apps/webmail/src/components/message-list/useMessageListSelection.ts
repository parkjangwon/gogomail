'use client';

import { useState, useRef, useEffect, type MutableRefObject, type KeyboardEvent as ReactKeyboardEvent } from 'react';
import { MessageSummary } from '@/lib/api';
import { moveNavFocus } from '@/lib/navKeyboard';
import { isEditableTarget, normalizeShortcutKey } from '@/lib/keyboard/shortcutFocusGuard';
import { toggleSelection } from '@/lib/mail/bulkSelection';

export interface UseMessageListSelectionOptions {
  filteredMessages: MessageSummary[];
  messages: MessageSummary[];
  onSelect: (id: string) => void;
  onToggleReadMessage?: (id: string, read: boolean) => void;
  onArchiveMessage?: (id: string) => void;
  onSnoozeMessage?: (id: string, until: Date) => void;
  onPinMessage?: (id: string) => void;
  onDeleteMessage?: (id: string) => void;
  onBulkDelete?: (ids: string[]) => void;
}

export interface UseMessageListSelectionResult {
  bulkSelected: Set<string>;
  toggleBulk: (id: string, shiftKey?: boolean) => void;
  selectAll: () => void;
  clearAll: () => void;
  getActionMessages: (ids: string[]) => MessageSummary[];
  handleRowKeyDownCapture: (event: ReactKeyboardEvent<HTMLDivElement>) => void;
  hoveredMessageIdRef: MutableRefObject<string | null>;
}

export function useMessageListSelection({
  filteredMessages,
  messages,
  onSelect,
  onToggleReadMessage,
  onArchiveMessage,
  onSnoozeMessage,
  onPinMessage,
  onDeleteMessage,
  onBulkDelete,
}: UseMessageListSelectionOptions): UseMessageListSelectionResult {
  const [bulkSelected, setBulkSelected] = useState<Set<string>>(new Set());
  const lastBulkIndexRef = useRef<number | null>(null);
  const hoveredMessageIdRef = useRef<string | null>(null);

  const toggleBulk = (id: string, shiftKey?: boolean) => {
    const orderedIds = filteredMessages.map((m) => m.id);
    setBulkSelected((prev: Set<string>) => {
      const result = toggleSelection(
        { selected: prev, anchorIndex: lastBulkIndexRef.current },
        id,
        orderedIds,
        shiftKey ?? false,
      );
      lastBulkIndexRef.current = result.anchorIndex;
      return result.selected;
    });
  };

  const selectAll = () => setBulkSelected(new Set(filteredMessages.map((m) => m.id)));
  const clearAll = () => { setBulkSelected(new Set()); lastBulkIndexRef.current = null; };

  const getActionMessages = (ids: string[]) => ids
    .map((id) => filteredMessages.find((m) => m.id === id) ?? messages.find((m) => m.id === id))
    .filter((m): m is MessageSummary => Boolean(m));

  const runActionForIds = (ids: string[], action: (id: string) => void, clearAfter = false) => {
    ids.forEach(action);
    if (clearAfter) clearAll();
  };
  const toggleReadForIds = (ids: string[], read: boolean, clearAfter = false) => {
    ids.forEach((id) => onToggleReadMessage?.(id, read));
    if (clearAfter) clearAll();
  };
  const snoozeIdsForOneHour = (ids: string[], clearAfter = false) => {
    const until = new Date(Date.now() + 60 * 60 * 1000);
    ids.forEach((id) => onSnoozeMessage?.(id, until));
    if (clearAfter) clearAll();
  };

  // Row-level keyboard navigation (Arrow/j/k/Home/End/Space/Enter/o)
  const handleRowKeyDownCapture = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement | null;
    if (!target) return;
    if (target.closest('button, a, input, textarea, select, [role="button"]')) return;

    const row = target.closest<HTMLElement>('[data-message-id]');
    if (!row) return;

    if (event.key === 'ArrowDown' || event.key === 'j') {
      event.preventDefault();
      event.stopPropagation();
      moveNavFocus(row, 'next', 'message-list');
      return;
    }

    if (event.key === 'ArrowUp' || event.key === 'k') {
      event.preventDefault();
      event.stopPropagation();
      moveNavFocus(row, 'prev', 'message-list');
      return;
    }

    if (event.key === 'Home') {
      event.preventDefault();
      event.stopPropagation();
      moveNavFocus(row, 'first', 'message-list');
      return;
    }

    if (event.key === 'End') {
      event.preventDefault();
      event.stopPropagation();
      moveNavFocus(row, 'last', 'message-list');
      return;
    }

    if (event.key === ' ' || event.key === 'Spacebar') {
      event.preventDefault();
      event.stopPropagation();
      const id = row.dataset.messageId;
      if (id) toggleBulk(id, event.shiftKey);
      return;
    }

    if (event.key === 'Enter' || event.key === 'o') {
      event.preventDefault();
      event.stopPropagation();
      const id = row.dataset.messageId;
      if (id) onSelect(id);
    }
  };

  // Escape clears bulk selection
  const bulkSize = bulkSelected.size;
  const clearAllRef = useRef(clearAll);
  const selectAllRef = useRef(selectAll);
  useEffect(() => { clearAllRef.current = clearAll; selectAllRef.current = selectAll; });
  useEffect(() => {
    if (bulkSize === 0) return;
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') { e.stopPropagation(); clearAllRef.current(); }
    };
    window.addEventListener('keydown', handler, { capture: true });
    return () => window.removeEventListener('keydown', handler, { capture: true });
  }, [bulkSize]);

  // Ctrl/Cmd+A selects all
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey) || e.key !== 'a') return;
      const tag = (e.target as HTMLElement).tagName;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || (e.target as HTMLElement).isContentEditable) return;
      e.preventDefault();
      selectAllRef.current();
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, []);

  // Global action shortcuts (m/e/z/p/#/Delete) for hovered or bulk messages
  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      const target = event.target as HTMLElement | null;
      if (isEditableTarget(target, true)) return;
      const bulkIds = [...bulkSelected];
      const ids = bulkIds.length > 0 ? bulkIds : hoveredMessageIdRef.current ? [hoveredMessageIdRef.current] : [];
      if (ids.length === 0) return;
      const actionMessages = getActionMessages(ids);
      if (actionMessages.length === 0) return;
      const lowerKey = normalizeShortcutKey(event.key).toLowerCase();
      const isBulkAction = bulkIds.length > 0;
      const finish = (run: () => void) => {
        event.preventDefault();
        event.stopPropagation();
        event.stopImmediatePropagation();
        run();
      };

      if (lowerKey === 'm' && onToggleReadMessage) {
        const readTarget = actionMessages.some((m) => !m.read);
        finish(() => toggleReadForIds(ids, readTarget, isBulkAction));
        return;
      }
      if (lowerKey === 'e' && onArchiveMessage) {
        finish(() => runActionForIds(ids, onArchiveMessage, isBulkAction));
        return;
      }
      if (lowerKey === 'z' && onSnoozeMessage) {
        finish(() => snoozeIdsForOneHour(ids, isBulkAction));
        return;
      }
      if (lowerKey === 'p' && onPinMessage) {
        finish(() => runActionForIds(ids, onPinMessage, isBulkAction));
        return;
      }
      if ((event.key === '#' || event.key === 'Delete') && onDeleteMessage) {
        finish(() => {
          if (isBulkAction && onBulkDelete) {
            onBulkDelete(ids);
            clearAll();
          } else {
            runActionForIds(ids, onDeleteMessage);
          }
        });
      }
    };
    window.addEventListener('keydown', handler, { capture: true });
    return () => window.removeEventListener('keydown', handler, { capture: true });
  }, [bulkSelected, filteredMessages, messages, onToggleReadMessage, onArchiveMessage, onSnoozeMessage, onPinMessage, onDeleteMessage, onBulkDelete]);

  return {
    bulkSelected,
    toggleBulk,
    selectAll,
    clearAll,
    getActionMessages,
    handleRowKeyDownCapture,
    hoveredMessageIdRef,
  };
}
