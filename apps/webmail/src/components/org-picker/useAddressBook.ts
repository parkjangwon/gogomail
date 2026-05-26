import { useState, useEffect } from 'react';
import { listAddressBooks, listContacts, parseVCard, AddressBook } from '@/lib/api';
import type { PickerItem } from '@/lib/mail-address';

export interface UseAddressBookResult {
  addressBooks: AddressBook[];
  selectedBook: AddressBook | null;
  setSelectedBook: (book: AddressBook | null) => void;
  bookContacts: PickerItem[];
  booksLoading: boolean;
  bookLoading: boolean;
  contactsSearch: string;
  setContactsSearch: (s: string) => void;
  filteredContacts: PickerItem[];
  addressBookToken: (book: AddressBook) => PickerItem;
}

export function useAddressBook(tab: 'org' | 'contacts'): UseAddressBookResult {
  const [addressBooks, setAddressBooks] = useState<AddressBook[]>([]);
  const [selectedBook, setSelectedBook] = useState<AddressBook | null>(null);
  const [bookContacts, setBookContacts] = useState<PickerItem[]>([]);
  const [booksLoading, setBooksLoading] = useState(false);
  const [bookLoading, setBookLoading] = useState(false);
  const [contactsSearch, setContactsSearch] = useState('');

  // Load address books when switching to contacts tab
  useEffect(() => {
    if (tab !== 'contacts') return;
    if (addressBooks.length > 0) return;
    setBooksLoading(true);
    listAddressBooks()
      .then((books) => {
        setAddressBooks(books);
        if (books.length > 0 && !selectedBook) setSelectedBook(books[0]);
        setBooksLoading(false);
      })
      .catch(() => setBooksLoading(false));
  }, [tab, addressBooks.length, selectedBook]);

  // Load contacts when selectedBook changes
  useEffect(() => {
    if (!selectedBook) return;
    setBookLoading(true);
    listContacts(selectedBook.ID)
      .then((contacts) => {
        const items: PickerItem[] = contacts
          .map((c: { ID: string; VCard: string }) => {
            const parsed = parseVCard(c.VCard);
            return { id: c.ID, display_name: parsed.fn || parsed.email, email: parsed.email };
          })
          .filter((i) => !!i.email);
        setBookContacts(items);
        setBookLoading(false);
      })
      .catch(() => setBookLoading(false));
  }, [selectedBook]);

  const cq = contactsSearch.trim().toLowerCase();
  const filteredContacts: PickerItem[] = cq
    ? bookContacts.filter(
        (c) =>
          c.display_name.toLowerCase().includes(cq) ||
          c.email.toLowerCase().includes(cq)
      )
    : bookContacts;

  const addressBookToken = (book: AddressBook): PickerItem => ({
    id: `addressbook:${book.ID}`,
    display_name: book.Name,
    email: `addressbook:${book.ID}`,
    kind: 'addressbook',
    count: bookContacts.length,
  });

  return {
    addressBooks,
    selectedBook,
    setSelectedBook,
    bookContacts,
    booksLoading,
    bookLoading,
    contactsSearch,
    setContactsSearch,
    filteredContacts,
    addressBookToken,
  };
}
