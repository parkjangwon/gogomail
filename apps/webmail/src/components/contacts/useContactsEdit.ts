import { useState, useCallback, type Dispatch, type SetStateAction } from 'react';
import {
  ContactObject,
  listContacts,
  deleteContact,
  upsertContact,
} from '@/lib/api';
import { stableId } from '@/lib/stableId';
import { type ParsedContact } from './contactsViewHelpers';

interface UseContactsEditParams {
  selectedContact: ContactObject | null;
  selectedBookId: string | null;
  contacts: ContactObject[];
  setContacts: Dispatch<SetStateAction<ContactObject[]>>;
  selectedParsed: ParsedContact | null;
  setSelectedContactIdx: Dispatch<SetStateAction<number | null>>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
}

export function useContactsEdit({
  selectedContact,
  selectedBookId,
  contacts,
  setContacts,
  selectedParsed,
  setSelectedContactIdx,
  t,
}: UseContactsEditParams) {
  const [editMode, setEditMode] = useState(false);
  const [isNewContact, setIsNewContact] = useState(false);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState('');
  const [editFields, setEditFields] = useState<ParsedContact>({
    fn: '', email: '', tel: '', org: '', title: '', note: '',
  });

  const handleDelete = useCallback(async () => {
    if (!selectedContact || !selectedBookId) return;
    const name = selectedParsed?.fn || selectedContact.ObjectName;
    if (!confirm(t('deleteConfirm', { name }))) return;
    try {
      await deleteContact(selectedBookId, selectedContact.ObjectName);
      setContacts((prev) => prev.filter((c) => c.ID !== selectedContact.ID));
      setSelectedContactIdx(null);
    } catch { /* ignore — contact remains visible if delete fails */ }
  }, [selectedContact, selectedBookId, selectedParsed, t, setContacts, setSelectedContactIdx]);

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

  const handleSave = useCallback(async () => {
    if (!editFields.fn.trim()) { setSaveError(t('saveErrorName')); return; }
    if (!selectedBookId) { setSaveError(t('saveErrorBook')); return; }
    setSaving(true);
    setSaveError('');
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
      setSaveError(e instanceof Error ? e.message : t('saveErrorGeneric'));
    } finally {
      setSaving(false);
    }
  }, [editFields, selectedBookId, isNewContact, selectedContact, t, setContacts, setSelectedContactIdx]);

  const handleNewContactStart = useCallback(() => {
    setEditFields({ fn: '', email: '', tel: '', org: '', title: '', note: '' });
    setIsNewContact(true);
    setSaveError('');
    setSelectedContactIdx(null);
    setEditMode(true);
  }, [setSelectedContactIdx]);

  return {
    editMode,
    setEditMode,
    isNewContact,
    setIsNewContact,
    saving,
    saveError,
    setSaveError,
    editFields,
    setEditFields,
    handleDelete,
    handleEditStart,
    handleEditCancel,
    handleSave,
    handleNewContactStart,
  };
}
