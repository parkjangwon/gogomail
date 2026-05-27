'use client';

import { useState, useEffect, useRef } from 'react';
import { useTranslations } from 'next-intl';
import {
  DriveNode,
} from '@/lib/api';
import { BreadcrumbItem } from '@/lib/drive/driveUtils';
import { DriveShareModal } from './DriveShareModal';
import {
  FolderIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';
import { DriveToolbar } from './drive/DriveToolbar';
import { DriveNodeCard } from './drive/DriveNodeCard';

import {
  getDriveNodeDragPayload,
  isDriveNodeDrag,
  collectDroppedFiles,
} from './drive/driveViewHelpers';
import { useDriveUpload } from './drive/useDriveUpload';
import { useDriveSidebar } from './drive/useDriveSidebar';
import { useDriveNodes } from './drive/useDriveNodes';
import { useDriveInteractions } from './drive/useDriveInteractions';
import { useDriveFileOps } from './drive/useDriveFileOps';
import { DriveUploadModal } from './drive/DriveUploadModal';
import { DriveTrashView } from './drive/DriveTrashView';
import { DriveSidebar } from './drive/DriveSidebar';

export function DriveView() {
  const t = useTranslations('drive');
  const [activeSection, setActiveSection] = useState<'drive' | 'trash'>('drive');
  const [breadcrumb, setBreadcrumb] = useState<BreadcrumbItem[]>([{ id: '', name: t('myDrive') }]);

  const fileInputRef = useRef<HTMLInputElement>(null);
  const folderInputRef = useRef<HTMLInputElement>(null);
  const newFolderRef = useRef<HTMLInputElement>(null);
  const renameRef = useRef<HTMLInputElement>(null);

  const currentParentId = breadcrumb[breadcrumb.length - 1]?.id ?? '';

  const {
    nodes,
    setNodes,
    trashNodes,
    setTrashNodes,
    usage,
    setUsage,
    loading,
    trashLoading,
    refreshDriveNodes,
    loadTrashNodes,
  } = useDriveNodes({ breadcrumb, activeSection, t });

  const upload = useDriveUpload({ onUploadComplete: refreshDriveNodes, t });
  const {
    driveUploadBatch,
    driveUploads,
    driveUploadModalOpen,
    setDriveUploadModalOpen,
    driveUploadResumable,
    driveUploadModalDismissedRef,
    enqueueDriveUploads,
    pauseDriveUpload,
    resumeDriveUpload,
    cancelDriveUpload,
    DRIVE_UPLOAD_STATUS_LABELS,
  } = upload;

  const sidebar = useDriveSidebar({ breadcrumb });
  const {
    sidebarFolderChildren,
    setSidebarFolderChildren,
    sidebarExpandedFolders,
    sidebarLoadedFolders,
    setSidebarLoadedFolders,
    sidebarLoadingFolders,
    sidebarLoadKey,
    loadSidebarFolders,
    reloadSidebarCurrentPath,
    toggleSidebarFolder,
  } = sidebar;

  const {
    menuNodeId,
    setMenuNodeId,
    renameNodeId,
    setRenameNodeId,
    renameName,
    setRenameName,
    newFolderMode,
    setNewFolderMode,
    newFolderName,
    setNewFolderName,
    dragOver,
    setDragOver,
    draggingNodeIds,
    setDraggingNodeIds,
    selectedNodeIds,
    dropTargetFolderId,
    setDropTargetFolderId,
    shareNode,
    setShareNode,
    handleCreateFolder,
    handleRename,
    handleMoveNodes,
    applySelection,
  } = useDriveInteractions({
    breadcrumb,
    setBreadcrumb,
    nodes,
    setNodes,
    refreshDriveNodes,
    setUsage,
    sidebarLoadKey,
    setSidebarFolderChildren,
    setSidebarLoadedFolders,
    reloadSidebarCurrentPath,
    t,
  });

  const {
    handleTrash,
    handleRestore,
    handlePermanentDelete,
    handleEmptyTrash,
    handleUploadEntries,
    handleUploadFromList,
  } = useDriveFileOps({
    nodes,
    setNodes,
    setTrashNodes,
    trashNodes,
    setUsage,
    breadcrumb,
    driveUploadResumable,
    enqueueDriveUploads,
    t,
  });

  useEffect(() => {
    const folderInput = folderInputRef.current;
    if (!folderInput) return;
    folderInput.setAttribute('webkitdirectory', '');
    folderInput.setAttribute('directory', '');
  }, []);

  useEffect(() => {
    if (activeSection === 'drive') loadSidebarFolders('');
  }, [activeSection, loadSidebarFolders]);

  useEffect(() => { if (newFolderMode) setTimeout(() => newFolderRef.current?.focus(), 50); }, [newFolderMode]);
  useEffect(() => { if (renameNodeId) setTimeout(() => renameRef.current?.select(), 50); }, [renameNodeId]);

  function navigateTo(item: BreadcrumbItem) {
    const idx = breadcrumb.findIndex((b) => b.id === item.id);
    if (idx !== -1) setBreadcrumb(breadcrumb.slice(0, idx + 1));
  }

  function openFolder(node: DriveNode) {
    if (node.node_type !== 'folder') return;
    setBreadcrumb((prev) => [...prev, { id: node.id, name: node.name }]);
  }

  const uploadPanelOpen = driveUploadModalOpen && driveUploads.length > 0;
  const draggingNodeNames = draggingNodeIds
    .map((id) => nodes.find((node) => node.id === id)?.name)
    .filter(Boolean) as string[];


  return (
    <div style={{ flex: 1, minWidth: 0, height: '100%', display: 'flex', background: 'var(--color-bg-primary)', position: 'relative' }}>
      <style jsx>{`
        @keyframes driveMultiDragPulse {
          0%, 100% { transform: translate3d(0, 0, 0) scale(1); opacity: 0.96; }
          50% { transform: translate3d(0, -2px, 0) scale(1.01); opacity: 1; }
        }
        @keyframes driveDropPulse {
          0%, 100% { box-shadow: inset 0 0 0 0 rgba(59, 130, 246, 0.28); }
          50% { box-shadow: inset 0 0 0 1px rgba(59, 130, 246, 0.28); }
        }
      `}</style>

      {/* ── Sidebar ── */}
      <DriveSidebar
        activeSection={activeSection}
        setActiveSection={setActiveSection}
        breadcrumb={breadcrumb}
        setBreadcrumb={setBreadcrumb}
        trashNodeCount={trashNodes.length}
        usage={usage}
        sidebarFolderChildren={sidebarFolderChildren}
        sidebarExpandedFolders={sidebarExpandedFolders}
        sidebarLoadedFolders={sidebarLoadedFolders}
        sidebarLoadingFolders={sidebarLoadingFolders}
        sidebarLoadKey={sidebarLoadKey}
        toggleSidebarFolder={toggleSidebarFolder}
        dropTargetFolderId={dropTargetFolderId}
        setDropTargetFolderId={setDropTargetFolderId}
        handleMoveNodes={handleMoveNodes}
        handleUploadEntries={handleUploadEntries}
      />

      {/* ── Main content ── */}
      {activeSection === 'trash' ? (
        <DriveTrashView
          trashNodes={trashNodes}
          trashLoading={trashLoading}
          onRefresh={loadTrashNodes}
          onEmptyTrash={handleEmptyTrash}
          onRestore={handleRestore}
          onPermanentDelete={handlePermanentDelete}
        />
      ) : (
        /* Drive view */
        <div
          data-testid="drive-drop-surface"
          style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', position: 'relative' }}
          onDragOver={(e) => {
            const isInternalDrive = isDriveNodeDrag(e.dataTransfer);
            if (!isInternalDrive) {
              e.preventDefault();
              setDragOver(true);
            }
          }}
          onDragLeave={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node)) setDragOver(false); }}
          onDrop={async (e) => {
            e.preventDefault();
            setDragOver(false);
            const payloadNodeId = getDriveNodeDragPayload(e.dataTransfer);
            if (payloadNodeId) return;
            const files = await collectDroppedFiles(e.dataTransfer);
            if (files.length) await handleUploadEntries(files, currentParentId || undefined, 'drop');
          }}
        >
          {dragOver && (
            <div aria-hidden="true" style={{ position: 'absolute', inset: 0, background: 'var(--color-accent-subtle)', border: '2px dashed var(--color-accent)', borderRadius: '4px', zIndex: 100, display: 'flex', alignItems: 'center', justifyContent: 'center', fontSize: '16px', fontWeight: 600, color: 'var(--color-accent)', pointerEvents: 'none' }}>
              {t('dropOverlay')}
            </div>
          )}

          {/* Toolbar */}
          <DriveToolbar
            breadcrumb={breadcrumb}
            dropTargetFolderId={dropTargetFolderId}
            setDropTargetFolderId={setDropTargetFolderId}
            navigateTo={navigateTo}
            handleMoveNodes={handleMoveNodes}
            handleUploadEntries={handleUploadEntries}
            handleUploadFromList={handleUploadFromList}
            refreshDriveNodes={refreshDriveNodes}
            setNewFolderMode={setNewFolderMode}
            driveUploads={driveUploads}
            driveUploadModalDismissedRef={driveUploadModalDismissedRef}
            setDriveUploadModalOpen={setDriveUploadModalOpen}
            fileInputRef={fileInputRef}
            folderInputRef={folderInputRef}
            draggingNodeIds={draggingNodeIds}
            draggingNodeNames={draggingNodeNames}
          />

          {uploadPanelOpen && (
            <DriveUploadModal
              driveUploadBatch={driveUploadBatch}
              driveUploads={driveUploads}
              driveUploadResumable={driveUploadResumable}
              driveUploadModalDismissedRef={driveUploadModalDismissedRef}
              DRIVE_UPLOAD_STATUS_LABELS={DRIVE_UPLOAD_STATUS_LABELS}
              currentParentId={currentParentId}
              fileInputRef={fileInputRef}
              folderInputRef={folderInputRef}
              onClose={() => setDriveUploadModalOpen(false)}
              onPause={pauseDriveUpload}
              onResume={resumeDriveUpload}
              onCancel={cancelDriveUpload}
              onDropFiles={(files) => handleUploadEntries(files, currentParentId || undefined, 'drop')}
            />
          )}


          {/* File grid */}
          <div style={{ flex: 1, overflowY: 'auto', padding: '16px 20px' }}>
            {/* New folder input */}
            {newFolderMode && (
              <div style={{ display: 'flex', alignItems: 'center', gap: '8px', marginBottom: '12px', padding: '8px 12px', borderRadius: '8px', border: '1px solid var(--color-accent)', background: 'var(--color-accent-subtle)' }}>
                <FolderIcon style={{ width: '20px', height: '20px', color: '#f59e0b', flexShrink: 0 }} />
                <input
                  ref={newFolderRef}
                  value={newFolderName}
                  onChange={(e) => setNewFolderName(e.target.value)}
                  onKeyDown={(e) => { if (e.key === 'Enter') handleCreateFolder(); if (e.key === 'Escape') { setNewFolderMode(false); setNewFolderName(''); } }}
                  placeholder={t('newFolderPlaceholder')}
                  style={{ flex: 1, border: 'none', background: 'transparent', outline: 'none', fontSize: '13px', color: 'var(--color-text-primary)' }}
                />
                <button onClick={handleCreateFolder} style={{ padding: '3px 10px', borderRadius: '5px', border: 'none', background: 'var(--color-accent)', color: '#fff', fontSize: '12px', cursor: 'pointer' }}>{t('createFolder')}</button>
                <button onClick={() => { setNewFolderMode(false); setNewFolderName(''); }} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'flex' }}><XMarkIcon style={{ width: '16px', height: '16px' }} /></button>
              </div>
            )}

            {loading ? (
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))', gap: '12px' }}>
                {Array.from({ length: 8 }).map((_, i) => (
                  <div key={i} style={{ height: '120px', borderRadius: '8px', background: 'var(--color-bg-secondary)', animation: 'pulse 1.5s ease-in-out infinite' }} />
                ))}
              </div>
            ) : nodes.length === 0 && !newFolderMode ? (
              <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', height: '300px', gap: '12px', color: 'var(--color-text-tertiary)' }}>
                <FolderIcon style={{ width: '48px', height: '48px', opacity: 0.4 }} />
                <div style={{ fontSize: '14px' }}>{t('emptyFolderTitle')}</div>
                <div style={{ fontSize: '12px', opacity: 0.8 }}>{t('emptyFolderHint')}</div>
              </div>
            ) : (
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(160px, 1fr))', gap: '12px' }}>
                {nodes.map((node) => (
                  <DriveNodeCard
                    key={node.id}
                    node={node}
                    nodes={nodes}
                    menuNodeId={menuNodeId}
                    setMenuNodeId={setMenuNodeId}
                    renameNodeId={renameNodeId}
                    setRenameNodeId={setRenameNodeId}
                    renameName={renameName}
                    setRenameName={setRenameName}
                    renameRef={renameRef}
                    dropTargetFolderId={dropTargetFolderId}
                    setDropTargetFolderId={setDropTargetFolderId}
                    draggingNodeIds={draggingNodeIds}
                    setDraggingNodeIds={setDraggingNodeIds}
                    selectedNodeIds={selectedNodeIds}
                    applySelection={applySelection}
                    onDoubleClick={() => openFolder(node)}
                    handleRename={handleRename}
                    handleTrash={handleTrash}
                    handleMoveNodes={handleMoveNodes}
                    handleUploadEntries={handleUploadEntries}
                    setShareNode={setShareNode}
                  />
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {shareNode && <DriveShareModal node={shareNode} onClose={() => setShareNode(null)} />}
    </div>
  );
}
