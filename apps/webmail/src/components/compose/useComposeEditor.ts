'use client';
import React, { useCallback, useEffect, useRef } from 'react';
import { useEditor } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import Link from '@tiptap/extension-link';
import Underline from '@tiptap/extension-underline';
import TextAlign from '@tiptap/extension-text-align';
import Placeholder from '@tiptap/extension-placeholder';
import Image from '@tiptap/extension-image';
import { uploadAttachment } from '@/lib/api';
import type { UIComposeIntent, MessageDetail } from '@/lib/api';
import { ignoreNonCritical } from '@/lib/promise';
import type { UploadedAttachment } from './ComposeAttachmentPanel';
import { escapeHtml } from '@/lib/compose/composeUtils';
import { buildQuoteHTML } from '@/lib/mail-address';
import { SLASH_COMMANDS } from '@/lib/compose/slashCommands';
import type { SlashCommand } from '@/lib/compose/slashCommands';
import type { Editor } from '@tiptap/react';

interface UseComposeEditorParams {
  intent: UIComposeIntent;
  sourceMessage?: MessageDetail;
  draftMessage?: MessageDetail;
  initialBody?: string;
  signature: string;
  // Tiptap placeholder text
  bodyPlaceholder: string;
  bodyAria: string;
  // Slash command deps
  slashMenuRef: React.MutableRefObject<{ query: string; top: number; cursorTop: number; left: number } | null>;
  setSlashMenu: (v: { query: string; top: number; cursorTop: number; left: number } | null) => void;
  setSlashIndex: React.Dispatch<React.SetStateAction<number>>;
  slashIndexRef: React.MutableRefObject<number>;
  slashStartPosRef: React.MutableRefObject<number | null>;
  runSlashCommandRef: React.MutableRefObject<((cmd: SlashCommand) => void) | null>;
  runSlashCommandBase: (cmd: SlashCommand, editor: Editor | null) => void;
  // Image resize
  setImageResizeToolbar: (v: { top: number; left: number } | null) => void;
  // Auto-save
  toRef: React.MutableRefObject<string>;
  ccRef: React.MutableRefObject<string>;
  bccRef: React.MutableRefObject<string>;
  subjectRef: React.MutableRefObject<string>;
  triggerAutoSave: (to: string, cc: string, bcc: string, subject: string, text: string, html: string) => void;
  // Attachments
  draftIdRef: React.MutableRefObject<string | null>;
  setUploadedAttachments: React.Dispatch<React.SetStateAction<UploadedAttachment[]>>;
  // Send countdown attachment guard
  sendCountdown: number | null;
  uploadedAttachments: UploadedAttachment[];
  readyAttachmentIds: () => string[];
  setSendCountdown: (v: number | null) => void;
  pendingMsgRef: React.MutableRefObject<unknown>;
  pendingDraftSendRef: React.MutableRefObject<boolean>;
  setError: (v: string) => void;
  errAttachmentChanged: string;
}

export function useComposeEditor(params: UseComposeEditorParams) {
  const {
    intent,
    sourceMessage,
    draftMessage,
    initialBody,
    signature,
    bodyPlaceholder,
    bodyAria,
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
    errAttachmentChanged,
  } = params;

  const imageInputRef = useRef<HTMLInputElement>(null);

  // Build initialContent
  const sigHTML = signature.trim()
    ? `<p></p><p>--</p><p>${signature.trim().split('\n').map((l: string) => escapeHtml(l)).join('</p><p>')}</p>`
    : '';

  const quoteOnReply = (() => {
    try {
      return (JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as { quoteOnReply?: boolean }).quoteOnReply !== false;
    } catch {
      return true;
    }
  })();

  const initialContent = draftMessage
    ? (draftMessage.html_body ?? (draftMessage.text_body
        ? draftMessage.text_body.split('\n').map((l: string) => `<p>${escapeHtml(l) || '&nbsp;'}</p>`).join('')
        : ''))
    : (sourceMessage && (intent === 'reply' || intent === 'reply_all' || intent === 'forward')
        ? `<p></p>${sigHTML ? sigHTML + '<p></p>' : ''}${quoteOnReply ? buildQuoteHTML(intent, sourceMessage) : ''}`
        : initialBody
        ? `${initialBody.split('\n').map((l: string) => `<p>${escapeHtml(l) || '&nbsp;'}</p>`).join('')}<p></p>${sigHTML}`
        : `<p></p>${sigHTML}`);

  const editor = useEditor({
    extensions: [
      StarterKit,
      Underline,
      Link.configure({ openOnClick: false }),
      TextAlign.configure({ types: ['heading', 'paragraph'] }),
      Placeholder.configure({ placeholder: bodyPlaceholder }),
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
        'aria-label': bodyAria,
        role: 'textbox',
        'aria-multiline': 'true',
      },
      handleKeyDown: (_view: unknown, event: KeyboardEvent) => {
        const menu = slashMenuRef.current;
        if (!menu) return false;
        if (event.key === 'ArrowDown') {
          event.preventDefault();
          setSlashIndex((i: number) => {
            const cmds = SLASH_COMMANDS.filter((c) =>
              !menu.query || c.id.startsWith(menu.query.toLowerCase()) || c.label.includes(menu.query)
            );
            return Math.min(i + 1, cmds.length - 1);
          });
          return true;
        }
        if (event.key === 'ArrowUp') {
          event.preventDefault();
          setSlashIndex((i: number) => Math.max(i - 1, 0));
          return true;
        }
        if (event.key === 'Enter') {
          const cmds = SLASH_COMMANDS.filter((c) =>
            !menu.query || c.id.startsWith(menu.query.toLowerCase()) || c.label.includes(menu.query)
          );
          const cmd = cmds[slashIndexRef.current];
          if (cmd) {
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
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [editor]);

  // Attachment change guard during send countdown
  useEffect(() => {
    if (sendCountdown === null || sendCountdown <= 0 || !pendingMsgRef.current) return;

    const hasUnreadyAttachment = uploadedAttachments.some((attachment) => attachment.uploading || attachment.error);
    const currentAttachmentIds = readyAttachmentIds().slice().sort().join('\n');
    const pendingMsg = pendingMsgRef.current as { attachment_ids?: string[] };
    const pendingAttachmentIds = [...(pendingMsg.attachment_ids ?? [])].sort().join('\n');

    if (hasUnreadyAttachment || currentAttachmentIds !== pendingAttachmentIds) {
      setSendCountdown(null);
      pendingMsgRef.current = null;
      pendingDraftSendRef.current = false;
      setError(errAttachmentChanged);
    }
  }, [sendCountdown, uploadedAttachments, readyAttachmentIds, setSendCountdown, pendingMsgRef, pendingDraftSendRef, setError, errAttachmentChanged]);

  const handleImageFileSelect = useCallback(async (file: File) => {
    if (!editor) return;
    let src: string;
    if (file.size < 500 * 1024) {
      src = await new Promise<string>((resolve, reject) => {
        const reader = new FileReader();
        reader.onload = () => resolve(reader.result as string);
        reader.onerror = reject;
        reader.readAsDataURL(file);
      });
    } else {
      const objectUrl = URL.createObjectURL(file);
      ignoreNonCritical(
        uploadAttachment(file, draftIdRef.current || undefined).then((att) => {
          setUploadedAttachments((prev) => [...prev, { id: att.id, filename: att.filename, size: att.size }]);
        }),
        'compose.inlineImage.upload',
      );
      src = objectUrl;
    }
    editor.chain().focus().setImage({ src, alt: file.name }).run();
  }, [editor, draftIdRef, setUploadedAttachments]);

  const runSlashCommand = useCallback((cmd: SlashCommand) => {
    runSlashCommandBase(cmd, editor ?? null);
  }, [runSlashCommandBase, editor]);

  // Keep ref in sync so the stale-closure-safe handleKeyDown can call the latest version
  runSlashCommandRef.current = runSlashCommand;

  return { editor, imageInputRef, initialContent, handleImageFileSelect, runSlashCommand };
}
