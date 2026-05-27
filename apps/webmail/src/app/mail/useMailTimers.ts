import { useEffect } from 'react';
import { type MessageSummary } from '@/lib/api';
import { type ToastItem } from '@/components/Toast';

interface UseMailTimersParams {
  messages: MessageSummary[];
  addToast: (message: string, type?: ToastItem['type'], options?: { duration?: number; action?: ToastItem['action'] }) => void;
  t: (key: string, values?: Record<string, unknown>) => string;
  refresh: () => void;
}

export function useMailTimers({
  messages,
  addToast,
  t,
  refresh,
}: UseMailTimersParams): void {
  // Check every 60s if any snoozed message should reappear
  useEffect(() => {
    const check = () => {
      try {
        const stored: Record<string, string> = JSON.parse(localStorage.getItem('webmail_snoozed') ?? '{}');
        const now = Date.now();
        const expired = Object.entries(stored).filter(([, ts]) => new Date(ts).getTime() <= now);
        if (expired.length === 0) return;
        const remaining = { ...stored };
        expired.forEach(([id]) => delete remaining[id]);
        localStorage.setItem('webmail_snoozed', JSON.stringify(remaining));
        addToast(t('misc.mailPage.snoozeReturned', { count: expired.length }), 'info');
        refresh();
      } catch { /* ignore */ }
    };
    const id = setInterval(check, 60_000);
    check();
    return () => clearInterval(id);
  }, [addToast, refresh]);

  // Check for overdue follow-up reminders on load and every 5 minutes
  useEffect(() => {
    const checkFollowUps = () => {
      try {
        type FollowUp = { remindAt: string; subject: string; to: string; createdAt: string };
        const followups: FollowUp[] = JSON.parse(localStorage.getItem('webmail_followups') ?? '[]');
        const now = Date.now();
        const overdue = followups.filter((f) => new Date(f.remindAt).getTime() <= now);
        if (overdue.length === 0) return;
        const remaining = followups.filter((f) => new Date(f.remindAt).getTime() > now);
        localStorage.setItem('webmail_followups', JSON.stringify(remaining));
        overdue.forEach((f) => {
          addToast(t('misc.mailPage.followUpReminder', { subject: f.subject || t('misc.mailPage.noSubject') }), 'info', { duration: 8000 });
        });
      } catch { /* ignore */ }
    };
    checkFollowUps();
    const id = setInterval(checkFollowUps, 5 * 60_000);
    return () => clearInterval(id);
  }, [addToast]);

  // Extract sender names from messages and store as contacts
  useEffect(() => {
    if (messages.length === 0) return;
    try {
      const stored: Record<string, string> = JSON.parse(localStorage.getItem('webmail_contacts') ?? '{}');
      let changed = false;
      messages.forEach((m) => {
        if (m.from_name && m.from_addr) {
          const key = m.from_addr.toLowerCase();
          if (!stored[key] || stored[key] !== m.from_name) {
            stored[key] = m.from_name;
            changed = true;
          }
        }
      });
      if (changed) localStorage.setItem('webmail_contacts', JSON.stringify(stored));
    } catch { /* ignore */ }
  }, [messages]);
}
