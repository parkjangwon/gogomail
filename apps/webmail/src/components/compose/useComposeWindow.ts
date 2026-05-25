'use client';

import { useState, useRef } from 'react';

interface UseComposeWindowOptions {
  isMobile: boolean | undefined;
}

interface ComposeWindowState {
  pos: { x: number; y: number } | null;
  setPos: React.Dispatch<React.SetStateAction<{ x: number; y: number } | null>>;
  size: { w: number; h: number };
  setSize: React.Dispatch<React.SetStateAction<{ w: number; h: number }>>;
  minimized: boolean;
  setMinimized: React.Dispatch<React.SetStateAction<boolean>>;
  fullscreen: boolean;
  setFullscreen: React.Dispatch<React.SetStateAction<boolean>>;
  dialogRef: React.RefObject<HTMLDivElement | null>;
  startDrag: (e: React.MouseEvent<HTMLDivElement>) => void;
  startResize: (e: React.MouseEvent, dir: string) => void;
}

export function useComposeWindow({ isMobile }: UseComposeWindowOptions): ComposeWindowState {
  const dialogRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null);
  const [size, setSize] = useState<{ w: number; h: number }>(() => {
    try {
      const s = localStorage.getItem('webmail_compose_size');
      const parsed = s ? JSON.parse(s) : { w: 560, h: 520 };
      const maxH = typeof window !== 'undefined' ? window.innerHeight - 60 : 800;
      return { w: parsed.w, h: Math.min(parsed.h, maxH) };
    } catch { return { w: 560, h: 520 }; }
  });
  const [minimized, setMinimized] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);

  function startDrag(e: React.MouseEvent<HTMLDivElement>) {
    if (fullscreen || minimized || isMobile) return;
    const dialog = dialogRef.current;
    if (!dialog) return;
    const rect = dialog.getBoundingClientRect();
    const curX = pos?.x ?? rect.left;
    const curY = pos?.y ?? rect.top;
    const offsetX = e.clientX - curX;
    const offsetY = e.clientY - curY;
    function onMove(ev: MouseEvent) {
      const nx = Math.max(0, Math.min(ev.clientX - offsetX, window.innerWidth - size.w));
      const ny = Math.max(0, Math.min(ev.clientY - offsetY, window.innerHeight - size.h));
      setPos({ x: nx, y: ny });
    }
    function onUp() {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    }
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }

  function startResize(e: React.MouseEvent, dir: string) {
    e.preventDefault();
    e.stopPropagation();
    const dialog = dialogRef.current;
    if (!dialog) return;
    const rect = dialog.getBoundingClientRect();
    const startX = e.clientX, startY = e.clientY;
    const startW = rect.width, startH = rect.height;
    const startL = rect.left, startT = rect.top;
    function onMove(ev: MouseEvent) {
      let nw = startW, nh = startH;
      let nx = pos?.x ?? startL, ny = pos?.y ?? startT;
      if (dir.includes('e')) nw = Math.max(400, startW + ev.clientX - startX);
      if (dir.includes('s')) nh = Math.max(300, startH + ev.clientY - startY);
      if (dir.includes('w')) { nw = Math.max(400, startW - (ev.clientX - startX)); nx = startL + (startW - nw); }
      if (dir.includes('n')) { nh = Math.max(300, startH - (ev.clientY - startY)); ny = startT + (startH - nh); }
      setSize({ w: nw, h: nh });
      if (dir.includes('w') || dir.includes('n')) setPos({ x: nx, y: ny });
    }
    function onUp() {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
      setSize((s) => {
        try { localStorage.setItem('webmail_compose_size', JSON.stringify(s)); } catch { /* */ }
        return s;
      });
    }
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }

  return { pos, setPos, size, setSize, minimized, setMinimized, fullscreen, setFullscreen, dialogRef, startDrag, startResize };
}
