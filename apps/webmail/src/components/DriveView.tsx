'use client';

import { useState, useEffect, useRef } from 'react';
import { useTranslations } from 'next-intl';
import {
  DriveNode,
  downloadDriveNode,
} from '@/lib/api';
import { DriveNodeIcon } from '@/lib/driveNodeIcon';
import { formatBytes, formatDate, BreadcrumbItem, DRIVE_NODE_DRAG_MIME, DRIVE_NODE_DRAG_TEXT } from '@/lib/drive/driveUtils';
import { DriveShareModal } from './DriveShareModal';
import { DriveNodeMenu } from './drive/DriveNodeMenu';
import {
  FolderIcon, ArrowUpTrayIcon, FolderPlusIcon,
  EllipsisVerticalIcon,
  XMarkIcon, ArrowPathIcon, ChevronRightIcon,
} from '@heroicons/react/24/outline';

import {
  getDriveNodeDragPayload, parseDriveNodeIds,
  isDriveNodeDrag, createDriveDragGhost,
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
                          if (!isInternalDrive) {
                            return;
                          }
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
                            if (files.length) {
                              await handleUploadEntries(files, item.id || undefined, 'drop');
                            }
                            return;
                          }
                          e.preventDefault();
                          e.stopPropagation();
                          setDropTargetFolderId(null);
                          const payload = getDriveNodeDragPayload(e.dataTransfer);
                          const payloadNodeIds = parseDriveNodeIds(payload);
                          if (payloadNodeIds && payloadNodeIds.length > 0) {
                            await handleMoveNodes(payloadNodeIds.filter((id) => id !== item.id), item.id || '');
                            return;
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

            {/* Actions */}
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
            <input ref={fileInputRef} type="file" multiple style={{ display: 'none' }} onChange={(e) => { if (e.target.files) { handleUploadFromList(e.target.files, currentParentId || undefined, 'picker'); e.target.value = ''; } }} />
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
                {nodes.map((node) => {
                  const isRenaming = renameNodeId === node.id;
                  const isDropTarget = dropTargetFolderId === node.id;
                  const isDraggingSelf = draggingNodeIds.includes(node.id);
                  const isSelected = selectedNodeIds.includes(node.id);
                  return (
                    <div
                      key={node.id}
                      draggable
                      onClick={(e) => {
                        applySelection(node.id, e.ctrlKey || e.metaKey);
                        e.stopPropagation();
                      }}
                      onDragStart={(e) => {
                        const idsToDrag = (selectedNodeIds.includes(node.id) ? selectedNodeIds : [node.id]);
                        const dragNodeNames = idsToDrag
                          .map((dragId) => nodes.find((n) => n.id === dragId)?.name)
                          .filter(Boolean) as string[];
                        const payload = JSON.stringify({ nodeIds: [...new Set(idsToDrag)] });
                        e.dataTransfer.setData(DRIVE_NODE_DRAG_MIME, payload);
                        e.dataTransfer.setData(DRIVE_NODE_DRAG_TEXT, `nodes:${idsToDrag.join(',')}`);
                        e.dataTransfer.setData('text/plain', `${DRIVE_NODE_DRAG_TEXT}:${idsToDrag.join(',')}\n${t('moveNodesText', { count: idsToDrag.length })}`);
                        e.dataTransfer.effectAllowed = 'move';
                        if (idsToDrag.length > 1) {
                          const ghost = createDriveDragGhost(idsToDrag.length, dragNodeNames, t);
                          document.body.appendChild(ghost);
                          e.dataTransfer.setDragImage(ghost, 18, 18);
                          requestAnimationFrame(() => {
                            if (ghost.isConnected) document.body.removeChild(ghost);
                          });
                        }
                        setDraggingNodeIds(idsToDrag);
                      }}
                      onDragEnd={() => {
                        setDraggingNodeIds([]);
                        setDropTargetFolderId(null);
                      }}
                      onDragOver={(e) => {
                        if (node.node_type !== 'folder') return;
                        if (draggingNodeIds.includes(node.id)) return;
                        e.preventDefault();
                        e.stopPropagation();
                        setDropTargetFolderId(node.id);
                      }}
                      onDragLeave={(e) => {
                        if (e.currentTarget.contains(e.relatedTarget as Node)) return;
                        if (isDropTarget) setDropTargetFolderId(null);
                      }}
                      onDrop={async (e) => {
                        e.preventDefault();
                        e.stopPropagation();
                        if (node.node_type !== 'folder') return;
                        const payload = getDriveNodeDragPayload(e.dataTransfer);
                        const payloadNodeIds = parseDriveNodeIds(payload);
                        if (payloadNodeIds && payloadNodeIds.length > 0) {
                          await handleMoveNodes(payloadNodeIds.filter((id) => id !== node.id), node.id);
                          return;
                        }
                        const files = await collectDroppedFiles(e.dataTransfer);
                        if (files.length) await handleUploadEntries(files, node.id, 'drop');
                      }}
                      onDoubleClick={() => openFolder(node)}
                      style={{
                        position: 'relative',
                        borderRadius: '8px',
                        border: `1px solid ${isDropTarget ? 'var(--color-accent)' : 'var(--color-border-default)'}`,
                        background: isDraggingSelf || isSelected ? 'var(--color-bg-secondary)' : isDropTarget ? 'var(--color-accent-subtle)' : 'var(--color-bg-primary)',
                        padding: '14px 12px 10px',
                        cursor: node.node_type === 'folder' ? 'pointer' : 'default',
                        transition: 'background 140ms ease, border-color 140ms ease, transform 140ms ease',
                        animation: isDropTarget ? 'driveDropPulse 1.1s ease-in-out infinite' : 'none',
                      }}
                      onMouseEnter={(e) => {
                        (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)';
                        (e.currentTarget as HTMLDivElement).style.borderColor = 'var(--color-border-default)';
                      }}
                      onMouseLeave={(e) => {
                        const target = e.currentTarget as HTMLDivElement;
                        const selectedOrDragging = isDraggingSelf || isSelected;
                        target.style.background = selectedOrDragging
                          ? 'var(--color-bg-secondary)'
                          : isDropTarget
                            ? 'var(--color-accent-subtle)'
                            : 'var(--color-bg-primary)';
                        target.style.borderColor = isDropTarget ? 'var(--color-accent)' : 'var(--color-border-default)';
                      }}
                    >
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '10px' }}>
                        <DriveNodeIcon node={node} />
                        <div style={{ position: 'relative' }}>
                          <button
                            onClick={(e) => { e.stopPropagation(); setMenuNodeId(menuNodeId === node.id ? null : node.id); }}
                            style={{ background: 'none', border: 'none', cursor: 'pointer', padding: '2px', color: 'var(--color-text-tertiary)', display: 'flex', borderRadius: '4px' }}
                            onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                            onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
                          ><EllipsisVerticalIcon style={{ width: '16px', height: '16px' }} /></button>
                          {menuNodeId === node.id && (
                            <DriveNodeMenu
                              node={node}
                              onDownload={() => downloadDriveNode(node.id, node.name).catch(() => {})}
                              onRename={() => { setRenameNodeId(node.id); setRenameName(node.name); }}
                              onShare={() => setShareNode(node)}
                              onTrash={() => handleTrash(node.id)}
                              onClose={() => setMenuNodeId(null)}
                            />
                          )}
                        </div>
                      </div>
                      {isRenaming ? (
                        <input
                          ref={renameRef}
                          value={renameName}
                          onChange={(e) => setRenameName(e.target.value)}
                          onBlur={handleRename}
                          onKeyDown={(e) => { if (e.key === 'Enter') handleRename(); if (e.key === 'Escape') setRenameNodeId(null); }}
                          onClick={(e) => e.stopPropagation()}
                          style={{ width: '100%', border: '1px solid var(--color-accent)', borderRadius: '4px', padding: '2px 6px', fontSize: '12px', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', outline: 'none', boxSizing: 'border-box' }}
                        />
                      ) : (
                        <div style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', marginBottom: '4px' }}>{node.name}</div>
                      )}
                      <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
                        {node.node_type === 'file' ? formatBytes(node.size) : t('folderLabel')} · {formatDate(node.updated_at)}
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        </div>
      )}

      {shareNode && <DriveShareModal node={shareNode} onClose={() => setShareNode(null)} />}
    </div>
  );
}
