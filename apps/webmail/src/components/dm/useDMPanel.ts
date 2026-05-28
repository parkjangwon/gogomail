'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useTranslations } from 'next-intl';
import {
  addDMMembers,
  createDMInvite,
  listDMMedia,
  removeDMMember,
  transferDMOwner,
  type DMMediaItem,
  type DMMessage,
  type DMRoom,
  type DMUser,
} from '@/lib/api';
import { useWebmailAvatar } from '@/lib/webmailAvatar';
import { mergeMessage, messagePreview, roomTitle, DEV_CURRENT_USER_ID, type MediaTab } from './dmUtils';
import { useDMRooms } from './useDMRooms';
import { useDMMessages } from './useDMMessages';

type UseDMPanelOptions = {
  onUnreadChange?: (count: number) => void;
};

export function useDMPanel({ onUnreadChange }: UseDMPanelOptions) {
  const t = useTranslations('dmPanel');
  const selfAvatarUrl = useWebmailAvatar();

  // UI state kept in orchestrator
  const [detailsOpen, setDetailsOpen] = useState(false);
  const [inviteUrl, setInviteUrl] = useState('');
  const [memberInput, setMemberInput] = useState('');
  const [ownerInput, setOwnerInput] = useState('');
  const [reactionPickerMessageId, setReactionPickerMessageId] = useState<string | null>(null);
  const [previewImage, setPreviewImage] = useState<DMMessage | null>(null);
  const [imageMenu, setImageMenu] = useState<{ message: DMMessage; x: number; y: number } | null>(null);
  const [notice, setNotice] = useState('');
  const [mediaTab, setMediaTab] = useState<MediaTab>('files');
  const [mediaItems, setMediaItems] = useState<DMMediaItem[]>([]);
  const [error, setError] = useState('');

  // Refs for UI interactions
  const memberInputRef = useRef<HTMLInputElement | null>(null);
  const reactionPickerRef = useRef<HTMLSpanElement | null>(null);

  // Sub-hooks
  const roomsHook = useDMRooms({ onUnreadChange, t, setError });
  const {
    rooms, setRooms, publicRooms,
    activeRoomId, setActiveRoomId,
    directoryQuery, setDirectoryQuery,
    directoryUsers, directoryActiveIndex, setDirectoryActiveIndex,
    selectedUsers, setSelectedUsers,
    roomName, setRoomName,
    roomType, setRoomType,
    visibility, setVisibility,
    loadingRooms,
    newChatOpen, setNewChatOpen,
    newChatError,
    loadRooms,
    createRoom,
    addDirectoryUser,
    handleDirectoryKeyDown,
  } = roomsHook;

  const msgsHook = useDMMessages({
    activeRoomId,
    loadRooms,
    mediaTab,
    setMediaItems,
    setNotice,
    setImageMenu: (v) => setImageMenu(v),
    setReactionPickerMessageId,
    t,
    setError,
  });
  const {
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
    messageEndRef,
    sendingRef,
    composerComposingRef,
    fileInputRef,
    draftsRef,
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
  } = msgsHook;

  // Derived values
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

  // Effects that depend on orchestrator-level state
  useEffect(() => {
    if (!activeRoomId) { setMediaItems([]); return; }
    void listDMMedia(activeRoomId, mediaTab).then(setMediaItems).catch(() => setMediaItems([]));
  }, [activeRoomId, mediaTab]);

  useEffect(() => {
    setReactionPickerMessageId(null);
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

  // Room-level actions that need both rooms and messages state
  const addMembers = useCallback(async () => {
    const ids = memberInput.split(/[\s,]+/).map((item) => item.trim()).filter(Boolean);
    if (!activeRoomId || ids.length === 0) return;
    try {
      const added = await addDMMembers(activeRoomId, ids);
      setMessages((prev) => added.reduce(mergeMessage, prev)); setMemberInput(''); void loadRooms();
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.addMemberFailed')); }
  }, [activeRoomId, loadRooms, memberInput, t, setMessages]);

  const transferOwner = useCallback(async () => {
    if (!activeRoomId || !ownerInput.trim()) return;
    try {
      const msg = await transferDMOwner(activeRoomId, ownerInput.trim());
      setMessages((prev) => mergeMessage(prev, msg)); setOwnerInput(''); void loadRooms();
    } catch (err) { setError(err instanceof Error ? err.message : t('errors.ownerTransferFailed')); }
  }, [activeRoomId, loadRooms, ownerInput, t, setMessages]);

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
  }, [activeRoomId, loadRooms, t, setActiveRoomId, setMessages]);

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
    newChatError,
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
