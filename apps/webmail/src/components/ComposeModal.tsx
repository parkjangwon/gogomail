'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import { useTranslations } from 'next-intl';
import { EditorContent } from '@tiptap/react';
import { saveDraft, updateDraft } from '@/lib/api';
import type { UIComposeIntent, MessageDetail, SendMessageRequest } from '@/lib/api';
import { composeCloseSaveButtonAriaLabel } from '@/lib/composeCloseSaveButtonAriaLabel';
import { composeCloseSaveButtonLabel } from '@/lib/composeCloseSaveButtonLabel';
import { composeCloseSavePrompt } from '@/lib/composeCloseSavePrompt';
import { composeSendButtonLabel } from '@/lib/composeSendButtonLabel';
import { toDateTimeLocalValue } from '@/lib/dateTimeLocal';
import { formatSendResultLabel } from '@/lib/sendResultLabel';
import { parseAddrs, backendComposeIntent } from '@/lib/compose/composeUtils';
import { invalidRecipientAddresses, parseToPickerItems, pickerItemsToString } from '@/lib/mail-address';
import { OrgPickerModal } from './OrgPickerModal';
import { ComposeModalActions } from './ComposeModalActions';
import { ComposeModalFooter } from './ComposeModalFooter';
import { ComposeSlashCommandMenu } from './compose/ComposeSlashCommandMenu';
import { ComposeAttachmentPanel } from './compose/ComposeAttachmentPanel';
import { ComposeSigEditorPanel } from './compose/ComposeSigEditorPanel';
import { ComposeModalHeader } from './compose/ComposeModalHeader';
import { ComposeClosePanel } from './compose/ComposeClosePanel';
import { ComposeImageResizeToolbar } from './compose/ComposeImageResizeToolbar';
import { ComposeFormFields } from './compose/ComposeFormFields';
import { useComposeWindow } from './compose/useComposeWindow';
import { useComposeAttachments } from './compose/useComposeAttachments';
import { useComposeTemplates } from './compose/useComposeTemplates';
import { useComposeDraft } from './compose/useComposeDraft';
import { useComposeSlash } from './compose/useComposeSlash';
import { useComposeSend } from './compose/useComposeSend';
import { useComposeRecipients } from './compose/useComposeRecipients';
import { useComposeUI } from './compose/useComposeUI';
import { useComposeEditor } from './compose/useComposeEditor';
import {
  PaperClipIcon,
} from '@heroicons/react/24/outline';

interface ComposeModalProps {
  onClose: () => void;
  intent?: UIComposeIntent;
  sourceMessage?: MessageDetail;
  draftMessage?: MessageDetail;
  userEmail?: string;
  initialTo?: string;
  initialSubject?: string;
  initialBody?: string;
  focusSubjectOnOpen?: boolean;
  isMobile?: boolean;
  windowOffset?: number;
  onArchiveSource?: () => void;
  /** Called right after a mail is successfully sent — use to refresh the inbox. */
  onAfterSend?: () => void;
}

export function ComposeModal({ onClose, intent = 'new', sourceMessage, draftMessage, userEmail, initialTo, initialSubject, initialBody, focusSubjectOnOpen = false, isMobile, windowOffset = 0, onArchiveSource, onAfterSend }: ComposeModalProps) {
  const t = useTranslations('composeFull');
  const tMisc = useTranslations('misc.compose');

  const replySubject = sourceMessage
    ? intent === 'forward'
      ? `Fwd: ${sourceMessage.subject}`
      : `Re: ${sourceMessage.subject}`
    : '';

  // ---- Recipients hook ----
  const {
    to, setTo,
    cc, setCc,
    bcc, setBcc,
    showCc, setShowCc,
    showBcc, setShowBcc,
    fromAddress, setFromAddress,
    availableAddresses,
    recentRecipients,
    toRef, ccRef, bccRef,
  } = useComposeRecipients({ draftMessage, initialTo, intent, sourceMessage, userEmail });

  const [subject, setSubject] = useState(draftMessage ? (draftMessage.subject ?? '') : (initialSubject ?? replySubject));

  // ---- UI hook ----
  const {
    confirmClose, setConfirmClose,
    closeSaveInProgress, setCloseSaveInProgress,
    showSigEditor, setShowSigEditor,
    signature, setSignature,
    showEmojiPicker, setShowEmojiPicker,
    showOrgPicker, setShowOrgPicker,
    showSendDropdown, setShowSendDropdown,
    imageResizeToolbar, setImageResizeToolbar,
    trackOpens, setTrackOpens,
  } = useComposeUI();

  const fileInputRef = useRef<HTMLInputElement>(null);
  const sendDropdownRef = useRef<HTMLDivElement>(null);

  // Lazy ref for readyAttachmentIds — allows useComposeDraft to be called before useComposeAttachments
  // while still getting the live function at call time.
  const readyAttachmentIdsRef = useRef<() => string[]>(() => []);

  // ---- Draft hook (must come before useComposeAttachments to provide draftIdRef) ----
  const draftHook = useComposeDraft({
    to,
    cc,
    bcc,
    subject,
    intent,
    sourceMessage,
    fromAddress: userEmail ?? '',   // will be updated via setFromAddress; draft hook reads lazily
    scheduledAt: '',                 // updated below via sendHook; reads lazily in callbacks
    trackOpens,
    readyAttachmentIds: () => readyAttachmentIdsRef.current(),
    draftMessage,
  });
  const { draftIdRef, saveStatus, savedAt, setSaveStatus, setSavedAt, clearSentDraft, buildDraftData, triggerAutoSave } = draftHook;

  const {
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
  } = useComposeAttachments({ t, draftIdRef, initialDriveCrumbName: t('drive') });

  // Keep the lazy ref in sync
  readyAttachmentIdsRef.current = readyAttachmentIds;

  const {
    templates,
    templateSaveName,
    setTemplateSaveName,
    showTemplates,
    setShowTemplates,
    showTemplateSave,
    setShowTemplateSave,
    templateMenuRef,
    persistTemplates,
    saveTemplate,
    deleteTemplate,
  } = useComposeTemplates({
    t,
    getEditorHTML: () => editor?.getHTML() ?? '',
    subject,
  });

  const { pos, setPos: _setPos, size, minimized, setMinimized, fullscreen, setFullscreen, dialogRef, startDrag, startResize } = useComposeWindow({ isMobile });

  // ---- Send hook ----
  const sendHook = useComposeSend({
    draftIdRef,
    clearSentDraft,
    onAfterSend,
    onClose,
    onArchiveSource,
    recentRecipients,
  });
  const {
    sending,
    setSending,
    error,
    setError,
    sent,
    sendResult,
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
    handleSuccessfulSend,
    handleSendFailure,
    handleSendPreparationFailure,
    shouldSendSavedDraft,
    sendPreparedMessage,
  } = sendHook;

  // ---- Slash hook ----
  const slashHook = useComposeSlash();
  const {
    slashMenu,
    setSlashMenu,
    slashIndex,
    setSlashIndex,
    slashStartPosRef,
    slashMenuRef,
    slashIndexRef,
    runSlashCommandRef,
    runSlashCommand: runSlashCommandBase,
    filteredCommands,
  } = slashHook;

  const subjectRef = useRef(draftMessage ? (draftMessage.subject ?? '') : replySubject);
  const subjectInputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    if (!focusSubjectOnOpen) return;
    const id = window.setTimeout(() => subjectInputRef.current?.focus(), 0);
    return () => window.clearTimeout(id);
  }, [focusSubjectOnOpen]);

  const { editor, imageInputRef, handleImageFileSelect, runSlashCommand } = useComposeEditor({
    intent,
    sourceMessage,
    draftMessage,
    initialBody,
    signature,
    bodyPlaceholder: t('bodyPlaceholder'),
    bodyAria: t('bodyAria'),
    slashMenuRef,
    setSlashMenu,
    setSlashIndex,
    slashIndexRef,
    slashStartPosRef,
    runSlashCommandRef,
    runSlashCommandBase,
    setImageResizeToolbar,
    toRef,
    ccRef,
    bccRef,
    subjectRef,
    triggerAutoSave,
    draftIdRef,
    setUploadedAttachments,
    sendCountdown,
    uploadedAttachments,
    readyAttachmentIds,
    setSendCountdown,
    pendingMsgRef,
    pendingDraftSendRef,
    setError,
    errAttachmentChanged: t('errAttachmentChanged'),
  });

  const markDraftSaved = useCallback(() => {
    const now = new Date();
    setSavedAt(`${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`);
    setSaveStatus('saved');
  }, [setSavedAt, setSaveStatus]);

  const handleManualSave = useCallback(async () => {
    const bodyText = editor?.getText() ?? '';
    if (!to.trim() && !subject.trim() && !bodyText.trim()) return;
    setSaveStatus('saving');
    try {
      const data = buildDraftData(to, cc, bcc, subject, bodyText, editor?.getHTML() ?? '');
      if (draftIdRef.current) await updateDraft(draftIdRef.current, data);
      else { const r = await saveDraft(data); draftIdRef.current = r.draft.id; }
      markDraftSaved();
    } catch { setSaveStatus('idle'); }
  }, [to, cc, bcc, subject, editor, buildDraftData, markDraftSaved, draftIdRef, setSaveStatus]);

  const saveDraftAndClose = useCallback(async () => {
    if (closeSaveInProgress) return;
    setCloseSaveInProgress(true);
    const bodyText = editor?.getText() ?? '';
    try {
      if (to.trim() || subject.trim() || bodyText.trim()) {
        const data = buildDraftData(to, cc, bcc, subject, bodyText, editor?.getHTML() ?? '');
        try {
          if (draftIdRef.current) await updateDraft(draftIdRef.current, data);
          else { const r = await saveDraft(data); draftIdRef.current = r.draft.id; }
        } catch { /* close-save remains best-effort */ }
      }
    } finally {
      setCloseSaveInProgress(false);
      onClose();
    }
  }, [to, cc, bcc, subject, editor, buildDraftData, closeSaveInProgress, onClose, draftIdRef]);

  const discardDraftAndClose = useCallback(() => {
    onClose();
  }, [onClose]);

  const cancelPendingSend = useCallback(() => {
    setSendCountdown(null);
    pendingMsgRef.current = null;
    pendingDraftSendRef.current = false;
    sendExecutionRef.current = false;
    sendInProgressRef.current = false;
  }, [setSendCountdown]);

  // beforeunload guard
  useEffect(() => {
    function handleBeforeUnload(e: BeforeUnloadEvent) {
      if (saveStatus === 'saving') {
        e.preventDefault();
      }
    }
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [saveStatus]);

  useEffect(() => {
    if (!showSendDropdown) return;
    function handleOutsideClick(e: MouseEvent) {
      if (sendDropdownRef.current && !sendDropdownRef.current.contains(e.target as Node)) {
        closeSendDropdown();
      }
    }
    document.addEventListener('mousedown', handleOutsideClick);
    return () => document.removeEventListener('mousedown', handleOutsideClick);
  }, [showSendDropdown]);

  // Close slash menu on outside click
  useEffect(() => {
    if (!slashMenu) return;
    function handleOutsideClick(e: MouseEvent) {
      const target = e.target as Node;
      if (dialogRef.current?.contains(target)) return;
      setSlashMenu(null);
      slashStartPosRef.current = null;
    }
    document.addEventListener('mousedown', handleOutsideClick);
    return () => document.removeEventListener('mousedown', handleOutsideClick);
  }, [slashMenu]);

  // Escape key handler
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if (e.key !== 'Escape') return;
      if (sendCountdown !== null) {
        e.preventDefault();
        e.stopPropagation();
        cancelPendingSend();
        return;
      }
      if (confirmClose || closeSaveInProgress) return;
      const hasContent = !sent && (to.trim() || subject.trim() || (editor?.getText().trim()));
      if (hasContent) {
        setConfirmClose(true);
      } else {
        onClose();
      }
    }
    window.addEventListener('keydown', onKeyDown, true);
    return () => window.removeEventListener('keydown', onKeyDown, true);
  }, [sendCountdown, cancelPendingSend, confirmClose, closeSaveInProgress, sent, to, subject, editor, onClose]);

  async function handleSend(e: { preventDefault(): void }) {
    e.preventDefault();
    if (sending || sent || sendInProgressRef.current) return;
    if (sendCountdown !== null) {
      setError(t('alreadyScheduled'));
      return;
    }
    if (!to.trim()) {
      setError(t('errToRequired'));
      return;
    }
    const bodyText = editor?.getText() ?? '';
    if (!bodyText.trim() && !subject.trim()) {
      setError(t('errSubjectOrBody'));
      return;
    }
    setError('');
    const invalidRecipients = invalidRecipientAddresses(to, cc, bcc);
    if (invalidRecipients.length > 0) {
      setError(t('errAddressFormat', { addrs: invalidRecipients.join(', ') }));
      return;
    }
    const hasUploadingAttachments = uploadedAttachments.some((attachment) => attachment.uploading);
    if (hasUploadingAttachments) {
      setError(t('errAttachmentUploading'));
      return;
    }
    const hasFailedAttachments = uploadedAttachments.some((attachment) => attachment.error);
    if (hasFailedAttachments) {
      setError(t('errAttachmentFailed'));
      return;
    }
    if (scheduledAt) {
      const scheduledTime = new Date(scheduledAt).getTime();
      if (!Number.isFinite(scheduledTime)) {
        setError(t('errScheduleInvalid'));
        return;
      }
      if (scheduledTime <= Date.now()) {
        setError(t('errSchedulePast'));
        return;
      }
    }
    sendInProgressRef.current = true;
    sendExecutionRef.current = false;
    const attachmentIds = readyAttachmentIds();
    const draftData = buildDraftData(to, cc, bcc, subject.trim(), bodyText, editor?.getHTML() ?? '');
    const msg: SendMessageRequest = {
      to: parseAddrs(to),
      ...(cc.trim() && { cc: parseAddrs(cc) }),
      ...(bcc.trim() && { bcc: parseAddrs(bcc) }),
      subject: subject.trim(),
      text_body: bodyText,
      ...(editor && { html_body: editor.getHTML() }),
      ...(intent !== 'new' && sourceMessage && { intent: backendComposeIntent(intent), source_message_id: sourceMessage.id }),
      ...(attachmentIds.length > 0 && { attachment_ids: attachmentIds }),
      ...(scheduledAt && { scheduled_at: new Date(scheduledAt).toISOString() }),
      ...(fromAddress && { from: fromAddress }),
      ...(trackOpens && { track_opens: true }),
    };
    pendingMsgRef.current = msg;
    pendingDraftSendRef.current = false;
    setSending(true);
    try {
      if (draftIdRef.current) await updateDraft(draftIdRef.current, draftData);
      else {
        const saved = await saveDraft(draftData);
        draftIdRef.current = saved.draft.id;
      }
      pendingDraftSendRef.current = true;
      markDraftSaved();
    } catch (err: unknown) {
      handleSendPreparationFailure(err);
      return;
    } finally {
      setSending(false);
    }
    if (scheduledAt) {
      const useDraftSend = shouldSendSavedDraft();
      sendExecutionRef.current = true;
      setSending(true);
      sendPreparedMessage(msg, useDraftSend)
        .then(async (res) => { await handleSuccessfulSend(msg, res.message, useDraftSend); })
        .catch((err: unknown) => {
          handleSendFailure(err);
        })
        .finally(() => setSending(false));
    } else {
      let sendDelay = 5;
      try { sendDelay = Number((JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as { sendDelay?: number }).sendDelay ?? 5); } catch { /* */ }
      if (sendDelay === 0) {
        const useDraftSend = shouldSendSavedDraft();
        setSending(true);
        sendPreparedMessage(msg, useDraftSend)
          .then(async (res) => { await handleSuccessfulSend(msg, res.message, useDraftSend); })
          .catch((err: unknown) => {
            handleSendFailure(err);
          })
          .finally(() => setSending(false));
      } else {
        setSendCountdown(sendDelay);
      }
    }
  }

  const filteredCmds = slashMenu ? filteredCommands(slashMenu.query) : [];
  const scheduleOptions = getScheduleOptions();

  const sendResultLabel = formatSendResultLabel(sendResult);
  const sendButtonUploading = uploadedAttachments.some((a) => a.uploading);
  const sendButtonDisabled = sending || sent || sendCountdown !== null || sendButtonUploading;
  const miscT = (k: string) => tMisc(k.replace(/^misc\.compose\./, ''));
  const sendButtonLabel = composeSendButtonLabel({
    sending,
    sent,
    scheduled: !!scheduledAt,
    uploading: sendButtonUploading,
  }, miscT);
  const closeSavePrompt = composeCloseSavePrompt(!!scheduledAt, miscT);
  const closeSaveButtonAriaLabel = composeCloseSaveButtonAriaLabel(closeSaveInProgress, miscT);
  const closeSaveButtonLabel = composeCloseSaveButtonLabel(closeSaveInProgress, miscT);
  const scheduleMinDateTime = toDateTimeLocalValue(new Date(Date.now() + 60000));
  const closeSendDropdown = useCallback(() => setShowSendDropdown(false), []);
  const cancelCloseConfirmation = useCallback(() => setConfirmClose(false), []);

  function getScheduleOptions(): { label: string; sub: string; date: Date }[] {
    const now = new Date();
    const tomorrow = new Date(now);
    tomorrow.setDate(tomorrow.getDate() + 1);
    const tomorrowMorning = new Date(tomorrow); tomorrowMorning.setHours(8, 0, 0, 0);
    const tomorrowAfternoon = new Date(tomorrow); tomorrowAfternoon.setHours(13, 0, 0, 0);
    const nextMonday = new Date(now);
    const day = now.getDay();
    const daysUntilMonday = day === 0 ? 1 : (8 - day);
    nextMonday.setDate(now.getDate() + daysUntilMonday);
    nextMonday.setHours(8, 0, 0, 0);
    const fmt = (d: Date) => new Intl.DateTimeFormat('ko-KR', { month: 'numeric', day: 'numeric', hour: 'numeric', minute: '2-digit', hour12: true }).format(d);
    const dayFmt = (d: Date) => new Intl.DateTimeFormat('ko-KR', { weekday: 'short' }).format(d);
    return [
      { label: t('tmrMorning'), sub: fmt(tomorrowMorning), date: tomorrowMorning },
      { label: t('tmrAfternoon'), sub: fmt(tomorrowAfternoon), date: tomorrowAfternoon },
      { label: t('weekdayMorning', { weekday: dayFmt(nextMonday) }), sub: fmt(nextMonday), date: nextMonday },
    ];
  }

  return (
    <>
      <div aria-hidden="true" style={{ position: 'fixed', inset: 0, zIndex: 99, pointerEvents: 'none' }} />

      <div
        ref={dialogRef}
        role="dialog"
        aria-label={t('newMessageAria')}
        aria-modal="true"
        onDragEnter={(e) => { e.preventDefault(); dragCounterRef.current++; setDragOver(true); }}
        onDragLeave={() => { dragCounterRef.current--; if (dragCounterRef.current <= 0) { dragCounterRef.current = 0; setDragOver(false); } }}
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => { e.preventDefault(); dragCounterRef.current = 0; setDragOver(false); if (e.dataTransfer.files.length) handleFileSelect(e.dataTransfer.files); }}
        onPaste={(e) => {
          const imageFiles = Array.from(e.clipboardData.items)
            .filter((item) => item.type.startsWith('image/'))
            .map((item) => item.getAsFile())
            .filter(Boolean) as File[];
          if (imageFiles.length > 0) {
            const dt = new DataTransfer();
            imageFiles.forEach((f) => dt.items.add(f));
            handleFileSelect(dt.files);
          }
        }}
        style={{
          position: 'fixed',
          ...(isMobile
            ? { inset: 0, borderRadius: 0, width: '100%', maxWidth: 'none', maxHeight: '100dvh', height: '100dvh' }
            : fullscreen
              ? { inset: '16px', width: 'auto', maxWidth: 'none', bottom: '16px' }
              : pos
                ? { top: pos.y, left: pos.x, width: size.w, height: minimized ? undefined : size.h, maxHeight: minimized ? '44px' : undefined }
                : { bottom: '24px', insetInlineEnd: `${24 + windowOffset * 576}px`, width: size.w, height: minimized ? undefined : size.h, maxHeight: minimized ? '44px' : 'calc(100vh - 48px)' }
          ),
          background: 'var(--color-bg-primary)',
          border: `1px solid ${dragOver ? 'var(--color-accent)' : isMobile ? 'transparent' : 'var(--color-border-default)'}`,
          borderRadius: isMobile ? 0 : '8px',
          boxShadow: isMobile ? 'none' : dragOver ? '0 0 0 2px var(--color-accent-subtle)' : '0 8px 32px rgba(0,0,0,0.16)',
          zIndex: 100,
          display: 'flex',
          flexDirection: 'column',
          animation: 'composeIn 120ms ease-out',
          height: isMobile || (fullscreen && !minimized) ? '100%' : undefined,
          overflow: 'hidden',
          transition: 'border-color 100ms ease, box-shadow 100ms ease',
        }}
      >
        {/* Resize handles */}
        {!isMobile && !fullscreen && !minimized && (
          <>
            <div onMouseDown={(e) => startResize(e, 'n')} style={{ position: 'absolute', top: 0, left: 4, right: 4, height: '4px', cursor: 'n-resize', zIndex: 10 }} />
            <div onMouseDown={(e) => startResize(e, 's')} style={{ position: 'absolute', bottom: 0, left: 4, right: 4, height: '4px', cursor: 's-resize', zIndex: 10 }} />
            <div onMouseDown={(e) => startResize(e, 'e')} style={{ position: 'absolute', top: 4, right: 0, bottom: 4, width: '4px', cursor: 'e-resize', zIndex: 10 }} />
            <div onMouseDown={(e) => startResize(e, 'w')} style={{ position: 'absolute', top: 4, left: 0, bottom: 4, width: '4px', cursor: 'w-resize', zIndex: 10 }} />
            <div onMouseDown={(e) => startResize(e, 'ne')} style={{ position: 'absolute', top: 0, right: 0, width: '8px', height: '8px', cursor: 'ne-resize', zIndex: 11 }} />
            <div onMouseDown={(e) => startResize(e, 'nw')} style={{ position: 'absolute', top: 0, left: 0, width: '8px', height: '8px', cursor: 'nw-resize', zIndex: 11 }} />
            <div onMouseDown={(e) => startResize(e, 'se')} style={{ position: 'absolute', bottom: 0, right: 0, width: '8px', height: '8px', cursor: 'se-resize', zIndex: 11 }} />
            <div onMouseDown={(e) => startResize(e, 'sw')} style={{ position: 'absolute', bottom: 0, left: 0, width: '8px', height: '8px', cursor: 'sw-resize', zIndex: 11 }} />
          </>
        )}

        {dragOver && !minimized && (
          <div style={{ position: 'absolute', inset: 0, zIndex: 200, background: 'var(--color-accent-subtle)', display: 'flex', alignItems: 'center', justifyContent: 'center', pointerEvents: 'none', borderRadius: '8px' }}>
            <div style={{ textAlign: 'center', color: 'var(--color-accent)', fontSize: '15px', fontWeight: 500 }}>
              <PaperClipIcon style={{ width: '40px', height: '40px', marginBottom: '8px' }} />
              {t('dropFilesHere')}
            </div>
          </div>
        )}
        <style>{`
          @keyframes composeIn {
            from { opacity: 0; transform: scale(0.97) translateY(8px); }
            to   { opacity: 1; transform: scale(1) translateY(0); }
          }
          .tiptap p.is-editor-empty:first-child::before {
            content: attr(data-placeholder);
            float: left;
            color: var(--color-text-tertiary);
            pointer-events: none;
            height: 0;
          }
          .tiptap a { color: var(--color-accent); text-decoration: underline; }
          .tiptap p { margin: 0 0 4px; }
          .tiptap ul, .tiptap ol { padding-left: 20px; }
.tiptap blockquote { border-left: 3px solid var(--color-border-default); margin: 4px 0; padding: 4px 12px; color: var(--color-text-secondary); }
.tiptap code { background: var(--color-bg-secondary); border: 1px solid var(--color-border-subtle); border-radius: 3px; padding: 1px 4px; font-family: monospace; font-size: 12px; }
.ProseMirror img { max-width: 100%; height: auto; cursor: pointer; }
.ProseMirror img.ProseMirror-selectednode { outline: 2px solid var(--color-accent); }
        `}</style>

        {/* Header */}
        <ComposeModalHeader
          minimized={minimized}
          setMinimized={setMinimized}
          fullscreen={fullscreen}
          setFullscreen={setFullscreen}
          isMobile={isMobile}
          intent={intent}
          subject={subject}
          sent={sent}
          to={to}
          editor={editor}
          setConfirmClose={setConfirmClose}
          onClose={onClose}
          startDrag={startDrag}
        />

        {/* Close confirmation panel */}
        {confirmClose && (
          <ComposeClosePanel
            closeSaveInProgress={closeSaveInProgress}
            closeSavePrompt={closeSavePrompt}
            closeSaveButtonAriaLabel={closeSaveButtonAriaLabel}
            closeSaveButtonLabel={closeSaveButtonLabel}
            onSaveDraft={() => { void saveDraftAndClose(); }}
            onDiscard={discardDraftAndClose}
            onCancel={cancelCloseConfirmation}
          />
        )}

        {/* Form */}
        <form
          onSubmit={handleSend}
          onKeyDown={(e) => {
            if ((e.ctrlKey || e.metaKey) && e.key === 'Enter') { e.preventDefault(); handleSend(e); }
            if ((e.ctrlKey || e.metaKey) && e.key === 's') { e.preventDefault(); void handleManualSave(); }
          }}
          style={{ display: 'flex', flexDirection: 'column', flex: 1, overflow: 'hidden' }}
        >

          <ComposeFormFields
            to={to} setTo={setTo}
            cc={cc} setCc={setCc}
            bcc={bcc} setBcc={setBcc}
            showCc={showCc} setShowCc={setShowCc}
            showBcc={showBcc} setShowBcc={setShowBcc}
            fromAddress={fromAddress} setFromAddress={setFromAddress}
            availableAddresses={availableAddresses}
            userEmail={userEmail}
            recentRecipients={recentRecipients}
            toRef={toRef} ccRef={ccRef} bccRef={bccRef} subjectRef={subjectRef}
            subject={subject} setSubject={setSubject} subjectInputRef={subjectInputRef}
            error={error} setError={setError}
            triggerAutoSave={triggerAutoSave}
            editor={editor}
            setShowOrgPicker={setShowOrgPicker}
          />

          {/* TipTap editor body */}
          <div style={{ flex: 1, overflowY: 'auto', minHeight: 0 }}>
            <EditorContent editor={editor} />
          </div>

          {/* Signature editor */}
          <ComposeSigEditorPanel
            open={showSigEditor}
            signature={signature}
            setSignature={setSignature}
          />

          <ComposeAttachmentPanel
            attachments={uploadedAttachments}
            onRemove={(id) => setUploadedAttachments((prev) => prev.filter((a) => a.id !== id))}
            onRetry={retryAttachmentUpload}
          />

          <ComposeModalFooter
            sendDropdownRef={sendDropdownRef}
            showSendDropdown={showSendDropdown}
            setShowSendDropdown={setShowSendDropdown}
            sending={sending}
            sendButtonDisabled={sendButtonDisabled}
            sendButtonLabel={sendButtonLabel}
            sendButtonUploading={sendButtonUploading}
            sendResultLabel={sendResultLabel}
            error={error}
            sent={sent}
            saveStatus={saveStatus}
            savedAt={savedAt}
            sendCountdown={sendCountdown}
            sendAndArchiveRef={sendAndArchiveRef}
            scheduledAt={scheduledAt}
            setScheduledAt={setScheduledAt}
            setShowSchedule={setShowSchedule}
            scheduleOptions={scheduleOptions}
            handleSend={handleSend}
            closeSendDropdown={closeSendDropdown}
            onArchiveSource={onArchiveSource}
            onCancelPendingSend={cancelPendingSend}
          />
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px', padding: '0 12px 8px', flexShrink: 0 }}>
            <div style={{ flex: 1 }} />
            <ComposeModalActions
              editor={editor}
              fileInputRef={fileInputRef}
              imageInputRef={imageInputRef}
              handleFileSelect={handleFileSelect}
              handleImageFileSelect={handleImageFileSelect}
              showEmojiPicker={showEmojiPicker}
              setShowEmojiPicker={setShowEmojiPicker}
              showDrivePicker={showDrivePicker}
              setShowDrivePicker={setShowDrivePicker}
              drivePickerNodes={drivePickerNodes}
              drivePickerLoading={drivePickerLoading}
              drivePickerCrumbs={drivePickerCrumbs}
              attachingDriveId={attachingDriveId}
              openDrivePicker={openDrivePicker}
              handleAttachFromDrive={handleAttachFromDrive}
              showTemplates={showTemplates}
              setShowTemplates={setShowTemplates}
              showTemplateSave={showTemplateSave}
              setShowTemplateSave={setShowTemplateSave}
              templates={templates}
              templateSaveName={templateSaveName}
              setTemplateSaveName={setTemplateSaveName}
              saveTemplate={saveTemplate}
              deleteTemplate={deleteTemplate}
              subject={subject}
              setSubject={setSubject}
              showSigEditor={showSigEditor}
              setShowSigEditor={setShowSigEditor}
              trackOpens={trackOpens}
              setTrackOpens={setTrackOpens}
              showSchedule={showSchedule}
              setShowSchedule={setShowSchedule}
              scheduledAt={scheduledAt}
              setScheduledAt={setScheduledAt}
              scheduleMinDateTime={scheduleMinDateTime}
              scheduleOptions={scheduleOptions}
              imageResizeToolbar={imageResizeToolbar}
            />
          </div>
        </form>
      </div>

      <ComposeSlashCommandMenu
        menu={slashMenu}
        commands={filteredCmds}
        selectedIndex={slashIndex}
        onSelect={(cmd) => runSlashCommand(cmd)}
        onHover={setSlashIndex}
      />

      {/* Org picker */}
      {showOrgPicker && (
        <OrgPickerModal
          initialTo={parseToPickerItems(to)}
          initialCc={parseToPickerItems(cc)}
          initialBcc={parseToPickerItems(bcc)}
          onClose={() => setShowOrgPicker(false)}
          onConfirm={({ to: t, cc: c, bcc: b }) => {
            const newTo = pickerItemsToString(t);
            const newCc = pickerItemsToString(c);
            const newBcc = pickerItemsToString(b);
            setTo(newTo); toRef.current = newTo;
            if (newCc) { setShowCc(true); setCc(newCc); ccRef.current = newCc; }
            if (newBcc) { setShowBcc(true); setBcc(newBcc); bccRef.current = newBcc; }
            setShowOrgPicker(false);
          }}
        />
      )}

      {/* Floating image resize toolbar */}
      <ComposeImageResizeToolbar
        editor={editor}
        imageResizeToolbar={imageResizeToolbar}
      />
    </>
  );
}
