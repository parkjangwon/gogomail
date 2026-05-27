'use client';

import { type Dispatch, type SetStateAction, useCallback, useRef } from 'react';
import { useTranslations } from 'next-intl';
import {
  PlusIcon,
  MagnifyingGlassIcon,
} from '@heroicons/react/24/outline';
import { type ContactObject } from '@/lib/api';
import { ContactsSidebar } from './ContactsSidebar';
import {
  avatarColor,
  initials,
  type ParsedContact,
} from './contacts/contactsViewHelpers';
import { ContactDetailPanel } from './contacts/ContactDetailPanel';
import { useContactsBooks } from './contacts/useContactsBooks';
import { useContactsEdit } from './contacts/useContactsEdit';
import { useContactsList } from './contacts/useContactsList';

interface ContactsViewProps {
  onCompose?: (email: string) => void;
}

export function ContactsView({ onCompose }: ContactsViewProps) {
  const t = useTranslations('contacts');

  // Refs to bridge circular dependencies between hooks.
  // useContactsBooks needs setContacts/setSelectedContactIdx from useContactsList (called later).
  // useContactsList keyboard handler needs handleDelete/setEditMode from useContactsEdit (also called later).
  // We initialise refs here and sync them synchronously during each render after each hook call.
  const setContactsRef = useRef<Dispatch<SetStateAction<ContactObject[]>> | undefined>(undefined);
  const setSelectedContactIdxRef = useRef<Dispatch<SetStateAction<number | null>> | undefined>(undefined);
  const handleDeleteRef = useRef<(() => void) | undefined>(undefined);
  const setEditModeRef = useRef<((v: boolean) => void) | undefined>(undefined);

  const {
    addressBooks,
    selectedBookId,
    setSelectedBookId,
    booksLoading,
    hoveredBookId,
    setHoveredBookId,
    renamingBookId,
    setRenamingBookId,
    renameValue,
    setRenameValue,
    showNewBookInput,
    setShowNewBookInput,
    newBookName,
    setNewBookName,
    bookActionLoading,
    handleCreateBook,
    handleRenameBook,
    handleDeleteBook,
  } = useContactsBooks({
    t,
    // Proxy through refs so useContactsBooks callbacks always reach the real state setters
    setContacts: (v: Parameters<Dispatch<SetStateAction<ContactObject[]>>>[0]) => setContactsRef.current?.(v),
    setSelectedContactIdx: (v: Parameters<Dispatch<SetStateAction<number | null>>>[0]) => setSelectedContactIdxRef.current?.(v),
  });

  const {
    contacts,
    setContacts,
    selectedContactIdx,
    setSelectedContactIdx,
    searchQuery,
    setSearchQuery,
    loading,
    viewSettings,
    parsed,
    filtered,
    sortedFiltered,
    selectedContact,
    selectedContactRaw,
    selectedParsed,
    handleSelectContact,
  } = useContactsList({
    selectedBookId: selectedBookId ?? '',
    onCompose,
    handleDelete: handleDeleteRef.current,
    setEditMode: setEditModeRef.current,
  });

  // Sync list setters so the books-hook callbacks propagate to list state
  setContactsRef.current = setContacts;
  setSelectedContactIdxRef.current = setSelectedContactIdx;

  const {
    editMode,
    setEditMode,
    saving,
    saveError,
    editFields,
    setEditFields,
    handleDelete,
    handleEditStart,
    handleEditCancel,
    handleSave,
    handleNewContactStart,
  } = useContactsEdit({
    selectedContact,
    selectedBookId: selectedBookId ?? '',
    contacts,
    setContacts,
    selectedParsed,
    setSelectedContactIdx,
    t,
  });

  // Sync edit callbacks so the keyboard handler in useContactsList picks them up
  handleDeleteRef.current = handleDelete;
  setEditModeRef.current = setEditMode;

  const handleSelectBook = useCallback(
    (id: string) => {
      setSelectedBookId(id);
      setSearchQuery('');
    },
    [setSelectedBookId, setSearchQuery]
  );

  const containerRef = useRef<HTMLDivElement>(null);

  const displayName = (idx: number) => {
    const p = parsed[contacts.indexOf(sortedFiltered[idx])];
    return p?.fn || sortedFiltered[idx]?.ObjectName || t('noName');
  };

  const displayEmail = (idx: number) => {
    const p = parsed[contacts.indexOf(sortedFiltered[idx])];
    return p?.email || '';
  };

  const displayCompany = (idx: number) => {
    const p = parsed[contacts.indexOf(sortedFiltered[idx])];
    return p?.org || '';
  };

  return (
    <div
      ref={containerRef}
      style={{
        flex: 1,
        display: 'flex',
        height: '100%',
        overflow: 'hidden',
        background: 'var(--color-bg-primary)',
      }}
    >
      <ContactsSidebar
        addressBooks={addressBooks}
        selectedBookId={selectedBookId}
        booksLoading={booksLoading}
        showNewBookInput={showNewBookInput}
        setShowNewBookInput={setShowNewBookInput}
        newBookName={newBookName}
        setNewBookName={setNewBookName}
        bookActionLoading={bookActionLoading}
        hoveredBookId={hoveredBookId}
        setHoveredBookId={setHoveredBookId}
        renamingBookId={renamingBookId}
        setRenamingBookId={setRenamingBookId}
        renameValue={renameValue}
        setRenameValue={setRenameValue}
        onSelectBook={handleSelectBook}
        onCreateBook={handleCreateBook}
        onRenameBook={handleRenameBook}
        onDeleteBook={handleDeleteBook}
      />

      {/* Middle pane: Contact list */}
      <div
        style={{
          width: '300px',
          flexShrink: 0,
          borderRight: '1px solid var(--color-border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
        }}
      >
        {/* Search */}
        <div
          style={{
            padding: '10px 12px',
            borderBottom: '1px solid var(--color-border-subtle)',
            flexShrink: 0,
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              background: 'var(--color-bg-secondary)',
              borderRadius: '6px',
              padding: '6px 10px',
            }}
          >
            <MagnifyingGlassIcon style={{ width: '14px', height: '14px', color: 'var(--color-text-tertiary)', flexShrink: 0 }} />
            <input
              type="search"
              placeholder={t('searchPlaceholder')}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              style={{
                flex: 1,
                background: 'none',
                border: 'none',
                outline: 'none',
                fontSize: '13px',
                color: 'var(--color-text-primary)',
              }}
            />
          </div>
        </div>

        {/* New contact button */}
        {selectedBookId && !editMode && (
          <div style={{ padding: '6px 12px', borderBottom: '1px solid var(--color-border-subtle)', flexShrink: 0 }}>
            <button
              onClick={handleNewContactStart}
              style={{ width: '100%', padding: '6px', border: '1px dashed var(--color-border-default)', borderRadius: '6px', background: 'none', cursor: 'pointer', fontSize: '12px', color: 'var(--color-text-tertiary)', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '4px' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
            >
              <PlusIcon style={{ width: '13px', height: '13px' }} />
              {t('newContact')}
            </button>
          </div>
        )}

        {/* Contact list */}
        <div style={{ flex: 1, overflowY: 'auto' }}>
          {!selectedBookId ? (
            <div style={{ padding: '32px 16px', textAlign: 'center', color: 'var(--color-text-tertiary)', fontSize: '13px' }}>
              {t('selectAddressBook')}
            </div>
          ) : loading ? (
            <div style={{ padding: '32px 16px', textAlign: 'center', color: 'var(--color-text-tertiary)', fontSize: '13px' }}>
              {t('loading')}
            </div>
          ) : sortedFiltered.length === 0 ? (
            <div style={{ padding: '32px 16px', textAlign: 'center', color: 'var(--color-text-tertiary)', fontSize: '13px' }}>
              {searchQuery ? t('noContactsSearch') : t('noContacts')}
            </div>
          ) : (
            sortedFiltered.map((_, idx) => {
              const name = displayName(idx);
              const email = displayEmail(idx);
              const company = displayCompany(idx);
              const active = selectedContactIdx === idx;
              const color = avatarColor(name);
              return (
                <button
                  key={sortedFiltered[idx].ID}
                  onClick={() => handleSelectContact(idx)}
                  style={{
                    width: '100%',
                    textAlign: 'left',
                    padding: viewSettings.density === 'compact' ? '7px 14px' : '10px 14px',
                    background: active ? 'var(--color-bg-tertiary)' : 'none',
                    border: 'none',
                    borderBottom: '1px solid var(--color-border-subtle)',
                    cursor: 'pointer',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '10px',
                  }}
                  onMouseEnter={(e) => { if (!active) (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  onMouseLeave={(e) => { if (!active) (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
                >
                  {/* Avatar */}
                  <div
                    style={{
                      width: viewSettings.density === 'compact' ? '30px' : '36px',
                      height: viewSettings.density === 'compact' ? '30px' : '36px',
                      borderRadius: '50%',
                      background: color,
                      color: '#fff',
                      display: 'flex',
                      alignItems: 'center',
                      justifyContent: 'center',
                      fontSize: '13px',
                      fontWeight: 700,
                      flexShrink: 0,
                    }}
                  >
                    {initials(name)}
                  </div>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <div style={{ fontSize: '13px', fontWeight: 500, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                      {name}
                    </div>
                    {email && (
                      <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {email}
                      </div>
                    )}
                    {viewSettings.showCompany && company && (
                      <div style={{ fontSize: '11px', color: 'var(--color-text-tertiary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {company}
                      </div>
                    )}
                  </div>
                </button>
              );
            })
          )}
        </div>
      </div>

      {/* Right pane: Contact detail */}
      <ContactDetailPanel
        selectedContact={selectedContact}
        selectedParsed={selectedParsed}
        editMode={editMode}
        editFields={editFields}
        setEditFields={setEditFields}
        saving={saving}
        saveError={saveError}
        onCompose={onCompose}
        onEditStart={handleEditStart}
        onEditCancel={handleEditCancel}
        onSave={handleSave}
        onDelete={handleDelete}
        t={t as (key: string, values?: Record<string, unknown>) => string}
      />
    </div>
  );
}
