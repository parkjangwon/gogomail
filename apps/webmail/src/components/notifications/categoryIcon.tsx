'use client';

import {
  EnvelopeIcon,
  PaperAirplaneIcon,
  ExclamationTriangleIcon,
  XCircleIcon,
  CalendarDaysIcon,
  CalendarIcon,
  CloudIcon,
  BellIcon,
} from '@heroicons/react/24/outline';
import type { NotificationCategory, NotificationSeverity } from '@/lib/notifications/types';

export function severityColor(severity: NotificationSeverity): string {
  switch (severity) {
    case 'success':
      return 'var(--color-success, #16a34a)';
    case 'warning':
      return 'var(--color-warning, #ca8a04)';
    case 'error':
      return 'var(--color-destructive, #dc2626)';
    case 'info':
    default:
      return 'var(--color-accent)';
  }
}

export function CategoryIcon({
  category,
  severity,
  size = 18,
}: {
  category: NotificationCategory;
  severity: NotificationSeverity;
  size?: number;
}) {
  const color = severityColor(severity);
  const style: React.CSSProperties = { width: size, height: size, color };
  switch (category) {
    case 'mail_received':
      return <EnvelopeIcon style={style} />;
    case 'mail_sent':
      return <PaperAirplaneIcon style={style} />;
    case 'mail_send_failed':
      return <XCircleIcon style={style} />;
    case 'mail_bounced':
      return <ExclamationTriangleIcon style={style} />;
    case 'calendar_reminder':
      return <CalendarIcon style={style} />;
    case 'calendar_invite':
      return <CalendarDaysIcon style={style} />;
    case 'drive_share':
      return <CloudIcon style={style} />;
    case 'system':
    case 'custom':
    default:
      return <BellIcon style={style} />;
  }
}
