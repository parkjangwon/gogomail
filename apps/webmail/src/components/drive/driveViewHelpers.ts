import { DriveNode } from '@/lib/api';
import { DRIVE_NODE_DRAG_MIME, DRIVE_NODE_DRAG_TEXT, DroppedFileEntry, FileSystemEntryLike, DirectoryReaderLike } from '@/lib/drive/driveUtils';

export type DriveUploadStatus = 'queued' | 'creating_session' | 'uploading' | 'paused' | 'finalizing' | 'done' | 'error' | 'canceled';
export type DriveUploadSource = 'picker' | 'folder' | 'drop';
export type DriveSort = 'typeName' | 'name' | 'updated' | 'size';

export const DRIVE_UPLOAD_CONCURRENCY = 3;

export type DriveUploadBatch = {
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

export type DriveUploadItem = {
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

export type FileSystemHandleLike =
  | { kind: 'file'; name: string; getFile?: () => Promise<File> }
  | { kind: 'directory'; name: string; entries?: () => AsyncIterable<[string, FileSystemHandleLike]> | Iterable<[string, FileSystemHandleLike]> };

export function loadDriveSortSetting(): DriveSort {
  try {
    const settings = JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as Record<string, unknown>;
    const sort = settings.driveSort;
    return sort === 'name' || sort === 'updated' || sort === 'size' ? sort : 'typeName';
  } catch {
    return 'typeName';
  }
}

export function sortDriveNodes(nodes: DriveNode[], sort: DriveSort): DriveNode[] {
  return [...nodes].sort((a, b) => {
    if (sort === 'typeName' && a.node_type !== b.node_type) return a.node_type === 'folder' ? -1 : 1;
    if (sort === 'updated') return Date.parse(b.updated_at) - Date.parse(a.updated_at) || a.name.localeCompare(b.name, 'ko');
    if (sort === 'size') return b.size - a.size || a.name.localeCompare(b.name, 'ko');
    return a.name.localeCompare(b.name, 'ko', { sensitivity: 'base' });
  });
}


export function getDriveNodeDragPayload(dataTransfer: DataTransfer): string | null {
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

export function parseDriveNodeIds(payload: string | null): string[] | null {
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

export function isDriveNodeDrag(dataTransfer: DataTransfer): boolean {
  return (
    Array.from(dataTransfer.types).includes(DRIVE_NODE_DRAG_MIME) ||
    Array.from(dataTransfer.types).includes(DRIVE_NODE_DRAG_TEXT)
  );
}

export function createDriveDragGhost(count: number, names: string[], tFn: (key: string, values?: Record<string, string | number | Date>) => string): HTMLElement {
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

export function normalizeDroppedPath(path: string): string {
  return path.replace(/[\\/]+/g, '/').replace(/^\/+|\/+$/g, '');
}

export function getDriveUploadSourceLabel(source: DriveUploadSource, tFn: (key: string) => string): string {
  switch (source) {
    case 'picker':
      return tFn('sourceFilePicker');
    case 'folder':
      return tFn('sourceFolderPicker');
    case 'drop':
      return tFn('sourceDrop');
  }
}

export function driveUploadNeedsFreshSession(message: string): boolean {
  const lower = message.toLowerCase();
  return lower.includes('storage store') && lower.includes('is required');
}

export function formatDriveUploadError(error: unknown, tFn: (key: string) => string): string {
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

export async function readAllEntries(reader: DirectoryReaderLike): Promise<FileSystemEntryLike[]> {
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

export function readFileFromEntry(entry: FileSystemEntryLike): Promise<File> {
  return new Promise((resolve, reject) => {
    entry.file((file) => resolve(file), (err) => reject(err));
  });
}

export async function collectDroppedFilesFromEntry(entry: FileSystemEntryLike, basePath: string, out: DroppedFileEntry[]) {
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

export async function collectDroppedFilesFromHandle(
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

export async function collectDroppedFiles(dataTransfer: DataTransfer): Promise<DroppedFileEntry[]> {
  const entries: DroppedFileEntry[] = [];
  const seen = new Set<string>();
  const pushEntry = (file: File, relativePath: string) => {
    const normalized = normalizeDroppedPath(relativePath || file.name);
    const key = `${normalized} ${file.size} ${file.lastModified}`;
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
