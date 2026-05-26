'use client';

import { useState, useRef, useMemo, useCallback } from 'react';
import { MessageSummary } from '@/lib/api';

export interface ContactCardState {
  name: string;
  addr: string;
  count: number;
  x: number;
  y: number;
}

export interface UseContactHoverCardResult {
  contactCard: ContactCardState | null;
  handleAvatarEnter: (name: string, addr: string, rect: DOMRect) => void;
  handleAvatarLeave: () => void;
  closeContactCard: () => void;
}

export function useContactHoverCard(messages: MessageSummary[]): UseContactHoverCardResult {
  const [contactCard, setContactCard] = useState<ContactCardState | null>(null);
  const contactCardTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const senderCounts = useMemo(() => {
    const m: Record<string, number> = {};
    for (const msg of messages) m[msg.from_addr] = (m[msg.from_addr] ?? 0) + 1;
    return m;
  }, [messages]);

  const handleAvatarEnter = useCallback((name: string, addr: string, rect: DOMRect) => {
    if (contactCardTimerRef.current) clearTimeout(contactCardTimerRef.current);
    const count = senderCounts[addr] ?? 1;
    contactCardTimerRef.current = setTimeout(() => {
      setContactCard({ name, addr, count, x: rect.right + 6, y: rect.top - 8 });
    }, 350);
  }, [senderCounts]);

  const handleAvatarLeave = useCallback(() => {
    if (contactCardTimerRef.current) clearTimeout(contactCardTimerRef.current);
    contactCardTimerRef.current = setTimeout(() => setContactCard(null), 120);
  }, []);

  const closeContactCard = useCallback(() => {
    if (contactCardTimerRef.current) clearTimeout(contactCardTimerRef.current);
    setContactCard(null);
  }, []);

  return { contactCard, handleAvatarEnter, handleAvatarLeave, closeContactCard };
}
