'use client';

import { type Dispatch, type MutableRefObject, type RefObject, type SetStateAction } from 'react';
import { useTranslations } from 'next-intl';
import {
  ArrowUpTrayIcon, FolderPlusIcon, ArrowPathIcon, ChevronRightIcon,
} from '@heroicons/react/24/outline';
import {
  getDriveNodeDragPayload, parseDriveNodeIds,
  isDriveNodeDrag,
  collectDroppedFiles,
  type DriveUploadSource,
  type DriveUploadItem,
} from './driveViewHelpers';
import { type BreadcrumbItem, type DroppedFileEntry } from '@/lib/drive/driveUtils';

interface DriveToolbarProps {
  breadcrumb: BreadcrumbItem[];
  dropTargetFolderId: string | null;
  setDropTargetFolderId: Dispatch<SetStateAction<string | null>>;
  navigateTo: (item: BreadcrumbItem) => void;
  handleMoveNodes: (nodeIds: string[], targetFolderId: string) => Promise<void>;
  handleUploadEntries: (files: DroppedFileEntry[], folderId?: string, source?: DriveUploadSource) => Promise<void>;
  handleUploadFromList: (files: FileList, folderId?: string, source?: DriveUploadSource) => void;
  refreshDriveNodes: () => void;
  setNewFolderMode: (v: boolean) => void;
  driveUploads: DriveUploadItem[];
  driveUploadModalDismissedRef: MutableRefObject<boolean>;
  setDriveUploadModalOpen: Dispatch<SetStateAction<boolean>>;
  fileInputRef: RefObject<HTMLInputElement | null>;
  folderInputRef: RefObject<HTMLInputElement | null>;
  draggingNodeIds: string[];
  draggingNodeNames: string[];
}

export function DriveToolbar({
  breadcrumb,
  dropTargetFolderId,
  setDropTargetFolderId,
  navigateTo,
  handleMoveNodes,
  handleUploadEntries,
  handleUploadFromList,
  refreshDriveNodes,
  setNewFolderMode,
  driveUploads,
  driveUploadModalDismissedRef,
  setDriveUploadModalOpen,
  fileInputRef,
  folderInputRef,
  draggingNodeIds,
  draggingNodeNames,
}: DriveToolbarProps) {
  const t = useTranslations('drive');
  const currentParentId = breadcrumb[breadcrumb.length - 1]?.id ?? '';

  return (
    <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '12px 20px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
      {/* Breadcrumb */}
      <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flex: 1, minWidth: 0, overflow: 'hidden' }}>
        {breadcrumb.map((item, i) => (
          <span key={item.id} style={{ display: 'flex', alignItems: 'center', gap: '4px', minWidth: 0 }}>
            {i > 0 && <ChevronRightIcon style={{ width: '14px', height: '14px', color: 'var(--color-text-tertiary)', flexShrink: 0 }} />}
            {(() => {
              const isBreadcrumbDropTarget = dropTargetFolderId === item.id;
              const isCurrentFolder = item.id === currentParentId;
              return (
                <button
                  onClick={() => navigateTo(item)}
                  onDragOver={(e) => {
                    const isInternalDrive = isDriveNodeDrag(e.dataTransfer);
                    if (!isInternalDrive) return;
                    e.preventDefault();
                    e.stopPropagation();
                    if (!isCurrentFolder) setDropTargetFolderId(item.id);
                  }}
                  onDragLeave={(e) => {
                    if (isBreadcrumbDropTarget && !e.currentTarget.contains(e.relatedTarget as Node)) {
                      setDropTargetFolderId(null);
                    }
                  }}
                  onDrop={async (e) => {
                    const isInternalDrive = isDriveNodeDrag(e.dataTransfer);
                    if (!isInternalDrive) {
                      const files = await collectDroppedFiles(e.dataTransfer);
                      if (files.length) await handleUploadEntries(files, item.id || undefined, 'drop');
                      return;
                    }
                    e.preventDefault();
                    e.stopPropagation();
                    setDropTargetFolderId(null);
                    const payload = getDriveNodeDragPayload(e.dataTransfer);
                    const payloadNodeIds = parseDriveNodeIds(payload);
                    if (payloadNodeIds && payloadNodeIds.length > 0) {
                      await handleMoveNodes(payloadNodeIds.filter((id) => id !== item.id), item.id || '');
                    }
                  }}
                  style={{
                    background: isBreadcrumbDropTarget ? 'var(--color-accent-subtle)' : 'none',
                    border: 'none',
                    cursor: i === breadcrumb.length - 1 ? 'default' : 'pointer',
                    fontSize: '14px',
                    fontWeight: i === breadcrumb.length - 1 ? 600 : 400,
                    color: i === breadcrumb.length - 1 ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                    padding: '2px 4px',
                    borderRadius: '4px',
                    maxWidth: '180px',
                    overflow: 'hidden',
                    textOverflow: 'ellipsis',
                    whiteSpace: 'nowrap',
                  }}
                  onMouseEnter={(e) => {
                    if (i < breadcrumb.length - 1) {
                      (e.currentTarget as HTMLButtonElement).style.background = isBreadcrumbDropTarget
                        ? 'var(--color-accent-subtle)'
                        : 'var(--color-bg-secondary)';
                    }
                  }}
                  onMouseLeave={(e) => {
                    (e.currentTarget as HTMLButtonElement).style.background = isBreadcrumbDropTarget
                      ? 'var(--color-accent-subtle)'
                      : 'none';
                  }}
                >{item.name}</button>
              );
            })()}
          </span>
        ))}
      </div>

      {/* Action buttons */}
      <button onClick={() => refreshDriveNodes()} title={t('refresh')}
        style={{ padding: '5px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', color: 'var(--color-text-secondary)', display: 'flex', alignItems: 'center' }}>
        <ArrowPathIcon style={{ width: '15px', height: '15px' }} />
      </button>
      <button onClick={() => setNewFolderMode(true)} title={t('newFolder')}
        style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '13px', cursor: 'pointer' }}>
        <FolderPlusIcon style={{ width: '15px', height: '15px' }} /> {t('newFolder')}
      </button>
      <button
        onClick={(e) => {
          if (driveUploads.length > 0) {
            driveUploadModalDismissedRef.current = false;
            setDriveUploadModalOpen(true);
            return;
          }
          if (e.shiftKey) folderInputRef.current?.click();
          else fileInputRef.current?.click();
        }}
        title={driveUploads.length > 0 ? t('openUploadWindow') : t('uploadTooltip')}
        style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '5px 14px', borderRadius: '6px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '13px', fontWeight: 500, cursor: 'pointer' }}>
        <ArrowUpTrayIcon style={{ width: '15px', height: '15px' }} /> {driveUploads.length > 0 ? t('uploadWindow', { count: driveUploads.length }) : t('uploadButton')}
      </button>
      {draggingNodeIds.length > 1 && (
        <div
          style={{
            marginLeft: '8px',
            display: 'inline-flex',
            alignItems: 'center',
            gap: '8px',
            padding: '5px 10px',
            borderRadius: '999px',
            border: '1px solid var(--color-accent)',
            background: 'rgba(96, 165, 250, 0.12)',
            color: 'var(--color-accent)',
            fontSize: '11px',
            fontWeight: 600,
            whiteSpace: 'nowrap',
            maxWidth: '240px',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            animation: 'driveMultiDragPulse 1.1s ease-in-out infinite',
          }}
          title={draggingNodeNames.join(', ')}
        >
          {t('multiDragBadge', { count: draggingNodeIds.length })}
        </div>
      )}
      <input ref={fileInputRef} type="file" multiple style={{ display: 'none' }}
        onChange={(e) => { if (e.target.files) { handleUploadFromList(e.target.files, currentParentId || undefined, 'picker'); e.target.value = ''; } }}
      />
      <input
        ref={folderInputRef}
        type="file"
        multiple
        style={{ display: 'none' }}
        onChange={(e) => {
          if (e.target.files) {
            handleUploadFromList(e.target.files, currentParentId || undefined, 'folder');
            e.target.value = '';
          }
        }}
      />
    </div>
  );
}
