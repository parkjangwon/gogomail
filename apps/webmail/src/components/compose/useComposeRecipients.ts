'use client';

import { useState, useRef, useEffect } from 'react';
import { listUserAddresses } from '@/lib/api';
import type { UIComposeIntent, MessageDetail, UserAddressEntry } from '@/lib/api';
import { emailOf } from '@/lib/mail-address';

interface UseComposeRecipientsParams {
  draftMessage?: MessageDetail;
  initialTo?: string;
  intent: UIComposeIntent;
  sourceMessage?: MessageDetail;
  userEmail?: string;
}

interface UseComposeRecipientsReturn {
  to: string;
  setTo: React.Dispatch<React.SetStateAction<string>>;
  cc: string;
  setCc: React.Dispatch<React.SetStateAction<string>>;
  bcc: string;
  setBcc: React.Dispatch<React.SetStateAction<string>>;
  showCc: boolean;
  setShowCc: React.Dispatch<React.SetStateAction<boolean>>;
  showBcc: boolean;
  setShowBcc: React.Dispatch<React.SetStateAction<boolean>>;
  fromAddress: string;
  setFromAddress: React.Dispatch<React.SetStateAction<string>>;
  availableAddresses: UserAddressEntry[];
  setAvailableAddresses: React.Dispatch<React.SetStateAction<UserAddressEntry[]>>;
  recentRecipients: string[];
  toRef: React.MutableRefObject<string>;
  ccRef: React.MutableRefObject<string>;
  bccRef: React.MutableRefObject<string>;
}

export function useComposeRecipients({
  draftMessage,
  initialTo,
  intent,
  sourceMessage,
  userEmail,
}: UseComposeRecipientsParams): UseComposeRecipientsReturn {
  const replyTo = intent === 'reply' || intent === 'reply_all'
    ? sourceMessage?.from_addr ?? ''
    : '';
  const replyCc = intent === 'reply_all' && sourceMessage
    ? (sourceMessage.to_addrs ?? [])
        .map(emailOf)
        .filter((addr) => !userEmail || addr.toLowerCase() !== userEmail.toLowerCase())
        .join(', ')
    : '';

  const draftTo = draftMessage ? (draftMessage.to_addrs ?? []).map(emailOf).join(', ') : '';
  const draftCc = draftMessage ? (draftMessage.cc_addrs ?? []).map(emailOf).join(', ') : '';

  const [to, setTo] = useState(draftMessage ? draftTo : (initialTo ?? replyTo));
  const [cc, setCc] = useState(draftMessage ? draftCc : replyCc);
  const [bcc, setBcc] = useState('');
  const [showCc, setShowCc] = useState(!!(draftMessage ? draftCc : replyCc));
  const [showBcc, setShowBcc] = useState(false);
  const [fromAddress, setFromAddress] = useState(userEmail ?? '');
  const [availableAddresses, setAvailableAddresses] = useState<UserAddressEntry[]>([]);

  const [recentRecipients] = useState<string[]>(() => {
    try {
      const recents: string[] = JSON.parse(localStorage.getItem('webmail_recent_recipients') ?? '[]');
      const contacts: Record<string, string> = JSON.parse(localStorage.getItem('webmail_contacts') ?? '{}');
      const enriched = recents.map((r) => {
        if (r.includes('<')) return r;
        const name = contacts[r.toLowerCase()];
        return name ? `${name} <${r}>` : r;
      });
      const recentEmails = new Set(recents.map((r) => { const m = r.match(/<([^>]+)>/); return (m ? m[1] : r).toLowerCase(); }));
      Object.entries(contacts).forEach(([email, name]) => {
        if (!recentEmails.has(email)) enriched.push(`${name} <${email}>`);
      });
      return enriched.slice(0, 50);
    } catch { return []; }
  });

  const toRef = useRef(draftMessage ? draftTo : (initialTo ?? replyTo));
  const ccRef = useRef(draftMessage ? draftCc : replyCc);
  const bccRef = useRef('');

  useEffect(() => {
    listUserAddresses().then((addrs) => {
      setAvailableAddresses(addrs);
      const primary = addrs.find((a) => a.is_primary);
      if (primary && !fromAddress) setFromAddress(primary.address);
    }).catch(() => {});
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return {
    to,
    setTo,
    cc,
    setCc,
    bcc,
    setBcc,
    showCc,
    setShowCc,
    showBcc,
    setShowBcc,
    fromAddress,
    setFromAddress,
    availableAddresses,
    setAvailableAddresses,
    recentRecipients,
    toRef,
    ccRef,
    bccRef,
  };
}
