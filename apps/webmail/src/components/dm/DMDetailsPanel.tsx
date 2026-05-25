'use client';
import { type DMTFunction } from './types';

import type { RefObject } from 'react';
import type { DMMediaItem, DMRoom, DMUser } from '@/lib/api';
import { UserPlusIcon, TrashIcon } from '@heroicons/react/24/outline';
import { MemberAvatar, RoomAvatar } from './DMRoomList';

type MediaTab = 'files' | 'links' | 'drive';

function formatBytes(size?: number): string {
  if (!size || size <= 0) return '';
  if (size < 1024) return `${size} B`;
  if (size < 1024 * 1024) return `${(size / 1024).toFixed(1)} KB`;
  return `${(size / 1024 / 1024).toFixed(1)} MB`;
}

function memberName(member?: DMUser, fallback = ''): string {
  return member?.display_name || member?.id || fallback;
}

function memberEmail(member?: DMUser): string {
  return member?.email || '';
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

type DMDetailsPanelProps = {
  activeRoom: DMRoom;
  currentUserId: string;
  selfAvatarUrl: string;
  inviteUrl: string;
  memberInput: string;
  ownerInput: string;
  mediaTab: MediaTab;
  mediaItems: DMMediaItem[];
  mediaTabLabels: Record<MediaTab, string>;
  memberInputRef: RefObject<HTMLInputElement | null>;
  onMakeInvite: () => void;
  onAddMembers: () => void;
  onTransferOwner: () => void;
  onSetMemberInput: (value: string) => void;
  onSetOwnerInput: (value: string) => void;
  onSetMediaTab: (tab: MediaTab) => void;
  onLeaveOrRemove: (userId: string) => void;
  onComposeToAddress?: (email: string) => void;
  titleForRoom: (room: DMRoom) => string;
  t: DMTFunction;
};

export function DMDetailsPanel({
  activeRoom,
  currentUserId,
  selfAvatarUrl,
  inviteUrl,
  memberInput,
  ownerInput,
  mediaTab,
  mediaItems,
  mediaTabLabels,
  memberInputRef,
  onMakeInvite,
  onAddMembers,
  onTransferOwner,
  onSetMemberInput,
  onSetOwnerInput,
  onSetMediaTab,
  onLeaveOrRemove,
  onComposeToAddress,
  titleForRoom,
  t,
}: DMDetailsPanelProps) {
  return (
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
            <button type="button" onClick={onMakeInvite} style={{ width: '100%', border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'var(--color-bg-primary)', color: 'var(--color-text-secondary)', padding: '7px 9px', fontSize: 12, cursor: 'pointer' }}>{t('createInvite')}</button>
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
                <button type="button" onClick={() => onLeaveOrRemove(member.id)} aria-label={t('removeMember')} style={{ border: 'none', background: 'transparent', color: 'var(--color-text-tertiary)', cursor: 'pointer', padding: 2 }}>
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
            <button key={tab} type="button" onClick={() => onSetMediaTab(tab)} style={pillButton(mediaTab === tab)}>{mediaTabLabels[tab]}</button>
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
      {activeRoom.room_type === 'group' && (
        <div style={{ padding: 12, borderBottom: '1px solid var(--color-border-subtle)' }}>
          <div style={{ display: 'flex', gap: 6, marginBottom: 8 }}>
            <input ref={memberInputRef} value={memberInput} onChange={(e) => onSetMemberInput(e.currentTarget.value)} placeholder={t('userIds')} style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '6px 8px', fontSize: 12 }} />
            <button type="button" onClick={onAddMembers} aria-label={t('addMembers')} style={{ width: 30, border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
              <UserPlusIcon style={{ width: 15, height: 15 }} />
            </button>
          </div>
          <div style={{ display: 'flex', gap: 6 }}>
            <input value={ownerInput} onChange={(e) => onSetOwnerInput(e.currentTarget.value)} placeholder={t('ownerUserId')} style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '6px 8px', fontSize: 12 }} />
            <button type="button" onClick={onTransferOwner} style={{ border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', padding: '0 8px', fontSize: 12, cursor: 'pointer' }}>{t('owner')}</button>
          </div>
        </div>
      )}
    </aside>
  );
}
