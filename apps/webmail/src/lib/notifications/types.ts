/**
 * Notification center — public type contracts.
 *
 * Extending: add new categories to `NotificationCategory`. The store, bell,
 * and item components treat unknown categories gracefully (they fall back to
 * the generic icon and severity styling).
 */

export type NotificationCategory =
  | 'mail_received'
  | 'mail_sent'
  | 'mail_send_failed'
  | 'mail_bounced'
  | 'calendar_reminder'
  | 'calendar_invite'
  | 'drive_share'
  | 'system'
  | 'custom';

export type NotificationSeverity = 'info' | 'success' | 'warning' | 'error';

export interface Notification {
  id: string;
  category: NotificationCategory;
  severity: NotificationSeverity;
  title: string;
  body?: string;
  timestamp: number; // epoch ms
  read: boolean;
  actionUrl?: string;
  iconName?: string;
  metadata?: Record<string, unknown>;
}

export type NotificationInput = Omit<Notification, 'id' | 'timestamp' | 'read'> & {
  /** Optional client-supplied id for deduplication (e.g. per-event keys). */
  id?: string;
  /** If true and an existing entry shares the same id, skip the push. */
  dedupe?: boolean;
};
