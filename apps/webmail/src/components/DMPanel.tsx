'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { ClipboardEvent, CSSProperties, KeyboardEvent, MouseEvent as ReactMouseEvent } from 'react';
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
import {
  ArrowDownTrayIcon,
  ArrowLeftIcon,
  ArrowPathIcon,
  ChatBubbleLeftRightIcon,
  ClipboardDocumentIcon,
  InformationCircleIcon,
  LinkIcon,
  MagnifyingGlassIcon,
  PaperAirplaneIcon,
  PaperClipIcon,
  FaceSmileIcon,
  PlusIcon,
  TrashIcon,
  UserPlusIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';
import { avatarColor } from './message-list/messageListTypes';

type DMPanelProps = {
  userEmail?: string;
  onUnreadChange?: (count: number) => void;
  onClose?: () => void;
  onComposeToAddress?: (email: string) => void;
  onStartWindowDrag?: (event: ReactMouseEvent<HTMLElement>) => void;
};

type MediaTab = 'files' | 'links' | 'drive';
type DMDraft = { body: string; driveFileId: string };

const DEV_CURRENT_USER_ID = process.env.NEXT_PUBLIC_GOGOMAIL_DEV_USER_ID ?? '';
const DM_DRAFT_STORAGE_KEY = 'webmail_dm_drafts_v1';
const REACTION_EMOJI = [
  '😀', '😂', '🥰', '😍', '😮', '😢', '😎', '🙏',
  '👍', '👎', '❤️', '🎉', '✨', '🔥', '💯', '✅',
  '👏', '🙌', '🤝', '💪', '👀', '💡', '📌', '🚀',
  '☕', '🍕', '🎵', '🏆', '❌', '⚠️', '💬', '🎁',
];

function formatTime(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}

function formatBytes(size?: number): string {
  if (!size || size <= 0) return '';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

function initials(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return '?';
  return trimmed.split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase();
}

function memberName(member?: DMUser, fallback = ''): string {
  return member?.display_name || member?.id || fallback;
}

function memberEmail(member?: DMUser): string {
  return member?.email || '';
}

function memberAvatarURL(member: DMUser | undefined, currentUserId: string, selfAvatarUrl: string): string {
  return member?.avatar_url || (member?.id === currentUserId ? selfAvatarUrl : '');
}

function MemberAvatar({ member, currentUserId, selfAvatarUrl, size = 30, label }: { member?: DMUser; currentUserId: string; selfAvatarUrl: string; size?: number; label?: string }) {
  const name = memberName(member, label);
  const avatarUrl = memberAvatarURL(member, currentUserId, selfAvatarUrl);
  return (
    <span aria-hidden={!label} aria-label={label} style={{ width: size, height: size, borderRadius: '50%', background: avatarUrl ? 'transparent' : avatarColor(member?.id || name), color: '#fff', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontSize: Math.max(10, size * 0.36), fontWeight: 700, flexShrink: 0, overflow: 'hidden', border: '1px solid var(--color-border-subtle)' }}>
      {avatarUrl ? <img src={avatarUrl} alt={label || ''} style={{ width: '100%', height: '100%', objectFit: 'cover' }} /> : initials(name)}
    </span>
  );
}

function RoomAvatar({ room, currentUserId, selfAvatarUrl }: { room: DMRoom; currentUserId: string; selfAvatarUrl: string }) {
  const others = (room.members ?? []).filter((member) => member.id !== currentUserId);
  const members = room.room_type === 'direct' ? [others[0] ?? room.members?.[0]] : (others.length ? others : room.members ?? []).slice(0, 2);
  if (room.room_type === 'group' && members.length > 1) {
    return (
      <span aria-hidden="true" style={{ position: 'relative', width: 34, height: 30, flexShrink: 0, display: 'inline-flex' }}>
        <span style={{ position: 'absolute', left: 0, top: 2 }}><MemberAvatar member={members[0]} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} size={26} /></span>
        <span style={{ position: 'absolute', right: 0, bottom: 0 }}><MemberAvatar member={members[1]} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} size={24} /></span>
      </span>
    );
  }
  return <MemberAvatar member={members[0]} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} size={30} />;
}

function readDMDrafts(): Record<string, DMDraft> {
  try {
    if (typeof window === 'undefined') return {};
    const parsed = JSON.parse(localStorage.getItem(DM_DRAFT_STORAGE_KEY) ?? '{}') as Record<string, DMDraft>;
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function writeDMDrafts(drafts: Record<string, DMDraft>) {
  try {
    if (typeof window === 'undefined') return;
    localStorage.setItem(DM_DRAFT_STORAGE_KEY, JSON.stringify(drafts));
  } catch {
    /* best-effort */
  }
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

  if (room.room_type === 'direct') {
    return otherNames[0] || room.name?.trim() || labels.direct;
  }
  if (room.name?.trim()) return room.name;
  if (otherNames.length > 1) return labels.groupOthers(otherNames[0], otherNames.length - 1);
  return otherNames[0] || labels.group;
}

function messagePreview(message: DMMessage | undefined, labels: { deleted: string; file: string; drive: string }): string {
  if (!message) return '';
  if (message.deleted_at) return labels.deleted;
  if (message.message_type === 'file') return message.attachment_name || message.body || labels.file;
  if (message.message_type === 'drive_link') return message.body || message.drive_file_id || labels.drive;
  return message.body;
}

function isDMImageMessage(message: DMMessage): boolean {
  if (message.message_type !== 'file') return false;
  const mime = (message.attachment_mime_type ?? '').toLowerCase();
  if (['image/jpeg', 'image/jpg', 'image/png', 'image/webp'].includes(mime)) return true;
  return /\.(jpe?g|png|webp)$/i.test(message.attachment_name ?? message.body ?? '');
}

function downloadFromURL(url: string, filename: string) {
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename || 'download';
  document.body.appendChild(anchor);
  anchor.click();
  anchor.remove();
}

function mergeMessage(existing: DMMessage[], next: DMMessage): DMMessage[] {
  const index = existing.findIndex((m) => m.id === next.id);
  if (index === -1) return [...existing, next].sort((a, b) => Date.parse(a.created_at) - Date.parse(b.created_at));
  const merged = [...existing];
  merged[index] = next;
  return merged;
}

function pillButton(active: boolean): CSSProperties {
  return {
    border: '1px solid var(--color-border-default)',
    borderRadius: '6px',
    background: active ? 'var(--color-accent-subtle)' : 'transparent',
    color: active ? 'var(--color-accent)' : 'var(--color-text-secondary)',
    fontSize: '12px',
    fontWeight: 600,
    padding: '5px 9px',
    cursor: 'pointer',
  };
}

export function DMPanel({ userEmail, onUnreadChange, onClose, onComposeToAddress, onStartWindowDrag }: DMPanelProps) {
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
    files: t('tabFiles'),
    links: t('tabLinks'),
    drive: t('tabDrive'),
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
    if (body.trim() || drive.trim()) {
      next[roomId] = { body, driveFileId: drive };
    } else {
      delete next[roomId];
    }
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
    } finally {
      setLoadingRooms(false);
    }
  }, [activeRoomId, onUnreadChange, t]);

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
    } finally {
      setLoadingMessages(false);
    }
  }, [activeRoomId, loadRooms, t]);

  useEffect(() => { void loadRooms(); }, [loadRooms]);
  useEffect(() => { void loadMessages(); }, [loadMessages]);

  useEffect(() => {
    const id = window.setInterval(() => {
      if (document.visibilityState === 'visible') void loadRooms();
    }, 5000);
    return () => window.clearInterval(id);
  }, [loadRooms]);

  useEffect(() => {
    const id = window.setInterval(() => {
      if (document.visibilityState === 'visible') void loadMessages();
    }, 3000);
    return () => window.clearInterval(id);
  }, [loadMessages]);

  useEffect(() => {
    if (!newChatOpen) {
      setDirectoryUsers([]);
      setDirectoryActiveIndex(0);
      return;
    }
    const id = window.setTimeout(() => {
      void listDirectoryUsers(directoryQuery || undefined, 30).then((users) => {
        setDirectoryUsers(users);
        setDirectoryActiveIndex(0);
      });
    }, 180);
    return () => window.clearTimeout(id);
  }, [directoryQuery, newChatOpen]);

  useEffect(() => {
    if (!pendingPasteFile) {
      setPendingPastePreview('');
      return;
    }
    const url = URL.createObjectURL(pendingPasteFile);
    setPendingPastePreview(url);
    return () => URL.revokeObjectURL(url);
  }, [pendingPasteFile]);

  useEffect(() => {
    if (!activeRoomId || !searchQuery.trim()) {
      setSearchResults([]);
      return;
    }
    const id = window.setTimeout(() => {
      void searchDMMessages(activeRoomId, searchQuery.trim(), undefined, 20)
        .then((results) => setSearchResults(results.map((r) => r.message)))
        .catch(() => setSearchResults([]));
    }, 250);
    return () => window.clearTimeout(id);
  }, [activeRoomId, searchQuery]);

  useEffect(() => {
    if (!activeRoomId) {
      setMediaItems([]);
      return;
    }
    void listDMMedia(activeRoomId, mediaTab).then(setMediaItems).catch(() => setMediaItems([]));
  }, [activeRoomId, mediaTab]);

  useEffect(() => {
    messageEndRef.current?.scrollIntoView({ block: 'end' });
  }, [messages.length, activeRoomId]);

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
    const closeOnKey = (event: globalThis.KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      event.stopPropagation();
      setImageMenu(null);
    };
    document.addEventListener('mousedown', close);
    document.addEventListener('keydown', closeOnKey, true);
    return () => {
      document.removeEventListener('mousedown', close);
      document.removeEventListener('keydown', closeOnKey, true);
    };
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
      setSelectedUsers([]);
      setRoomName('');
      setDirectoryQuery('');
      setNewChatOpen(false);
      setError('');
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.roomCreateFailed'));
    }
  }, [roomName, roomType, selectedUsers, t, visibility]);

  const addDirectoryUser = useCallback((user: DirectoryUser) => {
    setSelectedUsers((prev) => {
      if (roomType === 'direct') return [user];
      return prev.some((item) => item.id === user.id) ? prev : [...prev, user];
    });
  }, [roomType]);

  const handleDirectoryKeyDown = useCallback((event: KeyboardEvent<HTMLInputElement>) => {
    if (directoryUsers.length === 0) return;
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setDirectoryActiveIndex((index) => Math.min(index + 1, directoryUsers.length - 1));
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setDirectoryActiveIndex((index) => Math.max(index - 1, 0));
    } else if (event.key === 'Enter') {
      event.preventDefault();
      addDirectoryUser(directoryUsers[directoryActiveIndex] ?? directoryUsers[0]);
    }
  }, [addDirectoryUser, directoryActiveIndex, directoryUsers]);

  const send = useCallback(async () => {
    if (!activeRoomId || (!composer.trim() && !driveFileId.trim())) return;
    if (composerComposingRef.current || sendingRef.current) return;
    const body = composer.trim();
    const drive = driveFileId.trim();
    sendingRef.current = true;
    setComposer('');
    setDriveFileId('');
    persistDraft(activeRoomId, '', '');
    try {
      const sent = await sendDMMessage(activeRoomId, body, drive || undefined);
      setMessages((prev) => mergeMessage(prev, sent));
      void loadRooms();
    } catch (err) {
      setComposer(body);
      setDriveFileId(drive);
      persistDraft(activeRoomId, body, drive);
      setError(err instanceof Error ? err.message : t('errors.sendFailed'));
    } finally {
      sendingRef.current = false;
    }
  }, [activeRoomId, composer, driveFileId, loadRooms, persistDraft, t]);

  const uploadFile = useCallback(async (file: File) => {
    if (!activeRoomId) return;
    try {
      const msg = await uploadDMAttachment(activeRoomId, file);
      setMessages((prev) => mergeMessage(prev, msg));
      void loadRooms();
      void listDMMedia(activeRoomId, mediaTab).then(setMediaItems).catch(() => {});
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.uploadFailed'));
    }
  }, [activeRoomId, loadRooms, mediaTab, t]);

  const uploadPastedImages = useCallback((event: ClipboardEvent<HTMLInputElement>) => {
    const files: File[] = [];
    for (const item of Array.from(event.clipboardData.items)) {
      if (!item.type.startsWith('image/')) continue;
      const file = item.getAsFile();
      if (file) files.push(new File([file], file.name || `clipboard-${Date.now()}.png`, { type: file.type || item.type }));
    }
    if (files.length === 0) return;
    event.preventDefault();
    setPendingPasteFile(files[0]);
  }, []);

  const confirmPendingPaste = useCallback(() => {
    if (!pendingPasteFile) return;
    const file = pendingPasteFile;
    setPendingPasteFile(null);
    void uploadFile(file);
  }, [pendingPasteFile, uploadFile]);

  const copyImageToClipboard = useCallback(async (message: DMMessage) => {
    if (!message.attachment_download_url) return;
    try {
      const response = await fetch(message.attachment_download_url);
      if (!response.ok) throw new Error(`image fetch failed: ${response.status}`);
      const blob = await response.blob();
      await navigator.clipboard.write([new ClipboardItem({ [blob.type || 'image/png']: blob })]);
      setNotice(t('imageCopied'));
      window.setTimeout(() => setNotice(''), 1800);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.copyFailed'));
    } finally {
      setImageMenu(null);
    }
  }, [t]);

  const submitEdit = useCallback(async () => {
    if (!editingId || !editingBody.trim()) return;
    try {
      const msg = await editDMMessage(editingId, editingBody);
      setMessages((prev) => mergeMessage(prev, msg));
      setEditingId(null);
      setEditingBody('');
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.editFailed'));
    }
  }, [editingBody, editingId, t]);

  const removeMessage = useCallback(async (messageId: string) => {
    try {
      const msg = await deleteDMMessage(messageId);
      setMessages((prev) => mergeMessage(prev, msg));
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.deleteFailed'));
    }
  }, [t]);

  const toggleReaction = useCallback(async (messageId: string, emoji: string) => {
    try {
      await toggleDMReaction(messageId, emoji);
      void loadMessages();
      setReactionPickerMessageId(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.reactionFailed'));
    } finally {
      setReactionPickerMessageId(null);
    }
  }, [loadMessages, t]);

  const addMembers = useCallback(async () => {
    const ids = memberInput.split(/[\s,]+/).map((item) => item.trim()).filter(Boolean);
    if (!activeRoomId || ids.length === 0) return;
    try {
      const added = await addDMMembers(activeRoomId, ids);
      setMessages((prev) => added.reduce(mergeMessage, prev));
      setMemberInput('');
      void loadRooms();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.addMemberFailed'));
    }
  }, [activeRoomId, loadRooms, memberInput, t]);

  const transferOwner = useCallback(async () => {
    if (!activeRoomId || !ownerInput.trim()) return;
    try {
      const msg = await transferDMOwner(activeRoomId, ownerInput.trim());
      setMessages((prev) => mergeMessage(prev, msg));
      setOwnerInput('');
      void loadRooms();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.ownerTransferFailed'));
    }
  }, [activeRoomId, loadRooms, ownerInput, t]);

  const makeInvite = useCallback(async () => {
    if (!activeRoomId) return;
    try {
      const result = await createDMInvite(activeRoomId);
      setInviteUrl(result.invite_url);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.inviteFailed'));
    }
  }, [activeRoomId, t]);

  const handleWindowHeaderMouseDown = useCallback((event: ReactMouseEvent<HTMLElement>) => {
    const target = event.target as HTMLElement;
    if (target.closest('button,input,textarea,a,[role="button"],[role="menuitem"]')) return;
    onStartWindowDrag?.(event);
  }, [onStartWindowDrag]);

  const leaveOrRemove = useCallback(async (userId: string) => {
    if (!activeRoomId) return;
    try {
      const result = await removeDMMember(activeRoomId, userId);
      if (result.deleted_room) {
        setActiveRoomId('');
        setMessages([]);
      } else if (result.system_message) {
        setMessages((prev) => mergeMessage(prev, result.system_message!));
      }
      void loadRooms();
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.removeMemberFailed'));
    }
  }, [activeRoomId, loadRooms, t]);

  return (
    <div style={{ flex: 1, minWidth: 0, display: 'flex', height: '100%', overflow: 'hidden', background: 'var(--color-bg-primary)', position: 'relative' }}>
      <aside style={{ width: '100%', flexShrink: 0, borderRight: activeRoom ? '1px solid var(--color-border-subtle)' : 'none', background: 'var(--color-bg-secondary)', display: activeRoom ? 'none' : 'flex', flexDirection: 'column', minHeight: 0 }}>
        <div onMouseDown={handleWindowHeaderMouseDown} style={{ padding: '14px', borderBottom: '1px solid var(--color-border-subtle)', cursor: onStartWindowDrag ? 'move' : 'default' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <ChatBubbleLeftRightIcon style={{ width: 19, height: 19, color: 'var(--color-accent)' }} />
            <h1 style={{ margin: 0, fontSize: 16, lineHeight: 1.3, color: 'var(--color-text-primary)', fontWeight: 700 }}>{t('title')}</h1>
            {unread > 0 && <span style={{ marginLeft: 2, borderRadius: 10, padding: '1px 7px', fontSize: 11, color: '#fff', background: 'var(--color-destructive)' }}>{unread > 99 ? '99+' : unread}</span>}
            <button type="button" aria-label={t('refresh')} onClick={() => { void loadRooms(); void loadMessages(); }} style={{ marginLeft: 'auto', width: 30, height: 30, border: 'none', borderRadius: 6, background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', display: 'grid', placeItems: 'center' }}>
              <ArrowPathIcon style={{ width: 17, height: 17 }} />
            </button>
            <button type="button" aria-label={t('newDM')} onClick={() => setNewChatOpen((open) => !open)} style={{ width: 30, height: 30, border: '1px solid var(--color-border-default)', borderRadius: 6, background: newChatOpen ? 'var(--color-accent)' : 'var(--color-bg-primary)', color: newChatOpen ? '#fff' : 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
              <PlusIcon style={{ width: 17, height: 17 }} />
            </button>
          </div>
          {newChatOpen && (
            <div style={{ marginTop: 12, border: '1px solid var(--color-border-subtle)', borderRadius: 8, background: 'var(--color-bg-primary)', padding: 10 }}>
              <div style={{ display: 'flex', gap: 6, marginBottom: 8 }}>
                <button type="button" onClick={() => setRoomType('direct')} style={pillButton(roomType === 'direct')}>{t('direct')}</button>
                <button type="button" onClick={() => setRoomType('group')} style={pillButton(roomType === 'group')}>{t('group')}</button>
                {roomType === 'group' && (
                  <button type="button" onClick={() => setVisibility((v) => v === 'private' ? 'public' : 'private')} style={pillButton(visibility === 'public')}>
                    {visibility === 'public' ? t('public') : t('private')}
                  </button>
                )}
              </div>
              {roomType === 'group' && (
                <input
                  value={roomName}
                  onChange={(e) => setRoomName(e.currentTarget.value)}
                  placeholder={t('roomName')}
                  style={{ width: '100%', boxSizing: 'border-box', marginBottom: 8, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '7px 9px', fontSize: 13 }}
                />
              )}
              <div style={{ display: 'flex', gap: 6 }}>
                <input
                  value={directoryQuery}
                  onChange={(e) => setDirectoryQuery(e.currentTarget.value)}
                  onKeyDown={handleDirectoryKeyDown}
                  placeholder={t('searchPeople')}
                  style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '7px 9px', fontSize: 13 }}
                />
                <button type="button" onClick={createRoom} disabled={selectedUsers.length === 0 || (roomType === 'group' && !roomName.trim())} aria-label={t('createRoom')} style={{ width: 34, border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer', opacity: selectedUsers.length === 0 || (roomType === 'group' && !roomName.trim()) ? 0.55 : 1 }}>
                  <PlusIcon style={{ width: 17, height: 17 }} />
                </button>
              </div>
              {selectedUsers.length > 0 && (
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 5, marginTop: 8 }}>
                  {selectedUsers.map((user) => (
                    <button key={user.id} type="button" onClick={() => setSelectedUsers((prev) => prev.filter((item) => item.id !== user.id))} style={{ border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-secondary)', borderRadius: 6, padding: '3px 7px', fontSize: 12, cursor: 'pointer' }}>
                      {user.display_name || user.email}
                    </button>
                  ))}
                </div>
              )}
              {directoryUsers.length > 0 && (
                <div style={{ marginTop: 8, maxHeight: 150, overflow: 'auto', border: '1px solid var(--color-border-subtle)', borderRadius: 6, background: 'var(--color-bg-primary)' }}>
                  {directoryUsers.map((user, index) => (
                    <button key={user.id} type="button" onMouseEnter={() => setDirectoryActiveIndex(index)} onClick={() => addDirectoryUser(user)} style={{ width: '100%', textAlign: 'left', border: 'none', borderBottom: '1px solid var(--color-border-subtle)', background: index === directoryActiveIndex ? 'var(--color-accent-subtle)' : 'transparent', color: 'var(--color-text-primary)', padding: '8px 9px', cursor: 'pointer' }}>
                      <span style={{ display: 'block', fontSize: 13, fontWeight: 600 }}>{user.display_name || user.email}</span>
                      <span style={{ display: 'block', fontSize: 11, color: 'var(--color-text-tertiary)' }}>{user.email}</span>
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        <div style={{ flex: 1, overflow: 'auto', minHeight: 0 }}>
          {loadingRooms && rooms.length === 0 ? (
            <div style={{ padding: 16, color: 'var(--color-text-tertiary)', fontSize: 13 }}>{t('loading')}</div>
          ) : rooms.length === 0 ? (
            <div style={{ padding: 20, color: 'var(--color-text-tertiary)', fontSize: 13, lineHeight: 1.5 }}>
              <div style={{ fontWeight: 700, color: 'var(--color-text-secondary)', marginBottom: 4 }}>{t('noConversationsTitle')}</div>
              <div>{t('noConversationsDesc')}</div>
            </div>
          ) : rooms.map((room) => (
            <button
              key={room.id}
              type="button"
              onClick={() => { setActiveRoomId(room.id); setInviteUrl(''); }}
              style={{ width: '100%', border: 'none', borderBottom: '1px solid var(--color-border-subtle)', background: activeRoomId === room.id ? 'var(--color-accent-subtle)' : 'transparent', color: 'var(--color-text-primary)', padding: '10px 14px', textAlign: 'left', cursor: 'pointer' }}
            >
              <span style={{ display: 'flex', alignItems: 'center', gap: 9 }}>
                <RoomAvatar room={room} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} />
                <span style={{ flex: 1, minWidth: 0 }}>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                    <span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 13, fontWeight: room.unread_count ? 700 : 600 }}>{titleForRoom(room)}</span>
                    {room.last_message?.created_at && <span style={{ flexShrink: 0, fontSize: 11, color: 'var(--color-text-tertiary)' }}>{formatTime(room.last_message.created_at)}</span>}
                    {!!room.unread_count && <span style={{ borderRadius: 8, padding: '1px 6px', fontSize: 10, background: 'var(--color-accent)', color: '#fff' }}>{room.unread_count}</span>}
                  </span>
                  <span style={{ display: 'block', marginTop: 3, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 12, color: 'var(--color-text-tertiary)' }}>{previewForMessage(room.last_message) || t('membersCount', { count: room.member_count ?? room.members?.length ?? 0 })}</span>
                </span>
              </span>
            </button>
          ))}
          {publicRooms.length > 0 && (
            <div style={{ borderTop: '1px solid var(--color-border-subtle)' }}>
              <div style={{ padding: '10px 14px 4px', fontSize: 11, fontWeight: 700, color: 'var(--color-text-tertiary)', textTransform: 'uppercase' }}>{t('public')}</div>
              {publicRooms.map((room) => (
                <button key={room.id} type="button" onClick={() => setActiveRoomId(room.id)} style={{ width: '100%', border: 'none', borderTop: '1px solid var(--color-border-subtle)', background: 'transparent', color: 'var(--color-text-primary)', padding: '9px 14px', textAlign: 'left', cursor: 'pointer' }}>
                  <span style={{ display: 'flex', alignItems: 'center', gap: 9 }}>
                    <RoomAvatar room={room} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} />
                    <span style={{ minWidth: 0 }}>
                      <span style={{ display: 'block', fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{titleForRoom(room)}</span>
                      <span style={{ display: 'block', fontSize: 12, color: 'var(--color-text-tertiary)' }}>{t('membersCount', { count: room.member_count ?? 0 })}</span>
                    </span>
                  </span>
                </button>
              ))}
            </div>
          )}
        </div>
      </aside>

      <main style={{ flex: 1, minWidth: 0, display: activeRoom ? 'flex' : 'none', flexDirection: 'column', height: '100%' }}>
        <header
          onMouseDown={handleWindowHeaderMouseDown}
          style={{ minHeight: 58, borderBottom: '1px solid var(--color-border-subtle)', display: 'flex', alignItems: 'center', gap: 8, padding: '8px 10px', flexShrink: 0, flexWrap: 'wrap', cursor: onStartWindowDrag ? 'move' : 'default' }}
        >
          <div style={{ minWidth: 0, flex: '1 1 180px', display: 'flex', alignItems: 'center', gap: 8 }}>
            <button type="button" onClick={() => { setActiveRoomId(''); setDetailsOpen(false); setSearchQuery(''); }} aria-label={t('backToList')} style={{ width: 32, height: 32, border: 'none', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer', flexShrink: 0 }}>
              <ArrowLeftIcon style={{ width: 18, height: 18 }} />
            </button>
            {activeRoom && <RoomAvatar room={activeRoom} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} />}
            <div style={{ minWidth: 0 }}>
              <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{activeRoom ? titleForRoom(activeRoom) : t('title')}</div>
              <div style={{ fontSize: 12, color: 'var(--color-text-tertiary)' }}>{activeRoom ? t('membersCount', { count: activeRoom.members?.length ?? activeRoom.member_count ?? 0 }) : userEmail}</div>
            </div>
          </div>
          <div style={{ display: 'flex', gap: 6, alignItems: 'center', flex: '1 1 180px', justifyContent: 'flex-end', minWidth: 0 }}>
            {activeRoom && (
              <>
                <div style={{ position: 'relative', flex: '1 1 110px', minWidth: 0, maxWidth: 220 }}>
                  <MagnifyingGlassIcon style={{ position: 'absolute', left: 8, top: 7, width: 15, height: 15, color: 'var(--color-text-tertiary)' }} />
                  <input value={searchQuery} onChange={(e) => setSearchQuery(e.currentTarget.value)} placeholder={t('search')} style={{ width: '100%', boxSizing: 'border-box', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '6px 9px 6px 28px', fontSize: 13 }} />
                </div>
                <button type="button" onClick={() => fileInputRef.current?.click()} disabled={!activeRoomId} aria-label={t('attachFile')} style={{ width: 32, height: 32, border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                  <PaperClipIcon style={{ width: 17, height: 17 }} />
                </button>
                <button type="button" onClick={() => setDetailsOpen((open) => !open)} aria-label={t('conversationDetails')} style={{ width: 32, height: 32, border: '1px solid var(--color-border-default)', borderRadius: 6, background: detailsOpen ? 'var(--color-accent-subtle)' : 'transparent', color: detailsOpen ? 'var(--color-accent)' : 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                  <InformationCircleIcon style={{ width: 17, height: 17 }} />
                </button>
              </>
            )}
            {onClose && (
              <button type="button" onClick={onClose} aria-label={t('close')} style={{ width: 32, height: 32, border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                <XMarkIcon style={{ width: 17, height: 17 }} />
              </button>
            )}
            <input ref={fileInputRef} type="file" style={{ display: 'none' }} onChange={(event) => {
              const file = event.currentTarget.files?.[0];
              event.currentTarget.value = '';
              if (file) void uploadFile(file);
            }} />
          </div>
        </header>

        {error && (
          <div role="alert" style={{ padding: '8px 16px', borderBottom: '1px solid var(--color-border-subtle)', color: 'var(--color-destructive)', fontSize: 12, flexShrink: 0 }}>
            {error}
          </div>
        )}

        {activeRoom ? (
          <div style={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: detailsOpen ? 'minmax(0, 1fr) minmax(170px, 34%)' : 'minmax(0, 1fr)' }}>
            <section style={{ display: 'flex', flexDirection: 'column', minWidth: 0, minHeight: 0 }}>
              <div style={{ flex: 1, minHeight: 0, overflow: 'auto', padding: '14px 12px' }}>
                {loadingMessages && messages.length === 0 ? (
                  <div style={{ color: 'var(--color-text-tertiary)', fontSize: 13 }}>{t('loading')}</div>
                ) : (
                  messages.map((message) => {
                    const mine = !!currentUserId && message.sender_id === currentUserId;
                    const system = message.message_type === 'system';
                    const reactions = message.reactions ?? [];
                    const sender = message.sender_id ? memberById.get(message.sender_id) : undefined;
                    const senderLabel = memberName(sender, message.sender_id || 'system');
                    const imageMessage = isDMImageMessage(message);
                    const imageSrc = message.attachment_download_url;
                    return (
                      <div key={message.id} style={{ display: 'flex', justifyContent: system ? 'center' : mine ? 'flex-end' : 'flex-start', alignItems: 'flex-end', gap: 7, marginBottom: 9 }}>
                        {!system && !mine && <MemberAvatar member={sender} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} size={28} label={senderLabel} />}
                        <div style={{ maxWidth: system ? '78%' : 'min(76%, 680px)', minWidth: 0, borderRadius: system ? 6 : 8, border: system ? '1px solid var(--color-border-subtle)' : 'none', background: system ? 'var(--color-bg-secondary)' : mine ? 'var(--color-accent)' : 'var(--color-bg-secondary)', color: system ? 'var(--color-text-secondary)' : mine ? '#fff' : 'var(--color-text-primary)', padding: system ? '5px 9px' : '8px 10px' }}>
                          {!system && (
                            <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 4 }}>
                              <span style={{ fontSize: 11, fontWeight: 700, color: mine ? 'rgba(255,255,255,0.78)' : 'var(--color-text-tertiary)' }}>{senderLabel}</span>
                              <span style={{ fontSize: 11, color: mine ? 'rgba(255,255,255,0.68)' : 'var(--color-text-tertiary)' }}>{formatTime(message.created_at)}{message.edited_at ? ` · ${t('edited')}` : ''}</span>
                            </div>
                          )}
                          {editingId === message.id ? (
                            <div style={{ display: 'flex', gap: 6 }}>
                              <input value={editingBody} onChange={(e) => setEditingBody(e.currentTarget.value)} style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', borderRadius: 5, padding: '5px 7px', fontSize: 13 }} />
                              <button type="button" onClick={submitEdit} style={{ border: 'none', borderRadius: 5, background: 'var(--color-accent)', color: '#fff', padding: '0 9px', fontSize: 12, cursor: 'pointer' }}>{t('save')}</button>
                            </div>
                          ) : imageMessage && imageSrc && !message.deleted_at ? (
                            <div>
                              <button type="button" onClick={() => setPreviewImage(message)} aria-label={t('openImage')} style={{ display: 'block', border: 'none', padding: 0, background: 'transparent', cursor: 'zoom-in', maxWidth: '100%' }}>
                                <img
                                  src={imageSrc}
                                  alt={message.attachment_name || message.body || t('imageAttachment')}
                                  onContextMenu={(event) => {
                                    event.preventDefault();
                                    setImageMenu({ message, x: event.clientX, y: event.clientY });
                                  }}
                                  style={{ display: 'block', maxWidth: 'min(320px, 100%)', maxHeight: 260, objectFit: 'cover', borderRadius: 7, border: mine ? '1px solid rgba(255,255,255,0.24)' : '1px solid var(--color-border-subtle)' }}
                                />
                              </button>
                              <div style={{ marginTop: 6, fontSize: 12, color: mine ? 'rgba(255,255,255,0.82)' : 'var(--color-text-secondary)', display: 'flex', alignItems: 'center', gap: 6, minWidth: 0 }}>
                                <PaperClipIcon style={{ width: 13, height: 13, flexShrink: 0 }} />
                                <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{message.attachment_name || message.body || t('imageAttachment')}</span>
                                {message.attachment_size ? <span style={{ opacity: 0.72, flexShrink: 0 }}>{formatBytes(message.attachment_size)}</span> : null}
                                <button type="button" onClick={() => downloadFromURL(imageSrc, message.attachment_name || message.body || 'image')} aria-label={t('downloadFile')} style={{ border: 'none', background: 'transparent', color: mine ? 'rgba(255,255,255,0.86)' : 'var(--color-accent)', padding: 1, cursor: 'pointer', display: 'inline-flex' }}>
                                  <ArrowDownTrayIcon style={{ width: 14, height: 14 }} />
                                </button>
                                <button type="button" onClick={() => copyImageToClipboard(message)} aria-label={t('copyImage')} style={{ border: 'none', background: 'transparent', color: mine ? 'rgba(255,255,255,0.86)' : 'var(--color-accent)', padding: 1, cursor: 'pointer', display: 'inline-flex' }}>
                                  <ClipboardDocumentIcon style={{ width: 14, height: 14 }} />
                                </button>
                              </div>
                            </div>
                          ) : (
                            <div style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontSize: system ? 12 : 13, lineHeight: 1.5 }}>
                              {message.message_type === 'file' && <PaperClipIcon style={{ width: 14, height: 14, verticalAlign: '-2px', marginRight: 4 }} />}
                              {message.message_type === 'drive_link' && <LinkIcon style={{ width: 14, height: 14, verticalAlign: '-2px', marginRight: 4 }} />}
                              {message.deleted_at ? t('deletedMessage') : message.body || message.attachment_name || message.drive_file_id}
                              {message.attachment_size ? <span style={{ marginLeft: 6, opacity: 0.72 }}>{formatBytes(message.attachment_size)}</span> : null}
                              {message.message_type === 'file' && message.attachment_download_url && !message.deleted_at && (
                                <button type="button" onClick={() => downloadFromURL(message.attachment_download_url!, message.attachment_name || message.body || 'download')} aria-label={t('downloadFile')} style={{ marginLeft: 6, border: 'none', background: 'transparent', color: mine ? 'rgba(255,255,255,0.86)' : 'var(--color-accent)', padding: 1, cursor: 'pointer', verticalAlign: '-2px' }}>
                                  <ArrowDownTrayIcon style={{ width: 14, height: 14 }} />
                                </button>
                              )}
                            </div>
                          )}
                          {!system && !message.deleted_at && (
                            <div style={{ display: 'flex', gap: 4, marginTop: 6, alignItems: 'center', justifyContent: mine ? 'flex-end' : 'flex-start' }}>
                              {reactions.map((reaction) => (
                                <button key={reaction.emoji} type="button" onClick={() => toggleReaction(message.id, reaction.emoji)} style={{ border: 'none', borderRadius: 10, padding: '1px 6px', background: reaction.mine ? 'var(--color-accent-subtle)' : mine ? 'rgba(255,255,255,0.18)' : 'var(--color-bg-tertiary)', color: mine ? '#fff' : reaction.mine ? 'var(--color-accent)' : 'var(--color-text-secondary)', fontSize: 11, cursor: 'pointer' }}>
                                  {reaction.emoji}{reaction.count ? ` ${reaction.count}` : ''}
                                </button>
                              ))}
                              <span ref={reactionPickerMessageId === message.id ? reactionPickerRef : undefined} style={{ position: 'relative', display: 'inline-flex' }}>
                                <button type="button" onClick={() => setReactionPickerMessageId((id) => id === message.id ? null : message.id)} aria-label={t('react')} style={{ border: 'none', borderRadius: 10, padding: '1px 5px', background: mine ? 'rgba(255,255,255,0.18)' : 'var(--color-bg-tertiary)', color: mine ? '#fff' : 'var(--color-text-secondary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}>
                                  <FaceSmileIcon style={{ width: 13, height: 13 }} />
                                </button>
                                {reactionPickerMessageId === message.id && (
                                  <span style={{ position: 'absolute', top: '100%', right: mine ? 0 : 'auto', left: mine ? 'auto' : 0, marginTop: 6, width: 230, padding: 8, border: '1px solid var(--color-border-default)', borderRadius: 8, background: 'var(--color-bg-primary)', boxShadow: '0 12px 32px rgba(0,0,0,0.16)', display: 'flex', flexWrap: 'wrap', gap: 3, zIndex: 90 }}>
                                    {REACTION_EMOJI.map((emoji) => (
                                      <button key={emoji} type="button" onClick={() => toggleReaction(message.id, emoji)} style={{ width: 25, height: 25, border: 'none', borderRadius: 5, background: 'transparent', cursor: 'pointer', fontSize: 17, lineHeight: 1 }}>
                                        {emoji}
                                      </button>
                                    ))}
                                  </span>
                                )}
                              </span>
                              <button type="button" onClick={() => { setEditingId(message.id); setEditingBody(message.body); }} style={{ border: 'none', background: 'transparent', color: mine ? 'rgba(255,255,255,0.82)' : 'var(--color-text-tertiary)', fontSize: 11, cursor: 'pointer' }}>{t('edit')}</button>
                              <button type="button" onClick={() => removeMessage(message.id)} aria-label={t('deleteMessage')} style={{ border: 'none', background: 'transparent', color: mine ? 'rgba(255,255,255,0.82)' : 'var(--color-text-tertiary)', cursor: 'pointer', padding: 0 }}>
                                <TrashIcon style={{ width: 13, height: 13 }} />
                              </button>
                            </div>
                          )}
                        </div>
                        {!system && mine && <MemberAvatar member={sender} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} size={28} label={senderLabel} />}
                      </div>
                    );
                  })
                )}
                <div ref={messageEndRef} />
              </div>
              <footer style={{ borderTop: '1px solid var(--color-border-subtle)', padding: '9px 10px', flexShrink: 0 }}>
                {driveComposerOpen && (
                  <input value={driveFileId} onChange={(e) => { const value = e.currentTarget.value; setDriveFileId(value); persistDraft(activeRoomId, composer, value); }} placeholder={t('driveFileId')} style={{ width: '100%', boxSizing: 'border-box', marginBottom: 8, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '7px 9px', fontSize: 13 }} />
                )}
                <div style={{ display: 'flex', gap: 7, minWidth: 0 }}>
                  <button type="button" onClick={() => setDriveComposerOpen((open) => !open)} aria-label={t('addDriveFile')} style={{ width: 34, minWidth: 34, border: '1px solid var(--color-border-default)', borderRadius: 6, background: driveComposerOpen ? 'var(--color-accent-subtle)' : 'transparent', color: driveComposerOpen ? 'var(--color-accent)' : 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                    <LinkIcon style={{ width: 16, height: 16 }} />
                  </button>
                  <input
                    value={composer}
                    onChange={(e) => { const value = e.currentTarget.value; setComposer(value); persistDraft(activeRoomId, value, driveFileId); }}
                    onPaste={uploadPastedImages}
                    onCompositionStart={() => { composerComposingRef.current = true; }}
                    onCompositionEnd={() => { composerComposingRef.current = false; }}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        const nativeEvent = e.nativeEvent as globalThis.KeyboardEvent & { isComposing?: boolean };
                        if (nativeEvent.isComposing || nativeEvent.keyCode === 229 || composerComposingRef.current) return;
                        e.preventDefault();
                        void send();
                      }
                    }}
                    placeholder={t('message')}
                    style={{ flex: '1 1 120px', minWidth: 0, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '7px 9px', fontSize: 13 }}
                  />
                  <button type="button" onClick={send} disabled={!composer.trim() && !driveFileId.trim()} aria-label={t('sendMessage')} style={{ width: 34, minWidth: 34, border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                    <PaperAirplaneIcon style={{ width: 17, height: 17 }} />
                  </button>
                </div>
              </footer>
            </section>

            {detailsOpen && (
            <aside style={{ borderLeft: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)', minWidth: 0, minHeight: 0, overflow: 'auto' }}>
              <div style={{ padding: 12, borderBottom: '1px solid var(--color-border-subtle)' }}>
                <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text-primary)', marginBottom: 12 }}>{t('conversationDetails')}</div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 10, marginBottom: 12 }}>
                  <RoomAvatar room={activeRoom} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} />
                  <div style={{ minWidth: 0 }}>
                    <div style={{ fontSize: 13, fontWeight: 700, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{titleForRoom(activeRoom)}</div>
                    <div style={{ fontSize: 12, color: 'var(--color-text-tertiary)' }}>{t('membersCount', { count: activeRoom.members?.length ?? activeRoom.member_count ?? 0 })}</div>
                  </div>
                </div>
                {activeRoom.room_type === 'group' && (
                  <>
                    <button type="button" onClick={makeInvite} style={{ width: '100%', border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', padding: '7px 9px', fontSize: 12, cursor: 'pointer' }}>{t('createInvite')}</button>
                    {inviteUrl && <input readOnly value={inviteUrl} onFocus={(e) => e.currentTarget.select()} style={{ marginTop: 8, width: '100%', boxSizing: 'border-box', border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', padding: '6px 8px', fontSize: 12 }} />}
                  </>
                )}
              </div>
              <div style={{ padding: 12, borderBottom: '1px solid var(--color-border-subtle)' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 8, marginBottom: 8 }}>
                  <div style={{ color: 'var(--color-text-primary)', fontSize: 13, fontWeight: 700 }}>{t('members')}</div>
                  {activeRoom.room_type === 'group' && <button type="button" onClick={() => memberInputRef.current?.focus()} aria-label={t('addMembers')} style={{ border: 'none', background: 'transparent', color: 'var(--color-accent)', fontSize: 12, fontWeight: 700, cursor: 'pointer', padding: 0 }}>{t('addMembers')}</button>}
                </div>
                {(activeRoom.members ?? []).map((member) => {
                  const name = memberName(member);
                  const email = memberEmail(member);
                  const isOwner = activeRoom.owner_id === member.id;
                  const canRemove = activeRoom.room_type === 'group' || member.id === currentUserId;
                  return (
                    <div key={member.id} style={{ display: 'flex', alignItems: 'center', gap: 9, padding: '7px 0', fontSize: 12, color: 'var(--color-text-secondary)' }}>
                      <MemberAvatar member={member} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} size={34} label={name} />
                      <span style={{ flex: 1, minWidth: 0 }}>
                        <span style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                          <span style={{ minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontWeight: 700, color: 'var(--color-text-primary)' }}>{name}</span>
                          {isOwner && <span style={{ borderRadius: 999, background: 'var(--color-accent-subtle)', color: 'var(--color-accent)', padding: '1px 6px', fontSize: 10, fontWeight: 700 }}>{t('owner')}</span>}
                        </span>
                        {email ? (
                          <button type="button" onClick={() => onComposeToAddress?.(email)} style={{ display: 'block', maxWidth: '100%', marginTop: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: onComposeToAddress ? 'var(--color-accent)' : 'var(--color-text-tertiary)', background: 'transparent', border: 'none', padding: 0, font: 'inherit', cursor: onComposeToAddress ? 'pointer' : 'default', textAlign: 'left' }}>{email}</button>
                        ) : (
                          <span style={{ display: 'block', marginTop: 2, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', color: 'var(--color-text-tertiary)' }}>{member.id}</span>
                        )}
                      </span>
                      {canRemove && (
                        <button type="button" onClick={() => leaveOrRemove(member.id)} aria-label={t('removeMember')} style={{ border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', padding: 2 }}>
                          <TrashIcon style={{ width: 13, height: 13 }} />
                        </button>
                      )}
                    </div>
                  );
                })}
              </div>
              <div style={{ padding: 12, borderBottom: '1px solid var(--color-border-subtle)' }}>
                <div style={{ display: 'flex', gap: 5, marginBottom: 10, flexWrap: 'wrap' }}>
                  {(['files', 'links', 'drive'] as MediaTab[]).map((tab) => (
                    <button key={tab} type="button" onClick={() => setMediaTab(tab)} style={pillButton(mediaTab === tab)}>{mediaTabLabels[tab]}</button>
                  ))}
                </div>
                {mediaItems.length === 0 ? (
                  <div style={{ color: 'var(--color-text-tertiary)', fontSize: 12 }}>{t('noItems')}</div>
                ) : mediaItems.map((item) => (
                  <div key={`${item.message_id}-${item.url ?? item.attachment_name ?? item.drive_file_id}`} style={{ padding: '7px 0', borderTop: '1px solid var(--color-border-subtle)', fontSize: 12, color: 'var(--color-text-secondary)', overflowWrap: 'anywhere' }}>
                    {item.download_url ? <a href={item.download_url} style={{ color: 'var(--color-accent)' }}>{item.attachment_name || item.download_url}</a> : (item.url || item.attachment_name || item.drive_name || item.drive_file_id)}
                    {item.attachment_size ? <span style={{ display: 'block', color: 'var(--color-text-tertiary)' }}>{formatBytes(item.attachment_size)}</span> : null}
                  </div>
                ))}
              </div>
              {activeRoom.room_type === 'group' && <div style={{ padding: 12, borderBottom: '1px solid var(--color-border-subtle)' }}>
                <div style={{ display: 'flex', gap: 6, marginBottom: 8 }}>
                  <input ref={memberInputRef} value={memberInput} onChange={(e) => setMemberInput(e.currentTarget.value)} placeholder={t('userIds')} style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '6px 8px', fontSize: 12 }} />
                  <button type="button" onClick={addMembers} aria-label={t('addMembers')} style={{ width: 30, border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                    <UserPlusIcon style={{ width: 15, height: 15 }} />
                  </button>
                </div>
                <div style={{ display: 'flex', gap: 6 }}>
                  <input value={ownerInput} onChange={(e) => setOwnerInput(e.currentTarget.value)} placeholder={t('ownerUserId')} style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '6px 8px', fontSize: 12 }} />
                  <button type="button" onClick={transferOwner} style={{ border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', padding: '0 8px', fontSize: 12, cursor: 'pointer' }}>{t('owner')}</button>
                </div>
              </div>}
            </aside>
            )}
          </div>
        ) : (
          <div style={{ flex: 1, display: 'grid', placeItems: 'center', color: 'var(--color-text-tertiary)', fontSize: 14 }}>
            <div style={{ textAlign: 'center', maxWidth: 280, lineHeight: 1.5 }}>
              <ChatBubbleLeftRightIcon style={{ width: 42, height: 42, color: 'var(--color-text-tertiary)', marginBottom: 10 }} />
              <div style={{ color: 'var(--color-text-primary)', fontWeight: 700, marginBottom: 4 }}>{t('selectTitle')}</div>
              <div style={{ marginBottom: 14 }}>{t('selectDesc')}</div>
              <button type="button" onClick={() => setNewChatOpen(true)} style={{ border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', padding: '8px 12px', fontSize: 13, fontWeight: 700, cursor: 'pointer' }}>{t('newChat')}</button>
            </div>
          </div>
        )}

        {searchResults.length > 0 && (
          <div style={{ position: 'absolute', top: 64, right: detailsOpen ? 12 : 12, width: 'min(320px, calc(100% - 24px))', maxHeight: 260, overflow: 'auto', zIndex: 70, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', boxShadow: '0 12px 32px rgba(0,0,0,0.12)', borderRadius: 8 }}>
            {searchResults.map((message) => (
              <button key={message.id} type="button" onClick={() => setSearchQuery('')} style={{ display: 'block', width: '100%', border: 'none', borderBottom: '1px solid var(--color-border-subtle)', background: 'transparent', color: 'var(--color-text-primary)', padding: 10, textAlign: 'left', cursor: 'pointer' }}>
                <span style={{ display: 'block', fontSize: 12, color: 'var(--color-text-tertiary)' }}>{formatTime(message.created_at)}</span>
                <span style={{ display: 'block', fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{previewForMessage(message)}</span>
              </button>
            ))}
          </div>
        )}
      </main>
      {previewImage?.attachment_download_url && (
        <div role="dialog" aria-modal="true" aria-label={previewImage.attachment_name || t('imageAttachment')} onClick={() => setPreviewImage(null)} style={{ position: 'absolute', inset: 0, zIndex: 140, background: 'rgba(15,23,42,0.72)', display: 'grid', placeItems: 'center', padding: 24 }}>
          <button type="button" onClick={(e) => { e.stopPropagation(); setPreviewImage(null); }} aria-label={t('closeImage')} style={{ position: 'absolute', top: 14, right: 14, width: 34, height: 34, border: '1px solid rgba(255,255,255,0.36)', borderRadius: 6, background: 'rgba(15,23,42,0.42)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
            <XMarkIcon style={{ width: 19, height: 19 }} />
          </button>
          <div onClick={(e) => e.stopPropagation()} style={{ position: 'absolute', left: 14, top: 14, display: 'flex', gap: 8 }}>
            <button type="button" onClick={() => downloadFromURL(previewImage.attachment_download_url!, previewImage.attachment_name || previewImage.body || 'image')} aria-label={t('downloadFile')} style={{ width: 34, height: 34, border: '1px solid rgba(255,255,255,0.36)', borderRadius: 6, background: 'rgba(15,23,42,0.42)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
              <ArrowDownTrayIcon style={{ width: 18, height: 18 }} />
            </button>
            <button type="button" onClick={() => copyImageToClipboard(previewImage)} aria-label={t('copyImage')} style={{ width: 34, height: 34, border: '1px solid rgba(255,255,255,0.36)', borderRadius: 6, background: 'rgba(15,23,42,0.42)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
              <ClipboardDocumentIcon style={{ width: 18, height: 18 }} />
            </button>
          </div>
          <img onClick={(e) => e.stopPropagation()} src={previewImage.attachment_download_url} alt={previewImage.attachment_name || previewImage.body || t('imageAttachment')} style={{ maxWidth: 'min(92vw, 920px)', maxHeight: 'min(82vh, 760px)', objectFit: 'contain', borderRadius: 8, boxShadow: '0 24px 80px rgba(0,0,0,0.34)', background: '#fff' }} />
        </div>
      )}
      {imageMenu && (
        <div role="menu" style={{ position: 'fixed', left: imageMenu.x, top: imageMenu.y, zIndex: 180, minWidth: 138, border: '1px solid var(--color-border-default)', borderRadius: 7, background: 'var(--color-bg-primary)', boxShadow: '0 12px 30px rgba(0,0,0,0.18)', padding: 4 }}>
          <button type="button" role="menuitem" onMouseDown={(event) => event.stopPropagation()} onClick={() => copyImageToClipboard(imageMenu.message)} style={{ width: '100%', border: 'none', borderRadius: 5, background: 'transparent', color: 'var(--color-text-primary)', padding: '7px 9px', textAlign: 'left', cursor: 'pointer', fontSize: 13 }}>{t('copyImage')}</button>
          <button type="button" role="menuitem" onMouseDown={(event) => event.stopPropagation()} onClick={() => { downloadFromURL(imageMenu.message.attachment_download_url!, imageMenu.message.attachment_name || imageMenu.message.body || 'image'); setImageMenu(null); }} style={{ width: '100%', border: 'none', borderRadius: 5, background: 'transparent', color: 'var(--color-text-primary)', padding: '7px 9px', textAlign: 'left', cursor: 'pointer', fontSize: 13 }}>{t('downloadFile')}</button>
        </div>
      )}
      {pendingPasteFile && (
        <div role="dialog" aria-modal="true" aria-label={t('confirmImageAttach')} onClick={() => setPendingPasteFile(null)} style={{ position: 'absolute', inset: 0, zIndex: 150, background: 'rgba(15,23,42,0.46)', display: 'grid', placeItems: 'center', padding: 24 }}>
          <div onClick={(e) => e.stopPropagation()} style={{ width: 'min(360px, 100%)', border: '1px solid var(--color-border-default)', borderRadius: 8, background: 'var(--color-bg-primary)', boxShadow: '0 20px 50px rgba(0,0,0,0.22)', padding: 16 }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text-primary)', marginBottom: 10 }}>{t('confirmImageAttach')}</div>
            {pendingPastePreview && <img src={pendingPastePreview} alt={pendingPasteFile.name || t('imageAttachment')} style={{ display: 'block', width: '100%', maxHeight: 240, objectFit: 'contain', border: '1px solid var(--color-border-subtle)', borderRadius: 7, background: 'var(--color-bg-secondary)', marginBottom: 10 }} />}
            <div style={{ fontSize: 12, color: 'var(--color-text-secondary)', marginBottom: 14 }}>{pendingPasteFile.name || t('imageAttachment')} {formatBytes(pendingPasteFile.size)}</div>
            <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 8 }}>
              <button type="button" onClick={() => setPendingPasteFile(null)} style={{ border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', padding: '7px 11px', fontSize: 13, cursor: 'pointer' }}>{t('cancel')}</button>
              <button type="button" onClick={confirmPendingPaste} style={{ border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', padding: '7px 11px', fontSize: 13, fontWeight: 700, cursor: 'pointer' }}>{t('attachImage')}</button>
            </div>
          </div>
        </div>
      )}
      {notice && (
        <div role="status" style={{ position: 'absolute', left: '50%', bottom: 18, transform: 'translateX(-50%)', zIndex: 170, borderRadius: 999, background: 'rgba(15,23,42,0.88)', color: '#fff', padding: '6px 12px', fontSize: 12 }}>
          {notice}
        </div>
      )}
    </div>
  );
}
