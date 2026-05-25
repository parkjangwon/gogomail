'use client';

import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { CSSProperties } from 'react';
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
  type DirectoryUser,
} from '@/lib/api';
import {
  ArrowPathIcon,
  ChatBubbleLeftRightIcon,
  InformationCircleIcon,
  LinkIcon,
  MagnifyingGlassIcon,
  PaperAirplaneIcon,
  PaperClipIcon,
  PlusIcon,
  TrashIcon,
  UserPlusIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';

type DMPanelProps = {
  userEmail?: string;
  onUnreadChange?: (count: number) => void;
  onClose?: () => void;
};

type MediaTab = 'files' | 'links' | 'drive';

const CURRENT_USER_ID = process.env.NEXT_PUBLIC_GOGOMAIL_DEV_USER_ID ?? '';
const EMOJI = ['👍', '🙏', '🔥', '✅'];

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

function roomTitle(room: DMRoom, fallbackDirect: string, fallbackGroup: string): string {
  if (room.name?.trim()) return room.name;
  const names = room.members?.map((m) => m.display_name || m.id).filter(Boolean) ?? [];
  return names.length > 0 ? names.join(', ') : room.room_type === 'direct' ? fallbackDirect : fallbackGroup;
}

function messagePreview(message: DMMessage | undefined, labels: { deleted: string; file: string; drive: string }): string {
  if (!message) return '';
  if (message.deleted_at) return labels.deleted;
  if (message.message_type === 'file') return message.attachment_name || message.body || labels.file;
  if (message.message_type === 'drive_link') return message.body || message.drive_file_id || labels.drive;
  return message.body;
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

export function DMPanel({ userEmail, onUnreadChange, onClose }: DMPanelProps) {
  const t = useTranslations('dmPanel');
  const [rooms, setRooms] = useState<DMRoom[]>([]);
  const [publicRooms, setPublicRooms] = useState<DMRoom[]>([]);
  const [activeRoomId, setActiveRoomId] = useState<string>('');
  const [messages, setMessages] = useState<DMMessage[]>([]);
  const [directoryQuery, setDirectoryQuery] = useState('');
  const [directoryUsers, setDirectoryUsers] = useState<DirectoryUser[]>([]);
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
  const [loadingRooms, setLoadingRooms] = useState(false);
  const [loadingMessages, setLoadingMessages] = useState(false);
  const [error, setError] = useState('');
  const fileInputRef = useRef<HTMLInputElement | null>(null);
  const messageEndRef = useRef<HTMLDivElement | null>(null);

  const activeRoom = rooms.find((room) => room.id === activeRoomId) ?? null;
  const unread = useMemo(() => rooms.reduce((sum, room) => sum + (room.unread_count ?? 0), 0), [rooms]);
  const previewLabels = useMemo(() => ({ deleted: t('deletedMessage'), file: t('file'), drive: t('drive') }), [t]);
  const mediaTabLabels = useMemo<Record<MediaTab, string>>(() => ({
    files: t('tabFiles'),
    links: t('tabLinks'),
    drive: t('tabDrive'),
  }), [t]);
  const titleForRoom = useCallback((room: DMRoom) => roomTitle(room, t('directMessage'), t('group')), [t]);
  const previewForMessage = useCallback((message?: DMMessage) => messagePreview(message, previewLabels), [previewLabels]);

  const loadRooms = useCallback(async () => {
    setLoadingRooms(true);
    try {
      const [joined, publicList] = await Promise.all([listDMRooms(), listPublicDMRooms()]);
      setRooms(joined);
      setPublicRooms(publicList);
      onUnreadChange?.(joined.reduce((sum, room) => sum + (room.unread_count ?? 0), 0));
      if (!activeRoomId && joined[0]) setActiveRoomId(joined[0].id);
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
      return;
    }
    const id = window.setTimeout(() => {
      void listDirectoryUsers(directoryQuery || undefined, 30).then(setDirectoryUsers);
    }, 180);
    return () => window.clearTimeout(id);
  }, [directoryQuery, newChatOpen]);

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

  const send = useCallback(async () => {
    if (!activeRoomId || (!composer.trim() && !driveFileId.trim())) return;
    const body = composer.trim();
    const drive = driveFileId.trim();
    setComposer('');
    setDriveFileId('');
    try {
      const sent = await sendDMMessage(activeRoomId, body, drive || undefined);
      setMessages((prev) => mergeMessage(prev, sent));
      void loadRooms();
    } catch (err) {
      setComposer(body);
      setDriveFileId(drive);
      setError(err instanceof Error ? err.message : t('errors.sendFailed'));
    }
  }, [activeRoomId, composer, driveFileId, loadRooms, t]);

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
    } catch (err) {
      setError(err instanceof Error ? err.message : t('errors.reactionFailed'));
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
      <aside style={{ width: 300, flexShrink: 0, borderRight: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)', display: 'flex', flexDirection: 'column', minHeight: 0 }}>
        <div style={{ padding: '14px', borderBottom: '1px solid var(--color-border-subtle)' }}>
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
                  {directoryUsers.map((user) => (
                    <button key={user.id} type="button" onClick={() => setSelectedUsers((prev) => prev.some((item) => item.id === user.id) ? prev : [...prev, user])} style={{ width: '100%', textAlign: 'left', border: 'none', borderBottom: '1px solid var(--color-border-subtle)', background: 'transparent', color: 'var(--color-text-primary)', padding: '8px 9px', cursor: 'pointer' }}>
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
              <span style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                <span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 13, fontWeight: room.unread_count ? 700 : 600 }}>{titleForRoom(room)}</span>
                {!!room.unread_count && <span style={{ borderRadius: 8, padding: '1px 6px', fontSize: 10, background: 'var(--color-accent)', color: '#fff' }}>{room.unread_count}</span>}
              </span>
              <span style={{ display: 'block', marginTop: 3, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 12, color: 'var(--color-text-tertiary)' }}>{previewForMessage(room.last_message) || t('membersCount', { count: room.member_count ?? room.members?.length ?? 0 })}</span>
            </button>
          ))}
          {publicRooms.length > 0 && (
            <div style={{ borderTop: '1px solid var(--color-border-subtle)' }}>
              <div style={{ padding: '10px 14px 4px', fontSize: 11, fontWeight: 700, color: 'var(--color-text-tertiary)', textTransform: 'uppercase' }}>{t('public')}</div>
              {publicRooms.map((room) => (
                <button key={room.id} type="button" onClick={() => setActiveRoomId(room.id)} style={{ width: '100%', border: 'none', borderTop: '1px solid var(--color-border-subtle)', background: 'transparent', color: 'var(--color-text-primary)', padding: '9px 14px', textAlign: 'left', cursor: 'pointer' }}>
                  <span style={{ display: 'block', fontSize: 13, fontWeight: 600 }}>{titleForRoom(room)}</span>
                  <span style={{ display: 'block', fontSize: 12, color: 'var(--color-text-tertiary)' }}>{t('membersCount', { count: room.member_count ?? 0 })}</span>
                </button>
              ))}
            </div>
          )}
        </div>
      </aside>

      <main style={{ flex: 1, minWidth: 0, display: 'flex', flexDirection: 'column', height: '100%' }}>
        <header style={{ minHeight: 58, borderBottom: '1px solid var(--color-border-subtle)', display: 'flex', alignItems: 'center', gap: 12, padding: '10px 16px', flexShrink: 0 }}>
          <div style={{ minWidth: 0, flex: 1 }}>
            <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{activeRoom ? titleForRoom(activeRoom) : t('title')}</div>
            <div style={{ fontSize: 12, color: 'var(--color-text-tertiary)' }}>{activeRoom ? t('membersCount', { count: activeRoom.members?.length ?? activeRoom.member_count ?? 0 }) : userEmail}</div>
          </div>
          <div style={{ display: 'flex', gap: 6, alignItems: 'center' }}>
            {activeRoom && (
              <>
                <div style={{ position: 'relative' }}>
                  <MagnifyingGlassIcon style={{ position: 'absolute', left: 8, top: 7, width: 15, height: 15, color: 'var(--color-text-tertiary)' }} />
                  <input value={searchQuery} onChange={(e) => setSearchQuery(e.currentTarget.value)} placeholder={t('search')} style={{ width: 180, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '6px 9px 6px 28px', fontSize: 13 }} />
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
          <div style={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: detailsOpen ? 'minmax(0, 1fr) 260px' : 'minmax(0, 1fr)' }}>
            <section style={{ display: 'flex', flexDirection: 'column', minWidth: 0, minHeight: 0 }}>
              <div style={{ flex: 1, minHeight: 0, overflow: 'auto', padding: '16px 18px' }}>
                {loadingMessages && messages.length === 0 ? (
                  <div style={{ color: 'var(--color-text-tertiary)', fontSize: 13 }}>{t('loading')}</div>
                ) : (
                  messages.map((message) => {
                    const mine = CURRENT_USER_ID && message.sender_id === CURRENT_USER_ID;
                    const system = message.message_type === 'system';
                    return (
                      <div key={message.id} style={{ display: 'flex', justifyContent: system ? 'center' : mine ? 'flex-end' : 'flex-start', marginBottom: 9 }}>
                        <div style={{ maxWidth: system ? '70%' : 'min(72%, 680px)', borderRadius: system ? 6 : 8, border: system ? '1px solid var(--color-border-subtle)' : 'none', background: system ? 'var(--color-bg-secondary)' : mine ? 'var(--color-accent)' : 'var(--color-bg-secondary)', color: system ? 'var(--color-text-secondary)' : mine ? '#fff' : 'var(--color-text-primary)', padding: system ? '5px 9px' : '8px 10px' }}>
                          {!system && (
                            <div style={{ display: 'flex', gap: 8, alignItems: 'center', marginBottom: 4 }}>
                              <span style={{ fontSize: 11, fontWeight: 700, color: mine ? 'rgba(255,255,255,0.78)' : 'var(--color-text-tertiary)' }}>{message.sender_id || 'system'}</span>
                              <span style={{ fontSize: 11, color: mine ? 'rgba(255,255,255,0.68)' : 'var(--color-text-tertiary)' }}>{formatTime(message.created_at)}{message.edited_at ? ` · ${t('edited')}` : ''}</span>
                            </div>
                          )}
                          {editingId === message.id ? (
                            <div style={{ display: 'flex', gap: 6 }}>
                              <input value={editingBody} onChange={(e) => setEditingBody(e.currentTarget.value)} style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', borderRadius: 5, padding: '5px 7px', fontSize: 13 }} />
                              <button type="button" onClick={submitEdit} style={{ border: 'none', borderRadius: 5, background: 'var(--color-accent)', color: '#fff', padding: '0 9px', fontSize: 12, cursor: 'pointer' }}>{t('save')}</button>
                            </div>
                          ) : (
                            <div style={{ whiteSpace: 'pre-wrap', overflowWrap: 'anywhere', fontSize: system ? 12 : 13, lineHeight: 1.5 }}>
                              {message.message_type === 'file' && <PaperClipIcon style={{ width: 14, height: 14, verticalAlign: '-2px', marginRight: 4 }} />}
                              {message.message_type === 'drive_link' && <LinkIcon style={{ width: 14, height: 14, verticalAlign: '-2px', marginRight: 4 }} />}
                              {message.deleted_at ? t('deletedMessage') : message.body || message.attachment_name || message.drive_file_id}
                              {message.attachment_size ? <span style={{ marginLeft: 6, opacity: 0.72 }}>{formatBytes(message.attachment_size)}</span> : null}
                            </div>
                          )}
                          {!system && !message.deleted_at && (
                            <div style={{ display: 'flex', gap: 4, marginTop: 6, alignItems: 'center', justifyContent: mine ? 'flex-end' : 'flex-start' }}>
                              {EMOJI.map((emoji) => (
                                <button key={emoji} type="button" onClick={() => toggleReaction(message.id, emoji)} style={{ border: 'none', borderRadius: 10, padding: '1px 5px', background: mine ? 'rgba(255,255,255,0.18)' : 'var(--color-bg-tertiary)', color: mine ? '#fff' : 'var(--color-text-secondary)', fontSize: 11, cursor: 'pointer' }}>
                                  {emoji}{message.reactions?.find((r) => r.emoji === emoji)?.count ? ` ${message.reactions.find((r) => r.emoji === emoji)!.count}` : ''}
                                </button>
                              ))}
                              <button type="button" onClick={() => { setEditingId(message.id); setEditingBody(message.body); }} style={{ border: 'none', background: 'transparent', color: mine ? 'rgba(255,255,255,0.82)' : 'var(--color-text-tertiary)', fontSize: 11, cursor: 'pointer' }}>{t('edit')}</button>
                              <button type="button" onClick={() => removeMessage(message.id)} aria-label={t('deleteMessage')} style={{ border: 'none', background: 'transparent', color: mine ? 'rgba(255,255,255,0.82)' : 'var(--color-text-tertiary)', cursor: 'pointer', padding: 0 }}>
                                <TrashIcon style={{ width: 13, height: 13 }} />
                              </button>
                            </div>
                          )}
                        </div>
                      </div>
                    );
                  })
                )}
                <div ref={messageEndRef} />
              </div>
              <footer style={{ borderTop: '1px solid var(--color-border-subtle)', padding: '10px 12px', flexShrink: 0 }}>
                {driveComposerOpen && (
                  <input value={driveFileId} onChange={(e) => setDriveFileId(e.currentTarget.value)} placeholder={t('driveFileId')} style={{ width: '100%', boxSizing: 'border-box', marginBottom: 8, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '7px 9px', fontSize: 13 }} />
                )}
                <div style={{ display: 'flex', gap: 8 }}>
                  <button type="button" onClick={() => setDriveComposerOpen((open) => !open)} aria-label={t('addDriveFile')} style={{ width: 36, border: '1px solid var(--color-border-default)', borderRadius: 6, background: driveComposerOpen ? 'var(--color-accent-subtle)' : 'transparent', color: driveComposerOpen ? 'var(--color-accent)' : 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                    <LinkIcon style={{ width: 16, height: 16 }} />
                  </button>
                  <input
                    value={composer}
                    onChange={(e) => setComposer(e.currentTarget.value)}
                    onKeyDown={(e) => {
                      if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault();
                        void send();
                      }
                    }}
                    placeholder={t('message')}
                    style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '7px 9px', fontSize: 13 }}
                  />
                  <button type="button" onClick={send} disabled={!composer.trim() && !driveFileId.trim()} aria-label={t('sendMessage')} style={{ width: 36, border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                    <PaperAirplaneIcon style={{ width: 17, height: 17 }} />
                  </button>
                </div>
              </footer>
            </section>

            {detailsOpen && (
            <aside style={{ borderLeft: '1px solid var(--color-border-subtle)', background: 'var(--color-bg-secondary)', minHeight: 0, overflow: 'auto' }}>
              <div style={{ padding: 12, borderBottom: '1px solid var(--color-border-subtle)' }}>
                <div style={{ display: 'flex', gap: 5, marginBottom: 10 }}>
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
              <div style={{ padding: 12, borderBottom: '1px solid var(--color-border-subtle)' }}>
                <div style={{ display: 'flex', gap: 6, marginBottom: 8 }}>
                  <input value={memberInput} onChange={(e) => setMemberInput(e.currentTarget.value)} placeholder={t('userIds')} style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '6px 8px', fontSize: 12 }} />
                  <button type="button" onClick={addMembers} aria-label={t('addMembers')} style={{ width: 30, border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                    <UserPlusIcon style={{ width: 15, height: 15 }} />
                  </button>
                </div>
                <div style={{ display: 'flex', gap: 6 }}>
                  <input value={ownerInput} onChange={(e) => setOwnerInput(e.currentTarget.value)} placeholder={t('ownerUserId')} style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '6px 8px', fontSize: 12 }} />
                  <button type="button" onClick={transferOwner} style={{ border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', padding: '0 8px', fontSize: 12, cursor: 'pointer' }}>{t('owner')}</button>
                </div>
              </div>
              <div style={{ padding: 12, borderBottom: '1px solid var(--color-border-subtle)' }}>
                <button type="button" onClick={makeInvite} style={{ width: '100%', border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', padding: '7px 9px', fontSize: 12, cursor: 'pointer' }}>{t('createInvite')}</button>
                {inviteUrl && <input readOnly value={inviteUrl} onFocus={(e) => e.currentTarget.select()} style={{ marginTop: 8, width: '100%', boxSizing: 'border-box', border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', padding: '6px 8px', fontSize: 12 }} />}
              </div>
              <div style={{ padding: 12 }}>
                <div style={{ marginBottom: 8, color: 'var(--color-text-tertiary)', fontSize: 11, fontWeight: 700, textTransform: 'uppercase' }}>{t('members')}</div>
                {(activeRoom.members ?? []).map((member) => (
                  <div key={member.id} style={{ display: 'flex', alignItems: 'center', gap: 7, padding: '5px 0', fontSize: 12, color: 'var(--color-text-secondary)' }}>
                    <span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{member.display_name || member.id}</span>
                    <button type="button" onClick={() => leaveOrRemove(member.id)} aria-label={t('removeMember')} style={{ border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', padding: 0 }}>
                      <TrashIcon style={{ width: 13, height: 13 }} />
                    </button>
                  </div>
                ))}
              </div>
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
          <div style={{ position: 'absolute', top: 60, right: detailsOpen ? 280 : 16, width: 320, maxHeight: 260, overflow: 'auto', zIndex: 70, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', boxShadow: '0 12px 32px rgba(0,0,0,0.12)', borderRadius: 8 }}>
            {searchResults.map((message) => (
              <button key={message.id} type="button" onClick={() => setSearchQuery('')} style={{ display: 'block', width: '100%', border: 'none', borderBottom: '1px solid var(--color-border-subtle)', background: 'transparent', color: 'var(--color-text-primary)', padding: 10, textAlign: 'left', cursor: 'pointer' }}>
                <span style={{ display: 'block', fontSize: 12, color: 'var(--color-text-tertiary)' }}>{formatTime(message.created_at)}</span>
                <span style={{ display: 'block', fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{previewForMessage(message)}</span>
              </button>
            ))}
          </div>
        )}
      </main>
    </div>
  );
}
