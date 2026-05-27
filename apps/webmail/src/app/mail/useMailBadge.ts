'use client';

import { useEffect } from 'react';
import { type Folder } from '@/lib/api';
import { type NavigatorWithBadging } from './mailPageHelpers';

interface UseMailBadgeParams {
  folders: Folder[];
  badgeCountMode: 'unread' | 'all' | 'none';
}

export function useMailBadge({ folders, badgeCountMode }: UseMailBadgeParams) {
  useEffect(() => {
    const totalUnread = folders.reduce((sum, f) => sum + (f.unread ?? 0), 0);
    const totalMessages = folders.reduce((sum, f) => sum + (f.total ?? 0), 0);
    const badgeCount = badgeCountMode === 'none' ? 0 : badgeCountMode === 'all' ? totalMessages : totalUnread;
    document.title = badgeCount > 0 ? `GoGoMail (${badgeCount})` : 'GoGoMail';
    const badging = navigator as NavigatorWithBadging;
    if (badgeCount > 0 && typeof badging.setAppBadge === 'function') {
      void badging.setAppBadge(badgeCount).catch(() => {});
    } else if (badgeCount === 0 && typeof badging.clearAppBadge === 'function') {
      void badging.clearAppBadge().catch(() => {});
    }

    // Draw favicon with optional badge on 32x32 canvas
    try {
      const size = 32;
      const canvas = document.createElement('canvas');
      canvas.width = size; canvas.height = size;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;
      // Envelope icon
      ctx.fillStyle = '#6366f1';
      ctx.beginPath();
      ctx.roundRect(2, 6, 28, 20, 3);
      ctx.fill();
      ctx.fillStyle = '#fff';
      ctx.beginPath();
      ctx.moveTo(2, 8); ctx.lineTo(16, 18); ctx.lineTo(30, 8);
      ctx.strokeStyle = '#fff'; ctx.lineWidth = 2; ctx.stroke();
      // Badge
      if (badgeCount > 0) {
        const label = badgeCount > 99 ? '99+' : String(badgeCount);
        const badgeR = label.length > 2 ? 9 : 7;
        const bx = size - badgeR - 1, by = badgeR + 1;
        ctx.fillStyle = '#ef4444';
        ctx.beginPath(); ctx.arc(bx, by, badgeR, 0, Math.PI * 2); ctx.fill();
        ctx.fillStyle = '#fff';
        ctx.font = `bold ${label.length > 2 ? 7 : 9}px sans-serif`;
        ctx.textAlign = 'center'; ctx.textBaseline = 'middle';
        ctx.fillText(label, bx, by + 0.5);
      }
      let link = document.querySelector<HTMLLinkElement>('link[rel~="icon"]');
      if (!link) { link = document.createElement('link'); link.rel = 'icon'; document.head.appendChild(link); }
      link.href = canvas.toDataURL('image/png');
    } catch { /* canvas not supported */ }
  }, [folders, badgeCountMode]);
}
