'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { ClipboardEvent } from 'react';
import {
  deleteDMMessage,
  editDMMessage,
  listDMMedia,
  listDMMessages,
  markDMRead,
  searchDMMessages,
  sendDMMessage,
  toggleDMReaction,
  uploadDMAttachment,
  type DMMediaItem,
  type DMMessage,
} from '@/lib/api';
import { ignoreNonCritical } from '@/lib/promise';
import { mergeMessage, readDMDrafts, writeDMDrafts, type DMDraft, type MediaTab } from './dmUtils';

export interface UseDMMessagesParams {
  activeRoomId: string;
  loadRooms: () => Promise<void>;
  mediaTab: MediaTab;
  setMediaItems: (items: DMMediaItem[]) => void;
  setNotice: (notice: string) => void;
  setImageMenu: (menu: null) => void;
  setReactionPickerMessageId: (id: string | null) => void;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
  setError: (error: string) => void;
}

export function useDMMessages({
  activeRoomId,
  loadRooms,
  mediaTab,
  setMediaItems,
  setNotice,
  setImageMenu,
  setReactionPickerMessageId,
  t,
  setError,
}: UseDMMessagesParams) {
  const [messages, setMessages] = useState<DMMessage[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<DMMessage[]>([]);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingBody, setEditingBody] = useState('');
  const [composer, setComposer] = useState('');
  const [driveFileId, setDriveFileId] = useState('');
  const [driveComposerOpen, setDriveComposerOpen] = useState(false);
  const [pendingPasteFile, setPendingPasteFile] = useState<File | null>(null);
  const [pendingPastePreview, setPendingPastePreview] = useState('');

  const messageEndRef = useRef<HTMLDivElement | null>(null);
  const sendingRef = useRef(false);
  const composerComposingRef = useRef(false);
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const draftsRef = useRef<Record<string, DMDraft>>(readDMDrafts());

  const persistDraft = useCallback((roomId: string, body: string, drive: string) => {
    if (!roomId) return;
    const next = { ...draftsRef.current };
    if (body.trim() || drive.trim()) { next[roomId] = { body, driveFileId: drive }; }
    else { delete next[roomId]; }
    draftsRef.current = next;
    writeDMDrafts(next);
  }, []);

  const loadMessages = useCallback(async () => {
    if (!activeRoomId) return;
    setLoadingMessages(true);
    try {
      const next = await listDMMessages(activeRoomId, { limit: 80 });
      const sorted = [...next].sort((a, b) => Date.parse(a.created_at) - Date.parse(b.created_at));
      setMessages(sorted);
      const last = sorted[sorted.length - 1];
      if (last) {
        ignoreNonCritical(markDMRead(activeRoomId, last.id).then(loadRooms), 'dm.messages.markRead');
      }
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.messagesUnavailable'));
    } finally { setLoadingMessages(false); }
  }, [activeRoomId, loadRooms, t, setError]);

  useEffect(() => { void loadMessages(); }, [loadMessages]);
  useEffect(() => {
    const id = window.setInterval(() => { if (document.visibilityState === 'visible') void loadMessages(); }, 3000);
    return () => window.clearInterval(id);
  }, [loadMessages]);

  useEffect(() => {
    if (!pendingPasteFile) { setPendingPastePreview(''); return; }
    const url = URL.createObjectURL(pendingPasteFile);
    setPendingPastePreview(url);
    return () => URL.revokeObjectURL(url);
  }, [pendingPasteFile]);

  useEffect(() => {
    if (!activeRoomId || !searchQuery.trim()) { setSearchResults([]); return; }
    const id = window.setTimeout(() => {
      void searchDMMessages(activeRoomId, searchQuery.trim(), undefined, 20)
        .then((results) => setSearchResults(results.map((r) => r.message)))
        .catch(() => setSearchResults([]));
    }, 250);
    return () => window.clearTimeout(id);
  }, [activeRoomId, searchQuery]);

  useEffect(() => { messageEndRef.current?.scrollIntoView({ block: 'end' }); }, [messages.length, activeRoomId]);

  useEffect(() => {
    setDriveComposerOpen(false);
    const draft = activeRoomId ? draftsRef.current[activeRoomId] : undefined;
    setComposer(draft?.body ?? '');
    setDriveFileId(draft?.driveFileId ?? '');
  }, [activeRoomId]);

  const send = useCallback(async () => {
    if (!activeRoomId || (!composer.trim() && !driveFileId.trim())) return;
    if (composerComposingRef.current || sendingRef.current) return;
    const body = composer.trim(); const drive = driveFileId.trim();
    sendingRef.current = true; setComposer(''); setDriveFileId(''); persistDraft(activeRoomId, '', '');
    try {
      const sent = await sendDMMessage(activeRoomId, body, drive || undefined);
      setMessages((prev) => mergeMessage(prev, sent)); void loadRooms();
    } catch (err) {
      setComposer(body); setDriveFileId(drive); persistDraft(activeRoomId, body, drive);
      setError(err instanceof Error ? err.message : t('errors.sendFailed'));
    } finally { sendingRef.current = false; }
  }, [activeRoomId, composer, driveFileId, loadRooms, persistDraft, t, setError]);

  const uploadFile = useCallback(async (file: File) => {
    if (!activeRoomId) return;
    try {
      const msg = await uploadDMAttachment(activeRoomId, file);
      setMessages((prev) => mergeMessage(prev, msg)); void loadRooms();
      ignoreNonCritical(listDMMedia(activeRoomId, mediaTab).then(setMediaItems), 'dm.media.refresh');
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.uploadFailed')); }
  }, [activeRoomId, loadRooms, mediaTab, setMediaItems, t, setError]);

  const uploadPastedImages = useCallback((event: ClipboardEvent<HTMLInputElement>) => {
    const files: File[] = [];
    for (const item of Array.from(event.clipboardData.items)) {
      if (!item.type.startsWith('image/')) continue;
      const file = item.getAsFile();
      if (file) files.push(new File([file], file.name || `clipboard-${Date.now()}.png`, { type: file.type || item.type }));
    }
    if (files.length === 0) return;
    event.preventDefault(); setPendingPasteFile(files[0]);
  }, []);

  const confirmPendingPaste = useCallback(() => {
    if (!pendingPasteFile) return;
    const file = pendingPasteFile; setPendingPasteFile(null); void uploadFile(file);
  }, [pendingPasteFile, uploadFile]);

  const copyImageToClipboard = useCallback(async (message: DMMessage) => {
    if (!message.attachment_download_url) return;
    try {
      const response = await fetch(message.attachment_download_url);
      if (!response.ok) throw new Error(`image fetch failed: ${response.status}`);
      const blob = await response.blob();
      await navigator.clipboard.write([new ClipboardItem({ [blob.type || 'image/png']: blob })]);
      setNotice(t('imageCopied')); window.setTimeout(() => setNotice(''), 1800);
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.copyFailed')); }
    finally { setImageMenu(null); }
  }, [t, setNotice, setError, setImageMenu]);

  const submitEdit = useCallback(async () => {
    if (!editingId || !editingBody.trim()) return;
    try {
      const msg = await editDMMessage(editingId, editingBody);
      setMessages((prev) => mergeMessage(prev, msg)); setEditingId(null); setEditingBody('');
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.editFailed')); }
  }, [editingBody, editingId, t, setError]);

  const removeMessage = useCallback(async (messageId: string) => {
    try {
      const msg = await deleteDMMessage(messageId);
      setMessages((prev) => mergeMessage(prev, msg));
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.deleteFailed')); }
  }, [t, setError]);

  const toggleReaction = useCallback(async (messageId: string, emoji: string) => {
    try {
      await toggleDMReaction(messageId, emoji); void loadMessages(); setReactionPickerMessageId(null);
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.reactionFailed')); }
    finally { setReactionPickerMessageId(null); }
  }, [loadMessages, t, setError, setReactionPickerMessageId]);

  return {
    messages, setMessages,
    searchQuery, setSearchQuery,
    searchResults,
    loadingMessages,
    editingId, setEditingId,
    editingBody, setEditingBody,
    composer, setComposer,
    driveFileId, setDriveFileId,
    driveComposerOpen, setDriveComposerOpen,
    pendingPasteFile, setPendingPasteFile,
    pendingPastePreview,
    // refs
    messageEndRef,
    sendingRef,
    composerComposingRef,
    fileInputRef,
    draftsRef,
    // callbacks
    persistDraft,
    loadMessages,
    send,
    uploadFile,
    uploadPastedImages,
    confirmPendingPaste,
    copyImageToClipboard,
    submitEdit,
    removeMessage,
    toggleReaction,
  };
}
