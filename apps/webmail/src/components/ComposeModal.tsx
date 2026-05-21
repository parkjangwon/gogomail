'use client';

import { useState, useRef, useEffect, useCallback } from 'react';
import { useEditor, EditorContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Underline from '@tiptap/extension-underline';
import TextAlign from '@tiptap/extension-text-align';
import Placeholder from '@tiptap/extension-placeholder';
import Image from '@tiptap/extension-image';
import { sendMessage, saveDraft, updateDraft, deleteDraft, sendDraft, uploadAttachment, attachDriveFileToEmail, listDriveNodes, listUserAddresses, getPreferences, setPreferences } from '@/lib/api';
import type { DriveNode, UIComposeIntent, MessageDetail, SendMessageRequest, SendMessageResult, UserAddressEntry } from '@/lib/api';
import { composeCloseSaveButtonAriaLabel } from '@/lib/composeCloseSaveButtonAriaLabel';
import { composeCloseSaveButtonLabel } from '@/lib/composeCloseSaveButtonLabel';
import { composeCloseSavePrompt } from '@/lib/composeCloseSavePrompt';
import { composeSendButtonLabel } from '@/lib/composeSendButtonLabel';
import { toDateTimeLocalValue } from '@/lib/dateTimeLocal';
import { formatSendResultLabel } from '@/lib/sendResultLabel';
import { DriveNodeIcon } from '@/lib/driveNodeIcon';
import { stableId } from '@/lib/stableId';
import { escapeHtml, parseAddrs, EmailTemplate, backendComposeIntent } from '@/lib/compose/composeUtils';
import { loadLocalEmailTemplates, normalizeEmailTemplates, saveLocalEmailTemplates } from '@/lib/emailTemplates';
import { buildQuoteHTML, emailOf, invalidRecipientAddresses, parseToPickerItems, pickerItemsToString } from '@/lib/mail-address';
import { SLASH_COMMANDS, type SlashCommand } from '@/lib/compose/slashCommands';
import { RecipientChips } from './RecipientChips';
import { OrgPickerModal } from './OrgPickerModal';
import { ComposeModalActions } from './ComposeModalActions';
import { ComposeModalFooter } from './ComposeModalFooter';
import { ComposeSlashCommandMenu } from './compose/ComposeSlashCommandMenu';
import {
  PaperClipIcon,
  LinkIcon,
  PencilSquareIcon as PencilSquareIconHero,
  DocumentTextIcon,
  CalendarIcon,
  ChevronUpIcon,
  ExclamationTriangleIcon,
  ArrowPathIcon,
  ListBulletIcon,
  NumberedListIcon,
  XMarkIcon,
  CloudIcon,
  ChevronRightIcon,
  FaceSmileIcon,
  ArchiveBoxIcon,
  PhotoIcon,
  UsersIcon,
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
  isMobile?: boolean;
  windowOffset?: number;
  onArchiveSource?: () => void;
}

const SCHEDULE_INPUT_HELP = '예약 전송은 현재 시각 이후만 선택할 수 있습니다.';

export function ComposeModal({ onClose, intent = 'new', sourceMessage, draftMessage, userEmail, initialTo, initialSubject, initialBody, isMobile, windowOffset = 0, onArchiveSource }: ComposeModalProps) {
  const replyTo = intent === 'reply' || intent === 'reply_all'
    ? sourceMessage?.from_addr ?? ''
    : '';
  const replyCc = intent === 'reply_all' && sourceMessage
    ? (sourceMessage.to_addrs ?? [])
        .map(emailOf)
        .filter((addr) => !userEmail || addr.toLowerCase() !== userEmail.toLowerCase())
        .join(', ')
    : '';
  const replySubject = sourceMessage
    ? intent === 'forward'
      ? `Fwd: ${sourceMessage.subject}`
      : `Re: ${sourceMessage.subject}`
    : '';

  const draftTo = draftMessage ? (draftMessage.to_addrs ?? []).map(emailOf).join(', ') : '';
  const draftCc = draftMessage ? (draftMessage.cc_addrs ?? []).map(emailOf).join(', ') : '';

  const [to, setTo] = useState(draftMessage ? draftTo : (initialTo ?? replyTo));
  const [cc, setCc] = useState(draftMessage ? draftCc : replyCc);
  const [bcc, setBcc] = useState('');
  const [showCc, setShowCc] = useState(!!(draftMessage ? draftCc : replyCc));
  const [showBcc, setShowBcc] = useState(false);
  const [subject, setSubject] = useState(draftMessage ? (draftMessage.subject ?? '') : (initialSubject ?? replySubject));
  const [sending, setSending] = useState(false);
  const [error, setError] = useState('');
  const [sent, setSent] = useState(false);
  const [sendResult, setSendResult] = useState<SendMessageResult | null>(null);
  const [sendCountdown, setSendCountdown] = useState<number | null>(null);
  const [trackOpens, setTrackOpens] = useState(false);
  const pendingMsgRef = useRef<SendMessageRequest | null>(null);
  const pendingDraftSendRef = useRef(false);
  const sendAndArchiveRef = useRef(false);
  const [scheduledAt, setScheduledAt] = useState('');
  const [showSchedule, setShowSchedule] = useState(false);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'saving' | 'saved'>('idle');
  const [savedAt, setSavedAt] = useState('');
  const [minimized, setMinimized] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const [confirmClose, setConfirmClose] = useState(false);
  const [closeSaveInProgress, setCloseSaveInProgress] = useState(false);
  const [showSigEditor, setShowSigEditor] = useState(false);
  const [signature, setSignature] = useState(() => {
    try { return localStorage.getItem('webmail_signature') ?? ''; } catch { return ''; }
  });
  const [recentRecipients] = useState<string[]>(() => {
    try {
      const recents: string[] = JSON.parse(localStorage.getItem('webmail_recent_recipients') ?? '[]');
      const contacts: Record<string, string> = JSON.parse(localStorage.getItem('webmail_contacts') ?? '{}');
      // Enrich plain email entries with stored contact names
      const enriched = recents.map((r) => {
        if (r.includes('<')) return r;
        const name = contacts[r.toLowerCase()];
        return name ? `${name} <${r}>` : r;
      });
      // Add contacts not yet in recents
      const recentEmails = new Set(recents.map((r) => { const m = r.match(/<([^>]+)>/); return (m ? m[1] : r).toLowerCase(); }));
      Object.entries(contacts).forEach(([email, name]) => {
        if (!recentEmails.has(email)) enriched.push(`${name} <${email}>`);
      });
      return enriched.slice(0, 50);
    } catch { return []; }
  });
  const draftIdRef = useRef<string>(draftMessage?.id ?? '');
  const saveTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [uploadedAttachments, setUploadedAttachments] = useState<Array<{ id: string; filename: string; size: number; uploading?: boolean; error?: string; file?: File }>>([]);
  const [dragOver, setDragOver] = useState(false);
  const dragCounterRef = useRef(0);
  const [showTemplates, setShowTemplates] = useState(false);
  const [templates, setTemplates] = useState<EmailTemplate[]>(() => loadLocalEmailTemplates());
  const [templateSaveName, setTemplateSaveName] = useState('');
  const [showTemplateSave, setShowTemplateSave] = useState(false);
  const templateMenuRef = useRef<HTMLDivElement>(null);
  const sendDropdownRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    let cancelled = false;
    getPreferences().then((prefs) => {
      if (cancelled || !prefs.templates) return;
      const serverTemplates = normalizeEmailTemplates(prefs.templates);
      setTemplates(serverTemplates);
      saveLocalEmailTemplates(serverTemplates);
    }).catch(() => {});
    return () => { cancelled = true; };
  }, []);

  const persistTemplates = useCallback((next: EmailTemplate[]) => {
    const normalized = normalizeEmailTemplates(next);
    setTemplates(normalized);
    saveLocalEmailTemplates(normalized);
    setPreferences({ templates: normalized }).catch(() => {});
  }, []);

  // Slash command menu state
  const [slashMenu, setSlashMenu] = useState<{ query: string; top: number; cursorTop: number; left: number } | null>(null);
  const [slashIndex, setSlashIndex] = useState(0);
  const slashStartPosRef = useRef<number | null>(null);
  const slashMenuRef = useRef<typeof slashMenu>(null);
  const slashIndexRef = useRef(0);
  const runSlashCommandRef = useRef<((cmd: SlashCommand) => void) | null>(null);
  useEffect(() => { slashMenuRef.current = slashMenu; }, [slashMenu]);
  useEffect(() => { slashIndexRef.current = slashIndex; }, [slashIndex]);

  const [showEmojiPicker, setShowEmojiPicker] = useState(false);
  const [showDrivePicker, setShowDrivePicker] = useState(false);
  const [drivePickerNodes, setDrivePickerNodes] = useState<DriveNode[]>([]);
  const [drivePickerLoading, setDrivePickerLoading] = useState(false);
  const [drivePickerCrumbs, setDrivePickerCrumbs] = useState<Array<{ id: string | undefined; name: string }>>([{ id: undefined, name: '드라이브' }]);
  const [attachingDriveId, setAttachingDriveId] = useState<string | null>(null);

  const [showOrgPicker, setShowOrgPicker] = useState(false);

  const dialogRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState<{ x: number; y: number } | null>(null);
  const [size, setSize] = useState<{ w: number; h: number }>(() => {
    try {
      const s = localStorage.getItem('webmail_compose_size');
      const parsed = s ? JSON.parse(s) : { w: 560, h: 520 };
      const maxH = typeof window !== 'undefined' ? window.innerHeight - 60 : 800;
      return { w: parsed.w, h: Math.min(parsed.h, maxH) };
    } catch { return { w: 560, h: 520 }; }
  });
  const [showSendDropdown, setShowSendDropdown] = useState(false);
  const [fromAddress, setFromAddress] = useState(userEmail ?? '');
  const [availableAddresses, setAvailableAddresses] = useState<UserAddressEntry[]>([]);

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
        setUploadedAttachments((prev) => prev.map((a) => a.id === tempId ? { ...a, uploading: false, error: '업로드 실패' } : a));
      }
    }
  }, []);

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
        attachment.id === attachmentId ? { ...attachment, uploading: false, error: '업로드 실패' } : attachment,
      ));
    }
  }, [uploadedAttachments]);

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
  }, [drivePickerCrumbs, openDrivePicker]);

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

  const rememberSendResult = useCallback((result: SendMessageResult | undefined) => {
    if (result) setSendResult(result);
  }, []);

  const sendResultLabel = formatSendResultLabel(sendResult);
  const sendButtonUploading = uploadedAttachments.some((a) => a.uploading);
  const sendButtonDisabled = sending || sent || sendButtonUploading;
  const sendButtonLabel = composeSendButtonLabel({
    sending,
    sent,
    scheduled: !!scheduledAt,
    uploading: sendButtonUploading,
  });
  const closeSavePrompt = composeCloseSavePrompt(!!scheduledAt);
  const closeSaveButtonAriaLabel = composeCloseSaveButtonAriaLabel(closeSaveInProgress);
  const closeSaveButtonLabel = composeCloseSaveButtonLabel(closeSaveInProgress);
  const scheduleMinDateTime = toDateTimeLocalValue(new Date(Date.now() + 60000));
  const closeSendDropdown = useCallback(() => setShowSendDropdown(false), []);
  const cancelCloseConfirmation = useCallback(() => setConfirmClose(false), []);

  const persistSuccessfulSendLocalState = useCallback((msg: SendMessageRequest) => {
    try {
      const newAddrs = [...(msg.to ?? []), ...(msg.cc ?? []), ...(msg.bcc ?? [])]
        .map((a) => a.name ? `${a.name} <${a.address}>` : a.address).filter(Boolean);
      const merged = [...new Set([...newAddrs, ...recentRecipients])].slice(0, 30);
      localStorage.setItem('webmail_recent_recipients', JSON.stringify(merged));
      const followUpDays = Number((JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as Record<string, unknown>).followUpDays ?? 0);
      if (followUpDays > 0 && msg.to?.length) {
        const remindAt = new Date(Date.now() + followUpDays * 86400000).toISOString();
        const followups: Record<string, unknown>[] = JSON.parse(localStorage.getItem('webmail_followups') ?? '[]');
        followups.push({ remindAt, subject: msg.subject ?? '', to: msg.to[0].address, createdAt: new Date().toISOString() });
        localStorage.setItem('webmail_followups', JSON.stringify(followups));
      }
    } catch { /* keep send success independent from local storage */ }
  }, [recentRecipients]);

  const handleSuccessfulSend = useCallback(async (msg: SendMessageRequest, result: SendMessageResult, useDraftSend: boolean) => {
    rememberSendResult(result);
    persistSuccessfulSendLocalState(msg);
    await clearSentDraft(!useDraftSend);
    pendingDraftSendRef.current = false;
    setSent(true);
    setTimeout(() => {
      if (sendAndArchiveRef.current) {
        onArchiveSource?.();
        sendAndArchiveRef.current = false;
      }
      onClose();
    }, 1500);
  }, [clearSentDraft, onArchiveSource, onClose, persistSuccessfulSendLocalState, rememberSendResult]);

  const handleSendFailure = useCallback((err: unknown, clearCountdown = false) => {
    const message = err instanceof Error ? err.message : '전송에 실패했습니다.';
    setError(`${message} 초안은 보존되어 다시 전송할 수 있습니다.`);
    pendingDraftSendRef.current = false;
    if (clearCountdown) setSendCountdown(null);
  }, []);

  const handleSendPreparationFailure = useCallback((err: unknown) => {
    const message = err instanceof Error ? err.message : '초안 전송 준비에 실패했습니다.';
    pendingMsgRef.current = null;
    pendingDraftSendRef.current = false;
    setError(`${message} 전송은 시작되지 않았습니다. 내용을 확인한 뒤 다시 저장하거나 전송해 주세요.`);
  }, []);

  const shouldSendSavedDraft = useCallback(() => pendingDraftSendRef.current && !!draftIdRef.current, []);

  const sendPreparedMessage = useCallback((msg: SendMessageRequest, useDraftSend: boolean) => {
    const draftId = draftIdRef.current;
    return useDraftSend && draftId ? sendDraft(draftId) : sendMessage(msg);
  }, []);

  const buildDraftData = useCallback((toVal: string, ccVal: string, bccVal: string, subjectVal: string, bodyText: string, bodyHtml: string) => {
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
  }, [intent, sourceMessage, fromAddress, readyAttachmentIds, trackOpens, scheduledAt]);

  const triggerAutoSave = useCallback((toVal: string, ccVal: string, bccVal: string, subjectVal: string, bodyText: string, bodyHtml: string) => {
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
        setSavedAt(`${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`);
        setSaveStatus('saved');
      } catch {
        setSaveStatus('idle');
      }
    }, 3000);
  }, [buildDraftData]);

  useEffect(() => {
    return () => { if (saveTimerRef.current) clearTimeout(saveTimerRef.current); };
  }, []);

  useEffect(() => {
    listUserAddresses().then((addrs) => {
      setAvailableAddresses(addrs);
      const primary = addrs.find((a) => a.is_primary);
      if (primary && !fromAddress) setFromAddress(primary.address);
    }).catch(() => {});
  }, []);

  useEffect(() => {
    function handleClick(e: MouseEvent) {
      if (templateMenuRef.current && !templateMenuRef.current.contains(e.target as Node)) {
        setShowTemplates(false);
        setShowTemplateSave(false);
      }
    }
    if (showTemplates) document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, [showTemplates]);

  useEffect(() => {
    if (!showSendDropdown) return;
    function handleOutsideClick(e: MouseEvent) {
      if (sendDropdownRef.current && !sendDropdownRef.current.contains(e.target as Node)) {
        closeSendDropdown();
      }
    }
    document.addEventListener('mousedown', handleOutsideClick);
    return () => document.removeEventListener('mousedown', handleOutsideClick);
  }, [closeSendDropdown, showSendDropdown]);

  // Close slash menu on outside click
  useEffect(() => {
    if (!slashMenu) return;
    function handleOutsideClick(e: MouseEvent) {
      const target = e.target as Node;
      // If the click is inside the editor, let the onUpdate handler decide
      if (dialogRef.current?.contains(target)) return;
      setSlashMenu(null);
      slashStartPosRef.current = null;
    }
    document.addEventListener('mousedown', handleOutsideClick);
    return () => document.removeEventListener('mousedown', handleOutsideClick);
  }, [slashMenu]);

  const toRef = useRef(draftMessage ? draftTo : replyTo);
  const ccRef = useRef(draftMessage ? draftCc : replyCc);
  const bccRef = useRef('');
  const subjectRef = useRef(draftMessage ? (draftMessage.subject ?? '') : replySubject);

  const sigHTML = signature.trim()
    ? `<p></p><p>--</p><p>${signature.trim().split('\n').map((l) => escapeHtml(l)).join('</p><p>')}</p>`
    : '';

  const quoteOnReply = (() => {
    try { return (JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as { quoteOnReply?: boolean }).quoteOnReply !== false; } catch { return true; }
  })();

  const initialContent = draftMessage
    ? (draftMessage.html_body ?? (draftMessage.text_body
        ? draftMessage.text_body.split('\n').map((l) => `<p>${escapeHtml(l) || '&nbsp;'}</p>`).join('')
        : ''))
    : (sourceMessage && (intent === 'reply' || intent === 'reply_all' || intent === 'forward')
        ? `<p></p>${sigHTML ? sigHTML + '<p></p>' : ''}${quoteOnReply ? buildQuoteHTML(intent, sourceMessage) : ''}`
        : initialBody
        ? `${initialBody.split('\n').map((l) => `<p>${escapeHtml(l) || '&nbsp;'}</p>`).join('')}<p></p>${sigHTML}`
        : `<p></p>${sigHTML}`);

  const imageInputRef = useRef<HTMLInputElement>(null);
  const [imageResizeToolbar, setImageResizeToolbar] = useState<{ top: number; left: number } | null>(null);

  const editor = useEditor({
    extensions: [
      StarterKit,
      Underline,
      Link.configure({ openOnClick: false }),
      TextAlign.configure({ types: ['heading', 'paragraph'] }),
      Placeholder.configure({ placeholder: '메시지를 입력하세요...' }),
      Image.configure({ inline: true, allowBase64: true }),
    ],
    content: initialContent,
    immediatelyRender: false,
    editorProps: {
      attributes: {
        style: [
          'min-height: 200px',
          'padding: 12px 16px',
          'outline: none',
          'font-size: 14px',
          'line-height: 1.6',
          'color: var(--color-text-primary)',
          'font-family: inherit',
        ].join(';'),
        'aria-label': '메일 본문',
        role: 'textbox',
        'aria-multiline': 'true',
      },
      handleKeyDown: (_view, event) => {
        const menu = slashMenuRef.current;
        if (!menu) return false;
        if (event.key === 'ArrowDown') {
          event.preventDefault();
          setSlashIndex((i) => {
            const cmds = SLASH_COMMANDS.filter((c) =>
              !menu.query || c.id.startsWith(menu.query.toLowerCase()) || c.label.includes(menu.query)
            );
            return Math.min(i + 1, cmds.length - 1);
          });
          return true;
        }
        if (event.key === 'ArrowUp') {
          event.preventDefault();
          setSlashIndex((i) => Math.max(i - 1, 0));
          return true;
        }
        if (event.key === 'Enter') {
          const cmds = SLASH_COMMANDS.filter((c) =>
            !menu.query || c.id.startsWith(menu.query.toLowerCase()) || c.label.includes(menu.query)
          );
          const cmd = cmds[slashIndexRef.current];
          if (cmd) {
            // runSlashCommand will be called after editor is fully initialized
            // Use a microtask to avoid calling a stale closure
            setTimeout(() => runSlashCommandRef.current?.(cmd), 0);
            return true;
          }
          return false;
        }
        if (event.key === 'Escape') {
          setSlashMenu(null);
          slashStartPosRef.current = null;
          return true;
        }
        return false;
      },
    },
    onUpdate: ({ editor: e }) => {
      triggerAutoSave(toRef.current, ccRef.current, bccRef.current, subjectRef.current, e.getText(), e.getHTML());
      // Slash command detection
      const { from } = e.state.selection;
      const textBefore = e.state.doc.textBetween(Math.max(0, from - 50), from, '\n');
      const slashMatch = textBefore.match(/\/(\w*)$/);
      if (slashMatch) {
        const query = slashMatch[1];
        const coords = e.view.coordsAtPos(from);
        slashStartPosRef.current = from - slashMatch[0].length;
        setSlashMenu({ query, top: coords.bottom + 4, cursorTop: coords.top, left: coords.left });
        setSlashIndex(0);
      } else {
        setSlashMenu(null);
        slashStartPosRef.current = null;
      }
    },
    onSelectionUpdate: ({ editor: e }) => {
      if (e.isActive('image')) {
        // Find the selected image DOM node and position the toolbar
        const selectedImg = e.view.dom.querySelector('img.ProseMirror-selectednode') as HTMLImageElement | null;
        if (selectedImg) {
          const rect = selectedImg.getBoundingClientRect();
          setImageResizeToolbar({ top: rect.bottom + 6, left: rect.left });
        } else {
          setImageResizeToolbar(null);
        }
      } else {
        setImageResizeToolbar(null);
      }
    },
  });

  // Move cursor to start so user types above the quoted text
  useEffect(() => {
    if (editor && initialContent) {
      editor.commands.focus('start');
    }
  }, [editor, initialContent]);

  useEffect(() => {
    if (sendCountdown === null) return;
    if (sendCountdown === 0) {
      const msg = pendingMsgRef.current;
      if (!msg) return;
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
    const t = setTimeout(() => setSendCountdown((n) => (n !== null ? n - 1 : null)), 1000);
    return () => clearTimeout(t);
  }, [sendCountdown, handleSuccessfulSend, handleSendFailure, sendPreparedMessage, shouldSendSavedDraft]);

  useEffect(() => {
    if (sendCountdown === null || sendCountdown <= 0 || !pendingMsgRef.current) return;

    const hasUnreadyAttachment = uploadedAttachments.some((attachment) => attachment.uploading || attachment.error);
    const currentAttachmentIds = readyAttachmentIds().slice().sort().join('\n');
    const pendingAttachmentIds = [...(pendingMsgRef.current.attachment_ids ?? [])].sort().join('\n');

    if (hasUnreadyAttachment || currentAttachmentIds !== pendingAttachmentIds) {
      setSendCountdown(null);
      pendingMsgRef.current = null;
      pendingDraftSendRef.current = false;
      setError('첨부파일 상태가 변경되어 전송 예약을 취소했습니다. 다시 확인 후 전송해 주세요.');
    }
  }, [sendCountdown, uploadedAttachments, readyAttachmentIds]);

  const markDraftSaved = useCallback(() => {
    const now = new Date();
    setSavedAt(`${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`);
    setSaveStatus('saved');
  }, []);

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
  }, [to, cc, bcc, subject, editor, buildDraftData, markDraftSaved]);

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
  }, [to, cc, bcc, subject, editor, buildDraftData, closeSaveInProgress, onClose]);

  const discardDraftAndClose = useCallback(() => {
    onClose();
  }, [onClose]);

  const saveTemplate = () => {
    const name = templateSaveName.trim();
    if (!name) return;
    const body = editor?.getHTML() ?? '';
    const newTemplate: EmailTemplate = { id: stableId('template'), name, subject, body };
    const updated = [...templates, newTemplate];
    persistTemplates(updated);
    setTemplateSaveName('');
    setShowTemplateSave(false);
  };

  const deleteTemplate = useCallback((id: string) => {
    persistTemplates(templates.filter((t) => t.id !== id));
  }, [persistTemplates, templates]);

  async function handleSend(e: { preventDefault(): void }) {
    e.preventDefault();
    if (sending || sent) return;
    if (sendCountdown !== null) {
      setError('이미 전송 대기 중입니다. 취소 후 다시 전송해 주세요.');
      return;
    }
    if (!to.trim()) {
      setError('받는 사람 주소를 입력하세요.');
      return;
    }
    const bodyText = editor?.getText() ?? '';
    if (!bodyText.trim() && !subject.trim()) {
      setError('제목 또는 본문을 입력하세요.');
      return;
    }
    setError('');
    const invalidRecipients = invalidRecipientAddresses(to, cc, bcc);
    if (invalidRecipients.length > 0) {
      setError(`주소 형식을 확인해 주세요: ${invalidRecipients.join(', ')}`);
      return;
    }
    const hasUploadingAttachments = uploadedAttachments.some((attachment) => attachment.uploading);
    if (hasUploadingAttachments) {
      setError('첨부파일 업로드가 완료될 때까지 기다려 주세요.');
      return;
    }
    const hasFailedAttachments = uploadedAttachments.some((attachment) => attachment.error);
    if (hasFailedAttachments) {
      setError('업로드에 실패한 첨부파일을 제거하거나 다시 업로드해 주세요.');
      return;
    }
    if (scheduledAt) {
      const scheduledTime = new Date(scheduledAt).getTime();
      if (!Number.isFinite(scheduledTime)) {
        setError('예약 전송 시간을 확인해 주세요.');
        return;
      }
      if (scheduledTime <= Date.now()) {
        setError('예약 전송 시간은 현재 시각 이후여야 합니다.');
        return;
      }
    }
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
      // Scheduled sends bypass the undo countdown and go immediately
      const useDraftSend = shouldSendSavedDraft();
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
        // No undo window — send immediately
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

  const handleImageFileSelect = useCallback(async (file: File) => {
    if (!editor) return;
    let src: string;
    if (file.size < 500 * 1024) {
      // Small image: convert to base64 data URL inline (fast, no upload needed)
      src = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result as string);
        reader.onerror = reject;
        reader.readAsDataURL(file);
      });
    } else {
      // Large image: upload as attachment, then create an object URL for inline display
      const objectUrl = URL.createObjectURL(file);
      // Also upload in the background so it's attached to the email
      uploadAttachment(file, draftIdRef.current || undefined)
        .then((att) => {
          setUploadedAttachments((prev) => [...prev, { id: att.id, filename: att.filename, size: att.size }]);
        })
        .catch(() => { /* silent — image still displays via objectUrl */ });
      src = objectUrl;
    }
    editor.chain().focus().setImage({ src, alt: file.name }).run();
  }, [editor]);

  const runSlashCommand = useCallback((cmd: SlashCommand) => {
    if (!editor || slashStartPosRef.current === null) return;
    const { from } = editor.state.selection;
    editor.chain().focus()
      .deleteRange({ from: slashStartPosRef.current, to: from })
      .run();
    switch (cmd.id) {
      case 'h1': editor.chain().focus().toggleHeading({ level: 1 }).run(); break;
      case 'h2': editor.chain().focus().toggleHeading({ level: 2 }).run(); break;
      case 'h3': editor.chain().focus().toggleHeading({ level: 3 }).run(); break;
      case 'bullet': editor.chain().focus().toggleBulletList().run(); break;
      case 'numbered': editor.chain().focus().toggleOrderedList().run(); break;
      case 'quote': editor.chain().focus().toggleBlockquote().run(); break;
      case 'code': editor.chain().focus().toggleCodeBlock().run(); break;
      case 'hr': editor.chain().focus().setHorizontalRule().run(); break;
      case 'bold': editor.chain().focus().toggleBold().run(); break;
      case 'italic': editor.chain().focus().toggleItalic().run(); break;
    }
    setSlashMenu(null);
    slashStartPosRef.current = null;
  }, [editor]);
  // Keep ref in sync so the stale-closure-safe handleKeyDown can call the latest version
  runSlashCommandRef.current = runSlashCommand;

  const filteredCmds = slashMenu
    ? SLASH_COMMANDS.filter((c) =>
        !slashMenu.query ||
        c.id.startsWith(slashMenu.query.toLowerCase()) ||
        c.label.includes(slashMenu.query)
      )
    : [];
  const scheduleOptions = getScheduleOptions();

  function startDrag(e: React.MouseEvent<HTMLDivElement>) {
    if (fullscreen || minimized || isMobile) return;
    const dialog = dialogRef.current;
    if (!dialog) return;
    const rect = dialog.getBoundingClientRect();
    // if no pos set yet, compute current position
    const curX = pos?.x ?? rect.left;
    const curY = pos?.y ?? rect.top;
    const offsetX = e.clientX - curX;
    const offsetY = e.clientY - curY;
    function onMove(ev: MouseEvent) {
      const nx = Math.max(0, Math.min(ev.clientX - offsetX, window.innerWidth - size.w));
      const ny = Math.max(0, Math.min(ev.clientY - offsetY, window.innerHeight - size.h));
      setPos({ x: nx, y: ny });
    }
    function onUp() {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
    }
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }

  function startResize(e: React.MouseEvent, dir: string) {
    e.preventDefault();
    e.stopPropagation();
    const dialog = dialogRef.current;
    if (!dialog) return;
    const rect = dialog.getBoundingClientRect();
    const startX = e.clientX, startY = e.clientY;
    const startW = rect.width, startH = rect.height;
    const startL = rect.left, startT = rect.top;
    function onMove(ev: MouseEvent) {
      let nw = startW, nh = startH;
      let nx = pos?.x ?? startL, ny = pos?.y ?? startT;
      if (dir.includes('e')) nw = Math.max(400, startW + ev.clientX - startX);
      if (dir.includes('s')) nh = Math.max(300, startH + ev.clientY - startY);
      if (dir.includes('w')) { nw = Math.max(400, startW - (ev.clientX - startX)); nx = startL + (startW - nw); }
      if (dir.includes('n')) { nh = Math.max(300, startH - (ev.clientY - startY)); ny = startT + (startH - nh); }
      setSize({ w: nw, h: nh });
      if (dir.includes('w') || dir.includes('n')) setPos({ x: nx, y: ny });
    }
    function onUp() {
      document.removeEventListener('mousemove', onMove);
      document.removeEventListener('mouseup', onUp);
      setSize((s) => {
        try { localStorage.setItem('webmail_compose_size', JSON.stringify(s)); } catch { /* */ }
        return s;
      });
    }
    document.addEventListener('mousemove', onMove);
    document.addEventListener('mouseup', onUp);
  }

  function getScheduleOptions(): { label: string; sub: string; date: Date }[] {
    const now = new Date();
    const tomorrow = new Date(now);
    tomorrow.setDate(tomorrow.getDate() + 1);
    const tomorrowMorning = new Date(tomorrow); tomorrowMorning.setHours(8, 0, 0, 0);
    const tomorrowAfternoon = new Date(tomorrow); tomorrowAfternoon.setHours(13, 0, 0, 0);
    // next Monday
    const nextMonday = new Date(now);
    const day = now.getDay(); // 0=Sun, 1=Mon...
    const daysUntilMonday = day === 0 ? 1 : (8 - day);
    nextMonday.setDate(now.getDate() + daysUntilMonday);
    nextMonday.setHours(8, 0, 0, 0);
    const fmt = (d: Date) => new Intl.DateTimeFormat('ko-KR', { month: 'numeric', day: 'numeric', hour: 'numeric', minute: '2-digit', hour12: true }).format(d);
    const dayFmt = (d: Date) => new Intl.DateTimeFormat('ko-KR', { weekday: 'short' }).format(d);
    return [
      { label: '내일 아침', sub: fmt(tomorrowMorning), date: tomorrowMorning },
      { label: '내일 오후', sub: fmt(tomorrowAfternoon), date: tomorrowAfternoon },
      { label: `${dayFmt(nextMonday)}요일 오전`, sub: fmt(nextMonday), date: nextMonday },
    ];
  }

  return (
    <>
      <div aria-hidden="true" style={{ position: 'fixed', inset: 0, zIndex: 99, pointerEvents: 'none' }} />

      <div
        ref={dialogRef}
        role="dialog"
        aria-label="새 메시지 작성"
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
              파일을 여기에 놓으세요
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
        <div
          onClick={minimized ? () => setMinimized(false) : undefined}
          onMouseDown={startDrag}
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: '10px 16px',
            borderBottom: minimized ? 'none' : '1px solid var(--color-border-subtle)',
            background: 'var(--color-bg-secondary)',
            borderRadius: minimized ? '8px' : '8px 8px 0 0',
            cursor: minimized ? 'pointer' : (fullscreen || isMobile ? 'default' : 'move'),
            flexShrink: 0,
          }}
        >
          <span style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', flex: 1, minWidth: 0 }}>
            {minimized && subject ? subject : (intent === 'reply' || intent === 'reply_all' ? '답장' : intent === 'forward' ? '전달' : '새 메시지')}
          </span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flexShrink: 0, marginLeft: '8px' }}>
            {!isMobile && <>
            <button
              onClick={(e) => { e.stopPropagation(); setFullscreen((v) => !v); if (minimized) setMinimized(false); }}
              aria-label={fullscreen ? '창 축소' : '전체화면'}
              title={fullscreen ? '창 축소' : '전체화면'}
              style={{
                width: '24px', height: '24px', borderRadius: '4px', border: 'none',
                background: 'transparent', color: 'var(--color-text-secondary)',
                cursor: 'pointer', fontSize: '12px', lineHeight: 1,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >{fullscreen ? '⊡' : '⊞'}</button>
            <button
              onClick={(e) => { e.stopPropagation(); setMinimized((v) => !v); }}
              aria-label={minimized ? '창 복원' : '창 최소화'}
              style={{
                width: '24px', height: '24px', borderRadius: '4px', border: 'none',
                background: 'transparent', color: 'var(--color-text-secondary)',
                cursor: 'pointer', fontSize: '14px', lineHeight: 1,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >{minimized ? '□' : '─'}</button>
            </>}
            <button
              onClick={() => {
                const hasContent = !sent && (to.trim() || subject.trim() || (editor && editor.getText().trim()));
                if (hasContent) setConfirmClose(true); else onClose();
              }}
              aria-label="창 닫기"
              style={{
                width: '24px', height: '24px', borderRadius: '4px', border: 'none',
                background: 'transparent', color: 'var(--color-text-secondary)',
                cursor: 'pointer', fontSize: isMobile ? '20px' : '16px', lineHeight: 1,
                display: 'flex', alignItems: 'center', justifyContent: 'center',
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >{isMobile ? '←' : '×'}</button>
          </div>
        </div>

        {/* Close confirmation panel */}
        {confirmClose && (
          <div
            role="alertdialog"
            aria-modal="false"
            aria-labelledby="compose-close-save-title"
            aria-busy={closeSaveInProgress}
            onKeyDown={(e) => {
              if (e.key === 'Escape') {
                e.stopPropagation();
                if (closeSaveInProgress) return;
                cancelCloseConfirmation();
              }
            }}
            style={{ padding: '10px 16px', borderBottom: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)', display: 'flex', alignItems: 'center', gap: '8px', flexShrink: 0 }}
          >
            <span id="compose-close-save-title" style={{ fontSize: '13px', color: 'var(--color-text-primary)', flex: 1 }}>{closeSavePrompt}</span>
            <button
              type="button"
              aria-label={closeSaveButtonAriaLabel}
              disabled={closeSaveInProgress}
              onClick={() => { void saveDraftAndClose(); }}
              style={{ fontSize: '12px', padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-primary)', cursor: closeSaveInProgress ? 'not-allowed' : 'pointer' }}
            >{closeSaveButtonLabel}</button>
            <button
              type="button"
              aria-label="저장하지 않고 작성창 닫기"
              disabled={closeSaveInProgress}
              onClick={discardDraftAndClose}
              style={{ fontSize: '12px', padding: '4px 10px', borderRadius: '5px', border: '1px solid rgba(217,79,61,0.4)', background: 'transparent', color: 'var(--color-destructive)', cursor: closeSaveInProgress ? 'not-allowed' : 'pointer' }}
            >버리기</button>
            <button
              type="button"
              aria-label="닫기 취소하고 작성 계속하기"
              disabled={closeSaveInProgress}
              onClick={cancelCloseConfirmation}
              style={{ fontSize: '12px', padding: '4px 10px', borderRadius: '5px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', cursor: closeSaveInProgress ? 'not-allowed' : 'pointer' }}
            >취소</button>
          </div>
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

          {/* From */}
          {(userEmail || availableAddresses.length > 0) && (
            <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '6px 16px', gap: '8px', flexShrink: 0, background: 'var(--color-bg-secondary)' }}>
              <span style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', flexShrink: 0 }}>보내는 사람</span>
              {availableAddresses.length > 1 ? (
                <select
                  value={fromAddress}
                  onChange={(e) => setFromAddress(e.target.value)}
                  style={{ fontSize: '13px', color: 'var(--color-text-secondary)', background: 'transparent', border: 'none', outline: 'none', cursor: 'pointer', flex: 1 }}
                >
                  {availableAddresses.map((a) => (
                    <option key={a.id} value={a.address}>{a.address}</option>
                  ))}
                </select>
              ) : (
                <span style={{ fontSize: '13px', color: 'var(--color-text-secondary)' }}>{fromAddress || userEmail}</span>
              )}
            </div>
          )}

          {/* To */}
          <div style={{ display: 'flex', alignItems: 'center', borderBottom: `1px solid ${error.includes('받는 사람') ? 'var(--color-destructive)' : 'var(--color-border-subtle)'}`, padding: '0 16px', flexShrink: 0 }}>
            <label htmlFor="compose-to" style={{ fontSize: '13px', color: error.includes('받는 사람') ? 'var(--color-destructive)' : 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>받는 사람</label>
            <RecipientChips
              id="compose-to"
              value={to}
              onChange={(v) => { setTo(v); toRef.current = v; if (error) setError(''); triggerAutoSave(v, ccRef.current, bccRef.current, subjectRef.current, editor?.getText() ?? '', editor?.getHTML() ?? ''); }}
              placeholder="example@domain.com"
              autoFocus
              hasError={error.includes('받는 사람')}
              suggestions={recentRecipients}
            />
            <div style={{ display: 'flex', gap: '4px', flexShrink: 0, marginLeft: '4px' }}>
              <button type="button" onClick={() => setShowOrgPicker(true)} title="조직도에서 선택"
                style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}>
                <UsersIcon style={{ width: '15px', height: '15px' }} />
              </button>
              <button type="button"
                onClick={() => { setShowCc(v => !v); if (showCc) { setCc(''); ccRef.current = ''; } }}
                style={{ fontSize: '12px', color: showCc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)', background: 'none', border: 'none', cursor: 'pointer', padding: '2px 6px', borderRadius: '4px', fontWeight: 500 }}
                onMouseEnter={(e) => { (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
                onMouseLeave={(e) => { (e.currentTarget).style.color = showCc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)'; }}
              >Cc</button>
              <button type="button"
                onClick={() => { setShowBcc(v => !v); if (showBcc) { setBcc(''); bccRef.current = ''; } }}
                style={{ fontSize: '12px', color: showBcc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)', background: 'none', border: 'none', cursor: 'pointer', padding: '2px 6px', borderRadius: '4px', fontWeight: 500 }}
                onMouseEnter={(e) => { (e.currentTarget).style.color = 'var(--color-text-primary)'; }}
                onMouseLeave={(e) => { (e.currentTarget).style.color = showBcc ? 'var(--color-text-primary)' : 'var(--color-text-tertiary)'; }}
              >Bcc</button>
            </div>
          </div>

          {/* CC — only when toggled */}
          {showCc && (
            <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px', flexShrink: 0 }}>
              <label htmlFor="compose-cc" style={{ fontSize: '13px', color: 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>Cc</label>
              <RecipientChips
                id="compose-cc"
                value={cc}
                onChange={(v) => { setCc(v); ccRef.current = v; triggerAutoSave(toRef.current, v, bccRef.current, subjectRef.current, editor?.getText() ?? '', editor?.getHTML() ?? ''); }}
                placeholder="example@domain.com, ..."
                suggestions={recentRecipients}
              />
              <button type="button" onClick={() => setShowOrgPicker(true)} title="조직도에서 선택"
                style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}>
                <UsersIcon style={{ width: '15px', height: '15px' }} />
              </button>
              <button type="button" onClick={() => { setShowCc(false); setCc(''); ccRef.current = ''; }} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}><XMarkIcon style={{ width: '13px', height: '13px' }} /></button>
            </div>
          )}

          {/* BCC — only when toggled */}
          {showBcc && (
            <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px', flexShrink: 0 }}>
              <label htmlFor="compose-bcc" style={{ fontSize: '13px', color: 'var(--color-text-secondary)', flexShrink: 0, paddingRight: '8px' }}>Bcc</label>
              <RecipientChips
                id="compose-bcc"
                value={bcc}
                onChange={(v) => { setBcc(v); bccRef.current = v; triggerAutoSave(toRef.current, ccRef.current, v, subjectRef.current, editor?.getText() ?? '', editor?.getHTML() ?? ''); }}
                placeholder="example@domain.com, ..."
                suggestions={recentRecipients}
              />
              <button type="button" onClick={() => setShowOrgPicker(true)} title="조직도에서 선택"
                style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}>
                <UsersIcon style={{ width: '15px', height: '15px' }} />
              </button>
              <button type="button" onClick={() => { setShowBcc(false); setBcc(''); bccRef.current = ''; }} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px 4px', display: 'inline-flex', flexShrink: 0 }}><XMarkIcon style={{ width: '13px', height: '13px' }} /></button>
            </div>
          )}

          {/* Subject */}
          <div style={{ display: 'flex', alignItems: 'center', borderBottom: '1px solid var(--color-border-subtle)', padding: '0 16px', flexShrink: 0 }}>
            <input
              id="compose-subject"
              type="text"
              value={subject}
              onChange={(e) => { setSubject(e.target.value); subjectRef.current = e.target.value; triggerAutoSave(toRef.current, ccRef.current, bccRef.current, e.target.value, editor?.getText() ?? '', editor?.getHTML() ?? ''); }}
              placeholder="제목"
              style={{ flex: 1, padding: '10px 0', border: 'none', outline: 'none', fontSize: '14px', background: 'transparent', color: 'var(--color-text-primary)', fontWeight: 500 }}
            />
          </div>

          {/* TipTap editor body */}
          <div style={{ flex: 1, overflowY: 'auto', minHeight: 0 }}>
            <EditorContent editor={editor} />
          </div>

          {/* Signature editor */}
          {showSigEditor && (
            <div style={{ padding: '8px 16px', borderTop: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)' }}>
              <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginBottom: '4px', fontWeight: 600, textTransform: 'uppercase', letterSpacing: '0.05em' }}>서명</div>
              <textarea
                value={signature}
                onChange={(e) => setSignature(e.target.value)}
                onBlur={() => { try { localStorage.setItem('webmail_signature', signature); } catch { /* ignore */ } }}
                placeholder="서명을 입력하세요..."
                rows={3}
                style={{ width: '100%', padding: '6px 8px', borderRadius: '4px', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', fontSize: '13px', resize: 'vertical', outline: 'none', boxSizing: 'border-box', fontFamily: 'inherit' }}
              />
              <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>변경 사항은 다음 메시지 작성 시 적용됩니다</div>
            </div>
          )}

          {/* Attachment chips */}
          {uploadedAttachments.length > 0 && (
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px', padding: '6px 16px', borderTop: '1px solid var(--color-border-subtle)' }}>
              {uploadedAttachments.map((att) => {
                const kb = att.size < 1024 * 1024 ? `${Math.round(att.size / 1024)} KB` : `${(att.size / 1024 / 1024).toFixed(1)} MB`;
                return (
                  <div key={att.id} style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', padding: '3px 8px', borderRadius: '12px', border: `1px solid ${att.error ? 'rgba(217,79,61,0.4)' : 'var(--color-border-default)'}`, background: 'var(--color-bg-secondary)', fontSize: '12px', color: att.error ? 'var(--color-destructive)' : 'var(--color-text-primary)' }}>
                    <span style={{ display: 'inline-flex', alignItems: 'center' }}>{att.uploading ? <ArrowPathIcon style={{ width: '12px', height: '12px', animation: 'spin 1s linear infinite' }} /> : att.error ? <ExclamationTriangleIcon style={{ width: '12px', height: '12px' }} /> : <PaperClipIcon style={{ width: '12px', height: '12px' }} />}</span>
                    <span style={{ maxWidth: '160px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{att.filename}</span>
                    {!att.uploading && <span style={{ color: 'var(--color-text-tertiary)' }}>{kb}</span>}
                    {att.error && att.file && (
                      <button
                        type="button"
                        onClick={() => retryAttachmentUpload(att.id)}
                        style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-accent)', lineHeight: 1, padding: '0 2px', fontSize: '11px', fontWeight: 600 }}
                      >재시도</button>
                    )}
                    <button
                      type="button"
                      onClick={() => setUploadedAttachments((prev) => prev.filter((a) => a.id !== att.id))}
                      style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', lineHeight: 1, padding: '0 2px', display: 'inline-flex' }}
                    ><XMarkIcon style={{ width: '12px', height: '12px' }} /></button>
                  </div>
                );
              })}
            </div>
          )}

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
            pendingMsgRef={pendingMsgRef}
            pendingDraftSendRef={pendingDraftSendRef}
            sendAndArchiveRef={sendAndArchiveRef}
            scheduledAt={scheduledAt}
            setScheduledAt={setScheduledAt}
            setShowSchedule={setShowSchedule}
            scheduleOptions={scheduleOptions}
            handleSend={handleSend}
            closeSendDropdown={closeSendDropdown}
            onArchiveSource={onArchiveSource}
            setSendCountdown={setSendCountdown}
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
      {imageResizeToolbar && editor?.isActive('image') && (
        <div
          style={{
            position: 'fixed',
            top: imageResizeToolbar.top,
            left: imageResizeToolbar.left,
            zIndex: 500,
            display: 'flex',
            gap: '2px',
            background: 'var(--color-bg-primary)',
            border: '1px solid var(--color-border-default)',
            borderRadius: '6px',
            boxShadow: '0 4px 16px rgba(0,0,0,0.16)',
            padding: '3px',
          }}
        >
          {([['소', '25%'], ['중', '50%'], ['대', '75%'], ['원본', '100%']] as const).map(([label, pct]) => (
            <button
              key={label}
              type="button"
              onMouseDown={(e) => {
                e.preventDefault();
                editor.chain().focus().updateAttributes('image', { style: `width: ${pct}` }).run();
              }}
              style={{
                padding: '2px 8px',
                fontSize: '11px',
                fontWeight: 500,
                borderRadius: '4px',
                border: 'none',
                background: 'transparent',
                color: 'var(--color-text-secondary)',
                cursor: 'pointer',
              }}
              onMouseEnter={(e) => { (e.currentTarget).style.background = 'var(--color-bg-tertiary)'; }}
              onMouseLeave={(e) => { (e.currentTarget).style.background = 'transparent'; }}
            >
              {label}
            </button>
          ))}
        </div>
      )}
    </>
  );
}
