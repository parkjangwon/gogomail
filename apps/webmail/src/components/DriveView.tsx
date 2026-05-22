'use client';

import { useState, useEffect, useRef, useCallback, useMemo } from 'react';
import { useTranslations } from 'next-intl';
import {
  DriveNode, DriveUsage,
  listDriveNodes, listTrashedDriveNodes, getDriveUsage, createDriveFolder,
  renameDriveNode, moveDriveNode, trashDriveNode, restoreDriveNode, deleteDriveNodePermanently,
  downloadDriveNode, uploadDriveFileWithOptions, getWebmailCapabilities, cancelDriveUploadSession,
} from '@/lib/api';
import { DriveNodeIcon } from '@/lib/driveNodeIcon';
import { formatBytes, formatDate, BreadcrumbItem, SidebarFolderItem, DRIVE_NODE_DRAG_MIME, DRIVE_NODE_DRAG_TEXT, DroppedFileEntry, FileSystemEntryLike, DirectoryReaderLike } from '@/lib/drive/driveUtils';
import { DriveShareModal } from './DriveShareModal';
import { DriveNodeMenu } from './drive/DriveNodeMenu';
import {
  FolderIcon, ArrowUpTrayIcon, FolderPlusIcon,
  EllipsisVerticalIcon, ArrowDownTrayIcon, LinkIcon, PencilIcon,
  TrashIcon, XMarkIcon, ArrowPathIcon, ChevronRightIcon, ArrowUturnLeftIcon, PauseIcon, PlayIcon,
} from '@heroicons/react/24/outline';
import { FolderIcon as FolderSolid, TrashIcon as TrashSolid } from '@heroicons/react/24/solid';

type DriveUploadStatus = 'queued' | 'creating_session' | 'uploading' | 'paused' | 'finalizing' | 'done' | 'error' | 'canceled';
type DriveUploadSource = 'picker' | 'folder' | 'drop';

const DRIVE_UPLOAD_CONCURRENCY = 3;

type DriveUploadBatch = {
  id: string;
  source: DriveUploadSource;
  fileCount: number;
  totalBytes: number;
  files: Array<{
    name: string;
    relativePath: string;
    size: number;
  }>;
  createdAt: number;
};

type DriveUploadItem = {
  id: string;
  file: File;
  parentId?: string;
  relativePath: string;
  status: DriveUploadStatus;
  uploadedBytes: number;
  totalBytes: number;
  resumable: boolean;
  sessionId?: string;
  storageBackend?: string;
  error?: string;
  node?: DriveNode;
  batchId?: string;
  source?: DriveUploadSource;
};


function getDriveNodeDragPayload(dataTransfer: DataTransfer): string | null {
  const raw = dataTransfer.getData(DRIVE_NODE_DRAG_MIME);
  if (raw) {
    try {
      const parsed = JSON.parse(raw) as { nodeId?: string; nodeIds?: string[] };
      if (parsed.nodeIds && parsed.nodeIds.length > 0) return JSON.stringify(parsed);
      return parsed.nodeId ?? null;
    } catch {
      return null;
    }
  }

  const fallback = dataTransfer.getData(DRIVE_NODE_DRAG_TEXT);
  if (fallback.startsWith('node:')) {
    const nodeId = fallback.slice('node:'.length).trim();
    return nodeId || null;
  }
  if (fallback.startsWith('nodes:')) {
    const payload = fallback.slice('nodes:'.length).trim();
    return payload || null;
  }

  const plain = dataTransfer.getData('text/plain');
  if (plain.startsWith(`${DRIVE_NODE_DRAG_TEXT}:`)) {
    const nodeId = plain.slice(`${DRIVE_NODE_DRAG_TEXT}:`.length).trim();
    return nodeId || null;
  }

  return null;
}

function parseDriveNodeIds(payload: string | null): string[] | null {
  if (!payload) return null;
  try {
    const parsed = JSON.parse(payload) as { nodeIds?: string[] };
    if (Array.isArray(parsed.nodeIds) && parsed.nodeIds.length > 0) return [...new Set(parsed.nodeIds)];
  } catch {
    if (payload.includes(',')) {
      const ids = payload.split(',').map((v) => v.trim()).filter(Boolean);
      if (ids.length > 0) return ids;
    }
    return [payload];
  }
  return [payload];
}

function isDriveNodeDrag(dataTransfer: DataTransfer): boolean {
  return (
    Array.from(dataTransfer.types).includes(DRIVE_NODE_DRAG_MIME) ||
    Array.from(dataTransfer.types).includes(DRIVE_NODE_DRAG_TEXT)
  );
}

function createDriveDragGhost(count: number, names: string[], tFn: (key: string, values?: Record<string, string | number | Date>) => string): HTMLElement {
  const wrap = document.createElement('div');
  wrap.style.position = 'absolute';
  wrap.style.top = '-9999px';
  wrap.style.left = '-9999px';
  wrap.style.padding = '10px 12px';
  wrap.style.borderRadius = '10px';
  wrap.style.background = '#121926';
  wrap.style.color = '#f8fafc';
  wrap.style.boxShadow = '0 10px 24px rgba(8, 12, 24, 0.35)';
  wrap.style.border = '1px solid rgba(148, 163, 184, 0.28)';
  wrap.style.fontSize = '12px';
  wrap.style.fontFamily = 'system-ui, -apple-system, Segoe UI, Roboto, sans-serif';
  wrap.style.minWidth = '130px';
  wrap.style.maxWidth = '220px';
  wrap.style.whiteSpace = 'nowrap';
  wrap.style.overflow = 'hidden';
  wrap.style.animation = 'driveMultiDragPulse 1s ease-in-out infinite';

  const title = document.createElement('div');
  title.textContent = tFn('draggingItems', { count });
  title.style.fontWeight = '600';
  title.style.marginBottom = '4px';
  title.style.letterSpacing = '0.01em';
  wrap.appendChild(title);

  const detail = document.createElement('div');
  const visible = names.length > 0 ? names.slice(0, 2) : [];
  detail.textContent = visible.length > 0
    ? `${visible.join(', ')}${names.length > 2 ? ', ...' : ''}`
    : tFn('selectedItems');
  detail.style.opacity = '0.92';
  wrap.appendChild(detail);

  return wrap;
}

function normalizeDroppedPath(path: string): string {
  return path.replace(/[\\/]+/g, '/').replace(/^\/+|\/+$/g, '');
}

function getDriveUploadSourceLabel(source: DriveUploadSource, tFn: (key: string) => string): string {
  switch (source) {
    case 'picker':
      return tFn('sourceFilePicker');
    case 'folder':
      return tFn('sourceFolderPicker');
    case 'drop':
      return tFn('sourceDrop');
  }
}

function driveUploadNeedsFreshSession(message: string): boolean {
  const lower = message.toLowerCase();
  return lower.includes('storage store') && lower.includes('is required');
}

function formatDriveUploadError(error: unknown, tFn: (key: string) => string): string {
  const message = error instanceof Error ? error.message : String(error ?? tFn('uploadFailed'));
  const lower = message.toLowerCase();
  if (
    lower.includes('duplicate key') ||
    lower.includes('already exists') ||
    lower.includes('conflict') ||
    lower.includes('same name')
  ) {
    return tFn('duplicateFileError');
  }
  return message || tFn('uploadFailed');
}

async function readAllEntries(reader: DirectoryReaderLike): Promise<FileSystemEntryLike[]> {
  const entries: FileSystemEntryLike[] = [];
  while (true) {
    const chunk = await new Promise<FileSystemEntryLike[]>((resolve, reject) => {
      try {
        reader.readEntries(resolve, (err) => reject(err));
      } catch (err) {
        reject(err as DOMException);
      }
    });
    if (!chunk.length) break;
    entries.push(...chunk);
  }
  return entries;
}

function readFileFromEntry(entry: FileSystemEntryLike): Promise<File> {
  return new Promise((resolve, reject) => {
    entry.file((file) => resolve(file), (err) => reject(err));
  });
}

async function collectDroppedFilesFromEntry(entry: FileSystemEntryLike, basePath: string, out: DroppedFileEntry[]) {
  if (entry.isFile) {
    const file = await readFileFromEntry(entry);
    const relativePath = normalizeDroppedPath(basePath ? `${basePath}/${entry.name}` : entry.name);
    out.push({ file, relativePath });
    return;
  }

  if (!entry.isDirectory) return;
  const nextBasePath = normalizeDroppedPath(basePath ? `${basePath}/${entry.name}` : entry.name);
  const children = await readAllEntries(entry.createReader());
  for (const child of children) {
    await collectDroppedFilesFromEntry(child, nextBasePath, out);
  }
}

type FileSystemHandleLike =
  | { kind: 'file'; name: string; getFile?: () => Promise<File> }
  | { kind: 'directory'; name: string; entries?: () => AsyncIterable<[string, FileSystemHandleLike]> | Iterable<[string, FileSystemHandleLike]> };

async function collectDroppedFilesFromHandle(
  handle: FileSystemHandleLike,
  basePath: string,
  out: DroppedFileEntry[],
) {
  if (handle.kind === 'file') {
    const file = handle.getFile ? await handle.getFile() : null;
    if (!file) return;
    const relativePath = normalizeDroppedPath(basePath ? `${basePath}/${handle.name}` : handle.name);
    out.push({ file, relativePath });
    return;
  }

  if (handle.kind !== 'directory' || !handle.entries) return;
  const nextBasePath = normalizeDroppedPath(basePath ? `${basePath}/${handle.name}` : handle.name);
  const iterator = handle.entries();
  for await (const [, child] of iterator as AsyncIterable<[string, FileSystemHandleLike]>) {
    await collectDroppedFilesFromHandle(child, nextBasePath, out);
  }
}

async function collectDroppedFiles(dataTransfer: DataTransfer): Promise<DroppedFileEntry[]> {
  const entries: DroppedFileEntry[] = [];
  const seen = new Set<string>();
  const pushEntry = (file: File, relativePath: string) => {
    const normalized = normalizeDroppedPath(relativePath || file.name);
    const key = `${normalized}\u0000${file.size}\u0000${file.lastModified}`;
    if (seen.has(key)) return;
    seen.add(key);
    entries.push({ file, relativePath: normalized });
  };

  const fileSnapshot = Array.from(dataTransfer.files || []).map((file) => ({
    file,
    relativePath: (file as File & { webkitRelativePath?: string }).webkitRelativePath?.trim() || file.name,
  }));
  for (const item of fileSnapshot) {
    pushEntry(item.file, item.relativePath);
  }

  const dataTransferItemItems = Array.from(dataTransfer.items || []);
  for (const item of dataTransferItemItems) {
    if (item.kind !== 'file') continue;

    const handleItem = item as DataTransferItem & {
      getAsFileSystemHandle?: () => Promise<FileSystemHandleLike | null>;
    };
    if (handleItem.getAsFileSystemHandle) {
      try {
        const handle = await handleItem.getAsFileSystemHandle();
        if (handle) {
          await collectDroppedFilesFromHandle(handle, '', entries);
          continue;
        }
      } catch {
        // fall through to legacy entry/file handling
      }
    }

    const webkitLikeItem = item as DataTransferItem & {
      webkitGetAsEntry?: () => FileSystemEntryLike | null;
    };
    const entry = webkitLikeItem.webkitGetAsEntry?.() as FileSystemEntryLike | null;
    if (entry) {
      const nested: DroppedFileEntry[] = [];
      await collectDroppedFilesFromEntry(entry, '', nested);
      for (const child of nested) pushEntry(child.file, child.relativePath);
      continue;
    }

    const file = item.getAsFile();
    if (file) pushEntry(file, file.name);
  }

  return entries;
}

export function DriveView() {
  const t = useTranslations('drive');
  const DRIVE_UPLOAD_STATUS_LABELS = useMemo<Record<DriveUploadStatus, string>>(() => ({
    queued: t('upload.status.queued'),
    creating_session: t('upload.status.creatingSession'),
    uploading: t('upload.status.uploading'),
    paused: t('upload.status.paused'),
    finalizing: t('upload.status.finalizing'),
    done: t('upload.status.done'),
    error: t('upload.status.error'),
    canceled: t('upload.status.canceled'),
  }), [t]);
  const [activeSection, setActiveSection] = useState<'drive' | 'trash'>('drive');
  const [breadcrumb, setBreadcrumb] = useState<BreadcrumbItem[]>([{ id: '', name: t('myDrive') }]);
  const [nodes, setNodes] = useState<DriveNode[]>([]);
  const [trashNodes, setTrashNodes] = useState<DriveNode[]>([]);
  const [usage, setUsage] = useState<DriveUsage | null>(null);
  const [loading, setLoading] = useState(true);
  const [trashLoading, setTrashLoading] = useState(false);
  const [menuNodeId, setMenuNodeId] = useState<string | null>(null);
  const [renameNodeId, setRenameNodeId] = useState<string | null>(null);
  const [renameName, setRenameName] = useState('');
  const [shareNode, setShareNode] = useState<DriveNode | null>(null);
  const [newFolderMode, setNewFolderMode] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');
  const [driveUploadBatch, setDriveUploadBatch] = useState<DriveUploadBatch | null>(null);
  const [driveUploads, setDriveUploads] = useState<DriveUploadItem[]>([]);
  const [driveUploadModalOpen, setDriveUploadModalOpen] = useState(false);
  const [driveUploadResumable, setDriveUploadResumable] = useState<boolean | null>(null);
  const [dragOver, setDragOver] = useState(false);
  const [draggingNodeIds, setDraggingNodeIds] = useState<string[]>([]);
  const [selectedNodeIds, setSelectedNodeIds] = useState<string[]>([]);
  const [dropTargetFolderId, setDropTargetFolderId] = useState<string | null>(null);
  const [sidebarFolderChildren, setSidebarFolderChildren] = useState<Record<string, SidebarFolderItem[]>>({});
  const [sidebarExpandedFolders, setSidebarExpandedFolders] = useState<Set<string>>(new Set(['']));
  const [sidebarLoadedFolders, setSidebarLoadedFolders] = useState<Set<string>>(new Set());
  const [sidebarLoadingFolders, setSidebarLoadingFolders] = useState<Record<string, boolean>>({});
  const fileInputRef = useRef<HTMLInputElement>(null);
  const folderInputRef = useRef<HTMLInputElement>(null);
  const newFolderRef = useRef<HTMLInputElement>(null);
  const renameRef = useRef<HTMLInputElement>(null);
  const driveUploadControllersRef = useRef<Map<string, AbortController>>(new Map());
  const driveUploadAbortReasonsRef = useRef<Map<string, 'pause' | 'cancel'>>(new Map());
  const driveUploadActiveIdsRef = useRef<Set<string>>(new Set());
  const driveUploadsRef = useRef<DriveUploadItem[]>([]);
  const driveUploadSchedulerRef = useRef(false);
  const driveUploadModalDismissedRef = useRef(false);

  const currentParentId = breadcrumb[breadcrumb.length - 1]?.id ?? '';

  function getUploadRelativePath(file: File): string {
    const withPath = (file as File & { webkitRelativePath?: string }).webkitRelativePath;
    if (withPath && withPath.trim()) return normalizeDroppedPath(withPath);
    return file.name;
  }

  function buildDriveUploadBatch(
    source: DriveUploadSource,
    files: Array<{ file: File; relativePath: string }>,
  ): DriveUploadBatch {
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
  }

  useEffect(() => {
    const folderInput = folderInputRef.current;
    if (!folderInput) return;
    folderInput.setAttribute('webkitdirectory', '');
    folderInput.setAttribute('directory', '');
  }, []);

  const loadNodes = useCallback(async (parentId: string) => {
    setLoading(true);
    const data = await listDriveNodes(parentId || undefined);
    setNodes(data.sort((a, b) => {
      if (a.node_type !== b.node_type) return a.node_type === 'folder' ? -1 : 1;
      return a.name.localeCompare(b.name, 'ko');
    }));
    setLoading(false);
  }, []);

  const loadTrashNodes = useCallback(async () => {
    setTrashLoading(true);
    const data = await listTrashedDriveNodes();
    setTrashNodes(data.sort((a, b) => a.name.localeCompare(b.name, 'ko')));
    setTrashLoading(false);
  }, []);

  const sidebarLoadKey = useCallback((folderId: string) => folderId || '__ROOT__', []);

  const loadSidebarFolders = useCallback(async (parentId: string) => {
    const key = sidebarLoadKey(parentId);
    if (sidebarLoadedFolders.has(key) || sidebarLoadingFolders[key]) return;

    setSidebarLoadingFolders((prev) => ({ ...prev, [key]: true }));
    try {
      const data = await listDriveNodes(parentId || undefined);
      const sortedFolders = data
        .filter((n) => n.node_type === 'folder')
        .sort((a, b) => a.name.localeCompare(b.name, 'ko'))
        .map((n) => ({ id: n.id, name: n.name }));
      setSidebarFolderChildren((prev) => ({ ...prev, [key]: sortedFolders }));
      setSidebarLoadedFolders((prev) => {
        const next = new Set(prev);
        next.add(key);
        return next;
      });
    } finally {
      setSidebarLoadingFolders((prev) => {
        const next = { ...prev };
        delete next[key];
        return next;
      });
    }
  }, [sidebarLoadedFolders, sidebarLoadKey, sidebarLoadingFolders]);

  const reloadSidebarCurrentPath = useCallback(() => {
    setSidebarFolderChildren({});
    setSidebarLoadedFolders(new Set());
  }, []);

  useEffect(() => {
    loadNodes(currentParentId);
    getDriveUsage().then(setUsage).catch(() => {});
  }, [currentParentId, loadNodes]);

  useEffect(() => {
    if (activeSection === 'trash') loadTrashNodes();
  }, [activeSection, loadTrashNodes]);

  useEffect(() => {
    if (activeSection === 'drive') loadSidebarFolders('');
  }, [activeSection, loadSidebarFolders]);

  useEffect(() => {
    driveUploadsRef.current = driveUploads;
  }, [driveUploads]);

  useEffect(() => {
    let alive = true;
    getWebmailCapabilities().then((caps) => {
      if (!alive) return;
      setDriveUploadResumable(Boolean(caps?.drive?.resumable_chunked_uploads));
    });
    return () => {
      alive = false;
    };
  }, []);

  useEffect(() => () => {
    for (const controller of driveUploadControllersRef.current.values()) {
      controller.abort();
    }
    driveUploadControllersRef.current.clear();
    driveUploadAbortReasonsRef.current.clear();
  }, []);

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

  async function handleCreateFolder() {
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
  }

  async function handleRename() {
    if (!renameNodeId || !renameName.trim()) { setRenameNodeId(null); return; }
    const ok = await renameDriveNode(renameNodeId, renameName.trim());
    if (ok) setNodes((prev) => prev.map((n) => n.id === renameNodeId ? { ...n, name: renameName.trim() } : n));
    setRenameNodeId(null);
  }

  async function handleTrash(nodeId: string) {
    const ok = await trashDriveNode(nodeId);
    if (ok) setNodes((prev) => prev.filter((n) => n.id !== nodeId));
    getDriveUsage().then(setUsage).catch(() => {});
  }

  async function handleRestore(nodeId: string) {
    const ok = await restoreDriveNode(nodeId);
    if (ok) {
      setTrashNodes((prev) => prev.filter((n) => n.id !== nodeId));
      getDriveUsage().then(setUsage).catch(() => {});
    }
  }

  async function handlePermanentDelete(nodeId: string) {
    if (!confirm(t('deleteConfirm'))) return;
    const ok = await deleteDriveNodePermanently(nodeId);
    if (ok) {
      setTrashNodes((prev) => prev.filter((n) => n.id !== nodeId));
      getDriveUsage().then(setUsage).catch(() => {});
    }
  }

  async function handleEmptyTrash() {
    if (!confirm(t('emptyTrashConfirm', { count: trashNodes.length }))) return;
    await Promise.all(trashNodes.map((n) => deleteDriveNodePermanently(n.id)));
    setTrashNodes([]);
    getDriveUsage().then(setUsage).catch(() => {});
  }

  function getFolderCache(): Map<string, string> {
    const cache = new Map<string, string>();
    for (const node of nodes) {
      if (node.node_type !== 'folder') continue;
      cache.set(`${node.parent_id || ''}|${node.name}`, node.id);
    }
    return cache;
  }

  async function resolveFolderInParent(
    parentId: string | undefined,
    name: string,
    cache: Map<string, string>,
  ): Promise<string | undefined> {
    const key = `${parentId || ''}|${name}`;
    const cached = cache.get(key);
    if (cached) return cached;

    const children = await listDriveNodes(parentId || undefined);
    const found = children.find((node) => node.node_type === 'folder' && node.parent_id === (parentId || '') && node.name === name);
    if (!found) return undefined;

    cache.set(key, found.id);
    return found.id;
  }

  async function ensureFolderPath(
    parentParts: string[],
    startParentId: string | undefined,
    cache: Map<string, string>,
  ): Promise<string | undefined> {
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
  }

  const updateDriveUpload = useCallback((uploadId: string, updater: (item: DriveUploadItem) => DriveUploadItem) => {
    setDriveUploads((prev) => prev.map((item) => (item.id === uploadId ? updater(item) : item)));
  }, []);

  const refreshDriveNodes = useCallback(async () => {
    await loadNodes(currentParentId);
    getDriveUsage().then(setUsage).catch(() => {});
  }, [currentParentId, loadNodes]);

  const runDriveUpload = useCallback(async (uploadId: string) => {
    const next = driveUploadsRef.current.find((item) => item.id === uploadId);
    if (!next || next.status !== 'queued') return;

    driveUploadActiveIdsRef.current.add(next.id);
    const controller = new AbortController();
    driveUploadControllersRef.current.set(next.id, controller);
    driveUploadAbortReasonsRef.current.delete(next.id);

    try {
      updateDriveUpload(next.id, (item) => ({
        ...item,
        status: 'creating_session',
        error: undefined,
      }));

      const node = await uploadDriveFileWithOptions(next.file, {
        parentId: next.parentId,
        resumable: next.resumable,
        resumeSessionId: next.sessionId,
        signal: controller.signal,
        onProgress: (progress) => {
          updateDriveUpload(next.id, (item) => ({
            ...item,
            status: progress.phase === 'creating_session'
              ? 'creating_session'
              : progress.phase === 'finalizing'
                ? 'finalizing'
                : 'uploading',
            sessionId: progress.sessionId ?? item.sessionId,
            storageBackend: progress.storageBackend ?? item.storageBackend,
            uploadedBytes: progress.uploadedBytes,
            totalBytes: progress.totalBytes,
          }));
        },
      });

      updateDriveUpload(next.id, (item) => ({
        ...item,
        status: 'done',
        uploadedBytes: item.totalBytes,
        node: node ?? item.node,
        error: undefined,
      }));
      await refreshDriveNodes();
    } catch (error) {
      const reason = driveUploadAbortReasonsRef.current.get(next.id);
      if (controller.signal.aborted || reason === 'pause' || reason === 'cancel') {
        updateDriveUpload(next.id, (item) => ({
          ...item,
          status: reason === 'cancel' ? 'canceled' : 'paused',
          error: undefined,
        }));
      } else {
        const message = formatDriveUploadError(error, t);
        updateDriveUpload(next.id, (item) => ({
          ...item,
          status: 'error',
          error: message,
          sessionId: driveUploadNeedsFreshSession(message) ? undefined : item.sessionId,
          storageBackend: driveUploadNeedsFreshSession(message) ? undefined : item.storageBackend,
          uploadedBytes: driveUploadNeedsFreshSession(message) ? 0 : item.uploadedBytes,
        }));
      }
    } finally {
      driveUploadControllersRef.current.delete(next.id);
      driveUploadAbortReasonsRef.current.delete(next.id);
      driveUploadActiveIdsRef.current.delete(next.id);
      driveUploadSchedulerRef.current = false;
      void scheduleDriveUploads();
    }
  }, [refreshDriveNodes, updateDriveUpload]);

  const scheduleDriveUploads = useCallback(() => {
    if (driveUploadSchedulerRef.current) return;
    driveUploadSchedulerRef.current = true;
    try {
      const runningCount = driveUploadActiveIdsRef.current.size;
      let availableSlots = DRIVE_UPLOAD_CONCURRENCY - runningCount;
      while (availableSlots > 0) {
        const next = driveUploadsRef.current.find((item) => item.status === 'queued' && !driveUploadActiveIdsRef.current.has(item.id));
        if (!next) break;
        availableSlots -= 1;
        void runDriveUpload(next.id);
      }
    } finally {
      driveUploadSchedulerRef.current = false;
    }
  }, [runDriveUpload]);

  useEffect(() => {
    void scheduleDriveUploads();
  }, [driveUploads, scheduleDriveUploads]);

  const enqueueDriveUploads = useCallback((
    items: Array<{ file: File; relativePath: string; parentId?: string; resumable: boolean; batchId: string; source: DriveUploadSource }>,
    batch?: DriveUploadBatch | null,
  ) => {
    if (!items.length) return;
    driveUploadModalDismissedRef.current = false;
    setDriveUploadModalOpen(true);
    if (batch) setDriveUploadBatch(batch);
    setDriveUploads((prev) => [
      ...prev,
      ...items.map((item) => ({
        id: crypto.randomUUID(),
        file: item.file,
        parentId: item.parentId,
        relativePath: item.relativePath,
        status: 'queued' as const,
        uploadedBytes: 0,
        totalBytes: item.file.size,
        resumable: item.resumable,
        batchId: item.batchId,
        source: item.source,
      })),
    ]);
    void scheduleDriveUploads();
  }, [scheduleDriveUploads]);

  async function handleUploadEntries(files: DroppedFileEntry[], targetParentId?: string, source: DriveUploadSource = 'drop') {
    const folderCache = getFolderCache();
    const queueItems: Array<{ file: File; relativePath: string; parentId?: string; resumable: boolean; batchId: string; source: DriveUploadSource }> = [];

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
  }

  function handleUploadFromList(files: FileList, targetParentId?: string, source: DriveUploadSource = 'picker') {
    const entries = Array.from(files).map((file) => ({ file, relativePath: getUploadRelativePath(file) }));
    handleUploadEntries(entries, targetParentId, source).catch(() => {});
  }

  function pauseDriveUpload(uploadId: string) {
    const controller = driveUploadControllersRef.current.get(uploadId);
    if (!controller) return;
    driveUploadAbortReasonsRef.current.set(uploadId, 'pause');
    controller.abort();
  }

  async function resumeDriveUpload(uploadId: string) {
    updateDriveUpload(uploadId, (item) => ({
      ...item,
      status: 'queued',
      error: undefined,
    }));
    driveUploadModalDismissedRef.current = false;
    setDriveUploadModalOpen(true);
    await scheduleDriveUploads();
  }

  async function cancelDriveUpload(uploadId: string) {
    const item = driveUploadsRef.current.find((entry) => entry.id === uploadId);
    if (!item) return;
    const controller = driveUploadControllersRef.current.get(uploadId);
    if (controller) {
      driveUploadAbortReasonsRef.current.set(uploadId, 'cancel');
      controller.abort();
    }
    if (item.sessionId) {
      await cancelDriveUploadSession(item.sessionId);
    }
    updateDriveUpload(uploadId, (current) => ({
      ...current,
      status: 'canceled',
      error: undefined,
    }));
  }

  async function handleMoveNodes(nodeIds: string[], targetParentId: string) {
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
      loadNodes(currentParentId);
      getDriveUsage().then(setUsage).catch(() => {});
    }
    setSelectedNodeIds((prev) => prev.filter((id) => !movedNodeIds.includes(id)));
    reloadSidebarCurrentPath();
    setDraggingNodeIds([]);
    setDropTargetFolderId(null);
  }

  function applySelection(nodeId: string, multi: boolean) {
    setSelectedNodeIds((prev) => {
      if (!multi) return [nodeId];
      const next = [...prev];
      const idx = next.indexOf(nodeId);
      if (idx === -1) next.push(nodeId);
      else next.splice(idx, 1);
      return next;
    });
  }

  const usedPct = usage && usage.quota_limit > 0 ? Math.min(100, (usage.quota_used / usage.quota_limit) * 100) : 0;
  const barColor = usedPct >= 90 ? '#ef4444' : usedPct >= 70 ? '#f59e0b' : '#22c55e';
  const activeDriveUploads = driveUploads.filter((item) => item.status === 'creating_session' || item.status === 'uploading' || item.status === 'finalizing');
  const queuedDriveUploads = driveUploads.filter((item) => item.status === 'queued');
  const completedDriveUploads = driveUploads.filter((item) => item.status === 'done' || item.status === 'canceled');
  const erroredDriveUploads = driveUploads.filter((item) => item.status === 'error');
  const totalDriveUploadBytes = driveUploads.reduce((sum, item) => sum + item.totalBytes, 0);
  const totalDriveUploadProgressBytes = driveUploads.reduce((sum, item) => sum + Math.min(item.uploadedBytes, item.totalBytes), 0);
  const uploadProgressPct = totalDriveUploadBytes > 0 ? Math.min(100, Math.round((totalDriveUploadProgressBytes / totalDriveUploadBytes) * 100)) : 0;
  const uploadPanelOpen = driveUploadModalOpen && driveUploads.length > 0;
  const uploadBatchNames = driveUploadBatch?.files ?? [];
  const draggingNodeNames = draggingNodeIds
    .map((id) => nodes.find((node) => node.id === id)?.name)
    .filter(Boolean) as string[];

  const toggleSidebarFolder = useCallback((folderId: string) => {
    setSidebarExpandedFolders((prev) => {
      const next = new Set(prev);
      if (next.has(folderId)) next.delete(folderId);
      else next.add(folderId);
      return next;
    });
    void loadSidebarFolders(folderId);
  }, [loadSidebarFolders]);

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
                  if (!e.currentTarget.contains(e.relatedTarget as Node)) setDropTargetFolderId((prev) => (prev === folder.id ? null : prev));
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

  const sidebarDropTargetActive = dropTargetFolderId === '';

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
            {trashNodes.length > 0 && (
              <span style={{ marginLeft: 'auto', fontSize: '11px', background: 'var(--color-bg-tertiary)', color: 'var(--color-text-tertiary)', borderRadius: '10px', padding: '1px 6px' }}>{trashNodes.length}</span>
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

      {/* ── Main content ── */}
      {activeSection === 'trash' ? (
        /* Trash view */
        <div style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column' }}>
          {/* Trash toolbar */}
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '12px 20px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
            <TrashSolid style={{ width: '18px', height: '18px', color: 'var(--color-text-tertiary)' }} />
            <span style={{ fontSize: '15px', fontWeight: 600, color: 'var(--color-text-primary)', flex: 1 }}>{t('trash')}</span>
            <button onClick={loadTrashNodes} title={t('refresh')}
              style={{ padding: '5px 8px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', cursor: 'pointer', color: 'var(--color-text-secondary)', display: 'flex', alignItems: 'center' }}>
              <ArrowPathIcon style={{ width: '15px', height: '15px' }} />
            </button>
            {trashNodes.length > 0 && (
              <button onClick={handleEmptyTrash}
                style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '5px 14px', borderRadius: '6px', border: '1px solid var(--color-destructive)', background: 'transparent', color: 'var(--color-destructive)', fontSize: '13px', fontWeight: 500, cursor: 'pointer' }}>
                <TrashIcon style={{ width: '14px', height: '14px' }} />
                {t('emptyTrash')}
              </button>
            )}
          </div>

          {/* Trash file list */}
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
                  <div key={node.id} style={{ display: 'flex', alignItems: 'center', gap: '12px', padding: '10px 14px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)' }}
                    onMouseEnter={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-secondary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLDivElement).style.background = 'var(--color-bg-primary)'; }}
                  >
                    <div style={{ flexShrink: 0 }}><DriveNodeIcon node={node} /></div>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{node.name}</div>
                      <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>
                        {node.node_type === 'file' ? formatBytes(node.size) : t('folderLabel')} · {formatDate(node.updated_at)}
                      </div>
                    </div>
                    <button onClick={() => handleRestore(node.id)}
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '5px', padding: '5px 12px', borderRadius: '6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', fontSize: '12px', cursor: 'pointer', flexShrink: 0 }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'transparent'; }}
                    >
                      <ArrowUturnLeftIcon style={{ width: '13px', height: '13px' }} />
                      {t('restore')}
                    </button>
                    <button onClick={() => handlePermanentDelete(node.id)}
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
            <button onClick={() => loadNodes(currentParentId)} title={t('refresh')}
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
            <div
              data-testid="drive-upload-modal"
              role="dialog"
              aria-modal="true"
              aria-label={t('uploadModal.title')}
              onDragOver={(e) => {
                if (Array.from(e.dataTransfer.types).includes('Files')) {
                  e.preventDefault();
                  e.stopPropagation();
                }
              }}
              onDrop={async (e) => {
                e.preventDefault();
                e.stopPropagation();
                const files = await collectDroppedFiles(e.dataTransfer);
                if (files.length) await handleUploadEntries(files, currentParentId || undefined, 'drop');
              }}
              style={{
                position: 'fixed',
                inset: 0,
                zIndex: 60,
                background: 'rgba(15, 23, 42, 0.58)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                padding: '24px',
              }}
              onMouseDown={(e) => {
                if (e.target === e.currentTarget && activeDriveUploads.length === 0) {
                  driveUploadModalDismissedRef.current = true;
                  setDriveUploadModalOpen(false);
                }
              }}
            >
              <div style={{ width: 'min(1120px, 100%)', maxHeight: 'min(84vh, 920px)', display: 'flex', flexDirection: 'column', borderRadius: '10px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', boxShadow: '0 24px 72px rgba(15, 23, 42, 0.42)', overflow: 'hidden' }}>
                <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: '16px', padding: '18px 20px', borderBottom: '1px solid var(--color-border-subtle)' }}>
                  <div style={{ minWidth: 0, flex: 1 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px', flexWrap: 'wrap' }}>
                      <div style={{ fontSize: '15px', fontWeight: 700, color: 'var(--color-text-primary)' }}>{t('uploadModal.title')}</div>
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '3px 8px', borderRadius: '999px', background: 'var(--color-bg-secondary)', color: 'var(--color-text-secondary)', fontSize: '11px', fontWeight: 500 }}>
                        {t('uploadModal.selectedCount', { count: driveUploadBatch?.fileCount ?? driveUploads.length })}
                      </span>
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '3px 8px', borderRadius: '999px', background: 'var(--color-accent-subtle)', color: 'var(--color-accent)', fontSize: '11px', fontWeight: 500 }}>
                        {t('uploadModal.concurrent', { active: activeDriveUploads.length, max: DRIVE_UPLOAD_CONCURRENCY })}
                      </span>
                      <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '3px 8px', borderRadius: '999px', background: 'rgba(34, 197, 94, 0.10)', color: '#15803d', fontSize: '11px', fontWeight: 500 }}>
                        {driveUploadResumable ? t('uploadModal.resumableOn') : t('uploadModal.resumableOff')}
                      </span>
                    </div>
                    <div style={{ marginTop: '6px', fontSize: '12px', lineHeight: 1.5, color: 'var(--color-text-tertiary)' }}>
                      {driveUploadResumable
                        ? t('uploadModal.descResumable')
                        : t('uploadModal.descNonResumable')}
                    </div>
                  </div>
                  <button
                    type="button"
                    onClick={() => {
                      driveUploadModalDismissedRef.current = true;
                      setDriveUploadModalOpen(false);
                    }}
                    title={t('uploadModal.closeWindow')}
                    style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '32px', height: '32px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', flexShrink: 0 }}
                  >
                    <XMarkIcon style={{ width: '14px', height: '14px' }} />
                  </button>
                </div>
                {driveUploadBatch && (
                  <div style={{ padding: '14px 20px', borderBottom: '1px solid var(--color-border-subtle)', background: 'linear-gradient(180deg, rgba(148, 163, 184, 0.06), rgba(148, 163, 184, 0.02))' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '8px', flexWrap: 'wrap', fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
                      <span>{getDriveUploadSourceLabel(driveUploadBatch.source, t)}</span>
                      <span>{formatBytes(driveUploadBatch.totalBytes)}</span>
                      <span>{t('uploadModal.filesCount', { count: driveUploadBatch.fileCount })}</span>
                    </div>
                    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                      {uploadBatchNames.map((item, index) => (
                        <span key={`${driveUploadBatch.id}-${index}-${item.relativePath}-${item.size}`} style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '6px 10px', borderRadius: '999px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', fontSize: '11px', maxWidth: '100%', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={`${item.relativePath} · ${formatBytes(item.size)}`}>
                          <strong style={{ fontWeight: 600 }}>{item.name}</strong>
                          <span style={{ color: 'var(--color-text-tertiary)' }}>{formatBytes(item.size)}</span>
                        </span>
                      ))}
                      {driveUploadBatch.fileCount > uploadBatchNames.length && (
                        <span style={{ display: 'inline-flex', alignItems: 'center', padding: '6px 10px', borderRadius: '999px', border: '1px dashed var(--color-border-default)', color: 'var(--color-text-tertiary)', fontSize: '11px' }}>
                          {t('uploadModal.moreCount', { count: driveUploadBatch.fileCount - uploadBatchNames.length })}
                        </span>
                      )}
                    </div>
                  </div>
                )}
                <div style={{ padding: '12px 20px', borderBottom: '1px solid var(--color-border-subtle)' }}>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '12px', marginBottom: '10px', flexWrap: 'wrap', fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
                    <span>{t('uploadModal.inProgress', { count: activeDriveUploads.length })}</span>
                    <span>{t('uploadModal.queued', { count: queuedDriveUploads.length })}</span>
                    <span>{t('uploadModal.completed', { count: completedDriveUploads.length })}</span>
                    <span>{t('uploadModal.failed', { count: erroredDriveUploads.length })}</span>
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
                    <div style={{ flex: 1, minWidth: 0 }}>
                      <div style={{ height: '7px', borderRadius: '999px', background: 'var(--color-bg-tertiary)', overflow: 'hidden' }}>
                        <div style={{ width: `${uploadProgressPct}%`, height: '100%', borderRadius: '999px', background: 'var(--color-accent)', transition: 'width 180ms ease' }} />
                      </div>
                    </div>
                    <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', whiteSpace: 'nowrap' }}>
                      {formatBytes(totalDriveUploadProgressBytes)} / {formatBytes(totalDriveUploadBytes)}
                    </div>
                  </div>
                </div>
                <div style={{ overflowY: 'auto', padding: '10px 0' }}>
                  {driveUploads.map((item) => {
                    const progress = item.totalBytes > 0 ? Math.min(100, Math.round((item.uploadedBytes / item.totalBytes) * 100)) : 0;
                    const label = DRIVE_UPLOAD_STATUS_LABELS[item.status];
                    const canPause = item.status === 'creating_session' || item.status === 'uploading' || item.status === 'finalizing';
                    const canResume = item.status === 'paused' || item.status === 'error';
                    const canCancel = item.status !== 'done' && item.status !== 'canceled';
                    return (
                      <div key={item.id} style={{ padding: '10px 20px', borderBottom: '1px solid var(--color-border-subtle)' }}>
                        <div style={{ display: 'flex', alignItems: 'flex-start', gap: '14px' }}>
                          <div style={{ flex: 1, minWidth: 0 }}>
                            <div style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0, flexWrap: 'wrap' }}>
                              <div style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={item.relativePath}>
                                {item.relativePath}
                              </div>
                              <span style={{ display: 'inline-flex', alignItems: 'center', padding: '3px 8px', borderRadius: '999px', background: item.status === 'error' ? 'rgba(239, 68, 68, 0.12)' : item.status === 'done' ? 'rgba(34, 197, 94, 0.12)' : 'var(--color-bg-secondary)', color: item.status === 'error' ? 'var(--color-destructive)' : item.status === 'done' ? '#15803d' : 'var(--color-text-secondary)', fontSize: '11px', fontWeight: 500, flexShrink: 0 }}>
                                {label}
                              </span>
                            </div>
                            <div style={{ marginTop: '8px', height: '8px', borderRadius: '999px', background: 'var(--color-bg-tertiary)', overflow: 'hidden' }}>
                              <div style={{ width: `${progress}%`, height: '100%', borderRadius: '999px', background: item.status === 'error' ? 'var(--color-destructive)' : item.status === 'done' ? '#22c55e' : 'var(--color-accent)', transition: 'width 160ms ease' }} />
                            </div>
                            <div style={{ display: 'flex', justifyContent: 'space-between', gap: '8px', marginTop: '8px', fontSize: '11px', color: 'var(--color-text-tertiary)' }}>
                              <span>{formatBytes(item.uploadedBytes)} / {formatBytes(item.totalBytes)}</span>
                              <span>{item.resumable ? t('uploadModal.itemResumable') : t('uploadModal.itemNonResumable')}</span>
                            </div>
                            {item.error && (
                              <div style={{ marginTop: '8px', fontSize: '11px', color: 'var(--color-destructive)', lineHeight: 1.45 }}>
                                {item.error}
                              </div>
                            )}
                          </div>
                          <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexShrink: 0 }}>
                            {canPause && (
                              <button
                                type="button"
                                onClick={() => pauseDriveUpload(item.id)}
                                title={t('uploadModal.pause')}
                                style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '30px', height: '30px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)' }}
                              >
                                <PauseIcon style={{ width: '14px', height: '14px' }} />
                              </button>
                            )}
                            {canResume && (
                              <button
                                type="button"
                                onClick={() => { void resumeDriveUpload(item.id); }}
                                title={t('uploadModal.resume')}
                                style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '30px', height: '30px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)' }}
                              >
                                <PlayIcon style={{ width: '14px', height: '14px' }} />
                              </button>
                            )}
                            {canCancel && (
                              <button
                                type="button"
                                onClick={() => { void cancelDriveUpload(item.id); }}
                                title={t('uploadModal.cancel')}
                                style={{ display: 'inline-flex', alignItems: 'center', justifyContent: 'center', width: '30px', height: '30px', borderRadius: '8px', border: '1px solid var(--color-destructive)', background: 'transparent', color: 'var(--color-destructive)' }}
                              >
                                <XMarkIcon style={{ width: '14px', height: '14px' }} />
                              </button>
                            )}
                          </div>
                        </div>
                      </div>
                    );
                  })}
                </div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '12px', padding: '14px 20px', borderTop: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)' }}>
                  <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', lineHeight: 1.5 }}>
                    {t('uploadModal.footerHint')}
                  </div>
                  <div style={{ display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}>
                    <button
                      type="button"
                      onClick={() => fileInputRef.current?.click()}
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '7px 12px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', fontSize: '12px', fontWeight: 500 }}
                    >
                      <ArrowUpTrayIcon style={{ width: '14px', height: '14px' }} />
                      {t('uploadModal.addFiles')}
                    </button>
                    <button
                      type="button"
                      onClick={() => folderInputRef.current?.click()}
                      style={{ display: 'inline-flex', alignItems: 'center', gap: '6px', padding: '7px 12px', borderRadius: '8px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', fontSize: '12px', fontWeight: 500 }}
                    >
                      <FolderPlusIcon style={{ width: '14px', height: '14px' }} />
                      {t('uploadModal.addFolder')}
                    </button>
                  </div>
                </div>
              </div>
            </div>
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
