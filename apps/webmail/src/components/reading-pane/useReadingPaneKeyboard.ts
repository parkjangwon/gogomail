'use client';

import { useCallback, type RefObject, type KeyboardEvent as ReactKeyboardEvent } from 'react';

type ComposeIntent = 'reply' | 'reply_all' | 'forward';

interface UseReadingPaneKeyboardParams {
  scrollContainerRef: RefObject<HTMLElement | null>;
  onBack?: () => void;
  onDelete?: () => void;
  onStar?: () => void;
  onArchive?: () => void;
  onToggleRead?: () => void;
  onOpenFullCompose: (intent: ComposeIntent) => void;
}

export function useReadingPaneKeyboard({
  scrollContainerRef,
  onBack,
  onDelete,
  onStar,
  onArchive,
  onToggleRead,
  onOpenFullCompose,
}: UseReadingPaneKeyboardParams) {
  const handleReadingPaneKeyDown = useCallback(
    (event: ReactKeyboardEvent<HTMLElement>) => {
      if ((event.target as HTMLElement | null)?.closest('input, textarea, select, [contenteditable="true"]')) return;
      if (event.metaKey || event.ctrlKey || event.altKey) return;

      const container = scrollContainerRef.current;
      const stop = () => {
        event.preventDefault();
        event.stopPropagation();
        event.nativeEvent.stopImmediatePropagation?.();
      };
      const scrollBy = (top: number) => {
        stop();
        container?.scrollBy({ top, behavior: 'smooth' });
      };
      const scrollTo = (top: number) => {
        stop();
        container?.scrollTo({ top, behavior: 'smooth' });
      };

      if (event.key === 'ArrowDown') { scrollBy(80); return; }
      if (event.key === 'ArrowUp') { scrollBy(-80); return; }
      if (event.key === 'PageDown') {
        scrollBy(Math.max(120, (container?.clientHeight ?? 0) * 0.85));
        return;
      }
      if (event.key === 'PageUp') {
        scrollBy(-Math.max(120, (container?.clientHeight ?? 0) * 0.85));
        return;
      }
      if (event.key === 'Home') { scrollTo(0); return; }
      if (event.key === 'End') { scrollTo(container?.scrollHeight ?? 0); return; }
      if (event.key === 'Escape') {
        if (!onBack) return;
        stop();
        onBack();
        return;
      }
      if (event.key === 'Delete' || event.key === 'Backspace' || event.key === '#') {
        if (!onDelete) return;
        stop();
        onDelete();
        return;
      }

      const key = event.key.toLowerCase();
      if (key === 'r') { stop(); onOpenFullCompose('reply'); return; }
      if (key === 'a') { stop(); onOpenFullCompose('reply_all'); return; }
      if (key === 'f') { stop(); onOpenFullCompose('forward'); return; }
      if (key === 's') { if (!onStar) return; stop(); onStar(); return; }
      if (key === 'e') { if (!onArchive) return; stop(); onArchive(); return; }
      if (key === 'm') { if (!onToggleRead) return; stop(); onToggleRead(); return; }
    },
    [scrollContainerRef, onBack, onDelete, onStar, onArchive, onToggleRead, onOpenFullCompose],
  );

  return { handleReadingPaneKeyDown };
}
