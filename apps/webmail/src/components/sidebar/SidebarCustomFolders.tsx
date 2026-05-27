'use client';

import type { CSSProperties } from 'react';
import { useTranslations } from 'next-intl';
import type { Folder } from '@/lib/api';
import {
  FolderIcon,
  PencilIcon,
  TrashIcon,
  XMarkIcon,
  CheckIcon,
} from '@heroicons/react/24/outline';
import { handleVerticalNavKeyDown } from '@/lib/navKeyboard';

function formatBadge(count: number): string {
  if (count <= 0) return '';
  if (count > 99) return '99+';
  return String(count);
}

interface SidebarCustomFoldersProps {
  folders: Folder[];
  systemFolderIds: Set<string>;
  activeFolderId: string;
  onSelectFolder: (id: string) => void;
  onRenameFolder?: (id: string, name: string) => void;
  onDeleteFolder?: (id: string) => void;
  onCreateFolder?: (name: string) => void;
  onDropMessage?: (messageId: string, folderId: string) => void;
  dragOverFolderId: string | null;
  setDragOverFolderId: (id: string | null) => void;
  renamingFolderId: string | null;
  setRenamingFolderId: (id: string | null) => void;
  renamingValue: string;
  setRenamingValue: (v: string) => void;
  hoveredFolderId: string | null;
  setHoveredFolderId: (id: string | null) => void;
  showNewFolder: boolean;
  setShowNewFolder: (v: boolean) => void;
  newFolderInput: string;
  setNewFolderInput: (v: string) => void;
}

export function SidebarCustomFolders({
  folders,
  systemFolderIds,
  activeFolderId,
  onSelectFolder,
  onRenameFolder,
  onDeleteFolder,
  onCreateFolder,
  onDropMessage,
  dragOverFolderId,
  setDragOverFolderId,
  renamingFolderId,
  setRenamingFolderId,
  renamingValue,
  setRenamingValue,
  hoveredFolderId,
  setHoveredFolderId,
  showNewFolder,
  setShowNewFolder,
  newFolderInput,
  setNewFolderInput,
}: SidebarCustomFoldersProps) {
  const t = useTranslations('sidebar');
  const customFolders = folders.filter((f) => !systemFolderIds.has(f.id));

  return (
    <>
      {(customFolders.length > 0 || onCreateFolder) && (
        <div style={{ padding: '12px 16px 4px', fontSize: '11px', fontWeight: 600, letterSpacing: '0.06em', textTransform: 'uppercase', color: 'var(--color-text-tertiary)' }}>
          <span>{t('personalFolders')}</span>
        </div>
      )}

      {customFolders.map((f) => {
        const isActive = activeFolderId === f.id;
        const badge = formatBadge(f.unread);
        const isRenaming = renamingFolderId === f.id;
        const isHovered = hoveredFolderId === f.id;
        return (
          <div
            key={f.id}
            style={{ position: 'relative', marginInline: '4px 2px' }}
            onMouseEnter={() => setHoveredFolderId(f.id)}
            onMouseLeave={() => setHoveredFolderId(null)}
          >
            {isRenaming ? (
              <div style={{ display: 'flex', alignItems: 'center', gap: '4px', padding: '4px 8px' }}>
                <input
                  autoFocus
                  value={renamingValue}
                  onChange={(e) => setRenamingValue(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === 'Enter' && renamingValue.trim()) { onRenameFolder?.(f.id, renamingValue.trim()); setRenamingFolderId(null); }
                    if (e.key === 'Escape') setRenamingFolderId(null);
                  }}
                  style={{ flex: 1, fontSize: '13px', padding: '3px 6px', border: '1px solid var(--color-accent)', borderRadius: '4px', outline: 'none', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)' }}
                />
                <button
                  onClick={() => { if (renamingValue.trim()) { onRenameFolder?.(f.id, renamingValue.trim()); setRenamingFolderId(null); } }}
                  style={{ padding: '3px 6px', border: 'none', background: 'var(--color-accent)', color: '#fff', borderRadius: '4px', cursor: 'pointer', display: 'inline-flex' }}
                >
                  <CheckIcon style={{ width: '12px', height: '12px' }} />
                </button>
                <button
                  onClick={() => setRenamingFolderId(null)}
                  style={{ padding: '3px 6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', borderRadius: '4px', cursor: 'pointer', display: 'inline-flex' }}
                >
                  <XMarkIcon style={{ width: '12px', height: '12px' }} />
                </button>
              </div>
            ) : (
              <button
                onClick={() => onSelectFolder(f.id)}
                data-nav-group="sidebar-nav"
                data-nav-current={isActive ? 'true' : undefined}
                onKeyDown={(e) => handleVerticalNavKeyDown(e, 'sidebar-nav')}
                aria-current={isActive ? 'page' : undefined}
                style={{
                  width: '100%',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  gap: '8px',
                  padding: '7px 12px',
                  border: dragOverFolderId === f.id ? '1px solid var(--color-accent)' : '1px solid transparent',
                  background: dragOverFolderId === f.id ? 'var(--color-accent-subtle)' : isActive ? 'var(--color-bg-tertiary)' : 'transparent',
                  color: isActive ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                  fontSize: '14px',
                  fontWeight: isActive ? 500 : 400,
                  cursor: 'pointer',
                  borderRadius: '4px',
                  transition: 'background 80ms ease, border 80ms ease',
                } as CSSProperties}
                onDragOver={(e) => { if (onDropMessage) { e.preventDefault(); setDragOverFolderId(f.id); } }}
                onDragLeave={() => setDragOverFolderId(null)}
                onDrop={(e) => { e.preventDefault(); setDragOverFolderId(null); const id = e.dataTransfer.getData('text/plain'); if (id && onDropMessage) onDropMessage(id, f.id); }}
              >
                <span style={{ display: 'flex', alignItems: 'center', gap: '8px', minWidth: 0, flex: 1, overflow: 'hidden' }}>
                  <span style={{ flexShrink: 0, display: 'inline-flex', opacity: 0.65 }}>
                    <FolderIcon style={{ width: '14px', height: '14px' }} />
                  </span>
                  <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{f.name}</span>
                </span>
                {isHovered && (onRenameFolder || onDeleteFolder) ? (
                  <span style={{ display: 'flex', gap: '2px', flexShrink: 0, marginInlineStart: '4px' }}>
                    {onRenameFolder && (
                      <span
                        onClick={(e) => { e.stopPropagation(); setRenamingValue(f.name); setRenamingFolderId(f.id); }}
                        style={{ padding: '2px 4px', borderRadius: '3px', cursor: 'pointer', color: 'var(--color-text-tertiary)', display: 'inline-flex', alignItems: 'center' }}
                        title={t('rename')}
                      >
                        <PencilIcon style={{ width: '12px', height: '12px' }} />
                      </span>
                    )}
                    {onDeleteFolder && (
                      <span
                        onClick={(e) => { e.stopPropagation(); if (window.confirm(t('deleteFolderConfirm', { name: f.name }))) onDeleteFolder(f.id); }}
                        style={{ padding: '2px 4px', borderRadius: '3px', cursor: 'pointer', color: 'var(--color-destructive)', display: 'inline-flex', alignItems: 'center' }}
                        title={t('delete')}
                      >
                        <TrashIcon style={{ width: '12px', height: '12px' }} />
                      </span>
                    )}
                  </span>
                ) : badge ? (
                  <span style={{ fontSize: '12px', color: 'var(--color-text-secondary)', background: 'var(--color-bg-tertiary)', borderRadius: '10px', padding: '1px 6px', flexShrink: 0, marginInlineStart: '8px' }}>
                    {badge}
                  </span>
                ) : null}
              </button>
            )}
          </div>
        );
      })}

      {onCreateFolder && (
        <div style={{ marginInline: '4px' }}>
          {showNewFolder ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: '4px', padding: '4px 8px' }}>
              <input
                autoFocus
                value={newFolderInput}
                onChange={(e) => setNewFolderInput(e.target.value)}
                placeholder={t('folderNamePlaceholder')}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && newFolderInput.trim()) { onCreateFolder(newFolderInput.trim()); setNewFolderInput(''); setShowNewFolder(false); }
                  if (e.key === 'Escape') { setShowNewFolder(false); setNewFolderInput(''); }
                }}
                style={{ flex: 1, fontSize: '13px', padding: '3px 6px', border: '1px solid var(--color-accent)', borderRadius: '4px', outline: 'none', background: 'var(--color-bg-primary)', color: 'var(--color-text-primary)' }}
              />
              <button
                onClick={() => { if (newFolderInput.trim()) { onCreateFolder(newFolderInput.trim()); setNewFolderInput(''); setShowNewFolder(false); } }}
                style={{ padding: '3px 6px', border: 'none', background: 'var(--color-accent)', color: '#fff', borderRadius: '4px', cursor: 'pointer', display: 'inline-flex' }}
              >
                <CheckIcon style={{ width: '12px', height: '12px' }} />
              </button>
              <button
                onClick={() => { setShowNewFolder(false); setNewFolderInput(''); }}
                style={{ padding: '3px 6px', border: '1px solid var(--color-border-default)', background: 'transparent', color: 'var(--color-text-secondary)', borderRadius: '4px', cursor: 'pointer', display: 'inline-flex' }}
              >
                <XMarkIcon style={{ width: '12px', height: '12px' }} />
              </button>
            </div>
          ) : (
            <button
              onClick={() => setShowNewFolder(true)}
              data-nav-group="sidebar-nav"
              onKeyDown={(e) => handleVerticalNavKeyDown(e, 'sidebar-nav')}
              style={{ width: '100%', textAlign: 'left', padding: '5px 12px', border: '1px solid transparent', background: 'transparent', color: 'var(--color-text-tertiary)', fontSize: '13px', cursor: 'pointer', borderRadius: '4px', display: 'flex', alignItems: 'center', gap: '6px' }}
              onMouseEnter={(e) => { e.currentTarget.style.color = 'var(--color-text-secondary)'; }}
              onMouseLeave={(e) => { e.currentTarget.style.color = 'var(--color-text-tertiary)'; }}
            >
              <span>+</span> {t('addFolder')}
            </button>
          )}
        </div>
      )}
    </>
  );
}
