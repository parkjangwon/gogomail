'use client';

import { useState, useEffect, useCallback, useRef } from 'react';
import {
  AddressBook,
  ContactObject,
  listAddressBooks,
  createAddressBook,
  renameAddressBook,
  deleteAddressBook,
  listContacts,
  deleteContact,
  parseVCard,
  upsertContact,
} from '@/lib/api';
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
import { ContactsSidebar } from './ContactsSidebar';
import { stableId } from '@/lib/stableId';

interface ContactsViewProps {
  onCompose?: (email: string) => void;
}

const AVATAR_COLORS = [
  '#6366f1', '#8b5cf6', '#ec4899', '#ef4444',
  '#f97316', '#eab308', '#22c55e', '#14b8a6',
  '#3b82f6', '#06b6d4',
];

function avatarColor(name: string): string {
  let h = 0;
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) & 0xffff;
  return AVATAR_COLORS[h % AVATAR_COLORS.length];
}

function initials(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length === 0 || !parts[0]) return '?';
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

interface ParsedContact {
  fn: string;
  email: string;
  tel: string;
  org: string;
  title: string;
  note: string;
}

function useContactsParsed(contacts: ContactObject[]): ParsedContact[] {
  return contacts.map((c) => parseVCard(c.VCard));
}

export function ContactsView({ onCompose }: ContactsViewProps) {
  const [addressBooks, setAddressBooks] = useState<AddressBook[]>([]);
  const [selectedBookId, setSelectedBookId] = useState<string | null>(null);
  const [contacts, setContacts] = useState<ContactObject[]>([]);
  const [selectedContactIdx, setSelectedContactIdx] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [editMode, setEditMode] = useState(false);
  const [isNewContact, setIsNewContact] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [editFields, setEditFields] = useState<ParsedContact>({ fn: '', email: '', tel: '', org: '', title: '', note: '' });
  const [loading, setLoading] = useState(false);
  const [booksLoading, setBooksLoading] = useState(true);

  // Address book CRUD state
  const [hoveredBookId, setHoveredBookId] = useState<string | null>(null);
  const [renamingBookId, setRenamingBookId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [showNewBookInput, setShowNewBookInput] = useState(false);
  const [newBookName, setNewBookName] = useState('');
  const [bookActionLoading, setBookActionLoading] = useState(false);

  const parsed = useContactsParsed(contacts);

  const filtered = contacts.filter((_, i) => {
    if (!searchQuery.trim()) return true;
    const q = searchQuery.toLowerCase();
    const p = parsed[i];
    return (
      p.fn.toLowerCase().includes(q) ||
      p.email.toLowerCase().includes(q) ||
      p.org.toLowerCase().includes(q)
    );
  });

  // Load address books on mount
  useEffect(() => {
    setBooksLoading(true);
    listAddressBooks().then((books) => {
      setAddressBooks(books);
      if (books.length > 0 && !selectedBookId) {
        setSelectedBookId(books[0].ID);
      }
      setBooksLoading(false);
    });
  }, []);

  // Load contacts when selected book changes
  useEffect(() => {
    if (!selectedBookId) return;
    setLoading(true);
    setSelectedContactIdx(null);
    setEditMode(false);
    listContacts(selectedBookId).then((cts) => {
      setContacts(cts);
      setLoading(false);
    });
  }, [selectedBookId]);

  const selectedContact = selectedContactIdx !== null ? filtered[selectedContactIdx] ?? null : null;
  const selectedContactRaw = selectedContact
    ? contacts.find((c) => c.ID === selectedContact.ID) ?? null
    : null;
  const selectedParsed = selectedContactRaw ? parseVCard(selectedContactRaw.VCard) : null;

  const handleSelectBook = useCallback((id: string) => {
    setSelectedBookId(id);
    setSearchQuery('');
  }, []);

  const handleSelectContact = useCallback((idx: number) => {
    setSelectedContactIdx(idx);
    setEditMode(false);
  }, []);

  const handleDelete = useCallback(async () => {
    if (!selectedContact || !selectedBookId) return;
    const name = selectedParsed?.fn || selectedContact.ObjectName;
    if (!confirm(`"${name}" 연락처를 삭제하시겠습니까?`)) return;
    await deleteContact(selectedBookId, selectedContact.ObjectName);
    setContacts((prev) => prev.filter((c) => c.ID !== selectedContact.ID));
    setSelectedContactIdx(null);
  }, [selectedContact, selectedBookId, selectedParsed]);

  const handleEditStart = useCallback(() => {
    if (!selectedParsed) return;
    setEditFields({ ...selectedParsed });
    setEditMode(true);
  }, [selectedParsed]);

  const handleEditCancel = useCallback(() => {
    setEditMode(false);
    setIsNewContact(false);
    setSaveError('');
  }, []);

  // Address book handlers
  const handleCreateBook = useCallback(async () => {
    const name = newBookName.trim();
    if (!name) return;
    setBookActionLoading(true);
    try {
      const book = await createAddressBook(name);
      setAddressBooks((prev) => [...prev, book]);
      setSelectedBookId(book.ID);
      setNewBookName('');
      setShowNewBookInput(false);
    } catch {
      // silently ignore; user can retry
    } finally {
      setBookActionLoading(false);
    }
  }, [newBookName]);

  const handleRenameBook = useCallback(async (id: string) => {
    const name = renameValue.trim();
    if (!name) { setRenamingBookId(null); return; }
    setBookActionLoading(true);
    try {
      const updated = await renameAddressBook(id, name);
      setAddressBooks((prev) => prev.map((b) => (b.ID === id ? updated : b)));
    } catch {
      // ignore
    } finally {
      setRenamingBookId(null);
      setBookActionLoading(false);
    }
  }, [renameValue]);

  const handleDeleteBook = useCallback(async (id: string, name: string) => {
    if (!confirm(`"${name}" 주소록을 삭제하시겠습니까? 포함된 연락처도 모두 삭제됩니다.`)) return;
    setBookActionLoading(true);
    try {
      await deleteAddressBook(id);
      setAddressBooks((prev) => prev.filter((b) => b.ID !== id));
      if (selectedBookId === id) {
        setSelectedBookId(null);
        setContacts([]);
        setSelectedContactIdx(null);
      }
    } catch {
      // ignore
    } finally {
      setBookActionLoading(false);
    }
  }, [selectedBookId]);

  // j/k/c/Delete keyboard shortcuts
  const containerRef = useRef<HTMLDivElement>(null);
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const tag = (e.target as HTMLElement).tagName;
      const editable = (e.target as HTMLElement).isContentEditable;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || editable) return;

      if (e.key === 'j') {
        setSelectedContactIdx((prev) => {
          const next = (prev ?? -1) + 1;
          return next < filtered.length ? next : prev;
        });
      } else if (e.key === 'k') {
        setSelectedContactIdx((prev) => {
          if (prev === null) return null;
          return prev > 0 ? prev - 1 : 0;
        });
      } else if (e.key === 'c') {
        if (selectedParsed?.email && onCompose) {
          onCompose(selectedParsed.email);
        }
      } else if (e.key === 'Delete' || e.key === 'Backspace') {
        if (selectedContact) handleDelete();
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [filtered.length, selectedParsed, onCompose, selectedContact, handleDelete]);

  const displayName = (idx: number) => {
    const p = parsed[contacts.indexOf(filtered[idx])];
    return p?.fn || filtered[idx]?.ObjectName || '이름 없음';
  };

  const displayEmail = (idx: number) => {
    const p = parsed[contacts.indexOf(filtered[idx])];
    return p?.email || '';
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
              placeholder="연락처 검색..."
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
              onClick={() => {
                setEditFields({ fn: '', email: '', tel: '', org: '', title: '', note: '' });
                setIsNewContact(true);
                setSaveError('');
                setSelectedContactIdx(null);
                setEditMode(true);
              }}
              style={{ width: '100%', padding: '6px', border: '1px dashed var(--color-border-default)', borderRadius: '6px', background: 'none', cursor: 'pointer', fontSize: '12px', color: 'var(--color-text-tertiary)', display: 'flex', alignItems: 'center', justifyContent: 'center', gap: '4px' }}
              onMouseEnter={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'var(--color-bg-secondary)'; }}
              onMouseLeave={(e) => { (e.currentTarget as HTMLButtonElement).style.background = 'none'; }}
            >
              <PlusIcon style={{ width: '13px', height: '13px' }} />
              새 연락처
            </button>
          </div>
        )}

        {/* Contact list */}
        <div style={{ flex: 1, overflowY: 'auto' }}>
          {!selectedBookId ? (
            <div style={{ padding: '32px 16px', textAlign: 'center', color: 'var(--color-text-tertiary)', fontSize: '13px' }}>
              주소록을 선택하세요
            </div>
          ) : loading ? (
            <div style={{ padding: '32px 16px', textAlign: 'center', color: 'var(--color-text-tertiary)', fontSize: '13px' }}>
              로딩 중...
            </div>
          ) : filtered.length === 0 ? (
            <div style={{ padding: '32px 16px', textAlign: 'center', color: 'var(--color-text-tertiary)', fontSize: '13px' }}>
              {searchQuery ? '검색 결과가 없습니다' : '연락처가 없습니다. 새로 추가하세요.'}
            </div>
          ) : (
            filtered.map((_, idx) => {
              const name = displayName(idx);
              const email = displayEmail(idx);
              const active = selectedContactIdx === idx;
              const color = avatarColor(name);
              return (
                <button
                  key={filtered[idx].ID}
                  onClick={() => handleSelectContact(idx)}
                  style={{
                    width: '100%',
                    textAlign: 'left',
                    padding: '10px 14px',
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
                      width: '36px',
                      height: '36px',
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
            <span style={{ fontSize: '14px' }}>주소록을 선택하세요</span>
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
                  {selectedParsed.fn || selectedContact.ObjectName || '이름 없음'}
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
                    onClick={() => onCompose(selectedParsed.email)}
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
                    메일 쓰기
                  </button>
                )}
                {!editMode && (
                  <button
                    onClick={handleEditStart}
                    title="편집"
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
                  title="삭제"
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
                      { label: '이름', field: 'fn' as const },
                      { label: '이메일', field: 'email' as const },
                      { label: '전화', field: 'tel' as const },
                      { label: '회사', field: 'org' as const },
                      { label: '직함', field: 'title' as const },
                      { label: '메모', field: 'note' as const },
                    ] as { label: string; field: keyof ParsedContact }[]
                  ).map(({ label, field }) => (
                    <div key={field} style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                      <label style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-text-tertiary)', textTransform: 'uppercase', letterSpacing: '0.05em' }}>
                        {label}
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
                      취소
                    </button>
                    <button
                      disabled={saving}
                      onClick={async () => {
                        if (!editFields.fn.trim()) { setSaveError('이름은 필수입니다'); return; }
                        if (!selectedBookId) { setSaveError('주소록을 선택하세요'); return; }
                        setSaving(true); setSaveError('');
                        try {
                          const objectName = isNewContact
                            ? `${stableId('contact')}.vcf`
                            : (selectedContact?.ObjectName ?? `${Date.now()}.vcf`);
                          await upsertContact(selectedBookId, objectName, editFields);
                          const updated = await listContacts(selectedBookId);
                          setContacts(updated);
                          setIsNewContact(false);
                          setEditMode(false);
                          if (isNewContact) setSelectedContactIdx(updated.length - 1);
                        } catch (e) {
                          setSaveError(e instanceof Error ? e.message : '저장 실패');
                        } finally { setSaving(false); }
                      }}
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
                      {saving ? '저장 중...' : '저장'}
                    </button>
                  </div>
                </div>
              ) : (
                /* Read-only detail */
                <div style={{ display: 'flex', flexDirection: 'column', gap: '0' }}>
                  {(
                    [
                      { label: '이메일', value: selectedParsed.email, icon: <EnvelopeIcon style={{ width: '14px', height: '14px' }} /> },
                      { label: '전화', value: selectedParsed.tel, icon: <PhoneIcon style={{ width: '14px', height: '14px' }} /> },
                      { label: '회사', value: selectedParsed.org, icon: <BuildingOfficeIcon style={{ width: '14px', height: '14px' }} /> },
                      { label: '직함', value: selectedParsed.title, icon: null },
                      { label: '메모', value: selectedParsed.note, icon: null },
                    ] as { label: string; value: string; icon: React.ReactNode }[]
                  )
                    .filter((row) => row.value)
                    .map((row) => (
                      <div
                        key={row.label}
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
                          {row.label}
                        </div>
                        <div style={{ flex: 1, fontSize: '14px', color: 'var(--color-text-primary)', wordBreak: 'break-all' }}>
                          {row.label === '이메일' && onCompose ? (
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
