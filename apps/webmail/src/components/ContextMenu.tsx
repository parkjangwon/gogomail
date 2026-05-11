'use client';

import { useEffect, useRef } from 'react';

interface ContextMenuItem {
  label: string;
  onClick: () => void;
  danger?: boolean;
}

interface ContextMenuProps {
  x: number;
  y: number;
  items: ContextMenuItem[];
  onClose: () => void;
}

export function ContextMenu({ x, y, items, onClose }: ContextMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        onClose();
      }
    }
    function handleKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    document.addEventListener('mousedown', handleClick);
    document.addEventListener('keydown', handleKey);
    return () => {
      document.removeEventListener('mousedown', handleClick);
      document.removeEventListener('keydown', handleKey);
    };
  }, [onClose]);

  // Adjust position to stay within viewport
  const adjustedX = Math.min(x, window.innerWidth - 180);
  const adjustedY = Math.min(y, window.innerHeight - items.length * 36 - 8);

  return (
    <div
      ref={menuRef}
      role="menu"
      aria-label="메시지 작업"
      style={{
        position: 'fixed',
        top: adjustedY,
        left: adjustedX,
        zIndex: 400,
        background: 'var(--color-bg-primary)',
        border: '1px solid var(--color-border-default)',
        borderRadius: '6px',
        boxShadow: '0 4px 16px rgba(0,0,0,0.15)',
        minWidth: '160px',
        overflow: 'hidden',
        padding: '4px 0',
      }}
    >
      {items.map((item) => (
        <button
          key={item.label}
          role="menuitem"
          onClick={() => { item.onClick(); onClose(); }}
          style={{
            display: 'block',
            width: '100%',
            textAlign: 'left',
            padding: '8px 14px',
            border: 'none',
            background: 'transparent',
            color: item.danger ? 'var(--color-destructive)' : 'var(--color-text-primary)',
            fontSize: '13px',
            cursor: 'pointer',
          }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}
