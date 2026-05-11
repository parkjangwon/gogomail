'use client';

import { useEffect } from 'react';

export interface ToastItem {
  id: string;
  message: string;
  type: 'success' | 'error' | 'info';
  duration?: number;
  action?: { label: string; onClick: () => void };
}

interface ToastProps {
  toasts: ToastItem[];
  onDismiss: (id: string) => void;
}

export function ToastContainer({ toasts, onDismiss }: ToastProps) {
  return (
    <div
      aria-live="polite"
      aria-atomic="false"
      style={{
        position: 'fixed',
        bottom: '24px',
        left: '50%',
        transform: 'translateX(-50%)',
        zIndex: 500,
        display: 'flex',
        flexDirection: 'column',
        gap: '8px',
        pointerEvents: 'none',
      }}
    >
      {toasts.map((t) => (
        <ToastMessage key={t.id} toast={t} onDismiss={onDismiss} />
      ))}
    </div>
  );
}

function ToastMessage({ toast, onDismiss }: { toast: ToastItem; onDismiss: (id: string) => void }) {
  useEffect(() => {
    const timer = setTimeout(() => onDismiss(toast.id), toast.duration ?? 3000);
    return () => clearTimeout(timer);
  }, [toast.id, onDismiss, toast.duration]);

  const bg =
    toast.type === 'success' ? 'var(--color-success, #16a34a)'
    : toast.type === 'error' ? 'var(--color-destructive, #dc2626)'
    : 'var(--color-bg-tertiary)';

  const color = toast.type === 'info' ? 'var(--color-text-primary)' : '#fff';

  return (
    <div
      role="status"
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '12px',
        padding: '9px 16px',
        borderRadius: '6px',
        background: bg,
        color,
        fontSize: '13px',
        fontWeight: 500,
        boxShadow: '0 2px 8px rgba(0,0,0,0.16)',
        pointerEvents: 'auto',
        animation: 'toastIn 150ms ease-out',
        whiteSpace: 'nowrap',
      }}
    >
      <style>{`@keyframes toastIn { from { opacity:0; transform:translateY(8px); } to { opacity:1; transform:translateY(0); } }`}</style>
      <span>{toast.message}</span>
      {toast.action && (
        <button
          onClick={() => { toast.action!.onClick(); onDismiss(toast.id); }}
          style={{
            background: 'none',
            border: `1px solid ${toast.type === 'info' ? 'var(--color-border-default)' : 'rgba(255,255,255,0.6)'}`,
            borderRadius: '4px',
            color,
            fontSize: '12px',
            fontWeight: 600,
            padding: '2px 8px',
            cursor: 'pointer',
            flexShrink: 0,
          }}
        >
          {toast.action.label}
        </button>
      )}
    </div>
  );
}
