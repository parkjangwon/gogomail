import { useCallback, useEffect, useState } from 'react';
import {
  Attachment,
  downloadAttachment,
  listAttachments,
  saveAttachmentToDrive,
} from '@/lib/api';

interface UseReadingPaneAttachmentsParams {
  messageId: string | undefined;
  hasAttachment: boolean | undefined;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: any;
}

interface UseReadingPaneAttachmentsResult {
  attachments: Attachment[];
  attachmentsLoading: boolean;
  downloadingId: string | null;
  savingToDriveId: string | null;
  driveToast: string;
  handleDownload: (att: Attachment) => Promise<void>;
  handleSaveToDrive: (att: Attachment) => Promise<void>;
}

export function useReadingPaneAttachments({
  messageId,
  hasAttachment,
  t,
}: UseReadingPaneAttachmentsParams): UseReadingPaneAttachmentsResult {
  const [attachments, setAttachments] = useState<Attachment[]>([]);
  const [attachmentsLoading, setAttachmentsLoading] = useState(false);
  const [downloadingId, setDownloadingId] = useState<string | null>(null);
  const [savingToDriveId, setSavingToDriveId] = useState<string | null>(null);
  const [driveToast, setDriveToast] = useState('');

  useEffect(() => {
    if (!hasAttachment || !messageId) {
      setAttachments([]);
      return;
    }

    setAttachmentsLoading(true);
    listAttachments(messageId)
      .then((result) => setAttachments(result))
      .catch(() => setAttachments([]))
      .finally(() => setAttachmentsLoading(false));
  }, [messageId, hasAttachment]);

  const handleDownload = useCallback(async (att: Attachment) => {
    if (!messageId) return;
    setDownloadingId(att.id);
    try {
      await downloadAttachment(messageId, att.id, att.filename);
    } catch {
      // ignore
    } finally {
      setDownloadingId(null);
    }
  }, [messageId]);

  const handleSaveToDrive = useCallback(async (att: Attachment) => {
    if (!messageId) return;
    setSavingToDriveId(att.id);
    try {
      const node = await saveAttachmentToDrive(messageId, att.id, att.filename, att.mime_type);
      setDriveToast(node ? t('misc.readingPane.savedToDrive', { filename: att.filename }) : t('misc.readingPane.driveSaveFailed'));
      setTimeout(() => setDriveToast(''), 3000);
    } catch {
      setDriveToast(t('misc.readingPane.driveSaveFailed'));
      setTimeout(() => setDriveToast(''), 3000);
    } finally {
      setSavingToDriveId(null);
    }
  }, [messageId, t]);

  return {
    attachments,
    attachmentsLoading,
    downloadingId,
    savingToDriveId,
    driveToast,
    handleDownload,
    handleSaveToDrive,
  };
}
