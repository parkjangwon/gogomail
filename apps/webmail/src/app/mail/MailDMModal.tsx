'use client';

import type { MouseEvent } from 'react';
import { DMPanel } from '@/components/DMPanel';
import {
  DM_MODAL_MIN_WIDTH,
  DM_MODAL_MIN_HEIGHT,
  DM_RESIZE_HANDLES,
  type DMModalRect,
  type DMResizeEdge,
} from './mailPageHelpers';

interface MailDMModalProps {
  isMobile: boolean;
  rect: DMModalRect;
  userEmail: string | undefined;
  onUnreadChange: (count: number) => void;
  onClose: () => void;
  onComposeToAddress: (email: string) => void;
  onStartWindowDrag: (e: MouseEvent<HTMLElement>) => void;
  onStartResize: (edge: DMResizeEdge, e: MouseEvent<HTMLDivElement>) => void;
}

export function MailDMModal({
  isMobile,
  rect,
  userEmail,
  onUnreadChange,
  onClose,
  onComposeToAddress,
  onStartWindowDrag,
  onStartResize,
}: MailDMModalProps) {
  return (
    <div
      role="dialog"
      aria-modal="false"
      aria-label="DM"
      style={{
        position: 'fixed',
        ...(isMobile
          ? { inset: 0, width: '100%', height: '100dvh', borderRadius: 0 }
          : { left: rect.left, top: rect.top, width: rect.width, height: rect.height, minWidth: `min(${DM_MODAL_MIN_WIDTH}px, calc(100vw - 24px))`, minHeight: `min(${DM_MODAL_MIN_HEIGHT}px, calc(100vh - 24px))`, maxWidth: 'calc(100vw - 24px)', maxHeight: 'calc(100vh - 24px)', borderRadius: 8 }),
        zIndex: 120,
        overflow: 'hidden',
        background: 'var(--color-bg-primary)',
        border: isMobile ? 'none' : '1px solid var(--color-border-default)',
        boxShadow: isMobile ? 'none' : '0 12px 42px rgba(0,0,0,0.20)',
        display: 'flex',
        animation: 'composeIn 120ms ease-out',
      }}
    >
      {!isMobile && DM_RESIZE_HANDLES.map((handle) => (
        <div
          key={handle.edge}
          aria-hidden="true"
          onMouseDown={(event) => onStartResize(handle.edge, event)}
          style={{
            position: 'absolute',
            zIndex: 4,
            cursor: handle.cursor,
            ...handle.style,
          }}
        />
      ))}
      <DMPanel
        userEmail={userEmail}
        onUnreadChange={onUnreadChange}
        onClose={onClose}
        onComposeToAddress={onComposeToAddress}
        onStartWindowDrag={onStartWindowDrag}
      />
    </div>
  );
}
