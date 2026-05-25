'use client';
import { type DMTFunction } from './types';

import type { KeyboardEvent, MouseEvent as ReactMouseEvent } from 'react';
import type { DMRoom, DMUser, DirectoryUser } from '@/lib/api';
import {
  ArrowPathIcon,
  ChatBubbleLeftRightIcon,
  PlusIcon,
} from '@heroicons/react/24/outline';
import { avatarColor } from '../message-list/messageListTypes';

function initials(value: string): string {
  const trimmed = value.trim();
  if (!trimmed) return '?';
  return trimmed.split(/\s+/).map((part) => part[0]).join('').slice(0, 2).toUpperCase();
}

function memberName(member?: DMUser, fallback = ''): string {
  return member?.display_name || member?.id || fallback;
}

function memberAvatarURL(member: DMUser | undefined, currentUserId: string, selfAvatarUrl: string): string {
  return member?.avatar_url || (member?.id === currentUserId ? selfAvatarUrl : '');
}

export function directoryUserToDMUser(user: DirectoryUser): DMUser {
  return {
    id: user.id,
    display_name: user.display_name || user.email,
    email: user.email,
    avatar_url: user.avatar_url,
  };
}

export function MemberAvatar({ member, currentUserId, selfAvatarUrl, size = 30, label }: { member?: DMUser; currentUserId: string; selfAvatarUrl: string; size?: number; label?: string }) {
  const name = memberName(member, label);
  const avatarUrl = memberAvatarURL(member, currentUserId, selfAvatarUrl);
  return (
    <span aria-hidden={!label} aria-label={label} style={{ width: size, height: size, borderRadius: '50%', background: avatarUrl ? 'transparent' : avatarColor(member?.id || name), color: '#fff', display: 'inline-flex', alignItems: 'center', justifyContent: 'center', fontSize: Math.max(10, size * 0.36), fontWeight: 700, flexShrink: 0, overflow: 'hidden', border: '1px solid var(--color-border-subtle)' }}>
      {avatarUrl ? <img src={avatarUrl} alt={label || ''} style={{ width: '100%', height: '100%', objectFit: 'cover' }} /> : initials(name)}
    </span>
  );
}

export function RoomAvatar({ room, currentUserId, selfAvatarUrl }: { room: DMRoom; currentUserId: string; selfAvatarUrl: string }) {
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

function pillButton(active: boolean): React.CSSProperties {
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

function formatTime(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}

type DMRoomListProps = {
  rooms: DMRoom[];
  publicRooms: DMRoom[];
  activeRoomId: string;
  loadingRooms: boolean;
  currentUserId: string;
  selfAvatarUrl: string;
  unread: number;
  newChatOpen: boolean;
  roomType: 'direct' | 'group';
  roomName: string;
  visibility: 'private' | 'public';
  directoryQuery: string;
  directoryUsers: DirectoryUser[];
  directoryActiveIndex: number;
  selectedUsers: DirectoryUser[];
  onSelectRoom: (roomId: string) => void;
  onRefresh: () => void;
  onToggleNewChat: () => void;
  onSetRoomType: (type: 'direct' | 'group') => void;
  onToggleVisibility: () => void;
  onSetRoomName: (name: string) => void;
  onSetDirectoryQuery: (query: string) => void;
  onDirectoryKeyDown: (e: KeyboardEvent<HTMLInputElement>) => void;
  onDirectoryHover: (index: number) => void;
  onAddDirectoryUser: (user: DirectoryUser) => void;
  onRemoveSelectedUser: (userId: string) => void;
  onCreateRoom: () => void;
  onStartWindowDrag?: (event: ReactMouseEvent<HTMLElement>) => void;
  titleForRoom: (room: DMRoom) => string;
  previewForMessage: (message: DMRoom['last_message']) => string;
  t: DMTFunction;
};

export function DMRoomList({
  rooms,
  publicRooms,
  activeRoomId,
  loadingRooms,
  currentUserId,
  selfAvatarUrl,
  unread,
  newChatOpen,
  roomType,
  roomName,
  visibility,
  directoryQuery,
  directoryUsers,
  directoryActiveIndex,
  selectedUsers,
  onSelectRoom,
  onRefresh,
  onToggleNewChat,
  onSetRoomType,
  onToggleVisibility,
  onSetRoomName,
  onSetDirectoryQuery,
  onDirectoryKeyDown,
  onDirectoryHover,
  onAddDirectoryUser,
  onRemoveSelectedUser,
  onCreateRoom,
  onStartWindowDrag,
  titleForRoom,
  previewForMessage,
  t,
}: DMRoomListProps) {
  function handleHeaderMouseDown(event: ReactMouseEvent<HTMLElement>) {
    const target = event.target as HTMLElement;
    if (target.closest('button,input,textarea,a,[role="button"],[role="menuitem"]')) return;
    onStartWindowDrag?.(event);
  }

  return (
    <aside style={{ width: '100%', flexShrink: 0, borderRight: activeRoomId ? '1px solid var(--color-border-subtle)' : 'none', background: 'var(--color-bg-secondary)', display: activeRoomId ? 'none' : 'flex', flexDirection: 'column', minHeight: 0 }}>
      <div onMouseDown={handleHeaderMouseDown} style={{ padding: '14px', borderBottom: '1px solid var(--color-border-subtle)', cursor: onStartWindowDrag ? 'move' : 'default' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
          <ChatBubbleLeftRightIcon style={{ width: 19, height: 19, color: 'var(--color-accent)' }} />
          <h1 style={{ margin: 0, fontSize: 16, lineHeight: 1.3, color: 'var(--color-text-primary)', fontWeight: 700 }}>{t('title')}</h1>
          {unread > 0 && <span style={{ marginLeft: 2, borderRadius: 10, padding: '1px 7px', fontSize: 11, color: '#fff', background: 'var(--color-destructive)' }}>{unread > 99 ? '99+' : unread}</span>}
          <button type="button" aria-label={t('refresh')} onClick={onRefresh} style={{ marginLeft: 'auto', width: 30, height: 30, border: 'none', borderRadius: 6, background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', display: 'grid', placeItems: 'center' }}>
            <ArrowPathIcon style={{ width: 17, height: 17 }} />
          </button>
          <button type="button" aria-label={t('newDM')} onClick={onToggleNewChat} style={{ width: 30, height: 30, border: '1px solid var(--color-border-default)', borderRadius: 6, background: newChatOpen ? 'var(--color-accent)' : 'var(--color-bg-primary)', color: newChatOpen ? '#fff' : 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
            <PlusIcon style={{ width: 17, height: 17 }} />
          </button>
        </div>
        {newChatOpen && (
          <div style={{ marginTop: 12, border: '1px solid var(--color-border-subtle)', borderRadius: 8, background: 'var(--color-bg-primary)', padding: 10 }}>
            <div style={{ display: 'flex', gap: 6, marginBottom: 8 }}>
              <button type="button" onClick={() => onSetRoomType('direct')} style={pillButton(roomType === 'direct')}>{t('direct')}</button>
              <button type="button" onClick={() => onSetRoomType('group')} style={pillButton(roomType === 'group')}>{t('group')}</button>
              {roomType === 'group' && (
                <button type="button" onClick={onToggleVisibility} style={pillButton(visibility === 'public')}>
                  {visibility === 'public' ? t('public') : t('private')}
                </button>
              )}
            </div>
            {roomType === 'group' && (
              <input
                value={roomName}
                onChange={(e) => onSetRoomName(e.currentTarget.value)}
                placeholder={t('roomName')}
                style={{ width: '100%', boxSizing: 'border-box', marginBottom: 8, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '7px 9px', fontSize: 13 }}
              />
            )}
            <div style={{ display: 'flex', gap: 6 }}>
              <input
                value={directoryQuery}
                onChange={(e) => onSetDirectoryQuery(e.currentTarget.value)}
                onKeyDown={onDirectoryKeyDown}
                placeholder={t('searchPeople')}
                style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '7px 9px', fontSize: 13 }}
              />
              <button type="button" onClick={onCreateRoom} disabled={selectedUsers.length === 0 || (roomType === 'group' && !roomName.trim())} aria-label={t('createRoom')} style={{ width: 34, border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer', opacity: selectedUsers.length === 0 || (roomType === 'group' && !roomName.trim()) ? 0.55 : 1 }}>
                <PlusIcon style={{ width: 17, height: 17 }} />
              </button>
            </div>
            {selectedUsers.length > 0 && (
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: 5, marginTop: 8 }}>
                {selectedUsers.map((user) => (
                  <button key={user.id} type="button" onClick={() => onRemoveSelectedUser(user.id)} style={{ border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-secondary)', borderRadius: 6, padding: '3px 7px 3px 4px', fontSize: 12, cursor: 'pointer', display: 'inline-flex', alignItems: 'center', gap: 5, maxWidth: '100%' }}>
                    <MemberAvatar member={directoryUserToDMUser(user)} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} size={18} label={user.display_name || user.email} />
                    {user.display_name || user.email}
                  </button>
                ))}
              </div>
            )}
            {directoryUsers.length > 0 && (
              <div style={{ marginTop: 8, maxHeight: 150, overflow: 'auto', border: '1px solid var(--color-border-subtle)', borderRadius: 6, background: 'var(--color-bg-primary)' }}>
                {directoryUsers.map((user, index) => (
                  <button key={user.id} type="button" onMouseEnter={() => onDirectoryHover(index)} onClick={() => onAddDirectoryUser(user)} style={{ width: '100%', textAlign: 'left', border: 'none', borderBottom: '1px solid var(--color-border-subtle)', background: index === directoryActiveIndex ? 'var(--color-accent-subtle)' : 'transparent', color: 'var(--color-text-primary)', padding: '8px 9px', cursor: 'pointer', display: 'flex', alignItems: 'center', gap: 9 }}>
                    <MemberAvatar member={directoryUserToDMUser(user)} currentUserId={currentUserId} selfAvatarUrl={selfAvatarUrl} size={30} label={user.display_name || user.email} />
                    <span style={{ flex: 1, minWidth: 0 }}>
                      <span style={{ display: 'block', fontSize: 13, fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{user.display_name || user.email}</span>
                      <span style={{ display: 'block', fontSize: 11, color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{user.email}{user.org_unit_name ? ` · ${user.org_unit_name}` : ''}</span>
                    </span>
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
            onClick={() => onSelectRoom(room.id)}
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
              <button key={room.id} type="button" onClick={() => onSelectRoom(room.id)} style={{ width: '100%', border: 'none', borderTop: '1px solid var(--color-border-subtle)', background: 'transparent', color: 'var(--color-text-primary)', padding: '9px 14px', textAlign: 'left', cursor: 'pointer' }}>
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
  );
}
