'use client';

import { type Dispatch, type SetStateAction, useCallback, useRef } from 'react';
import { useTranslations } from 'next-intl';
import {
  UserGroupIcon,
  PlusIcon,
  MagnifyingGlassIcon,
  PencilIcon,
  TrashIcon,
  EnvelopeIcon,
  PhoneIcon,
  BuildingOfficeIcon,
  XMarkIcon,
  CheckIcon,
} from '@heroicons/react/24/outline';
import { type ContactObject } from '@/lib/api';
import { ContactsSidebar } from './ContactsSidebar';
import {
  avatarColor,
  initials,
  type ParsedContact,
} from './contacts/contactsViewHelpers';
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
      <div style={{ flex: 1, display: 'flex', flexDirection: 'column', overflow: 'hidden', minWidth: 0 }}>
        {!selectedContact || !selectedParsed ? (
          <div
            style={{
              flex: 1,
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              justifyContent: 'center',
              gap: '12px',
              color: 'var(--color-text-tertiary)',
            }}
          >
            <UserGroupIcon style={{ width: '48px', height: '48px', opacity: 0.3 }} />
            <span style={{ fontSize: '14px' }}>{t('selectAddressBook')}</span>
          </div>
        ) : (
          <>
            {/* Detail header */}
            <div
              style={{
                padding: '20px 24px 16px',
                borderBottom: '1px solid var(--color-border-subtle)',
                display: 'flex',
                alignItems: 'center',
                gap: '16px',
                flexShrink: 0,
              }}
            >
              {/* Avatar */}
              <div
                style={{
                  width: '56px',
                  height: '56px',
                  borderRadius: '50%',
                  background: avatarColor(selectedParsed.fn || selectedContact.ObjectName),
                  color: '#fff',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  fontSize: '20px',
                  fontWeight: 700,
                  flexShrink: 0,
                }}
              >
                {initials(selectedParsed.fn || selectedContact.ObjectName || '?')}
              </div>
              <div style={{ flex: 1, minWidth: 0 }}>
                <div style={{ fontSize: '18px', fontWeight: 600, color: 'var(--color-text-primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                  {selectedParsed.fn || selectedContact.ObjectName || t('noName')}
                </div>
                {selectedParsed.org && (
                  <div style={{ fontSize: '13px', color: 'var(--color-text-tertiary)', marginTop: '2px' }}>
                    {selectedParsed.org}
                  </div>
                )}
              </div>
              {/* Action buttons */}
              <div style={{ display: 'flex', gap: '8px', flexShrink: 0 }}>
                {selectedParsed.email && onCompose && (
                  <button
                    onClick={() => onCompose(selectedParsed!.email)}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: '6px',
                      padding: '7px 14px',
                      background: 'var(--color-accent)',
                      color: '#fff',
                      border: 'none',
                      borderRadius: '6px',
                      fontSize: '13px',
                      fontWeight: 500,
                      cursor: 'pointer',
                    }}
                    onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '0.85'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.opacity = '1'; }}
                  >
                    <EnvelopeIcon style={{ width: '14px', height: '14px' }} />
                    {t('composeButton')}
                  </button>
                )}
                {!editMode && (
                  <button
                    onClick={handleEditStart}
                    title={t('editTitle')}
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      padding: '7px',
                      background: 'var(--color-bg-secondary)',
                      border: '1px solid var(--color-border-subtle)',
                      borderRadius: '6px',
                      cursor: 'pointer',
                      color: 'var(--color-text-secondary)',
                    }}
                    onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                    onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                  >
                    <PencilIcon style={{ width: '14px', height: '14px' }} />
                  </button>
                )}
                <button
                  onClick={handleDelete}
                  title={t('deleteTitle')}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    padding: '7px',
                    background: 'var(--color-bg-secondary)',
                    border: '1px solid var(--color-border-subtle)',
                    borderRadius: '6px',
                    cursor: 'pointer',
                    color: '#ef4444',
                  }}
                  onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-tertiary)'; }}
                  onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
                >
                  <TrashIcon style={{ width: '14px', height: '14px' }} />
                </button>
              </div>
            </div>

            {/* Detail body */}
            <div style={{ flex: 1, overflowY: 'auto', padding: '20px 24px' }}>
              {editMode ? (
                /* Edit form */
                <div style={{ display: 'flex', flexDirection: 'column', gap: '14px', maxWidth: '480px' }}>
                  {(
                    [
                      { fieldKey: 'name' as const, field: 'fn' as const },
                      { fieldKey: 'email' as const, field: 'email' as const },
                      { fieldKey: 'tel' as const, field: 'tel' as const },
                      { fieldKey: 'org' as const, field: 'org' as const },
                      { fieldKey: 'title' as const, field: 'title' as const },
                      { fieldKey: 'note' as const, field: 'note' as const },
                    ] as { fieldKey: string; field: keyof ParsedContact }[]
                  ).map(({ fieldKey, field }) => (
                    <div key={field} style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                      <label style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                        {t(`fields.${fieldKey}`)}
                      </label>
                      {field === 'note' ? (
                        <textarea
                          value={editFields[field]}
                          onChange={(e) => setEditFields((prev) => ({ ...prev, [field]: e.target.value }))}
                          rows={3}
                          style={{
                            padding: '8px 10px',
                            borderRadius: '6px',
                            border: '1px solid var(--color-border-default)',
                            background: 'var(--color-bg-secondary)',
                            color: 'var(--color-text-primary)',
                            fontSize: '13px',
                            resize: 'vertical',
                            outline: 'none',
                            fontFamily: 'inherit',
                          }}
                        />
                      ) : (
                        <input
                          type="text"
                          value={editFields[field]}
                          onChange={(e) => setEditFields((prev) => ({ ...prev, [field]: e.target.value }))}
                          style={{
                            padding: '8px 10px',
                            borderRadius: '6px',
                            border: '1px solid var(--color-border-default)',
                            background: 'var(--color-bg-secondary)',
                            color: 'var(--color-text-primary)',
                            fontSize: '13px',
                            outline: 'none',
                          }}
                        />
                      )}
                    </div>
                  ))}
                  {saveError && <div style={{ fontSize: '12px', color: '#e53e3e', marginTop: '4px' }}>{saveError}</div>}
                  <div style={{ display: 'flex', gap: '8px', marginTop: '4px' }}>
                    <button
                      onClick={handleEditCancel}
                      style={{
                        display: 'flex', alignItems: 'center', gap: '4px',
                        padding: '7px 14px',
                        background: 'var(--color-bg-secondary)',
                        border: '1px solid var(--color-border-default)',
                        borderRadius: '6px',
                        fontSize: '13px',
                        cursor: 'pointer',
                        color: 'var(--color-text-secondary)',
                      }}
                    >
                      <XMarkIcon style={{ width: '14px', height: '14px' }} />
                      {t('cancel')}
                    </button>
                    <button
                      disabled={saving}
                      onClick={handleSave}
                      style={{
                        display: 'flex', alignItems: 'center', gap: '4px',
                        padding: '7px 14px',
                        background: 'var(--color-accent)',
                        border: 'none',
                        borderRadius: '6px',
                        fontSize: '13px',
                        fontWeight: 500,
                        cursor: saving ? 'wait' : 'pointer',
                        color: '#fff',
                        opacity: saving ? 0.7 : 1,
                      }}
                    >
                      <CheckIcon style={{ width: '14px', height: '14px' }} />
                      {saving ? t('saving') : t('save')}
                    </button>
                  </div>
                </div>
              ) : (
                /* Read-only detail */
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0' }}>
                  {(
                    [
                      { fieldKey: 'email' as const, value: selectedParsed.email, icon: <EnvelopeIcon style={{ width: '14px', height: '14px' }} /> },
                      { fieldKey: 'tel' as const, value: selectedParsed.tel, icon: <PhoneIcon style={{ width: '14px', height: '14px' }} /> },
                      { fieldKey: 'org' as const, value: selectedParsed.org, icon: <BuildingOfficeIcon style={{ width: '14px', height: '14px' }} /> },
                      { fieldKey: 'title' as const, value: selectedParsed.title, icon: null },
                      { fieldKey: 'note' as const, value: selectedParsed.note, icon: null },
                    ] as { fieldKey: string; value: string; icon: React.ReactNode }[]
                  )
                    .filter((row) => row.value)
                    .map((row) => (
                      <div
                        key={row.fieldKey}
                        style={{
                          display: 'flex',
                          alignItems: 'flex-start',
                          gap: '12px',
                          padding: '12px 0',
                          borderBottom: '1px solid var(--color-border-subtle)',
                        }}
                      >
                        <div style={{ width: '80px', flexShrink: 0, fontSize: '12px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.04em', paddingTop: '1px', display: 'flex', alignItems: 'center', gap: '5px' }}>
                          {row.icon}
                          {t(`fields.${row.fieldKey}`)}
                        </div>
                        <div style={{ flex: 1, fontSize: '14px', color: 'var(--color-text-primary)', wordBreak: 'break-all' }}>
                          {row.fieldKey === 'email' && onCompose ? (
                            <button
                              onClick={() => onCompose(row.value)}
                              style={{ background: 'none', border: 'none', padding: 0, color: 'var(--color-accent)', cursor: 'pointer', fontSize: '14px', textDecoration: 'underline' }}
                            >
                              {row.value}
                            </button>
                          ) : (
                            row.value
                          )}
                        </div>
                      </div>
                    ))}
                </div>
              )}
            </div>
          </>
        )}
      </div>
    </div>
  );
}
