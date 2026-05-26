import { ContactObject, parseVCard } from '@/lib/api';

export const AVATAR_COLORS = [
  '#6366f1', '#8b5cf6', '#ec4899', '#ef4444',
  '#f97316', '#eab308', '#22c55e', '#14b8a6',
  '#3b82f6', '#06b6d4',
];

export function avatarColor(name: string): string {
  let h = 0;
  for (let i = 0; i < name.length; i++) h = (h * 31 + name.charCodeAt(i)) & 0xffff;
  return AVATAR_COLORS[h % AVATAR_COLORS.length];
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/);
  if (parts.length === 0 || !parts[0]) return '?';
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

export interface ParsedContact {
  fn: string;
  email: string;
  tel: string;
  org: string;
  title: string;
  note: string;
}

export type ContactsSort = 'name' | 'email' | 'company';
export type ContactsDensity = 'comfortable' | 'compact';

export function loadContactViewSettings() {
  try {
    const settings = JSON.parse(localStorage.getItem('webmail_settings') ?? '{}') as Record<string, unknown>;
    return {
      sort: (settings.contactsSort as ContactsSort) ?? 'name',
      density: (settings.contactsDensity as ContactsDensity) ?? 'comfortable',
      showCompany: settings.contactsShowCompany !== false,
    };
  } catch {
    return { sort: 'name' as ContactsSort, density: 'comfortable' as ContactsDensity, showCompany: true };
  }
}

export function useContactsParsed(contacts: ContactObject[]): ParsedContact[] {
  return contacts.map((c) => parseVCard(c.VCard));
}
