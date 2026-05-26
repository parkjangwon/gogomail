import { useEffect, useState } from 'react';
import {
  Attachment,
  MessageDeliveryStatus,
  TrackingEvent,
  createCalendarEvent,
  getMessageDeliveryStatus,
  getMessageTracking,
  listCalendars,
} from '@/lib/api';
import { ICSEvent } from './readingPaneTypes';

interface UseReadingPaneCalendarParams {
  messageId: string | undefined;
  fromAddr: string | undefined;
  userEmail: string | undefined;
  folderId: string | undefined;
  folderSystemType: string | undefined;
  attachments: Attachment[];
}

interface UseReadingPaneCalendarResult {
  icsEvents: ICSEvent[];
  addingCalendarId: string | null;
  calendarAdded: string | null;
  deliveryStatus: MessageDeliveryStatus | null;
  deliveryOpen: boolean;
  setDeliveryOpen: React.Dispatch<React.SetStateAction<boolean>>;
  trackingEvents: TrackingEvent[] | null;
  trackingOpen: boolean;
  setTrackingOpen: React.Dispatch<React.SetStateAction<boolean>>;
  handleAddToCalendar: (event: ICSEvent) => Promise<void>;
}

const parseIcsDate = (value: string): Date | null => {
  try {
    const clean = value.trim().replace(/z$/i, '');
    if (clean.length === 8) {
      return new Date(`${clean.slice(0, 4)}-${clean.slice(4, 6)}-${clean.slice(6, 8)}T00:00:00`);
    }
    if (clean.includes('T')) {
      return new Date(`${clean.slice(0, 4)}-${clean.slice(4, 6)}-${clean.slice(6, 8)}T${clean.slice(9, 11)}:${clean.slice(11, 13)}:${clean.slice(13, 15)}`);
    }
    return new Date(clean);
  } catch {
    return null;
  }
};

export function useReadingPaneCalendar({
  messageId,
  fromAddr,
  userEmail,
  folderSystemType,
  attachments,
}: UseReadingPaneCalendarParams): UseReadingPaneCalendarResult {
  const [icsEvents, setIcsEvents] = useState<ICSEvent[]>([]);
  const [addingCalendarId, setAddingCalendarId] = useState<string | null>(null);
  const [calendarAdded, setCalendarAdded] = useState<string | null>(null);
  const [deliveryStatus, setDeliveryStatus] = useState<MessageDeliveryStatus | null>(null);
  const [deliveryOpen, setDeliveryOpen] = useState(false);
  const [trackingEvents, setTrackingEvents] = useState<TrackingEvent[] | null>(null);
  const [trackingOpen, setTrackingOpen] = useState(false);

  useEffect(() => {
    if (attachments.length === 0) {
      setIcsEvents([]);
      return;
    }
    const icsAtts = attachments.filter((a) => a.filename.toLowerCase().endsWith('.ics') || a.mime_type === 'text/calendar');
    if (icsAtts.length === 0) {
      setIcsEvents([]);
      return;
    }
    if (!messageId) return;
    Promise.all(
      icsAtts.map(async (att) => {
        try {
          const resp = await fetch(`/api/mail/messages/${messageId}/attachments/${att.id}/download`);
          if (!resp.ok) return null;
          const text = await resp.text();
          const get = (key: string) => {
            const m = text.match(new RegExp(`^${key}[;:][^:]*:?(.+)$`, 'mi'));
            return m ? m[1].trim() : undefined;
          };
          const summary = get('SUMMARY');
          const dtstart = get('DTSTART');
          if (!summary || !dtstart) return null;
          return {
            summary,
            dtstart,
            dtend: get('DTEND'),
            location: get('LOCATION'),
            description: get('DESCRIPTION'),
          } as ICSEvent;
        } catch {
          return null;
        }
      }),
    ).then((results) => {
      setIcsEvents(results.filter(Boolean) as ICSEvent[]);
    });
  }, [attachments, messageId]);

  useEffect(() => {
    // Delivery tracking is only meaningful when viewing an outgoing message
    // from the Sent folder. Avoid showing it for self-sent emails sitting in
    // the inbox — the sender/recipient coincidence makes isSent=true even though
    // the user is reading it as a recipient, not as the original sender.
    const senderMatch = fromAddr && userEmail
      ? fromAddr.toLowerCase() === userEmail.toLowerCase()
      : false;
    const isSentView = senderMatch && folderSystemType === 'sent';

    setDeliveryStatus(null);
    setDeliveryOpen(false);
    setTrackingEvents(null);
    setTrackingOpen(false);

    if (!messageId || !isSentView) return;

    getMessageDeliveryStatus(messageId)
      .then(setDeliveryStatus)
      .catch(() => {});
    getMessageTracking(messageId)
      .then((events) => {
        if (events.length > 0) {
          setTrackingEvents(events);
        }
      })
      .catch(() => {});
  }, [messageId, fromAddr, userEmail, folderSystemType]);

  const handleAddToCalendar = async (event: ICSEvent) => {
    setAddingCalendarId(event.dtstart);
    try {
      const calendars = await listCalendars();
      const cal = calendars[0];
      if (!cal) return;
      const start = parseIcsDate(event.dtstart) ?? new Date();
      const end = parseIcsDate(event.dtend || '') ?? new Date(start.getTime() + 60 * 60 * 1000);
      await createCalendarEvent(cal.ID, {
        title: event.summary,
        start,
        end,
        allDay: event.dtstart.length === 8,
        location: event.location,
        description: event.description,
      });
      setCalendarAdded(event.dtstart);
      setTimeout(() => setCalendarAdded(null), 3000);
    } catch {
      // ignore
    } finally {
      setAddingCalendarId(null);
    }
  };

  return {
    icsEvents,
    addingCalendarId,
    calendarAdded,
    deliveryStatus,
    deliveryOpen,
    setDeliveryOpen,
    trackingEvents,
    trackingOpen,
    setTrackingOpen,
    handleAddToCalendar,
  };
}
