import { useState, useEffect, useCallback, useRef } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';
import { listDMRooms } from '@/lib/api';
import {
  DM_MODAL_MIN_WIDTH,
  DM_MODAL_MIN_HEIGHT,
  DM_MODAL_MARGIN,
  DM_RESIZE_HANDLES,
  getDefaultDMModalRect,
  type DMModalRect,
  type DMResizeEdge,
} from './mailPageHelpers';

export interface UseDMModalParams {
  isMobile: boolean;
}

export function useDMModal({ isMobile }: UseDMModalParams) {
  const [showDMModal, setShowDMModal] = useState(false);
  const [dmModalRect, setDMModalRect] = useState<DMModalRect | null>(null);
  const [dmUnreadCount, setDMUnreadCount] = useState(0);
  const dmModalRectRef = useRef<DMModalRect | null>(null);

  // Poll DM unread count
  useEffect(() => {
    let cancelled = false;
    const refreshDMUnread = () => {
      listDMRooms()
        .then((dmRooms) => {
          if (!cancelled) setDMUnreadCount(dmRooms.reduce((sum, room) => sum + (room.unread_count ?? 0), 0));
        })
        .catch(() => {
          if (!cancelled) setDMUnreadCount(0);
        });
    };
    refreshDMUnread();
    const id = window.setInterval(() => {
      if (document.visibilityState === 'visible') refreshDMUnread();
    }, 5000);
    return () => { cancelled = true; window.clearInterval(id); };
  }, []);

  const clampDMModalRect = useCallback((rect: DMModalRect): DMModalRect => {
    if (typeof window === 'undefined') return rect;
    const viewportWidth = window.innerWidth;
    const viewportHeight = window.innerHeight;
    const minWidth = Math.min(DM_MODAL_MIN_WIDTH, Math.max(280, viewportWidth - DM_MODAL_MARGIN * 2));
    const minHeight = Math.min(DM_MODAL_MIN_HEIGHT, Math.max(300, viewportHeight - DM_MODAL_MARGIN * 2));
    const maxWidth = Math.max(minWidth, viewportWidth - DM_MODAL_MARGIN * 2);
    const maxHeight = Math.max(minHeight, viewportHeight - DM_MODAL_MARGIN * 2);
    const width = Math.min(Math.max(rect.width, minWidth), maxWidth);
    const height = Math.min(Math.max(rect.height, minHeight), maxHeight);
    const maxLeft = Math.max(DM_MODAL_MARGIN, viewportWidth - width - DM_MODAL_MARGIN);
    const maxTop = Math.max(DM_MODAL_MARGIN, viewportHeight - height - DM_MODAL_MARGIN);
    const left = Math.min(Math.max(rect.left, DM_MODAL_MARGIN), maxLeft);
    const top = Math.min(Math.max(rect.top, DM_MODAL_MARGIN), maxTop);
    return { left, top, width, height };
  }, []);

  // Keep ref in sync
  useEffect(() => {
    dmModalRectRef.current = dmModalRect;
  }, [dmModalRect]);

  // Clamp rect when modal opens
  useEffect(() => {
    if (!showDMModal || isMobile) return;
    setDMModalRect((rect) => clampDMModalRect(rect ?? getDefaultDMModalRect()));
  }, [showDMModal, isMobile, clampDMModalRect]);

  // Re-clamp on viewport resize
  useEffect(() => {
    if (isMobile) return undefined;
    const handleViewportResize = () => setDMModalRect((rect) => (rect ? clampDMModalRect(rect) : rect));
    window.addEventListener('resize', handleViewportResize);
    return () => window.removeEventListener('resize', handleViewportResize);
  }, [isMobile, clampDMModalRect]);

  const startDMModalResize = useCallback((edge: DMResizeEdge, event: ReactMouseEvent<HTMLDivElement>) => {
    if (isMobile) return;
    event.preventDefault();
    event.stopPropagation();
    const startRect = dmModalRectRef.current ?? clampDMModalRect(getDefaultDMModalRect());
    const startX = event.clientX;
    const startY = event.clientY;
    const previousCursor = document.body.style.cursor;
    const previousUserSelect = document.body.style.userSelect;
    const cursor = DM_RESIZE_HANDLES.find((handle) => handle.edge === edge)?.cursor ?? 'default';
    document.body.style.cursor = cursor;
    document.body.style.userSelect = 'none';

    const handleMove = (moveEvent: MouseEvent) => {
      const dx = moveEvent.clientX - startX;
      const dy = moveEvent.clientY - startY;
      let { left, top, width, height } = startRect;
      if (edge.includes('e')) width += dx;
      if (edge.includes('s')) height += dy;
      if (edge.includes('w')) {
        width -= dx;
        left += dx;
      }
      if (edge.includes('n')) {
        height -= dy;
        top += dy;
      }
      setDMModalRect(clampDMModalRect({ left, top, width, height }));
    };

    const stopResize = () => {
      document.removeEventListener('mousemove', handleMove);
      document.removeEventListener('mouseup', stopResize);
      document.body.style.cursor = previousCursor;
      document.body.style.userSelect = previousUserSelect;
    };

    document.addEventListener('mousemove', handleMove);
    document.addEventListener('mouseup', stopResize);
  }, [isMobile, clampDMModalRect]);

  const startDMModalDrag = useCallback((event: ReactMouseEvent<HTMLElement>) => {
    if (isMobile) return;
    event.preventDefault();
    event.stopPropagation();
    const startRect = dmModalRectRef.current ?? clampDMModalRect(getDefaultDMModalRect());
    const offsetX = event.clientX - startRect.left;
    const offsetY = event.clientY - startRect.top;
    const previousCursor = document.body.style.cursor;
    const previousUserSelect = document.body.style.userSelect;
    document.body.style.cursor = 'move';
    document.body.style.userSelect = 'none';

    const handleMove = (moveEvent: MouseEvent) => {
      setDMModalRect(clampDMModalRect({
        ...startRect,
        left: moveEvent.clientX - offsetX,
        top: moveEvent.clientY - offsetY,
      }));
    };

    const stopDrag = () => {
      document.removeEventListener('mousemove', handleMove);
      document.removeEventListener('mouseup', stopDrag);
      document.body.style.cursor = previousCursor;
      document.body.style.userSelect = previousUserSelect;
    };

    document.addEventListener('mousemove', handleMove);
    document.addEventListener('mouseup', stopDrag);
  }, [isMobile, clampDMModalRect]);

  return {
    showDMModal,
    setShowDMModal,
    dmModalRect,
    setDMModalRect,
    dmUnreadCount,
    setDMUnreadCount,
    startDMModalResize,
    startDMModalDrag,
  };
}
