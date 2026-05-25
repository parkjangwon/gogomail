'use client';
import { type DMTFunction } from './types';

import type { RefObject } from 'react';
import type { DMMessage, DMRoom, DMUser } from '@/lib/api';
import {
  ArrowDownTrayIcon,
  ClipboardDocumentIcon,
  FaceSmileIcon,
  LinkIcon,
  PaperClipIcon,
  TrashIcon,
} from '@heroicons/react/24/outline';
import { MemberAvatar } from './DMRoomList';

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

function memberName(member?: DMUser, fallback = ''): string {
  return member?.display_name || member?.id || fallback;
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

type DMMessageListProps = {
  messages: DMMessage[];
  currentUserId: string;
  selfAvatarUrl: string;
  loadingMessages: boolean;
  messagesEndRef: RefObject<HTMLDivElement | null>;
  reactionPickerMessageId: string | null;
  reactionPickerRef: RefObject<HTMLSpanElement | null>;
  editingId: string | null;
  editingBody: string;
  memberById: Map<string, DMUser>;
  onSetPreviewImage: (message: DMMessage) => void;
  onSetImageMenu: (menu: { message: DMMessage; x: number; y: number } | null) => void;
  onSetEditingId: (id: string | null) => void;
  onSetEditingBody: (body: string) => void;
  onSetReactionPickerMessageId: (id: string | null) => void;
  onToggleReaction: (messageId: string, emoji: string) => void;
  onSubmitEdit: () => void;
  onRemoveMessage: (messageId: string) => void;
  onCopyImage: (message: DMMessage) => void;
  t: DMTFunction;
};

export function DMMessageList({
  messages,
  currentUserId,
  selfAvatarUrl,
  loadingMessages,
  messagesEndRef,
  reactionPickerMessageId,
  reactionPickerRef,
  editingId,
  editingBody,
  memberById,
  onSetPreviewImage,
  onSetImageMenu,
  onSetEditingId,
  onSetEditingBody,
  onSetReactionPickerMessageId,
  onToggleReaction,
  onSubmitEdit,
  onRemoveMessage,
  onCopyImage,
  t,
}: DMMessageListProps) {
  return (
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
                    <input value={editingBody} onChange={(e) => onSetEditingBody(e.currentTarget.value)} style={{ flex: 1, minWidth: 0, border: '1px solid var(--color-border-default)', borderRadius: 5, padding: '5px 7px', fontSize: 13 }} />
                    <button type="button" onClick={onSubmitEdit} style={{ border: 'none', borderRadius: 5, background: 'var(--color-accent)', color: '#fff', padding: '0 9px', fontSize: 12, cursor: 'pointer' }}>{t('save')}</button>
                  </div>
                ) : imageMessage && imageSrc && !message.deleted_at ? (
                  <div>
                    <button type="button" onClick={() => onSetPreviewImage(message)} aria-label={t('openImage')} style={{ display: 'block', border: 'none', padding: 0, background: 'transparent', cursor: 'zoom-in', maxWidth: '100%' }}>
                      <img
                        src={imageSrc}
                        alt={message.attachment_name || message.body || t('imageAttachment')}
                        onContextMenu={(event) => {
                          event.preventDefault();
                          onSetImageMenu({ message, x: event.clientX, y: event.clientY });
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
                      <button type="button" onClick={() => onCopyImage(message)} aria-label={t('copyImage')} style={{ border: 'none', background: 'transparent', color: mine ? 'rgba(255,255,255,0.86)' : 'var(--color-accent)', padding: 1, cursor: 'pointer', display: 'inline-flex' }}>
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
                      <button key={reaction.emoji} type="button" onClick={() => onToggleReaction(message.id, reaction.emoji)} style={{ border: 'none', borderRadius: 10, padding: '1px 6px', background: reaction.mine ? 'var(--color-accent-subtle)' : mine ? 'rgba(255,255,255,0.18)' : 'var(--color-bg-tertiary)', color: mine ? '#fff' : reaction.mine ? 'var(--color-accent)' : 'var(--color-text-secondary)', fontSize: 11, cursor: 'pointer' }}>
                        {reaction.emoji}{reaction.count ? ` ${reaction.count}` : ''}
                      </button>
                    ))}
                    <span ref={reactionPickerMessageId === message.id ? reactionPickerRef : undefined} style={{ position: 'relative', display: 'inline-flex' }}>
                      <button type="button" onClick={() => onSetReactionPickerMessageId(reactionPickerMessageId === message.id ? null : message.id)} aria-label={t('react')} style={{ border: 'none', borderRadius: 10, padding: '1px 5px', background: mine ? 'rgba(255,255,255,0.18)' : 'var(--color-bg-tertiary)', color: mine ? '#fff' : 'var(--color-text-secondary)', cursor: 'pointer', display: 'inline-flex', alignItems: 'center' }}>
                        <FaceSmileIcon style={{ width: 13, height: 13 }} />
                      </button>
                      {reactionPickerMessageId === message.id && (
                        <span style={{ position: 'absolute', top: '100%', right: mine ? 0 : 'auto', left: mine ? 'auto' : 0, marginTop: 6, width: 230, padding: 8, border: '1px solid var(--color-border-default)', borderRadius: 8, background: 'var(--color-bg-primary)', boxShadow: '0 12px 32px rgba(0,0,0,0.16)', display: 'flex', flexWrap: 'wrap', gap: 3, zIndex: 90 }}>
                          {REACTION_EMOJI.map((emoji) => (
                            <button key={emoji} type="button" onClick={() => onToggleReaction(message.id, emoji)} style={{ width: 25, height: 25, border: 'none', borderRadius: 5, background: 'transparent', cursor: 'pointer', fontSize: 17, lineHeight: 1 }}>
                              {emoji}
                            </button>
                          ))}
                        </span>
                      )}
                    </span>
                    <button type="button" onClick={() => { onSetEditingId(message.id); onSetEditingBody(message.body); }} style={{ border: 'none', background: 'transparent', color: mine ? 'rgba(255,255,255,0.82)' : 'var(--color-text-tertiary)', fontSize: 11, cursor: 'pointer' }}>{t('edit')}</button>
                    <button type="button" onClick={() => onRemoveMessage(message.id)} aria-label={t('deleteMessage')} style={{ border: 'none', background: 'transparent', color: mine ? 'rgba(255,255,255,0.82)' : 'var(--color-text-tertiary)', cursor: 'pointer', padding: 0 }}>
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
      <div ref={messagesEndRef} />
    </div>
  );
}
