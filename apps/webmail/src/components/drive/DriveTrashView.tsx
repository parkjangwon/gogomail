'use client';

import { useTranslations } from 'next-intl';
import type { DriveNode } from '@/lib/api';
import { DriveNodeIcon } from '@/lib/driveNodeIcon';
import { formatBytes, formatDate } from '@/lib/drive/driveUtils';
import { TrashIcon as TrashSolid } from '@heroicons/react/24/solid';
import {
  TrashIcon,
  ArrowPathIcon,
  ArrowUturnLeftIcon,
} from '@heroicons/react/24/outline';

interface DriveTrashViewProps {
  trashNodes: DriveNode[];
  trashLoading: boolean;
  onRefresh: () => void;
  onEmptyTrash: () => void;
  onRestore: (id: string) => void;
  onPermanentDelete: (id: string) => void;
}

export function DriveTrashView({
  trashNodes,
  trashLoading,
  onRefresh,
  onEmptyTrash,
  onRestore,
  onPermanentDelete,
}: DriveTrashViewProps) {
  const t = useTranslations('drive');

  return (
    <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
      {/* Toolbar */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '12px 20px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
        <TrashSolid style={{ width: '18px', height: '18px', color: 'var(--color-text-tertiary)' }} />
        <span style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)', flex: 1 }}>{t('trash')}</span>
        <button
          onClick={onRefresh}
          title={t('refresh')}
          style={{ padding: '5px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', color: 'var(--color-text-secondary)', display: 'flex', alignItems: 'center' }}
        >
          <ArrowPathIcon style={{ width: '15px', height: '15px' }} />
        </button>
        {trashNodes.length > 0 && (
          <button
            onClick={onEmptyTrash}
            style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-destructive)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', fontWeight: 500, cursor: 'pointer' }}
          >
            <TrashIcon style={{ width: '14px', height: '14px' }} />
            {t('emptyTrash')}
          </button>
        )}
      </div>

      {/* File list */}
      <div style={{ flex: 1, overflowY: 'auto', padding: '16px 20px' }}>
        {trashLoading ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} style={{ height: '56px', borderRadius: '8px', background: 'var(--color-bg-secondary)', animation: 'pulse 1.5s ease-in-out infinite' }} />
            ))}
          </div>
        ) : trashNodes.length === 0 ? (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '300px', gap: '12px', color: 'var(--color-text-tertiary)' }}>
            <TrashIcon style={{ width: '48px', height: '48px', opacity: 0.3 }} />
            <div style={{ fontSize: '14px' }}>{t('trashEmpty')}</div>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
            {trashNodes.map((node) => (
              <div
                key={node.id}
                style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '10px 14px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)' }}
                onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)'; }}
                onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-primary)'; }}
              >
                <div style={{ flexShrink: 0 }}><DriveNodeIcon node={node} /></div>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                    {node.name}
                  </div>
                  <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>
                    {node.node_type === 'file' ? formatBytes(node.size) : t('folderLabel')} · {formatDate(node.updated_at)}
                  </div>
                </div>
                <button
                  onClick={() => onRestore(node.id)}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer', flexShrink: 0 }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >
                  <ArrowUturnLeftIcon style={{ width: '13px', height: '13px' }} />
                  {t('restore')}
                </button>
                <button
                  onClick={() => onPermanentDelete(node.id)}
                  style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--color-destructive)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '12px', cursor: 'pointer', flexShrink: 0 }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = '#fef2f2'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                >
                  <TrashIcon style={{ width: '13px', height: '13px' }} />
                  {t('permanentDelete')}
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
