import { useState, useCallback, Dispatch, SetStateAction } from 'react';
import { listDriveNodes } from '@/lib/api';
import { BreadcrumbItem, SidebarFolderItem } from '@/lib/drive/driveUtils';

export interface UseDriveSidebarParams {
  breadcrumb: BreadcrumbItem[];
}

export interface UseDriveSidebarReturn {
  sidebarFolderChildren: Record<string, SidebarFolderItem[]>;
  setSidebarFolderChildren: Dispatch<SetStateAction<Record<string, SidebarFolderItem[]>>>;
  sidebarExpandedFolders: Set<string>;
  sidebarLoadedFolders: Set<string>;
  setSidebarLoadedFolders: Dispatch<SetStateAction<Set<string>>>;
  sidebarLoadingFolders: Record<string, boolean>;
  sidebarLoadKey: (folderId: string) => string;
  loadSidebarFolders: (parentId: string) => Promise<void>;
  reloadSidebarCurrentPath: () => void;
  toggleSidebarFolder: (folderId: string) => void;
}

export function useDriveSidebar(_params: UseDriveSidebarParams): UseDriveSidebarReturn {
  const [sidebarFolderChildren, setSidebarFolderChildren] = useState<Record<string, SidebarFolderItem[]>>({});
  const [sidebarExpandedFolders, setSidebarExpandedFolders] = useState<Set<string>>(new Set(['']));
  const [sidebarLoadedFolders, setSidebarLoadedFolders] = useState<Set<string>>(new Set());
  const [sidebarLoadingFolders, setSidebarLoadingFolders] = useState<Record<string, boolean>>({});

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

  const toggleSidebarFolder = useCallback((folderId: string) => {
    setSidebarExpandedFolders((prev) => {
      const next = new Set(prev);
      if (next.has(folderId)) next.delete(folderId);
      else next.add(folderId);
      return next;
    });
    void loadSidebarFolders(folderId);
  }, [loadSidebarFolders]);

  return {
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
  };
}
