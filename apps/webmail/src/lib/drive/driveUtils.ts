export function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1024 * 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  return `${(bytes / 1024 / 1024 / 1024).toFixed(2)} GB`;
}

export function formatDate(iso: string): string {
  return new Intl.DateTimeFormat('ko-KR', { year: 'numeric', month: 'short', day: 'numeric' }).format(new Date(iso));
}

export interface BreadcrumbItem {
  id: string;
  name: string;
}

export interface SidebarFolderItem {
  id: string;
  name: string;
}

export const DRIVE_NODE_DRAG_MIME = 'application/x-gogomail-drive-node';
export const DRIVE_NODE_DRAG_TEXT = 'application/x-gogomail-drive-node-id';

export interface DroppedFileEntry {
  file: File;
  relativePath: string;
}

export type FileSystemEntryLike = {
  isFile: boolean;
  isDirectory: boolean;
  name: string;
  fullPath?: string;
  file: (cb: (file: File) => void, errCb?: (err: DOMException) => void) => void;
  createReader: () => {
    readEntries: (
      cb: (entries: FileSystemEntryLike[]) => void,
      errCb?: (err: DOMException) => void,
    ) => void;
  };
};

export type DirectoryReaderLike = {
  readEntries: (
    cb: (entries: FileSystemEntryLike[]) => void,
    errCb?: (err: DOMException) => void,
  ) => void;
};
