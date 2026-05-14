'use client';

import type { Dispatch, SetStateAction } from 'react';
import type { AddressBook } from '@/lib/api';
import {
  UserGroupIcon,
  PlusIcon,
  PencilIcon,
  TrashIcon,
  XMarkIcon,
  CheckIcon,
} from '@heroicons/react/24/outline';

interface ContactsSidebarProps {
  addressBooks: AddressBook[];
  selectedBookId: string | null;
  booksLoading: boolean;
  showNewBookInput: boolean;
  setShowNewBookInput: Dispatch<SetStateAction<boolean>>;
  newBookName: string;
  setNewBookName: Dispatch<SetStateAction<string>>;
  bookActionLoading: boolean;
  hoveredBookId: string | null;
  setHoveredBookId: Dispatch<SetStateAction<string | null>>;
  renamingBookId: string | null;
  setRenamingBookId: Dispatch<SetStateAction<string | null>>;
  renameValue: string;
  setRenameValue: Dispatch<SetStateAction<string>>;
  onSelectBook: (id: string) => void;
  onCreateBook: () => void;
  onRenameBook: (id: string) => void;
  onDeleteBook: (id: string, name: string) => void;
}

export function ContactsSidebar({
  addressBooks,
  selectedBookId,
  booksLoading,
  showNewBookInput,
  setShowNewBookInput,
  newBookName,
  setNewBookName,
  bookActionLoading,
  hoveredBookId,
  setHoveredBookId,
  renamingBookId,
  setRenamingBookId,
  renameValue,
  setRenameValue,
  onSelectBook,
  onCreateBook,
  onRenameBook,
  onDeleteBook,
}: ContactsSidebarProps) {
  return (
    <div
      style={{
        width: '200px',
        flexShrink: 0,
        borderRight: '1px solid var(--color-border-subtle)',
        display: 'flex',
        flexDirection: 'column',
        overflow: 'hidden',
      }}
    >
      <div
        style={{
          padding: '12px 16px',
          borderBottom: '1px solid var(--color-border-subtle)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          flexShrink: 0,
        }}
      >
        <span style={{ fontSize: '13px', fontWeight: 600, color: 'var(--color-text-primary)' }}>
          그룹
        </span>
        <button
          title="새 그룹"
          disabled={bookActionLoading}
          onClick={() => { setShowNewBookInput(true); setNewBookName(''); }}
          style={{
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            color: 'var(--color-text-tertiary)',
            padding: '2px',
            borderRadius: '4px',
            display: 'flex',
            alignItems: 'center',
          }}
          onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-primary)'; }}
          onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; }}
        >
          <PlusIcon style={{ width: '16px', height: '16px' }} />
        </button>
      </div>

      {showNewBookInput && (
        <div style={{ padding: '6px 10px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
          <input
            autoFocus
            type="text"
            placeholder="그룹 이름..."
            value={newBookName}
            onChange={(e) => setNewBookName(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter') { onCreateBook(); }
              else if (e.key === 'Escape') { setShowNewBookInput(false); setNewBookName(''); }
            }}
            style={{
              width: '100%',
              padding: '5px 8px',
              borderRadius: '5px',
              border: '1px solid var(--color-border-default)',
              background: 'var(--color-bg-secondary)',
              color: 'var(--color-text-primary)',
              fontSize: '12px',
              outline: 'none',
              boxSizing: 'border-box',
            }}
          />
          <div style={{ display: 'flex', gap: '4px', marginTop: '4px' }}>
            <button
              disabled={bookActionLoading || !newBookName.trim()}
              onClick={onCreateBook}
              style={{
                flex: 1, padding: '4px', fontSize: '11px', borderRadius: '4px',
                background: 'var(--color-accent)', color: '#fff', border: 'none', cursor: 'pointer',
                opacity: (!newBookName.trim() || bookActionLoading) ? 0.5 : 1,
              }}
            >
              추가
            </button>
            <button
              onClick={() => { setShowNewBookInput(false); setNewBookName(''); }}
              style={{
                flex: 1, padding: '4px', fontSize: '11px', borderRadius: '4px',
                background: 'var(--color-bg-secondary)', border: '1px solid var(--color-border-default)',
                cursor: 'pointer', color: 'var(--color-text-secondary)',
              }}
            >
              취소
            </button>
          </div>
        </div>
      )}

      <div style={{ flex: 1, overflowY: 'auto' }}>
        {booksLoading ? (
          <div style={{ padding: '16px', fontSize: '12px', color: 'var(--color-text-tertiary)', textAlign: 'center' }}>
            로딩 중...
          </div>
        ) : addressBooks.length === 0 && !showNewBookInput ? (
          <div style={{ padding: '16px', fontSize: '12px', color: 'var(--color-text-tertiary)', textAlign: 'center' }}>
            + 버튼으로 그룹을 추가하세요
          </div>
        ) : (
          addressBooks.map((book) => {
            const active = book.ID === selectedBookId;
            const hovered = hoveredBookId === book.ID;
            const renaming = renamingBookId === book.ID;
            return (
              <div
                key={book.ID}
                onMouseEnter={() => setHoveredBookId(book.ID)}
                onMouseLeave={() => setHoveredBookId(null)}
                style={{
                  position: 'relative',
                  background: active ? 'var(--color-bg-tertiary)' : hovered ? 'var(--color-bg-secondary)' : 'none',
                }}
              >
                {renaming ? (
                  <div style={{ padding: '6px 8px', display: 'flex', gap: '4px', alignItems: 'center' }}>
                    <input
                      autoFocus
                      type="text"
                      value={renameValue}
                      onChange={(e) => setRenameValue(e.target.value)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') { onRenameBook(book.ID); }
                        else if (e.key === 'Escape') { setRenamingBookId(null); }
                      }}
                      style={{
                        flex: 1,
                        padding: '3px 6px',
                        borderRadius: '4px',
                        border: '1px solid var(--color-border-default)',
                        background: 'var(--color-bg-primary)',
                        color: 'var(--color-text-primary)',
                        fontSize: '12px',
                        outline: 'none',
                      }}
                    />
                    <button
                      disabled={bookActionLoading}
                      onClick={() => onRenameBook(book.ID)}
                      style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-accent)', padding: '2px', display: 'flex', alignItems: 'center' }}
                    >
                      <CheckIcon style={{ width: '13px', height: '13px' }} />
                    </button>
                    <button
                      onClick={() => setRenamingBookId(null)}
                      style={{ background: 'none', border: 'none', cursor: 'pointer', color: 'var(--color-text-tertiary)', padding: '2px', display: 'flex', alignItems: 'center' }}
                    >
                      <XMarkIcon style={{ width: '13px', height: '13px' }} />
                    </button>
                  </div>
                ) : (
                  <button
                    onClick={() => onSelectBook(book.ID)}
                    style={{
                      width: '100%',
                      textAlign: 'left',
                      padding: '8px 16px',
                      background: 'none',
                      border: 'none',
                      cursor: 'pointer',
                      fontSize: '13px',
                      color: active ? 'var(--color-text-primary)' : 'var(--color-text-secondary)',
                      fontWeight: active ? 600 : 400,
                      display: 'flex',
                      alignItems: 'center',
                      gap: '8px',
                      paddingRight: hovered ? '64px' : '16px',
                    }}
                  >
                    <UserGroupIcon style={{ width: '14px', height: '14px', flexShrink: 0, opacity: 0.6 }} />
                    <span style={{ overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {book.Name || '기본 주소록'}
                    </span>
                  </button>
                )}
                {hovered && !renaming && (
                  <div
                    style={{
                      position: 'absolute',
                      right: '6px',
                      top: '50%',
                      transform: 'translateY(-50%)',
                      display: 'flex',
                      gap: '2px',
                    }}
                  >
                    <button
                      title="이름 변경"
                      onClick={(e) => {
                        e.stopPropagation();
                        setRenameValue(book.Name || '');
                        setRenamingBookId(book.ID);
                      }}
                      style={{
                        background: 'none', border: 'none', cursor: 'pointer',
                        color: 'var(--color-text-tertiary)', padding: '3px', borderRadius: '3px',
                        display: 'flex', alignItems: 'center',
                      }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-primary)'; (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.color = 'var(--color-text-tertiary)'; (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
                    >
                      <PencilIcon style={{ width: '12px', height: '12px' }} />
                    </button>
                    <button
                      title="삭제"
                      onClick={(e) => {
                        e.stopPropagation();
                        onDeleteBook(book.ID, book.Name || '기본 주소록');
                      }}
                      style={{
                        background: 'none', border: 'none', cursor: 'pointer',
                        color: '#ef4444', padding: '3px', borderRadius: '3px',
                        display: 'flex', alignItems: 'center',
                      }}
                      onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                      onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
                    >
                      <TrashIcon style={{ width: '12px', height: '12px' }} />
                    </button>
                  </div>
                )}
              </div>
            );
          })
        )}
      </div>
    </div>
  );
}
