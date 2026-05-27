'use client';

import { useCallback, useEffect, useState } from 'react';
import type { KeyboardEvent } from 'react';
import { useTranslations } from 'next-intl';
import {
  createDMRoom,
  listDMRooms,
  listDirectoryUsers,
  listOrgTree,
  listPublicDMRooms,
  type DMRoom,
  type DirectoryUser,
} from '@/lib/api';
import { matchesDirectoryUser } from './dmUtils';

export interface UseDMRoomsParams {
  onUnreadChange?: (count: number) => void;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
  setError: (error: string) => void;
}

export function useDMRooms({ onUnreadChange, t, setError }: UseDMRoomsParams) {
  const [rooms, setRooms] = useState<DMRoom[]>([]);
  const [publicRooms, setPublicRooms] = useState<DMRoom[]>([]);
  const [activeRoomId, setActiveRoomId] = useState<string>('');
  const [directoryQuery, setDirectoryQuery] = useState('');
  const [directoryUsers, setDirectoryUsers] = useState<DirectoryUser[]>([]);
  const [directoryActiveIndex, setDirectoryActiveIndex] = useState(0);
  const [selectedUsers, setSelectedUsers] = useState<DirectoryUser[]>([]);
  const [roomName, setRoomName] = useState('');
  const [roomType, setRoomType] = useState<'direct' | 'group'>('direct');
  const [visibility, setVisibility] = useState<'private' | 'public'>('private');
  const [loadingRooms, setLoadingRooms] = useState(false);
  const [newChatOpen, setNewChatOpen] = useState(false);

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
  }, [onUnreadChange, t, setError]);

  useEffect(() => { void loadRooms(); }, [loadRooms]);
  useEffect(() => {
    const id = window.setInterval(() => { if (document.visibilityState === 'visible') void loadRooms(); }, 5000);
    return () => window.clearInterval(id);
  }, [loadRooms]);

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
  }, [roomName, roomType, selectedUsers, t, visibility, setError]);

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

  return {
    rooms, setRooms,
    publicRooms,
    activeRoomId, setActiveRoomId,
    directoryQuery, setDirectoryQuery,
    directoryUsers, directoryActiveIndex, setDirectoryActiveIndex,
    selectedUsers, setSelectedUsers,
    roomName, setRoomName,
    roomType, setRoomType,
    visibility, setVisibility,
    loadingRooms,
    newChatOpen, setNewChatOpen,
    loadRooms,
    createRoom,
    addDirectoryUser,
    handleDirectoryKeyDown,
  };
}
