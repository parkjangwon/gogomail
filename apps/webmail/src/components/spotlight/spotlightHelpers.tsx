import { ReactNode } from 'react';
import { useTranslations } from 'next-intl';
import {
  InboxIcon,
  PaperAirplaneIcon,
  PencilSquareIcon,
  TrashIcon,
  ArchiveBoxIcon,
} from '@heroicons/react/24/outline';

export interface SpotlightItem {
  type: 'action' | 'mail' | 'contact' | 'calendar' | 'drive' | 'folder' | 'template' | 'settings' | 'notification';
  id: string;
  title: string;
  subtitle?: string;
  badge?: string;
  icon: ReactNode;
  onSelect: () => void;
}

export const SYSTEM_ICONS: Record<string, ReactNode> = {
  inbox: <InboxIcon style={{ width: 16, height: 16 }} />,
  sent: <PaperAirplaneIcon style={{ width: 16, height: 16 }} />,
  drafts: <PencilSquareIcon style={{ width: 16, height: 16 }} />,
  trash: <TrashIcon style={{ width: 16, height: 16 }} />,
  spam: <ArchiveBoxIcon style={{ width: 16, height: 16 }} />,
  archive: <ArchiveBoxIcon style={{ width: 16, height: 16 }} />,
};

export const SCOPES = ['all', 'mail', 'contacts', 'calendar', 'drive', 'folders', 'commands', 'settings', 'notifications'] as const;

export type SpotlightT = ReturnType<typeof useTranslations>;

export function sectionLabel(t: SpotlightT, type: SpotlightItem['type']): string {
  switch (type) {
    case 'action': return t('section.action');
    case 'folder': return t('section.folder');
    case 'mail': return t('section.mail');
    case 'contact': return t('section.contact');
    case 'calendar': return t('section.calendar');
    case 'drive': return t('section.drive');
    case 'template': return t('section.template');
    case 'settings': return t('section.settings');
    case 'notification': return t('section.notification');
  }
}

export function relativeTime(t: SpotlightT, iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const m = Math.floor(diff / 60000);
  if (m < 1) return t('time.justNow');
  if (m < 60) return t('time.minutesAgo', { n: m });
  const h = Math.floor(m / 60);
  if (h < 24) return t('time.hoursAgo', { n: h });
  const d = Math.floor(h / 24);
  if (d < 7) return t('time.daysAgo', { n: d });
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' }).format(new Date(iso));
}

export function formatDriveSize(bytes: number): string {
  if (!Number.isFinite(bytes) || bytes <= 0) return '';
  if (bytes < 1024) return `${bytes}B`;
  const units = ['KB', 'MB', 'GB', 'TB'];
  let value = bytes / 1024;
  let index = 0;
  while (value >= 1024 && index < units.length - 1) {
    value /= 1024;
    index += 1;
  }
  return `${value >= 10 ? Math.round(value) : value.toFixed(1)}${units[index]}`;
}
