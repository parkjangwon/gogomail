import { useState, useCallback, type Dispatch, type SetStateAction } from 'react';
import { createDriveFolder, renameDriveNode, moveDriveNode } from '@/lib/api';
import type { DriveNode, DriveUsage } from '@/lib/api';
import type { BreadcrumbItem, SidebarFolderItem } from '@/lib/drive/driveUtils';
import { refreshDriveUsage } from './driveUsageRefresh';

export interface UseDriveInteractionsParams {
  breadcrumb: BreadcrumbItem[];
  nodes: DriveNode[];
  setNodes: Dispatch<SetStateAction<DriveNode[]>>;
  refreshDriveNodes: () => Promise<void>;
  setUsage: Dispatch<SetStateAction<DriveUsage | null>>;
  sidebarLoadKey: (folderId: string) => string;
  setSidebarFolderChildren: Dispatch<SetStateAction<Record<string, SidebarFolderItem[]>>>;
  setSidebarLoadedFolders: Dispatch<SetStateAction<Set<string>>>;
  reloadSidebarCurrentPath: () => void;
}

export interface UseDriveInteractionsReturn {
  menuNodeId: string | null;
  setMenuNodeId: Dispatch<SetStateAction<string | null>>;
  renameNodeId: string | null;
  setRenameNodeId: Dispatch<SetStateAction<string | null>>;
  renameName: string;
  setRenameName: Dispatch<SetStateAction<string>>;
  newFolderMode: boolean;
  setNewFolderMode: Dispatch<SetStateAction<boolean>>;
  newFolderName: string;
  setNewFolderName: Dispatch<SetStateAction<string>>;
  dragOver: boolean;
  setDragOver: Dispatch<SetStateAction<boolean>>;
  draggingNodeIds: string[];
  setDraggingNodeIds: Dispatch<SetStateAction<string[]>>;
  selectedNodeIds: string[];
  setSelectedNodeIds: Dispatch<SetStateAction<string[]>>;
  dropTargetFolderId: string | null;
  setDropTargetFolderId: Dispatch<SetStateAction<string | null>>;
  shareNode: DriveNode | null;
  setShareNode: Dispatch<SetStateAction<DriveNode | null>>;
  handleCreateFolder: () => Promise<void>;
  handleRename: () => Promise<void>;
  handleMoveNodes: (nodeIds: string[], targetParentId: string) => Promise<void>;
  applySelection: (nodeId: string, multi: boolean) => void;
}

export function useDriveInteractions({
  breadcrumb,
  nodes,
  setNodes,
  refreshDriveNodes,
  setUsage,
  sidebarLoadKey,
  setSidebarFolderChildren,
  setSidebarLoadedFolders,
  reloadSidebarCurrentPath,
}: UseDriveInteractionsParams): UseDriveInteractionsReturn {
  const [menuNodeId, setMenuNodeId] = useState<string | null>(null);
  const [renameNodeId, setRenameNodeId] = useState<string | null>(null);
  const [renameName, setRenameName] = useState('');
  const [shareNode, setShareNode] = useState<DriveNode | null>(null);
  const [newFolderMode, setNewFolderMode] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const [dragOver, setDragOver] = useState(false);
  const [draggingNodeIds, setDraggingNodeIds] = useState<string[]>([]);
  const [selectedNodeIds, setSelectedNodeIds] = useState<string[]>([]);
  const [dropTargetFolderId, setDropTargetFolderId] = useState<string | null>(null);

  const currentParentId = breadcrumb[breadcrumb.length - 1]?.id ?? '';

  const handleCreateFolder = useCallback(async () => {
    if (!newFolderName.trim()) { setNewFolderMode(false); return; }
    const created = await createDriveFolder(newFolderName.trim(), currentParentId || undefined);
    if (created) setNodes((prev) => [created, ...prev]);
    if (created && created.node_type === 'folder') {
      const key = sidebarLoadKey(currentParentId);
      setSidebarFolderChildren((prev) => {
        const current = prev[key] ?? [];
        const next = [...current, { id: created.id, name: created.name }]
          .filter((value, index, source) => source.findIndex((item) => item.id === value.id) === index)
          .sort((a, b) => a.name.localeCompare(b.name, 'ko'));
        return { ...prev, [key]: next };
      });
      setSidebarLoadedFolders((prev) => {
        const next = new Set(prev);
        next.add(key);
        return next;
      });
    }
    setNewFolderName('');
    setNewFolderMode(false);
  }, [newFolderName, currentParentId, setNodes, sidebarLoadKey, setSidebarFolderChildren, setSidebarLoadedFolders]);

  const handleRename = useCallback(async () => {
    if (!renameNodeId || !renameName.trim()) { setRenameNodeId(null); return; }
    const ok = await renameDriveNode(renameNodeId, renameName.trim());
    if (ok) setNodes((prev) => prev.map((n) => n.id === renameNodeId ? { ...n, name: renameName.trim() } : n));
    setRenameNodeId(null);
  }, [renameNodeId, renameName, setNodes]);

  const handleMoveNodes = useCallback(async (nodeIds: string[], targetParentId: string) => {
    if (!nodeIds.length) {
      setDraggingNodeIds([]);
      setDropTargetFolderId(null);
      return;
    }

    const movingById = new Set(nodeIds);
    let movedAny = false;
    const movedNodeIds: string[] = [];

    for (const nodeId of movingById) {
      const source = nodes.find((n) => n.id === nodeId);
      if (!source) continue;
      if (source.node_type === 'folder' && source.id === targetParentId) continue;
      if ((source.parent_id || '') === targetParentId) continue;

      const ok = await moveDriveNode(nodeId, targetParentId);
      if (ok) {
        movedAny = true;
        movedNodeIds.push(nodeId);
        setNodes((prev) => prev.filter((n) => n.id !== nodeId));
      }
    }

    if (movedAny) {
      await refreshDriveNodes();
      refreshDriveUsage(setUsage);
    }
    setSelectedNodeIds((prev) => prev.filter((id) => !movedNodeIds.includes(id)));
    reloadSidebarCurrentPath();
    setDraggingNodeIds([]);
    setDropTargetFolderId(null);
  }, [nodes, setNodes, refreshDriveNodes, setUsage, reloadSidebarCurrentPath]);

  const applySelection = useCallback((nodeId: string, multi: boolean) => {
    setSelectedNodeIds((prev) => {
      if (!multi) return [nodeId];
      const next = [...prev];
      const idx = next.indexOf(nodeId);
      if (idx === -1) next.push(nodeId);
      else next.splice(idx, 1);
      return next;
    });
  }, []);

  return {
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
    setSelectedNodeIds,
    dropTargetFolderId,
    setDropTargetFolderId,
    shareNode,
    setShareNode,
    handleCreateFolder,
    handleRename,
    handleMoveNodes,
    applySelection,
  };
}
