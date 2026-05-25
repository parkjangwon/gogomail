'use client';

import { useState, useRef, useCallback } from 'react';
import { uploadAttachment, attachDriveFileToEmail, listDriveNodes } from '@/lib/api';
import type { DriveNode } from '@/lib/api';
import { stableId } from '@/lib/stableId';
import type { UploadedAttachment } from './ComposeAttachmentPanel';

interface UseComposeAttachmentsOptions {
  t: (k: string) => string;
  draftIdRef: React.MutableRefObject<string>;
  initialDriveCrumbName: string;
}

interface ComposeAttachmentsState {
  uploadedAttachments: UploadedAttachment[];
  setUploadedAttachments: React.Dispatch<React.SetStateAction<UploadedAttachment[]>>;
  dragOver: boolean;
  setDragOver: React.Dispatch<React.SetStateAction<boolean>>;
  dragCounterRef: React.MutableRefObject<number>;
  showDrivePicker: boolean;
  setShowDrivePicker: React.Dispatch<React.SetStateAction<boolean>>;
  drivePickerNodes: DriveNode[];
  drivePickerLoading: boolean;
  drivePickerCrumbs: Array<{ id: string | undefined; name: string }>;
  attachingDriveId: string | null;
  handleFileSelect: (files: FileList) => Promise<void>;
  retryAttachmentUpload: (id: string) => Promise<void>;
  openDrivePicker: (parentId?: string, crumbs?: Array<{ id: string | undefined; name: string }>) => Promise<void>;
  handleAttachFromDrive: (node: DriveNode) => Promise<void>;
  readyAttachmentIds: () => string[];
}

export function useComposeAttachments({ t, draftIdRef, initialDriveCrumbName }: UseComposeAttachmentsOptions): ComposeAttachmentsState {
  const [uploadedAttachments, setUploadedAttachments] = useState<UploadedAttachment[]>([]);
  const [dragOver, setDragOver] = useState(false);
  const dragCounterRef = useRef(0);
  const [showDrivePicker, setShowDrivePicker] = useState(false);
  const [drivePickerNodes, setDrivePickerNodes] = useState<DriveNode[]>([]);
  const [drivePickerLoading, setDrivePickerLoading] = useState(false);
  const [drivePickerCrumbs, setDrivePickerCrumbs] = useState<Array<{ id: string | undefined; name: string }>>(
    [{ id: undefined, name: initialDriveCrumbName }],
  );
  const [attachingDriveId, setAttachingDriveId] = useState<string | null>(null);

  const readyAttachmentIds = useCallback(() =>
    uploadedAttachments
      .filter((a) => !a.uploading && !a.error)
      .map((a) => a.id),
  [uploadedAttachments]);

  const handleFileSelect = useCallback(async (files: FileList) => {
    const newFiles = Array.from(files);
    for (const file of newFiles) {
      const tempId = stableId('tmp');
      setUploadedAttachments((prev) => [...prev, { id: tempId, filename: file.name, size: file.size, uploading: true, file }]);
      try {
        const att = await uploadAttachment(file, draftIdRef.current || undefined);
        setUploadedAttachments((prev) => prev.map((a) => a.id === tempId ? { id: att.id, filename: att.filename, size: att.size } : a));
      } catch {
        setUploadedAttachments((prev) => prev.map((a) => a.id === tempId ? { ...a, uploading: false, error: t('uploadFailed') } : a));
      }
    }
  }, [t, draftIdRef]);

  const retryAttachmentUpload = useCallback(async (attachmentId: string) => {
    const failedAttachment = uploadedAttachments.find((attachment) => attachment.id === attachmentId && attachment.error && attachment.file);
    if (!failedAttachment?.file) return;

    setUploadedAttachments((prev) => prev.map((attachment) =>
      attachment.id === attachmentId ? { ...attachment, uploading: true, error: undefined } : attachment,
    ));

    try {
      const att = await uploadAttachment(failedAttachment.file, draftIdRef.current || undefined);
      setUploadedAttachments((prev) => prev.map((attachment) =>
        attachment.id === attachmentId ? { id: att.id, filename: att.filename, size: att.size } : attachment,
      ));
    } catch {
      setUploadedAttachments((prev) => prev.map((attachment) =>
        attachment.id === attachmentId ? { ...attachment, uploading: false, error: t('uploadFailed') } : attachment,
      ));
    }
  }, [uploadedAttachments, t, draftIdRef]);

  const openDrivePicker = useCallback(async (parentId?: string, crumbs?: Array<{ id: string | undefined; name: string }>) => {
    setShowDrivePicker(true);
    setDrivePickerLoading(true);
    if (crumbs) setDrivePickerCrumbs(crumbs);
    const nodes = await listDriveNodes(parentId);
    setDrivePickerNodes(nodes ?? []);
    setDrivePickerLoading(false);
  }, []);

  const handleAttachFromDrive = useCallback(async (node: DriveNode) => {
    if (node.node_type === 'folder') {
      const newCrumbs = [...drivePickerCrumbs, { id: node.id, name: node.name }];
      await openDrivePicker(node.id, newCrumbs);
      return;
    }
    setAttachingDriveId(node.id);
    const att = await attachDriveFileToEmail(node.id, node.name, node.mime_type ?? '', draftIdRef.current || undefined);
    if (att) {
      setUploadedAttachments((prev) => [...prev, { id: att.id, filename: att.filename, size: att.size }]);
      setShowDrivePicker(false);
    }
    setAttachingDriveId(null);
  }, [drivePickerCrumbs, openDrivePicker, draftIdRef]);

  return {
    uploadedAttachments,
    setUploadedAttachments,
    dragOver,
    setDragOver,
    dragCounterRef,
    showDrivePicker,
    setShowDrivePicker,
    drivePickerNodes,
    drivePickerLoading,
    drivePickerCrumbs,
    attachingDriveId,
    handleFileSelect,
    retryAttachmentUpload,
    openDrivePicker,
    handleAttachFromDrive,
    readyAttachmentIds,
  };
}
