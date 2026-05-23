'use client';

import { ChevronRightIcon } from '@heroicons/react/24/outline';
import { useEffect, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';

interface ContextMenuItem {
  label: string;
  onClick?: () => void;
  danger?: boolean;
  children?: ContextMenuItem[];
  separator?: boolean;
}

interface ContextMenuProps {
  x: number;
  y: number;
  items: ContextMenuItem[];
  onClose: () => void;
}

function SubMenu({ items, parentRef, onClose }: { items: ContextMenuItem[]; parentRef: React.RefObject<HTMLButtonElement | null>; onClose: () => void }) {
  const menuRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState({ top: 0, left: 0 });

  useEffect(() => {
    if (!parentRef.current || !menuRef.current) return;
    const rect = parentRef.current.getBoundingClientRect();
    const mh = menuRef.current.offsetHeight || items.length * 36 + 8;
    const mw = menuRef.current.offsetWidth || 180;
    const top = Math.min(rect.top, window.innerHeight - mh - 8);
    const left = rect.right + mw > window.innerWidth ? rect.left - mw : rect.right;
    setPos({ top, left });
  }, [items.length, parentRef]);

  return (
    <div
      ref={menuRef}
      role="menu"
      style={{
        position: 'fixed',
        top: pos.top,
        left: pos.left,
        zIndex: 401,
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
          onClick={() => { item.onClick?.(); onClose(); }}
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

function MenuItem({ item, onClose }: { item: ContextMenuItem; onClose: () => void }) {
  const [showSub, setShowSub] = useState(false);
  const btnRef = useRef<HTMLButtonElement>(null);

  if (item.separator) {
    return <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '4px 0' }} />;
  }

  return (
    <div
      style={{ position: 'relative' }}
      onMouseEnter={() => item.children && setShowSub(true)}
      onMouseLeave={() => item.children && setShowSub(false)}
    >
      <button
        ref={btnRef}
        role="menuitem"
        onClick={() => { if (!item.children) { item.onClick?.(); onClose(); } }}
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          width: '100%',
          textAlign: 'left',
          padding: '8px 14px',
          border: 'none',
          background: showSub ? 'var(--color-bg-secondary)' : 'transparent',
          color: item.danger ? 'var(--color-destructive)' : 'var(--color-text-primary)',
          fontSize: '13px',
          cursor: 'pointer',
        }}
        onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
        onMouseLeave={(e) => { if (!showSub) (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
      >
        <span>{item.label}</span>
        {item.children && <ChevronRightIcon style={{ width: '14px', height: '14px', opacity: 0.5, flexShrink: 0 }} />}
      </button>
      {showSub && item.children && (
        <SubMenu items={item.children} parentRef={btnRef} onClose={onClose} />
      )}
    </div>
  );
}

export function ContextMenu({ x, y, items, onClose }: ContextMenuProps) {
  const t = useTranslations('contextMenu');
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

  const totalItems = items.filter((i) => !i.separator).length;
  const adjustedX = Math.min(x, window.innerWidth - 180);
  const adjustedY = Math.min(y, window.innerHeight - totalItems * 36 - 8);

  return (
    <div
      ref={menuRef}
      role="menu"
      aria-label={t('messageActionsAria')}
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
      {items.map((item, i) => (
        <MenuItem key={item.separator ? `sep-${i}` : item.label} item={item} onClose={onClose} />
      ))}
    </div>
  );
}
