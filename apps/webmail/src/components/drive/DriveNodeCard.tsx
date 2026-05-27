'use client';

import { type RefObject } from 'react';
import { useTranslations } from 'next-intl';
import { type DriveNode, downloadDriveNode } from '@/lib/api';
import { DriveNodeIcon } from '@/lib/driveNodeIcon';
import { formatBytes, formatDate, DRIVE_NODE_DRAG_MIME, DRIVE_NODE_DRAG_TEXT, type DroppedFileEntry } from '@/lib/drive/driveUtils';
import { EllipsisVerticalIcon } from '@heroicons/react/24/outline';
import { DriveNodeMenu } from './DriveNodeMenu';
import {
  getDriveNodeDragPayload,
  parseDriveNodeIds,
  createDriveDragGhost,
  collectDroppedFiles,
  type DriveUploadSource,
} from './driveViewHelpers';

interface DriveNodeCardProps {
  node: DriveNode;
  nodes: DriveNode[];
  menuNodeId: string | null;
  setMenuNodeId: (id: string | null) => void;
  renameNodeId: string | null;
  setRenameNodeId: (id: string | null) => void;
  renameName: string;
  setRenameName: (name: string) => void;
  renameRef: RefObject<HTMLInputElement | null>;
  dropTargetFolderId: string | null;
  setDropTargetFolderId: (id: string | null) => void;
  draggingNodeIds: string[];
  setDraggingNodeIds: (ids: string[]) => void;
  selectedNodeIds: string[];
  applySelection: (id: string, multi: boolean) => void;
  onDoubleClick: () => void;
  handleRename: () => void;
  handleTrash: (id: string) => void;
  handleMoveNodes: (ids: string[], targetId: string) => Promise<void>;
  handleUploadEntries: (files: DroppedFileEntry[], folderId?: string, source?: DriveUploadSource) => Promise<void>;
  setShareNode: (node: DriveNode) => void;
}

export function DriveNodeCard({
  node,
  nodes,
  menuNodeId,
  setMenuNodeId,
  renameNodeId,
  setRenameNodeId,
  renameName,
  setRenameName,
  renameRef,
  dropTargetFolderId,
  setDropTargetFolderId,
  draggingNodeIds,
  setDraggingNodeIds,
  selectedNodeIds,
  applySelection,
  onDoubleClick,
  handleRename,
  handleTrash,
  handleMoveNodes,
  handleUploadEntries,
  setShareNode,
}: DriveNodeCardProps) {
  const t = useTranslations('drive');

  const isRenaming = renameNodeId === node.id;
  const isDropTarget = dropTargetFolderId === node.id;
  const isDraggingSelf = draggingNodeIds.includes(node.id);
  const isSelected = selectedNodeIds.includes(node.id);

  return (
    <div
      draggable
      onClick={(e) => {
        applySelection(node.id, e.ctrlKey || e.metaKey);
        e.stopPropagation();
      }}
      onDragStart={(e) => {
        const idsToDrag = selectedNodeIds.includes(node.id) ? selectedNodeIds : [node.id];
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
      onDoubleClick={onDoubleClick}
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
          >
            <EllipsisVerticalIcon style={{ width: '16px', height: '16px' }} />
          </button>
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
        <div style={{ fontSize: '12px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', marginBottom: '4px' }}>
          {node.name}
        </div>
      )}
      <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
        {node.node_type === 'file' ? formatBytes(node.size) : t('folderLabel')} · {formatDate(node.updated_at)}
      </div>
    </div>
  );
}
