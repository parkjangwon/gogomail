'use client';

import { useEffect, useRef } from 'react';
import { ArrowDownTrayIcon, LinkIcon, PencilIcon, TrashIcon } from '@heroicons/react/24/outline';
import type { DriveNode } from '@/lib/api';

interface DriveNodeMenuProps {
  node: DriveNode;
  onDownload: () => void;
  onRename: () => void;
  onShare: () => void;
  onTrash: () => void;
  onClose: () => void;
}

export function DriveNodeMenu({ node, onDownload, onRename, onShare, onTrash, onClose }: DriveNodeMenuProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    }
    document.addEventListener('mousedown', onDown);
    return () => document.removeEventListener('mousedown', onDown);
  }, [onClose]);

  const item = (label: string, icon: React.ReactNode, onClick: () => void, danger?: boolean): React.ReactNode => (
    <button
      onClick={() => {
        onClick();
        onClose();
      }}
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '8px',
        width: '100%',
        padding: '7px 14px',
        border: 'none',
        background: 'transparent',
        color: danger ? 'var(--color-destructive)' : 'var(--color-text-primary)',
        fontSize: '13px',
        cursor: 'pointer',
        textAlign: 'left',
      }}
      onMouseEnter={(e) => {
        (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)';
      }}
      onMouseLeave={(e) => {
        (e.currentTarget as HTMLButtonElement).style.background = 'transparent';
      }}
    >
      {icon}
      {label}
    </button>
  );

  return (
    <div
      ref={ref}
      style={{
        position: 'absolute',
        top: '100%',
        right: 0,
        marginTop: '2px',
        background: 'var(--color-bg-primary)',
        border: '1px solid var(--color-border-default)',
        borderRadius: '8px',
        boxShadow: '0 4px 20px rgba(0,0,0,0.14)',
        zIndex: 200,
        minWidth: '180px',
        overflow: 'hidden',
        padding: '4px 0',
      }}
    >
      {node.node_type === 'file' && item('다운로드', <ArrowDownTrayIcon style={{ width: '14px', height: '14px' }} />, onDownload)}
      {item('이름 변경', <PencilIcon style={{ width: '14px', height: '14px' }} />, onRename)}
      {item('공유 링크', <LinkIcon style={{ width: '14px', height: '14px' }} />, onShare)}
      <div style={{ height: '1px', background: 'var(--color-border-subtle)', margin: '4px 0' }} />
      {item('휴지통', <TrashIcon style={{ width: '14px', height: '14px' }} />, onTrash, true)}
    </div>
  );
}
