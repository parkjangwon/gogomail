'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ClipboardEvent, KeyboardEvent } from 'react';
import { useTranslations } from 'next-intl';
import {
  addDMMembers,
  createDMInvite,
  createDMRoom,
  deleteDMMessage,
  editDMMessage,
  listDMMedia,
  listDMMessages,
  listDMRooms,
  listDirectoryUsers,
  listOrgTree,
  listPublicDMRooms,
  markDMRead,
  removeDMMember,
  searchDMMessages,
  sendDMMessage,
  toggleDMReaction,
  transferDMOwner,
  uploadDMAttachment,
  type DMMediaItem,
  type DMMessage,
  type DMRoom,
  type DMUser,
  type DirectoryUser,
} from '@/lib/api';
import { useWebmailAvatar } from '@/lib/webmailAvatar';

type MediaTab = 'files' | 'links' | 'drive';
type DMDraft = { body: string; driveFileId: string };

const DEV_CURRENT_USER_ID = process.env.NEXT_PUBLIC_GOGOMAIL_DEV_USER_ID ?? '';
const DM_DRAFT_STORAGE_KEY = 'webmail_dm_drafts_v1';

function matchesDirectoryUser(user: DirectoryUser, query: string): boolean {
  if (!query) return true;
  const needle = query.toLowerCase();
  return [user.display_name, user.email, user.org_unit_name]
    .some((value) => (value ?? '').toLowerCase().includes(needle));
}

function messagePreview(message: DMMessage | undefined, labels: { deleted: string; file: string; drive: string }): string {
  if (!message) return '';
  if (message.deleted_at) return labels.deleted;
  if (message.message_type === 'file') return message.attachment_name || message.body || labels.file;
  if (message.message_type === 'drive_link') return message.body || message.drive_file_id || labels.drive;
  return message.body;
}

function roomTitle(
  room: DMRoom,
  currentUserId: string,
  labels: { direct: string; group: string; groupOthers: (name: string, count: number) => string },
): string {
  const otherNames = (room.members ?? [])
    .filter((member) => member.id !== currentUserId)
    .map((member) => member.display_name || member.id)
    .filter(Boolean);
  if (room.room_type === 'direct') return otherNames[0] || room.name?.trim() || labels.direct;
  if (room.name?.trim()) return room.name;
  if (otherNames.length > 1) return labels.groupOthers(otherNames[0], otherNames.length - 1);
  return otherNames[0] || labels.group;
}

function mergeMessage(existing: DMMessage[], next: DMMessage): DMMessage[] {
  const index = existing.findIndex((m) => m.id === next.id);
  if (index === -1) return [...existing, next].sort((a, b) => Date.parse(a.created_at) - Date.parse(b.created_at));
  const merged = [...existing];
  merged[index] = next;
  return merged;
}

function readDMDrafts(): Record<string, DMDraft> {
  try {
    if (typeof window === 'undefined') return {};
    const parsed = JSON.parse(localStorage.getItem(DM_DRAFT_STORAGE_KEY) ?? '{}') as Record<string, DMDraft>;
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch { return {}; }
}

function writeDMDrafts(drafts: Record<string, DMDraft>) {
  try {
    if (typeof window === 'undefined') return;
    localStorage.setItem(DM_DRAFT_STORAGE_KEY, JSON.stringify(drafts));
  } catch { /* best-effort */ }
}

type UseDMPanelOptions = {
  onUnreadChange?: (count: number) => void;
};

export function useDMPanel({ onUnreadChange }: UseDMPanelOptions) {
  const t = useTranslations('dmPanel');
  const selfAvatarUrl = useWebmailAvatar();
  const [rooms, setRooms] = useState<DMRoom[]>([]);
  const [publicRooms, setPublicRooms] = useState<DMRoom[]>([]);
  const [activeRoomId, setActiveRoomId] = useState<string>('');
  const [messages, setMessages] = useState<DMMessage[]>([]);
  const [directoryQuery, setDirectoryQuery] = useState('');
  const [directoryUsers, setDirectoryUsers] = useState<DirectoryUser[]>([]);
  const [directoryActiveIndex, setDirectoryActiveIndex] = useState(0);
  const [selectedUsers, setSelectedUsers] = useState<DirectoryUser[]>([]);
  const [roomName, setRoomName] = useState('');
  const [roomType, setRoomType] = useState<'direct' | 'group'>('direct');
  const [visibility, setVisibility] = useState<'private' | 'public'>('private');
  const [composer, setComposer] = useState('');
  const [driveFileId, setDriveFileId] = useState('');
  const [searchQuery, setSearchQuery] = useState('');
  const [searchResults, setSearchResults] = useState<DMMessage[]>([]);
  const [mediaTab, setMediaTab] = useState<MediaTab>('files');
  const [mediaItems, setMediaItems] = useState<DMMediaItem[]>([]);
  const [inviteUrl, setInviteUrl] = useState('');
  const [memberInput, setMemberInput] = useState('');
  const [ownerInput, setOwnerInput] = useState('');
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editingBody, setEditingBody] = useState('');
  const [newChatOpen, setNewChatOpen] = useState(false);
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [driveComposerOpen, setDriveComposerOpen] = useState(false);
  const [reactionPickerMessageId, setReactionPickerMessageId] = useState<string | null>(null);
  const [previewImage, setPreviewImage] = useState<DMMessage | null>(null);
  const [imageMenu, setImageMenu] = useState<{ message: DMMessage; x: number; y: number } | null>(null);
  const [pendingPasteFile, setPendingPasteFile] = useState<File | null>(null);
  const [pendingPastePreview, setPendingPastePreview] = useState('');
  const [notice, setNotice] = useState('');
  const [loadingRooms, setLoadingRooms] = useState(false);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [error, setError] = useState('');
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const memberInputRef = useRef<HTMLInputElement | null>(null);
  const messageEndRef = useRef<HTMLDivElement | null>(null);
  const reactionPickerRef = useRef<HTMLSpanElement | null>(null);
  const draftsRef = useRef<Record<string, DMDraft>>(readDMDrafts());
  const composerComposingRef = useRef(false);
  const sendingRef = useRef(false);

  const activeRoom = rooms.find((room) => room.id === activeRoomId) ?? null;
  const unread = useMemo(() => rooms.reduce((sum, room) => sum + (room.unread_count ?? 0), 0), [rooms]);
  const currentUserId = activeRoom?.current_user_id || rooms.find((room) => room.current_user_id)?.current_user_id || DEV_CURRENT_USER_ID;
  const memberById = useMemo(() => {
    const map = new Map<string, DMUser>();
    for (const member of activeRoom?.members ?? []) map.set(member.id, member);
    return map;
  }, [activeRoom]);
  const previewLabels = useMemo(() => ({ deleted: t('deletedMessage'), file: t('file'), drive: t('drive') }), [t]);
  const mediaTabLabels = useMemo<Record<MediaTab, string>>(() => ({
    files: t('tabFiles'), links: t('tabLinks'), drive: t('tabDrive'),
  }), [t]);
  const titleForRoom = useCallback(
    (room: DMRoom) => roomTitle(room, currentUserId, {
      direct: t('directMessage'),
      group: t('group'),
      groupOthers: (name, count) => t('groupTitleOthers', { name, count }),
    }),
    [currentUserId, t],
  );
  const previewForMessage = useCallback((message?: DMMessage) => messagePreview(message, previewLabels), [previewLabels]);
  const persistDraft = useCallback((roomId: string, body: string, drive: string) => {
    if (!roomId) return;
    const next = { ...draftsRef.current };
    if (body.trim() || drive.trim()) { next[roomId] = { body, driveFileId: drive }; }
    else { delete next[roomId]; }
    draftsRef.current = next;
    writeDMDrafts(next);
  }, []);

  const loadRooms = useCallback(async () => {
    setLoadingRooms(true);
    try {
      const [joined, publicList] = await Promise.all([listDMRooms(), listPublicDMRooms()]);
      setRooms(joined);
      setPublicRooms(publicList);
      onUnreadChange?.(joined.reduce((sum, room) => sum + (room.unread_count ?? 0), 0));
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.unavailable'));
    } finally { setLoadingRooms(false); }
  }, [onUnreadChange, t]);

  const loadMessages = useCallback(async () => {
    if (!activeRoomId) return;
    setLoadingMessages(true);
    try {
      const next = await listDMMessages(activeRoomId, { limit: 80 });
      setMessages([...next].sort((a, b) => Date.parse(a.created_at) - Date.parse(b.created_at)));
      const last = next[next.length - 1];
      if (last) void markDMRead(activeRoomId, last.id).then(loadRooms).catch(() => {});
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.messagesUnavailable'));
    } finally { setLoadingMessages(false); }
  }, [activeRoomId, loadRooms, t]);

  useEffect(() => { void loadRooms(); }, [loadRooms]);
  useEffect(() => { void loadMessages(); }, [loadMessages]);
  useEffect(() => {
    const id = window.setInterval(() => { if (document.visibilityState === 'visible') void loadRooms(); }, 5000);
    return () => window.clearInterval(id);
  }, [loadRooms]);
  useEffect(() => {
    const id = window.setInterval(() => { if (document.visibilityState === 'visible') void loadMessages(); }, 3000);
    return () => window.clearInterval(id);
  }, [loadMessages]);

  useEffect(() => {
    if (!newChatOpen) { setDirectoryUsers([]); setDirectoryActiveIndex(0); return; }
    const id = window.setTimeout(() => {
      const query = directoryQuery.trim();
      void Promise.all([listDirectoryUsers(query || undefined, 30), listOrgTree()]).then(([users, orgUnits]) => {
        const byId = new Map<string, DirectoryUser>();
        for (const user of users) byId.set(user.id, user);
        for (const unit of orgUnits) {
          const unitMatches = unit.display_name.toLowerCase().includes(query.toLowerCase());
          for (const member of unit.members ?? []) {
            const candidate: DirectoryUser = {
              id: member.id, display_name: member.display_name, email: member.email,
              avatar_url: member.avatar_url, org_unit_name: unit.display_name,
            };
            if (!unitMatches && !matchesDirectoryUser(candidate, query)) continue;
            const existing = byId.get(candidate.id);
            byId.set(candidate.id, { ...candidate, ...existing, avatar_url: existing?.avatar_url || candidate.avatar_url, org_unit_name: existing?.org_unit_name || candidate.org_unit_name });
          }
        }
        setDirectoryUsers([...byId.values()].slice(0, 30));
        setDirectoryActiveIndex(0);
      });
    }, 180);
    return () => window.clearTimeout(id);
  }, [directoryQuery, newChatOpen]);

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

  useEffect(() => {
    if (!activeRoomId) { setMediaItems([]); return; }
    void listDMMedia(activeRoomId, mediaTab).then(setMediaItems).catch(() => setMediaItems([]));
  }, [activeRoomId, mediaTab]);

  useEffect(() => { messageEndRef.current?.scrollIntoView({ block: 'end' }); }, [messages.length, activeRoomId]);

  useEffect(() => {
    setReactionPickerMessageId(null);
    const draft = activeRoomId ? draftsRef.current[activeRoomId] : undefined;
    setComposer(draft?.body ?? '');
    setDriveFileId(draft?.driveFileId ?? '');
  }, [activeRoomId]);

  useEffect(() => {
    if (!reactionPickerMessageId) return;
    function closeOnOutsidePointer(event: MouseEvent) {
      const target = event.target;
      if (target instanceof Node && reactionPickerRef.current?.contains(target)) return;
      setReactionPickerMessageId(null);
    }
    document.addEventListener('mousedown', closeOnOutsidePointer);
    return () => document.removeEventListener('mousedown', closeOnOutsidePointer);
  }, [reactionPickerMessageId]);

  useEffect(() => {
    if (!imageMenu) return;
    const close = () => setImageMenu(null);
    const closeOnKey = (event: globalThis.KeyboardEvent) => { if (event.key !== 'Escape') return; event.stopPropagation(); setImageMenu(null); };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', closeOnKey, true);
    return () => { document.removeEventListener('mousedown', close); document.removeEventListener('keydown', closeOnKey, true); };
  }, [imageMenu]);

  const createRoom = useCallback(async () => {
    if (selectedUsers.length === 0) return;
    try {
      const room = await createDMRoom({
        room_type: roomType,
        user_ids: roomType === 'direct' ? [selectedUsers[0].id] : selectedUsers.map((u) => u.id),
        name: roomType === 'group' ? roomName.trim() : undefined,
        visibility: roomType === 'group' ? visibility : undefined,
      });
      setRooms((prev) => [room, ...prev.filter((item) => item.id !== room.id)]);
      setActiveRoomId(room.id);
      setSelectedUsers([]); setRoomName(''); setDirectoryQuery(''); setNewChatOpen(false); setError('');
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.roomCreateFailed')); }
  }, [roomName, roomType, selectedUsers, t, visibility]);

  const addDirectoryUser = useCallback((user: DirectoryUser) => {
    setSelectedUsers((prev) => {
      if (roomType === 'direct') return [user];
      return prev.some((item) => item.id === user.id) ? prev : [...prev, user];
    });
  }, [roomType]);

  const handleDirectoryKeyDown = useCallback((event: KeyboardEvent<HTMLInputElement>) => {
    if (directoryUsers.length === 0) return;
    if (event.key === 'ArrowDown') { event.preventDefault(); setDirectoryActiveIndex((i) => Math.min(i + 1, directoryUsers.length - 1)); }
    else if (event.key === 'ArrowUp') { event.preventDefault(); setDirectoryActiveIndex((i) => Math.max(i - 1, 0)); }
    else if (event.key === 'Enter') { event.preventDefault(); addDirectoryUser(directoryUsers[directoryActiveIndex] ?? directoryUsers[0]); }
  }, [addDirectoryUser, directoryActiveIndex, directoryUsers]);

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
  }, [activeRoomId, composer, driveFileId, loadRooms, persistDraft, t]);

  const uploadFile = useCallback(async (file: File) => {
    if (!activeRoomId) return;
    try {
      const msg = await uploadDMAttachment(activeRoomId, file);
      setMessages((prev) => mergeMessage(prev, msg)); void loadRooms();
      void listDMMedia(activeRoomId, mediaTab).then(setMediaItems).catch(() => {});
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.uploadFailed')); }
  }, [activeRoomId, loadRooms, mediaTab, t]);

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
  }, [t]);

  const submitEdit = useCallback(async () => {
    if (!editingId || !editingBody.trim()) return;
    try {
      const msg = await editDMMessage(editingId, editingBody);
      setMessages((prev) => mergeMessage(prev, msg)); setEditingId(null); setEditingBody('');
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.editFailed')); }
  }, [editingBody, editingId, t]);

  const removeMessage = useCallback(async (messageId: string) => {
    try {
      const msg = await deleteDMMessage(messageId);
      setMessages((prev) => mergeMessage(prev, msg));
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.deleteFailed')); }
  }, [t]);

  const toggleReaction = useCallback(async (messageId: string, emoji: string) => {
    try {
      await toggleDMReaction(messageId, emoji); void loadMessages(); setReactionPickerMessageId(null);
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.reactionFailed')); }
    finally { setReactionPickerMessageId(null); }
  }, [loadMessages, t]);

  const addMembers = useCallback(async () => {
    const ids = memberInput.split(/[\s,]+/).map((item) => item.trim()).filter(Boolean);
    if (!activeRoomId || ids.length === 0) return;
    try {
      const added = await addDMMembers(activeRoomId, ids);
      setMessages((prev) => added.reduce(mergeMessage, prev)); setMemberInput(''); void loadRooms();
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.addMemberFailed')); }
  }, [activeRoomId, loadRooms, memberInput, t]);

  const transferOwner = useCallback(async () => {
    if (!activeRoomId || !ownerInput.trim()) return;
    try {
      const msg = await transferDMOwner(activeRoomId, ownerInput.trim());
      setMessages((prev) => mergeMessage(prev, msg)); setOwnerInput(''); void loadRooms();
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.ownerTransferFailed')); }
  }, [activeRoomId, loadRooms, ownerInput, t]);

  const makeInvite = useCallback(async () => {
    if (!activeRoomId) return;
    try {
      const result = await createDMInvite(activeRoomId); setInviteUrl(result.invite_url);
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.inviteFailed')); }
  }, [activeRoomId, t]);

  const leaveOrRemove = useCallback(async (userId: string) => {
    if (!activeRoomId) return;
    try {
      const result = await removeDMMember(activeRoomId, userId);
      if (result.deleted_room) { setActiveRoomId(''); setMessages([]); }
      else if (result.system_message) { setMessages((prev) => mergeMessage(prev, result.system_message!)); }
      void loadRooms();
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.removeMemberFailed')); }
  }, [activeRoomId, loadRooms, t]);

  return {
    t, selfAvatarUrl,
    // state
    rooms, publicRooms, activeRoomId, setActiveRoomId,
    messages,
    directoryQuery, setDirectoryQuery,
    directoryUsers, directoryActiveIndex, setDirectoryActiveIndex,
    selectedUsers, setSelectedUsers,
    roomName, setRoomName,
    roomType, setRoomType,
    visibility, setVisibility,
    composer, setComposer,
    driveFileId, setDriveFileId,
    searchQuery, setSearchQuery,
    searchResults,
    mediaTab, setMediaTab,
    mediaItems,
    inviteUrl, setInviteUrl,
    memberInput, setMemberInput,
    ownerInput, setOwnerInput,
    editingId, setEditingId,
    editingBody, setEditingBody,
    newChatOpen, setNewChatOpen,
    detailsOpen, setDetailsOpen,
    driveComposerOpen, setDriveComposerOpen,
    reactionPickerMessageId, setReactionPickerMessageId,
    previewImage, setPreviewImage,
    imageMenu, setImageMenu,
    pendingPasteFile, setPendingPasteFile,
    pendingPastePreview,
    notice,
    loadingRooms, loadingMessages,
    error,
    // refs
    fileInputRef, memberInputRef, messageEndRef, reactionPickerRef, composerComposingRef,
    // derived
    activeRoom, unread, currentUserId, memberById, mediaTabLabels,
    // callbacks
    titleForRoom, previewForMessage, persistDraft,
    loadRooms, loadMessages,
    createRoom, addDirectoryUser, handleDirectoryKeyDown,
    send, uploadFile, uploadPastedImages, confirmPendingPaste,
    copyImageToClipboard, submitEdit, removeMessage, toggleReaction,
    addMembers, transferOwner, makeInvite, leaveOrRemove,
  };
}
