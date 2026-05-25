'use client';

import type { MouseEvent as ReactMouseEvent } from 'react';
import { useCallback, useRef, useState } from 'react';
import {
  ArrowLeftIcon,
  ChatBubbleLeftRightIcon,
  EllipsisHorizontalIcon,
  InformationCircleIcon,
  MagnifyingGlassIcon,
  PaperClipIcon,
  XMarkIcon,
} from '@heroicons/react/24/outline';
import { exportDMRoom } from '@/lib/api/dm';
import { DMRoomList, RoomAvatar } from './dm/DMRoomList';
import { DMMessageList } from './dm/DMMessageList';
import { DMComposer } from './dm/DMComposer';
import { DMDetailsPanel } from './dm/DMDetailsPanel';
import { DMOverlays } from './dm/DMOverlays';
import { useDMPanel } from './dm/useDMPanel';

type DMPanelProps = {
  userEmail?: string;
  onUnreadChange?: (count: number) => void;
  onClose?: () => void;
  onComposeToAddress?: (email: string) => void;
  onStartWindowDrag?: (event: ReactMouseEvent<HTMLElement>) => void;
};

function formatTime(value?: string): string {
  if (!value) return '';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '';
  return date.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
}

export function DMPanel({ userEmail, onUnreadChange, onClose, onComposeToAddress, onStartWindowDrag }: DMPanelProps) {
  const dm = useDMPanel({ onUnreadChange });
  const { t } = dm;

  const [showMoreMenu, setShowMoreMenu] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [exportError, setExportError] = useState<string | null>(null);
  const exportingRef = useRef(false);

  const handleExportRoom = useCallback(async () => {
    if (!dm.activeRoom || exportingRef.current) return;
    setShowMoreMenu(false);
    setExportError(null);
    setExporting(true);
    exportingRef.current = true;
    try {
      const blob = await exportDMRoom(dm.activeRoom.id);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      const title = dm.titleForRoom(dm.activeRoom).replace(/[^a-z0-9\-_ ]/gi, '_').slice(0, 60);
      a.download = `dm-${title}.txt`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch {
      setExportError(t('exportError'));
    } finally {
      setExporting(false);
      exportingRef.current = false;
    }
  }, [dm.activeRoom, t]);

  const handleWindowHeaderMouseDown = useCallback((event: ReactMouseEvent<HTMLElement>) => {
    const target = event.target as HTMLElement;
    if (target.closest('button,input,textarea,a,[role="button"],[role="menuitem"]')) return;
    onStartWindowDrag?.(event);
  }, [onStartWindowDrag]);

  return (
    <div style={{ flex: 1, minWidth: 0, display: 'flex', height: '100%', overflow: 'hidden', background: 'var(--color-bg-primary)', position: 'relative' }}>
      <DMRoomList
        rooms={dm.rooms}
        publicRooms={dm.publicRooms}
        activeRoomId={dm.activeRoomId}
        loadingRooms={dm.loadingRooms}
        currentUserId={dm.currentUserId}
        selfAvatarUrl={dm.selfAvatarUrl}
        unread={dm.unread}
        newChatOpen={dm.newChatOpen}
        roomType={dm.roomType}
        roomName={dm.roomName}
        visibility={dm.visibility}
        directoryQuery={dm.directoryQuery}
        directoryUsers={dm.directoryUsers}
        directoryActiveIndex={dm.directoryActiveIndex}
        selectedUsers={dm.selectedUsers}
        onSelectRoom={(roomId) => { dm.setActiveRoomId(roomId); dm.setInviteUrl(''); }}
        onRefresh={() => { void dm.loadRooms(); void dm.loadMessages(); }}
        onToggleNewChat={() => dm.setNewChatOpen((open) => !open)}
        onSetRoomType={dm.setRoomType}
        onToggleVisibility={() => dm.setVisibility((v) => v === 'private' ? 'public' : 'private')}
        onSetRoomName={dm.setRoomName}
        onSetDirectoryQuery={dm.setDirectoryQuery}
        onDirectoryKeyDown={dm.handleDirectoryKeyDown}
        onDirectoryHover={dm.setDirectoryActiveIndex}
        onAddDirectoryUser={dm.addDirectoryUser}
        onRemoveSelectedUser={(userId) => dm.setSelectedUsers((prev) => prev.filter((item) => item.id !== userId))}
        onCreateRoom={dm.createRoom}
        onStartWindowDrag={onStartWindowDrag}
        titleForRoom={dm.titleForRoom}
        previewForMessage={dm.previewForMessage}
        t={t}
      />

      <main style={{ flex: 1, minWidth: 0, display: dm.activeRoom ? 'flex' : 'none', flexDirection: 'column', height: '100%' }}>
        <header
          onMouseDown={handleWindowHeaderMouseDown}
          style={{ minHeight: 58, borderBottom: '1px solid var(--color-border-subtle)', display: 'flex', alignItems: 'center', gap: 8, padding: '8px 10px', flexShrink: 0, flexWrap: 'wrap', cursor: onStartWindowDrag ? 'move' : 'default' }}
        >
          <div style={{ minWidth: 0, flex: '1 1 180px', display: 'flex', alignItems: 'center', gap: 8 }}>
            <button type="button" onClick={() => { dm.setActiveRoomId(''); dm.setDetailsOpen(false); dm.setSearchQuery(''); }} aria-label={t('backToList')} style={{ width: 32, height: 32, border: 'none', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer', flexShrink: 0 }}>
              <ArrowLeftIcon style={{ width: 18, height: 18 }} />
            </button>
            {dm.activeRoom && <RoomAvatar room={dm.activeRoom} currentUserId={dm.currentUserId} selfAvatarUrl={dm.selfAvatarUrl} />}
            <div style={{ minWidth: 0 }}>
              <div style={{ fontSize: 15, fontWeight: 700, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{dm.activeRoom ? dm.titleForRoom(dm.activeRoom) : t('title')}</div>
              <div style={{ fontSize: 12, color: 'var(--color-text-tertiary)' }}>{dm.activeRoom ? t('membersCount', { count: dm.activeRoom.members?.length ?? dm.activeRoom.member_count ?? 0 }) : userEmail}</div>
            </div>
          </div>
          <div style={{ display: 'flex', gap: 6, alignItems: 'center', flex: '1 1 180px', justifyContent: 'flex-end', minWidth: 0 }}>
            {dm.activeRoom && (
              <>
                <div style={{ position: 'relative', flex: '1 1 110px', minWidth: 0, maxWidth: 220 }}>
                  <MagnifyingGlassIcon style={{ position: 'absolute', left: 8, top: 7, width: 15, height: 15, color: 'var(--color-text-tertiary)' }} />
                  <input value={dm.searchQuery} onChange={(e) => dm.setSearchQuery(e.currentTarget.value)} placeholder={t('search')} style={{ width: '100%', boxSizing: 'border-box', border: '1px solid var(--color-border-default)', background: 'var(--color-bg-secondary)', color: 'var(--color-text-primary)', borderRadius: 6, padding: '6px 9px 6px 28px', fontSize: 13 }} />
                </div>
                <button type="button" onClick={() => dm.fileInputRef.current?.click()} disabled={!dm.activeRoomId} aria-label={t('attachFile')} style={{ width: 32, height: 32, border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                  <PaperClipIcon style={{ width: 17, height: 17 }} />
                </button>
                <button type="button" onClick={() => dm.setDetailsOpen((open) => !open)} aria-label={t('conversationDetails')} style={{ width: 32, height: 32, border: '1px solid var(--color-border-default)', borderRadius: 6, background: dm.detailsOpen ? 'var(--color-accent-subtle)' : 'transparent', color: dm.detailsOpen ? 'var(--color-accent)' : 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                  <InformationCircleIcon style={{ width: 17, height: 17 }} />
                </button>
                <div style={{ position: 'relative' }}>
                  <button
                    type="button"
                    onClick={() => setShowMoreMenu((v) => !v)}
                    aria-label="More options"
                    style={{ width: 32, height: 32, border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}
                  >
                    <EllipsisHorizontalIcon style={{ width: 17, height: 17 }} />
                  </button>
                  {showMoreMenu && (
                    <>
                      <div
                        role="presentation"
                        style={{ position: 'fixed', inset: 0, zIndex: 10 }}
                        onClick={() => setShowMoreMenu(false)}
                      />
                      <div style={{ position: 'absolute', right: 0, top: '100%', marginTop: 4, width: 192, background: 'var(--color-bg-primary)', border: '1px solid var(--color-border-default)', borderRadius: 8, boxShadow: '0 4px 16px rgba(0,0,0,0.12)', zIndex: 20, overflow: 'hidden' }}>
                        <button
                          type="button"
                          onClick={() => { void handleExportRoom(); }}
                          disabled={exporting}
                          style={{ width: '100%', textAlign: 'left', padding: '10px 16px', fontSize: 13, background: 'transparent', border: 'none', color: 'var(--color-text-primary)', cursor: exporting ? 'default' : 'pointer', opacity: exporting ? 0.5 : 1 }}
                        >
                          {exporting ? t('exportDownloading') : t('exportRoom')}
                        </button>
                      </div>
                    </>
                  )}
                </div>
              </>
            )}
            {onClose && (
              <button type="button" onClick={onClose} aria-label={t('close')} style={{ width: 32, height: 32, border: '1px solid var(--color-border-default)', borderRadius: 6, background: 'transparent', color: 'var(--color-text-secondary)', display: 'grid', placeItems: 'center', cursor: 'pointer' }}>
                <XMarkIcon style={{ width: 17, height: 17 }} />
              </button>
            )}
            <input ref={dm.fileInputRef} type="file" style={{ display: 'none' }} onChange={(event) => {
              const file = event.currentTarget.files?.[0];
              event.currentTarget.value = '';
              if (file) void dm.uploadFile(file);
            }} />
          </div>
        </header>

        {dm.error && (
          <div role="alert" style={{ padding: '8px 16px', borderBottom: '1px solid var(--color-border-subtle)', color: 'var(--color-destructive)', fontSize: 12, flexShrink: 0 }}>
            {dm.error}
          </div>
        )}

        {exportError && (
          <div role="alert" style={{ padding: '8px 16px', borderBottom: '1px solid var(--color-border-subtle)', color: 'var(--color-destructive)', fontSize: 12, flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <span>{exportError}</span>
            <button type="button" onClick={() => setExportError(null)} style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-destructive)', fontSize: 12, padding: '0 4px' }}>×</button>
          </div>
        )}

        {dm.activeRoom ? (
          <div style={{ flex: 1, minHeight: 0, display: 'grid', gridTemplateColumns: dm.detailsOpen ? 'minmax(0, 1fr) minmax(170px, 34%)' : 'minmax(0, 1fr)' }}>
            <section style={{ display: 'flex', flexDirection: 'column', minWidth: 0, minHeight: 0 }}>
              <DMMessageList
                messages={dm.messages}
                currentUserId={dm.currentUserId}
                selfAvatarUrl={dm.selfAvatarUrl}
                loadingMessages={dm.loadingMessages}
                messagesEndRef={dm.messageEndRef}
                reactionPickerMessageId={dm.reactionPickerMessageId}
                reactionPickerRef={dm.reactionPickerRef}
                editingId={dm.editingId}
                editingBody={dm.editingBody}
                memberById={dm.memberById}
                onSetPreviewImage={dm.setPreviewImage}
                onSetImageMenu={dm.setImageMenu}
                onSetEditingId={dm.setEditingId}
                onSetEditingBody={dm.setEditingBody}
                onSetReactionPickerMessageId={dm.setReactionPickerMessageId}
                onToggleReaction={dm.toggleReaction}
                onSubmitEdit={dm.submitEdit}
                onRemoveMessage={dm.removeMessage}
                onCopyImage={dm.copyImageToClipboard}
                t={t}
              />
              <DMComposer
                composer={dm.composer}
                driveFileId={dm.driveFileId}
                driveComposerOpen={dm.driveComposerOpen}
                activeRoomId={dm.activeRoomId}
                composerComposingRef={dm.composerComposingRef}
                onChangeComposer={(value) => { dm.setComposer(value); dm.persistDraft(dm.activeRoomId, value, dm.driveFileId); }}
                onChangeDriveFileId={(value) => { dm.setDriveFileId(value); dm.persistDraft(dm.activeRoomId, dm.composer, value); }}
                onToggleDriveComposer={() => dm.setDriveComposerOpen((open) => !open)}
                onSend={dm.send}
                onPaste={dm.uploadPastedImages}
                onCompositionStart={() => { dm.composerComposingRef.current = true; }}
                onCompositionEnd={() => { dm.composerComposingRef.current = false; }}
                t={t}
              />
            </section>

            {dm.detailsOpen && (
              <DMDetailsPanel
                activeRoom={dm.activeRoom}
                currentUserId={dm.currentUserId}
                selfAvatarUrl={dm.selfAvatarUrl}
                inviteUrl={dm.inviteUrl}
                memberInput={dm.memberInput}
                ownerInput={dm.ownerInput}
                mediaTab={dm.mediaTab}
                mediaItems={dm.mediaItems}
                mediaTabLabels={dm.mediaTabLabels}
                memberInputRef={dm.memberInputRef}
                onMakeInvite={dm.makeInvite}
                onAddMembers={dm.addMembers}
                onTransferOwner={dm.transferOwner}
                onSetMemberInput={dm.setMemberInput}
                onSetOwnerInput={dm.setOwnerInput}
                onSetMediaTab={dm.setMediaTab}
                onLeaveOrRemove={dm.leaveOrRemove}
                onComposeToAddress={onComposeToAddress}
                titleForRoom={dm.titleForRoom}
                t={t}
              />
            )}
          </div>
        ) : (
          <div style={{ flex: 1, display: 'grid', placeItems: 'center', color: 'var(--color-text-tertiary)', fontSize: 14 }}>
            <div style={{ textAlign: 'center', maxWidth: 280, lineHeight: 1.5 }}>
              <ChatBubbleLeftRightIcon style={{ width: 42, height: 42, color: 'var(--color-text-tertiary)', marginBottom: 10 }} />
              <div style={{ color: 'var(--color-text-primary)', fontWeight: 700, marginBottom: 4 }}>{t('selectTitle')}</div>
              <div style={{ marginBottom: 14 }}>{t('selectDesc')}</div>
              <button type="button" onClick={() => dm.setNewChatOpen(true)} style={{ border: 'none', borderRadius: 6, background: 'var(--color-accent)', color: '#fff', padding: '8px 12px', fontSize: 13, fontWeight: 700, cursor: 'pointer' }}>{t('newChat')}</button>
            </div>
          </div>
        )}

        {dm.searchResults.length > 0 && (
          <div style={{ position: 'absolute', top: 64, right: 12, width: 'min(320px, calc(100% - 24px))', maxHeight: 260, overflow: 'auto', zIndex: 70, border: '1px solid var(--color-border-default)', background: 'var(--color-bg-primary)', boxShadow: '0 12px 32px rgba(0,0,0,0.12)', borderRadius: 8 }}>
            {dm.searchResults.map((message) => (
              <button key={message.id} type="button" onClick={() => dm.setSearchQuery('')} style={{ display: 'block', width: '100%', border: 'none', borderBottom: '1px solid var(--color-border-subtle)', background: 'transparent', color: 'var(--color-text-primary)', padding: 10, textAlign: 'left', cursor: 'pointer' }}>
                <span style={{ display: 'block', fontSize: 12, color: 'var(--color-text-tertiary)' }}>{formatTime(message.created_at)}</span>
                <span style={{ display: 'block', fontSize: 13, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{dm.previewForMessage(message)}</span>
              </button>
            ))}
          </div>
        )}
      </main>

      <DMOverlays
        previewImage={dm.previewImage}
        imageMenu={dm.imageMenu}
        pendingPasteFile={dm.pendingPasteFile}
        pendingPastePreview={dm.pendingPastePreview}
        notice={dm.notice}
        onClosePreview={() => dm.setPreviewImage(null)}
        onCopyImage={dm.copyImageToClipboard}
        onSetImageMenu={dm.setImageMenu}
        onCancelPaste={() => dm.setPendingPasteFile(null)}
        onConfirmPaste={dm.confirmPendingPaste}
        t={t}
      />
    </div>
  );
}
