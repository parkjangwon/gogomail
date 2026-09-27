'use client';

import { useEffect, useState } from 'react';

const BREAKPOINT = 768;

function matchesMobile(): boolean {
  try {
    return window.matchMedia(`(max-width: ${BREAKPOINT}px)`).matches;
  } catch {
    return false;
  }
}

export function useIsMobile(): boolean {
  // Initialize from matchMedia so the first paint already uses the right
  // layout — otherwise phones (e.g. a folded Fold7) flash the desktop
  // 3-pane chrome before the effect flips it to mobile.
  const [isMobile, setIsMobile] = useState<boolean>(matchesMobile);

  useEffect(() => {
    const mq = window.matchMedia(`(max-width: ${BREAKPOINT}px)`);
    setIsMobile(mq.matches);
    const handler = (e: MediaQueryListEvent) => setIsMobile(e.matches);
    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, []);

  return isMobile;
}
