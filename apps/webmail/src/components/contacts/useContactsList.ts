'use client';
import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { listContacts, parseVCard, type ContactObject } from '@/lib/api';
import { loadContactViewSettings, useContactsParsed } from './contactsViewHelpers';

interface UseContactsListParams {
  selectedBookId: string;
  onCompose?: (email: string) => void;
  handleDelete?: () => void;
  setEditMode?: (v: boolean) => void;
}

export function useContactsList({
  selectedBookId,
  onCompose,
  handleDelete,
  setEditMode,
}: UseContactsListParams) {
  const [contacts, setContacts] = useState<ContactObject[]>([]);
  const [selectedContactIdx, setSelectedContactIdx] = useState<number | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [loading, setLoading] = useState(false);
  const [viewSettings, setViewSettings] = useState(loadContactViewSettings);

  // Stable refs for callbacks that may change after initial render (circular dependency)
  const handleDeleteRef = useRef(handleDelete);
  const setEditModeRef = useRef(setEditMode);
  useEffect(() => { handleDeleteRef.current = handleDelete; }, [handleDelete]);
  useEffect(() => { setEditModeRef.current = setEditMode; }, [setEditMode]);

  const parsed = useContactsParsed(contacts);

  // Derived: filtered
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

  // Derived: sortedFiltered
  const sortedFiltered = useMemo(() => {
    const rank = (contact: ContactObject) => {
      const p = parsed[contacts.indexOf(contact)];
      if (viewSettings.sort === 'email') return p?.email || p?.fn || contact.ObjectName;
      if (viewSettings.sort === 'company') return p?.org || p?.fn || p?.email || contact.ObjectName;
      return p?.fn || p?.email || contact.ObjectName;
    };
    return [...filtered].sort((a, b) =>
      rank(a).localeCompare(rank(b), 'ko', { sensitivity: 'base' })
    );
  }, [contacts, filtered, parsed, viewSettings.sort]);

  // Derived: selectedContact, selectedContactRaw, selectedParsed
  const selectedContact =
    selectedContactIdx !== null ? sortedFiltered[selectedContactIdx] ?? null : null;
  const selectedContactRaw = selectedContact
    ? contacts.find((c) => c.ID === selectedContact.ID) ?? null
    : null;
  const selectedParsed = selectedContactRaw ? parseVCard(selectedContactRaw.VCard) : null;

  // Stable ref for selectedParsed and selectedContact (for keyboard handler closure)
  const selectedParsedRef = useRef(selectedParsed);
  const selectedContactRef = useRef(selectedContact);
  const onComposeRef = useRef(onCompose);
  useEffect(() => { selectedParsedRef.current = selectedParsed; });
  useEffect(() => { selectedContactRef.current = selectedContact; });
  useEffect(() => { onComposeRef.current = onCompose; }, [onCompose]);

  // Load contacts when selectedBookId changes
  useEffect(() => {
    if (!selectedBookId) return;
    setLoading(true);
    setSelectedContactIdx(null);
    setEditModeRef.current?.(false);
    listContacts(selectedBookId).then((cts) => {
      setContacts(cts);
      setLoading(false);
    }).catch(() => setLoading(false));
  }, [selectedBookId]);

  // Sync viewSettings from localStorage
  useEffect(() => {
    const refresh = (event?: StorageEvent) => {
      if (event && event.key !== 'webmail_settings') return;
      setViewSettings(loadContactViewSettings());
    };
    window.addEventListener('storage', refresh);
    return () => window.removeEventListener('storage', refresh);
  }, []);

  // Keyboard shortcuts: j/k/c/Delete
  // Uses refs so this effect only re-registers when list length or compose target changes,
  // avoiding the circular dependency with handleDelete/setEditMode.
  useEffect(() => {
    function onKey(e: KeyboardEvent) {
      const tag = (e.target as HTMLElement).tagName;
      const editable = (e.target as HTMLElement).isContentEditable;
      if (tag === 'INPUT' || tag === 'TEXTAREA' || editable) return;

      if (e.key === 'j') {
        setSelectedContactIdx((prev) => {
          const next = (prev ?? -1) + 1;
          return next < sortedFiltered.length ? next : prev;
        });
      } else if (e.key === 'k') {
        setSelectedContactIdx((prev) => {
          if (prev === null) return null;
          return prev > 0 ? prev - 1 : 0;
        });
      } else if (e.key === 'c') {
        const sp = selectedParsedRef.current;
        const oc = onComposeRef.current;
        if (sp?.email && oc) {
          e.preventDefault();
          e.stopPropagation();
          e.stopImmediatePropagation();
          oc(sp.email);
        }
      } else if (e.key === 'Delete' || e.key === 'Backspace') {
        if (selectedContactRef.current) handleDeleteRef.current?.();
      }
    }
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [sortedFiltered.length]);

  // handleSelectContact callback
  const handleSelectContact = useCallback(
    (idx: number) => {
      setSelectedContactIdx(idx);
      setEditModeRef.current?.(false);
    },
    []
  );

  return {
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
  };
}
