import { useState, useRef, useCallback, useEffect } from 'react';
import { useTranslations } from 'next-intl';
import { sendMessage, sendDraft } from '@/lib/api';
import type { SendMessageRequest, SendMessageResult } from '@/lib/api';
import { useOptionalNotifications } from '@/lib/notifications/store';

interface UseComposeSendParams {
  draftIdRef: React.MutableRefObject<string>;
  clearSentDraft: (deleteRemote?: boolean) => Promise<void>;
  onAfterSend?: () => void;
  onClose: () => void;
  onArchiveSource?: () => void;
  recentRecipients: string[];
}

interface UseComposeSendReturn {
  sending: boolean;
  setSending: React.Dispatch<React.SetStateAction<boolean>>;
  error: string;
  setError: React.Dispatch<React.SetStateAction<string>>;
  sent: boolean;
  setSent: React.Dispatch<React.SetStateAction<boolean>>;
  sendResult: SendMessageResult | null;
  setSendResult: React.Dispatch<React.SetStateAction<SendMessageResult | null>>;
  sendCountdown: number | null;
  setSendCountdown: React.Dispatch<React.SetStateAction<number | null>>;
  scheduledAt: string;
  setScheduledAt: React.Dispatch<React.SetStateAction<string>>;
  showSchedule: boolean;
  setShowSchedule: React.Dispatch<React.SetStateAction<boolean>>;
  pendingMsgRef: React.MutableRefObject<SendMessageRequest | null>;
  pendingDraftSendRef: React.MutableRefObject<boolean>;
  sendInProgressRef: React.MutableRefObject<boolean>;
  sendExecutionRef: React.MutableRefObject<boolean>;
  sendAndArchiveRef: React.MutableRefObject<boolean>;
  rememberSendResult: (result: SendMessageResult | undefined) => void;
  handleSuccessfulSend: (
    msg: SendMessageRequest,
    result: SendMessageResult,
    useDraftSend: boolean
  ) => Promise<void>;
  handleSendFailure: (err: unknown, clearCountdown?: boolean) => void;
  handleSendPreparationFailure: (err: unknown) => void;
  shouldSendSavedDraft: () => boolean;
  sendPreparedMessage: (
    msg: SendMessageRequest,
    useDraftSend: boolean
  ) => Promise<{ message: SendMessageResult }>;
  persistSuccessfulSendLocalState: (msg: SendMessageRequest) => void;
}

export function useComposeSend({
  draftIdRef,
  clearSentDraft,
  onAfterSend,
  onClose,
  onArchiveSource,
  recentRecipients,
}: UseComposeSendParams): UseComposeSendReturn {
  const t = useTranslations('composeFull');
  const tNotif = useTranslations('notifications');
  const notifications = useOptionalNotifications();

  const [sending, setSending] = useState(false);
  const [error, setError] = useState('');
  const [sent, setSent] = useState(false);
  const [sendResult, setSendResult] = useState<SendMessageResult | null>(null);
  const [sendCountdown, setSendCountdown] = useState<number | null>(null);
  const [scheduledAt, setScheduledAt] = useState('');
  const [showSchedule, setShowSchedule] = useState(false);

  const pendingMsgRef = useRef<SendMessageRequest | null>(null);
  const pendingDraftSendRef = useRef(false);
  const sendInProgressRef = useRef(false);
  const sendExecutionRef = useRef(false);
  const sendAndArchiveRef = useRef(false);

  const rememberSendResult = useCallback((result: SendMessageResult | undefined) => {
    if (result) setSendResult(result);
  }, []);

  const persistSuccessfulSendLocalState = useCallback(
    (msg: SendMessageRequest) => {
      try {
        const newAddrs = [...(msg.to ?? []), ...(msg.cc ?? []), ...(msg.bcc ?? [])]
          .map((a) => (a.name ? `${a.name} <${a.address}>` : a.address))
          .filter(Boolean);
        const merged = [...new Set([...newAddrs, ...recentRecipients])].slice(0, 30);
        localStorage.setItem('webmail_recent_recipients', JSON.stringify(merged));
        const followUpDays = Number(
          (
            JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as Record<string, unknown>
          ).followUpDays ?? 0
        );
        if (followUpDays > 0 && msg.to?.length) {
          const remindAt = new Date(Date.now() + followUpDays * 86400000).toISOString();
          const followups: Record<string, unknown>[] = JSON.parse(
            localStorage.getItem('webmail_followups') ?? '[]'
          );
          followups.push({
            remindAt,
            subject: msg.subject ?? '',
            to: msg.to[0].address,
            createdAt: new Date().toISOString(),
          });
          localStorage.setItem('webmail_followups', JSON.stringify(followups));
        }
      } catch {
        /* keep send success independent from local storage */
      }
    },
    [recentRecipients]
  );

  const handleSuccessfulSend = useCallback(
    async (msg: SendMessageRequest, result: SendMessageResult, useDraftSend: boolean) => {
      rememberSendResult(result);
      persistSuccessfulSendLocalState(msg);
      await clearSentDraft(!useDraftSend);
      pendingDraftSendRef.current = false;
      sendInProgressRef.current = true;
      setSent(true);
      if (notifications) {
        const firstRecipient = msg.to?.[0];
        const recipientLabel = firstRecipient
          ? firstRecipient.name
            ? `${firstRecipient.name} <${firstRecipient.address}>`
            : firstRecipient.address
          : '';
        notifications.push({
          category: 'mail_sent',
          severity: 'success',
          title: tNotif('mailSent'),
          body: msg.subject
            ? `${msg.subject}${recipientLabel ? ` — ${recipientLabel}` : ''}`
            : recipientLabel || undefined,
          actionUrl: result?.id ? `/mail/${result.id}` : undefined,
          metadata: { messageId: result?.id },
        });
      }
      onAfterSend?.();
      setTimeout(() => {
        if (sendAndArchiveRef.current) {
          onArchiveSource?.();
          sendAndArchiveRef.current = false;
        }
        onClose();
      }, 1500);
    },
    [
      clearSentDraft,
      onAfterSend,
      onArchiveSource,
      onClose,
      persistSuccessfulSendLocalState,
      rememberSendResult,
      notifications,
      tNotif,
    ]
  );

  const handleSendFailure = useCallback(
    (err: unknown, clearCountdown = false) => {
      const message = err instanceof Error ? err.message : t('errSendFailed');
      setError(t('draftPreserved', { message }));
      pendingDraftSendRef.current = false;
      sendExecutionRef.current = false;
      sendInProgressRef.current = false;
      if (clearCountdown) setSendCountdown(null);
      if (notifications) {
        notifications.push({
          category: 'mail_send_failed',
          severity: 'error',
          title: tNotif('mailSendFailed'),
          body: message,
        });
      }
    },
    [t, notifications, tNotif]
  );

  const handleSendPreparationFailure = useCallback(
    (err: unknown) => {
      const message = err instanceof Error ? err.message : t('draftPrepFailed');
      pendingMsgRef.current = null;
      pendingDraftSendRef.current = false;
      sendExecutionRef.current = false;
      sendInProgressRef.current = false;
      setError(t('sendNotStarted', { message }));
    },
    [t]
  );

  const shouldSendSavedDraft = useCallback(
    () => pendingDraftSendRef.current && !!draftIdRef.current,
    [draftIdRef]
  );

  const sendPreparedMessage = useCallback(
    (msg: SendMessageRequest, useDraftSend: boolean) => {
      const draftId = draftIdRef.current;
      return useDraftSend && draftId ? sendDraft(draftId) : sendMessage(msg);
    },
    [draftIdRef]
  );

  // Send countdown effect
  useEffect(() => {
    if (sendCountdown === null) return;
    if (sendCountdown === 0) {
      if (sendExecutionRef.current) return;
      const msg = pendingMsgRef.current;
      if (!msg) {
        sendInProgressRef.current = false;
        return;
      }
      sendExecutionRef.current = true;
      setSendCountdown(null);
      const useDraftSend = shouldSendSavedDraft();
      setSending(true);
      sendPreparedMessage(msg, useDraftSend)
        .then(async (res) => {
          await handleSuccessfulSend(msg, res.message, useDraftSend);
        })
        .catch((err: unknown) => {
          handleSendFailure(err, true);
        })
        .finally(() => setSending(false));
      return;
    }
    const timer = setTimeout(() => setSendCountdown((n) => (n !== null ? n - 1 : null)), 1000);
    return () => clearTimeout(timer);
  }, [sendCountdown, handleSuccessfulSend, handleSendFailure, sendPreparedMessage, shouldSendSavedDraft]);

  return {
    sending,
    setSending,
    error,
    setError,
    sent,
    setSent,
    sendResult,
    setSendResult,
    sendCountdown,
    setSendCountdown,
    scheduledAt,
    setScheduledAt,
    showSchedule,
    setShowSchedule,
    pendingMsgRef,
    pendingDraftSendRef,
    sendInProgressRef,
    sendExecutionRef,
    sendAndArchiveRef,
    rememberSendResult,
    handleSuccessfulSend,
    handleSendFailure,
    handleSendPreparationFailure,
    shouldSendSavedDraft,
    sendPreparedMessage,
    persistSuccessfulSendLocalState,
  };
}
