'use client';

import { type Dispatch, type SetStateAction } from 'react';
import { useTranslations } from 'next-intl';
import {
  BreadcrumbItem,
  SidebarFolderItem,
  DroppedFileEntry,
  formatBytes,
} from '@/lib/drive/driveUtils';
import {
  getDriveNodeDragPayload,
  parseDriveNodeIds,
  collectDroppedFiles,
  type DriveUploadSource,
} from './driveViewHelpers';
import { FolderIcon, ChevronRightIcon } from '@heroicons/react/24/outline';
import { FolderIcon as FolderSolid, TrashIcon as TrashSolid } from '@heroicons/react/24/solid';

interface DriveSidebarProps {
  activeSection: 'drive' | 'trash';
  setActiveSection: (section: 'drive' | 'trash') => void;
  breadcrumb: BreadcrumbItem[];
  setBreadcrumb: (bc: BreadcrumbItem[]) => void;
  trashNodeCount: number;
  usage: { quota_used: number; quota_limit: number } | null;
  sidebarFolderChildren: Record<string, SidebarFolderItem[]>;
  sidebarExpandedFolders: Set<string>;
  sidebarLoadedFolders: Set<string>;
  sidebarLoadingFolders: Record<string, boolean>;
  sidebarLoadKey: (parentId: string) => string;
  toggleSidebarFolder: (id: string) => void;
  dropTargetFolderId: string | null;
  setDropTargetFolderId: Dispatch<SetStateAction<string | null>>;
  handleMoveNodes: (nodeIds: string[], targetFolderId: string) => Promise<void>;
  handleUploadEntries: (files: DroppedFileEntry[], folderId?: string, source?: DriveUploadSource) => Promise<void>;
}

export function DriveSidebar({
  activeSection,
  setActiveSection,
  breadcrumb,
  setBreadcrumb,
  trashNodeCount,
  usage,
  sidebarFolderChildren,
  sidebarExpandedFolders,
  sidebarLoadedFolders,
  sidebarLoadingFolders,
  sidebarLoadKey,
  toggleSidebarFolder,
  dropTargetFolderId,
  setDropTargetFolderId,
  handleMoveNodes,
  handleUploadEntries,
}: DriveSidebarProps) {
  const t = useTranslations('drive');

  const usedPct = usage && usage.quota_limit > 0
    ? Math.min(100, (usage.quota_used / usage.quota_limit) * 100)
    : 0;
  const barColor = usedPct >= 90 ? '#ef4444' : usedPct >= 70 ? '#f59e0b' : '#22c55e';
  const sidebarDropTargetActive = dropTargetFolderId === '';

  const renderSidebarFolders = (parentId: string, depth: number, path: BreadcrumbItem[]): React.ReactNode => {
    const key = sidebarLoadKey(parentId);
    const children = sidebarFolderChildren[key] ?? [];
    const isLoading = sidebarLoadingFolders[key];

    if (!children.length && !isLoading && sidebarLoadedFolders.has(key) && parentId === '') {
      return null;
    }

    return (
      <div>
        {isLoading && children.length === 0 && (
          <div style={{ marginLeft: `${depth * 12}px`, padding: '6px 8px', fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
            {t('loadingFolder')}
          </div>
        )}
        {children.map((folder) => {
          const isExpanded = sidebarExpandedFolders.has(folder.id);
          const childKey = sidebarLoadKey(folder.id);
          const childLoading = sidebarLoadingFolders[childKey];
          const hasKnownChildren = (sidebarFolderChildren[childKey] ?? []).length > 0;
          const isDropTarget = dropTargetFolderId === folder.id;
          const isCurrentPath = breadcrumb.some((item) => item.id === folder.id);
          const folderPath = [...path, { id: folder.id, name: folder.name }];

          return (
            <div key={folder.id}>
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                  margin: '2px 0',
                  padding: '6px 8px 6px 0',
                  borderRadius: '6px',
                  marginLeft: `${depth * 8}px`,
                  background: isDropTarget ? 'var(--color-accent-subtle)' : isCurrentPath ? 'var(--color-bg-secondary)' : 'transparent',
                  border: isDropTarget ? '1px solid var(--color-accent)' : '1px solid transparent',
                  color: isCurrentPath ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                  transition: 'background 140ms ease, border-color 140ms ease',
                  cursor: 'pointer',
                  fontSize: '12px',
                }}
                onClick={() => setBreadcrumb(folderPath)}
                onDragOver={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  setDropTargetFolderId(folder.id);
                }}
                onDragLeave={(e) => {
                  if (!e.currentTarget.contains(e.relatedTarget as Node)) {
                    setDropTargetFolderId((prev) => (prev === folder.id ? null : prev));
                  }
                }}
                onDrop={async (e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  const payload = getDriveNodeDragPayload(e.dataTransfer);
                  const payloadNodeIds = parseDriveNodeIds(payload);
                  if (payloadNodeIds && payloadNodeIds.length > 0) {
                    await handleMoveNodes(payloadNodeIds.filter((id) => id !== folder.id), folder.id);
                    return;
                  }
                  const files = await collectDroppedFiles(e.dataTransfer);
                  if (files.length) await handleUploadEntries(files, folder.id, 'drop');
                  setDropTargetFolderId(null);
                }}
              >
                <button
                  type="button"
                  onClick={(e) => {
                    e.stopPropagation();
                    toggleSidebarFolder(folder.id);
                  }}
                  style={{
                    width: '16px',
                    height: '16px',
                    border: 'none',
                    background: 'transparent',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                    cursor: 'pointer',
                    color: 'var(--color-text-tertiary)',
                  }}
                >
                  <ChevronRightIcon style={{
                    width: '12px',
                    height: '12px',
                    transform: `rotate(${isExpanded ? 90 : 0}deg)`,
                    transition: 'transform 140ms ease',
                  }} />
                </button>
                <FolderIcon style={{ width: '14px', height: '14px', flexShrink: 0 }} />
                <span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {folder.name}
                  {childLoading && !hasKnownChildren ? t('loadingSuffix') : ''}
                </span>
              </div>
              {isExpanded && (
                <div>
                  {renderSidebarFolders(folder.id, depth + 1, folderPath)}
                </div>
              )}
            </div>
          );
        })}
      </div>
    );
  };

  return (
    <div style={{ width: '200px', flexShrink: 0, borderRight: '1px solid var(--color-border-subtle)', display: 'flex', flexDirection: 'column', padding: '12px 0', overflowY: 'auto' }}>
      {/* Nav items */}
      <div style={{ padding: '0 8px', marginBottom: '4px' }}>
        <button
          onClick={() => {
            setActiveSection('drive');
            setBreadcrumb([{ id: '', name: t('myDrive') }]);
          }}
          onDragOver={(e) => {
            e.preventDefault();
            setDropTargetFolderId('');
          }}
          onDragLeave={(e) => {
            if (!e.currentTarget.contains(e.relatedTarget as Node)) {
              setDropTargetFolderId((prev) => (prev === '' ? null : prev));
            }
          }}
          onDrop={async (e) => {
            e.preventDefault();
            e.stopPropagation();
            const payload = getDriveNodeDragPayload(e.dataTransfer);
            const payloadNodeIds = parseDriveNodeIds(payload);
            if (payloadNodeIds && payloadNodeIds.length > 0) {
              await handleMoveNodes(payloadNodeIds, '');
              return;
            }
            const files = await collectDroppedFiles(e.dataTransfer);
            if (files.length) await handleUploadEntries(files, undefined, 'drop');
            setDropTargetFolderId(null);
          }}
          style={{ display: 'flex', alignItems: 'center', gap: '8px', width: '100%', padding: '7px 10px', borderRadius: '6px', border: 'none', background: activeSection === 'drive' || sidebarDropTargetActive ? 'var(--color-accent-subtle)' : 'transparent', color: activeSection === 'drive' ? 'var(--color-accent)' : 'var(--color-text-secondary)', fontSize: '13px', fontWeight: activeSection === 'drive' ? 600 : 400, cursor: 'pointer', textAlign: 'left' }}
          onMouseEnter={(e) => { if (activeSection !== 'drive') (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
          onMouseLeave={(e) => { if (activeSection !== 'drive') (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          <FolderSolid style={{ width: '16px', height: '16px', flexShrink: 0 }} />
          {t('myDrive')}
        </button>
        <div style={{ marginTop: '6px' }}>
          {renderSidebarFolders('', 1, [{ id: '', name: t('myDrive') }])}
        </div>
      </div>
      <div style={{ padding: '0 8px', marginBottom: '16px' }}>
        <button
          onClick={() => setActiveSection('trash')}
          style={{ display: 'flex', alignItems: 'center', gap: '8px', width: '100%', padding: '7px 10px', borderRadius: '6px', border: 'none', background: activeSection === 'trash' ? 'var(--color-accent-subtle)' : 'transparent', color: activeSection === 'trash' ? 'var(--color-accent)' : 'var(--color-text-secondary)', fontSize: '13px', fontWeight: activeSection === 'trash' ? 600 : 400, cursor: 'pointer', textAlign: 'left' }}
          onMouseEnter={(e) => { if (activeSection !== 'trash') (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
          onMouseLeave={(e) => { if (activeSection !== 'trash') (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
        >
          <TrashSolid style={{ width: '16px', height: '16px', flexShrink: 0 }} />
          {t('trash')}
          {trashNodeCount > 0 && (
            <span style={{ marginLeft: 'auto', fontSize: '11px', background: 'var(--color-bg-tertiary)', color: 'var(--color-text-tertiary)', borderRadius: '10px', padding: '1px 6px' }}>{trashNodeCount}</span>
          )}
        </button>
      </div>

      {/* Spacer */}
      <div style={{ flex: 1 }} />

      {/* Storage bar */}
      {usage && (
        <div style={{ padding: '12px 14px', borderTop: '1px solid var(--color-border-subtle)' }}>
          <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginBottom: '6px', fontWeight: 500 }}>{t('storage')}</div>
          <div style={{ height: '6px', borderRadius: '3px', background: 'var(--color-bg-tertiary)', overflow: 'hidden', marginBottom: '6px' }}>
            <div style={{ height: '100%', borderRadius: '3px', width: `${usedPct}%`, background: barColor, transition: 'width 400ms ease' }} />
          </div>
          <div style={{ fontSize: '11px', color: 'var(--color-text-secondary)', lineHeight: 1.4 }}>
            <span style={{ fontWeight: 500, color: barColor }}>{formatBytes(usage.quota_used)}</span>
            <span style={{ color: 'var(--color-text-tertiary)' }}> / {formatBytes(usage.quota_limit)}{t('storageUsedSuffix')}</span>
          </div>
          {usedPct >= 70 && (
            <div style={{ marginTop: '4px', fontSize: '10px', color: barColor, fontWeight: 500 }}>
              {usedPct >= 90 ? t('storageNearFull') : t('storageHighUsage')}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
