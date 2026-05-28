'use client';

import { useEffect, useRef } from 'react';
import { MessageSummary, markRead, starMessage, moveMessage } from '@/lib/api';
import { loadFilterRules } from '@/components/settings/settingsConfig';

interface UseMailFilterRulesParams {
  messages: MessageSummary[];
  setMessages: (updater: (prev: MessageSummary[]) => MessageSummary[]) => void;
  adjustUnread: (folderId: string, delta: number) => void;
  activeFolderId: string;
  setLabel: (id: string, color: string | null) => void;
  setMessageLabels: (updater: (prev: Record<string, string>) => Record<string, string>) => void;
  folders: Array<{ id: string; system_type?: string }>;
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  t: (key: string, values?: Record<string, any>) => string;
}

export function useMailFilterRules(params: UseMailFilterRulesParams) {
  const {
    messages,
    setMessages,
    setMessageLabels,
    folders,
  } = params;

  // Track message IDs that have already been processed by filter rules to
  // prevent re-applying actions on re-renders triggered by setMessages calls.
  const processedIdsRef = useRef(new Set<string>());

  // Apply client-side filter rules to newly loaded messages
  useEffect(() => {
    if (messages.length === 0) return;
    const rules = loadFilterRules().filter((r) => r.enabled);
    if (rules.length === 0) return;

    // Only process messages we haven't seen before
    const unprocessed = messages.filter((m) => !processedIdsRef.current.has(m.id));
    if (unprocessed.length === 0) return;
    unprocessed.forEach((m) => processedIdsRef.current.add(m.id));

    const labelUpdates: Record<string, string> = {};
    const markReadIds: string[] = [];
    const markUnreadIds: string[] = [];
    const markStarredIds: string[] = [];
    const trashIds: string[] = [];

    for (const msg of unprocessed) {
      for (const rule of rules) {
        const condResults = rule.conditions.map((cond) => {
          if (cond.field === 'has_attachment') return !!(msg as MessageSummary & { has_attachment?: boolean }).has_attachment;
          if (cond.field === 'is_unread') return !msg.read;
          if (cond.field === 'size_larger') return ((msg as MessageSummary & { size?: number }).size ?? 0) > Number(cond.value);
          if (cond.field === 'size_smaller') return ((msg as MessageSummary & { size?: number }).size ?? Infinity) < Number(cond.value);
          const haystack = ((): string => {
            switch (cond.field) {
              case 'from': return (msg.from_addr + ' ' + (msg.from_name ?? '')).toLowerCase();
              case 'to': return ((msg as MessageSummary & { to?: string }).to ?? '').toLowerCase();
              case 'cc': return ((msg as MessageSummary & { cc?: string }).cc ?? '').toLowerCase();
              case 'subject': return (msg.subject ?? '').toLowerCase();
              case 'body': return (msg.preview ?? '').toLowerCase();
              default: return '';
            }
          })();
          const needle = cond.value.toLowerCase();
          switch (cond.matchType) {
            case 'contains': return haystack.includes(needle);
            case 'not_contains': return !haystack.includes(needle);
            case 'equals': return haystack.trim() === needle;
            case 'starts_with': return haystack.startsWith(needle);
            case 'ends_with': return haystack.endsWith(needle);
            case 'regex': try { return new RegExp(cond.value, 'i').test(haystack); } catch { return false; }
            default: return false;
          }
        });
        const matches = rule.logic === 'and' ? condResults.every(Boolean) : condResults.some(Boolean);
        if (!matches) continue;

        const a = rule.action;
        if (a.labelColor && !labelUpdates[msg.id]) labelUpdates[msg.id] = a.labelColor;
        if (a.markRead && !msg.read) markReadIds.push(msg.id);
        if (a.markUnread && msg.read) markUnreadIds.push(msg.id);
        if (a.markStarred && !msg.starred) markStarredIds.push(msg.id);
        if (a.deleteMsg) trashIds.push(msg.id);
        if (rule.stopProcessing) break;
      }
    }

    if (Object.keys(labelUpdates).length > 0) {
      setMessageLabels((prev) => {
        const next = { ...prev };
        let changed = false;
        for (const [id, color] of Object.entries(labelUpdates)) {
          if (!next[id]) { next[id] = color; changed = true; }
        }
        if (changed) { try { localStorage.setItem('webmail_labels', JSON.stringify(next)); } catch { /* */ } return next; }
        return prev;
      });
    }
    if (markReadIds.length > 0) {
      setMessages((prev) => prev.map((m) => markReadIds.includes(m.id) ? { ...m, read: true } : m));
      markReadIds.forEach((id) => markRead(id, true).catch(() => {}));
    }
    if (markUnreadIds.length > 0) {
      setMessages((prev) => prev.map((m) => markUnreadIds.includes(m.id) ? { ...m, read: false } : m));
      markUnreadIds.forEach((id) => markRead(id, false).catch(() => {}));
    }
    if (markStarredIds.length > 0) {
      setMessages((prev) => prev.map((m) => markStarredIds.includes(m.id) ? { ...m, starred: true } : m));
      markStarredIds.forEach((id) => starMessage(id, true).catch(() => {}));
    }
    if (trashIds.length > 0) {
      const trashFolder = folders.find((f) => f.system_type === 'trash');
      if (trashFolder) {
        setMessages((prev) => prev.filter((m) => !trashIds.includes(m.id)));
        trashIds.forEach((id) => moveMessage(id, trashFolder.id).catch(() => {}));
      }
    }
  }, [messages, folders]);
}
