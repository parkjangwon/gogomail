'use client';

import { useTranslations } from 'next-intl';
import type { DriveNode } from '@/lib/api';
import { DriveNodeIcon } from '@/lib/driveNodeIcon';
import { ChevronRightIcon } from '@heroicons/react/24/outline';

interface ComposeDrivePickerPanelProps {
  open: boolean;
  drivePickerNodes: DriveNode[];
  drivePickerLoading: boolean;
  drivePickerCrumbs: Array<{ id: string | undefined; name: string }>;
  attachingDriveId: string | null;
  openDrivePicker: (parentId?: string, crumbs?: Array<{ id: string | undefined; name: string }>) => void | Promise<void>;
  handleAttachFromDrive: (node: DriveNode) => void | Promise<void>;
}

export function ComposeDrivePickerPanel({
  open,
  drivePickerNodes,
  drivePickerLoading,
  drivePickerCrumbs,
  attachingDriveId,
  openDrivePicker,
  handleAttachFromDrive,
}: ComposeDrivePickerPanelProps) {
  const t = useTranslations('composeFull');

  if (!open) return null;

  return (
    <div style={{ position: 'absolute', bottom: '100%', left: 0, marginBottom: '4px', background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: '8px', boxShadow: '0 8px 24px rgba(0,0,0,0.16)', zIndex: 400, width: '280px', overflow: 'hidden' }}>
      <div style={{ display: 'flex', alignItems: 'center', gap: '2px', padding: '8px 10px', borderBottom: '1px solid var(--color-border-subtle)', flexWrap: 'wrap' }}>
        {drivePickerCrumbs.map((crumb, i) => (
          <span key={i} style={{ display: 'inline-flex', alignItems: 'center', gap: '2px' }}>
            {i > 0 && <ChevronRightIcon style={{ width: '10px', height: '10px', color: 'var(--color-text-tertiary)', flexShrink: 0 }} />}
            <button
              type="button"
              onClick={() => { const newCrumbs = drivePickerCrumbs.slice(0, i + 1); openDrivePicker(crumb.id, newCrumbs); }}
              style={{ background: 'none', border: 'none', cursor: 'pointer', fontSize: '12px', color: i === drivePickerCrumbs.length - 1 ? 'var(--color-text-primary)' : 'var(--color-accent)', padding: '1px 3px', borderRadius: '3px', fontWeight: i === drivePickerCrumbs.length - 1 ? 600 : 400 }}
            >
              {crumb.name}
            </button>
          </span>
        ))}
      </div>
      <div style={{ maxHeight: '240px', overflowY: 'auto' }}>
        {drivePickerLoading ? (
          <div style={{ padding: '20px', textAlign: 'center', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('fileLoading')}</div>
        ) : drivePickerNodes.length === 0 ? (
          <div style={{ padding: '20px', textAlign: 'center', fontSize: '12px', color: 'var(--color-text-tertiary)' }}>{t('noFiles')}</div>
        ) : drivePickerNodes.map((node) => (
          <button
            key={node.id}
            type="button"
            onClick={() => handleAttachFromDrive(node)}
            disabled={attachingDriveId === node.id}
            style={{ width: '100%', display: 'flex', alignItems: 'center', gap: '8px', padding: '8px 12px', border: 'none', background: 'transparent', cursor: attachingDriveId === node.id ? 'wait' : 'pointer', textAlign: 'left' }}
            onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-secondary)'; }}
            onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
          >
            <DriveNodeIcon node={node} size={14} />
            <span style={{ fontSize: '13px', color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1 }}>{node.name}</span>
            {node.node_type === 'folder' && <ChevronRightIcon style={{ width: '12px', height: '12px', color: 'var(--color-text-tertiary)', flexShrink: 0 }} />}
            {attachingDriveId === node.id && <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>{t('attaching')}</span>}
          </button>
        ))}
      </div>
    </div>
  );
}
