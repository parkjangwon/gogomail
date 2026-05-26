import { useState, useEffect, useCallback, type Dispatch, type SetStateAction } from 'react';
import {
  AddressBook,
  ContactObject,
  listAddressBooks,
  createAddressBook,
  renameAddressBook,
  deleteAddressBook,
} from '@/lib/api';

interface UseContactsBooksParams {
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
  setContacts: Dispatch<SetStateAction<ContactObject[]>>;
  setSelectedContactIdx: Dispatch<SetStateAction<number | null>>;
}

export function useContactsBooks({
  t,
  setContacts,
  setSelectedContactIdx,
}: UseContactsBooksParams) {
  const [addressBooks, setAddressBooks] = useState<AddressBook[]>([]);
  const [selectedBookId, setSelectedBookId] = useState<string | null>(null);
  const [booksLoading, setBooksLoading] = useState(true);
  const [hoveredBookId, setHoveredBookId] = useState<string | null>(null);
  const [renamingBookId, setRenamingBookId] = useState<string | null>(null);
  const [renameValue, setRenameValue] = useState('');
  const [showNewBookInput, setShowNewBookInput] = useState(false);
  const [newBookName, setNewBookName] = useState('');
  const [bookActionLoading, setBookActionLoading] = useState(false);

  // Load address books on mount
  useEffect(() => {
    setBooksLoading(true);
    listAddressBooks().then((books) => {
      setAddressBooks(books);
      if (books.length > 0) {
        setSelectedBookId((prev) => (prev ? prev : books[0].ID));
      }
      setBooksLoading(false);
    });
  }, []);

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
    if (!confirm(t('deleteBookConfirm', { name }))) return;
    setBookActionLoading(true);
    try {
      await deleteAddressBook(id);
      setAddressBooks((prev) => prev.filter((b) => b.ID !== id));
      setSelectedBookId((prev) => {
        if (prev === id) {
          setContacts([]);
          setSelectedContactIdx(null);
          return null;
        }
        return prev;
      });
    } catch {
      // ignore
    } finally {
      setBookActionLoading(false);
    }
  }, [t, setContacts, setSelectedContactIdx]);

  return {
    addressBooks,
    setAddressBooks,
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
  };
}
