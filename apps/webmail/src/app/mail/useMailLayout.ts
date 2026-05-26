import { useState, useRef, type Dispatch, type SetStateAction, type MutableRefObject } from 'react';

interface UseMailLayoutReturn {
  mobileSidebarOpen: boolean;
  setMobileSidebarOpen: Dispatch<SetStateAction<boolean>>;
  sidebarCollapsed: boolean;
  setSidebarCollapsed: Dispatch<SetStateAction<boolean>>;
  sidebarWidth: number;
  setSidebarWidth: Dispatch<SetStateAction<number>>;
  readingPaneWidth: number;
  setReadingPaneWidth: Dispatch<SetStateAction<number>>;
  swipeDeltaX: number;
  setSwipeDeltaX: Dispatch<SetStateAction<number>>;
  swipeTouchStartRef: MutableRefObject<number | null>;
}

export function useMailLayout(): UseMailLayoutReturn {
  const [mobileSidebarOpen, setMobileSidebarOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [sidebarWidth, setSidebarWidth] = useState(() => {
    try { return parseInt(localStorage.getItem('webmail_sidebar_width') ?? '220', 10) || 220; } catch { return 220; }
  });
  const [readingPaneWidth, setReadingPaneWidth] = useState(() => {
    try { return parseInt(localStorage.getItem('webmail_reading_pane_width') ?? '0', 10) || 0; } catch { return 0; }
  });
  const [swipeDeltaX, setSwipeDeltaX] = useState(0);
  const swipeTouchStartRef = useRef<number | null>(null);

  return {
    mobileSidebarOpen,
    setMobileSidebarOpen,
    sidebarCollapsed,
    setSidebarCollapsed,
    sidebarWidth,
    setSidebarWidth,
    readingPaneWidth,
    setReadingPaneWidth,
    swipeDeltaX,
    setSwipeDeltaX,
    swipeTouchStartRef,
  };
}
