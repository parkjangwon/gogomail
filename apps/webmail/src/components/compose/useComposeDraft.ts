import { useState, useRef, useCallback, useEffect } from 'react';
import { saveDraft, updateDraft, deleteDraft } from '@/lib/api';
import type { UIComposeIntent, MessageDetail, DraftData } from '@/lib/api';
import { parseAddrs } from '@/lib/compose/composeUtils';
import { backendComposeIntent } from '@/lib/compose/composeUtils';

interface UseComposeDraftParams {
  to: string;
  cc: string;
  bcc: string;
  subject: string;
  intent: UIComposeIntent;
  sourceMessage?: MessageDetail;
  fromAddress: string;
  scheduledAt: string;
  trackOpens: boolean;
  readyAttachmentIds: () => string[];
  draftMessage?: MessageDetail | null;
}

interface UseComposeDraftReturn {
  draftIdRef: React.MutableRefObject<string>;
  saveStatus: 'idle' | 'saving' | 'saved';
  savedAt: string;
  setSaveStatus: React.Dispatch<React.SetStateAction<'idle' | 'saving' | 'saved'>>;
  setSavedAt: React.Dispatch<React.SetStateAction<string>>;
  clearSentDraft: (deleteRemote?: boolean) => Promise<void>;
  triggerAutoSave: (
    toVal: string,
    ccVal: string,
    bccVal: string,
    subjectVal: string,
    bodyText: string,
    bodyHtml: string
  ) => void;
  buildDraftData: (
    toVal: string,
    ccVal: string,
    bccVal: string,
    subjectVal: string,
    bodyText: string,
    bodyHtml: string
  ) => DraftData;
}

export function useComposeDraft({
  intent,
  sourceMessage,
  fromAddress,
  scheduledAt,
  trackOpens,
  readyAttachmentIds,
  draftMessage,
}: UseComposeDraftParams): UseComposeDraftReturn {
  const draftIdRef = useRef<string>(draftMessage?.id ?? '');
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [savedAt, setSavedAt] = useState('');

  // Cleanup timer on unmount
  useEffect(() => {
    return () => {
      if (saveTimerRef.current) clearTimeout(saveTimerRef.current);
    };
  }, []);

  const clearSentDraft = useCallback(async (deleteRemote = true) => {
    const draftId = draftIdRef.current;
    if (!draftId) return;
    draftIdRef.current = '';
    if (!deleteRemote) return;
    try {
      await deleteDraft(draftId);
    } catch {
      // Sending succeeded; draft cleanup is best-effort and must not fail the send.
    }
  }, []);

  const buildDraftData = useCallback(
    (toVal: string, ccVal: string, bccVal: string, subjectVal: string, bodyText: string, bodyHtml: string): DraftData => {
      const attachmentIds = readyAttachmentIds();
      return {
        intent: backendComposeIntent(intent),
        ...(intent !== 'new' && sourceMessage && { source_message_id: sourceMessage.id }),
        to: parseAddrs(toVal),
        ...(ccVal.trim() && { cc: parseAddrs(ccVal) }),
        ...(bccVal.trim() && { bcc: parseAddrs(bccVal) }),
        subject: subjectVal,
        text_body: bodyText,
        html_body: bodyHtml,
        ...(fromAddress && { from: fromAddress }),
        ...(attachmentIds.length > 0 && { attachment_ids: attachmentIds }),
        ...(trackOpens && { track_opens: true }),
        ...(scheduledAt && { scheduled_at: new Date(scheduledAt).toISOString() }),
      };
    },
    [intent, sourceMessage, fromAddress, readyAttachmentIds, trackOpens, scheduledAt]
  );

  const triggerAutoSave = useCallback(
    (toVal: string, ccVal: string, bccVal: string, subjectVal: string, bodyText: string, bodyHtml: string) => {
      if (saveTimerRef.current) clearTimeout(saveTimerRef.current);
      saveTimerRef.current = setTimeout(async () => {
        if (!toVal.trim() && !subjectVal.trim() && !bodyText.trim()) return;
        setSaveStatus('saving');
        try {
          const data = buildDraftData(toVal, ccVal, bccVal, subjectVal, bodyText, bodyHtml);
          if (draftIdRef.current) {
            await updateDraft(draftIdRef.current, data);
          } else {
            const res = await saveDraft(data);
            draftIdRef.current = res.draft.id;
          }
          const now = new Date();
          setSavedAt(
            `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`
          );
          setSaveStatus('saved');
        } catch {
          setSaveStatus('idle');
        }
      }, 3000);
    },
    [buildDraftData]
  );

  return {
    draftIdRef,
    saveStatus,
    savedAt,
    setSaveStatus,
    setSavedAt,
    clearSentDraft,
    triggerAutoSave,
    buildDraftData,
  };
}
