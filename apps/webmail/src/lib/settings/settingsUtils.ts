import { stableId } from '@/lib/stableId';

export type ReadMark = 'instant' | '2s' | 'manual';
export type ExternalImages = 'always' | 'ask' | 'never';
export type SendDelay = 0 | 5 | 10 | 30;
export type Theme = 'light' | 'dark' | 'system';
export type FontSize = 'small' | 'medium' | 'large';

export const ACCENT_COLORS = [
  { value: '#2563eb', label: 'Blue', labelKey: 'misc.settingsAccent.blue' },
  { value: '#7c3aed', label: 'Purple', labelKey: 'misc.settingsAccent.purple' },
  { value: '#0d9488', label: 'Teal', labelKey: 'misc.settingsAccent.teal' },
  { value: '#16a34a', label: 'Green', labelKey: 'misc.settingsAccent.green' },
  { value: '#dc2626', label: 'Red', labelKey: 'misc.settingsAccent.red' },
  { value: '#ea580c', label: 'Orange', labelKey: 'misc.settingsAccent.orange' },
  { value: '#d97706', label: 'Gold', labelKey: 'misc.settingsAccent.gold' },
];

export interface FilterCondition {
  field: 'from' | 'to' | 'cc' | 'subject' | 'body' | 'has_attachment' | 'is_unread' | 'size_larger' | 'size_smaller';
  matchType: 'contains' | 'not_contains' | 'equals' | 'starts_with' | 'ends_with' | 'regex';
  value: string;
}

export interface FilterAction {
  labelColor?: string;
  moveToFolder?: string;
  markRead?: boolean;
  markUnread?: boolean;
  markStarred?: boolean;
  markImportant?: boolean;
  skipInbox?: boolean;
  deleteMsg?: boolean;
  forwardTo?: string;
}

export interface FilterRule {
  id: string;
  name: string;
  enabled: boolean;
  logic: 'and' | 'or';
  conditions: FilterCondition[];
  action: FilterAction;
  stopProcessing?: boolean;
}

export const FILTER_RULES_KEY = 'webmail_filter_rules';
export const LABEL_COLORS = ['#ef4444', '#f97316', '#eab308', '#22c55e', '#3b82f6', '#8b5cf6', '#ec4899', '#6b7280'];

export function migrateFilterRule(r: Record<string, unknown>): FilterRule {
  if (Array.isArray(r.conditions)) return { enabled: true, ...r } as unknown as FilterRule;
  return {
    id: (r.id as string) ?? stableId('filter'),
    name: (r.name as string) ?? '',
    enabled: true,
    logic: 'and',
    conditions: [{ field: (r.field === 'any' ? 'from' : (r.field as FilterCondition['field'])) ?? 'from', matchType: 'contains', value: (r.value as string) ?? '' }],
    action: { labelColor: r.labelColor as string | undefined },
  };
}

export function loadFilterRules(): FilterRule[] {
  try {
    const raw = JSON.parse(localStorage.getItem(FILTER_RULES_KEY) ?? '[]');
    return (Array.isArray(raw) ? raw : []).map((r) => (typeof r === 'object' && r !== null ? migrateFilterRule(r) : null)).filter((r): r is FilterRule => r !== null);
  } catch {
    return [];
  }
}

export function saveFilterRules(rules: FilterRule[]): void {
  localStorage.setItem(FILTER_RULES_KEY, JSON.stringify(rules));
}
