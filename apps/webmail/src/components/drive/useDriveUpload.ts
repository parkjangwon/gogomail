import { useState, useRef, useCallback, useEffect, Dispatch, SetStateAction, MutableRefObject } from 'react';
import { useTranslations } from 'next-intl';
import { uploadDriveFileWithOptions, getWebmailCapabilities, cancelDriveUploadSession } from '@/lib/api';
import {
  DriveUploadStatus, DriveUploadSource, DriveUploadBatch, DriveUploadItem,
  DRIVE_UPLOAD_CONCURRENCY,
  driveUploadNeedsFreshSession, formatDriveUploadError,
} from './driveViewHelpers';

export interface UseDriveUploadParams {
  onUploadComplete: () => Promise<void>;
  t: ReturnType<typeof useTranslations<'drive'>>;
}

export interface UseDriveUploadReturn {
  driveUploadBatch: DriveUploadBatch | null;
  setDriveUploadBatch: Dispatch<SetStateAction<DriveUploadBatch | null>>;
  driveUploads: DriveUploadItem[];
  setDriveUploads: Dispatch<SetStateAction<DriveUploadItem[]>>;
  driveUploadModalOpen: boolean;
  setDriveUploadModalOpen: Dispatch<SetStateAction<boolean>>;
  driveUploadResumable: boolean | null;
  driveUploadModalDismissedRef: MutableRefObject<boolean>;
  enqueueDriveUploads: (
    items: Array<{ file: File; relativePath: string; parentId?: string; resumable: boolean; batchId: string; source: DriveUploadSource }>,
    batch?: DriveUploadBatch | null,
  ) => void;
  updateDriveUpload: (uploadId: string, updater: (item: DriveUploadItem) => DriveUploadItem) => void;
  scheduleDriveUploads: () => void;
  pauseDriveUpload: (uploadId: string) => void;
  resumeDriveUpload: (uploadId: string) => Promise<void>;
  cancelDriveUpload: (uploadId: string) => Promise<void>;
  DRIVE_UPLOAD_STATUS_LABELS: Record<DriveUploadStatus, string>;
}

export function useDriveUpload({ onUploadComplete, t }: UseDriveUploadParams): UseDriveUploadReturn {
  const DRIVE_UPLOAD_STATUS_LABELS: Record<DriveUploadStatus, string> = {
    queued: t('upload.status.queued'),
    creating_session: t('upload.status.creatingSession'),
    uploading: t('upload.status.uploading'),
    paused: t('upload.status.paused'),
    finalizing: t('upload.status.finalizing'),
    done: t('upload.status.done'),
    error: t('upload.status.error'),
    canceled: t('upload.status.canceled'),
  };

  const [driveUploadBatch, setDriveUploadBatch] = useState<DriveUploadBatch | null>(null);
  const [driveUploads, setDriveUploads] = useState<DriveUploadItem[]>([]);
  const [driveUploadModalOpen, setDriveUploadModalOpen] = useState(false);
  const [driveUploadResumable, setDriveUploadResumable] = useState<boolean | null>(null);

  const driveUploadControllersRef = useRef<Map<string, AbortController>>(new Map());
  const driveUploadAbortReasonsRef = useRef<Map<string, 'pause' | 'cancel'>>(new Map());
  const driveUploadActiveIdsRef = useRef<Set<string>>(new Set());
  const driveUploadsRef = useRef<DriveUploadItem[]>([]);
  const driveUploadSchedulerRef = useRef(false);
  const driveUploadModalDismissedRef = useRef(false);

  // Keep ref in sync with state
  useEffect(() => {
    driveUploadsRef.current = driveUploads;
  }, [driveUploads]);

  // Fetch resumable capability once on mount
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

  // Abort all controllers on unmount
  useEffect(() => () => {
    for (const controller of driveUploadControllersRef.current.values()) {
      controller.abort();
    }
    driveUploadControllersRef.current.clear();
    driveUploadAbortReasonsRef.current.clear();
  }, []);

  const updateDriveUpload = useCallback((uploadId: string, updater: (item: DriveUploadItem) => DriveUploadItem) => {
    setDriveUploads((prev) => prev.map((item) => (item.id === uploadId ? updater(item) : item)));
  }, []);

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
      await onUploadComplete();
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
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [onUploadComplete, updateDriveUpload, t]);

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

  // Trigger scheduler whenever uploads change
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

  return {
    driveUploadBatch,
    setDriveUploadBatch,
    driveUploads,
    setDriveUploads,
    driveUploadModalOpen,
    setDriveUploadModalOpen,
    driveUploadResumable,
    driveUploadModalDismissedRef,
    enqueueDriveUploads,
    updateDriveUpload,
    scheduleDriveUploads,
    pauseDriveUpload,
    resumeDriveUpload,
    cancelDriveUpload,
    DRIVE_UPLOAD_STATUS_LABELS,
  };
}
