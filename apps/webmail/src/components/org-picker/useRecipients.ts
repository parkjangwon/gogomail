import { useState } from 'react';
import type { PickerItem } from '@/lib/mail-address';

export interface UseRecipientsResult {
  toList: Map<string, PickerItem>;
  ccList: Map<string, PickerItem>;
  bccList: Map<string, PickerItem>;
  activeField: 'to' | 'cc' | 'bcc';
  setActiveField: (field: 'to' | 'cc' | 'bcc') => void;
  addToActive: (item: PickerItem) => void;
  removeFromList: (field: 'to' | 'cc' | 'bcc', id: string) => void;
  clearList: (field: 'to' | 'cc' | 'bcc') => void;
}

export function useRecipients(
  initialTo: PickerItem[],
  initialCc: PickerItem[],
  initialBcc: PickerItem[],
): UseRecipientsResult {
  const [toList, setToList] = useState<Map<string, PickerItem>>(
    () => new Map(initialTo.map((i) => [i.id, i]))
  );
  const [ccList, setCcList] = useState<Map<string, PickerItem>>(
    () => new Map(initialCc.map((i) => [i.id, i]))
  );
  const [bccList, setBccList] = useState<Map<string, PickerItem>>(
    () => new Map(initialBcc.map((i) => [i.id, i]))
  );
  const [activeField, setActiveField] = useState<'to' | 'cc' | 'bcc'>('to');

  function getActiveList(): Map<string, PickerItem> {
    if (activeField === 'to') return toList;
    if (activeField === 'cc') return ccList;
    return bccList;
  }

  function setActiveList(next: Map<string, PickerItem>) {
    if (activeField === 'to') setToList(next);
    else if (activeField === 'cc') setCcList(next);
    else setBccList(next);
  }

  function addToActive(item: PickerItem) {
    const cur = getActiveList();
    if (cur.has(item.id)) return;
    const next = new Map(cur);
    next.set(item.id, item);
    setActiveList(next);
  }

  function removeFromList(field: 'to' | 'cc' | 'bcc', id: string) {
    const setter = field === 'to' ? setToList : field === 'cc' ? setCcList : setBccList;
    const cur = field === 'to' ? toList : field === 'cc' ? ccList : bccList;
    const next = new Map(cur);
    next.delete(id);
    setter(next);
  }

  function clearList(field: 'to' | 'cc' | 'bcc') {
    if (field === 'to') setToList(new Map());
    else if (field === 'cc') setCcList(new Map());
    else setBccList(new Map());
  }

  return {
    toList,
    ccList,
    bccList,
    activeField,
    setActiveField,
    addToActive,
    removeFromList,
    clearList,
  };
}
