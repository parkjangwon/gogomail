'use client';
import { useCallback, type Dispatch, type SetStateAction } from 'react';
import {
  trashDriveNode, restoreDriveNode, deleteDriveNodePermanently,
  listDriveNodes, createDriveFolder,
} from '@/lib/api';
import type { DriveNode } from '@/lib/api';
import type { DroppedFileEntry } from '@/lib/drive/driveUtils';
import { ignoreNonCritical } from '@/lib/promise';
import { type DriveUsageSetter, refreshDriveUsage } from './driveUsageRefresh';
import { normalizeDroppedPath } from './driveViewHelpers';
import type { DriveUploadBatch, DriveUploadSource } from './driveViewHelpers';

type DriveTranslation = (key: string, values?: Record<string, string | number | Date>) => string;

interface UseDriveFileOpsParams {
  nodes: DriveNode[];
  setNodes: Dispatch<SetStateAction<DriveNode[]>>;
  setTrashNodes: Dispatch<SetStateAction<DriveNode[]>>;
  trashNodes: DriveNode[];
  setUsage: DriveUsageSetter;
  driveUploadResumable: boolean | null;
  enqueueDriveUploads: (
    items: Array<{ file: File; relativePath: string; parentId?: string; resumable: boolean; batchId: string; source: DriveUploadSource }>,
    batch?: DriveUploadBatch | null,
  ) => void;
  t: DriveTranslation;
}

export function useDriveFileOps({
  nodes,
  setNodes,
  setTrashNodes,
  trashNodes,
  setUsage,
  driveUploadResumable,
  enqueueDriveUploads,
  t,
}: UseDriveFileOpsParams) {
  const handleTrash = useCallback(async (nodeId: string) => {
    const ok = await trashDriveNode(nodeId);
    if (ok) setNodes((prev) => prev.filter((n) => n.id !== nodeId));
    refreshDriveUsage(setUsage);
  }, [setNodes, setUsage]);

  const handleRestore = useCallback(async (nodeId: string) => {
    const ok = await restoreDriveNode(nodeId);
    if (ok) {
      setTrashNodes((prev) => prev.filter((n) => n.id !== nodeId));
      refreshDriveUsage(setUsage);
    }
  }, [setTrashNodes, setUsage]);

  const handlePermanentDelete = useCallback(async (nodeId: string) => {
    if (!confirm(t('deleteConfirm'))) return;
    const ok = await deleteDriveNodePermanently(nodeId);
    if (ok) {
      setTrashNodes((prev) => prev.filter((n) => n.id !== nodeId));
      refreshDriveUsage(setUsage);
    }
  }, [setTrashNodes, setUsage, t]);

  const handleEmptyTrash = useCallback(async () => {
    if (!confirm(t('emptyTrashConfirm', { count: trashNodes.length }))) return;
    await Promise.all(trashNodes.map((n) => deleteDriveNodePermanently(n.id)));
    setTrashNodes([]);
    refreshDriveUsage(setUsage);
  }, [trashNodes, setTrashNodes, setUsage, t]);

  const getFolderCache = useCallback((): Map<string, string> => {
    const cache = new Map<string, string>();
    for (const node of nodes) {
      if (node.node_type !== 'folder') continue;
      cache.set(`${node.parent_id || ''}|${node.name}`, node.id);
    }
    return cache;
  }, [nodes]);

  const resolveFolderInParent = useCallback(async (
    parentId: string | undefined,
    name: string,
    cache: Map<string, string>,
  ): Promise<string | undefined> => {
    const key = `${parentId || ''}|${name}`;
    const cached = cache.get(key);
    if (cached) return cached;

    const children = await listDriveNodes(parentId || undefined);
    const found = children.find(
      (node) => node.node_type === 'folder' && node.parent_id === (parentId || '') && node.name === name,
    );
    if (!found) return undefined;

    cache.set(key, found.id);
    return found.id;
  }, []);

  const ensureFolderPath = useCallback(async (
    parentParts: string[],
    startParentId: string | undefined,
    cache: Map<string, string>,
  ): Promise<string | undefined> => {
    let current = startParentId || '';
    for (const part of parentParts) {
      const name = part.trim();
      if (!name) continue;

      const existingId = await resolveFolderInParent(current || undefined, name, cache);
      if (existingId) {
        current = existingId;
        continue;
      }

      const key = `${current}|${name}`;
      const created = await createDriveFolder(name, current || undefined);
      if (!created) return undefined;
      cache.set(key, created.id);
      current = created.id;
    }
    return current || undefined;
  }, [resolveFolderInParent]);

  const getUploadRelativePath = useCallback((file: File): string => {
    const withPath = (file as File & { webkitRelativePath?: string }).webkitRelativePath;
    if (withPath && withPath.trim()) return normalizeDroppedPath(withPath);
    return file.name;
  }, []);

  const buildDriveUploadBatch = useCallback((
    source: DriveUploadSource,
    files: Array<{ file: File; relativePath: string }>,
  ): DriveUploadBatch => {
    return {
      id: crypto.randomUUID(),
      source,
      fileCount: files.length,
      totalBytes: files.reduce((sum, item) => sum + item.file.size, 0),
      files: files.slice(0, 6).map((item) => ({
        name: item.file.name,
        relativePath: item.relativePath,
        size: item.file.size,
      })),
      createdAt: Date.now(),
    };
  }, []);

  const handleUploadEntries = useCallback(async (
    files: DroppedFileEntry[],
    targetParentId?: string,
    source: DriveUploadSource = 'drop',
  ) => {
    const folderCache = getFolderCache();
    const queueItems: Array<{
      file: File;
      relativePath: string;
      parentId?: string;
      resumable: boolean;
      batchId: string;
      source: DriveUploadSource;
    }> = [];

    for (const item of files) {
      const relPath = normalizeDroppedPath(item.relativePath);
      const segments = relPath.split('/').filter(Boolean);
      const fileName = segments.pop();
      if (!fileName) continue;

      const uploadParentId = await ensureFolderPath([...segments], targetParentId, folderCache);
      queueItems.push({
        file: item.file,
        relativePath: relPath,
        parentId: uploadParentId || undefined,
        resumable: driveUploadResumable === true,
        batchId: '',
        source,
      });
    }

    if (!queueItems.length) return;
    const batch = buildDriveUploadBatch(source, queueItems);
    enqueueDriveUploads(queueItems.map((item) => ({ ...item, batchId: batch.id })), batch);
  }, [getFolderCache, ensureFolderPath, driveUploadResumable, buildDriveUploadBatch, enqueueDriveUploads]);

  const handleUploadFromList = useCallback((
    files: FileList,
    targetParentId?: string,
    source: DriveUploadSource = 'picker',
  ) => {
    const entries = Array.from(files).map((file) => ({
      file,
      relativePath: getUploadRelativePath(file),
    }));
    ignoreNonCritical(handleUploadEntries(entries, targetParentId, source), 'drive.upload.entriesFromList');
  }, [getUploadRelativePath, handleUploadEntries]);

  return {
    getFolderCache,
    handleTrash,
    handleRestore,
    handlePermanentDelete,
    handleEmptyTrash,
    handleUploadEntries,
    handleUploadFromList,
  };
}
