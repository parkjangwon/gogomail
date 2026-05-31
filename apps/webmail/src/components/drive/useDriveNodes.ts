import { useState, useEffect, useCallback, type Dispatch, type SetStateAction } from 'react';
import { listDriveNodes, listTrashedDriveNodes } from '@/lib/api';
import type { DriveNode, DriveUsage } from '@/lib/api';
import type { BreadcrumbItem } from '@/lib/drive/driveUtils';
import { refreshDriveUsage } from './driveUsageRefresh';
import { type DriveSort, loadDriveSortSetting, sortDriveNodes } from './driveViewHelpers';

export interface UseDriveNodesParams {
  breadcrumb: BreadcrumbItem[];
  activeSection: 'drive' | 'trash';
}

export interface UseDriveNodesReturn {
  nodes: DriveNode[];
  setNodes: Dispatch<SetStateAction<DriveNode[]>>;
  trashNodes: DriveNode[];
  setTrashNodes: Dispatch<SetStateAction<DriveNode[]>>;
  usage: DriveUsage | null;
  setUsage: Dispatch<SetStateAction<DriveUsage | null>>;
  loading: boolean;
  setLoading: Dispatch<SetStateAction<boolean>>;
  trashLoading: boolean;
  setTrashLoading: Dispatch<SetStateAction<boolean>>;
  refreshDriveNodes: () => Promise<void>;
  loadTrashNodes: () => Promise<void>;
}

export function useDriveNodes({ breadcrumb, activeSection }: UseDriveNodesParams): UseDriveNodesReturn {
  const [nodes, setNodes] = useState<DriveNode[]>([]);
  const [trashNodes, setTrashNodes] = useState<DriveNode[]>([]);
  const [usage, setUsage] = useState<DriveUsage | null>(null);
  const [loading, setLoading] = useState(true);
  const [trashLoading, setTrashLoading] = useState(false);
  const [driveSort, setDriveSort] = useState<DriveSort>(loadDriveSortSetting);

  const currentParentId = breadcrumb[breadcrumb.length - 1]?.id ?? '';

  const refreshDriveNodes = useCallback(async () => {
    setLoading(true);
    const data = await listDriveNodes(currentParentId || undefined);
    setNodes(sortDriveNodes(data, driveSort));
    setLoading(false);
    refreshDriveUsage(setUsage);
  }, [currentParentId, driveSort]);

  const loadNodes = useCallback(async (parentId: string) => {
    setLoading(true);
    const data = await listDriveNodes(parentId || undefined);
    setNodes(sortDriveNodes(data, driveSort));
    setLoading(false);
  }, [driveSort]);

  const loadTrashNodes = useCallback(async () => {
    setTrashLoading(true);
    const data = await listTrashedDriveNodes();
    setTrashNodes(data.sort((a, b) => a.name.localeCompare(b.name, 'ko')));
    setTrashLoading(false);
  }, []);

  useEffect(() => {
    const refresh = (event?: StorageEvent) => {
      if (event && event.key !== 'webmail_settings') return;
      setDriveSort(loadDriveSortSetting());
    };
    window.addEventListener('storage', refresh);
    return () => window.removeEventListener('storage', refresh);
  }, []);

  useEffect(() => {
    loadNodes(currentParentId);
    refreshDriveUsage(setUsage);
  }, [currentParentId, loadNodes]);

  useEffect(() => {
    if (activeSection === 'trash') loadTrashNodes();
  }, [activeSection, loadTrashNodes]);

  return {
    nodes,
    setNodes,
    trashNodes,
    setTrashNodes,
    usage,
    setUsage,
    loading,
    setLoading,
    trashLoading,
    setTrashLoading,
    refreshDriveNodes,
    loadTrashNodes,
  };
}
